package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

const defaultBaseURL = "https://api.anthropic.com/v1/messages"

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
	return "anthropic"
}

func (p *Provider) Complete(
	ctx context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	if p.apiKey == "" {
		return llm.CompletionResponse{}, fmt.Errorf("anthropic api key is not configured")
	}

	payload := messagesRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		System:    req.System,
		Messages:  make([]message, 0, len(req.Messages)),
	}
	for _, msg := range req.Messages {
		payload.Messages = append(payload.Messages, message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	if req.CacheControl != nil {
		payload.CacheControl = &cacheControl{
			Type: req.CacheControl.Type,
			TTL:  req.CacheControl.TTL,
		}
	}
	if req.OutputFormat != nil {
		payload.OutputConfig = &outputConfig{
			Format: outputFormat{
				Type:   "json_schema",
				Schema: req.OutputFormat.Schema,
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("build anthropic request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("send anthropic request: %w", err)
	}

	respBody, readErr := io.ReadAll(httpResp.Body)
	closeErr := httpResp.Body.Close()
	if readErr != nil {
		err = fmt.Errorf("read anthropic response: %w", readErr)
		if closeErr != nil {
			return llm.CompletionResponse{}, errors.Join(
				err,
				fmt.Errorf("close anthropic response body: %w", closeErr),
			)
		}
		return llm.CompletionResponse{}, err
	}
	if httpResp.StatusCode >= http.StatusBadRequest {
		err = decodeError(
			"anthropic",
			httpResp.StatusCode,
			respBody,
		)
		if closeErr != nil {
			return llm.CompletionResponse{}, errors.Join(
				err,
				fmt.Errorf("close anthropic response body: %w", closeErr),
			)
		}
		return llm.CompletionResponse{}, err
	}
	if closeErr != nil {
		return llm.CompletionResponse{}, fmt.Errorf("close anthropic response body: %w", closeErr)
	}

	var parsed messagesResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("decode anthropic response: %w", err)
	}

	var parts []string
	for _, block := range parsed.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}

	return llm.CompletionResponse{
		Content:  strings.Join(parts, ""),
		Provider: p.Name(),
		Model:    parsed.Model,
		Usage: llm.Usage{
			Provider:                 p.Name(),
			Model:                    parsed.Model,
			InputTokens:              parsed.Usage.InputTokens,
			OutputTokens:             parsed.Usage.OutputTokens,
			CacheCreationInputTokens: parsed.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     parsed.Usage.CacheReadInputTokens,
		},
	}, nil
}

type messagesRequest struct {
	Model        string        `json:"model"`
	MaxTokens    int           `json:"max_tokens"`
	System       string        `json:"system,omitempty"`
	Messages     []message     `json:"messages"`
	OutputConfig *outputConfig `json:"output_config,omitempty"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type outputConfig struct {
	Format outputFormat `json:"format"`
}

type outputFormat struct {
	Type   string         `json:"type"`
	Schema map[string]any `json:"schema"`
}

type cacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

type messagesResponse struct {
	Model   string         `json:"model"`
	Content []contentBlock `json:"content"`
	Usage   usage          `json:"usage"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func decodeError(provider string, statusCode int, body []byte) error {
	var parsed errorEnvelope
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		return fmt.Errorf("%s api error (%d): %s", provider, statusCode, parsed.Error.Message)
	}
	return fmt.Errorf("%s api error (%d): %s", provider, statusCode, strings.TrimSpace(string(body)))
}
