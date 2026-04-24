import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
} from "react";
import { useAuth } from "@clerk/clerk-react";

import type { Tenant } from "../gen/tenant/v1/tenant_pb";
import ErrorBanner from "../components/ErrorBanner";
import { errorMessage } from "../lib/errorUtils";
import { useI18n } from "../lib/i18n";
import { useTenantClient } from "../lib/tenantClient";

type QueryStatus = {
  status: "online" | "offline";
  session_id?: string;
  hostname?: string;
  agent_version?: string;
  token_label?: string;
  connected_at?: string;
  last_heartbeat_at?: string;
};

type QueryResponse = {
  session_id: string;
  command_id: string;
  completed_at: string;
  columns: string[];
  rows: Array<Record<string, unknown>>;
  elapsed_ms: number;
  database_user: string;
  database_name: string;
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
  sectionCard: "rounded-3xl border border-slate-200 bg-white p-6 shadow-sm",
  sectionHeader:
    "flex items-center justify-between gap-4 border-b border-slate-200 pb-4",
  twoCol: "grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]",
  row: "flex items-center justify-between gap-3 px-3 py-2",
  rowActive: "rounded-lg bg-slate-950 text-white",
  rowIdle: "rounded-lg hover:bg-slate-100",
  textarea: [
    "min-h-[180px] w-full rounded-2xl border border-slate-300 px-4 py-3",
    "font-mono text-sm leading-6 text-slate-900",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
  primaryButton: [
    "inline-flex items-center justify-center rounded-xl bg-slate-950",
    "px-4 py-2 text-sm font-medium text-white transition",
    "hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-300",
  ].join(" "),
  statusPill:
    "rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em]",
  statusOnline: "bg-emerald-100 text-emerald-700",
  statusOffline: "bg-slate-200 text-slate-600",
  metaLabel: "text-xs uppercase tracking-[0.14em] text-slate-400",
  metaValue: "mt-1 text-sm font-medium text-slate-900",
  resultShell: "mt-5 flex flex-col gap-4",
  resultBox: [
    "overflow-auto rounded-2xl border border-slate-200 bg-slate-950 p-4",
    "font-mono text-xs leading-6 text-emerald-200",
  ].join(" "),
  muted: "text-sm text-slate-500",
};

function statusClass(status: QueryStatus["status"] | undefined) {
  return [
    styles.statusPill,
    status === "online" ? styles.statusOnline : styles.statusOffline,
  ].join(" ");
}

export default function QueryPage() {
  const client = useTenantClient();
  const { getToken } = useAuth();
  const { t } = useI18n();

  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [tenantsError, setTenantsError] = useState<string | null>(null);

  const [status, setStatus] = useState<QueryStatus | null>(null);
  const [statusError, setStatusError] = useState<string | null>(null);
  const [loadingStatus, setLoadingStatus] = useState(false);

  const [sql, setSQL] = useState("SELECT 1");
  const [running, setRunning] = useState(false);
  const [queryError, setQueryError] = useState<string | null>(null);
  const [queryResult, setQueryResult] = useState<QueryResponse | null>(null);

  const selectedTenant = useMemo(
    () => tenants.find((tenant) => tenant.id === selectedID) ?? null,
    [tenants, selectedID],
  );

  const authedFetch = useCallback(
    async (input: string, init?: RequestInit) => {
      const token = await getToken();
      const headers = new Headers(init?.headers);
      if (token) {
        headers.set("Authorization", `Bearer ${token}`);
      }
      return fetch(input, { ...init, headers });
    },
    [getToken],
  );

  const loadTenants = useCallback(async () => {
    try {
      const resp = await client.listTenants({});
      setTenants(resp.tenants);
      setTenantsError(null);
      if (!selectedID && resp.tenants.length > 0) {
        setSelectedID(resp.tenants[0].id);
      }
    } catch (err) {
      setTenantsError(errorMessage(err));
    }
  }, [client, selectedID]);

  useEffect(() => {
    void loadTenants();
  }, [loadTenants]);

  const loadStatus = useCallback(
    async (tenantID: string) => {
      setLoadingStatus(true);
      try {
        const response = await authedFetch(
          `/api/debug/tenants/${tenantID}/query`,
        );
        const body = (await response.json()) as QueryStatus | { error?: string };
        if (!response.ok) {
          throw new Error(
            "error" in body && body.error
              ? body.error
              : `status request failed with ${response.status}`,
          );
        }
        setStatus(body as QueryStatus);
        setStatusError(null);
      } catch (err) {
        setStatusError(errorMessage(err));
      } finally {
        setLoadingStatus(false);
      }
    },
    [authedFetch],
  );

  useEffect(() => {
    if (!selectedID) {
      setStatus(null);
      return;
    }
    void loadStatus(selectedID);
  }, [loadStatus, selectedID]);

  const runQuery = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selectedID) {
      return;
    }

    setRunning(true);
    setQueryError(null);
    try {
      const response = await authedFetch(
        `/api/debug/tenants/${selectedID}/query`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ sql }),
        },
      );
      const body = (await response.json()) as QueryResponse | { error?: string };
      if (!response.ok) {
        throw new Error(
          "error" in body && body.error
            ? body.error
            : `query failed with ${response.status}`,
        );
      }

      setQueryResult(body as QueryResponse);
      setQueryError(null);
      await loadStatus(selectedID);
    } catch (err) {
      setQueryError(errorMessage(err));
      setQueryResult(null);
    } finally {
      setRunning(false);
    }
  };

  const statusLabel = loadingStatus
    ? t("common.loading")
    : status?.status === "online"
      ? t("common.online")
      : t("common.offline");

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>{t("common.appLabel")}</p>
        <h1 className="text-3xl font-semibold tracking-tight">
          {t("queryDebug.hero.title")}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">
          {t("queryDebug.hero.subtitle")}
        </p>
      </section>

      <div className={styles.twoCol}>
        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">
                {t("queryDebug.tenants.title")}
              </h2>
              <p className={styles.muted}>
                {t("queryDebug.tenants.subtitle")}
              </p>
            </div>
          </div>

          {tenantsError ? (
            <div className="mt-4 text-sm text-rose-700">{tenantsError}</div>
          ) : null}

          <ul className="mt-4 flex flex-col gap-1">
            {tenants.length === 0 ? (
              <li className="px-3 py-6 text-center text-sm text-slate-500">
                {t("queryDebug.tenants.empty")}
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
                            active ? "text-slate-300" : "text-slate-500",
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
        </section>

        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">
                {selectedTenant?.name ?? t("queryDebug.detail.selectPrompt")}
              </h2>
              <p className={styles.muted}>
                {t("queryDebug.detail.subtitle")}
              </p>
            </div>
            <span className={statusClass(status?.status)}>{statusLabel}</span>
          </div>

          {statusError ? (
            <div className="mt-4 text-sm text-rose-700">{statusError}</div>
          ) : null}

          <div className="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <div>
              <div className={styles.metaLabel}>
                {t("queryDebug.meta.host")}
              </div>
              <div className={styles.metaValue}>
                {status?.hostname ?? t("common.na")}
              </div>
            </div>
            <div>
              <div className={styles.metaLabel}>
                {t("queryDebug.meta.session")}
              </div>
              <div className={styles.metaValue}>
                {status?.session_id ?? t("common.na")}
              </div>
            </div>
            <div>
              <div className={styles.metaLabel}>
                {t("queryDebug.meta.version")}
              </div>
              <div className={styles.metaValue}>
                {status?.agent_version ?? t("common.na")}
              </div>
            </div>
            <div>
              <div className={styles.metaLabel}>
                {t("queryDebug.meta.tokenLabel")}
              </div>
              <div className={styles.metaValue}>
                {status?.token_label ?? t("common.na")}
              </div>
            </div>
          </div>

          <form onSubmit={runQuery} className={styles.resultShell}>
            <label className="flex flex-col gap-2">
              <span className={styles.metaLabel}>
                {t("queryDebug.form.sql")}
              </span>
              <textarea
                value={sql}
                onChange={(event) => setSQL(event.target.value)}
                className={styles.textarea}
                placeholder={t("queryDebug.form.sqlPlaceholder")}
              />
            </label>

            <div className="flex items-center gap-3">
              <button
                type="submit"
                disabled={!selectedID || status?.status !== "online" || running}
                className={styles.primaryButton}
              >
                {running
                  ? t("queryDebug.form.running")
                  : t("queryDebug.form.run")}
              </button>
              <span className={styles.muted}>
                {status?.status === "online"
                  ? t("queryDebug.form.available")
                  : t("queryDebug.form.unavailable")}
              </span>
            </div>
          </form>

          <ErrorBanner message={queryError} className="mt-5" />

          {queryResult ? (
            <div className={styles.resultShell}>
              <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
                <div>
                  <div className={styles.metaLabel}>
                    {t("queryDebug.result.database")}
                  </div>
                  <div className={styles.metaValue}>
                    {queryResult.database_name}
                  </div>
                </div>
                <div>
                  <div className={styles.metaLabel}>
                    {t("queryDebug.result.user")}
                  </div>
                  <div className={styles.metaValue}>
                    {queryResult.database_user}
                  </div>
                </div>
                <div>
                  <div className={styles.metaLabel}>
                    {t("queryDebug.result.elapsed")}
                  </div>
                  <div className={styles.metaValue}>
                    {queryResult.elapsed_ms} ms
                  </div>
                </div>
                <div>
                  <div className={styles.metaLabel}>
                    {t("queryDebug.result.columns")}
                  </div>
                  <div className={styles.metaValue}>
                    {queryResult.columns.join(", ") || t("common.na")}
                  </div>
                </div>
              </div>

              <pre className={styles.resultBox}>
                {JSON.stringify(queryResult.rows, null, 2)}
              </pre>
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}
