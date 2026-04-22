package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

const messagesEndpointPath = "/v1/messages"

type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type Provider struct {
	apiKey string
	client anthropic.Client
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
	if baseURL := llm.NormalizeBaseURL(cfg.BaseURL, messagesEndpointPath); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	return &Provider{
		apiKey: apiKey,
		client: anthropic.NewClient(opts...),
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

	params, err := buildMessageParams(req)
	if err != nil {
		return llm.CompletionResponse{}, err
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return llm.CompletionResponse{}, fmt.Errorf("send anthropic request: %w", err)
	}

	var parts []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}

	return llm.CompletionResponse{
		Content:  strings.Join(parts, ""),
		Provider: p.Name(),
		Model:    string(resp.Model),
		Usage: llm.Usage{
			Provider:                 p.Name(),
			Model:                    string(resp.Model),
			InputTokens:              int(resp.Usage.InputTokens),
			OutputTokens:             int(resp.Usage.OutputTokens),
			CacheCreationInputTokens: int(resp.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int(resp.Usage.CacheReadInputTokens),
		},
	}, nil
}

func buildMessageParams(
	req llm.CompletionRequest,
) (anthropic.MessageNewParams, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  make([]anthropic.MessageParam, 0, len(req.Messages)),
	}
	if system := strings.TrimSpace(req.System); system != "" {
		block := anthropic.TextBlockParam{Text: system}
		if req.CacheControl != nil {
			cc, err := buildCacheControl(req.CacheControl)
			if err != nil {
				return anthropic.MessageNewParams{}, err
			}
			block.CacheControl = cc
		}
		params.System = []anthropic.TextBlockParam{block}
	}
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			if cachedText := strings.TrimSpace(msg.CachedContent); cachedText != "" {
				cachedBlock := anthropic.TextBlockParam{Text: cachedText}
				if req.CacheControl != nil {
					cc, err := buildCacheControl(req.CacheControl)
					if err != nil {
						return anthropic.MessageNewParams{}, err
					}
					cachedBlock.CacheControl = cc
				}
				blocks := []anthropic.ContentBlockParamUnion{
					{OfText: &cachedBlock},
				}
				if dynamicText := strings.TrimSpace(msg.Content); dynamicText != "" {
					blocks = append(blocks, anthropic.NewTextBlock(dynamicText))
				}
				params.Messages = append(
					params.Messages,
					anthropic.NewUserMessage(blocks...),
				)
			} else {
				params.Messages = append(
					params.Messages,
					anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)),
				)
			}
		case "assistant":
			params.Messages = append(
				params.Messages,
				anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)),
			)
		default:
			return anthropic.MessageNewParams{}, fmt.Errorf(
				"unsupported anthropic message role %q",
				msg.Role,
			)
		}
	}
	if req.OutputFormat != nil {
		params.OutputConfig = anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{
				Schema: req.OutputFormat.Schema,
			},
		}
	}
	return params, nil
}

func buildCacheControl(
	cfg *llm.CacheControl,
) (anthropic.CacheControlEphemeralParam, error) {
	if cfg == nil {
		return anthropic.CacheControlEphemeralParam{}, nil
	}
	if cacheType := strings.TrimSpace(cfg.Type); cacheType != "" && cacheType != "ephemeral" {
		return anthropic.CacheControlEphemeralParam{}, fmt.Errorf(
			"unsupported anthropic cache control type %q",
			cfg.Type,
		)
	}

	cacheControl := anthropic.NewCacheControlEphemeralParam()
	switch strings.TrimSpace(cfg.TTL) {
	case "", "5m":
		cacheControl.TTL = anthropic.CacheControlEphemeralTTLTTL5m
	case "1h":
		cacheControl.TTL = anthropic.CacheControlEphemeralTTLTTL1h
	default:
		return anthropic.CacheControlEphemeralParam{}, fmt.Errorf(
			"unsupported anthropic cache control ttl %q",
			cfg.TTL,
		)
	}

	return cacheControl, nil
}
