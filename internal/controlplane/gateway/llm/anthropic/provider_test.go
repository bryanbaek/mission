package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

func TestProviderCompleteCachedContentBuildsMultiBlockUserMessage(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("messages = %#v, want one message", payload["messages"])
		}
		userMsg := messages[0].(map[string]any)
		content, ok := userMsg["content"].([]any)
		if !ok || len(content) != 2 {
			t.Fatalf("user message content = %#v, want two blocks", userMsg["content"])
		}
		cachedBlock := content[0].(map[string]any)
		if got := cachedBlock["text"]; got != "schema context" {
			t.Fatalf("cached block text = %#v, want schema context", got)
		}
		blockCC, ok := cachedBlock["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("cached block cache_control = %#v, want map", cachedBlock["cache_control"])
		}
		if got := blockCC["type"]; got != "ephemeral" {
			t.Fatalf("cached block cache_control.type = %#v, want ephemeral", got)
		}
		dynamicBlock := content[1].(map[string]any)
		if got := dynamicBlock["text"]; got != "user question" {
			t.Fatalf("dynamic block text = %#v, want user question", got)
		}
		if _, hasCC := dynamicBlock["cache_control"]; hasCC {
			t.Fatal("dynamic block must not have cache_control")
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_456", "type": "message", "role": "assistant",
			"model": "claude-sonnet-4-6",
			"content": []map[string]any{{"type": "text", "text": "ok"}},
			"stop_reason": "end_turn", "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 1,
				"cache_creation_input_tokens": 0, "cache_read_input_tokens": 0},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := New(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL + "/v1/messages",
		HTTPClient: server.Client(),
	})

	_, err := provider.Complete(context.Background(), llm.CompletionRequest{
		Model:     "claude-sonnet-4-6",
		MaxTokens: 32,
		Messages: []llm.Message{
			{Role: "user", CachedContent: "schema context", Content: "user question"},
		},
		CacheControl: &llm.CacheControl{Type: "ephemeral", TTL: "5m"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
}

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
		// cache_control must be on the system block, not at the request level.
		if _, hasTopLevel := payload["cache_control"]; hasTopLevel {
			t.Fatal("cache_control must not be set at the request level")
		}
		sysCacheControl, ok := systemBlock["cache_control"].(map[string]any)
		if !ok {
			t.Fatalf("system block cache_control type = %T, want map[string]any", systemBlock["cache_control"])
		}
		if got := sysCacheControl["type"]; got != "ephemeral" {
			t.Fatalf("system cache_control.type = %#v, want ephemeral", got)
		}
		if got := sysCacheControl["ttl"]; got != "1h" {
			t.Fatalf("system cache_control.ttl = %#v, want 1h", got)
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
