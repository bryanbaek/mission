import { ConnectError } from "@connectrpc/connect";

import type { SemanticLayerContent } from "../../gen/semantic/v1/semantic_pb";
import type { OnboardingState } from "../../gen/onboarding/v1/onboarding_pb";
import type { useI18n } from "../../lib/i18n";

export const styles = {
  shell: "mx-auto flex min-h-screen max-w-6xl flex-col gap-6 px-4 py-8 sm:px-6",
  hero: [
    "rounded-[32px] border border-slate-200 bg-white p-8 shadow-sm",
    "flex flex-col gap-4",
  ].join(" "),
  card: "rounded-[32px] border border-slate-200 bg-white p-6 shadow-sm",
  bannerError: [
    "rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3",
    "text-sm text-rose-700",
  ].join(" "),
  bannerSuccess: [
    "rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3",
    "text-sm text-emerald-800",
  ].join(" "),
  buttonPrimary: [
    "inline-flex items-center justify-center rounded-xl bg-slate-950",
    "px-4 py-2 text-sm font-medium text-white transition",
    "hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-300",
  ].join(" "),
  buttonSecondary: [
    "inline-flex items-center justify-center rounded-xl border border-slate-300",
    "bg-white px-4 py-2 text-sm font-medium text-slate-700 transition",
    "hover:bg-slate-50 disabled:cursor-not-allowed disabled:text-slate-300",
  ].join(" "),
  progressTrack: "h-2 overflow-hidden rounded-full bg-slate-200",
  progressFill: "h-full rounded-full bg-emerald-500 transition-all",
  codeBlock: [
    "overflow-x-auto rounded-3xl border border-slate-200 bg-slate-950 p-4",
    "font-mono text-xs leading-6 text-emerald-200 whitespace-pre-wrap break-all",
  ].join(" "),
  textarea: [
    "min-h-[160px] w-full rounded-2xl border border-slate-300 px-4 py-3",
    "text-sm leading-6 text-slate-900 focus:border-slate-950 focus:outline-none",
  ].join(" "),
  input: [
    "w-full rounded-xl border border-slate-300 px-3 py-2 text-sm text-slate-900",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
};

export function onboardingErrorMessage(
  err: unknown,
  t: ReturnType<typeof useI18n>["t"],
  fallbackKey:
    | "onboarding.error.stateLoad"
    | "onboarding.error.welcome"
    | "onboarding.error.install"
    | "onboarding.error.status"
    | "onboarding.error.database"
    | "onboarding.error.schema"
    | "onboarding.error.semantic"
    | "onboarding.error.invites"
    | "onboarding.error.complete",
): string {
  const message = ConnectError.from(err).rawMessage;

  switch (message) {
    case "not a member of this tenant":
    case "owner role required":
    case "unauthenticated":
      return t("onboarding.error.permission");
    case "workspace name is required":
      return fallbackKey === "onboarding.error.welcome"
        ? t("onboarding.error.workspaceNameRequired")
        : t("onboarding.error.database");
    case "primary language must be confirmed as Korean":
      return fallbackKey === "onboarding.error.welcome"
        ? t("onboarding.error.primaryLanguage")
        : t(fallbackKey);
    case "one or more invite emails are invalid":
      return t("onboarding.error.inviteEmail");
    case "onboarding step is not available yet":
      return t("onboarding.error.invalidStep");
    case "earlier onboarding steps are locked after approval":
      return t("onboarding.error.stepLocked");
    case "semantic layer is not approved":
      return t("onboarding.error.semanticNotApproved");
    default:
      return t(fallbackKey);
  }
}

export function deepCloneContent(
  content: SemanticLayerContent,
): SemanticLayerContent {
  return JSON.parse(JSON.stringify(content)) as SemanticLayerContent;
}

export function serializeContent(
  content: SemanticLayerContent | null,
): string {
  return JSON.stringify(content ?? null);
}

type TimestampLike =
  | OnboardingState["updatedAt"]
  | OnboardingState["dbVerifiedAt"]
  | OnboardingState["agentConnectedAt"]
  | OnboardingState["semanticApprovedAt"];

export function formatTimestamp(value: TimestampLike): string {
  if (!value) {
    return "";
  }
  const ms = Number(value.seconds) * 1000 + Math.floor(value.nanos / 1_000_000);
  if (!Number.isFinite(ms) || ms <= 0) {
    return "";
  }
  return new Intl.DateTimeFormat("ko-KR", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(ms));
}

export function buildMySQLConnectionString(args: {
  username: string;
  password: string;
  host: string;
  port: string;
  databaseName: string;
}): string {
  const username = args.username.trim();
  const password = args.password.trim();
  const host = args.host.trim();
  const databaseName = args.databaseName.trim();
  const parsedPort = Number.parseInt(args.port, 10);
  const port =
    Number.isFinite(parsedPort) && parsedPort > 0 ? String(parsedPort) : "3306";

  if (!username || !password || !host || !databaseName) {
    return "";
  }

  return `${username}:${password}@tcp(${host}:${port})/${databaseName}`;
}

export function copyText(text: string) {
  if (!navigator.clipboard) {
    return Promise.reject(new Error("clipboard is not available"));
  }
  return navigator.clipboard.writeText(text);
}
