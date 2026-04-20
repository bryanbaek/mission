package llm

import (
	"context"
	"reflect"
	"testing"
)

type fakeProvider struct {
	gotRequest CompletionRequest
	response   CompletionResponse
}

func (f *fakeProvider) Name() string {
	return "fake"
}

func (f *fakeProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	f.gotRequest = req
	return f.response, nil
}

func TestProviderRoundTrip(t *testing.T) {
	t.Parallel()

	req := CompletionRequest{
		System: "You are helpful.",
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
		Model:     "test-model",
		MaxTokens: 256,
	}
	want := CompletionResponse{
		Content:  "response",
		Provider: "fake",
		Model:    "test-model",
		Usage: Usage{
			InputTokens:  12,
			OutputTokens: 4,
		},
	}

	impl := &fakeProvider{response: want}
	var provider Provider = impl

	if provider.Name() != "fake" {
		t.Fatalf("Name = %q, want fake", provider.Name())
	}

	got, err := provider.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if !reflect.DeepEqual(impl.gotRequest, req) {
		t.Fatalf("request = %+v, want %+v", impl.gotRequest, req)
	}
	if got != want {
		t.Fatalf("response = %+v, want %+v", got, want)
	}
}
