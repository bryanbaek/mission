import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
} from "react";
import { Link } from "react-router-dom";

import { WorkspaceRole } from "../gen/onboarding/v1/onboarding_pb";
import {
  QueryFeedbackRating,
  ReviewQueueFilter,
  type QueryRunReviewItem,
} from "../gen/query/v1/query_pb";
import type { Tenant } from "../gen/tenant/v1/tenant_pb";
import {
  StoredQueryRunCard,
  TenantSelectionPanel,
} from "../features/chat/ui";
import { normalizeError, styles, timestampToMillis } from "../features/chat/shared";
import { useI18n } from "../lib/i18n";
import { useOnboardingClient } from "../lib/onboardingClient";
import { useQueryClient } from "../lib/queryClient";
import { useTenantClient } from "../lib/tenantClient";

const reviewQueueLimit = 50;

function feedbackRatingLabel(
  rating: QueryFeedbackRating,
  t: ReturnType<typeof useI18n>["t"],
) {
  switch (rating) {
    case QueryFeedbackRating.UP:
      return t("chat.feedback.ratingHelpful");
    case QueryFeedbackRating.DOWN:
      return t("chat.feedback.ratingNeedsWork");
    default:
      return t("common.unknown");
  }
}

function chatLinkForReview(tenantId: string, question: string): string {
  const params = new URLSearchParams({
    auto: "1",
    q: question,
    tenant: tenantId,
  });
  return `/chat?${params.toString()}`;
}

