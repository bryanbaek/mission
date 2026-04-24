import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useNavigate, useParams } from "react-router-dom";

import type { OnboardingState } from "../../gen/onboarding/v1/onboarding_pb";
import { useI18n } from "../../lib/i18n";
import { onboardingStepPath } from "../../lib/onboarding";
import { useOnboardingClient } from "../../lib/onboardingClient";
import ErrorBanner from "../ErrorBanner";
import SemanticReviewStep from "./SemanticReviewStep";
import {
  CompleteStep,
  DatabaseStep,
  InstallStep,
  SchemaStep,
  StarterStep,
  WelcomeStep,
} from "./OnboardingStepPanels";
import StepFrame from "./OnboardingStepFrame";
import {
  buildMySQLConnectionString,
  copyText,
  formatTimestamp,
  onboardingErrorMessage,
  styles,
} from "./onboardingStepUtils";

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
  const suggestedConnectionString = buildMySQLConnectionString({
    username: state?.dbUsername ?? "",
    password: state?.generatedPassword ?? "",
    host: dbHost,
    port: dbPort,
    databaseName: dbName,
  });

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
      <ErrorBanner message={error} />
      {success ? <div className={styles.bannerSuccess}>{success}</div> : null}

      {step === 1 ? (
        <WelcomeStep
          workspaceName={workspaceName}
          confirmedLanguage={confirmedLanguage}
          onWorkspaceNameChange={setWorkspaceName}
          onConfirmedLanguageChange={setConfirmedLanguage}
          onNext={() =>
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
        />
      ) : null}

      {step === 2 ? (
        <InstallStep
          state={state}
          copiedKey={copiedKey}
          onCopy={(key, text) => void handleCopy(key, text)}
        />
      ) : null}

      {step === 3 ? (
        <DatabaseStep
          state={state}
          copiedKey={copiedKey}
          dbHost={dbHost}
          dbPort={dbPort}
          dbName={dbName}
          connectionString={connectionString}
          dbSubmitting={dbSubmitting}
          suggestedConnectionString={suggestedConnectionString}
          onDbHostChange={setDbHost}
          onDbPortChange={setDbPort}
          onDbNameChange={setDbName}
          onConnectionStringChange={setConnectionString}
          onCopy={(key, text) => void handleCopy(key, text)}
          onUseSuggested={() => {
            setConnectionString(suggestedConnectionString);
            setError(null);
          }}
          onGenerateSql={() => {
            setDbSubmitting(true);
            void onboardingClient
              .configureDatabase({
                tenantId,
                host: dbHost,
                port: Number.parseInt(dbPort, 10) || 3306,
                databaseName: dbName,
              })
              .then((response) => {
                const nextState = response.state ?? null;
                setState(nextState);
                if (!connectionString.trim()) {
                  const nextConnectionString = buildMySQLConnectionString({
                    username: nextState?.dbUsername ?? "",
                    password: nextState?.generatedPassword ?? "",
                    host: nextState?.dbHost || dbHost,
                    port:
                      nextState?.dbPort && nextState.dbPort > 0
                        ? String(nextState.dbPort)
                        : dbPort,
                    databaseName: nextState?.dbName || dbName,
                  });
                  if (nextConnectionString) {
                    setConnectionString(nextConnectionString);
                  }
                }
                setError(null);
              })
              .catch((err) =>
                setError(
                  onboardingErrorMessage(err, t, "onboarding.error.database"),
                ),
              )
              .finally(() => setDbSubmitting(false));
          }}
          onVerify={() => {
            const nextConnectionString =
              connectionString.trim() || suggestedConnectionString;
            if (!nextConnectionString) {
              setError(t("onboarding.step3.connectionRequired"));
              return;
            }
            setError(null);
            if (nextConnectionString !== connectionString) {
              setConnectionString(nextConnectionString);
            }
            setDbSubmitting(true);
            void onboardingClient
              .configureDatabase({
                tenantId,
                host: dbHost,
                port: Number.parseInt(dbPort, 10) || 3306,
                databaseName: dbName,
                connectionString: nextConnectionString,
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
                  onboardingErrorMessage(err, t, "onboarding.error.database"),
                ),
              )
              .finally(() => setDbSubmitting(false));
          }}
        />
      ) : null}

      {step === 4 ? (
        <SchemaStep
          state={state}
          schemaSubmitting={schemaSubmitting}
          holdSchemaSummary={holdSchemaSummary}
          onNext={() => navigate(onboardingStepPath(tenantId, 5))}
          onRun={() => {
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
        />
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
        <StarterStep
          tenantId={tenantId}
          onPick={(text) =>
            navigate(
              `/chat?tenant=${encodeURIComponent(tenantId)}&q=${encodeURIComponent(text)}&auto=1`,
            )
          }
          onComplete={() =>
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
                  onboardingErrorMessage(err, t, "onboarding.error.complete"),
                ),
              )
          }
        />
      ) : null}

      {step === 7 ? (
        <CompleteStep
          state={state}
          runningStep7={runningStep7}
          inviteModalOpen={inviteModalOpen}
          inviteEmails={inviteEmails}
          inviteSubmitting={inviteSubmitting}
          onOpenInviteModal={() => setInviteModalOpen(true)}
          onCloseInviteModal={() => setInviteModalOpen(false)}
          onInviteEmailsChange={setInviteEmails}
          onComplete={() => {
            setRunningStep7(true);
            void onboardingClient
              .completeOnboarding({ tenantId })
              .then((response) => {
                setState(response.state ?? null);
                setSuccess(t("onboarding.step7.completed"));
              })
              .catch((err) =>
                setError(
                  onboardingErrorMessage(err, t, "onboarding.error.complete"),
                ),
              )
              .finally(() => setRunningStep7(false));
          }}
          onCreateInvites={() => {
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
                  onboardingErrorMessage(err, t, "onboarding.error.invites"),
                ),
              )
              .finally(() => setInviteSubmitting(false));
          }}
        />
      ) : null}
    </StepFrame>
  );
}
