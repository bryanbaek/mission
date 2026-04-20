import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
} from "react";

import type { Timestamp } from "@bufbuild/protobuf/wkt";

import {
  type Tenant,
  type TenantTokenSummary,
} from "../gen/tenant/v1/tenant_pb";
import { useTenantClient } from "../lib/tenantClient";

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
  twoCol: "grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)]",
  row: "flex items-center justify-between gap-3 px-3 py-2",
  rowActive: "rounded-lg bg-slate-950 text-white",
  rowIdle: "rounded-lg hover:bg-slate-100",
  input: [
    "w-full rounded-lg border border-slate-300 px-3 py-2 text-sm",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
  primaryButton: [
    "inline-flex items-center justify-center rounded-xl bg-slate-950",
    "px-4 py-2 text-sm font-medium text-white transition",
    "hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-300",
  ].join(" "),
  ghostButton: [
    "inline-flex items-center justify-center rounded-xl px-3 py-1.5",
    "text-xs font-medium text-slate-600 transition",
    "hover:bg-slate-100 disabled:cursor-not-allowed disabled:text-slate-300",
  ].join(" "),
  dangerButton: [
    "inline-flex items-center justify-center rounded-xl px-3 py-1.5",
    "text-xs font-medium text-rose-600 transition",
    "hover:bg-rose-50 disabled:cursor-not-allowed disabled:text-slate-300",
  ].join(" "),
  errorBanner: [
    "rounded-2xl border border-rose-200 bg-rose-50",
    "px-4 py-3 text-sm text-rose-700",
  ].join(" "),
  successBanner: [
    "rounded-2xl border border-emerald-200 bg-emerald-50",
    "px-4 py-3 text-sm text-emerald-800",
  ].join(" "),
  tokenCode: [
    "block w-full break-all rounded-lg bg-slate-950 px-3 py-2",
    "font-mono text-xs text-emerald-200",
  ].join(" "),
  metaLabel: "text-xs uppercase tracking-[0.14em] text-slate-400",
  mono: "font-mono text-xs text-slate-700",
};