function ReviewQueueCard({
  filter,
  item,
  tenantId,
  onMarkReviewed,
  onRefresh,
}: {
  filter: ReviewQueueFilter;
  item: QueryRunReviewItem;
  tenantId: string;
  onMarkReviewed: (queryRunId: string) => Promise<void>;
  onRefresh: () => Promise<void>;
}) {
  const queryClient = useQueryClient();
  const { t } = useI18n();

  const run = item.run;
  const resolved =
    item.reviewedAt !== undefined || item.hasActiveCanonicalExample;
  const defaultExampleSql =
    item.latestFeedback?.correctedSql.trim() || run?.sqlExecuted || "";

  const [exampleQuestion, setExampleQuestion] = useState(run?.question ?? "");
  const [exampleSql, setExampleSql] = useState(defaultExampleSql);
  const [exampleNotes, setExampleNotes] = useState("");
  const [creatingExample, setCreatingExample] = useState(false);
  const [markingReviewed, setMarkingReviewed] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    setExampleQuestion(run?.question ?? "");
    setExampleSql(defaultExampleSql);
    setExampleNotes("");
    setActionError(null);
  }, [defaultExampleSql, run?.id, run?.question]);

  if (!run) {
    return null;
  }

  const canCreateExample =
    !creatingExample &&
    !item.hasActiveCanonicalExample &&
    exampleQuestion.trim() !== "" &&
    exampleSql.trim() !== "";

  const handleCreateExample = async (
    event: FormEvent<HTMLFormElement>,
  ) => {
    event.preventDefault();
    if (!canCreateExample) {
      return;
    }

    setCreatingExample(true);
    setActionError(null);
    try {
      await queryClient.createCanonicalQueryExample({
        tenantId,
        queryRunId: run.id,
        question: exampleQuestion,
        sql: exampleSql,
        notes: exampleNotes,
      });
      await onRefresh();
    } catch (err) {
      setActionError(normalizeError(err));
    } finally {
      setCreatingExample(false);
    }
  };

  const handleMarkReviewed = async () => {
    setMarkingReviewed(true);
    setActionError(null);
    try {
      await onMarkReviewed(run.id);
      await onRefresh();
    } catch (err) {
      setActionError(normalizeError(err));
    } finally {
      setMarkingReviewed(false);
    }
  };

  return (
    <StoredQueryRunCard
      item={run}
      actionSlot={
        <Link
          to={chatLinkForReview(tenantId, run.question)}
          className={styles.subtleButton}
        >
          {t("review.actions.openInChat")}
        </Link>
      }
    >
      <div className={styles.chipRow}>
        {item.hasFeedback ? (
          <span className={styles.warningChip}>
            {t("review.status.hasFeedback")}
          </span>
        ) : null}
        {item.hasActiveCanonicalExample ? (
          <span className={styles.successChip}>
            {t("review.status.activeExample")}
          </span>
        ) : null}
        {item.reviewedAt ? (
          <span className={styles.successChip}>
            {t("review.status.reviewed")}
          </span>
        ) : null}
        {!resolved ? (
          <span className={styles.warningChip}>
            {t("review.status.needsReview")}
          </span>
        ) : null}
      </div>

      {item.hasFeedback && item.latestFeedback ? (
        <section className="mt-4 rounded-2xl border border-slate-200 bg-white p-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold text-slate-900">
                {t("review.feedback.title")}
              </h3>
              <p className={`${styles.muted} mt-1`}>
                {t("review.feedback.subtitle")}
              </p>
            </div>
            <span
              className={
                item.latestFeedback.rating === QueryFeedbackRating.DOWN
                  ? styles.warningChip
                  : styles.successChip
              }
            >
              {feedbackRatingLabel(item.latestFeedback.rating, t)}
            </span>
          </div>

          {item.latestFeedback.comment ? (
            <p className="mt-4 rounded-xl bg-slate-50 px-3 py-2 text-sm text-slate-700">
              {item.latestFeedback.comment}
            </p>
          ) : (
            <p className={`${styles.muted} mt-4`}>
              {t("review.feedback.noComment")}
            </p>
          )}

          {item.latestFeedback.correctedSql ? (
            <details className={`${styles.details} mt-4`}>
              <summary className={styles.detailsHeader}>
                {t("review.feedback.correctedSql")}
              </summary>
              <div className={styles.detailsBody}>
                <pre className={styles.sqlBox}>
                  <code>{item.latestFeedback.correctedSql}</code>
                </pre>
              </div>
            </details>
          ) : null}
        </section>
      ) : (
        <div className={`${styles.bannerInfo} mt-4`}>
          {t("review.feedback.empty")}
        </div>
      )}

      {actionError ? (
        <div className={`${styles.bannerError} mt-4`}>{actionError}</div>
      ) : null}

      {!resolved ? (
        <section className="mt-4 rounded-2xl border border-slate-200 bg-white p-4">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 className="text-sm font-semibold text-slate-900">
                {t("review.actions.title")}
              </h3>
              <p className={styles.muted}>{t("review.actions.subtitle")}</p>
            </div>
            <button
              type="button"
              className={styles.secondaryButton}
              disabled={markingReviewed}
              onClick={() => void handleMarkReviewed()}
            >
              {markingReviewed
                ? t("review.actions.markingReviewed")
                : t("review.actions.markReviewed")}
            </button>
          </div>

          {!item.hasActiveCanonicalExample ? (
            <form
              onSubmit={handleCreateExample}
              className="mt-4 flex flex-col gap-4"
            >
              <label className="text-sm font-medium text-slate-900">
                {t("chat.examples.questionLabel")}
                <input
                  value={exampleQuestion}
                  onChange={(event) => setExampleQuestion(event.target.value)}
                  className={`${styles.input} mt-2`}
                />
              </label>

              <label className="text-sm font-medium text-slate-900">
                {t("chat.examples.sqlLabel")}
                <textarea
                  value={exampleSql}
                  onChange={(event) => setExampleSql(event.target.value)}
                  className={`${styles.textarea} mt-2 min-h-[140px] font-mono`}
                  placeholder={t("chat.examples.sqlPlaceholder")}
                />
              </label>

              <label className="text-sm font-medium text-slate-900">
                {t("chat.examples.notesLabel")}
                <textarea
                  value={exampleNotes}
                  onChange={(event) => setExampleNotes(event.target.value)}
                  className={`${styles.textarea} mt-2 min-h-[96px]`}
                  placeholder={t("review.examples.notesPlaceholder")}
                />
              </label>

              <div className="flex justify-end">
                <button
                  type="submit"
                  className={styles.primaryButton}
                  disabled={!canCreateExample}
                >
                  {creatingExample
                    ? t("chat.examples.creating")
                    : t("review.examples.create")}
                </button>
              </div>
            </form>
          ) : null}
        </section>
      ) : null}

      {filter === ReviewQueueFilter.ALL_RECENT && resolved ? (
        <div className={`${styles.bannerInfo} mt-4`}>
          {item.hasActiveCanonicalExample
            ? t("review.actions.resolvedWithExample")
            : t("review.actions.resolvedByReview")}
        </div>
      ) : null}
    </StoredQueryRunCard>
  );
}

