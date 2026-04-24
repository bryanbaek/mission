import { useEffect, useState } from "react";

import ErrorBanner from "../components/ErrorBanner";
import { errorMessage } from "../lib/errorUtils";
import { type Locale, useI18n } from "../lib/i18n";

type Health = { status: string; database: string };

type AgentSession = {
  session_id: string;
  tenant_id: string;
  token_id: string;
  token_label: string;
  hostname: string;
  agent_version: string;
  connected_at: string;
  last_heartbeat_at: string;
  disconnected_at?: string | null;
  status: "online" | "offline";
};

type PingResponse = {
  session_id: string;
  command_id: string;
  completed_at: string;
  round_trip_ms: number;
};

const styles = {
  article: [
    "grid gap-5 px-8 py-6",
    "lg:grid-cols-[minmax(0,1fr)_220px]",
  ].join(" "),
  button: [
    "inline-flex items-center justify-center rounded-xl bg-slate-950",
    "px-4 py-2 text-sm font-medium text-white transition",
    "hover:bg-slate-800 disabled:cursor-not-allowed",
    "disabled:bg-slate-300",
  ].join(" "),
  countPill:
    "rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600",
  emptyState: "px-8 py-16 text-center text-sm text-slate-500",
  healthCard: [
    "grid gap-3 rounded-2xl bg-slate-950 px-5 py-4",
    "text-sm text-slate-50",
  ].join(" "),
  heroCard: "rounded-3xl border border-slate-200 bg-white p-8 shadow-sm",
  heroIntro: [
    "flex flex-col gap-4 md:flex-row",
    "md:items-end md:justify-between",
  ].join(" "),
  introLabel: [
    "text-xs font-semibold uppercase tracking-[0.24em]",
    "text-slate-500",
  ].join(" "),
  metaGrid:
    "grid gap-3 text-sm text-slate-600 sm:grid-cols-2 xl:grid-cols-4",
  metaLabel: "text-xs uppercase tracking-[0.14em] text-slate-400",
  metaValue: "mt-1 font-medium text-slate-900",
  sectionCard: "rounded-3xl border border-slate-200 bg-white shadow-sm",
  sectionHeader: "border-b border-slate-200 px-8 py-5",
  sectionHeaderRow: "flex items-center justify-between gap-4",
  sessionCode:
    "rounded-lg bg-slate-100 px-2.5 py-1 text-xs text-slate-700",
  shell: "flex flex-col gap-6",
  sideCard:
    "flex flex-col justify-between gap-4 rounded-2xl bg-slate-50 p-4",
  sideDetails: "space-y-2 text-sm text-slate-600",
  statusBase: [
    "rounded-full px-3 py-1 text-xs font-semibold uppercase",
    "tracking-[0.18em]",
  ].join(" "),
  statusOffline: "bg-slate-200 text-slate-600",
  statusOnline: "bg-emerald-100 text-emerald-700",
  tenantValue: "mt-1 truncate font-mono text-xs text-slate-700",
};

function statusClass(isOnline: boolean) {
  return [
    styles.statusBase,
    isOnline ? styles.statusOnline : styles.statusOffline,
  ].join(" ");
}


function sessionCountLabel(
  count: number,
  locale: Locale,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (locale === "ko") {
    return t("agents.count.other", { count });
  }
  return count === 1
    ? t("agents.count.one", { count })
    : t("agents.count.other", { count });
}

