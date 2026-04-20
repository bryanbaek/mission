// Package llm defines the provider-agnostic interface the control plane uses
// to call LLMs. Concrete implementations (Anthropic, OpenAI) live in
// subpackages. A router (added in 1.2+) selects a provider per tenant.
package llm

import "context"

type CompletionRequest struct {
	System       string
	Messages     []Message
	Model        string
	MaxTokens    int
	OutputFormat *OutputFormat
	CacheControl *CacheControl
}

type Message struct {
	Role    string // "user" | "assistant"
	Content string
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
