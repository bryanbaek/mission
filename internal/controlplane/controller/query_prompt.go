package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
)

type generatedSQL struct {
	Reasoning string `json:"reasoning"`
	SQL       string `json:"sql"`
	Notes     string `json:"notes"`
}

func (c *QueryController) generateSQL(
	ctx context.Context,
	question string,
	promptCtx queryPromptContext,
	examples []model.TenantCanonicalQueryExample,
	priorSQL, priorError string,
) (generatedSQL, error) {
	cached, dynamic := buildQueryUserPrompt(
		question,
		promptCtx,
		examples,
		priorSQL,
		priorError,
	)

	resp, err := c.completer.Complete(ctx, llm.CompletionRequest{
		Operation: "query.generate_sql",
		System:    querySystemPrompt,
		Messages: []llm.Message{{
			Role:          "user",
			CachedContent: cached,
			Content:       dynamic,
		}},
		Model:          c.model,
		ProviderModels: llm.CloneProviderModels(c.providerModels),
		MaxTokens:      c.maxTokens,
		OutputFormat: &llm.OutputFormat{
			Name:   "text_to_sql",
			Schema: querySQLOutputSchema(),
		},
		CacheControl: &llm.CacheControl{
			Type: "ephemeral",
			TTL:  "1h",
		},
	})
	if err != nil {
		return generatedSQL{}, err
	}

	var out generatedSQL
	if err := json.Unmarshal([]byte(resp.Content), &out); err != nil {
		return generatedSQL{}, fmt.Errorf(
			"decode generated sql: %w (raw=%q)",
			err,
			resp.Content,
		)
	}
	out.SQL = strings.TrimSpace(out.SQL)
	if out.SQL == "" {
		return generatedSQL{}, errors.New("LLM returned empty SQL")
	}
	return out, nil
}

