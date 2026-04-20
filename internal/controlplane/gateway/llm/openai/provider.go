package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

const defaultBaseURL = "https://api.openai.com/v1/chat/completions"

type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func New(cfg Config) *Provider {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &Provider{
		apiKey:  strings.TrimSpace(cfg.APIKey),
		baseURL: baseURL,
		client:  client,
	}
}

func (p *Provider) Name() string {
	return "openai"
}

func (p *Provider) Complete(
	ctx context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	if p.apiKey == "" {
		return llm.CompletionResponse{}, fmt.Errorf("openai api key is not configured")
	}

	payload := chatCompletionsRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Messages:  make([]chatMessage, 0, len(req.Messages)+1),
	}
	if strings.TrimSpace(req.System) != "" {
		payload.Messages = append(payload.Messages, chatMessage{
			Role:    "system",
			Content: req.System,
		})
	}
	for _, msg := range req.Messages {
		payload.Messages = append(payload.Messages, chatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	if req.OutputFormat != nil {
		payload.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: jsonSchemaFormat{
				Name:   req.OutputFormat.Name,
				Strict: true,
				Schema: req.OutputFormat.Schema,
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("build openai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("send openai request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("read openai response: %w", err)
	}
	if httpResp.StatusCode >= http.StatusBadRequest {
		return llm.CompletionResponse{}, decodeError(
			httpResp.StatusCode,
			respBody,
		)
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("decode openai response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return llm.CompletionResponse{}, fmt.Errorf("openai response contained no choices")
	}

	return llm.CompletionResponse{
		Content:  parsed.Choices[0].Message.Content,
		Provider: p.Name(),
		Model:    parsed.Model,
		Usage: llm.Usage{
			Provider:     p.Name(),
			Model:        parsed.Model,
			InputTokens:  parsed.Usage.PromptTokens,
			OutputTokens: parsed.Usage.CompletionTokens,
		},
	}, nil
}

type chatCompletionsRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_completion_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string           `json:"type"`
	JSONSchema jsonSchemaFormat `json:"json_schema"`
}

type jsonSchemaFormat struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatCompletionsResponse struct {
	Model   string               `json:"model"`
	Choices []chatChoice         `json:"choices"`
	Usage   chatCompletionsUsage `json:"usage"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatCompletionsUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func decodeError(statusCode int, body []byte) error {
	var parsed errorEnvelope
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		return fmt.Errorf("openai api error (%d): %s", statusCode, parsed.Error.Message)
	}
	return fmt.Errorf("openai api error (%d): %s", statusCode, strings.TrimSpace(string(body)))
}
