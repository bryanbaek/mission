import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { ConnectError } from "@connectrpc/connect";
import { Link, useNavigate, useParams } from "react-router-dom";

import type {
  SemanticLayerContent,
  GetSemanticLayerResponse,
} from "../../gen/semantic/v1/semantic_pb";
import type { OnboardingState } from "../../gen/onboarding/v1/onboarding_pb";
import SemanticLayerEditor from "../SemanticLayerEditor";
import StarterQuestions from "../StarterQuestions";
import { useI18n } from "../../lib/i18n";
import { onboardingStepPath } from "../../lib/onboarding";
import { useOnboardingClient } from "../../lib/onboardingClient";
import { useSemanticClient } from "../../lib/semanticClient";
import LocaleToggle from "./LocaleToggle";

const styles = {
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

function onboardingErrorMessage(
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

function deepCloneContent(content: SemanticLayerContent): SemanticLayerContent {
  return JSON.parse(JSON.stringify(content)) as SemanticLayerContent;
}

function serializeContent(content: SemanticLayerContent | null): string {
  return JSON.stringify(content ?? null);
}

function formatTimestamp(
  value: OnboardingState["updatedAt"] | OnboardingState["dbVerifiedAt"] | OnboardingState["agentConnectedAt"] | OnboardingState["semanticApprovedAt"],
): string {
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

function StepFrame({
  step,
  title,
  subtitle,
  justHappened,
  nextText,
  backHref,
  children,
}: {
  step: number;
  title: string;
  subtitle: string;
  justHappened: string;
  nextText: string;
  backHref?: string;
  children: ReactNode;
}) {
  const { t } = useI18n();
  const progress = `${(step / 7) * 100}%`;

  return (
    <div className={styles.shell}>
      <section className={styles.hero}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-slate-500">
              {t("onboarding.common.stepOfTotal", {
                step,
                total: 7,
              })}
            </p>
            <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-900">
              {title}
            </h1>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            {backHref ? (
              <Link to={backHref} className={styles.buttonSecondary}>
                {t("onboarding.common.back")}
              </Link>
            ) : null}
            <LocaleToggle />
          </div>
        </div>
        <div className={styles.progressTrack}>
          <div className={styles.progressFill} style={{ width: progress }} />
        </div>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">{subtitle}</p>
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="rounded-3xl bg-slate-50 p-4">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">
              {t("onboarding.common.justHappenedLabel")}
            </p>
            <p className="mt-2 text-sm leading-6 text-slate-700">{justHappened}</p>
          </div>
          <div className="rounded-3xl bg-emerald-50 p-4">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-emerald-700">
              {t("onboarding.common.nextLabel")}
            </p>
            <p className="mt-2 text-sm leading-6 text-emerald-900">{nextText}</p>
          </div>
        </div>
      </section>
      {children}
    </div>
  );
}

function copyText(text: string) {
  if (!navigator.clipboard) {
    return Promise.reject(new Error("clipboard is not available"));
  }
  return navigator.clipboard.writeText(text);
}

function SemanticReviewStep({
  tenantId,
  onApproved,
}: {
  tenantId: string;
  onApproved: (nextState: OnboardingState) => void;
}) {
  const semanticClient = useSemanticClient();
  const onboardingClient = useOnboardingClient();
  const { t } = useI18n();

  const [response, setResponse] = useState<GetSemanticLayerResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [pageError, setPageError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [formContent, setFormContent] = useState<SemanticLayerContent | null>(
    null,
  );
  const [savedContent, setSavedContent] = useState<SemanticLayerContent | null>(
    null,
  );
  const [drafting, setDrafting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [approving, setApproving] = useState(false);

  const dirty = useMemo(
    () => serializeContent(formContent) !== serializeContent(savedContent),
    [formContent, savedContent],
  );

  const applyLoadedResponse = useCallback((next: GetSemanticLayerResponse) => {
    setResponse(next);
    if (next.currentLayer?.content) {
      const cloned = deepCloneContent(next.currentLayer.content);
      setFormContent(cloned);
      setSavedContent(deepCloneContent(next.currentLayer.content));
    } else {
      setFormContent(null);
      setSavedContent(null);
    }
  }, []);

  const loadSemantic = useCallback(async () => {
    setLoading(true);
    try {
      const result = await semanticClient.getSemanticLayer({ tenantId });
      applyLoadedResponse(result);
      setPageError(null);
    } catch {
      setPageError(t("onboarding.error.semantic"));
    } finally {
      setLoading(false);
    }
  }, [applyLoadedResponse, semanticClient, t, tenantId]);

  useEffect(() => {
    void loadSemantic();
  }, [loadSemantic]);

  const updateTableDescription = (tableIndex: number, description: string) => {
    setFormContent((current) => {
      if (!current) {
        return current;
      }
      const next = deepCloneContent(current);
      next.tables[tableIndex].description = description;
      return next;
    });
  };

  const updateColumnDescription = (
    tableIndex: number,
    columnIndex: number,
    description: string,
  ) => {
    setFormContent((current) => {
      if (!current) {
        return current;
      }
      const next = deepCloneContent(current);
      next.tables[tableIndex].columns[columnIndex].description = description;
      return next;
    });
  };

  const handleDraft = async () => {
    if (!response?.latestSchema?.id) {
      return;
    }
    setDrafting(true);
    setPageError(null);
    setNotice(null);
    setSuccess(null);
    try {
      const result = await semanticClient.draftSemanticLayer({
        tenantId,
        schemaVersionId: response.latestSchema.id,
      });
      await loadSemantic();
      setSuccess(t("semantic.success.draftCreated"));
      if (result.usage) {
        setNotice(
          `${t("semantic.notice.cacheUsage")}: ${result.usage.provider} / ${result.usage.model}`,
        );
      }
    } catch {
      setPageError(t("onboarding.error.semantic"));
    } finally {
      setDrafting(false);
    }
  };

  const saveDraft = useCallback(async (showSuccess: boolean) => {
    if (!response?.currentLayer?.id || !formContent) {
      return null;
    }
    setSaving(true);
    setPageError(null);
    try {
      const result = await semanticClient.updateSemanticLayer({
        tenantId,
        id: response.currentLayer.id,
        content: formContent,
      });
      await loadSemantic();
      if (showSuccess) {
        setSuccess(t("semantic.success.saved"));
      }
      return result.layer?.id ?? null;
    } catch {
      setPageError(t("onboarding.error.semantic"));
      return null;
    } finally {
      setSaving(false);
    }
  }, [formContent, loadSemantic, response?.currentLayer?.id, semanticClient, t, tenantId]);

  const handleApprove = async () => {
    if (!response?.currentLayer?.id) {
      return;
    }
    setApproving(true);
    setPageError(null);
    try {
      let layerId = response.currentLayer.id;
      if (dirty) {
        const savedId = await saveDraft(false);
        if (!savedId) {
          return;
        }
        layerId = savedId;
      }
      await semanticClient.approveSemanticLayer({ tenantId, id: layerId });
      const result = await onboardingClient.markSemanticApproved({
        tenantId,
        semanticLayerId: layerId,
      });
      setSuccess(t("semantic.success.approved"));
      onApproved(result.state!);
    } catch {
      setPageError(t("onboarding.error.semantic"));
    } finally {
      setApproving(false);
    }
  };

  return (
    <>
      {pageError ? <div className={styles.bannerError}>{pageError}</div> : null}
      {notice ? <div className={styles.bannerSuccess}>{notice}</div> : null}
      {success ? <div className={styles.bannerSuccess}>{success}</div> : null}
      <SemanticLayerEditor
        loading={loading}
        response={response}
        formContent={formContent}
        dirty={dirty}
        drafting={drafting}
        saving={saving}
        approving={approving}
        onDraft={() => void handleDraft()}
        onSave={() => void saveDraft(true)}
        onApprove={() => void handleApprove()}
        onUpdateTableDescription={updateTableDescription}
        onUpdateColumnDescription={updateColumnDescription}
      />
    </>
  );
}

export default function OnboardingStepScreen({ step }: { step: number }) {
  const params = useParams();
  const tenantId = params.tenantId;
  const onboardingClient = useOnboardingClient();
  const { t } = useI18n();
  const navigate = useNavigate();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [state, setState] = useState<OnboardingState | null>(null);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const [workspaceName, setWorkspaceName] = useState("");
  const [confirmedLanguage, setConfirmedLanguage] = useState(true);
  const step1AutosaveReady = useRef(false);

  const [connectionString, setConnectionString] = useState("");
  const [dbHost, setDbHost] = useState("");
  const [dbPort, setDbPort] = useState("3306");
  const [dbName, setDbName] = useState("");
  const [dbSubmitting, setDbSubmitting] = useState(false);

  const [schemaSubmitting, setSchemaSubmitting] = useState(false);
  const [holdSchemaSummary, setHoldSchemaSummary] = useState(false);
  const [runningStep7, setRunningStep7] = useState(false);
  const [inviteModalOpen, setInviteModalOpen] = useState(false);
  const [inviteEmails, setInviteEmails] = useState("");
  const [inviteSubmitting, setInviteSubmitting] = useState(false);

  const loadState = useCallback(async () => {
    if (!tenantId) {
      return;
    }
    setLoading(true);
    try {
      const response = await onboardingClient.getState({ tenantId });
      setState(response.state ?? null);
      setError(null);
    } catch (err) {
      setError(onboardingErrorMessage(err, t, "onboarding.error.stateLoad"));
    } finally {
      setLoading(false);
    }
  }, [onboardingClient, t, tenantId]);

  useEffect(() => {
    void loadState();
  }, [loadState]);

  useEffect(() => {
    if (!state) {
      return;
    }
    setWorkspaceName(state.name);
    setConfirmedLanguage(state.primaryLanguage === "ko" || state.primaryLanguage === "");
    setDbHost(state.dbHost);
    setDbPort(state.dbPort > 0 ? String(state.dbPort) : "3306");
    setDbName(state.dbName);
  }, [state]);

  useEffect(() => {
    if (!tenantId || step !== 1 || !state) {
      return;
    }
    if (!step1AutosaveReady.current) {
      step1AutosaveReady.current = true;
      return;
    }
    const timer = window.setTimeout(() => {
      void onboardingClient
        .saveWelcome({
          tenantId,
          workspaceName,
          primaryLanguage: "ko",
          confirmPrimaryLanguage: false,
          autoSave: true,
        })
        .then((response) => {
          setState(response.state ?? null);
        })
        .catch((err) => {
          setError(onboardingErrorMessage(err, t, "onboarding.error.welcome"));
        });
    }, 500);
    return () => window.clearTimeout(timer);
  }, [onboardingClient, state, step, t, tenantId, workspaceName]);

  useEffect(() => {
    if (!tenantId || step !== 2) {
      return;
    }
    let cancelled = false;

    const ensureBundle = async () => {
      try {
        const response = await onboardingClient.ensureInstallBundle({ tenantId });
        if (cancelled) {
          return;
        }
        setState(response.state ?? null);
        setError(null);
      } catch (err) {
        if (!cancelled) {
          setError(onboardingErrorMessage(err, t, "onboarding.error.install"));
        }
      }
    };

    void ensureBundle();
    const interval = window.setInterval(() => {
      void onboardingClient
        .getAgentConnectionStatus({ tenantId })
        .then((response) => {
          if (cancelled) {
            return;
          }
          const nextState = response.state ?? null;
          setState(nextState);
          if (nextState?.agentConnected) {
            navigate(onboardingStepPath(tenantId, 3), { replace: true });
          }
        })
        .catch((err) => {
          if (!cancelled) {
            setError(onboardingErrorMessage(err, t, "onboarding.error.status"));
          }
        });
    }, 5000);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [navigate, onboardingClient, step, t, tenantId]);

  useEffect(() => {
    if (!tenantId || !state || state.waitingForOwner) {
      return;
    }
    if (state.currentStep && state.currentStep < step) {
      navigate(onboardingStepPath(tenantId, state.currentStep), { replace: true });
    }
  }, [navigate, state, step, tenantId]);

  const stepMeta = useMemo(() => {
    const key = `onboarding.step${step}` as const;
    return {
      title: t(`${key}.title` as never),
      subtitle: t(`${key}.subtitle` as never),
      justHappened: t(`${key}.justHappened` as never),
      nextText: t(`${key}.next` as never),
    };
  }, [step, t]);

  const handleCopy = async (key: string, text: string) => {
    try {
      await copyText(text);
      setCopiedKey(key);
      setSuccess(t("onboarding.common.copied"));
      window.setTimeout(() => setCopiedKey(null), 1500);
    } catch {
      setError(t("onboarding.error.generic"));
    }
  };

  if (!tenantId) {
    return null;
  }

  if (loading || !state) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-100 p-6 text-sm text-slate-600">
        {t("onboarding.common.loading")}
      </div>
    );
  }

  if (state.waitingForOwner) {
    return (
      <StepFrame
        step={step}
        title={t("onboarding.waiting.title")}
        subtitle={t("onboarding.waiting.subtitle")}
        justHappened={t("onboarding.waiting.justHappened")}
        nextText={t("onboarding.waiting.next")}
      >
        <section className={styles.card}>
          <p className="text-sm leading-6 text-slate-700">
            {t("onboarding.waiting.body", { workspace: state.name })}
          </p>
          <p className="mt-4 text-xs text-slate-500">
            {t("onboarding.waiting.updatedAt", {
              time: formatTimestamp(state.updatedAt) || t("common.na"),
            })}
          </p>
        </section>
      </StepFrame>
    );
  }

  const backHref =
    step > 1 && step < 6 ? onboardingStepPath(tenantId, step - 1) : undefined;

  return (
    <StepFrame
      step={step}
      title={stepMeta.title}
      subtitle={stepMeta.subtitle}
      justHappened={stepMeta.justHappened}
      nextText={stepMeta.nextText}
      backHref={backHref}
    >
      {error ? <div className={styles.bannerError}>{error}</div> : null}
      {success ? <div className={styles.bannerSuccess}>{success}</div> : null}

      {step === 1 ? (
        <section className={styles.card}>
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_280px]">
            <div className="space-y-4">
              <label className="grid gap-2 text-sm font-medium text-slate-800">
                <span>{t("onboarding.step1.workspaceName")}</span>
                <input
                  className={styles.input}
                  value={workspaceName}
                  onChange={(event) => setWorkspaceName(event.target.value)}
                  placeholder={t("onboarding.step1.workspacePlaceholder")}
                />
              </label>
              <label className="flex items-start gap-3 rounded-3xl border border-slate-200 bg-slate-50 p-4">
                <input
                  type="checkbox"
                  checked={confirmedLanguage}
                  onChange={(event) => setConfirmedLanguage(event.target.checked)}
                  className="mt-1 h-4 w-4 rounded border-slate-300"
                />
                <span className="text-sm leading-6 text-slate-700">
                  {t("onboarding.step1.languageConfirm")}
                </span>
              </label>
            </div>
            <div className="rounded-3xl bg-slate-50 p-5 text-sm leading-6 text-slate-700">
              <p className="font-semibold text-slate-900">
                {t("onboarding.step1.summaryTitle")}
              </p>
              <p className="mt-3">{t("onboarding.step1.summaryBody")}</p>
            </div>
          </div>
          <div className="mt-6 flex justify-end">
            <button
              type="button"
              className={styles.buttonPrimary}
              onClick={() =>
                void onboardingClient
                  .saveWelcome({
                    tenantId,
                    workspaceName,
                    primaryLanguage: "ko",
                    confirmPrimaryLanguage: confirmedLanguage,
                    autoSave: false,
                  })
                  .then((response) => {
                    setState(response.state ?? null);
                    navigate(onboardingStepPath(tenantId, 2), { replace: true });
                  })
                  .catch((err) =>
                    setError(
                      onboardingErrorMessage(err, t, "onboarding.error.welcome"),
                    ),
                  )
              }
            >
              {t("onboarding.common.next")}
            </button>
          </div>
        </section>
      ) : null}

      {step === 2 ? (
        <>
          <section className={styles.card}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-slate-900">
                  {t("onboarding.step2.commandTitle")}
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t("onboarding.step2.commandBody")}
                </p>
              </div>
              <button
                type="button"
                className={styles.buttonSecondary}
                onClick={() => void handleCopy("docker", state.dockerRunCommand)}
              >
                {copiedKey === "docker"
                  ? t("onboarding.common.copied")
                  : t("onboarding.common.copy")}
              </button>
            </div>
            <pre className={`${styles.codeBlock} mt-4`}>
              <code>{state.dockerRunCommand}</code>
            </pre>
          </section>

          <section className={styles.card}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-slate-900">
                  {t("onboarding.step2.statusTitle")}
                </h2>
                <p className="mt-2 text-sm text-slate-600">
                  {state.agentConnected
                    ? t("onboarding.step2.statusConnected")
                    : t("onboarding.step2.statusWaiting")}
                </p>
              </div>
              <span
                className={[
                  "rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em]",
                  state.agentConnected
                    ? "bg-emerald-100 text-emerald-700"
                    : "bg-amber-100 text-amber-700",
                ].join(" ")}
              >
                {state.agentConnected
                  ? t("onboarding.common.connected")
                  : t("onboarding.common.pending")}
              </span>
            </div>
            {state.agentConnectedAt ? (
              <p className="mt-4 text-sm text-slate-600">
                {t("onboarding.step2.connectedAt", {
                  time: formatTimestamp(state.agentConnectedAt),
                })}
              </p>
            ) : null}
          </section>

          <section className={styles.card}>
            <details open={state.agentConnectionTimedOut}>
              <summary className="cursor-pointer text-lg font-semibold text-slate-900">
                {t("onboarding.step2.troubleshootingTitle")}
              </summary>
              <div className="mt-4 grid gap-4">
                {[1, 2, 3, 4, 5].map((index) => (
                  <article
                    key={index}
                    className="rounded-3xl border border-slate-200 bg-slate-50 p-4"
                  >
                    <p className="font-medium text-slate-900">
                      {t(`onboarding.step2.troubleshoot.${index}.title` as never)}
                    </p>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      {t(`onboarding.step2.troubleshoot.${index}.body` as never)}
                    </p>
                    <div className="mt-3 flex justify-end">
                  <button
                    type="button"
                    className={styles.buttonSecondary}
                    onClick={() =>
                      void handleCopy(
                        `trouble-${index}`,
                        t(`onboarding.step2.troubleshoot.${index}.fix` as never),
                      )
                    }
                  >
                        {t("onboarding.common.copyFix")}
                      </button>
                    </div>
                    <pre className={`${styles.codeBlock} mt-3`}>
                      <code>{t(`onboarding.step2.troubleshoot.${index}.fix` as never)}</code>
                    </pre>
                  </article>
                ))}
              </div>
            </details>
          </section>
        </>
      ) : null}

      {step === 3 ? (
        <>
          <section className={styles.card}>
            <div className="grid gap-4 md:grid-cols-3">
              <label className="grid gap-2 text-sm font-medium text-slate-800">
                <span>{t("onboarding.step3.host")}</span>
                <input
                  className={styles.input}
                  value={dbHost}
                  onChange={(event) => setDbHost(event.target.value)}
                  placeholder={t("onboarding.step3.hostPlaceholder")}
                />
              </label>
              <label className="grid gap-2 text-sm font-medium text-slate-800">
                <span>{t("onboarding.step3.port")}</span>
                <input
                  className={styles.input}
                  value={dbPort}
                  onChange={(event) => setDbPort(event.target.value)}
                />
              </label>
              <label className="grid gap-2 text-sm font-medium text-slate-800">
                <span>{t("onboarding.step3.database")}</span>
                <input
                  className={styles.input}
                  value={dbName}
                  onChange={(event) => setDbName(event.target.value)}
                  placeholder={t("onboarding.step3.databasePlaceholder")}
                />
              </label>
            </div>
            <div className="mt-6 flex justify-end">
              <button
                type="button"
                className={styles.buttonPrimary}
                disabled={dbSubmitting}
                onClick={() => {
                  setDbSubmitting(true);
                  void onboardingClient
                    .configureDatabase({
                      tenantId,
                      host: dbHost,
                      port: Number.parseInt(dbPort, 10) || 3306,
                      databaseName: dbName,
                    })
                    .then((response) => {
                      setState(response.state ?? null);
                      setError(null);
                    })
                    .catch((err) =>
                      setError(
                        onboardingErrorMessage(
                          err,
                          t,
                          "onboarding.error.database",
                        ),
                      ),
                    )
                    .finally(() => setDbSubmitting(false));
                }}
              >
                {t("onboarding.step3.generateSql")}
              </button>
            </div>
          </section>

          <section className={styles.card}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-slate-900">
                  {t("onboarding.step3.sqlTitle")}
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t("onboarding.step3.sqlBody")}
                </p>
              </div>
              <button
                type="button"
                className={styles.buttonSecondary}
                onClick={() => void handleCopy("sql", state.dbSetupSql)}
              >
                {copiedKey === "sql"
                  ? t("onboarding.common.copied")
                  : t("onboarding.common.copy")}
              </button>
            </div>
            <pre className={`${styles.codeBlock} mt-4`}>
              <code>{state.dbSetupSql}</code>
            </pre>
            <div className="mt-4 grid gap-3 lg:grid-cols-3">
              <div className="rounded-3xl bg-slate-50 p-4 text-sm leading-6 text-slate-700">
                {t("onboarding.step3.whyReadOnly")}
              </div>
              <div className="rounded-3xl bg-slate-50 p-4 text-sm leading-6 text-slate-700">
                {t("onboarding.step3.whySeparateUser")}
              </div>
              <div className="rounded-3xl bg-slate-50 p-4 text-sm leading-6 text-slate-700">
                {t("onboarding.step3.whyReplica")}
              </div>
            </div>
            <div className="mt-4 rounded-3xl border border-sky-200 bg-sky-50 p-4 text-sm leading-6 text-sky-900">
              {t("onboarding.step3.transport")}
            </div>
          </section>

          <section className={styles.card}>
            <label className="grid gap-2 text-sm font-medium text-slate-800">
              <span>{t("onboarding.step3.connectionString")}</span>
              <textarea
                className={styles.textarea}
                value={connectionString}
                onChange={(event) => setConnectionString(event.target.value)}
                placeholder={t("onboarding.step3.connectionPlaceholder")}
              />
            </label>
            {state.dbErrorMessageKo ? (
              <div className={`${styles.bannerError} mt-4`}>
                {state.dbErrorMessageKo}
              </div>
            ) : null}
            {state.dbVerifiedAt ? (
              <div className={`${styles.bannerSuccess} mt-4`}>
                {t("onboarding.step3.verifiedAt", {
                  time: formatTimestamp(state.dbVerifiedAt),
                })}
              </div>
            ) : null}
            <div className="mt-6 flex justify-end">
              <button
                type="button"
                className={styles.buttonPrimary}
                disabled={dbSubmitting}
                onClick={() => {
                  setDbSubmitting(true);
                  void onboardingClient
                    .configureDatabase({
                      tenantId,
                      host: dbHost,
                      port: Number.parseInt(dbPort, 10) || 3306,
                      databaseName: dbName,
                      connectionString,
                    })
                    .then((response) => {
                      const nextState = response.state ?? null;
                      setState(nextState);
                      if (nextState?.currentStep === 4) {
                        navigate(onboardingStepPath(tenantId, 4), {
                          replace: true,
                        });
                      }
                    })
                    .catch((err) =>
                      setError(
                        onboardingErrorMessage(
                          err,
                          t,
                          "onboarding.error.database",
                        ),
                      ),
                    )
                    .finally(() => setDbSubmitting(false));
                }}
              >
                {t("onboarding.step3.verify")}
              </button>
            </div>
          </section>
        </>
      ) : null}

      {step === 4 ? (
        <>
          <section className={styles.card}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-slate-900">
                  {t("onboarding.step4.title")}
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t("onboarding.step4.body")}
                </p>
              </div>
              <button
                type="button"
                className={styles.buttonPrimary}
                disabled={schemaSubmitting}
                onClick={() => {
                  setSchemaSubmitting(true);
                  void onboardingClient
                    .runSchemaIntrospection({ tenantId })
                    .then((response) => {
                      setState(response.state ?? null);
                      setHoldSchemaSummary(true);
                    })
                    .catch((err) =>
                      setError(
                        onboardingErrorMessage(err, t, "onboarding.error.schema"),
                      ),
                    )
                    .finally(() => setSchemaSubmitting(false));
                }}
              >
                {schemaSubmitting
                  ? t("onboarding.step4.running")
                  : t("onboarding.step4.run")}
              </button>
            </div>
          </section>

          {(holdSchemaSummary || !!state.schemaVersionId) && (
            <section className={styles.card}>
              <div className="grid gap-4 md:grid-cols-3">
                <div className="rounded-3xl bg-slate-50 p-5">
                  <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                    {t("onboarding.step4.tables")}
                  </p>
                  <p className="mt-2 text-2xl font-semibold text-slate-900">
                    {state.schemaTableCount}
                  </p>
                </div>
                <div className="rounded-3xl bg-slate-50 p-5">
                  <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                    {t("onboarding.step4.columns")}
                  </p>
                  <p className="mt-2 text-2xl font-semibold text-slate-900">
                    {state.schemaColumnCount}
                  </p>
                </div>
                <div className="rounded-3xl bg-slate-50 p-5">
                  <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                    {t("onboarding.step4.foreignKeys")}
                  </p>
                  <p className="mt-2 text-2xl font-semibold text-slate-900">
                    {state.schemaForeignKeyCount}
                  </p>
                </div>
              </div>
              <div className="mt-6 flex justify-end">
                <button
                  type="button"
                  className={styles.buttonPrimary}
                  onClick={() => navigate(onboardingStepPath(tenantId, 5))}
                >
                  {t("onboarding.common.next")}
                </button>
              </div>
            </section>
          )}
        </>
      ) : null}

      {step === 5 ? (
        <SemanticReviewStep
          tenantId={tenantId}
          onApproved={(nextState) => {
            setState(nextState);
            navigate(onboardingStepPath(tenantId, 6), { replace: true });
          }}
        />
      ) : null}

      {step === 6 ? (
        <section className={styles.card}>
          <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
            <div>
              <h2 className="text-lg font-semibold text-slate-900">
                {t("onboarding.step6.readyTitle")}
              </h2>
              <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-700">
                {t("onboarding.step6.readyBody")}
              </p>
              <div className="mt-6">
                <StarterQuestions
                  tenantId={tenantId}
                  onPick={(text) =>
                    navigate(
                      `/chat?tenant=${encodeURIComponent(tenantId)}&q=${encodeURIComponent(text)}&auto=1`,
                    )
                  }
                />
              </div>
            </div>
            <div className="rounded-3xl bg-emerald-50 p-5">
              <p className="text-sm leading-6 text-emerald-900">
                {t("onboarding.step6.next")}
              </p>
              <button
                type="button"
                className={`${styles.buttonPrimary} mt-4 w-full`}
                onClick={() =>
                  void onboardingClient
                    .completeStarterStep({ tenantId })
                    .then((response) => {
                      setState(response.state ?? null);
                      navigate(onboardingStepPath(tenantId, 7), {
                        replace: true,
                      });
                    })
                    .catch((err) =>
                      setError(
                        onboardingErrorMessage(
                          err,
                          t,
                          "onboarding.error.complete",
                        ),
                      ),
                    )
                }
              >
                {t("onboarding.step6.finish")}
              </button>
            </div>
          </div>
        </section>
      ) : null}

      {step === 7 ? (
        <>
          <section className={styles.card}>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-3xl bg-slate-50 p-5">
                <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                  {t("onboarding.step7.workspace")}
                </p>
                <p className="mt-2 text-lg font-semibold text-slate-900">
                  {state.name}
                </p>
              </div>
              <div className="rounded-3xl bg-emerald-50 p-5">
                <p className="text-xs uppercase tracking-[0.14em] text-emerald-700">
                  {t("onboarding.step7.agent")}
                </p>
                <p className="mt-2 text-lg font-semibold text-emerald-900">
                  {state.agentConnected
                    ? t("onboarding.common.connected")
                    : t("onboarding.common.pending")}
                </p>
              </div>
              <div className="rounded-3xl bg-emerald-50 p-5">
                <p className="text-xs uppercase tracking-[0.14em] text-emerald-700">
                  {t("onboarding.step7.database")}
                </p>
                <p className="mt-2 text-lg font-semibold text-emerald-900">
                  {state.dbVerifiedAt
                    ? t("onboarding.common.connected")
                    : t("onboarding.common.pending")}
                </p>
              </div>
              <div className="rounded-3xl bg-slate-50 p-5">
                <p className="text-xs uppercase tracking-[0.14em] text-slate-400">
                  {t("onboarding.step7.semanticLayer")}
                </p>
                <p className="mt-2 break-all text-sm font-semibold text-slate-900">
                  {state.semanticLayerId || t("common.na")}
                </p>
              </div>
            </div>
            <div className="mt-6 flex flex-wrap gap-3">
              <Link to="/chat" className={styles.buttonPrimary}>
                {t("onboarding.step7.chatCta")}
              </Link>
              <button
                type="button"
                className={styles.buttonSecondary}
                onClick={() => setInviteModalOpen(true)}
              >
                {t("onboarding.step7.inviteCta")}
              </button>
              {!state.onboardingComplete ? (
                <button
                  type="button"
                  className={styles.buttonSecondary}
                  disabled={runningStep7}
                  onClick={() => {
                    setRunningStep7(true);
                    void onboardingClient
                      .completeOnboarding({ tenantId })
                    .then((response) => {
                      setState(response.state ?? null);
                      setSuccess(t("onboarding.step7.completed"));
                    })
                      .catch((err) =>
                        setError(
                          onboardingErrorMessage(
                            err,
                            t,
                            "onboarding.error.complete",
                          ),
                        ),
                      )
                      .finally(() => setRunningStep7(false));
                  }}
                >
                  {t("onboarding.step7.complete")}
                </button>
              ) : null}
            </div>
          </section>

          <section className={styles.card}>
            <h2 className="text-lg font-semibold text-slate-900">
              {t("onboarding.step7.invitesTitle")}
            </h2>
            <div className="mt-4 flex flex-col gap-3">
              {state.invites.length === 0 ? (
                <p className="text-sm text-slate-600">
                  {t("onboarding.step7.invitesEmpty")}
                </p>
              ) : (
                state.invites.map((invite) => (
                  <div
                    key={invite.id}
                    className="rounded-3xl border border-slate-200 bg-slate-50 px-4 py-3"
                  >
                    <div className="font-medium text-slate-900">{invite.email}</div>
                    <div className="mt-1 text-xs text-slate-500">
                      {formatTimestamp(invite.createdAt)}
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>

          {inviteModalOpen ? (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-4">
              <div className="w-full max-w-xl rounded-[32px] bg-white p-6 shadow-2xl">
                <h2 className="text-xl font-semibold text-slate-900">
                  {t("onboarding.step7.inviteModalTitle")}
                </h2>
                <p className="mt-2 text-sm leading-6 text-slate-600">
                  {t("onboarding.step7.inviteModalBody")}
                </p>
                <textarea
                  className={`${styles.textarea} mt-4`}
                  value={inviteEmails}
                  onChange={(event) => setInviteEmails(event.target.value)}
                  placeholder={t("onboarding.step7.invitePlaceholder")}
                />
                <div className="mt-6 flex justify-end gap-3">
                  <button
                    type="button"
                    className={styles.buttonSecondary}
                    onClick={() => setInviteModalOpen(false)}
                  >
                    {t("onboarding.common.cancel")}
                  </button>
                  <button
                    type="button"
                    className={styles.buttonPrimary}
                    disabled={inviteSubmitting}
                    onClick={() => {
                      setInviteSubmitting(true);
                      void onboardingClient
                        .createInvites({
                          tenantId,
                          emails: [inviteEmails],
                        })
                        .then((response) => {
                          setState(response.state ?? null);
                          setInviteEmails("");
                          setInviteModalOpen(false);
                        })
                        .catch((err) =>
                          setError(
                            onboardingErrorMessage(
                              err,
                              t,
                              "onboarding.error.invites",
                            ),
                          ),
                        )
                        .finally(() => setInviteSubmitting(false));
                    }}
                  >
                    {t("onboarding.step7.saveInvites")}
                  </button>
                </div>
              </div>
            </div>
          ) : null}
        </>
      ) : null}
    </StepFrame>
  );
}
