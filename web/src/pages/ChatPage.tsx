import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { ConnectError } from "@connectrpc/connect";
import { useSearchParams } from "react-router-dom";

import {
  AskQuestionResponseSchema,
  QueryFeedbackRating,
  type AskQuestionResponse,
  type AttemptDebug,
  type CanonicalQueryExample,
} from "../gen/query/v1/query_pb";
import StarterQuestions from "../components/StarterQuestions";
import type { Tenant } from "../gen/tenant/v1/tenant_pb";
import { type Locale, useI18n } from "../lib/i18n";
import { useQueryClient } from "../lib/queryClient";
import { useTenantClient } from "../lib/tenantClient";

type QueryHistoryItem = {
  id: string;
  tenantId: string;
  tenantName: string;
  question: string;
  createdAt: number;
  status: "success" | "error";
  response: AskQuestionResponse | null;
  error: string | null;
};

const styles = {
  shell: "flex flex-col gap-6",
  heroCard: [
    "rounded-3xl border border-slate-200 bg-white p-8 shadow-sm",
    "flex flex-col gap-2",
  ].join(" "),
  introLabel: [
    "text-xs font-semibold uppercase tracking-[0.24em]",
    "text-slate-500",
  ].join(" "),
  grid: "grid gap-6 xl:grid-cols-[320px_minmax(0,1fr)]",
  sectionCard: "rounded-3xl border border-slate-200 bg-white p-6 shadow-sm",
  sectionHeader:
    "flex items-center justify-between gap-4 border-b border-slate-200 pb-4",
  row: "flex items-center justify-between gap-3 px-3 py-2",
  rowActive: "rounded-lg bg-slate-950 text-white",
  rowIdle: "rounded-lg hover:bg-slate-100",
  muted: "text-sm text-slate-500",
  textarea: [
    "min-h-[140px] w-full rounded-2xl border border-slate-300 px-4 py-3",
    "text-sm leading-6 text-slate-900",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
  input: [
    "w-full rounded-xl border border-slate-300 px-3 py-2 text-sm text-slate-900",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
  primaryButton: [
    "inline-flex items-center justify-center rounded-xl bg-slate-950",
    "px-4 py-2 text-sm font-medium text-white transition",
    "hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-300",
  ].join(" "),
  secondaryButton: [
    "inline-flex items-center justify-center rounded-xl border border-slate-300",
    "bg-white px-4 py-2 text-sm font-medium text-slate-700 transition",
    "hover:bg-slate-50 disabled:cursor-not-allowed disabled:text-slate-300",
  ].join(" "),
  subtleButton: [
    "inline-flex items-center justify-center rounded-xl px-3 py-2",
    "text-xs font-medium text-slate-600 transition hover:bg-slate-100",
    "disabled:cursor-not-allowed disabled:text-slate-300",
  ].join(" "),
  dangerButton: [
    "inline-flex items-center justify-center rounded-xl px-3 py-2",
    "text-xs font-medium text-rose-700 transition hover:bg-rose-50",
    "disabled:cursor-not-allowed disabled:text-slate-300",
  ].join(" "),
  bannerError: [
    "rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3",
    "text-sm text-rose-700",
  ].join(" "),
  bannerInfo: [
    "rounded-2xl border border-sky-200 bg-sky-50 px-4 py-3",
    "text-sm text-sky-800",
  ].join(" "),
  bannerSuccess: [
    "rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3",
    "text-sm text-emerald-800",
  ].join(" "),
  chipRow: "flex flex-wrap gap-2",
  chip: [
    "rounded-full bg-slate-100 px-3 py-1",
    "text-xs font-medium text-slate-700",
  ].join(" "),
  warningChip: [
    "rounded-full bg-amber-100 px-3 py-1",
    "text-xs font-medium text-amber-800",
  ].join(" "),
  successChip: [
    "rounded-full bg-emerald-100 px-3 py-1",
    "text-xs font-medium text-emerald-800",
  ].join(" "),
  ratingButton: [
    "inline-flex items-center justify-center rounded-full border px-3 py-2",
    "text-xs font-medium transition",
  ].join(" "),
  ratingActive: "border-slate-950 bg-slate-950 text-white",
  ratingIdle: "border-slate-300 bg-white text-slate-700 hover:bg-slate-50",
  historyItem: "rounded-[28px] border border-slate-200 bg-slate-50 p-5",
  promptCard: [
    "rounded-2xl border border-slate-200 bg-white px-4 py-3",
    "text-sm text-slate-900",
  ].join(" "),
  summaryCard: [
    "rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-4",
    "text-sm leading-6 text-emerald-950",
  ].join(" "),
  metaGrid: "grid gap-3 md:grid-cols-3",
  metaTile: "rounded-2xl border border-slate-200 bg-white px-4 py-3",
  metaLabel: "text-xs uppercase tracking-[0.14em] text-slate-400",
  metaValue: "mt-1 text-sm font-medium text-slate-900",
  sqlBox: [
    "overflow-auto rounded-2xl border border-slate-200 bg-slate-950 p-4",
    "font-mono text-xs leading-6 text-emerald-200",
  ].join(" "),
  tableShell: "overflow-x-auto rounded-2xl border border-slate-200 bg-white",
  table: "min-w-full border-collapse text-left text-sm text-slate-700",
  th: [
    "border-b border-slate-200 bg-slate-50 px-4 py-3",
    "text-xs font-semibold uppercase tracking-[0.14em] text-slate-500",
  ].join(" "),
  td: "border-b border-slate-100 px-4 py-3 align-top",
  attemptItem: [
    "rounded-2xl border border-slate-200 bg-white px-4 py-4",
    "text-sm text-slate-700",
  ].join(" "),
  details: "rounded-2xl border border-slate-200 bg-white",
  detailsHeader:
    "cursor-pointer list-none px-4 py-3 text-sm font-medium text-slate-900",
  detailsBody: "border-t border-slate-200 px-4 py-4",
  exampleItem: "rounded-2xl border border-slate-200 bg-slate-50 p-4",
};

function normalizeError(err: unknown): string {
  return ConnectError.from(err).rawMessage;
}

function extractErrorResult(err: unknown): AskQuestionResponse | null {
  const connectErr = ConnectError.from(err);
  const [detail] = connectErr.findDetails(AskQuestionResponseSchema);
  return detail ?? null;
}

function historyCountLabel(
  count: number,
  locale: Locale,
  formattedCount: string,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (locale === "ko") {
    return t("chat.history.count.other", { count: formattedCount });
  }
  return count === 1
    ? t("chat.history.count.one", { count: formattedCount })
    : t("chat.history.count.other", { count: formattedCount });
}

function stageLabel(
  stage: string,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (stage) {
    case "generation":
      return t("chat.stage.generation");
    case "validation":
      return t("chat.stage.validation");
    case "execution":
      return t("chat.stage.execution");
    default:
      return stage;
  }
}

function renderCell(
  response: AskQuestionResponse,
  rowIndex: number,
  column: string,
): string {
  const value = response.rows[rowIndex]?.values[column];
  if (value === undefined || value === "") {
    return "NULL";
  }
  return value;
}

function timestampToMillis(ts: Timestamp | undefined): number | null {
  if (!ts) {
    return null;
  }
  const ms = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  if (!Number.isFinite(ms) || ms <= 0) {
    return null;
  }
  return ms;
}

function ratingButtonClass(active: boolean) {
  return [
    styles.ratingButton,
    active ? styles.ratingActive : styles.ratingIdle,
  ].join(" ");
}

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

type QueryResultCardProps = {
  item: QueryHistoryItem;
  canManageExamples: boolean;
  onSubmitFeedback: (args: {
    tenantId: string;
    queryRunId: string;
    rating: QueryFeedbackRating;
    comment: string;
    correctedSql: string;
  }) => Promise<void>;
  onCreateCanonicalExample: (args: {
    tenantId: string;
    queryRunId: string;
    question: string;
    sql: string;
    notes: string;
  }) => Promise<void>;
  onCanonicalExampleChanged: (tenantId: string) => Promise<void>;
};

function QueryResultCard({
  item,
  canManageExamples,
  onSubmitFeedback,
  onCreateCanonicalExample,
  onCanonicalExampleChanged,
}: QueryResultCardProps) {
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
      setExampleSql(correctedSql.trim() !== "" ? correctedSql : defaultExampleSql);
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

          <form onSubmit={handleFeedbackSubmit} className="mt-4 flex flex-col gap-4">
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
                className={ratingButtonClass(rating === QueryFeedbackRating.DOWN)}
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

          <form onSubmit={handleCreateExample} className="mt-4 flex flex-col gap-4">
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

type CanonicalExamplesPanelProps = {
  examples: CanonicalQueryExample[];
  canManage: boolean;
  loading: boolean;
  error: string | null;
  onArchive: (exampleId: string) => Promise<void>;
};

function CanonicalExamplesPanel({
  examples,
  canManage,
  loading,
  error,
  onArchive,
}: CanonicalExamplesPanelProps) {
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

export default function ChatPage() {
  const tenantClient = useTenantClient();
  const queryClient = useQueryClient();
  const { formatNumber, locale, t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();

  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [tenantsError, setTenantsError] = useState<string | null>(null);
  const [canonicalExamples, setCanonicalExamples] = useState<
    CanonicalQueryExample[]
  >([]);
  const [loadingExamples, setLoadingExamples] = useState(false);
  const [canonicalExamplesError, setCanonicalExamplesError] = useState<
    string | null
  >(null);
  const [viewerCanManageByTenant, setViewerCanManageByTenant] = useState<
    Record<string, boolean>
  >({});

  const defaultQuestion = t("chat.form.defaultQuestion");
  const previousDefaultQuestion = useRef(defaultQuestion);
  const [question, setQuestion] = useState(defaultQuestion);
  const [submitting, setSubmitting] = useState(false);
  const [history, setHistory] = useState<QueryHistoryItem[]>([]);
  const autoSubmittedRef = useRef(false);
  const requestedTenantID = searchParams.get("tenant");

  useEffect(() => {
    setQuestion((current) =>
      current === previousDefaultQuestion.current ? defaultQuestion : current,
    );
    previousDefaultQuestion.current = defaultQuestion;
  }, [defaultQuestion]);

  useEffect(() => {
    if (searchParams.get("auto") !== "1") {
      autoSubmittedRef.current = false;
    }
  }, [searchParams]);

  const selectedTenant = useMemo(
    () => tenants.find((tenant) => tenant.id === selectedID) ?? null,
    [selectedID, tenants],
  );

  const loadTenants = useCallback(async () => {
    try {
      const resp = await tenantClient.listTenants({});
      setTenants(resp.tenants);
      setTenantsError(null);
      if (!selectedID && resp.tenants.length > 0) {
        const requestedTenant =
          requestedTenantID != null
            ? resp.tenants.find((tenant) => tenant.id === requestedTenantID)
            : undefined;
        setSelectedID(requestedTenant?.id ?? resp.tenants[0].id);
      }
    } catch (err) {
      setTenantsError(normalizeError(err));
    }
  }, [requestedTenantID, selectedID, tenantClient]);

  useEffect(() => {
    void loadTenants();
  }, [loadTenants]);

  const loadCanonicalExamples = useCallback(
    async (tenantID: string) => {
      setLoadingExamples(true);
      try {
        const response = await queryClient.listCanonicalQueryExamples({
          tenantId: tenantID,
        });
        setCanonicalExamples(response.examples);
        setCanonicalExamplesError(null);
        setViewerCanManageByTenant((current) => ({
          ...current,
          [tenantID]: response.viewerCanManage,
        }));
      } catch (err) {
        setCanonicalExamples([]);
        setCanonicalExamplesError(normalizeError(err));
      } finally {
        setLoadingExamples(false);
      }
    },
    [queryClient],
  );

  useEffect(() => {
    if (!selectedID) {
      setCanonicalExamples([]);
      setCanonicalExamplesError(null);
      return;
    }
    void loadCanonicalExamples(selectedID);
  }, [loadCanonicalExamples, selectedID]);

  const submitQuestion = useCallback(
    async (rawQuestion?: string) => {
      const trimmedQuestion = (rawQuestion ?? question).trim();
      if (!selectedTenant || trimmedQuestion === "" || submitting) {
        return;
      }

      setSubmitting(true);
      try {
        const response = await queryClient.askQuestion({
          tenantId: selectedTenant.id,
          question: trimmedQuestion,
        });
        setHistory((current) => [
          {
            id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
            tenantId: selectedTenant.id,
            tenantName: selectedTenant.name,
            question: trimmedQuestion,
            createdAt: Date.now(),
            status: "success",
            response,
            error: null,
          },
          ...current,
        ]);
        setQuestion("");
      } catch (err) {
        setHistory((current) => [
          {
            id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
            tenantId: selectedTenant.id,
            tenantName: selectedTenant.name,
            question: trimmedQuestion,
            createdAt: Date.now(),
            status: "error",
            response: extractErrorResult(err),
            error: normalizeError(err),
          },
          ...current,
        ]);
      } finally {
        setSubmitting(false);
      }
    },
    [queryClient, question, selectedTenant, submitting],
  );

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitQuestion();
  };

  const runQuestion = useCallback(
    (text: string) => {
      setQuestion(text);
      void submitQuestion(text);
    },
    [submitQuestion],
  );

  useEffect(() => {
    const auto = searchParams.get("auto");
    const queuedQuestion = searchParams.get("q")?.trim() ?? "";
    if (
      auto !== "1" ||
      queuedQuestion === "" ||
      selectedTenant == null ||
      autoSubmittedRef.current
    ) {
      return;
    }

    autoSubmittedRef.current = true;
    setQuestion(queuedQuestion);
    void submitQuestion(queuedQuestion).finally(() => {
      const nextParams = new URLSearchParams(searchParams);
      nextParams.delete("auto");
      nextParams.delete("q");
      nextParams.delete("tenant");
      setSearchParams(nextParams, { replace: true });
    });
  }, [searchParams, selectedTenant, setSearchParams, submitQuestion]);

  const submitFeedback = useCallback(
    async (args: {
      tenantId: string;
      queryRunId: string;
      rating: QueryFeedbackRating;
      comment: string;
      correctedSql: string;
    }) => {
      await queryClient.submitQueryFeedback({
        tenantId: args.tenantId,
        queryRunId: args.queryRunId,
        rating: args.rating,
        comment: args.comment,
        correctedSql: args.correctedSql,
      });
    },
    [queryClient],
  );

  const createCanonicalExample = useCallback(
    async (args: {
      tenantId: string;
      queryRunId: string;
      question: string;
      sql: string;
      notes: string;
    }) => {
      await queryClient.createCanonicalQueryExample({
        tenantId: args.tenantId,
        queryRunId: args.queryRunId,
        question: args.question,
        sql: args.sql,
        notes: args.notes,
      });
    },
    [queryClient],
  );

  const archiveCanonicalExample = useCallback(
    async (exampleId: string) => {
      if (!selectedID) {
        return;
      }
      await queryClient.archiveCanonicalQueryExample({
        tenantId: selectedID,
        exampleId,
      });
      await loadCanonicalExamples(selectedID);
    },
    [loadCanonicalExamples, queryClient, selectedID],
  );

  const canSubmit =
    selectedTenant !== null &&
    question.trim() !== "" &&
    !submitting;

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>{t("common.appLabel")}</p>
        <h1 className="text-3xl font-semibold tracking-tight">
          {t("chat.hero.title")}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">
          {t("chat.hero.subtitle")}
        </p>
      </section>

      <div className={styles.grid}>
        <aside>
          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className="text-lg font-semibold">
                  {t("chat.tenants.title")}
                </h2>
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
                        onClick={() => setSelectedID(tenant.id)}
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

          {selectedID ? (
            <CanonicalExamplesPanel
              examples={canonicalExamples}
              canManage={viewerCanManageByTenant[selectedID] ?? false}
              loading={loadingExamples}
              error={canonicalExamplesError}
              onArchive={archiveCanonicalExample}
            />
          ) : null}
        </aside>

        <div className="flex flex-col gap-6">
          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className="text-lg font-semibold">
                  {t("chat.form.title")}
                </h2>
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

            <form onSubmit={handleSubmit} className="mt-5 flex flex-col gap-4">
              {history.length === 0 && selectedTenant ? (
                <StarterQuestions
                  tenantId={selectedTenant.id}
                  onPick={runQuestion}
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
                onChange={(event) => setQuestion(event.target.value)}
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
                  {submitting
                    ? t("chat.form.submitting")
                    : t("chat.form.submit")}
                </button>
              </div>
            </form>
          </section>

          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className="text-lg font-semibold">
                  {t("chat.history.title")}
                </h2>
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
                    canManageExamples={
                      viewerCanManageByTenant[item.tenantId] ?? false
                    }
                    onSubmitFeedback={submitFeedback}
                    onCreateCanonicalExample={createCanonicalExample}
                    onCanonicalExampleChanged={loadCanonicalExamples}
                  />
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}
