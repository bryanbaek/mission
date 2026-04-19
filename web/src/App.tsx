import { useEffect, useState } from "react";

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
  errorBanner: [
    "mt-4 rounded-2xl border border-rose-200 bg-rose-50",
    "px-4 py-3 text-sm text-rose-700",
  ].join(" "),
  healthCard: [
    "grid gap-3 rounded-2xl bg-slate-950 px-5 py-4",
    "text-sm text-slate-50",
  ].join(" "),
  heroCard: [
    "rounded-3xl border border-slate-200 bg-white p-8 shadow-sm",
  ].join(" "),
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
  page: "min-h-screen bg-slate-100 p-6 text-slate-900",
  sectionCard: "rounded-3xl border border-slate-200 bg-white shadow-sm",
  sectionHeader: "border-b border-slate-200 px-8 py-5",
  sectionHeaderRow: "flex items-center justify-between gap-4",
  sessionCode:
    "rounded-lg bg-slate-100 px-2.5 py-1 text-xs text-slate-700",
  shell: "mx-auto flex max-w-6xl flex-col gap-6",
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

export default function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [agents, setAgents] = useState<AgentSession[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [pingingSessionID, setPingingSessionID] =
    useState<string | null>(null);
  const [pingResults, setPingResults] = useState<
    Record<string, PingResponse>
  >({});

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
          setError(err instanceof Error ? err.message : String(err));
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
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setPingingSessionID(null);
    }
  };

  return (
    <main className={styles.page}>
      <div className={styles.shell}>
        <section className={styles.heroCard}>
          <div className={styles.heroIntro}>
            <div className="space-y-2">
              <p className={styles.introLabel}>Mission</p>
              <h1 className="text-3xl font-semibold tracking-tight">
                Week 2.1 agent tunnel debug surface
              </h1>
              <p className="max-w-2xl text-sm leading-6 text-slate-600">
                Polling the control plane every 5 seconds for live agent
                presence and exposing a manual ping to validate the outbound
                tunnel end-to-end.
              </p>
            </div>
            <div className={styles.healthCard}>
              <div className="flex items-center gap-3">
                <span className="text-slate-400">API</span>
                <span className="font-medium">
                  {health?.status ?? "loading"}
                </span>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-slate-400">Postgres</span>
                <span className="font-medium">
                  {health?.database ?? "loading"}
                </span>
              </div>
            </div>
          </div>
          {error ? (
            <div className={styles.errorBanner}>
              {error}
            </div>
          ) : null}
        </section>

        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div className={styles.sectionHeaderRow}>
              <div>
                <h2 className="text-lg font-semibold">Agent sessions</h2>
                <p className="text-sm text-slate-500">
                  One runtime-scoped row per tenant token. Offline rows remain
                  until replaced.
                </p>
              </div>
              <div className={styles.countPill}>
                {agents.length} session{agents.length === 1 ? "" : "s"}
              </div>
            </div>
          </div>

          {agents.length === 0 ? (
            <div className={styles.emptyState}>
              No edge agents connected yet.
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
                          {agent.status}
                        </span>
                        <code className={styles.sessionCode}>
                          {agent.session_id}
                        </code>
                      </div>

                      <dl className={styles.metaGrid}>
                        <div>
                          <dt className={styles.metaLabel}>Host</dt>
                          <dd className={styles.metaValue}>
                            {agent.hostname}
                          </dd>
                        </div>
                        <div>
                          <dt className={styles.metaLabel}>Version</dt>
                          <dd className={styles.metaValue}>
                            {agent.agent_version || "unknown"}
                          </dd>
                        </div>
                        <div>
                          <dt className={styles.metaLabel}>Token label</dt>
                          <dd className={styles.metaValue}>
                            {agent.token_label}
                          </dd>
                        </div>
                        <div>
                          <dt className={styles.metaLabel}>Tenant</dt>
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
                            Connected
                          </span>
                          <span className={styles.metaValue}>
                            {formatDateTime(agent.connected_at)}
                          </span>
                        </div>
                        <div>
                          <span className={`block ${styles.metaLabel}`}>
                            Last heartbeat
                          </span>
                          <span className={styles.metaValue}>
                            {formatDateTime(agent.last_heartbeat_at)}
                          </span>
                        </div>
                        {agent.disconnected_at ? (
                          <div>
                            <span className={`block ${styles.metaLabel}`}>
                              Disconnected
                            </span>
                            <span className={styles.metaValue}>
                              {formatDateTime(agent.disconnected_at)}
                            </span>
                          </div>
                        ) : null}
                        {ping ? (
                          <div>
                            <span className={`block ${styles.metaLabel}`}>
                              Last ping
                            </span>
                            <span className={styles.metaValue}>
                              {ping.round_trip_ms} ms at{" "}
                              {formatDateTime(ping.completed_at)}
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
                          ? "Pinging..."
                          : "Ping agent"}
                      </button>
                    </div>
                  </article>
                );
              })}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}

function formatDateTime(value: string | null | undefined) {
  if (!value) {
    return "n/a";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(date);
}
