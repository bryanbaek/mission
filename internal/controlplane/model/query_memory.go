package model

import "time"

import "github.com/google/uuid"

type QueryPromptContextSource string

const (
	QueryPromptContextSourceApproved  QueryPromptContextSource = "approved"
	QueryPromptContextSourceDraft     QueryPromptContextSource = "draft"
	QueryPromptContextSourceRawSchema QueryPromptContextSource = "raw_schema"
)

type QueryRunStatus string

const (
	QueryRunStatusRunning   QueryRunStatus = "running"
	QueryRunStatusSucceeded QueryRunStatus = "succeeded"
	QueryRunStatusFailed    QueryRunStatus = "failed"
)

type QueryFeedbackRating string

const (
	QueryFeedbackRatingUp   QueryFeedbackRating = "up"
	QueryFeedbackRatingDown QueryFeedbackRating = "down"
)

type QueryRunAttempt struct {
	SQL   string `json:"sql"`
	Error string `json:"error"`
	Stage string `json:"stage"`
}

type TenantQueryRun struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	SchemaVersionID     uuid.UUID
	SemanticLayerID     *uuid.UUID
	PromptContextSource QueryPromptContextSource
	ClerkUserID         string
	Question            string
	Status              QueryRunStatus
	SQLOriginal         string
	SQLExecuted         string
	RowCount            int64
	ElapsedMS           int64
	ErrorStage          string
	ErrorMessage        string
	CreatedAt           time.Time
	CompletedAt         *time.Time
}

type TenantQueryFeedback struct {
	QueryRunID   uuid.UUID
	ClerkUserID  string
	Rating       QueryFeedbackRating
	Comment      string
	CorrectedSQL string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type TenantCanonicalQueryExample struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	SchemaVersionID  uuid.UUID
	SourceQueryRunID uuid.UUID
	CreatedByUserID  string
	Question         string
	SQL              string
	Notes            string
	ArchivedAt       *time.Time
	CreatedAt        time.Time
}
