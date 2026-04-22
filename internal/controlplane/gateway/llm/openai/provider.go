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

type StructuredOutputMode string

const (
	StructuredOutputJSONSchema StructuredOutputMode = "json_schema"
	StructuredOutputToolCall   StructuredOutputMode = "tool_call"
)

type TokenParameterStyle string

const (
	TokenParameterMaxCompletionTokens TokenParameterStyle = "max_completion_tokens"
	TokenParameterMaxTokens           TokenParameterStyle = "max_tokens"
)

type Config struct {
	Name                 string
	APIKey               string
	BaseURL              string
	HTTPClient           *http.Client
	StructuredOutputMode StructuredOutputMode
	TokenParameterStyle  TokenParameterStyle
}

type Provider struct {
	name                 string
	apiKey               string
	client               openai.Client
	structuredOutputMode StructuredOutputMode
	tokenParameterStyle  TokenParameterStyle
}

func New(cfg Config) *Provider {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}

	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = "openai"
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	opts := []option.RequestOption{option.WithHTTPClient(httpClient)}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL := llm.NormalizeBaseURL(cfg.BaseURL, chatCompletionsEndpointPath); baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	structuredOutputMode := cfg.StructuredOutputMode
	if structuredOutputMode == "" {
		structuredOutputMode = StructuredOutputJSONSchema
	}

	tokenParameterStyle := cfg.TokenParameterStyle
	if tokenParameterStyle == "" {
		tokenParameterStyle = TokenParameterMaxCompletionTokens
	}

	return &Provider{
		name:                 name,
		apiKey:               apiKey,
		client:               openai.NewClient(opts...),
		structuredOutputMode: structuredOutputMode,
		tokenParameterStyle:  tokenParameterStyle,
	}
}

func (p *Provider) Name() string {
	return p.name
}

func (p *Provider) Complete(
	ctx context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	if p.apiKey == "" {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("%s api key is not configured", p.Name()),
		)
	}
	if strings.TrimSpace(req.Model) == "" {
		return llm.CompletionResponse{}, llm.NewProviderError(
			p.Name(),
			fmt.Errorf("%s model is not configured", p.Name()),
		)
	}

	params, err := buildChatCompletionParams(
		req,
		p.structuredOutputMode,
		p.tokenParameterStyle,
	)
	if err != nil {
		return llm.CompletionResponse{}, llm.NewProviderError(p.Name(), err)
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return llm.CompletionResponse{}, classifyCompatibleError(p.Name(), err)
	}
	content, err := extractResponseContent(resp, req.OutputFormat, p.structuredOutputMode)
	if err != nil {
		return llm.CompletionResponse{}, llm.NewProviderError(p.Name(), err)
	}

	return llm.CompletionResponse{
		Content:  content,
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

func classifyCompatibleError(provider string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return llm.NewProviderError(provider, err)
	case errors.Is(err, context.DeadlineExceeded):
		return llm.NewTransientProviderError(provider, err)
	}

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests ||
			apiErr.StatusCode >= http.StatusInternalServerError {
			return llm.NewTransientProviderError(provider, err)
		}
		return llm.NewProviderError(provider, err)
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return llm.NewTransientProviderError(provider, err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return llm.NewTransientProviderError(provider, err)
	}

	return llm.NewProviderError(provider, err)
}

func buildChatCompletionParams(
	req llm.CompletionRequest,
	structuredOutputMode StructuredOutputMode,
	tokenParameterStyle TokenParameterStyle,
) (openai.ChatCompletionNewParams, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if system := strings.TrimSpace(req.System); system != "" {
		messages = append(messages, openai.SystemMessage(system))
	}
	for _, msg := range req.Messages {
		text := llm.MergeMessageContent(msg.CachedContent, msg.Content)
		switch msg.Role {
		case "user":
			messages = append(messages, openai.UserMessage(text))
		case "assistant":
			messages = append(messages, openai.AssistantMessage(text))
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
		switch tokenParameterStyle {
		case "", TokenParameterMaxCompletionTokens:
			params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
		case TokenParameterMaxTokens:
			params.MaxTokens = openai.Int(int64(req.MaxTokens))
		default:
			return openai.ChatCompletionNewParams{}, fmt.Errorf(
				"unsupported token parameter style %q",
				tokenParameterStyle,
			)
		}
	}
	if req.OutputFormat != nil {
		switch structuredOutputMode {
		case "", StructuredOutputJSONSchema:
			params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
					JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
						Name:   req.OutputFormat.Name,
						Strict: openai.Bool(true),
						Schema: req.OutputFormat.Schema,
					},
				},
			}
		case StructuredOutputToolCall:
			functionName := outputFormatToolName(req.OutputFormat.Name)
			params.ToolChoice = openai.ToolChoiceOptionFunctionToolChoice(
				openai.ChatCompletionNamedToolChoiceFunctionParam{
					Name: functionName,
				},
			)
			params.Tools = []openai.ChatCompletionToolUnionParam{
				openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
					Name:        functionName,
					Description: openai.String("Return the response as JSON arguments."),
					Strict:      openai.Bool(true),
					Parameters:  req.OutputFormat.Schema,
				}),
			}
		default:
			return openai.ChatCompletionNewParams{}, fmt.Errorf(
				"unsupported structured output mode %q",
				structuredOutputMode,
			)
		}
	}

	return params, nil
}

func extractResponseContent(
	resp *openai.ChatCompletion,
	outputFormat *llm.OutputFormat,
	structuredOutputMode StructuredOutputMode,
) (string, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai response contained no choices")
	}

	message := resp.Choices[0].Message
	if outputFormat != nil && structuredOutputMode == StructuredOutputToolCall {
		for _, toolCall := range message.ToolCalls {
			if functionCall, ok := toolCall.AsAny().(openai.ChatCompletionMessageFunctionToolCall); ok {
				if strings.TrimSpace(functionCall.Function.Arguments) != "" {
					return functionCall.Function.Arguments, nil
				}
			}
		}
		return "", fmt.Errorf("openai response contained no tool call arguments")
	}

	content := strings.TrimSpace(message.Content)
	if content == "" {
		return "", fmt.Errorf("openai response contained empty content")
	}
	return content, nil
}

func outputFormatToolName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "structured_output"
	}

	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_-")
	if out == "" {
		return "structured_output"
	}
	return out
}