func (c *QueryController) summarize(
	ctx context.Context,
	question, executedSQL string,
	execResult AgentExecuteQueryResult,
) (string, error) {
	userPrompt := buildSummaryUserPrompt(
		question,
		executedSQL,
		execResult,
		c.maxSummaryRows,
	)

	resp, err := c.completer.Complete(ctx, llm.CompletionRequest{
		Operation: "query.summarize",
		System:    querySummarySystemPrompt,
		Messages: []llm.Message{{
			Role:    "user",
			Content: userPrompt,
		}},
		Model:          c.summaryModel,
		ProviderModels: llm.CloneProviderModels(c.summaryProviderModels),
		MaxTokens:      c.summaryMaxTokens,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

// buildQueryUserPrompt splits the LLM user prompt into two parts for
// Anthropic prompt caching: cachedContent (semi-static schema context) and
// dynamicContent (per-query question and retry info).
func buildQueryUserPrompt(
	question string,
	promptCtx queryPromptContext,
	examples []model.TenantCanonicalQueryExample,
	priorSQL, priorError string,
) (cachedContent, dynamicContent string) {
	var cached strings.Builder

	if len(examples) > 0 {
		cached.WriteString("## 승인된 예시 쿼리\n")
		for index, example := range examples {
			fmt.Fprintf(&cached, "### 예시 %d\n", index+1)
			cached.WriteString("질문: ")
			cached.WriteString(example.Question)
			cached.WriteString("\n")
			if strings.TrimSpace(example.Notes) != "" {
				cached.WriteString("노트: ")
				cached.WriteString(example.Notes)
				cached.WriteString("\n")
			}
			cached.WriteString("SQL:\n```sql\n")
			cached.WriteString(example.SQL)
			cached.WriteString("\n```\n\n")
		}
	}

	if promptCtx.semanticLayer != nil {
		cached.WriteString("## 시맨틱 레이어 (")
		cached.WriteString(string(promptCtx.source))
		cached.WriteString(")\n")
		payload, err := json.MarshalIndent(promptCtx.semanticLayer, "", "  ")
		if err == nil {
			cached.Write(payload)
		}
		cached.WriteString("\n\n")
	}

	cached.WriteString("## 원본 MySQL 스키마\n")
	cached.Write(promptCtx.schemaRaw)
	cached.WriteString("\n\n")

	cached.WriteString(strings.TrimSpace(`
## 지시 사항
- 위 스키마, 시맨틱 레이어, 승인된 예시 쿼리만 근거로 SQL을 작성합니다.
- 읽기 전용 SELECT (또는 WITH / SHOW)만 사용합니다.
- 존재하지 않는 테이블이나 컬럼은 절대 참조하지 않습니다.
- 승인된 예시 쿼리의 패턴은 재사용할 수 있지만, 현재 질문에 맞게 필요한 부분만 조정합니다.
- LIMIT이 필요하면 명시적으로 선언하지만, 누락해도 시스템이 자동으로 LIMIT 1000을 붙입니다.
- reasoning 필드에는 어떤 테이블/컬럼을 사용했고 왜 그렇게 조인했는지 한국어로 간단히 기록합니다.
- sql 필드에는 실행 가능한 단일 MySQL 문만 담습니다. 주석이나 세미콜론은 포함하지 않습니다.
- notes 필드에는 추정한 부분이나 사용자가 알아야 할 주의 사항을 한국어로 적습니다.
`))

	var dynamic strings.Builder

	dynamic.WriteString("## 사용자 질문\n")
	dynamic.WriteString(question)
	dynamic.WriteString("\n\n")

	if priorSQL != "" {
		dynamic.WriteString("## 이전 시도 실패\n")
		dynamic.WriteString("아래 SQL을 생성했지만 검증 또는 실행에 실패했습니다. ")
		dynamic.WriteString("원인을 고려하여 수정된 SQL을 다시 만들어 주세요.\n\n")
		dynamic.WriteString("이전 SQL:\n```sql\n")
		dynamic.WriteString(priorSQL)
		dynamic.WriteString("\n```\n\n실패 사유:\n")
		dynamic.WriteString(priorError)
		dynamic.WriteString("\n\n")
	}

	return cached.String(), dynamic.String()
}

func buildSummaryUserPrompt(
	question, executedSQL string,
	execResult AgentExecuteQueryResult,
	maxRows int,
) string {
	var builder strings.Builder

	builder.WriteString("## 원본 질문\n")
	builder.WriteString(question)
	builder.WriteString("\n\n")

	builder.WriteString("## 실행한 SQL\n```sql\n")
	builder.WriteString(executedSQL)
	builder.WriteString("\n```\n\n")

	builder.WriteString("## 결과 메타데이터\n")
	fmt.Fprintf(&builder, "- 컬럼: %s\n", strings.Join(execResult.Columns, ", "))
	fmt.Fprintf(&builder, "- 행 수: %d\n", len(execResult.Rows))
	fmt.Fprintf(&builder, "- 실행 시간(ms): %d\n", execResult.ElapsedMS)
	builder.WriteString("\n")

	truncated := execResult.Rows
	if len(truncated) > maxRows {
		truncated = truncated[:maxRows]
	}
	builder.WriteString("## 결과 데이터 (JSON)\n")
	payload, err := json.MarshalIndent(truncated, "", "  ")
	if err != nil {
		payload = []byte("[]")
	}
	builder.Write(payload)
	builder.WriteString("\n")
	if len(execResult.Rows) > maxRows {
		fmt.Fprintf(
			&builder,
			"\n(전체 %d행 중 상위 %d행만 표시)\n",
			len(execResult.Rows),
			maxRows,
		)
	}

	builder.WriteString("\n## 지시 사항\n")
	builder.WriteString(strings.TrimSpace(`
- 결과를 바탕으로 질문에 대한 답을 한국어로 자연스럽게 요약합니다.
- 첫 문장은 핵심 답이어야 합니다.
- 구체적인 숫자/값은 결과에서 인용합니다.
- 데이터에 없는 내용은 추측하지 않습니다. 불확실하면 "데이터 기준"이라고 명시합니다.
- 3~5문장 이내로 작성합니다. 목록이나 마크다운은 사용하지 않습니다.
`))

	return builder.String()
}

const querySystemPrompt = `
당신은 한국어로 일하는 신중한 MySQL 분석가입니다.
사용자의 자연어 질문을 주어진 스키마와 시맨틱 레이어만 근거로 읽기 전용 MySQL SELECT 문으로 변환합니다.

반드시 지킬 규칙:
- 존재하지 않는 테이블/컬럼을 만들어내지 않습니다. 확신이 없으면 notes에 적어 두세요.
- DELETE, UPDATE, INSERT, REPLACE, CREATE, DROP, ALTER, TRUNCATE, GRANT, REVOKE, LOCK, SET, CALL, LOAD 등은 절대 생성하지 않습니다.
- SELECT ... INTO OUTFILE, INTO DUMPFILE, 공유 외 FOR UPDATE 잠금은 허용되지 않습니다.
- 세미콜론으로 여러 문장을 이어 붙이지 않습니다. 하나의 문장만 반환합니다.
- 주석(-- , /* */, #)은 생성하지 않습니다.
- 응답은 반드시 주어진 JSON 스키마를 따르며 reasoning, sql, notes 세 필드를 모두 포함합니다.
`

const querySummarySystemPrompt = `
당신은 한국어로 일하는 데이터 분석 요약가입니다.
쿼리 결과를 보고 사용자에게 명확하고 짧은 한국어 답변을 제공합니다.
데이터에 없는 사실은 지어내지 않습니다. 추측이 섞이면 "데이터 기준"이라고 명시합니다.
`

func querySQLOutputSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reasoning": stringSchema,
			"sql":       stringSchema,
			"notes":     stringSchema,
		},
		"required":             []string{"reasoning", "sql", "notes"},
		"additionalProperties": false,
	}
}
