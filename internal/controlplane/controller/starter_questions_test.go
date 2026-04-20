package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	anthropicgateway "github.com/bryanbaek/mission/internal/controlplane/gateway/llm/anthropic"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

type fakeStarterMembershipChecker struct {
	ensureFn func(context.Context, uuid.UUID, string) (model.TenantUser, error)
}

func (f fakeStarterMembershipChecker) EnsureMembership(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (model.TenantUser, error) {
	return f.ensureFn(ctx, tenantID, clerkUserID)
}

type fakeStarterLayerStore struct {
	latestApprovedByTenantFn func(context.Context, uuid.UUID) (model.TenantSemanticLayer, error)
}

func (f fakeStarterLayerStore) LatestApprovedByTenant(
	ctx context.Context,
	tenantID uuid.UUID,
) (model.TenantSemanticLayer, error) {
	return f.latestApprovedByTenantFn(ctx, tenantID)
}

type fakeStarterQuestionsStore struct {
	insertSetFn        func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, []model.StarterQuestion) error
	deactivatePriorFn  func(context.Context, uuid.UUID) error
	replaceActiveSetFn func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, []model.StarterQuestion) error
	latestActiveFn     func(context.Context, uuid.UUID) ([]model.StarterQuestion, uuid.UUID, time.Time, error)
}

func (f fakeStarterQuestionsStore) InsertSet(
	ctx context.Context,
	tenantID, semanticLayerID, setID uuid.UUID,
	questions []model.StarterQuestion,
) error {
	if f.insertSetFn != nil {
		return f.insertSetFn(ctx, tenantID, semanticLayerID, setID, questions)
	}
	return errors.New("unexpected InsertSet call")
}

func (f fakeStarterQuestionsStore) DeactivatePriorSets(
	ctx context.Context,
	tenantID uuid.UUID,
) error {
	if f.deactivatePriorFn != nil {
		return f.deactivatePriorFn(ctx, tenantID)
	}
	return errors.New("unexpected DeactivatePriorSets call")
}

func (f fakeStarterQuestionsStore) ReplaceActiveSet(
	ctx context.Context,
	tenantID, semanticLayerID, setID uuid.UUID,
	questions []model.StarterQuestion,
) error {
	if f.replaceActiveSetFn != nil {
		return f.replaceActiveSetFn(ctx, tenantID, semanticLayerID, setID, questions)
	}
	return errors.New("unexpected ReplaceActiveSet call")
}

func (f fakeStarterQuestionsStore) LatestActive(
	ctx context.Context,
	tenantID uuid.UUID,
) ([]model.StarterQuestion, uuid.UUID, time.Time, error) {
	return f.latestActiveFn(ctx, tenantID)
}

type fakeStarterCompleter struct {
	responses []llm.CompletionResponse
	errs      []error
	calls     []llm.CompletionRequest
}

func (f *fakeStarterCompleter) Name() string {
	return "fake"
}

func (f *fakeStarterCompleter) Complete(
	_ context.Context,
	req llm.CompletionRequest,
) (llm.CompletionResponse, error) {
	f.calls = append(f.calls, req)
	callIndex := len(f.calls) - 1
	if callIndex < len(f.errs) && f.errs[callIndex] != nil {
		return llm.CompletionResponse{}, f.errs[callIndex]
	}
	if callIndex >= len(f.responses) {
		return llm.CompletionResponse{}, errors.New("no fake response configured")
	}
	return f.responses[callIndex], nil
}

