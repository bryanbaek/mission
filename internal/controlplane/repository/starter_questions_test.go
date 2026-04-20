package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bryanbaek/mission/internal/controlplane/model"
)

func TestStarterQuestionsRepositoryInsertSetPersistsQuestions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	semanticLayerID := uuid.New()
	setID := uuid.New()
	questions := []model.StarterQuestion{
		{
			ID:           uuid.New(),
			Ordinal:      1,
			Text:         "고객 수는 얼마나 되나요?",
			Category:     model.StarterQuestionCategoryCount,
			PrimaryTable: "customers",
			IsActive:     true,
		},
		{
			ID:           uuid.New(),
			Ordinal:      2,
			Text:         "최근 주문 10건을 보여주세요.",
			Category:     model.StarterQuestionCategoryLatest,
			PrimaryTable: "orders",
			IsActive:     true,
		},
	}

	execCalls := 0
	tx := &fakeTx{
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if !strings.Contains(sql, "INSERT INTO tenant_starter_questions") {
				t.Fatalf("unexpected SQL: %q", sql)
			}
			if len(args) != 9 {
				t.Fatalf("unexpected arg count: %d", len(args))
			}
			if args[1] != tenantID || args[2] != semanticLayerID || args[3] != setID {
				t.Fatalf("unexpected ids: %#v", args[1:4])
			}
			execCalls++
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	repo := &StarterQuestionsRepository{
		db: &fakeTenantDB{
			beginFn: func(context.Context) (pgx.Tx, error) {
				return tx, nil
			},
		},
	}

	if err := repo.InsertSet(
		context.Background(),
		tenantID,
		semanticLayerID,
		setID,
		questions,
	); err != nil {
		t.Fatalf("InsertSet returned error: %v", err)
	}
	if execCalls != len(questions) {
		t.Fatalf("execCalls = %d, want %d", execCalls, len(questions))
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
}

func TestStarterQuestionsRepositoryReplaceActiveSetDeactivatesAndInserts(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	semanticLayerID := uuid.New()
	setID := uuid.New()
	question := model.StarterQuestion{
		ID:           uuid.New(),
		Ordinal:      1,
		Text:         "상품별 매출 상위 5개는 무엇인가요?",
		Category:     model.StarterQuestionCategoryTopN,
		PrimaryTable: "products",
		IsActive:     true,
	}

	callCount := 0
	tx := &fakeTx{
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			callCount++
			switch callCount {
			case 1:
				if !strings.Contains(sql, "SET is_active = FALSE") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || args[0] != tenantID {
					t.Fatalf("unexpected deactivate args: %#v", args)
				}
			case 2:
				if !strings.Contains(sql, "INSERT INTO tenant_starter_questions") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if args[1] != tenantID || args[2] != semanticLayerID || args[3] != setID {
					t.Fatalf("unexpected insert ids: %#v", args[1:4])
				}
			default:
				t.Fatalf("unexpected exec call %d", callCount)
			}
			return pgconn.NewCommandTag("OK"), nil
		},
	}

	repo := &StarterQuestionsRepository{
		db: &fakeTenantDB{
			beginFn: func(context.Context) (pgx.Tx, error) {
				return tx, nil
			},
		},
	}

	if err := repo.ReplaceActiveSet(
		context.Background(),
		tenantID,
		semanticLayerID,
		setID,
		[]model.StarterQuestion{question},
	); err != nil {
		t.Fatalf("ReplaceActiveSet returned error: %v", err)
	}
	if !tx.committed {
		t.Fatal("transaction was not committed")
	}
}

func TestStarterQuestionsRepositoryLatestActiveReturnsLatestSetInOrdinalOrder(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	setID := uuid.New()
	semanticLayerID := uuid.New()
	generatedAt := time.Unix(1_700_100_000, 0).UTC()

	repo := &StarterQuestionsRepository{
		db: &fakeTenantDB{
			queryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
				if !strings.Contains(sql, "WITH latest_set AS") {
					t.Fatalf("unexpected SQL: %q", sql)
				}
				if len(args) != 1 || args[0] != tenantID {
					t.Fatalf("unexpected args: %#v", args)
				}
				return &fakeRows{
					scans: []func(dest ...any) error{
						func(dest ...any) error {
							*(dest[0].(*uuid.UUID)) = uuid.New()
							*(dest[1].(*uuid.UUID)) = setID
							*(dest[2].(*uuid.UUID)) = tenantID
							*(dest[3].(*uuid.UUID)) = semanticLayerID
							*(dest[4].(*int32)) = 1
							*(dest[5].(*string)) = "고객 수는 얼마나 되나요?"
							*(dest[6].(*model.StarterQuestionCategory)) = model.StarterQuestionCategoryCount
							*(dest[7].(*string)) = "customers"
							*(dest[8].(*time.Time)) = generatedAt
							*(dest[9].(*bool)) = true
							return nil
						},
						func(dest ...any) error {
							*(dest[0].(*uuid.UUID)) = uuid.New()
							*(dest[1].(*uuid.UUID)) = setID
							*(dest[2].(*uuid.UUID)) = tenantID
							*(dest[3].(*uuid.UUID)) = semanticLayerID
							*(dest[4].(*int32)) = 2
							*(dest[5].(*string)) = "최근 주문 10건을 보여주세요."
							*(dest[6].(*model.StarterQuestionCategory)) = model.StarterQuestionCategoryLatest
							*(dest[7].(*string)) = "orders"
							*(dest[8].(*time.Time)) = generatedAt
							*(dest[9].(*bool)) = true
							return nil
						},
					},
				}, nil
			},
		},
	}

	questions, gotSetID, gotGeneratedAt, err := repo.LatestActive(
		context.Background(),
		tenantID,
	)
	if err != nil {
		t.Fatalf("LatestActive returned error: %v", err)
	}
	if gotSetID != setID {
		t.Fatalf("setID = %s, want %s", gotSetID, setID)
	}
	if !gotGeneratedAt.Equal(generatedAt) {
		t.Fatalf("generatedAt = %v, want %v", gotGeneratedAt, generatedAt)
	}
	if len(questions) != 2 {
		t.Fatalf("len(questions) = %d, want 2", len(questions))
	}
	if questions[0].Ordinal != 1 || questions[1].Ordinal != 2 {
		t.Fatalf("ordinals = %d, %d", questions[0].Ordinal, questions[1].Ordinal)
	}
}

func TestStarterQuestionsRepositoryLatestActiveReturnsNotFound(t *testing.T) {
	t.Parallel()

	repo := &StarterQuestionsRepository{
		db: &fakeTenantDB{
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return &fakeRows{}, nil
			},
		},
	}

	_, _, _, err := repo.LatestActive(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
