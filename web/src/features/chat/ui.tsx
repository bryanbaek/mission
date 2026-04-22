import {
  useEffect,
  useState,
  type ReactNode,
  type FormEvent,
} from "react";

import type { Tenant } from "../../gen/tenant/v1/tenant_pb";
import {
  QueryFeedbackRating,
  type AttemptDebug,
  type CanonicalQueryExample,
  type QueryRunHistoryItem,
} from "../../gen/query/v1/query_pb";
import { type Locale, useI18n } from "../../lib/i18n";
import StarterQuestions from "../../components/StarterQuestions";
import {
  historyCountLabel,
  normalizeError,
  queryPromptContextLabel,
  queryRunStatusChipClass,
  queryRunStatusLabel,
  ratingButtonClass,
  renderCell,
  stageLabel,
  styles,
  timestampToMillis,
  type CreateCanonicalExampleArgs,
  type QueryHistoryItem,
  type SubmitQueryFeedbackArgs,
} from "./shared";

function AttemptList({ attempts }: { attempts: AttemptDebug[] }) {
  const { t } = useI18n();

  if (attempts.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3">
      {attempts.map((attempt, index) => (
        <article
          key={`${attempt.stage}-${index}`}
          className={styles.attemptItem}
        >
          <div className="flex items-center justify-between gap-3">
            <p className="font-semibold text-slate-900">
              {t("chat.attempt.title", {
                index: index + 1,
                stage: stageLabel(attempt.stage, t),
              })}
            </p>
            {attempt.error ? (
              <span className={styles.warningChip}>
                {t("chat.attempt.failure")}
              </span>
            ) : (
              <span className={styles.successChip}>
                {t("chat.attempt.success")}
              </span>
            )}
          </div>
          {attempt.error ? (
            <p className="mt-3 rounded-xl bg-rose-50 px-3 py-2 text-rose-700">
              {attempt.error}
            </p>
          ) : (
            <p className="mt-3 text-slate-500">
              {t("chat.attempt.successHelp")}
            </p>
          )}
          {attempt.sql ? (
            <pre
              className={[
                "mt-3 overflow-auto rounded-xl bg-slate-950 p-3",
                "text-xs leading-6 text-emerald-200",
              ].join(" ")}
            >
              <code>{attempt.sql}</code>
            </pre>
          ) : null}
        </article>
      ))}
    </div>
  );
}

