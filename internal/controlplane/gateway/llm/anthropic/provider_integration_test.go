package anthropic

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
)

func TestProviderPromptCachingIntegration(t *testing.T) {
	if os.Getenv("MISSION_TEST_RUN_ANTHROPIC_CACHE") != "1" {
		t.Skip("set MISSION_TEST_RUN_ANTHROPIC_CACHE=1 to run Anthropic cache integration test")
	}

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		t.Skip("set ANTHROPIC_API_KEY to run Anthropic cache integration test")
	}

	modelName := strings.TrimSpace(os.Getenv("MISSION_TEST_ANTHROPIC_MODEL"))
	if modelName == "" {
		modelName = "claude-sonnet-4-6"
	}

	provider := New(Config{
		APIKey: apiKey,
	})

	req := llm.CompletionRequest{
		System:    strings.Repeat("당신은 한국어로 일하는 보수적인 데이터 분석가입니다. ", 160),
		Model:     modelName,
		MaxTokens: 128,
		Messages:  []llm.Message{{Role: "user", Content: largeCacheProbePrompt()}},
		CacheControl: &llm.CacheControl{
			Type: "ephemeral",
			TTL:  "1h",
		},
		OutputFormat: &llm.OutputFormat{
			Name: "cache_probe",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary": map[string]any{"type": "string"},
				},
				"required":             []string{"summary"},
				"additionalProperties": false,
			},
		},
	}

	first, err := provider.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("first Complete returned error: %v", err)
	}
	if first.Provider != "anthropic" {
		t.Fatalf("first provider = %q, want anthropic", first.Provider)
	}

	second, err := provider.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("second Complete returned error: %v", err)
	}
	if second.Usage.CacheReadInputTokens <= 0 {
		t.Fatalf(
			"CacheReadInputTokens = %d, want > 0 (first usage: %+v, second usage: %+v)",
			second.Usage.CacheReadInputTokens,
			first.Usage,
			second.Usage,
		)
	}
}

func largeCacheProbePrompt() string {
	var builder strings.Builder
	builder.WriteString("다음은 반복된 스키마 메타데이터입니다. 구조만 보고 한 줄 요약을 작성하세요.\n")
	for i := 0; i < 180; i++ {
		builder.WriteString(`{"table_schema":"mission_app","table_name":"orders","column_name":"order_total","column_type":"decimal(12,2)","column_comment":"Total order amount"}` + "\n")
	}
	return builder.String()
}