func starterSemanticLayerContent() model.SemanticLayerContent {
	return model.SemanticLayerContent{
		Tables: []model.SemanticTable{
			{
				TableSchema: "mission_app",
				TableName:   "customers",
				Description: "고객 마스터",
				Columns: []model.SemanticColumn{
					{TableSchema: "mission_app", TableName: "customers", ColumnName: "customer_id", Description: "고객 ID"},
					{TableSchema: "mission_app", TableName: "customers", ColumnName: "created_at", Description: "가입일"},
				},
			},
			{
				TableSchema: "mission_app",
				TableName:   "orders",
				Description: "주문",
				Columns: []model.SemanticColumn{
					{TableSchema: "mission_app", TableName: "orders", ColumnName: "order_id", Description: "주문 ID"},
					{TableSchema: "mission_app", TableName: "orders", ColumnName: "ordered_at", Description: "주문일"},
				},
			},
			{
				TableSchema: "mission_app",
				TableName:   "products",
				Description: "상품",
				Columns: []model.SemanticColumn{
					{TableSchema: "mission_app", TableName: "products", ColumnName: "product_id", Description: "상품 ID"},
					{TableSchema: "mission_app", TableName: "products", ColumnName: "name", Description: "상품명"},
				},
			},
			{
				TableSchema: "mission_app",
				TableName:   "addresses",
				Description: "주소",
				Columns: []model.SemanticColumn{
					{TableSchema: "mission_app", TableName: "addresses", ColumnName: "address_id", Description: "주소 ID"},
					{TableSchema: "mission_app", TableName: "addresses", ColumnName: "city", Description: "도시"},
				},
			},
			{
				TableSchema: "mission_app",
				TableName:   "order_items",
				Description: "주문 항목",
				Columns: []model.SemanticColumn{
					{TableSchema: "mission_app", TableName: "order_items", ColumnName: "order_id", Description: "주문 ID"},
					{TableSchema: "mission_app", TableName: "order_items", ColumnName: "quantity", Description: "수량"},
				},
			},
			{
				TableSchema: "mission_app",
				TableName:   "audit_events",
				Description: "감사 이벤트",
				Columns: []model.SemanticColumn{
					{TableSchema: "mission_app", TableName: "audit_events", ColumnName: "event_id", Description: "이벤트 ID"},
					{TableSchema: "mission_app", TableName: "audit_events", ColumnName: "created_at", Description: "발생 시각"},
				},
			},
		},
		Entities: []model.SemanticEntity{
			{Name: "고객", Description: "고객 엔터티", SourceTables: []string{"mission_app.customers"}},
		},
		CandidateMetrics: []model.CandidateMetric{
			{Name: "주문 수", Description: "전체 주문 수", SourceTables: []string{"mission_app.orders"}},
		},
	}
}

func starterLayerRecord(
	t *testing.T,
	tenantID uuid.UUID,
) model.TenantSemanticLayer {
	t.Helper()

	body, err := json.Marshal(starterSemanticLayerContent())
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	approvedAt := time.Unix(1_700_100_000, 0).UTC()
	return model.TenantSemanticLayer{
		ID:              uuid.New(),
		TenantID:        tenantID,
		SchemaVersionID: uuid.New(),
		Status:          model.SemanticLayerStatusApproved,
		Content:         body,
		CreatedAt:       approvedAt.Add(-time.Minute),
		ApprovedAt:      &approvedAt,
	}
}

func starterValidCandidates() []starterQuestionCandidate {
	return []starterQuestionCandidate{
		{Text: "이번 달 신규 고객 수는 몇 명인가요?", Category: "count", PrimaryTable: "customers"},
		{Text: "최근 6개월 주문 수 추이를 보여주세요.", Category: "trend", PrimaryTable: "orders"},
		{Text: "매출 기여가 큰 상품 상위 5개는 무엇인가요?", Category: "top_n", PrimaryTable: "products"},
		{Text: "가장 최근에 추가된 주소 10건을 보여주세요.", Category: "latest", PrimaryTable: "addresses"},
		{Text: "고객 수가 많은 도시와 적은 도시를 비교해 주세요.", Category: "comparison", PrimaryTable: "addresses"},
		{Text: "주문 항목 수량이 비정상적으로 큰 주문이 있었나요?", Category: "anomaly", PrimaryTable: "order_items"},
		{Text: "지난주 생성된 감사 이벤트 수는 몇 건인가요?", Category: "count", PrimaryTable: "audit_events"},
		{Text: "최근 3개월 고객 가입 추이를 보여주세요.", Category: "trend", PrimaryTable: "customers"},
		{Text: "최근 주문 10건을 보여주세요.", Category: "latest", PrimaryTable: "orders"},
		{Text: "주문 수가 가장 많은 고객 상위 5명은 누구인가요?", Category: "top_n", PrimaryTable: "customers"},
	}
}

func starterCompletionResponse(
	t *testing.T,
	questions []starterQuestionCandidate,
) llm.CompletionResponse {
	t.Helper()

	body, err := json.Marshal(starterQuestionsOutput{Questions: questions})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return llm.CompletionResponse{
		Content:  string(body),
		Provider: "fake",
		Model:    "claude-sonnet-4-6",
	}
}

func newStarterQuestionsController(
	membership starterQuestionsMembershipChecker,
	layers starterQuestionsLayerStore,
	questions starterQuestionsStore,
	completer llm.Provider,
) *StarterQuestionsController {
	return NewStarterQuestionsController(
		membership,
		layers,
		questions,
		completer,
		StarterQuestionsControllerConfig{
			Model:     "claude-sonnet-4-6",
			MaxTokens: 2048,
		},
	)
}

