package llmprovider

import (
	"fmt"
	"net/http"
	"strings"

	llmgateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	anthropicgateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm/anthropic"
	mistralgateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm/mistral"
	openaigateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm/openai"
)

const DefaultProviderName = "anthropic"

type Transport string

const (
	TransportAnthropic        Transport = "anthropic"
	TransportOpenAICompatible Transport = "openai_compatible"
	TransportMistral          Transport = "mistral"
)

type Spec struct {
	Name                  string
	APIKeyEnv             string
	SemanticLayerModelEnv string
	QueryModelEnv         string
	PreflightModelEnv     string
	DefaultBaseURL        string
	DefaultPreflightModel string
	Transport             Transport
	StructuredOutputMode  openaigateway.StructuredOutputMode
	TokenParameterStyle   openaigateway.TokenParameterStyle
}

var supportedSpecs = []Spec{
	{
		Name:                  "anthropic",
		APIKeyEnv:             "ANTHROPIC_API_KEY",
		SemanticLayerModelEnv: "ANTHROPIC_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "ANTHROPIC_QUERY_MODEL",
		PreflightModelEnv:     "ANTHROPIC_PREFLIGHT_MODEL",
		DefaultPreflightModel: "claude-3-5-haiku-latest",
		Transport:             TransportAnthropic,
	},
	{
		Name:                  "openai",
		APIKeyEnv:             "OPENAI_API_KEY",
		SemanticLayerModelEnv: "OPENAI_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "OPENAI_QUERY_MODEL",
		PreflightModelEnv:     "OPENAI_PREFLIGHT_MODEL",
		DefaultPreflightModel: "gpt-4.1-nano",
		Transport:             TransportOpenAICompatible,
		StructuredOutputMode:  openaigateway.StructuredOutputJSONSchema,
		TokenParameterStyle:   openaigateway.TokenParameterMaxCompletionTokens,
	},
	{
		Name:                  "together",
		APIKeyEnv:             "TOGETHER_API_KEY",
		SemanticLayerModelEnv: "TOGETHER_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "TOGETHER_QUERY_MODEL",
		PreflightModelEnv:     "TOGETHER_PREFLIGHT_MODEL",
		DefaultBaseURL:        "https://api.together.xyz/v1",
		Transport:             TransportOpenAICompatible,
		StructuredOutputMode:  openaigateway.StructuredOutputJSONSchema,
		TokenParameterStyle:   openaigateway.TokenParameterMaxTokens,
	},
	{
		Name:                  "mistral",
		APIKeyEnv:             "MISTRAL_API_KEY",
		SemanticLayerModelEnv: "MISTRAL_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "MISTRAL_QUERY_MODEL",
		PreflightModelEnv:     "MISTRAL_PREFLIGHT_MODEL",
		DefaultBaseURL:        "https://api.mistral.ai",
		Transport:             TransportMistral,
	},
	{
		Name:                  "cerebras",
		APIKeyEnv:             "CEREBRAS_API_KEY",
		SemanticLayerModelEnv: "CEREBRAS_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "CEREBRAS_QUERY_MODEL",
		PreflightModelEnv:     "CEREBRAS_PREFLIGHT_MODEL",
		DefaultBaseURL:        "https://api.cerebras.ai/v1",
		Transport:             TransportOpenAICompatible,
		StructuredOutputMode:  openaigateway.StructuredOutputJSONSchema,
		TokenParameterStyle:   openaigateway.TokenParameterMaxTokens,
	},
	{
		Name:                  "deepseek",
		APIKeyEnv:             "DEEPSEEK_API_KEY",
		SemanticLayerModelEnv: "DEEPSEEK_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "DEEPSEEK_QUERY_MODEL",
		PreflightModelEnv:     "DEEPSEEK_PREFLIGHT_MODEL",
		DefaultBaseURL:        "https://api.deepseek.com",
		Transport:             TransportOpenAICompatible,
		StructuredOutputMode:  openaigateway.StructuredOutputToolCall,
		TokenParameterStyle:   openaigateway.TokenParameterMaxTokens,
	},
	{
		Name:                  "xai",
		APIKeyEnv:             "XAI_API_KEY",
		SemanticLayerModelEnv: "XAI_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "XAI_QUERY_MODEL",
		PreflightModelEnv:     "XAI_PREFLIGHT_MODEL",
		DefaultBaseURL:        "https://api.x.ai/v1",
		Transport:             TransportOpenAICompatible,
		StructuredOutputMode:  openaigateway.StructuredOutputJSONSchema,
		TokenParameterStyle:   openaigateway.TokenParameterMaxTokens,
	},
	{
		Name:                  "fireworks",
		APIKeyEnv:             "FIREWORKS_API_KEY",
		SemanticLayerModelEnv: "FIREWORKS_SEMANTIC_LAYER_MODEL",
		QueryModelEnv:         "FIREWORKS_QUERY_MODEL",
		PreflightModelEnv:     "FIREWORKS_PREFLIGHT_MODEL",
		DefaultBaseURL:        "https://api.fireworks.ai/inference/v1",
		Transport:             TransportOpenAICompatible,
		StructuredOutputMode:  openaigateway.StructuredOutputJSONSchema,
		TokenParameterStyle:   openaigateway.TokenParameterMaxTokens,
	},
}

func Specs() []Spec {
	out := make([]Spec, len(supportedSpecs))
	copy(out, supportedSpecs)
	return out
}

func Names() []string {
	out := make([]string, 0, len(supportedSpecs))
	for _, spec := range supportedSpecs {
		out = append(out, spec.Name)
	}
	return out
}

func ByName(name string) (Spec, bool) {
	trimmed := strings.TrimSpace(name)
	for _, spec := range supportedSpecs {
		if spec.Name == trimmed {
			return spec, true
		}
	}
	return Spec{}, false
}

func Build(
	name string,
	apiKey string,
	httpClient *http.Client,
) (llmgateway.Provider, error) {
	return BuildWithBaseURL(name, apiKey, "", httpClient)
}

func BuildWithBaseURL(
	name string,
	apiKey string,
	baseURL string,
	httpClient *http.Client,
) (llmgateway.Provider, error) {
	spec, ok := ByName(name)
	if !ok {
		return nil, fmt.Errorf("unsupported llm provider %q", name)
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = spec.DefaultBaseURL
	}

	switch spec.Transport {
	case TransportAnthropic:
		return anthropicgateway.New(anthropicgateway.Config{
			APIKey:     apiKey,
			BaseURL:    baseURL,
			HTTPClient: httpClient,
		}), nil
	case TransportOpenAICompatible:
		return openaigateway.New(openaigateway.Config{
			Name:                 spec.Name,
			APIKey:               apiKey,
			BaseURL:              baseURL,
			HTTPClient:           httpClient,
			StructuredOutputMode: spec.StructuredOutputMode,
			TokenParameterStyle:  spec.TokenParameterStyle,
		}), nil
	case TransportMistral:
		return mistralgateway.New(mistralgateway.Config{
			APIKey:     apiKey,
			BaseURL:    baseURL,
			HTTPClient: httpClient,
		}), nil
	default:
		return nil, fmt.Errorf(
			"unsupported llm transport %q for provider %q",
			spec.Transport,
			spec.Name,
		)
	}
}
