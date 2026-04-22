package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunLLMPingOpenAICompatibleProvider(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("request path = %q, want /v1/chat/completions", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl_123",
			"object":  "chat.completion",
			"created": 1,
			"model":   "gpt-4.1-nano",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"logprobs":      nil,
					"message": map[string]any{
						"role":    "assistant",
						"content": "pong",
						"refusal": "",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		}); err != nil {
			t.Fatalf("encode response body: %v", err)
		}
	}))
	defer server.Close()

	err := run([]string{
		"llm-ping",
		"--provider", "openai",
		"--api-key", "test-key",
		"--model", "gpt-4.1-nano",
		"--base-url", server.URL + "/v1/chat/completions",
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRunLLMPingMistralProvider(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("request path = %q, want /v1/chat/completions", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"model": "mistral-small-latest",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "pong",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
			},
		}); err != nil {
			t.Fatalf("encode response body: %v", err)
		}
	}))
	defer server.Close()

	err := run([]string{
		"llm-ping",
		"--provider", "mistral",
		"--api-key", "test-key",
		"--model", "mistral-small-latest",
		"--base-url", server.URL,
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

func TestRunLLMPingRejectsUnknownProvider(t *testing.T) {
	t.Helper()

	err := run([]string{
		"llm-ping",
		"--provider", "unknown",
		"--api-key", "test-key",
		"--model", "test-model",
	})
	if err == nil {
		t.Fatal("run returned nil error for unknown provider")
	}
}