func starterTextsHash(questions []model.StarterQuestion) string {
	texts := make([]string, 0, len(questions))
	for _, question := range questions {
		texts = append(texts, question.Text)
	}
	sort.Strings(texts)
	sum := sha256.Sum256([]byte(strings.Join(texts, "\n")))
	return fmt.Sprintf("%x", sum[:])
}

func TestValidateStarterQuestionsRejectsHallucinatedTable(t *testing.T) {
	t.Parallel()

	questions := starterValidCandidates()
	questions[0].PrimaryTable = "ghost_table"

	err := validateStarterQuestions(questions, starterSemanticLayerContent())
	if err == nil || !strings.Contains(err.Error(), "primary_table") {
		t.Fatalf("err = %v, want primary_table validation error", err)
	}
}

func TestValidateStarterQuestionsRequiresDiversity(t *testing.T) {
	t.Parallel()

	questions := starterValidCandidates()
	for index := range questions {
		questions[index].Category = "count"
		if index%2 == 0 {
			questions[index].PrimaryTable = "customers"
		} else {
			questions[index].PrimaryTable = "orders"
		}
	}

	err := validateStarterQuestions(questions, starterSemanticLayerContent())
	if err == nil || !strings.Contains(err.Error(), "distinct categories") {
		t.Fatalf("err = %v, want diversity error", err)
	}
}

func TestStarterQuestionsControllerListReturnsCachedSet(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	setID := uuid.New()
	generatedAt := time.Unix(1_700_200_000, 0).UTC()
	cached := []model.StarterQuestion{
		{
			ID:           uuid.New(),
			SetID:        setID,
			TenantID:     tenantID,
			Ordinal:      1,
			Text:         "이번 달 신규 고객 수는 몇 명인가요?",
			Category:     model.StarterQuestionCategoryCount,
			PrimaryTable: "customers",
			IsActive:     true,
		},
	}

	completer := &fakeStarterCompleter{}
	ctrl := newStarterQuestionsController(
		fakeStarterMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID, Role: model.RoleMember}, nil
			},
		},
		fakeStarterLayerStore{
			latestApprovedByTenantFn: func(context.Context, uuid.UUID) (model.TenantSemanticLayer, error) {
				return model.TenantSemanticLayer{}, errors.New("should not load semantic layer")
			},
		},
		fakeStarterQuestionsStore{
			latestActiveFn: func(context.Context, uuid.UUID) ([]model.StarterQuestion, uuid.UUID, time.Time, error) {
				return cached, setID, generatedAt, nil
			},
		},
		completer,
	)

	got, err := ctrl.List(context.Background(), tenantID, "user_1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if got.SetID != setID || len(got.Questions) != 1 {
		t.Fatalf("got = %+v", got)
	}
	if len(completer.calls) != 0 {
		t.Fatalf("expected no LLM calls, got %d", len(completer.calls))
	}
}

func TestStarterQuestionsControllerListGeneratesAndPersistsWhenEmpty(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	layer := starterLayerRecord(t, tenantID)
	completer := &fakeStarterCompleter{
		responses: []llm.CompletionResponse{
			starterCompletionResponse(t, starterValidCandidates()),
		},
	}

	var persisted []model.StarterQuestion
	ctrl := newStarterQuestionsController(
		fakeStarterMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID, Role: model.RoleOwner}, nil
			},
		},
		fakeStarterLayerStore{
			latestApprovedByTenantFn: func(context.Context, uuid.UUID) (model.TenantSemanticLayer, error) {
				return layer, nil
			},
		},
		fakeStarterQuestionsStore{
			latestActiveFn: func(_ context.Context, _ uuid.UUID) ([]model.StarterQuestion, uuid.UUID, time.Time, error) {
				if len(persisted) == 0 {
					return nil, uuid.Nil, time.Time{}, repository.ErrNotFound
				}
				return persisted, persisted[0].SetID, time.Unix(1_700_200_010, 0).UTC(), nil
			},
			replaceActiveSetFn: func(_ context.Context, gotTenantID, gotSemanticLayerID, setID uuid.UUID, questions []model.StarterQuestion) error {
				if gotTenantID != tenantID {
					t.Fatalf("tenantID = %s, want %s", gotTenantID, tenantID)
				}
				if gotSemanticLayerID != layer.ID {
					t.Fatalf("semanticLayerID = %s, want %s", gotSemanticLayerID, layer.ID)
				}
				persisted = append([]model.StarterQuestion(nil), questions...)
				for index := range persisted {
					persisted[index].SetID = setID
				}
				return nil
			},
		},
		completer,
	)

	got, err := ctrl.List(context.Background(), tenantID, "user_1")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got.Questions) != 10 {
		t.Fatalf("len(got.Questions) = %d, want 10", len(got.Questions))
	}
	if got.Questions[0].Ordinal != 1 || got.Questions[9].Ordinal != 10 {
		t.Fatalf("ordinals = %d..%d", got.Questions[0].Ordinal, got.Questions[9].Ordinal)
	}
	if len(completer.calls) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(completer.calls))
	}
	req := completer.calls[0]
	if req.System != starterQuestionsSystemPrompt {
		t.Fatalf("system prompt mismatch")
	}
	if req.OutputFormat == nil || req.OutputFormat.Name != "starter_questions" {
		t.Fatalf("output format = %+v", req.OutputFormat)
	}
	if req.CacheControl == nil || req.CacheControl.TTL != "1h" {
		t.Fatalf("cache control = %+v", req.CacheControl)
	}
	if !strings.Contains(req.Messages[0].Content, "시맨틱 레이어 JSON") {
		t.Fatalf("user prompt = %q", req.Messages[0].Content)
	}
}

