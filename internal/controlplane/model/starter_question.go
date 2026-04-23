package model

import (
	"time"

	"github.com/google/uuid"
)

type StarterQuestionCategory string

const (
	StarterQuestionCategoryCount      StarterQuestionCategory = "count"
	StarterQuestionCategoryTrend      StarterQuestionCategory = "trend"
	StarterQuestionCategoryTopN       StarterQuestionCategory = "top_n"
	StarterQuestionCategoryLatest     StarterQuestionCategory = "latest"
	StarterQuestionCategoryComparison StarterQuestionCategory = "comparison"
	StarterQuestionCategoryAnomaly    StarterQuestionCategory = "anomaly"
)

type Locale string

const (
	LocaleKorean  Locale = "ko"
	LocaleEnglish Locale = "en"
)

func NormalizeLocale(raw string) Locale {
	switch Locale(raw) {
	case LocaleEnglish:
		return LocaleEnglish
	case LocaleKorean:
		return LocaleKorean
	default:
		return LocaleKorean
	}
}

type StarterQuestion struct {
	ID              uuid.UUID
	SetID           uuid.UUID
	TenantID        uuid.UUID
	SemanticLayerID uuid.UUID
	Ordinal         int32
	Text            string
	Category        StarterQuestionCategory
	PrimaryTable    string
	Locale          Locale
	CreatedAt       time.Time
	IsActive        bool
}