export default function AgentsPage() {
  const { formatDateTime, locale, t } = useI18n();
  const [health, setHealth] = useState<Health | null>(null);
  const [agents, setAgents] = useState<AgentSession[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [pingingSessionID, setPingingSessionID] =
    useState<string | null>(null);
  const [pingResults, setPingResults] = useState<
    Record<string, PingResponse>
  >({});

  const formatDateTimeValue = (value: string | null | undefined) => {
    if (!value) {
      return t("common.na");
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return value;
    }
    return formatDateTime(date, {
      dateStyle: "medium",
      timeStyle: "medium",
    });
  };

  const statusLabel = (status: AgentSession["status"]) =>
    status === "online" ? t("common.online") : t("common.offline");

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const [healthResp, agentsResp] = await Promise.all([
          fetch("/healthz"),
          fetch("/api/debug/agents"),
        ]);

        if (!healthResp.ok) {
          throw new Error(`health check failed with ${healthResp.status}`);
        }
        if (!agentsResp.ok) {
          throw new Error(
            `agent debug API failed with ${agentsResp.status}`,
          );
        }

        const [healthBody, agentsBody] = await Promise.all([
          healthResp.json() as Promise<Health>,
          agentsResp.json() as Promise<{ agents: AgentSession[] }>,
        ]);

        if (cancelled) {
          return;
        }

        setHealth(healthBody);
        setAgents(agentsBody.agents);
        setError(null);
      } catch (err) {
        if (!cancelled) {
          setError(errorMessage(err));
        }
      }
    };

    void load();
    const interval = window.setInterval(() => void load(), 5000);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, []);

  const runPing = async (sessionID: string) => {
    setPingingSessionID(sessionID);
    try {
      const response = await fetch(`/api/debug/agents/${sessionID}/ping`, {
        method: "POST",
      });
      const body = (await response.json()) as PingResponse | { error?: string };
      if (!response.ok) {
        throw new Error(
          "error" in body && body.error
            ? body.error
            : `ping failed with ${response.status}`,
        );
      }

      setPingResults((current) => ({
        ...current,
        [sessionID]: body as PingResponse,
      }));
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setPingingSessionID(null);
    }
  };

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <div className={styles.heroIntro}>
          <div className="space-y-2">
            <p className={styles.introLabel}>{t("common.appLabel")}</p>
            <h1 className="text-3xl font-semibold tracking-tight">
              {t("agents.hero.title")}
            </h1>
            <p className="max-w-2xl text-sm leading-6 text-slate-600">
              {t("agents.hero.subtitle")}
            </p>
          </div>
          <div className={styles.healthCard}>
            <div className="flex items-center gap-3">
              <span className="text-slate-400">
                {t("agents.health.api")}
              </span>
              <span className="font-medium">
                {health?.status ?? t("common.loading")}
              </span>
            </div>
            <div className="flex items-center gap-3">
              <span className="text-slate-400">
                {t("agents.health.database")}
              </span>
              <span className="font-medium">
                {health?.database ?? t("common.loading")}
              </span>
            </div>
          </div>
        </div>
        <ErrorBanner message={error} className="mt-4" />
      </section>

      <section className={styles.sectionCard}>
        <div className={styles.sectionHeader}>
          <div className={styles.sectionHeaderRow}>
            <div>
              <h2 className="text-lg font-semibold">
                {t("agents.section.title")}
              </h2>
              <p className="text-sm text-slate-500">
                {t("agents.section.subtitle")}
              </p>
            </div>
            <div className={styles.countPill}>
              {sessionCountLabel(agents.length, locale, t)}
            </div>
          </div>
        </div>

        {agents.length === 0 ? (
          <div className={styles.emptyState}>
            {t("agents.section.empty")}
          </div>
        ) : (
          <div className="divide-y divide-slate-200">
            {agents.map((agent) => {
              const ping = pingResults[agent.session_id];
              const isOnline = agent.status === "online";

              return (
                <article
                  key={agent.session_id}
                  className={styles.article}
                >
                  <div className="space-y-4">
                    <div className="flex flex-wrap items-center gap-3">
                      <span className={statusClass(isOnline)}>
                        {statusLabel(agent.status)}
                      </span>
                      <code className={styles.sessionCode}>
                        {agent.session_id}
                      </code>
                    </div>

                    <dl className={styles.metaGrid}>
                      <div>
                        <dt className={styles.metaLabel}>
                          {t("agents.meta.host")}
                        </dt>
                        <dd className={styles.metaValue}>
                          {agent.hostname}
                        </dd>
                      </div>
                      <div>
                        <dt className={styles.metaLabel}>
                          {t("agents.meta.version")}
                        </dt>
                        <dd className={styles.metaValue}>
                          {agent.agent_version || t("common.unknown")}
                        </dd>
                      </div>
                      <div>
                        <dt className={styles.metaLabel}>
                          {t("agents.meta.tokenLabel")}
                        </dt>
                        <dd className={styles.metaValue}>
                          {agent.token_label}
                        </dd>
                      </div>
                      <div>
                        <dt className={styles.metaLabel}>
                          {t("agents.meta.tenant")}
                        </dt>
                        <dd className={styles.tenantValue}>
                          {agent.tenant_id}
                        </dd>
                      </div>
                    </dl>
                  </div>

                  <div className={styles.sideCard}>
                    <div className={styles.sideDetails}>
                      <div>
                        <span className={`block ${styles.metaLabel}`}>
                          {t("agents.meta.connected")}
                        </span>
                        <span className={styles.metaValue}>
                          {formatDateTimeValue(agent.connected_at)}
                        </span>
                      </div>
                      <div>
                        <span className={`block ${styles.metaLabel}`}>
                          {t("agents.meta.lastHeartbeat")}
                        </span>
                        <span className={styles.metaValue}>
                          {formatDateTimeValue(agent.last_heartbeat_at)}
                        </span>
                      </div>
                      {agent.disconnected_at ? (
                        <div>
                          <span className={`block ${styles.metaLabel}`}>
                            {t("agents.meta.disconnected")}
                          </span>
                          <span className={styles.metaValue}>
                            {formatDateTimeValue(agent.disconnected_at)}
                          </span>
                        </div>
                      ) : null}
                      {ping ? (
                        <div>
                          <span className={`block ${styles.metaLabel}`}>
                            {t("agents.meta.lastPing")}
                          </span>
                          <span className={styles.metaValue}>
                            {t("agents.meta.lastPingValue", {
                              ms: ping.round_trip_ms,
                              time: formatDateTimeValue(ping.completed_at),
                            })}
                          </span>
                        </div>
                      ) : null}
                    </div>

                    <button
                      type="button"
                      disabled={
                        !isOnline || pingingSessionID === agent.session_id
                      }
                      onClick={() => void runPing(agent.session_id)}
                      className={styles.button}
                    >
                      {pingingSessionID === agent.session_id
                        ? t("agents.button.pinging")
                        : t("agents.button.ping")}
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