function formatTimestamp(ts: Timestamp | undefined): string {
  if (!ts) {
    return "";
  }
  const ms = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  if (!Number.isFinite(ms) || ms <= 0) {
    return "";
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(ms));
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export default function TenantsPage() {
  const client = useTenantClient();

  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [tokens, setTokens] = useState<TenantTokenSummary[]>([]);
  const [tenantsError, setTenantsError] = useState<string | null>(null);
  const [tokensError, setTokensError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const [newSlug, setNewSlug] = useState("");
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);

  const [newLabel, setNewLabel] = useState("");
  const [issuing, setIssuing] = useState(false);
  const [revokingID, setRevokingID] = useState<string | null>(null);
  const [plaintext, setPlaintext] = useState<string | null>(null);

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

  const loadTokens = useCallback(
    async (tenantID: string) => {
      try {
        const resp = await client.listAgentTokens({ tenantId: tenantID });
        setTokens(resp.tokens);
        setTokensError(null);
      } catch (err) {
        setTokensError(errorMessage(err));
      }
    },
    [client],
  );

  useEffect(() => {
    if (!selectedID) {
      setTokens([]);
      return;
    }
    void loadTokens(selectedID);
  }, [loadTokens, selectedID]);

  const selectedTenant = useMemo(
    () => tenants.find((t) => t.id === selectedID) ?? null,
    [tenants, selectedID],
  );

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setCreating(true);
    setActionError(null);
    try {
      const resp = await client.createTenant({ slug: newSlug, name: newName });
      setNewSlug("");
      setNewName("");
      if (resp.tenant) {
        setSelectedID(resp.tenant.id);
      }
      await loadTenants();
    } catch (err) {
      setActionError(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const handleIssue = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selectedID) {
      return;
    }
    setIssuing(true);
    setActionError(null);
    setPlaintext(null);
    try {
      const resp = await client.issueAgentToken({
        tenantId: selectedID,
        label: newLabel,
      });
      setNewLabel("");
      setPlaintext(resp.plaintext);
      await loadTokens(selectedID);
    } catch (err) {
      setActionError(errorMessage(err));
    } finally {
      setIssuing(false);
    }
  };

  const handleRevoke = async (tokenID: string) => {
    if (!selectedID) {
      return;
    }
    setRevokingID(tokenID);
    setActionError(null);
    try {
      await client.revokeAgentToken({
        tenantId: selectedID,
        tokenId: tokenID,
      });
      await loadTokens(selectedID);
    } catch (err) {
      setActionError(errorMessage(err));
    } finally {
      setRevokingID(null);
    }
  };

  const copyPlaintext = async () => {
    if (!plaintext || !navigator.clipboard) {
      return;
    }
    try {
      await navigator.clipboard.writeText(plaintext);
    } catch (err) {
      setActionError(errorMessage(err));
    }
  };

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>Mission</p>
        <h1 className="text-3xl font-semibold tracking-tight">
          Tenants &amp; agent tokens
        </h1>
        <p className="max-w-2xl text-sm leading-6 text-slate-600">
          Create a tenant, issue a scoped token, and use it to boot an edge
          agent. Plaintext tokens appear exactly once — copy immediately or
          revoke and re-issue.
        </p>
      </section>

      {actionError ? (
        <div className={styles.errorBanner}>{actionError}</div>
      ) : null}

      <div className={styles.twoCol}>
        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">Tenants</h2>
              <p className="text-sm text-slate-500">
                Workspaces you belong to.
              </p>
            </div>
          </div>

          {tenantsError ? (
            <div className="mt-4 text-sm text-rose-700">{tenantsError}</div>
          ) : null}

          <ul className="mt-3 flex flex-col gap-1">
            {tenants.length === 0 ? (
              <li className="px-3 py-6 text-center text-sm text-slate-500">
                No tenants yet. Create one below.
              </li>
            ) : (
              tenants.map((tenant) => {
                const isActive = tenant.id === selectedID;
                return (
                  <li key={tenant.id}>
                    <button
                      type="button"
                      onClick={() => setSelectedID(tenant.id)}
                      className={[
                        styles.row,
                        "w-full text-left",
                        isActive ? styles.rowActive : styles.rowIdle,
                      ].join(" ")}
                    >
                      <span>
                        <span className="block font-medium">{tenant.name}</span>
                        <span
                          className={[
                            "block text-xs",
                            isActive ? "text-slate-300" : "text-slate-500",
                          ].join(" ")}
                        >
                          {tenant.slug}
                        </span>
                      </span>
                      <span
                        className={[
                          "text-xs",
                          isActive ? "text-slate-300" : "text-slate-400",
                        ].join(" ")}
                      >
                        {formatTimestamp(tenant.createdAt)}
                      </span>
                    </button>
                  </li>
                );
              })
            )}
          </ul>

          <form onSubmit={handleCreate} className="mt-5 flex flex-col gap-3">
            <label className="flex flex-col gap-1 text-sm">
              <span className={styles.metaLabel}>Slug</span>
              <input
                required
                value={newSlug}
                onChange={(e) => setNewSlug(e.target.value)}
                placeholder="ecotech-demo"
                className={styles.input}
              />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span className={styles.metaLabel}>Name</span>
              <input
                required
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="Ecotech demo tenant"
                className={styles.input}
              />
            </label>
            <button
              type="submit"
              disabled={creating}
              className={styles.primaryButton}
            >
              {creating ? "Creating..." : "Create tenant"}
            </button>
          </form>
        </section>

        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">
                {selectedTenant ? selectedTenant.name : "Select a tenant"}
              </h2>
              <p className="text-sm text-slate-500">
                Agent tokens scoped to this tenant.
              </p>
            </div>
          </div>

          {tokensError ? (
            <div className="mt-4 text-sm text-rose-700">{tokensError}</div>
          ) : null}

          {plaintext ? (
            <div className="mt-4 flex flex-col gap-2">
              <div className={styles.successBanner}>
                Copy this token now — it will not be shown again.
              </div>
              <code className={styles.tokenCode}>{plaintext}</code>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => void copyPlaintext()}
                  className={styles.ghostButton}
                >
                  Copy
                </button>
                <button
                  type="button"
                  onClick={() => setPlaintext(null)}
                  className={styles.ghostButton}
                >
                  Dismiss
                </button>
              </div>
            </div>
          ) : null}

          {selectedTenant ? (
            <>
              <ul className="mt-4 divide-y divide-slate-200">
                {tokens.length === 0 ? (
                  <li className="px-3 py-6 text-center text-sm text-slate-500">
                    No tokens issued for this tenant yet.
                  </li>
                ) : (
                  tokens.map((token) => {
                    const revoked = Boolean(token.revokedAt);
                    return (
                      <li
                        key={token.id}
                        className="flex items-center justify-between gap-3 py-3"
                      >
                        <div>
                          <div className="text-sm font-medium text-slate-900">
                            {token.label}
                          </div>
                          <div className={styles.mono}>
                            id {token.id} · issued{" "}
                            {formatTimestamp(token.createdAt) || "—"}
                            {token.lastUsedAt
                              ? ` · last used ${formatTimestamp(
                                  token.lastUsedAt,
                                )}`
                              : ""}
                            {revoked
                              ? ` · revoked ${formatTimestamp(
                                  token.revokedAt,
                                )}`
                              : ""}
                          </div>
                        </div>
                        <button
                          type="button"
                          disabled={revoked || revokingID === token.id}
                          onClick={() => void handleRevoke(token.id)}
                          className={styles.dangerButton}
                        >
                          {revoked
                            ? "Revoked"
                            : revokingID === token.id
                              ? "Revoking..."
                              : "Revoke"}
                        </button>
                      </li>
                    );
                  })
                )}
              </ul>

              <form
                onSubmit={handleIssue}
                className="mt-5 flex items-end gap-3"
              >
                <label className="flex flex-1 flex-col gap-1 text-sm">
                  <span className={styles.metaLabel}>Label</span>
                  <input
                    required
                    value={newLabel}
                    onChange={(e) => setNewLabel(e.target.value)}
                    placeholder="edge-01"
                    className={styles.input}
                  />
                </label>
                <button
                  type="submit"
                  disabled={issuing}
                  className={styles.primaryButton}
                >
                  {issuing ? "Issuing..." : "Issue token"}
                </button>
              </form>
            </>
          ) : (
            <div className="px-3 py-10 text-center text-sm text-slate-500">
              Pick a tenant to manage its tokens.
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
