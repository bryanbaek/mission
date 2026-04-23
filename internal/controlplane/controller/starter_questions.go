package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/gateway/llm"
	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

var (
	ErrStarterQuestionsAccessDenied = errors.New("not a member of this tenant")
	ErrStarterQuestionsNoLayer      = errors.New("approved semantic layer not found")
	ErrStarterQuestionsInvalidLLM   = errors.New("starter questions output is invalid")
)

type starterQuestionsMembershipChecker interface {
	EnsureMembership(
		ctx context.Context,
		tenantID uuid.UUID,
		clerkUserID string,
	) (model.TenantUser, error)
}

type starterQuestionsLayerStore interface {
	LatestApprovedByTenant(
		ctx context.Context,
		tenantID uuid.UUID,
	) (model.TenantSemanticLayer, error)
}

type starterQuestionsStore interface {
	InsertSet(
		ctx context.Context,
		tenantID, semanticLayerID, setID uuid.UUID,
		questions []model.StarterQuestion,
	) error
	DeactivatePriorSets(ctx context.Context, tenantID uuid.UUID) error
	ReplaceActiveSet(
		ctx context.Context,
		tenantID, semanticLayerID, setID uuid.UUID,
		questions []model.StarterQuestion,
	) error
	LatestActive(
		ctx context.Context,
		tenantID uuid.UUID,
	) ([]model.StarterQuestion, uuid.UUID, time.Time, model.Locale, error)
}

type StarterQuestionsControllerConfig struct {
	Now            func() time.Time
	Model          string
	ProviderModels map[string]string
	MaxTokens      int
}

type StarterQuestionsListResult struct {
	Questions   []model.StarterQuestion
	GeneratedAt time.Time
	SetID       uuid.UUID
}

type StarterQuestionsController struct {
	layers         starterQuestionsLayerStore
	questions      starterQuestionsStore
	tenants        starterQuestionsMembershipChecker
	completer      llm.Provider
	model          string
	providerModels map[string]string
	maxTokens      int
	now            func() time.Time
}

type starterQuestionsOutput struct {
	Questions []starterQuestionCandidate `json:"questions"`
}

type starterQuestionCandidate struct {
	Text         string `json:"text"`
	Category     string `json:"category"`
	PrimaryTable string `json:"primary_table"`
}

func NewStarterQuestionsController(
	tenants starterQuestionsMembershipChecker,
	layers starterQuestionsLayerStore,
	questions starterQuestionsStore,
	completer llm.Provider,
	cfg StarterQuestionsControllerConfig,
) *StarterQuestionsController {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	modelName := strings.TrimSpace(cfg.Model)
	if modelName == "" {
		modelName = "claude-sonnet-4-6"
	}
	providerModels := llm.CloneProviderModels(cfg.ProviderModels)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2_048
	}
	return &StarterQuestionsController{
		layers:         layers,
		questions:      questions,
		tenants:        tenants,
		completer:      completer,
		model:          modelName,
		providerModels: providerModels,
		maxTokens:      maxTokens,
		now:            now,
	}
}

func (c *StarterQuestionsController) List(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	locale model.Locale,
) (StarterQuestionsListResult, error) {
	if err := c.ensureMembership(ctx, tenantID, clerkUserID); err != nil {
		return StarterQuestionsListResult{}, err
	}

	questions, setID, generatedAt, cachedLocale, err := c.questions.LatestActive(ctx, tenantID)
	switch {
	case err == nil:
		if cachedLocale == locale {
			return StarterQuestionsListResult{
				Questions:   questions,
				GeneratedAt: generatedAt,
				SetID:       setID,
			}, nil
		}
	case !errors.Is(err, repository.ErrNotFound):
		return StarterQuestionsListResult{}, err
	}

	return c.generateAndPersist(ctx, tenantID, locale)
}

func (c *StarterQuestionsController) Regenerate(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	locale model.Locale,
) (StarterQuestionsListResult, error) {
	if err := c.ensureMembership(ctx, tenantID, clerkUserID); err != nil {
		return StarterQuestionsListResult{}, err
	}

	return c.generateAndPersist(ctx, tenantID, locale)
}

func (c *StarterQuestionsController) ensureMembership(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) error {
	if _, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrStarterQuestionsAccessDenied
		}
		return err
	}
	return nil
}

