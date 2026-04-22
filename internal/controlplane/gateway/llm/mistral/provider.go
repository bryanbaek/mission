package mistral

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

const chatCompletionsEndpointPath = "/v1/chat/completions"

type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type Provider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type chatCompletionRequest struct {
	Model          string                 `json:"model"`
	Messages       []chatMessage          `json:"messages"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	ResponseFormat *mistralResponseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mistralResponseFormat struct {
	Type       string                   `json:"type"`
	JSONSchema *mistralJSONSchemaConfig `json:"json_schema,omitempty"`
}

type mistralJSONSchemaConfig struct {
	Name   string         `json:"name,omitempty"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict,omitempty"`
}

type chatCompletionResponse struct {
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
	Usage   mistralUsage           `json:"usage"`
}

type chatCompletionChoice struct {
	Message mistralMessage `json:"message"`
}

type mistralMessage struct {
	Content mistralContent `json:"content"`
}

type mistralContent struct {
	Text  string
	Parts []mistralContentPart
}

type mistralContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mistralUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (c *mistralContent) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		c.Text = text
		return nil
	}

	var parts []mistralContentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		c.Parts = parts
		return nil
	}

	return fmt.Errorf("unsupported mistral content payload: %s", string(data))
}

func (c mistralContent) String() string {
	if strings.TrimSpace(c.Text) != "" {
		return c.Text
	}

	var b strings.Builder
	for _, part := range c.Parts {
		if part.Type == "" || part.Type == "text" {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

func New(cfg Config) *Provider {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.mistral.ai"
	}

	return &Provider{
		apiKey:     strings.TrimSpace(cfg.APIKey),
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (p *Provider) Name() string {
	return "mistral"
}

func (p *Provider) Complete(
	ctx context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	if p.apiKey == "" {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("mistral api key is not configured"),
		)
	}
	if strings.TrimSpace(req.Model) == "" {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("mistral model is not configured"),
		)
	}

	payload, err := buildChatCompletionRequest(req)
	if err != nil {
		return llm.CompletionResponse{}, llm.NewProviderError(p.Name(), err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("marshal mistral request: %w", err),
		)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+chatCompletionsEndpointPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return llm.CompletionResponse{}, llm.NewProviderError(p.Name(), err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return llm.CompletionResponse{}, classifyMistralError(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return llm.CompletionResponse{}, classifyMistralHTTPError(httpResp.StatusCode)
	}

	var resp chatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("decode mistral response: %w", err),
		)
	}
	if len(resp.Choices) == 0 {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("mistral response contained no choices"),
		)
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content.String())
	if content == "" {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("mistral response contained empty content"),
		)
	}

	return llm.CompletionResponse{
		Content:  content,
		Provider: p.Name(),
		Model:    resp.Model,
		Usage: llm.Usage{
			Provider:     p.Name(),
			Model:        resp.Model,
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}, nil
}

func buildChatCompletionRequest(
	req llm.CompletionRequest,
) (chatCompletionRequest, error) {
	messages := make([]chatMessage, 0, len(req.Messages)+1)
	if system := strings.TrimSpace(req.System); system != "" {
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: system,
		})
	}
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user", "assistant":
			messages = append(messages, chatMessage{
				Role:    msg.Role,
				Content: llm.MergeMessageContent(msg.CachedContent, msg.Content),
			})
		default:
			return chatCompletionRequest{}, fmt.Errorf(
				"unsupported mistral message role %q",
				msg.Role,
			)
		}
	}

	out := chatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
	}
	if req.MaxTokens > 0 {
		out.MaxTokens = req.MaxTokens
	}
	if req.OutputFormat != nil {
		out.ResponseFormat = &mistralResponseFormat{
			Type: "json_schema",
			JSONSchema: &mistralJSONSchemaConfig{
				Name:   strings.TrimSpace(req.OutputFormat.Name),
				Schema: req.OutputFormat.Schema,
				Strict: true,
			},
		}
	}

	return out, nil
}

func classifyMistralError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return llm.NewProviderError("mistral", err)
	case errors.Is(err, context.DeadlineExceeded):
		return llm.NewTransientProviderError("mistral", err)
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return llm.NewTransientProviderError("mistral", err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return llm.NewTransientProviderError("mistral", err)
	}

	return llm.NewProviderError("mistral", err)
}

func classifyMistralHTTPError(statusCode int) error {
	err := fmt.Errorf("mistral api returned status %d", statusCode)
	if statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError {
		return llm.NewTransientProviderError("mistral", err)
	}
	return llm.NewProviderError("mistral", err)
}
