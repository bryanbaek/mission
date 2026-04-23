package controller

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/bryanbaek/mission/internal/controlplane/model"
	"github.com/bryanbaek/mission/internal/controlplane/repository"
)

func (c *QueryController) SubmitFeedback(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	queryRunID uuid.UUID,
	rating model.QueryFeedbackRating,
	comment, correctedSQL string,
) (SubmitQueryFeedbackResult, error) {
	if _, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SubmitQueryFeedbackResult{}, ErrQueryAccessDenied
		}
		return SubmitQueryFeedbackResult{}, err
	}
	if rating != model.QueryFeedbackRatingUp &&
		rating != model.QueryFeedbackRatingDown {
		return SubmitQueryFeedbackResult{}, ErrInvalidQueryFeedback
	}

	run, err := c.runs.GetByTenantAndID(ctx, tenantID, queryRunID)
	if errors.Is(err, repository.ErrNotFound) {
		return SubmitQueryFeedbackResult{}, ErrQueryRunNotFound
	}
	if err != nil {
		return SubmitQueryFeedbackResult{}, err
	}
	if run.ClerkUserID != clerkUserID {
		return SubmitQueryFeedbackResult{}, ErrQueryFeedbackAccessDenied
	}

	feedback, err := c.feedback.Upsert(
		ctx,
		queryRunID,
		clerkUserID,
		rating,
		comment,
		correctedSQL,
		c.now().UTC(),
	)
	if err != nil {
		return SubmitQueryFeedbackResult{}, err
	}
	return SubmitQueryFeedbackResult{Feedback: feedback}, nil
}

func (c *QueryController) ListMyQueryRuns(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	limit int32,
) (ListMyQueryRunsResult, error) {
	if _, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ListMyQueryRunsResult{}, ErrQueryAccessDenied
		}
		return ListMyQueryRunsResult{}, err
	}

	normalizedLimit := int(limit)
	if normalizedLimit <= 0 || normalizedLimit > defaultListMyQueryRunsLimit {
		normalizedLimit = defaultListMyQueryRunsLimit
	}

	runs, err := c.runs.ListByTenantAndUser(
		ctx,
		tenantID,
		clerkUserID,
		normalizedLimit,
	)
	if err != nil {
		return ListMyQueryRunsResult{}, err
	}
	return ListMyQueryRunsResult{
		Runs: append([]model.TenantQueryRun(nil), runs...),
	}, nil
}

func (c *QueryController) ListReviewQueue(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	filter model.ReviewQueueFilter,
	limit int32,
) (ListReviewQueueResult, error) {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return ListReviewQueueResult{}, ErrQueryAccessDenied
	}
	if err != nil {
		return ListReviewQueueResult{}, err
	}
	if membership.Role != model.RoleOwner {
		return ListReviewQueueResult{}, ErrQueryReviewOwnerOnly
	}

	normalizedLimit := int(limit)
	if normalizedLimit <= 0 || normalizedLimit > defaultReviewQueueLimit {
		normalizedLimit = defaultReviewQueueLimit
	}
	switch filter {
	case model.ReviewQueueFilterAllRecent:
	default:
		filter = model.ReviewQueueFilterOpen
	}

	items, err := c.runs.ListReviewQueue(ctx, tenantID, filter, normalizedLimit)
	if err != nil {
		return ListReviewQueueResult{}, err
	}
	return ListReviewQueueResult{
		Items: append([]model.TenantQueryRunReviewItem(nil), items...),
	}, nil
}

func (c *QueryController) MarkQueryRunReviewed(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	queryRunID uuid.UUID,
) (MarkQueryRunReviewedResult, error) {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return MarkQueryRunReviewedResult{}, ErrQueryAccessDenied
	}
	if err != nil {
		return MarkQueryRunReviewedResult{}, err
	}
	if membership.Role != model.RoleOwner {
		return MarkQueryRunReviewedResult{}, ErrQueryReviewOwnerOnly
	}

	reviewedAt := c.now().UTC()
	if err := c.runs.MarkReviewed(
		ctx,
		tenantID,
		queryRunID,
		reviewedAt,
		clerkUserID,
	); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return MarkQueryRunReviewedResult{}, ErrQueryRunNotFound
		}
		return MarkQueryRunReviewedResult{}, err
	}

	return MarkQueryRunReviewedResult{
		QueryRunID: queryRunID,
		ReviewedAt: reviewedAt,
	}, nil
}

func (c *QueryController) CreateCanonicalExample(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	queryRunID uuid.UUID,
	question, sql, notes string,
) (model.TenantCanonicalQueryExample, error) {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.TenantCanonicalQueryExample{}, ErrQueryAccessDenied
	}
	if err != nil {
		return model.TenantCanonicalQueryExample{}, err
	}
	if membership.Role != model.RoleOwner {
		return model.TenantCanonicalQueryExample{}, ErrCanonicalQueryExampleOwnerOnly
	}
	if strings.TrimSpace(question) == "" || strings.TrimSpace(sql) == "" {
		return model.TenantCanonicalQueryExample{}, ErrInvalidCanonicalQueryExample
	}

	run, err := c.runs.GetByTenantAndID(ctx, tenantID, queryRunID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.TenantCanonicalQueryExample{}, ErrQueryRunNotFound
	}
	if err != nil {
		return model.TenantCanonicalQueryExample{}, err
	}

	example, err := c.examples.Create(
		ctx,
		tenantID,
		run.SchemaVersionID,
		run.ID,
		clerkUserID,
		question,
		sql,
		notes,
	)
	if err != nil {
		return model.TenantCanonicalQueryExample{}, err
	}
	if err := c.runs.MarkReviewed(
		ctx,
		tenantID,
		run.ID,
		c.now().UTC(),
		clerkUserID,
	); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.TenantCanonicalQueryExample{}, ErrQueryRunNotFound
		}
		return model.TenantCanonicalQueryExample{}, err
	}
	return example, nil
}

func (c *QueryController) ListCanonicalExamples(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
) (ListCanonicalQueryExamplesResult, error) {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return ListCanonicalQueryExamplesResult{}, ErrQueryAccessDenied
	}
	if err != nil {
		return ListCanonicalQueryExamplesResult{}, err
	}
	examples, err := c.examples.ListActiveByTenant(
		ctx,
		tenantID,
		c.maxCanonicalExamples,
	)
	if err != nil {
		return ListCanonicalQueryExamplesResult{}, err
	}
	return ListCanonicalQueryExamplesResult{
		ViewerCanManage: membership.Role == model.RoleOwner,
		Examples:        examples,
	}, nil
}

func (c *QueryController) ArchiveCanonicalExample(
	ctx context.Context,
	tenantID uuid.UUID,
	clerkUserID string,
	exampleID uuid.UUID,
) error {
	membership, err := c.tenants.EnsureMembership(ctx, tenantID, clerkUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrQueryAccessDenied
	}
	if err != nil {
		return err
	}
	if membership.Role != model.RoleOwner {
		return ErrCanonicalQueryExampleOwnerOnly
	}
	if err := c.examples.Archive(ctx, tenantID, exampleID, c.now().UTC()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrCanonicalQueryExampleNotFound
		}
		return err
	}
	return nil
}
