import { useEffect, useMemo, useState } from "react";
import { ConnectError } from "@connectrpc/connect";
import { Navigate, useNavigate } from "react-router-dom";

import { WorkspaceRole } from "../../gen/onboarding/v1/onboarding_pb";
import { onboardingStepPath } from "../../lib/onboarding";
import { useOnboardingClient } from "../../lib/onboardingClient";
import { useI18n } from "../../lib/i18n";

const styles = {
  shell: "mx-auto flex min-h-screen max-w-5xl flex-col gap-6 px-4 py-8 sm:px-6",
  hero: [
    "rounded-[32px] border border-slate-200 bg-white p-8 shadow-sm",
    "flex flex-col gap-3",
  ].join(" "),
  card: "rounded-[32px] border border-slate-200 bg-white p-6 shadow-sm",
  button: [
    "inline-flex items-center justify-center rounded-xl bg-slate-950",
    "px-4 py-2 text-sm font-medium text-white transition",
    "hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-300",
  ].join(" "),
};

export default function OnboardingHomeScreen() {
  const onboardingClient = useOnboardingClient();
  const navigate = useNavigate();
  const { formatDateTime, setLocale, t } = useI18n();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [redirectPath, setRedirectPath] = useState<string | null>(null);
  const [ownerIncomplete, setOwnerIncomplete] = useState<
    Array<{
      tenantId: string;
      name: string;
      currentStep: number;
      updatedLabel: string;
    }>
  >([]);

  useEffect(() => {
    setLocale("ko");
  }, [setLocale]);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const response = await onboardingClient.listWorkspaces({});
        if (cancelled) {
          return;
        }
        const items = response.workspaces
          .filter(
            (workspace) =>
              workspace.role === WorkspaceRole.OWNER &&
              !workspace.onboardingComplete,
          )
          .map((workspace) => ({
            tenantId: workspace.tenantId,
            name: workspace.name,
            currentStep: workspace.currentStep,
            updatedLabel: workspace.updatedAt
              ? formatDateTime(
                  new Date(
                    Number(workspace.updatedAt.seconds) * 1000,
                  ),
                  {
                    dateStyle: "medium",
                    timeStyle: "short",
                  },
                )
              : "",
          }));

        if (items.length === 0) {
          setRedirectPath("/chat");
        } else if (items.length === 1) {
          setRedirectPath(
            onboardingStepPath(items[0].tenantId, items[0].currentStep),
          );
        } else {
          setOwnerIncomplete(items);
        }
        setError(null);
      } catch (err) {
        if (!cancelled) {
          const message = ConnectError.from(err).rawMessage;
          setError(
            message === "unauthenticated"
              ? t("onboarding.error.permission")
              : t("onboarding.error.workspacePicker"),
          );
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [formatDateTime, onboardingClient, t]);

  const heading = useMemo(
    () =>
      ownerIncomplete.length > 1
        ? t("onboarding.workspacePicker.title")
        : t("onboarding.workspacePicker.loadingTitle"),
    [ownerIncomplete.length, t],
  );

  if (redirectPath) {
    return <Navigate to={redirectPath} replace />;
  }

  return (
    <div className={styles.shell}>
      <section className={styles.hero}>
        <p className="text-xs font-semibold uppercase tracking-[0.24em] text-slate-500">
          {t("onboarding.common.label")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight text-slate-900">
          {heading}
        </h1>
        <p className="max-w-2xl text-sm leading-6 text-slate-600">
          {t("onboarding.workspacePicker.subtitle")}
        </p>
      </section>

      {loading ? (
        <section className={styles.card}>
          <p className="text-sm text-slate-600">{t("onboarding.common.loading")}</p>
        </section>
      ) : null}

      {error ? (
        <section className="rounded-[32px] border border-rose-200 bg-rose-50 p-6 text-sm text-rose-700">
          {error}
        </section>
      ) : null}

      {!loading && !error && ownerIncomplete.length > 1 ? (
        <section className={styles.card}>
          <div className="grid gap-4 md:grid-cols-2">
            {ownerIncomplete.map((workspace) => (
              <article
                key={workspace.tenantId}
                className="rounded-3xl border border-slate-200 bg-slate-50 p-5"
              >
                <h2 className="text-lg font-semibold text-slate-900">
                  {workspace.name}
                </h2>
                <p className="mt-2 text-sm text-slate-600">
                  {t("onboarding.workspacePicker.resume", {
                    step: workspace.currentStep,
                  })}
                </p>
                <p className="mt-2 text-xs text-slate-500">
                  {t("onboarding.workspacePicker.updatedAt", {
                    time: workspace.updatedLabel || t("common.na"),
                  })}
                </p>
                <button
                  type="button"
                  className={`${styles.button} mt-4 w-full`}
                  onClick={() =>
                    navigate(
                      onboardingStepPath(
                        workspace.tenantId,
                        workspace.currentStep,
                      ),
                    )
                  }
                >
                  {t("onboarding.workspacePicker.open")}
                </button>
              </article>
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