func TestStarterQuestionsControllerRegenerateRetriesAfterValidationFailure(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	layer := starterLayerRecord(t, tenantID)
	invalid := starterValidCandidates()
	for index := range invalid {
		invalid[index].Category = "count"
		invalid[index].PrimaryTable = "customers"
	}

	completer := &fakeStarterCompleter{
		responses: []llm.CompletionResponse{
			starterCompletionResponse(t, invalid),
			starterCompletionResponse(t, starterValidCandidates()),
		},
	}

	var persisted []model.StarterQuestion
	ctrl := newStarterQuestionsController(
		fakeStarterMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID, Role: model.RoleOwner}, nil
			},
		},
		fakeStarterLayerStore{
			latestApprovedByTenantFn: func(context.Context, uuid.UUID) (model.TenantSemanticLayer, error) {
				return layer, nil
			},
		},
		fakeStarterQuestionsStore{
			replaceActiveSetFn: func(_ context.Context, _, _, setID uuid.UUID, questions []model.StarterQuestion) error {
				persisted = append([]model.StarterQuestion(nil), questions...)
				for index := range persisted {
					persisted[index].SetID = setID
				}
				return nil
			},
			latestActiveFn: func(context.Context, uuid.UUID) ([]model.StarterQuestion, uuid.UUID, time.Time, error) {
				if len(persisted) == 0 {
					return nil, uuid.Nil, time.Time{}, repository.ErrNotFound
				}
				return persisted, persisted[0].SetID, time.Unix(1_700_200_030, 0).UTC(), nil
			},
		},
		completer,
	)

	got, err := ctrl.Regenerate(context.Background(), tenantID, "user_1")
	if err != nil {
		t.Fatalf("Regenerate returned error: %v", err)
	}
	if len(got.Questions) != 10 {
		t.Fatalf("len(got.Questions) = %d, want 10", len(got.Questions))
	}
	if len(completer.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(completer.calls))
	}
	if !strings.Contains(completer.calls[1].Messages[0].Content, "직전 출력 검증 실패") {
		t.Fatalf("retry prompt = %q", completer.calls[1].Messages[0].Content)
	}
	if !strings.Contains(completer.calls[1].Messages[0].Content, "distinct categories") {
		t.Fatalf("retry prompt missing validation feedback: %q", completer.calls[1].Messages[0].Content)
	}
}