export default function ReviewPage() {
  const onboardingClient = useOnboardingClient();
  const queryClient = useQueryClient();
  const tenantClient = useTenantClient();
  const { t } = useI18n();

  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [tenantsError, setTenantsError] = useState<string | null>(null);

  const [filter, setFilter] = useState<ReviewQueueFilter>(
    ReviewQueueFilter.OPEN,
  );
  const [items, setItems] = useState<QueryRunReviewItem[]>([]);
  const [loadingQueue, setLoadingQueue] = useState(false);
  const [queueError, setQueueError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadOwnerTenants = async () => {
      try {
        const [tenantsResp, workspacesResp] = await Promise.all([
          tenantClient.listTenants({}),
          onboardingClient.listWorkspaces({}),
        ]);
        if (cancelled) {
          return;
        }

        const ownerIDs = new Set(
          workspacesResp.workspaces
            .filter((workspace) => workspace.role === WorkspaceRole.OWNER)
            .map((workspace) => workspace.tenantId),
        );

        const ownerTenants = tenantsResp.tenants.filter((tenant) =>
          ownerIDs.has(tenant.id),
        );
        setTenants(ownerTenants);
        setTenantsError(null);
        setSelectedID((current) =>
          current && ownerTenants.some((tenant) => tenant.id === current)
            ? current
            : ownerTenants[0]?.id ?? null,
        );
      } catch (err) {
        if (!cancelled) {
          setTenantsError(normalizeError(err));
        }
      }
    };

    void loadOwnerTenants();
    return () => {
      cancelled = true;
    };
  }, [onboardingClient, tenantClient]);

  const loadQueue = useCallback(async () => {
    if (!selectedID) {
      setItems([]);
      setQueueError(null);
      return;
    }

    setLoadingQueue(true);
    try {
      const response = await queryClient.listReviewQueue({
        tenantId: selectedID,
        filter,
        limit: reviewQueueLimit,
      });
      setItems(response.items);
      setQueueError(null);
    } catch (err) {
      setItems([]);
      setQueueError(normalizeError(err));
    } finally {
      setLoadingQueue(false);
    }
  }, [filter, queryClient, selectedID]);

  useEffect(() => {
    void loadQueue();
  }, [loadQueue]);

  const emptyCopy = useMemo(() => {
    if (filter === ReviewQueueFilter.ALL_RECENT) {
      return t("review.queue.emptyAllRecent");
    }
    return t("review.queue.emptyOpen");
  }, [filter, t]);

  const handleMarkReviewed = useCallback(async (queryRunId: string) => {
    if (!selectedID) {
      return;
    }
    await queryClient.markQueryRunReviewed({
      tenantId: selectedID,
      queryRunId,
    });
  }, [queryClient, selectedID]);

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>{t("common.appLabel")}</p>
        <h1 className="text-3xl font-semibold tracking-tight">
          {t("review.hero.title")}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">
          {t("review.hero.subtitle")}
        </p>
      </section>

      <div className={styles.grid}>
        <aside>
          <TenantSelectionPanel
            tenants={tenants}
            selectedID={selectedID}
            tenantsError={tenantsError}
            onSelect={setSelectedID}
          />
          <div className={`${styles.bannerInfo} mt-6`}>
            {t("review.tenants.ownerOnly")}
          </div>
        </aside>

        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">{t("review.queue.title")}</h2>
              <p className={styles.muted}>{t("review.queue.subtitle")}</p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                className={
                  filter === ReviewQueueFilter.OPEN
                    ? styles.primaryButton
                    : styles.secondaryButton
                }
                onClick={() => setFilter(ReviewQueueFilter.OPEN)}
              >
                {t("review.filters.open")}
              </button>
              <button
                type="button"
                className={
                  filter === ReviewQueueFilter.ALL_RECENT
                    ? styles.primaryButton
                    : styles.secondaryButton
                }
                onClick={() => setFilter(ReviewQueueFilter.ALL_RECENT)}
              >
                {t("review.filters.allRecent")}
              </button>
            </div>
          </div>

          {queueError ? (
            <div className={`${styles.bannerError} mt-4`}>{queueError}</div>
          ) : null}

          {selectedID == null && tenants.length === 0 && !tenantsError ? (
            <div className={`${styles.bannerInfo} mt-4`}>
              {t("review.tenants.emptyOwners")}
            </div>
          ) : loadingQueue ? (
            <div className={`${styles.bannerInfo} mt-4`}>
              {t("review.queue.loading")}
            </div>
          ) : items.length === 0 ? (
            <div className={`${styles.bannerInfo} mt-4`}>{emptyCopy}</div>
          ) : (
            <div className="mt-4 flex flex-col gap-4">
              {items.map((item) => (
                <ReviewQueueCard
                  key={item.run?.id ?? `${timestampToMillis(item.reviewedAt) ?? 0}`}
                  filter={filter}
                  item={item}
                  tenantId={selectedID ?? ""}
                  onMarkReviewed={handleMarkReviewed}
                  onRefresh={loadQueue}
                />
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