func (c *StarterQuestionsController) generateAndPersist(
	ctx context.Context,
	tenantID uuid.UUID,
	locale model.Locale,
) (StarterQuestionsListResult, error) {
	layer, candidates, err := c.generate(ctx, tenantID, locale)
	if err != nil {
		return StarterQuestionsListResult{}, err
	}

	setID := uuid.New()
	questions := make([]model.StarterQuestion, 0, len(candidates))
	for index, candidate := range candidates {
		questions = append(questions, model.StarterQuestion{
			ID:              uuid.New(),
			SetID:           setID,
			TenantID:        tenantID,
			SemanticLayerID: layer.ID,
			Ordinal:         int32(index + 1),
			Text:            strings.TrimSpace(candidate.Text),
			Category:        model.StarterQuestionCategory(strings.TrimSpace(candidate.Category)),
			PrimaryTable:    strings.TrimSpace(candidate.PrimaryTable),
			Locale:          locale,
			IsActive:        true,
		})
	}

	if err := c.questions.ReplaceActiveSet(ctx, tenantID, layer.ID, setID, questions); err != nil {
		return StarterQuestionsListResult{}, err
	}

	persisted, persistedSetID, generatedAt, _, err := c.questions.LatestActive(ctx, tenantID)
	if err != nil {
		return StarterQuestionsListResult{}, err
	}

	return StarterQuestionsListResult{
		Questions:   persisted,
		GeneratedAt: generatedAt,
		SetID:       persistedSetID,
	}, nil
}

func (c *StarterQuestionsController) generate(
	ctx context.Context,
	tenantID uuid.UUID,
	locale model.Locale,
) (model.TenantSemanticLayer, []starterQuestionCandidate, error) {
	layer, err := c.layers.LatestApprovedByTenant(ctx, tenantID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.TenantSemanticLayer{}, nil, ErrStarterQuestionsNoLayer
	}
	if err != nil {
		return model.TenantSemanticLayer{}, nil, err
	}

	var content model.SemanticLayerContent
	if err := json.Unmarshal(layer.Content, &content); err != nil {
		return model.TenantSemanticLayer{}, nil, fmt.Errorf("decode semantic layer content: %w", err)
	}

	basePrompt := buildStarterQuestionsUserPrompt(content, locale)
	validationFeedback := ""

	for attempt := 0; attempt < 2; attempt++ {
		completion, err := c.completer.Complete(ctx, llm.CompletionRequest{
			Operation: "starter_questions.generate",
			System:    starterQuestionsSystemPrompt(locale),
			Messages: []llm.Message{{
				Role:          "user",
				CachedContent: basePrompt,
				Content:       buildStarterQuestionsRetryPrompt(validationFeedback, locale),
			}},
			Model:          c.model,
			ProviderModels: llm.CloneProviderModels(c.providerModels),
			MaxTokens:      c.maxTokens,
			OutputFormat: &llm.OutputFormat{
				Name:   "starter_questions",
				Schema: starterQuestionsOutputSchema(),
			},
			CacheControl: &llm.CacheControl{
				Type: "ephemeral",
				TTL:  "1h",
			},
		})
		if err != nil {
			return model.TenantSemanticLayer{}, nil, err
		}

		var payload starterQuestionsOutput
		if err := json.Unmarshal([]byte(completion.Content), &payload); err != nil {
			validationFeedback = starterQuestionsRetryDecodeFeedback(locale, err)
			if attempt == 0 {
				continue
			}
			return model.TenantSemanticLayer{}, nil, fmt.Errorf("%w: %s", ErrStarterQuestionsInvalidLLM, validationFeedback)
		}

		normalized := normalizeStarterQuestionCandidates(payload.Questions)
		if err := validateStarterQuestions(normalized, content); err != nil {
			validationFeedback = err.Error()
			if attempt == 0 {
				continue
			}
			return model.TenantSemanticLayer{}, nil, fmt.Errorf("%w: %v", ErrStarterQuestionsInvalidLLM, err)
		}

		return layer, normalized, nil
	}

	return model.TenantSemanticLayer{}, nil, fmt.Errorf("%w: retry budget exhausted", ErrStarterQuestionsInvalidLLM)
}

func normalizeStarterQuestionCandidates(
	questions []starterQuestionCandidate,
) []starterQuestionCandidate {
	out := make([]starterQuestionCandidate, 0, len(questions))
	for _, question := range questions {
		out = append(out, starterQuestionCandidate{
			Text:         strings.TrimSpace(question.Text),
			Category:     strings.TrimSpace(question.Category),
			PrimaryTable: strings.TrimSpace(question.PrimaryTable),
		})
	}
	return out
}

func starterQuestionCategoryValues() []string {
	return []string{
		string(model.StarterQuestionCategoryCount),
		string(model.StarterQuestionCategoryTrend),
		string(model.StarterQuestionCategoryTopN),
		string(model.StarterQuestionCategoryLatest),
		string(model.StarterQuestionCategoryComparison),
		string(model.StarterQuestionCategoryAnomaly),
	}
}
