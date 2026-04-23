import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import type {
  SemanticLayerContent,
  GetSemanticLayerResponse,
} from "../gen/semantic/v1/semantic_pb";
import type { Tenant } from "../gen/tenant/v1/tenant_pb";
import SemanticLayerEditor from "./SemanticLayerEditor";
import { useI18n } from "../lib/i18n";
import { useSemanticClient } from "../lib/semanticClient";
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
};

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function deepCloneContent(content: SemanticLayerContent): SemanticLayerContent {
  return JSON.parse(JSON.stringify(content)) as SemanticLayerContent;
}

function serializeContent(content: SemanticLayerContent | null): string {
  return JSON.stringify(content ?? null);
}

export default function SemanticLayerWorkspace() {
  const tenantClient = useTenantClient();
  const semanticClient = useSemanticClient();
  const { t } = useI18n();

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

  const dirty = useMemo(
    () => serializeContent(formContent) !== serializeContent(savedContent),
    [formContent, savedContent],
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
      setPageError(errorMessage(err) || t("semantic.loadErrorFallback"));
    } finally {
      setLoadingLayer(false);
    }
  }, [applyLoadedResponse, semanticClient, t]);

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
      setSuccess(t("semantic.success.draftCreated"));
      const usage = draftResp.usage;
      if (usage) {
        setNotice(
          `${t("semantic.notice.cacheUsage")}: ${usage.provider} / ${usage.model}` +
            ` · cache_read_input_tokens=${usage.cacheReadInputTokens}`,
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
        setSuccess(t("semantic.success.saved"));
      }
      return saveResp.layer?.id ?? null;
    } catch (err) {
      setPageError(errorMessage(err));
      return null;
    } finally {
      setSaving(false);
    }
  }, [formContent, loadSemanticLayer, response?.currentLayer?.id, selectedID, semanticClient, t]);

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
      setSuccess(t("semantic.success.approved"));
    } catch (err) {
      setPageError(errorMessage(err));
    } finally {
      setApproving(false);
    }
  };

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>{t("common.appLabel")}</p>
        <h1 className="text-3xl font-semibold tracking-tight">
          {t("semantic.hero.title")}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">
          {t("semantic.hero.subtitle")}
        </p>
      </section>

      {pageError ? <div className={styles.bannerError}>{pageError}</div> : null}
      {notice ? <div className={styles.bannerInfo}>{notice}</div> : null}
      {success ? <div className={styles.bannerSuccess}>{success}</div> : null}

      <div className={styles.twoCol}>
        <section className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">{t("semantic.tenants.title")}</h2>
              <p className={styles.muted}>{t("semantic.tenants.subtitle")}</p>
            </div>
          </div>

          {tenantsError ? (
            <div className="mt-4 text-sm text-rose-700">{tenantsError}</div>
          ) : null}

          <ul className="mt-4 flex flex-col gap-1">
            {tenants.length === 0 ? (
              <li className="px-3 py-6 text-center text-sm text-slate-500">
                {t("semantic.tenants.empty")}
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

        <SemanticLayerEditor
          loading={loadingLayer}
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
      </div>
    </div>
  );
}
