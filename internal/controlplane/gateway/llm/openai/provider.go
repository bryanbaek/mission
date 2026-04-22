package openai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

const chatCompletionsEndpointPath = "/chat/completions"

type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type Provider struct {
	apiKey string
	client openai.Client
}

func New(cfg Config) *Provider {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	opts := []option.RequestOption{option.WithHTTPClient(httpClient)}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL := llm.NormalizeBaseURL(cfg.BaseURL, chatCompletionsEndpointPath); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	return &Provider{
		apiKey: apiKey,
		client: openai.NewClient(opts...),
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
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("openai api key is not configured"),
		)
	}
	if strings.TrimSpace(req.Model) == "" {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("openai model is not configured"),
		)
	}

	params, err := buildChatCompletionParams(req)
	if err != nil {
		return llm.CompletionResponse{}, llm.NewProviderError(p.Name(), err)
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return llm.CompletionResponse{}, classifyOpenAIError(err)
	}
	if len(resp.Choices) == 0 {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("openai response contained no choices"),
		)
	}

	return llm.CompletionResponse{
		Content:  resp.Choices[0].Message.Content,
		Provider: p.Name(),
		Model:    resp.Model,
		Usage: llm.Usage{
			Provider:     p.Name(),
			Model:        resp.Model,
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
		},
	}, nil
}

func classifyOpenAIError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return llm.NewProviderError("openai", err)
	case errors.Is(err, context.DeadlineExceeded):
		return llm.NewTransientProviderError("openai", err)
	}

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests ||
			apiErr.StatusCode >= http.StatusInternalServerError {
			return llm.NewTransientProviderError("openai", err)
		}
		return llm.NewProviderError("openai", err)
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return llm.NewTransientProviderError("openai", err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return llm.NewTransientProviderError("openai", err)
	}

	return llm.NewProviderError("openai", err)
}

func buildChatCompletionParams(
	req llm.CompletionRequest,
) (openai.ChatCompletionNewParams, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if system := strings.TrimSpace(req.System); system != "" {
		messages = append(messages, openai.SystemMessage(system))
	}
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			messages = append(messages, openai.UserMessage(msg.Content))
		case "assistant":
			messages = append(messages, openai.AssistantMessage(msg.Content))
		default:
			return openai.ChatCompletionNewParams{}, fmt.Errorf(
				"unsupported openai message role %q",
				msg.Role,
			)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(req.Model),
		Messages: messages,
	}
	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.OutputFormat != nil {
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   req.OutputFormat.Name,
					Strict: openai.Bool(true),
					Schema: req.OutputFormat.Schema,
				},
			},
		}
	}

	return params, nil
}
