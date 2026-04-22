package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	openaisdk "github.com/openai/openai-go/v3"
)

func TestProviderCompleteUsesSDKAndLegacyEndpointBaseURL(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("request path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization header = %q, want Bearer test-key", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := payload["model"]; got != "gpt-4.1-nano" {
			t.Fatalf("model = %#v, want gpt-4.1-nano", got)
		}
		if got := payload["max_completion_tokens"]; got != float64(64) {
			t.Fatalf("max_completion_tokens = %#v, want 64", got)
		}

		messages, ok := payload["messages"].([]any)
		if !ok {
			t.Fatalf("messages type = %T, want []any", payload["messages"])
		}
		if len(messages) != 2 {
			t.Fatalf("len(messages) = %d, want 2", len(messages))
		}
		systemMessage := messages[0].(map[string]any)
		if got := systemMessage["role"]; got != "system" {
			t.Fatalf("system role = %#v, want system", got)
		}
		if got := systemMessage["content"]; got != "system prompt" {
			t.Fatalf("system content = %#v, want system prompt", got)
		}

		responseFormat, ok := payload["response_format"].(map[string]any)
		if !ok {
			t.Fatalf("response_format type = %T, want map[string]any", payload["response_format"])
		}
		if got := responseFormat["type"]; got != "json_schema" {
			t.Fatalf("response_format.type = %#v, want json_schema", got)
		}
		jsonSchema := responseFormat["json_schema"].(map[string]any)
		if got := jsonSchema["name"]; got != "reply" {
			t.Fatalf("json_schema.name = %#v, want reply", got)
		}
		if got := jsonSchema["strict"]; got != true {
			t.Fatalf("json_schema.strict = %#v, want true", got)
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
						"content": "{\"reply\":\"pong\"}",
						"refusal": "",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     11,
				"completion_tokens": 7,
				"total_tokens":      18,
			},
		}); err != nil {
			t.Fatalf("encode response body: %v", err)
		}
	}))
	defer server.Close()

	provider := New(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL + "/v1/chat/completions",
		HTTPClient: server.Client(),
	})

	resp, err := provider.Complete(context.Background(), llm.CompletionRequest{
		System:    "system prompt",
		Model:     "gpt-4.1-nano",
		MaxTokens: 64,
		Messages: []llm.Message{
			{Role: "user", Content: "ping"},
		},
		OutputFormat: &llm.OutputFormat{
			Name:   "reply",
			Schema: map[string]any{"type": "object"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Content != "{\"reply\":\"pong\"}" {
		t.Fatalf("Content = %q, want %q", resp.Content, "{\"reply\":\"pong\"}")
	}
	if resp.Provider != "openai" {
		t.Fatalf("Provider = %q, want openai", resp.Provider)
	}
	if resp.Model != "gpt-4.1-nano" {
		t.Fatalf("Model = %q, want gpt-4.1-nano", resp.Model)
	}
	if resp.Usage.InputTokens != 11 || resp.Usage.OutputTokens != 7 {
		t.Fatalf("Usage = %+v, want prompt=11 completion=7", resp.Usage)
	}
}

func TestClassifyOpenAIErrorMarksTransientFailures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "rate limit",
			err:       &openaisdk.Error{StatusCode: http.StatusTooManyRequests},
			transient: true,
		},
		{
			name:      "server error",
			err:       &openaisdk.Error{StatusCode: http.StatusBadGateway},
			transient: true,
		},
		{
			name: "transport error",
			err: &url.Error{
				Op:  "Post",
				URL: "https://api.openai.com/v1/chat/completions",
				Err: errors.New("dial tcp timeout"),
			},
			transient: true,
		},
		{
			name:      "bad request",
			err:       &openaisdk.Error{StatusCode: http.StatusBadRequest},
			transient: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := classifyOpenAIError(tc.err)
			if got := llm.IsTransientProviderError(err); got != tc.transient {
				t.Fatalf("IsTransientProviderError(%v) = %v, want %v", tc.err, got, tc.transient)
			}
		})
	}
}

func TestProviderCompleteRejectsMissingConfig(t *testing.T) {
	t.Parallel()

	provider := New(Config{})
	_, err := provider.Complete(context.Background(), llm.CompletionRequest{
		Model: "gpt-4.1",
	})
	if err == nil {
		t.Fatal("Complete returned nil error without API key")
	}
	if llm.IsTransientProviderError(err) {
		t.Fatalf("err = %v, want non-transient config error", err)
	}

	provider = New(Config{APIKey: "test-key"})
	_, err = provider.Complete(context.Background(), llm.CompletionRequest{})
	if err == nil {
		t.Fatal("Complete returned nil error without model")
	}
	if llm.IsTransientProviderError(err) {
		t.Fatalf("err = %v, want non-transient model error", err)
	}
}
