import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { ConnectError } from "@connectrpc/connect";

import {
  AskQuestionResponseSchema,
  QueryPromptContextSource,
  QueryRunStatus,
  type AskQuestionResponse,
} from "../../gen/query/v1/query_pb";
import { type Locale, useI18n } from "../../lib/i18n";

export type QueryHistoryItem = {
  id: string;
  tenantId: string;
  tenantName: string;
  question: string;
  createdAt: number;
  status: "success" | "error";
  response: AskQuestionResponse | null;
  error: string | null;
};

export type SubmitQueryFeedbackArgs = {
  tenantId: string;
  queryRunId: string;
  rating: number;
  comment: string;
  correctedSql: string;
};

export type CreateCanonicalExampleArgs = {
  tenantId: string;
  queryRunId: string;
  question: string;
  sql: string;
  notes: string;
};

export const styles = {
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
} as const;

export function extractErrorResult(err: unknown): AskQuestionResponse | null {
  const connectErr = ConnectError.from(err);
  const [detail] = connectErr.findDetails(AskQuestionResponseSchema);
  return detail ?? null;
}

export function historyCountLabel(
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

export function stageLabel(
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

export function renderCell(
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

export function timestampToMillis(ts: Timestamp | undefined): number | null {
  if (!ts) {
    return null;
  }
  const ms = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  if (!Number.isFinite(ms) || ms <= 0) {
    return null;
  }
  return ms;
}

export function ratingButtonClass(active: boolean) {
  return [
    styles.ratingButton,
    active ? styles.ratingActive : styles.ratingIdle,
  ].join(" ");
}

export function queryRunStatusChipClass(status: QueryRunStatus) {
  switch (status) {
    case QueryRunStatus.SUCCEEDED:
      return styles.successChip;
    case QueryRunStatus.FAILED:
      return styles.warningChip;
    default:
      return styles.chip;
  }
}

export function queryRunStatusLabel(
  status: QueryRunStatus,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (status) {
    case QueryRunStatus.RUNNING:
      return t("chat.persistent.status.running");
    case QueryRunStatus.SUCCEEDED:
      return t("chat.persistent.status.succeeded");
    case QueryRunStatus.FAILED:
      return t("chat.persistent.status.failed");
    default:
      return t("common.unknown");
  }
}

export function queryPromptContextLabel(
  source: QueryPromptContextSource,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (source) {
    case QueryPromptContextSource.APPROVED:
      return t("chat.persistent.source.approved");
    case QueryPromptContextSource.DRAFT:
      return t("chat.persistent.source.draft");
    case QueryPromptContextSource.RAW_SCHEMA:
      return t("chat.persistent.source.rawSchema");
    default:
      return t("common.unknown");
  }
}
