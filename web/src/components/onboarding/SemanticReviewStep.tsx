import { useCallback, useEffect, useMemo, useState } from "react";

import type {
  GetSemanticLayerResponse,
  SemanticLayerContent,
} from "../../gen/semantic/v1/semantic_pb";
import type { OnboardingState } from "../../gen/onboarding/v1/onboarding_pb";
import { useI18n } from "../../lib/i18n";
import { useOnboardingClient } from "../../lib/onboardingClient";
import { useSemanticClient } from "../../lib/semanticClient";
import SemanticLayerEditor from "../SemanticLayerEditor";
import {
  deepCloneContent,
  serializeContent,
  styles,
} from "./onboardingStepUtils";

export default function SemanticReviewStep({
  tenantId,
  onApproved,
}: {
  tenantId: string;
  onApproved: (nextState: OnboardingState) => void;
}) {
  const semanticClient = useSemanticClient();
  const onboardingClient = useOnboardingClient();
  const { t } = useI18n();

  const [response, setResponse] = useState<GetSemanticLayerResponse | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [pageError, setPageError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [formContent, setFormContent] = useState<SemanticLayerContent | null>(
    null,
  );
  const [savedContent, setSavedContent] =
    useState<SemanticLayerContent | null>(null);
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

  const saveDraft = useCallback(
    async (showSuccess: boolean) => {
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
    },
    [
      formContent,
      loadSemantic,
      response?.currentLayer?.id,
      semanticClient,
      t,
      tenantId,
    ],
  );

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
