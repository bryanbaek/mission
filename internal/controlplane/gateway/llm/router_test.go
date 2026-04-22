package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

type scriptedProvider struct {
	name      string
	fn        func(context.Context, CompletionRequest) (CompletionResponse, error)
	callCount int
	calls     []CompletionRequest
}

func (p *scriptedProvider) Name() string {
	return p.name
}

func (p *scriptedProvider) Complete(
	ctx context.Context,
	req CompletionRequest,
) (CompletionResponse, error) {
	p.callCount++
	p.calls = append(p.calls, req)
	if p.fn == nil {
		return CompletionResponse{}, nil
	}
	return p.fn(ctx, req)
}

func TestRouterFallsBackToNextProviderAndResolvesProviderModels(t *testing.T) {
	t.Parallel()

	anthropic := &scriptedProvider{
		name: "anthropic",
		fn: func(context.Context, CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{}, NewTransientProviderError(
				"anthropic",
				errors.New("upstream unavailable"),
			)
		},
	}
	openai := &scriptedProvider{
		name: "openai",
		fn: func(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{
				Content: "ok",
				Model:   req.Model,
			}, nil
		},
	}

	router := NewRouter("anthropic", openai, anthropic)
	resp, err := router.Complete(context.Background(), CompletionRequest{
		Model: "generic-model",
		ProviderModels: map[string]string{
			"anthropic": "claude-sonnet-4-6",
			"openai":    "gpt-4.1",
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if anthropic.callCount != 1 {
		t.Fatalf("anthropic callCount = %d, want 1", anthropic.callCount)
	}
	if openai.callCount != 1 {
		t.Fatalf("openai callCount = %d, want 1", openai.callCount)
	}
	if anthropic.calls[0].Model != "claude-sonnet-4-6" {
		t.Fatalf("anthropic model = %q", anthropic.calls[0].Model)
	}
	if openai.calls[0].Model != "gpt-4.1" {
		t.Fatalf("openai model = %q", openai.calls[0].Model)
	}
	if resp.Provider != "openai" {
		t.Fatalf("Provider = %q, want openai", resp.Provider)
	}
	if resp.Model != "gpt-4.1" {
		t.Fatalf("Model = %q, want gpt-4.1", resp.Model)
	}
}

func TestRouterDoesNotFailOverNonTransientError(t *testing.T) {
	t.Parallel()

	anthropic := &scriptedProvider{
		name: "anthropic",
		fn: func(context.Context, CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{}, NewProviderError(
				"anthropic",
				errors.New("invalid request"),
			)
		},
	}
	openai := &scriptedProvider{name: "openai"}

	router := NewRouter("anthropic", anthropic, openai)
	_, err := router.Complete(context.Background(), CompletionRequest{
		Model: "claude-sonnet-4-6",
	})
	if err == nil {
		t.Fatal("Complete returned nil error")
	}
	if IsUnavailableError(err) {
		t.Fatalf("err = %v, want non-unavailable error", err)
	}
	if openai.callCount != 0 {
		t.Fatalf("openai callCount = %d, want 0", openai.callCount)
	}
}

func TestRouterOpensCircuitAndRecoversHalfOpen(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	anthropicShouldSucceed := false
	anthropic := &scriptedProvider{
		name: "anthropic",
		fn: func(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
			if anthropicShouldSucceed {
				return CompletionResponse{
					Content: "anthropic ok",
					Model:   req.Model,
				}, nil
			}
			return CompletionResponse{}, NewTransientProviderError(
				"anthropic",
				errors.New("temporary upstream error"),
			)
		},
	}
	openai := &scriptedProvider{
		name: "openai",
		fn: func(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{
				Content: "openai ok",
				Model:   req.Model,
			}, nil
		},
	}

	router := NewRouter("anthropic", anthropic, openai)
	router.breakers["anthropic"].now = func() time.Time { return now }

	req := CompletionRequest{
		Model: "generic-model",
		ProviderModels: map[string]string{
			"anthropic": "claude-sonnet-4-6",
			"openai":    "gpt-4.1",
		},
	}

	for attempt := 0; attempt < 3; attempt++ {
		resp, err := router.Complete(context.Background(), req)
		if err != nil {
			t.Fatalf("attempt %d returned error: %v", attempt+1, err)
		}
		if resp.Provider != "openai" {
			t.Fatalf("attempt %d provider = %q, want openai", attempt+1, resp.Provider)
		}
	}

	resp, err := router.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("fourth attempt returned error: %v", err)
	}
	if resp.Provider != "openai" {
		t.Fatalf("fourth attempt provider = %q, want openai", resp.Provider)
	}
	if anthropic.callCount != 3 {
		t.Fatalf("anthropic callCount = %d, want 3 after open circuit", anthropic.callCount)
	}

	now = now.Add(31 * time.Second)
	anthropicShouldSucceed = true

	resp, err = router.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("half-open attempt returned error: %v", err)
	}
	if resp.Provider != "anthropic" {
		t.Fatalf("half-open provider = %q, want anthropic", resp.Provider)
	}
	if anthropic.callCount != 4 {
		t.Fatalf("anthropic callCount = %d, want 4 after half-open probe", anthropic.callCount)
	}
}

func TestRouterReturnsUnavailableWhenAllProvidersAreTransientlyFailing(t *testing.T) {
	t.Parallel()

	anthropic := &scriptedProvider{
		name: "anthropic",
		fn: func(context.Context, CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{}, NewTransientProviderError(
				"anthropic",
				errors.New("temporary anthropic outage"),
			)
		},
	}
	openai := &scriptedProvider{
		name: "openai",
		fn: func(context.Context, CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{}, NewTransientProviderError(
				"openai",
				errors.New("temporary openai outage"),
			)
		},
	}

	router := NewRouter("anthropic", anthropic, openai)
	_, err := router.Complete(context.Background(), CompletionRequest{
		Model: "generic-model",
	})
	if err == nil {
		t.Fatal("Complete returned nil error")
	}

	var unavailableErr *UnavailableError
	if !errors.As(err, &unavailableErr) {
		t.Fatalf("err = %T, want *UnavailableError", err)
	}
	if len(unavailableErr.Providers) != 2 {
		t.Fatalf("providers = %#v, want two attempted providers", unavailableErr.Providers)
	}
	if unavailableErr.Providers[0] != "anthropic" || unavailableErr.Providers[1] != "openai" {
		t.Fatalf("providers = %#v, want [anthropic openai]", unavailableErr.Providers)
	}
}
