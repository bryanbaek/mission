package controller

import (
	"encoding/json"
	"strings"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

const starterQuestionsSystemPromptKorean = `
당신은 한국 SMB의 데이터 분석가입니다. 주어진 시맨틱 레이어(테이블/컬럼/비즈니스 의미)만 사용해
사용자가 첫 화면에서 바로 클릭할 만한 유용한 질문 10개를 만듭니다.

규칙:
- 질문은 한국어로 작성하고 각 질문은 80자 이내로 간결해야 합니다.
- 시맨틱 레이어에 존재하는 테이블과 컬럼만 사용하세요. 존재하지 않는 테이블은 절대 만들지 마세요.
- 10개 질문은 서로 다른 테이블을 가능한 한 많이 다뤄야 하며(가용 테이블이 5개 이상이면 최소 5개),
  집계 형태도 다양해야 합니다: 최소 3가지 이상을 포함.
- 집계 형태(category) 허용값: count, trend, top_n, latest, comparison, anomaly.
- 각 질문마다 주로 다루는 테이블을 primary_table로 표기합니다.

응답은 반드시 JSON 스키마에 맞추어 반환하세요.
`

const starterQuestionsSystemPromptEnglish = `
You are a data analyst for a small-to-medium business. Using only the provided semantic layer
(tables, columns, business meanings), produce 10 useful questions a user can click on the landing screen.

Rules:
- Write the questions in English and keep each under 80 characters.
- Only use tables and columns that exist in the semantic layer. Never invent tables.
- The 10 questions should cover as many different tables as possible (at least 5 if 5+ tables are available),
  and the aggregation style should vary — include at least 3 different categories.
- Allowed category values: count, trend, top_n, latest, comparison, anomaly.
- For each question, set primary_table to the table it mainly reads from.

Respond strictly as structured JSON per the provided schema.
`

func starterQuestionsSystemPrompt(locale model.Locale) string {
	if locale == model.LocaleEnglish {
		return starterQuestionsSystemPromptEnglish
	}
	return starterQuestionsSystemPromptKorean
}

func buildStarterQuestionsUserPrompt(
	layer model.SemanticLayerContent,
	locale model.Locale,
) string {
	body, _ := json.Marshal(layer)
	header := strings.TrimSpace(`
다음은 승인된 시맨틱 레이어 JSON입니다.
이 JSON에 들어 있는 테이블, 컬럼, 엔터티, 지표만 근거로 시작 질문을 만드세요.

추가 규칙:
- 질문은 클릭 즉시 실행될 수 있을 만큼 구체적이어야 합니다.
- primary_table은 반드시 tables[].table_name 중 하나를 그대로 사용하세요.
- 설명문, 마크다운, 코드 블록 없이 구조화된 데이터만 반환하세요.

시맨틱 레이어 JSON:
`)
	if locale == model.LocaleEnglish {
		header = strings.TrimSpace(`
Below is the approved semantic layer as JSON.
Generate the starter questions using only the tables, columns, entities, and metrics found in this JSON.

Additional rules:
- Each question must be concrete enough that it can be executed immediately on click.
- primary_table must exactly match one of tables[].table_name.
- Return only the structured data — no prose, markdown, or code blocks.

Semantic layer JSON:
`)
	}
	return header + "\n" + string(body)
}

func buildStarterQuestionsRetryPrompt(
	validationFeedback string,
	locale model.Locale,
) string {
	validationFeedback = strings.TrimSpace(validationFeedback)
	if validationFeedback == "" {
		return ""
	}
	if locale == model.LocaleEnglish {
		return "\n\nPrevious output failed validation:\n" + validationFeedback +
			"\nRewrite all 10 questions addressing the errors above."
	}
	return "\n\n직전 출력 검증 실패:\n" + validationFeedback + "\n위 오류를 반영해 전체 10개를 다시 작성하세요."
}

func starterQuestionsRetryDecodeFeedback(locale model.Locale, err error) string {
	if locale == model.LocaleEnglish {
		return "Failed to decode response JSON: " + err.Error()
	}
	return "응답 JSON을 디코드할 수 없습니다: " + err.Error()
}

func starterQuestionsOutputSchema() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"questions": map[string]any{
				"type":     "array",
				"minItems": 10,
				"maxItems": 10,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text":          map[string]any{"type": "string", "maxLength": 80},
						"category":      map[string]any{"type": "string", "enum": starterQuestionCategoryValues()},
						"primary_table": stringSchema,
					},
					"required":             []string{"text", "category", "primary_table"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"questions"},
		"additionalProperties": false,
	}
}
