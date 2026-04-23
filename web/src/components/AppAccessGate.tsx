import { useEffect, useState } from "react";
import { ConnectError } from "@connectrpc/connect";
import { Navigate, Outlet, useLocation } from "react-router-dom";

import { WorkspaceRole } from "../gen/onboarding/v1/onboarding_pb";
import { onboardingStepPath } from "../lib/onboarding";
import { useOnboardingClient } from "../lib/onboardingClient";
import { useI18n } from "../lib/i18n";

const appReadyOnboardingStep = 6;

type WorkspaceAccessState = {
  currentStep: number;
  onboardingComplete: boolean;
  role: WorkspaceRole;
  tenantId: string;
};

function appAccessRedirectPath(
  workspaces: readonly WorkspaceAccessState[],
): string | null {
  const ownerBlocked = workspaces.filter(
    (workspace) =>
      workspace.role === WorkspaceRole.OWNER &&
      !workspace.onboardingComplete &&
      workspace.currentStep < appReadyOnboardingStep,
  );

  if (ownerBlocked.length === 1) {
    return onboardingStepPath(
      ownerBlocked[0].tenantId,
      ownerBlocked[0].currentStep,
    );
  }
  if (ownerBlocked.length > 1) {
    return "/onboarding";
  }
  return null;
}

export default function AppAccessGate() {
  const onboardingClient = useOnboardingClient();
  const location = useLocation();
  const { setLocale, t } = useI18n();

  const [redirectPath, setRedirectPath] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const response = await onboardingClient.listWorkspaces({});
        if (cancelled) {
          return;
        }

        const nextRedirectPath = appAccessRedirectPath(response.workspaces);
        if (nextRedirectPath) {
          setLocale("ko");
        }
        setRedirectPath(nextRedirectPath);
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
  }, [onboardingClient, setLocale, t]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-100 p-6 text-sm text-slate-600">
        {t("common.loading")}
      </div>
    );
  }

  if (redirectPath && location.pathname !== redirectPath) {
    return <Navigate to={redirectPath} replace />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-xl p-8 text-sm text-rose-700">
        {error}
      </div>
    );
  }

  return <Outlet />;
}
