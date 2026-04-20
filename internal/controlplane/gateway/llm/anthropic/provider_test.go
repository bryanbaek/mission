package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

func TestProviderCompleteUsesSDKAndLegacyEndpointBaseURL(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("request path = %q, want /v1/messages", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Fatalf("X-API-Key header = %q, want test-key", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("anthropic-version header was empty")
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := payload["model"]; got != "claude-sonnet-4-6" {
			t.Fatalf("model = %#v, want claude-sonnet-4-6", got)
		}
		if got := payload["max_tokens"]; got != float64(32) {
			t.Fatalf("max_tokens = %#v, want 32", got)
		}

		system, ok := payload["system"].([]any)
		if !ok || len(system) != 1 {
			t.Fatalf("system = %#v, want one text block", payload["system"])
		}
		systemBlock := system[0].(map[string]any)
		if got := systemBlock["text"]; got != "system prompt" {
			t.Fatalf("system text = %#v, want system prompt", got)
		}

		cacheControl, ok := payload["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("cache_control type = %T, want map[string]any", payload["cache_control"])
		}
		if got := cacheControl["type"]; got != "ephemeral" {
			t.Fatalf("cache_control.type = %#v, want ephemeral", got)
		}
		if got := cacheControl["ttl"]; got != "1h" {
			t.Fatalf("cache_control.ttl = %#v, want 1h", got)
		}

		outputConfig, ok := payload["output_config"].(map[string]any)
		if !ok {
			t.Fatalf("output_config type = %T, want map[string]any", payload["output_config"])
		}
		format := outputConfig["format"].(map[string]any)
		if got := format["type"]; got != "json_schema" {
			t.Fatalf("output_config.format.type = %#v, want json_schema", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-sonnet-4-6",
			"content": []map[string]any{
				{"type": "text", "text": "pong"},
				{"type": "text", "text": "!"},
			},
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":                13,
				"output_tokens":               4,
				"cache_creation_input_tokens": 2,
				"cache_read_input_tokens":     5,
			},
		}); err != nil {
			t.Fatalf("encode response body: %v", err)
		}
	}))
	defer server.Close()

	provider := New(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL + "/v1/messages",
		HTTPClient: server.Client(),
	})

	resp, err := provider.Complete(context.Background(), llm.CompletionRequest{
		System:    "system prompt",
		Model:     "claude-sonnet-4-6",
		MaxTokens: 32,
		Messages: []llm.Message{
			{Role: "user", Content: "ping"},
		},
		CacheControl: &llm.CacheControl{
			Type: "ephemeral",
			TTL:  "1h",
		},
		OutputFormat: &llm.OutputFormat{
			Name:   "reply",
			Schema: map[string]any{"type": "object"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Content != "pong!" {
		t.Fatalf("Content = %q, want pong!", resp.Content)
	}
	if resp.Provider != "anthropic" {
		t.Fatalf("Provider = %q, want anthropic", resp.Provider)
	}
	if resp.Model != "claude-sonnet-4-6" {
		t.Fatalf("Model = %q, want claude-sonnet-4-6", resp.Model)
	}
	if resp.Usage.InputTokens != 13 || resp.Usage.OutputTokens != 4 {
		t.Fatalf("Usage = %+v, want input=13 output=4", resp.Usage)
	}
	if resp.Usage.CacheCreationInputTokens != 2 || resp.Usage.CacheReadInputTokens != 5 {
		t.Fatalf("Usage cache = %+v, want create=2 read=5", resp.Usage)
	}
}
