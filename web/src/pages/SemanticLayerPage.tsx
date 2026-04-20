import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import { SemanticLayerStatus, type GetSemanticLayerResponse, type SemanticLayer, type SemanticLayerContent } from "../gen/semantic/v1/semantic_pb";
import type { Tenant } from "../gen/tenant/v1/tenant_pb";
import { t } from "../lib/i18n";
import { useSemanticClient } from "../lib/semanticClient";
import { useTenantClient } from "../lib/tenantClient";

type DiffKind = "added" | "changed" | "removed";

type DiffItem = {
  kind: DiffKind;
  scope: "table" | "column";
  name: string;
  before: string;
  after: string;
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
  twoCol: "grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]",
  sectionCard: "rounded-3xl border border-slate-200 bg-white p-6 shadow-sm",
  sectionHeader:
    "flex items-center justify-between gap-4 border-b border-slate-200 pb-4",
  row: "flex items-center justify-between gap-3 px-3 py-2",
  rowActive: "rounded-lg bg-slate-950 text-white",
  rowIdle: "rounded-lg hover:bg-slate-100",
  muted: "text-sm text-slate-500",
  bannerError: [
    "rounded-2xl border border-rose-200 bg-rose-50",
    "px-4 py-3 text-sm text-rose-700",
  ].join(" "),
  bannerInfo: [
    "rounded-2xl border border-sky-200 bg-sky-50",
    "px-4 py-3 text-sm text-sky-800",
  ].join(" "),
  bannerSuccess: [
    "rounded-2xl border border-emerald-200 bg-emerald-50",
    "px-4 py-3 text-sm text-emerald-800",
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
  input: [
    "w-full rounded-xl border border-slate-300 px-3 py-2 text-sm text-slate-900",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
  textarea: [
    "w-full rounded-2xl border border-slate-300 px-4 py-3 text-sm leading-6",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
  statusPill:
    "rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em]",
  statusDraft: "bg-amber-100 text-amber-700",
  statusApproved: "bg-emerald-100 text-emerald-700",
  statusArchived: "bg-slate-200 text-slate-600",
  metaGrid: "grid gap-4 md:grid-cols-2 xl:grid-cols-4",
  metaLabel: "text-xs uppercase tracking-[0.14em] text-slate-400",
  metaValue: "mt-1 text-sm font-medium text-slate-900 break-all",
  details: "rounded-2xl border border-slate-200 bg-slate-50",
  detailsHeader: "cursor-pointer list-none px-4 py-3",
  detailsBody: "border-t border-slate-200 px-4 py-4",
  columnRow:
    "grid gap-3 rounded-2xl border border-slate-200 bg-white p-4 xl:grid-cols-[240px_minmax(0,1fr)]",
  cardTitle: "text-lg font-semibold text-slate-900",
  cardSection: "flex flex-col gap-4",
  diffItem: "rounded-2xl border border-slate-200 bg-slate-50 p-4",
  readOnlyPill:
    "rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-500",
};

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function formatTimestamp(ts: Timestamp | undefined): string {
  if (!ts) {
    return "";
  }
  const ms = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  if (!Number.isFinite(ms) || ms <= 0) {
    return "";
  }
  return new Intl.DateTimeFormat("ko-KR", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(ms));
}

function deepCloneContent(content: SemanticLayerContent): SemanticLayerContent {
  return JSON.parse(JSON.stringify(content)) as SemanticLayerContent;
}

function serializeContent(
  content: SemanticLayerContent | null,
): string {
  return JSON.stringify(content ?? null);
}

function statusLabel(status: SemanticLayerStatus | undefined): string {
  switch (status) {
    case SemanticLayerStatus.DRAFT:
      return t("statusDraft");
    case SemanticLayerStatus.APPROVED:
      return t("statusApproved");
    case SemanticLayerStatus.ARCHIVED:
      return t("statusArchived");
    default:
      return "";
  }
}

function statusClass(status: SemanticLayerStatus | undefined): string {
  switch (status) {
    case SemanticLayerStatus.DRAFT:
      return [styles.statusPill, styles.statusDraft].join(" ");
    case SemanticLayerStatus.APPROVED:
      return [styles.statusPill, styles.statusApproved].join(" ");
    default:
      return [styles.statusPill, styles.statusArchived].join(" ");
  }
}

function diffKindLabel(kind: DiffKind): string {
  switch (kind) {
    case "added":
      return t("diffAdded");
    case "changed":
      return t("diffChanged");
    case "removed":
      return t("diffRemoved");
  }
}

function buildDescriptionMap(
  content: SemanticLayerContent | undefined,
): Map<string, { scope: "table" | "column"; description: string }> {
  const out = new Map<string, { scope: "table" | "column"; description: string }>();
  if (!content) {
    return out;
  }
  for (const table of content.tables) {
    out.set(`${table.tableSchema}.${table.tableName}`, {
      scope: "table",
      description: table.description.trim(),
    });
    for (const column of table.columns) {
      out.set(
        `${column.tableSchema}.${column.tableName}.${column.columnName}`,
        {
          scope: "column",
          description: column.description.trim(),
        },
      );
    }
  }
  return out;
}

function buildDiff(
  current: SemanticLayerContent | null,
  baseline: SemanticLayer | undefined,
): DiffItem[] {
  const currentMap = buildDescriptionMap(current ?? undefined);
  const baselineMap = buildDescriptionMap(baseline?.content);
  const keys = new Set([...currentMap.keys(), ...baselineMap.keys()]);
  const out: DiffItem[] = [];

  for (const key of keys) {
    const currentValue = currentMap.get(key);
    const baselineValue = baselineMap.get(key);
    const before = baselineValue?.description ?? "";
    const after = currentValue?.description ?? "";
    if (before === after) {
      continue;
    }
    const scope = currentValue?.scope ?? baselineValue?.scope ?? "column";
    if (!before && after) {
      out.push({ kind: "added", scope, name: key, before, after });
      continue;
    }
    if (before && !after) {
      out.push({ kind: "removed", scope, name: key, before, after });
      continue;
    }
    out.push({ kind: "changed", scope, name: key, before, after });
  }

  return out.sort((left, right) => left.name.localeCompare(right.name, "ko"));
}

export default function SemanticLayerPage() {
  const tenantClient = useTenantClient();
  const semanticClient = useSemanticClient();

  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [tenantsError, setTenantsError] = useState<string | null>(null);

  const [response, setResponse] = useState<GetSemanticLayerResponse | null>(null);
  const [loadingLayer, setLoadingLayer] = useState(false);
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

  const selectedTenant = useMemo(
    () => tenants.find((tenant) => tenant.id === selectedID) ?? null,
    [selectedID, tenants],
  );

  const dirty = useMemo(
    () => serializeContent(formContent) !== serializeContent(savedContent),
    [formContent, savedContent],
  );

  const diffItems = useMemo(
    () => buildDiff(formContent, response?.approvedBaseline),
    [formContent, response?.approvedBaseline],
  );

  const loadTenants = useCallback(async () => {
    try {
      const resp = await tenantClient.listTenants({});
      setTenants(resp.tenants);
      setTenantsError(null);
      if (!selectedID && resp.tenants.length > 0) {
        setSelectedID(resp.tenants[0].id);
      }
    } catch (err) {
      setTenantsError(errorMessage(err));
    }
  }, [selectedID, tenantClient]);

  useEffect(() => {
    void loadTenants();
  }, [loadTenants]);

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

  const loadSemanticLayer = useCallback(async (tenantID: string) => {
    setLoadingLayer(true);
    try {
      const resp = await semanticClient.getSemanticLayer({ tenantId: tenantID });
      applyLoadedResponse(resp);
      setPageError(null);
    } catch (err) {
      setPageError(errorMessage(err) || t("loadErrorFallback"));
    } finally {
      setLoadingLayer(false);
    }
  }, [applyLoadedResponse, semanticClient]);

  useEffect(() => {
    if (!selectedID) {
      setResponse(null);
      setFormContent(null);
      setSavedContent(null);
      return;
    }
    void loadSemanticLayer(selectedID);
  }, [loadSemanticLayer, selectedID]);

  const updateTableDescription = (
    tableIndex: number,
    description: string,
  ) => {
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
    if (!selectedID || !response?.latestSchema?.id) {
      return;
    }
    setDrafting(true);
    setPageError(null);
    setSuccess(null);
    setNotice(null);
    try {
      const draftResp = await semanticClient.draftSemanticLayer({
        tenantId: selectedID,
        schemaVersionId: response.latestSchema.id,
      });
      await loadSemanticLayer(selectedID);
      setSuccess(t("draftCreated"));
      if (draftResp.usage) {
        setNotice(
          `${t("cacheUsage")}: ${draftResp.usage.provider} / ${draftResp.usage.model} · cache_read_input_tokens=${draftResp.usage.cacheReadInputTokens}`,
        );
      }
    } catch (err) {
      setPageError(errorMessage(err));
    } finally {
      setDrafting(false);
    }
  };

  const saveDraft = useCallback(async (showSuccess: boolean) => {
    if (!selectedID || !response?.currentLayer?.id || !formContent) {
      return null;
    }
    setSaving(true);
    setPageError(null);
    if (showSuccess) {
      setSuccess(null);
    }
    try {
      const saveResp = await semanticClient.updateSemanticLayer({
        tenantId: selectedID,
        id: response.currentLayer.id,
        content: formContent,
      });
      await loadSemanticLayer(selectedID);
      if (showSuccess) {
        setSuccess(t("saved"));
      }
      return saveResp.layer?.id ?? null;
    } catch (err) {
      setPageError(errorMessage(err));
      return null;
    } finally {
      setSaving(false);
    }
  }, [formContent, loadSemanticLayer, response?.currentLayer?.id, selectedID, semanticClient]);

  const handleApprove = async () => {
    if (!selectedID || !response?.currentLayer?.id) {
      return;
    }
    setApproving(true);
    setPageError(null);
    setSuccess(null);
    try {
      let layerID = response.currentLayer.id;
      if (dirty) {
        const savedID = await saveDraft(false);
        if (!savedID) {
          return;
        }
        layerID = savedID;
      }
      await semanticClient.approveSemanticLayer({
        tenantId: selectedID,
        id: layerID,
      });
      await loadSemanticLayer(selectedID);
      setSuccess(t("approved"));
    } catch (err) {
      setPageError(errorMessage(err));
    } finally {
      setApproving(false);
    }
  };

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>{t("pageIntroLabel")}</p>
        <h1 className="text-3xl font-semibold tracking-tight">
          {t("semanticLayerTitle")}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">
          {t("semanticLayerSubtitle")}
        </p>
      </section>

      {pageError ? <div className={styles.bannerError}>{pageError}</div> : null}
      {notice ? <div className={styles.bannerInfo}>{notice}</div> : null}
      {success ? <div className={styles.bannerSuccess}>{success}</div> : null}

      <div className={styles.twoCol}>
        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">{t("tenantsTitle")}</h2>
              <p className={styles.muted}>{t("tenantsSubtitle")}</p>
            </div>
          </div>

          {tenantsError ? (
            <div className="mt-4 text-sm text-rose-700">{tenantsError}</div>
          ) : null}

          <ul className="mt-4 flex flex-col gap-1">
            {tenants.length === 0 ? (
              <li className="px-3 py-6 text-center text-sm text-slate-500">
                {t("noTenants")}
              </li>
            ) : (
              tenants.map((tenant) => {
                const active = tenant.id === selectedID;
                return (
                  <li key={tenant.id}>
                    <button
                      type="button"
                      onClick={() => {
                        setSelectedID(tenant.id);
                        setPageError(null);
                        setSuccess(null);
                        setNotice(null);
                      }}
                      className={[
                        styles.row,
                        "w-full text-left",
                        active ? styles.rowActive : styles.rowIdle,
                      ].join(" ")}
                    >
                      <span>
                        <span className="block font-medium">{tenant.name}</span>
                        <span className="text-xs opacity-70">{tenant.slug}</span>
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
                {selectedTenant?.name ?? t("semanticLayerTitle")}
              </h2>
              <p className={styles.muted}>
                {dirty ? t("dirtyHint") : t("pendingDraft")}
              </p>
            </div>
            {response?.currentLayer ? (
              <span className={statusClass(response.currentLayer.status)}>
                {statusLabel(response.currentLayer.status)}
              </span>
            ) : null}
          </div>

          {loadingLayer ? (
            <div className="py-10 text-sm text-slate-500">{t("loading")}</div>
          ) : null}

          {!loadingLayer && response && response.hasSchema && response.latestSchema ? (
            <div className={`${styles.metaGrid} mt-6`}>
              <div>
                <div className={styles.metaLabel}>{t("schemaVersion")}</div>
                <div className={styles.metaValue}>{response.latestSchema.id}</div>
              </div>
              <div>
                <div className={styles.metaLabel}>{t("schemaCapturedAt")}</div>
                <div className={styles.metaValue}>
                  {formatTimestamp(response.latestSchema.capturedAt)}
                </div>
              </div>
              <div>
                <div className={styles.metaLabel}>{t("databaseName")}</div>
                <div className={styles.metaValue}>
                  {response.latestSchema.databaseName}
                </div>
              </div>
              <div>
                <div className={styles.metaLabel}>{t("schemaHash")}</div>
                <div className={styles.metaValue}>
                  {response.latestSchema.schemaHash}
                </div>
              </div>
            </div>
          ) : null}

          {!loadingLayer && response && !response.hasSchema ? (
            <div className="mt-6 rounded-3xl border border-slate-200 bg-slate-50 p-6">
              <h3 className="text-lg font-semibold text-slate-900">
                {t("schemaNotCapturedTitle")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-slate-600">
                {t("schemaNotCapturedBody")}
              </p>
            </div>
          ) : null}

          {!loadingLayer &&
          response &&
          response.hasSchema &&
          response.needsDraft ? (
            <div className="mt-6 rounded-3xl border border-slate-200 bg-slate-50 p-6">
              <h3 className="text-lg font-semibold text-slate-900">
                {t("draftNeededTitle")}
              </h3>
              <p className="mt-2 text-sm leading-6 text-slate-600">
                {t("draftNeededBody")}
              </p>
              <button
                type="button"
                onClick={() => void handleDraft()}
                className={`${styles.primaryButton} mt-4`}
                disabled={drafting}
              >
                {drafting ? t("generating") : t("generateDraft")}
              </button>
            </div>
          ) : null}

          {!loadingLayer &&
          response?.currentLayer &&
          formContent ? (
            <div className="mt-6 grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
              <div className="flex flex-col gap-6">
                <div className="flex flex-wrap items-center gap-3">
                  <button
                    type="button"
                    onClick={() => void saveDraft(true)}
                    className={styles.secondaryButton}
                    disabled={!dirty || saving || approving}
                  >
                    {saving ? t("saving") : t("save")}
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleApprove()}
                    className={styles.primaryButton}
                    disabled={approving || saving}
                  >
                    {approving ? t("approving") : t("approve")}
                  </button>
                </div>

                <div className={styles.cardSection}>
                  <div>
                    <h3 className={styles.cardTitle}>{t("currentLayerTitle")}</h3>
                  </div>
                  {formContent.tables.map((table, tableIndex) => (
                    <details
                      key={`${table.tableSchema}.${table.tableName}`}
                      className={styles.details}
                      open={tableIndex === 0}
                    >
                      <summary className={styles.detailsHeader}>
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <div>
                            <div className="font-semibold text-slate-900">
                              {table.tableSchema}.{table.tableName}
                            </div>
                            <div className="mt-1 text-xs text-slate-500">
                              {table.tableType}
                            </div>
                          </div>
                          <div className="text-xs text-slate-500">
                            {t("columnCount")}: {table.columns.length}
                          </div>
                        </div>
                      </summary>
                      <div className={styles.detailsBody}>
                        <div className="grid gap-3">
                          <label className="grid gap-2 text-sm font-medium text-slate-800">
                            <span>{t("tableDescription")}</span>
                            <textarea
                              className={styles.textarea}
                              value={table.description}
                              onChange={(event) =>
                                updateTableDescription(
                                  tableIndex,
                                  event.target.value,
                                )
                              }
                            />
                          </label>
                          <div className="text-xs text-slate-500">
                            {t("originalComment")}: {table.tableComment || "-"}
                          </div>
                        </div>

                        <div className="mt-4 grid gap-4">
                          {table.columns.map((column, columnIndex) => (
                            <div
                              key={`${column.tableSchema}.${column.tableName}.${column.columnName}`}
                              className={styles.columnRow}
                            >
                              <div className="space-y-2">
                                <div className="font-medium text-slate-900">
                                  {column.columnName}
                                </div>
                                <div className="text-xs text-slate-500">
                                  {t("dataType")}: {column.columnType}
                                </div>
                                <div className="text-xs text-slate-500">
                                  {t("nullable")}: {column.isNullable ? t("yes") : t("no")}
                                </div>
                                <div className="text-xs text-slate-500">
                                  {t("originalComment")}: {column.columnComment || "-"}
                                </div>
                              </div>
                              <label className="grid gap-2 text-sm font-medium text-slate-800">
                                <span>{t("columnDescription")}</span>
                                <textarea
                                  className={styles.textarea}
                                  value={column.description}
                                  onChange={(event) =>
                                    updateColumnDescription(
                                      tableIndex,
                                      columnIndex,
                                      event.target.value,
                                    )
                                  }
                                />
                              </label>
                            </div>
                          ))}
                        </div>
                      </div>
                    </details>
                  ))}
                </div>

                <div className="grid gap-6 lg:grid-cols-2">
                  <section className={styles.sectionCard}>
                    <div className={styles.sectionHeader}>
                      <div>
                        <h3 className="text-lg font-semibold">{t("entitiesTitle")}</h3>
                      </div>
                      <span className={styles.readOnlyPill}>{t("readOnlyHint")}</span>
                    </div>
                    <div className="mt-4 flex flex-col gap-3">
                      {formContent.entities.length === 0 ? (
                        <p className={styles.muted}>{t("emptyEntities")}</p>
                      ) : (
                        formContent.entities.map((entity) => (
                          <div
                            key={entity.name}
                            className="rounded-2xl border border-slate-200 bg-slate-50 p-4"
                          >
                            <div className="font-medium text-slate-900">{entity.name}</div>
                            <div className="mt-2 text-sm text-slate-600">
                              {entity.description || "-"}
                            </div>
                            <div className="mt-2 text-xs text-slate-500">
                              {entity.sourceTables.join(", ") || "-"}
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </section>

                  <section className={styles.sectionCard}>
                    <div className={styles.sectionHeader}>
                      <div>
                        <h3 className="text-lg font-semibold">{t("metricsTitle")}</h3>
                      </div>
                      <span className={styles.readOnlyPill}>{t("readOnlyHint")}</span>
                    </div>
                    <div className="mt-4 flex flex-col gap-3">
                      {formContent.candidateMetrics.length === 0 ? (
                        <p className={styles.muted}>{t("emptyMetrics")}</p>
                      ) : (
                        formContent.candidateMetrics.map((metric) => (
                          <div
                            key={metric.name}
                            className="rounded-2xl border border-slate-200 bg-slate-50 p-4"
                          >
                            <div className="font-medium text-slate-900">{metric.name}</div>
                            <div className="mt-2 text-sm text-slate-600">
                              {metric.description || "-"}
                            </div>
                            <div className="mt-2 text-xs text-slate-500">
                              {metric.sourceTables.join(", ") || "-"}
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  </section>
                </div>
              </div>

              <div className="flex flex-col gap-6">
                <section className={styles.sectionCard}>
                  <div className={styles.sectionHeader}>
                    <div>
                      <h3 className="text-lg font-semibold">
                        {t("approvedBaselineTitle")}
                      </h3>
                    </div>
                  </div>
                  <div className="mt-4 flex flex-col gap-3">
                    {diffItems.length === 0 ? (
                      <p className={styles.muted}>{t("noDiff")}</p>
                    ) : (
                      diffItems.map((item) => (
                        <div key={`${item.kind}:${item.name}`} className={styles.diffItem}>
                          <div className="flex flex-wrap items-center gap-2">
                            <span className={statusClass(
                              item.kind === "added"
                                ? SemanticLayerStatus.APPROVED
                                : item.kind === "removed"
                                  ? SemanticLayerStatus.ARCHIVED
                                  : SemanticLayerStatus.DRAFT,
                            )}>
                              {diffKindLabel(item.kind)}
                            </span>
                            <span className="text-sm font-medium text-slate-900">
                              {item.scope === "table" ? t("diffTable") : t("diffColumn")}
                            </span>
                          </div>
                          <div className="mt-3 font-mono text-xs text-slate-700">
                            {item.name}
                          </div>
                          <div className="mt-3 grid gap-3">
                            <div>
                              <div className={styles.metaLabel}>{t("diffBefore")}</div>
                              <div className="mt-1 text-sm text-slate-600">
                                {item.before || "-"}
                              </div>
                            </div>
                            <div>
                              <div className={styles.metaLabel}>{t("diffAfter")}</div>
                              <div className="mt-1 text-sm text-slate-900">
                                {item.after || "-"}
                              </div>
                            </div>
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                </section>

                {response.currentLayer?.approvedByUserId ? (
                  <section className={styles.sectionCard}>
                    <div className="text-sm text-slate-600">
                      {t("approvedBy")}: {response.currentLayer.approvedByUserId}
                    </div>
                  </section>
                ) : null}
              </div>
            </div>
          ) : null}
        </section>
      </div>
    </div>
  );
}
