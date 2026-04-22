package mistral

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

func TestProviderCompleteUsesOfficialChatCompletionsAPI(t *testing.T) {
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
		if got := payload["model"]; got != "mistral-small-latest" {
			t.Fatalf("model = %#v, want mistral-small-latest", got)
		}
		if got := payload["max_tokens"]; got != float64(48) {
			t.Fatalf("max_tokens = %#v, want 48", got)
		}

		messages := payload["messages"].([]any)
		userMessage := messages[1].(map[string]any)
		if got := userMessage["content"]; got != "schema context\n\nquestion text" {
			t.Fatalf("user content = %#v, want merged cached+dynamic content", got)
		}

		responseFormat := payload["response_format"].(map[string]any)
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
			"model": "mistral-small-latest",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": []map[string]any{
							{
								"type": "text",
								"text": "{\"reply\":\"pong\"}",
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     12,
				"completion_tokens": 6,
			},
		}); err != nil {
			t.Fatalf("encode response body: %v", err)
		}
	}))
	defer server.Close()

	provider := New(Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	resp, err := provider.Complete(context.Background(), llm.CompletionRequest{
		System:    "system prompt",
		Model:     "mistral-small-latest",
		MaxTokens: 48,
		Messages: []llm.Message{
			{Role: "user", CachedContent: "schema context", Content: "question text"},
		},
		OutputFormat: &llm.OutputFormat{
			Name:   "reply",
			Schema: map[string]any{"type": "object"},
		},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Provider != "mistral" {
		t.Fatalf("Provider = %q, want mistral", resp.Provider)
	}
	if resp.Content != "{\"reply\":\"pong\"}" {
		t.Fatalf("Content = %q, want JSON response", resp.Content)
	}
	if resp.Usage.InputTokens != 12 || resp.Usage.OutputTokens != 6 {
		t.Fatalf("Usage = %+v, want prompt=12 completion=6", resp.Usage)
	}
}

func TestClassifyMistralErrorMarksTransientFailures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "rate limit",
			err:       classifyMistralHTTPError(http.StatusTooManyRequests),
			transient: true,
		},
		{
			name:      "server error",
			err:       classifyMistralHTTPError(http.StatusBadGateway),
			transient: true,
		},
		{
			name: "transport error",
			err: &url.Error{
				Op:  "Post",
				URL: "https://api.mistral.ai/v1/chat/completions",
				Err: errors.New("dial tcp timeout"),
			},
			transient: true,
		},
		{
			name:      "bad request",
			err:       classifyMistralHTTPError(http.StatusBadRequest),
			transient: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.err
			if tc.err != nil && tc.err == classifyMistralHTTPError(http.StatusBadGateway) {
				err = tc.err
			}
			if _, ok := tc.err.(*url.Error); ok {
				err = classifyMistralError(tc.err)
			}
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
		Model: "mistral-small-latest",
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