function QueryResultCard({
  item,
  canManageExamples,
  onSubmitFeedback,
  onCreateCanonicalExample,
  onCanonicalExampleChanged,
}: {
  item: QueryHistoryItem;
  canManageExamples: boolean;
  onSubmitFeedback: (args: SubmitQueryFeedbackArgs) => Promise<void>;
  onCreateCanonicalExample: (
    args: CreateCanonicalExampleArgs,
  ) => Promise<void>;
  onCanonicalExampleChanged: (tenantId: string) => Promise<void>;
}) {
  const { formatDateTime, formatNumber, t } = useI18n();
  const response = item.response;
  const queryRunId = response?.queryRunId ?? "";
  const defaultExampleSql = response?.sqlExecuted ?? "";

  const [rating, setRating] = useState<QueryFeedbackRating>(
    QueryFeedbackRating.UNSPECIFIED,
  );
  const [comment, setComment] = useState("");
  const [correctedSql, setCorrectedSql] = useState("");
  const [submittingFeedback, setSubmittingFeedback] = useState(false);
  const [feedbackError, setFeedbackError] = useState<string | null>(null);
  const [feedbackSuccess, setFeedbackSuccess] = useState<string | null>(null);

  const [exampleQuestion, setExampleQuestion] = useState(item.question);
  const [exampleQuestionDirty, setExampleQuestionDirty] = useState(false);
  const [exampleSql, setExampleSql] = useState(defaultExampleSql);
  const [exampleSqlDirty, setExampleSqlDirty] = useState(false);
  const [exampleNotes, setExampleNotes] = useState("");
  const [creatingExample, setCreatingExample] = useState(false);
  const [exampleError, setExampleError] = useState<string | null>(null);
  const [exampleSuccess, setExampleSuccess] = useState<string | null>(null);

  useEffect(() => {
    if (!exampleQuestionDirty) {
      setExampleQuestion(item.question);
    }
  }, [exampleQuestionDirty, item.question]);

  useEffect(() => {
    if (!exampleSqlDirty) {
      setExampleSql(
        correctedSql.trim() !== "" ? correctedSql : defaultExampleSql,
      );
    }
  }, [correctedSql, defaultExampleSql, exampleSqlDirty]);

  const effectiveExampleSql =
    exampleSqlDirty || correctedSql.trim() === ""
      ? exampleSql
      : correctedSql;

  const canSubmitFeedback =
    queryRunId !== "" &&
    rating !== QueryFeedbackRating.UNSPECIFIED &&
    !submittingFeedback;
  const canCreateExample =
    canManageExamples &&
    queryRunId !== "" &&
    effectiveExampleSql.trim() !== "" &&
    exampleQuestion.trim() !== "" &&
    !creatingExample;

  const handleFeedbackSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmitFeedback) {
      return;
    }
    setSubmittingFeedback(true);
    setFeedbackError(null);
    setFeedbackSuccess(null);
    try {
      await onSubmitFeedback({
        tenantId: item.tenantId,
        queryRunId,
        rating,
        comment,
        correctedSql,
      });
      setFeedbackSuccess(t("chat.feedback.success"));
    } catch (err) {
      setFeedbackError(normalizeError(err));
    } finally {
      setSubmittingFeedback(false);
    }
  };

  const handleCreateExample = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canCreateExample) {
      return;
    }
    setCreatingExample(true);
    setExampleError(null);
    setExampleSuccess(null);
    try {
      await onCreateCanonicalExample({
        tenantId: item.tenantId,
        queryRunId,
        question: exampleQuestion,
        sql: effectiveExampleSql,
        notes: exampleNotes,
      });
      setExampleSuccess(t("chat.examples.createSuccess"));
      await onCanonicalExampleChanged(item.tenantId);
    } catch (err) {
      setExampleError(normalizeError(err));
    } finally {
      setCreatingExample(false);
    }
  };

  return (
    <article className={styles.historyItem}>
      <div
        className={[
          "flex flex-col gap-3",
          "md:flex-row md:items-start md:justify-between",
        ].join(" ")}
      >
        <div className="min-w-0">
          <p
            className={[
              "text-xs font-semibold uppercase tracking-[0.18em]",
              "text-slate-400",
            ].join(" ")}
          >
            {item.tenantName} · {formatDateTime(item.createdAt)}
          </p>
          <div className="mt-2">
            <p className="text-sm font-medium text-slate-500">
              {t("chat.card.question")}
            </p>
            <div className={`${styles.promptCard} mt-2`}>{item.question}</div>
          </div>
        </div>
        <span
          className={
            item.status === "success" ? styles.successChip : styles.warningChip
          }
        >
          {item.status === "success"
            ? t("chat.card.success")
            : t("chat.card.error")}
        </span>
      </div>

      {item.error ? (
        <div className={`${styles.bannerError} mt-4`}>{item.error}</div>
      ) : null}

      {response?.summaryKo ? (
        <div className={`${styles.summaryCard} mt-4`}>
          <p
            className={[
              "text-xs font-semibold uppercase tracking-[0.18em]",
              "text-emerald-700",
            ].join(" ")}
          >
            {t("chat.result.summaryTitle")}
          </p>
          <p className="mt-2">{response.summaryKo}</p>
        </div>
      ) : null}

      {response?.warnings.length ? (
        <div className={`${styles.chipRow} mt-4`}>
          {response.warnings.map((warning, index) => (
            <span key={`${warning}-${index}`} className={styles.warningChip}>
              {warning}
            </span>
          ))}
        </div>
      ) : null}

      {response ? (
        <>
          <div className={`${styles.metaGrid} mt-4`}>
            <div className={styles.metaTile}>
              <p className={styles.metaLabel}>{t("chat.result.rowCount")}</p>
              <p className={styles.metaValue}>
                {formatNumber(response.rowCount)}
              </p>
            </div>
            <div className={styles.metaTile}>
              <p className={styles.metaLabel}>{t("chat.result.elapsed")}</p>
              <p className={styles.metaValue}>
                {formatNumber(response.elapsedMs)} ms
              </p>
            </div>
            <div className={styles.metaTile}>
              <p className={styles.metaLabel}>
                {t("chat.result.safetyLimit")}
              </p>
              <p className={styles.metaValue}>
                {response.limitInjected
                  ? t("chat.result.limitInjected")
                  : t("chat.result.noLimit")}
              </p>
            </div>
          </div>

          <details className={`${styles.details} mt-4`} open>
            <summary className={styles.detailsHeader}>
              {t("chat.result.dataTitle")}
            </summary>
            <div className={styles.detailsBody}>
              {response.rows.length === 0 ? (
                <p className={styles.muted}>{t("chat.result.noRows")}</p>
              ) : (
                <div className={styles.tableShell}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        {response.columns.map((column) => (
                          <th key={column} className={styles.th} scope="col">
                            {column}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {response.rows.map((row, rowIndex) => (
                        <tr key={`${item.id}-${rowIndex}`}>
                          {response.columns.map((column) => (
                            <td
                              key={`${rowIndex}-${column}`}
                              className={styles.td}
                            >
                              {row.values[column] ??
                                renderCell(response, rowIndex, column)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </details>

          <details className={`${styles.details} mt-4`}>
            <summary className={styles.detailsHeader}>
              {t("chat.result.sqlAttemptsTitle")}
            </summary>
            <div className={`${styles.detailsBody} flex flex-col gap-4`}>
              {response.sqlOriginal ? (
                <div>
                  <p className="text-sm font-medium text-slate-900">
                    {t("chat.result.originalSql")}
                  </p>
                  <pre className={`${styles.sqlBox} mt-2`}>
                    <code>{response.sqlOriginal}</code>
                  </pre>
                </div>
              ) : null}
              {response.sqlExecuted ? (
                <div>
                  <p className="text-sm font-medium text-slate-900">
                    {t("chat.result.executedSql")}
                  </p>
                  <pre className={`${styles.sqlBox} mt-2`}>
                    <code>{response.sqlExecuted}</code>
                  </pre>
                </div>
              ) : null}
              <div>
                <p className="text-sm font-medium text-slate-900">
                  {t("chat.result.attempts")}
                </p>
                <div className="mt-2">
                  <AttemptList attempts={response.attempts} />
                </div>
              </div>
            </div>
          </details>
        </>
      ) : null}

      {queryRunId !== "" ? (
        <section className="mt-4 rounded-2xl border border-slate-200 bg-white p-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold text-slate-900">
                {t("chat.feedback.title")}
              </h3>
              <p className={`${styles.muted} mt-1`}>
                {t("chat.feedback.subtitle")}
              </p>
            </div>
            <code className={styles.chip}>{queryRunId}</code>
          </div>

          {feedbackError ? (
            <div className={`${styles.bannerError} mt-4`}>{feedbackError}</div>
          ) : null}
          {feedbackSuccess ? (
            <div className={`${styles.bannerSuccess} mt-4`}>
              {feedbackSuccess}
            </div>
          ) : null}

          <form
            onSubmit={handleFeedbackSubmit}
            className="mt-4 flex flex-col gap-4"
          >
            <div className={styles.chipRow}>
              <button
                type="button"
                className={ratingButtonClass(rating === QueryFeedbackRating.UP)}
                aria-pressed={rating === QueryFeedbackRating.UP}
                onClick={() => setRating(QueryFeedbackRating.UP)}
              >
                {t("chat.feedback.ratingHelpful")}
              </button>
              <button
                type="button"
                className={ratingButtonClass(
                  rating === QueryFeedbackRating.DOWN,
                )}
                aria-pressed={rating === QueryFeedbackRating.DOWN}
                onClick={() => setRating(QueryFeedbackRating.DOWN)}
              >
                {t("chat.feedback.ratingNeedsWork")}
              </button>
            </div>

            <label className="text-sm font-medium text-slate-900">
              {t("chat.feedback.commentLabel")}
              <textarea
                value={comment}
                onChange={(event) => setComment(event.target.value)}
                className={`${styles.textarea} mt-2 min-h-[96px]`}
                placeholder={t("chat.feedback.commentPlaceholder")}
              />
            </label>

            <label className="text-sm font-medium text-slate-900">
              {t("chat.feedback.correctedSqlLabel")}
              <textarea
                value={correctedSql}
                onChange={(event) => setCorrectedSql(event.target.value)}
                className={`${styles.textarea} mt-2 min-h-[120px] font-mono`}
                placeholder={t("chat.feedback.correctedSqlPlaceholder")}
              />
            </label>

            <div className="flex justify-end">
              <button
                type="submit"
                className={styles.primaryButton}
                disabled={!canSubmitFeedback}
              >
                {submittingFeedback
                  ? t("chat.feedback.submitting")
                  : t("chat.feedback.submit")}
              </button>
            </div>
          </form>
        </section>
      ) : null}

      {canManageExamples && queryRunId !== "" ? (
        <section className="mt-4 rounded-2xl border border-slate-200 bg-white p-4">
          <div>
            <h3 className="text-sm font-semibold text-slate-900">
              {t("chat.examples.createTitle")}
            </h3>
            <p className={`${styles.muted} mt-1`}>
              {t("chat.examples.createSubtitle")}
            </p>
          </div>

          {exampleError ? (
            <div className={`${styles.bannerError} mt-4`}>{exampleError}</div>
          ) : null}
          {exampleSuccess ? (
            <div className={`${styles.bannerSuccess} mt-4`}>
              {exampleSuccess}
            </div>
          ) : null}

          <form
            onSubmit={handleCreateExample}
            className="mt-4 flex flex-col gap-4"
          >
            <label className="text-sm font-medium text-slate-900">
              {t("chat.examples.questionLabel")}
              <input
                value={exampleQuestion}
                onChange={(event) => {
                  setExampleQuestionDirty(true);
                  setExampleQuestion(event.target.value);
                }}
                className={`${styles.input} mt-2`}
              />
            </label>

            <label className="text-sm font-medium text-slate-900">
              {t("chat.examples.sqlLabel")}
              <textarea
                value={effectiveExampleSql}
                onChange={(event) => {
                  setExampleSqlDirty(true);
                  setExampleSql(event.target.value);
                }}
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
                placeholder={t("chat.examples.notesPlaceholder")}
              />
            </label>

            <div className="flex justify-end">
              <button
                type="submit"
                className={styles.secondaryButton}
                disabled={!canCreateExample}
              >
                {creatingExample
                  ? t("chat.examples.creating")
                  : t("chat.examples.create")}
              </button>
            </div>
          </form>
        </section>
      ) : null}
    </article>
  );
}

export function StoredQueryRunCard({
  item,
  onRerun,
  actionSlot,
  children,
}: {
  item: QueryRunHistoryItem;
  onRerun?: (question: string) => void;
  actionSlot?: ReactNode;
  children?: ReactNode;
}) {
  const { formatDateTime, formatNumber, t } = useI18n();
  const createdAtMillis = timestampToMillis(item.createdAt);
  const completedAtMillis = timestampToMillis(item.completedAt);

  return (
    <article className={styles.historyItem}>
      <div
        className={[
          "flex flex-col gap-3",
          "md:flex-row md:items-start md:justify-between",
        ].join(" ")}
      >
        <div className="min-w-0">
          <p
            className={[
              "text-xs font-semibold uppercase tracking-[0.18em]",
              "text-slate-400",
            ].join(" ")}
          >
            {createdAtMillis === null
              ? t("common.na")
              : formatDateTime(createdAtMillis)}
          </p>
          <div className="mt-2">
            <p className="text-sm font-medium text-slate-500">
              {t("chat.card.question")}
            </p>
            <div className={`${styles.promptCard} mt-2`}>{item.question}</div>
          </div>
        </div>
        <div className="flex items-center gap-2 self-start">
          <span className={queryRunStatusChipClass(item.status)}>
            {queryRunStatusLabel(item.status, t)}
          </span>
          {actionSlot ?? null}
          {actionSlot == null && onRerun ? (
            <button
              type="button"
              className={styles.subtleButton}
              onClick={() => onRerun(item.question)}
            >
              {t("chat.persistent.rerun")}
            </button>
          ) : null}
        </div>
      </div>

      <div className={`${styles.chipRow} mt-4`}>
        <span className={styles.chip}>
          {t("chat.persistent.context")}:{" "}
          {queryPromptContextLabel(item.promptContextSource, t)}
        </span>
        <span className={styles.chip}>{t("chat.persistent.metadataOnly")}</span>
      </div>

      {item.warnings.length ? (
        <div className={`${styles.chipRow} mt-4`}>
          {item.warnings.map((warning, index) => (
            <span key={`${warning}-${index}`} className={styles.warningChip}>
              {warning}
            </span>
          ))}
        </div>
      ) : null}

      <div className={`${styles.metaGrid} mt-4`}>
        <div className={styles.metaTile}>
          <p className={styles.metaLabel}>{t("chat.persistent.createdAt")}</p>
          <p className={styles.metaValue}>
            {createdAtMillis === null
              ? t("common.na")
              : formatDateTime(createdAtMillis)}
          </p>
        </div>
        <div className={styles.metaTile}>
          <p className={styles.metaLabel}>{t("chat.persistent.completedAt")}</p>
          <p className={styles.metaValue}>
            {completedAtMillis === null
              ? t("common.na")
              : formatDateTime(completedAtMillis)}
          </p>
        </div>
        <div className={styles.metaTile}>
          <p className={styles.metaLabel}>{t("chat.result.rowCount")}</p>
          <p className={styles.metaValue}>{formatNumber(item.rowCount)}</p>
        </div>
        <div className={styles.metaTile}>
          <p className={styles.metaLabel}>{t("chat.result.elapsed")}</p>
          <p className={styles.metaValue}>
            {formatNumber(item.elapsedMs)} ms
          </p>
        </div>
      </div>

      {item.errorMessage ? (
        <div className={`${styles.bannerError} mt-4`}>
          <p className="font-medium">{t("chat.persistent.failure")}</p>
          <p className="mt-1">
            {item.errorStage
              ? `${stageLabel(item.errorStage, t)} · ${item.errorMessage}`
              : item.errorMessage}
          </p>
        </div>
      ) : null}

      <details className={`${styles.details} mt-4`}>
        <summary className={styles.detailsHeader}>
          {t("chat.persistent.detailsTitle")}
        </summary>
        <div className={`${styles.detailsBody} flex flex-col gap-4`}>
          {item.sqlOriginal ? (
            <div>
              <p className="text-sm font-medium text-slate-900">
                {t("chat.result.originalSql")}
              </p>
              <pre className={`${styles.sqlBox} mt-2`}>
                <code>{item.sqlOriginal}</code>
              </pre>
            </div>
          ) : null}
          {item.sqlExecuted ? (
            <div>
              <p className="text-sm font-medium text-slate-900">
                {t("chat.result.executedSql")}
              </p>
              <pre className={`${styles.sqlBox} mt-2`}>
                <code>{item.sqlExecuted}</code>
              </pre>
            </div>
          ) : null}
          {item.attempts.length ? (
            <div>
              <p className="text-sm font-medium text-slate-900">
                {t("chat.result.attempts")}
              </p>
              <div className="mt-2">
                <AttemptList attempts={item.attempts} />
              </div>
            </div>
          ) : null}
        </div>
      </details>

      {children ? <div className="mt-4">{children}</div> : null}
    </article>
  );
}

export function TenantSelectionPanel({
  tenants,
  selectedID,
  tenantsError,
  onSelect,
}: {
  tenants: Tenant[];
  selectedID: string | null;
  tenantsError: string | null;
  onSelect: (tenantId: string) => void;
}) {
  const { t } = useI18n();

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className="text-lg font-semibold">{t("chat.tenants.title")}</h2>
          <p className={styles.muted}>{t("chat.tenants.subtitle")}</p>
        </div>
      </div>

      {tenantsError ? (
        <div className={`${styles.bannerError} mt-4`}>{tenantsError}</div>
      ) : null}

      <ul className="mt-4 flex flex-col gap-1">
        {tenants.length === 0 ? (
          <li className="px-3 py-6 text-center text-sm text-slate-500">
            {t("chat.tenants.empty")}
          </li>
        ) : (
          tenants.map((tenant) => {
            const active = tenant.id === selectedID;
            return (
              <li key={tenant.id}>
                <button
                  type="button"
                  onClick={() => onSelect(tenant.id)}
                  className={[
                    styles.row,
                    "w-full text-left",
                    active ? styles.rowActive : styles.rowIdle,
                  ].join(" ")}
                >
                  <span>
                    <span className="block font-medium">{tenant.name}</span>
                    <span
                      className={[
                        "block text-xs",
                        active ? "text-slate-300" : "text-slate-400",
                      ].join(" ")}
                    >
                      {tenant.slug}
                    </span>
                  </span>
                </button>
              </li>
            );
          })
        )}
      </ul>

      <div className={`${styles.bannerInfo} mt-4`}>
        {t("chat.tenants.guardrail")}
      </div>
    </section>
  );
}

export function CanonicalExamplesPanel({
  examples,
  canManage,
  loading,
  error,
  onArchive,
}: {
  examples: CanonicalQueryExample[];
  canManage: boolean;
  loading: boolean;
  error: string | null;
  onArchive: (exampleId: string) => Promise<void>;
}) {
  const { formatDateTime, t } = useI18n();
  const [archivingID, setArchivingID] = useState<string | null>(null);
  const [archiveError, setArchiveError] = useState<string | null>(null);

  const handleArchive = async (exampleId: string) => {
    setArchivingID(exampleId);
    setArchiveError(null);
    try {
      await onArchive(exampleId);
    } catch (err) {
      setArchiveError(normalizeError(err));
    } finally {
      setArchivingID(null);
    }
  };

  return (
    <section className={`${styles.sectionCard} mt-6`}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className="text-lg font-semibold">{t("chat.examples.title")}</h2>
          <p className={styles.muted}>{t("chat.examples.subtitle")}</p>
        </div>
        {examples.length > 0 ? (
          <span className={styles.chip}>{examples.length}</span>
        ) : null}
      </div>

      {error ? <div className={`${styles.bannerError} mt-4`}>{error}</div> : null}
      {archiveError ? (
        <div className={`${styles.bannerError} mt-4`}>{archiveError}</div>
      ) : null}

      {loading ? (
        <div className={`${styles.bannerInfo} mt-4`}>
          {t("chat.examples.loading")}
        </div>
      ) : examples.length === 0 ? (
        <div className={`${styles.bannerInfo} mt-4`}>
          {t("chat.examples.empty")}
        </div>
      ) : (
        <div className="mt-4 flex flex-col gap-3">
          {examples.map((example) => {
            const createdAtMillis = timestampToMillis(example.createdAt);
            return (
              <article key={example.id} className={styles.exampleItem}>
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-slate-900">
                      {example.question}
                    </p>
                    <p className={`${styles.muted} mt-1`}>
                      {createdAtMillis === null
                        ? t("common.na")
                        : formatDateTime(createdAtMillis)}
                    </p>
                  </div>
                  {canManage ? (
                    <button
                      type="button"
                      className={styles.dangerButton}
                      disabled={archivingID === example.id}
                      onClick={() => void handleArchive(example.id)}
                    >
                      {archivingID === example.id
                        ? t("chat.examples.archiving")
                        : t("chat.examples.archive")}
                    </button>
                  ) : null}
                </div>

                {example.notes ? (
                  <p className="mt-3 rounded-xl bg-white px-3 py-2 text-sm text-slate-700">
                    {example.notes}
                  </p>
                ) : null}

                <details className={`${styles.details} mt-3`}>
                  <summary className={styles.detailsHeader}>
                    {t("chat.examples.sqlPreview")}
                  </summary>
                  <div className={styles.detailsBody}>
                    <pre className={styles.sqlBox}>
                      <code>{example.sql}</code>
                    </pre>
                  </div>
                </details>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}

export function QueryComposerPanel({
  history,
  question,
  selectedTenant,
  submitting,
  onQuestionChange,
  onRunQuestion,
  onSubmit,
}: {
  history: QueryHistoryItem[];
  question: string;
  selectedTenant: Tenant | null;
  submitting: boolean;
  onQuestionChange: (value: string) => void;
  onRunQuestion: (question: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => Promise<void>;
}) {
  const { t } = useI18n();
  const canSubmit =
    selectedTenant !== null &&
    question.trim() !== "" &&
    !submitting;

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className="text-lg font-semibold">{t("chat.form.title")}</h2>
          <p className={styles.muted}>
            {selectedTenant
              ? t("chat.form.subtitle.selected", {
                  tenant: selectedTenant.name,
                })
              : t("chat.form.subtitle.unselected")}
          </p>
        </div>
        {selectedTenant ? (
          <span className={styles.chip}>{selectedTenant.slug}</span>
        ) : null}
      </div>

      <form onSubmit={onSubmit} className="mt-5 flex flex-col gap-4">
        {history.length === 0 && selectedTenant ? (
          <StarterQuestions
            tenantId={selectedTenant.id}
            onPick={onRunQuestion}
          />
        ) : null}
        <label
          className="text-sm font-medium text-slate-900"
          htmlFor="chat-question"
        >
          {t("chat.form.label")}
        </label>
        <textarea
          id="chat-question"
          value={question}
          onChange={(event) => onQuestionChange(event.target.value)}
          className={styles.textarea}
          placeholder={t("chat.form.placeholder")}
        />
        <div
          className={[
            "flex flex-col gap-3",
            "md:flex-row md:items-center md:justify-between",
          ].join(" ")}
        >
          <p className={styles.muted}>{t("chat.form.help")}</p>
          <button
            type="submit"
            className={styles.primaryButton}
            disabled={!canSubmit}
          >
            {submitting ? t("chat.form.submitting") : t("chat.form.submit")}
          </button>
        </div>
      </form>
    </section>
  );
}

export function PersistentHistoryPanel({
  runs,
  loading,
  error,
  onRerun,
  locale,
}: {
  runs: QueryRunHistoryItem[];
  loading: boolean;
  error: string | null;
  onRerun: (question: string) => void;
  locale: Locale;
}) {
  const { formatNumber, t } = useI18n();

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className="text-lg font-semibold">
            {t("chat.persistent.title")}
          </h2>
          <p className={styles.muted}>{t("chat.persistent.subtitle")}</p>
        </div>
        {runs.length > 0 ? (
          <span className={styles.chip}>
            {historyCountLabel(
              runs.length,
              locale,
              formatNumber(runs.length),
              t,
            )}
          </span>
        ) : null}
      </div>

      {error ? <div className={`${styles.bannerError} mt-4`}>{error}</div> : null}

      {loading ? (
        <div className={`${styles.bannerInfo} mt-4`}>
          {t("chat.persistent.loading")}
        </div>
      ) : runs.length === 0 ? (
        <div className={`${styles.bannerInfo} mt-4`}>
          {t("chat.persistent.empty")}
        </div>
      ) : (
        <div className="mt-4 flex flex-col gap-4">
          {runs.map((run) => (
            <StoredQueryRunCard
              key={run.id}
              item={run}
              onRerun={onRerun}
            />
          ))}
        </div>
      )}
    </section>
  );
}

export function SessionHistoryPanel({
  history,
  locale,
  viewerCanManageByTenant,
  onSubmitFeedback,
  onCreateCanonicalExample,
  onCanonicalExampleChanged,
}: {
  history: QueryHistoryItem[];
  locale: Locale;
  viewerCanManageByTenant: Record<string, boolean>;
  onSubmitFeedback: (args: SubmitQueryFeedbackArgs) => Promise<void>;
  onCreateCanonicalExample: (
    args: CreateCanonicalExampleArgs,
  ) => Promise<void>;
  onCanonicalExampleChanged: (tenantId: string) => Promise<void>;
}) {
  const { formatNumber, t } = useI18n();

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className="text-lg font-semibold">{t("chat.history.title")}</h2>
          <p className={styles.muted}>{t("chat.history.subtitle")}</p>
        </div>
        {history.length > 0 ? (
          <span className={styles.chip}>
            {historyCountLabel(
              history.length,
              locale,
              formatNumber(history.length),
              t,
            )}
          </span>
        ) : null}
      </div>

      {history.length === 0 ? (
        <div className={`${styles.bannerInfo} mt-4`}>
          {t("chat.history.empty")}
        </div>
      ) : (
        <div className="mt-4 flex flex-col gap-4">
          {history.map((item) => (
            <QueryResultCard
              key={item.id}
              item={item}
              canManageExamples={viewerCanManageByTenant[item.tenantId] ?? false}
              onSubmitFeedback={onSubmitFeedback}
              onCreateCanonicalExample={onCreateCanonicalExample}
              onCanonicalExampleChanged={onCanonicalExampleChanged}
            />
          ))}
        </div>
      )}
    </section>
  );
}
