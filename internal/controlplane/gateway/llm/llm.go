// Package llm defines the provider-agnostic interface the control plane uses
// to call LLMs. Concrete implementations (Anthropic, OpenAI) live in
// subpackages. A router (added in 1.2+) selects a provider per tenant.
package llm

import "context"

type CompletionRequest struct {
	System   string
	Messages []Message
	Model    string
	MaxTokens int
}

type Message struct {
	Role    string // "user" | "assistant"
	Content string
}

type CompletionResponse struct {
	Content string
	Model   string
}

type Provider interface {
	Name() string
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
