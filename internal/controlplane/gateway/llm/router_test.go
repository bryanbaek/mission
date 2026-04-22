package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bryanbaek/mission/internal/controlplane/reqlog"
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

func TestRouterLogsOperationOnCompleteAndFailover(t *testing.T) {
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

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	ctx := reqlog.WithLogger(context.Background(), logger)

	router := NewRouter("anthropic", anthropic, openai)
	_, err := router.Complete(ctx, CompletionRequest{
		Operation: "query.generate_sql",
		Model:     "generic-model",
		ProviderModels: map[string]string{
			"anthropic": "claude-sonnet-4-6",
			"openai":    "gpt-4.1",
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	entries := decodeLogEntries(t, logs.String())
	for _, msg := range []string{"llm.complete.failed", "llm.failover", "llm.complete"} {
		entry := findLogEntry(t, entries, msg)
		if got := entry["operation"]; got != "query.generate_sql" {
			t.Fatalf("%s operation = %#v, want query.generate_sql", msg, got)
		}
	}
}

func decodeLogEntries(t *testing.T, raw string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("json.Unmarshal(%q) returned error: %v", line, err)
		}
		out = append(out, entry)
	}
	return out
}

func findLogEntry(t *testing.T, entries []map[string]any, msg string) map[string]any {
	t.Helper()

	for _, entry := range entries {
		if entry["msg"] == msg {
			return entry
		}
	}
	t.Fatalf("log entry %q not found in %#v", msg, entries)
	return nil
}