func TestStarterQuestionsControllerRegenerateProducesDifferentSet(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	layer := starterLayerRecord(t, tenantID)
	firstSet := starterValidCandidates()
	secondSet := append([]starterQuestionCandidate(nil), starterValidCandidates()...)
	secondSet[0].Text = "최근 30일 신규 고객 수는 몇 명인가요?"
	secondSet[1].Text = "월별 주문 증가 추이를 보여주세요."
	secondSet[2].Text = "판매 수량이 많은 상품 상위 5개는 무엇인가요?"

	completer := &fakeStarterCompleter{
		responses: []llm.CompletionResponse{
			starterCompletionResponse(t, firstSet),
			starterCompletionResponse(t, secondSet),
		},
	}

	var persisted []model.StarterQuestion
	store := fakeStarterQuestionsStore{
		replaceActiveSetFn: func(_ context.Context, _, _, setID uuid.UUID, questions []model.StarterQuestion) error {
			persisted = append([]model.StarterQuestion(nil), questions...)
			for index := range persisted {
				persisted[index].SetID = setID
			}
			return nil
		},
		latestActiveFn: func(context.Context, uuid.UUID) ([]model.StarterQuestion, uuid.UUID, time.Time, error) {
			if len(persisted) == 0 {
				return nil, uuid.Nil, time.Time{}, repository.ErrNotFound
			}
			return persisted, persisted[0].SetID, time.Unix(1_700_200_050, 0).UTC(), nil
		},
	}

	ctrl := newStarterQuestionsController(
		fakeStarterMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID, Role: model.RoleOwner}, nil
			},
		},
		fakeStarterLayerStore{
			latestApprovedByTenantFn: func(context.Context, uuid.UUID) (model.TenantSemanticLayer, error) {
				return layer, nil
			},
		},
		store,
		completer,
	)

	first, err := ctrl.Regenerate(context.Background(), tenantID, "user_1")
	if err != nil {
		t.Fatalf("first Regenerate returned error: %v", err)
	}
	second, err := ctrl.Regenerate(context.Background(), tenantID, "user_1")
	if err != nil {
		t.Fatalf("second Regenerate returned error: %v", err)
	}

	firstHash := starterTextsHash(first.Questions)
	secondHash := starterTextsHash(second.Questions)
	if firstHash == secondHash {
		t.Fatalf("starter question hashes match: %s", firstHash)
	}
}

func TestStarterQuestionsGenerationIntegration(t *testing.T) {
	if os.Getenv("LLM_INTEGRATION") != "1" {
		t.Skip("set LLM_INTEGRATION=1 to run starter questions integration")
	}

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		t.Skip("set ANTHROPIC_API_KEY to run starter questions integration")
	}

	tenantID := uuid.New()
	layer := starterLayerRecord(t, tenantID)
	provider := anthropicgateway.New(anthropicgateway.Config{
		APIKey: apiKey,
	})

	var persisted []model.StarterQuestion
	ctrl := newStarterQuestionsController(
		fakeStarterMembershipChecker{
			ensureFn: func(context.Context, uuid.UUID, string) (model.TenantUser, error) {
				return model.TenantUser{TenantID: tenantID, Role: model.RoleOwner}, nil
			},
		},
		fakeStarterLayerStore{
			latestApprovedByTenantFn: func(context.Context, uuid.UUID) (model.TenantSemanticLayer, error) {
				return layer, nil
			},
		},
		fakeStarterQuestionsStore{
			replaceActiveSetFn: func(_ context.Context, _, _, setID uuid.UUID, questions []model.StarterQuestion) error {
				persisted = append([]model.StarterQuestion(nil), questions...)
				for index := range persisted {
					persisted[index].SetID = setID
				}
				return nil
			},
			latestActiveFn: func(context.Context, uuid.UUID) ([]model.StarterQuestion, uuid.UUID, time.Time, error) {
				if len(persisted) == 0 {
					return nil, uuid.Nil, time.Time{}, repository.ErrNotFound
				}
				return persisted, persisted[0].SetID, time.Unix(1_700_200_090, 0).UTC(), nil
			},
		},
		provider,
	)

	got, err := ctrl.Regenerate(context.Background(), tenantID, "user_1")
	if err != nil {
		t.Fatalf("Regenerate returned error: %v", err)
	}
	if len(got.Questions) != 10 {
		t.Fatalf("len(got.Questions) = %d, want 10", len(got.Questions))
	}

	distinctCategories := make(map[string]struct{})
	distinctTables := make(map[string]struct{})
	validTables := map[string]struct{}{
		"customers":    {},
		"orders":       {},
		"products":     {},
		"addresses":    {},
		"order_items":  {},
		"audit_events": {},
	}

	for _, question := range got.Questions {
		distinctCategories[string(question.Category)] = struct{}{}
		distinctTables[question.PrimaryTable] = struct{}{}
		if _, ok := validTables[question.PrimaryTable]; !ok {
			t.Fatalf("primary_table = %q, not in fixture table set", question.PrimaryTable)
		}
	}
	if len(distinctCategories) < 3 {
		t.Fatalf("distinctCategories = %d, want >= 3", len(distinctCategories))
	}
	if len(distinctTables) < 5 {
		t.Fatalf("distinctTables = %d, want >= 5", len(distinctTables))
	}
}
