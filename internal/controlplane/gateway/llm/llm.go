// Package llm defines the provider-agnostic interface the control plane uses
// to call LLMs. Concrete implementations (Anthropic, OpenAI) live in
// subpackages. A router (added in 1.2+) selects a provider per tenant.
package llm

import (
	"context"
	"strings"
)

const UserUnavailableMessage = "language model service is temporarily unavailable; please try again shortly"

type CompletionRequest struct {
	System         string
	Messages       []Message
	Model          string
	ProviderModels map[string]string
	MaxTokens      int
	OutputFormat   *OutputFormat
	CacheControl   *CacheControl
}

type Message struct {
	Role          string // "user" | "assistant"
	Content       string // dynamic part (question, retry info)
	CachedContent string // static part (schema, semantic layer, examples); Anthropic only
}

type OutputFormat struct {
	Name   string
	Schema map[string]any
}

type CacheControl struct {
	Type string
	TTL  string
}

type Usage struct {
	Provider                 string
	Model                    string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

type CompletionResponse struct {
	Content  string
	Provider string
	Model    string
	Usage    Usage
}

type Provider interface {
	Name() string
	Complete(
		ctx context.Context,
		req CompletionRequest,
	) (CompletionResponse, error)
}

func (r CompletionRequest) ModelForProvider(providerName string) string {
	if override := strings.TrimSpace(r.ProviderModels[providerName]); override != "" {
		return override
	}
	return strings.TrimSpace(r.Model)
}

func CloneProviderModels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if trimmedValue := strings.TrimSpace(value); trimmedValue != "" {
			out[trimmedKey] = trimmedValue
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
