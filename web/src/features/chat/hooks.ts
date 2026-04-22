import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import type { CanonicalQueryExample, QueryFeedbackRating, QueryRunHistoryItem } from "../../gen/query/v1/query_pb";
import type { Tenant } from "../../gen/tenant/v1/tenant_pb";
import type { QueryClient } from "../../lib/queryClient";
import type { TenantClient } from "../../lib/tenantClient";
import {
  extractErrorResult,
  normalizeError,
  type CreateCanonicalExampleArgs,
  type QueryHistoryItem,
  type SubmitQueryFeedbackArgs,
} from "./shared";

export function useChatTenants(
  tenantClient: TenantClient,
  requestedTenantID: string | null,
) {
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [tenantsError, setTenantsError] = useState<string | null>(null);

  const loadTenants = useCallback(async () => {
    try {
      const resp = await tenantClient.listTenants({});
      setTenants(resp.tenants);
      setTenantsError(null);
      if (!selectedID && resp.tenants.length > 0) {
        const requestedTenant =
          requestedTenantID != null
            ? resp.tenants.find((tenant) => tenant.id === requestedTenantID)
            : undefined;
        setSelectedID(requestedTenant?.id ?? resp.tenants[0].id);
      }
    } catch (err) {
      setTenantsError(normalizeError(err));
    }
  }, [requestedTenantID, selectedID, tenantClient]);

  useEffect(() => {
    void loadTenants();
  }, [loadTenants]);

  const selectedTenant = useMemo(
    () => tenants.find((tenant) => tenant.id === selectedID) ?? null,
    [selectedID, tenants],
  );

  return {
    loadTenants,
    selectedID,
    selectedTenant,
    setSelectedID,
    tenants,
    tenantsError,
  };
}

export function usePersistentHistory(
  queryClient: QueryClient,
  selectedID: string | null,
) {
  const [persistentHistory, setPersistentHistory] = useState<
    QueryRunHistoryItem[]
  >([]);
  const [loadingPersistentHistory, setLoadingPersistentHistory] =
    useState(false);
  const [persistentHistoryError, setPersistentHistoryError] = useState<
    string | null
  >(null);

  const loadPersistentHistory = useCallback(
    async (tenantId: string) => {
      setLoadingPersistentHistory(true);
      try {
        const response = await queryClient.listMyQueryRuns({
          tenantId,
          limit: 20,
        });
        setPersistentHistory(response.runs);
        setPersistentHistoryError(null);
      } catch (err) {
        setPersistentHistory([]);
        setPersistentHistoryError(normalizeError(err));
      } finally {
        setLoadingPersistentHistory(false);
      }
    },
    [queryClient],
  );

  useEffect(() => {
    if (!selectedID) {
      setPersistentHistory([]);
      setPersistentHistoryError(null);
      return;
    }
    void loadPersistentHistory(selectedID);
  }, [loadPersistentHistory, selectedID]);

  return {
    loadPersistentHistory,
    loadingPersistentHistory,
    persistentHistory,
    persistentHistoryError,
  };
}

export function useCanonicalExamples(
  queryClient: QueryClient,
  selectedID: string | null,
) {
  const [canonicalExamples, setCanonicalExamples] = useState<
    CanonicalQueryExample[]
  >([]);
  const [loadingExamples, setLoadingExamples] = useState(false);
  const [canonicalExamplesError, setCanonicalExamplesError] = useState<
    string | null
  >(null);
  const [viewerCanManageByTenant, setViewerCanManageByTenant] = useState<
    Record<string, boolean>
  >({});

  const loadCanonicalExamples = useCallback(
    async (tenantID: string) => {
      setLoadingExamples(true);
      try {
        const response = await queryClient.listCanonicalQueryExamples({
          tenantId: tenantID,
        });
        setCanonicalExamples(response.examples);
        setCanonicalExamplesError(null);
        setViewerCanManageByTenant((current) => ({
          ...current,
          [tenantID]: response.viewerCanManage,
        }));
      } catch (err) {
        setCanonicalExamples([]);
        setCanonicalExamplesError(normalizeError(err));
      } finally {
        setLoadingExamples(false);
      }
    },
    [queryClient],
  );

  useEffect(() => {
    if (!selectedID) {
      setCanonicalExamples([]);
      setCanonicalExamplesError(null);
      return;
    }
    void loadCanonicalExamples(selectedID);
  }, [loadCanonicalExamples, selectedID]);

  const archiveCanonicalExample = useCallback(
    async (exampleId: string) => {
      if (!selectedID) {
        return;
      }
      await queryClient.archiveCanonicalQueryExample({
        tenantId: selectedID,
        exampleId,
      });
      await loadCanonicalExamples(selectedID);
    },
    [loadCanonicalExamples, queryClient, selectedID],
  );

  return {
    archiveCanonicalExample,
    canonicalExamples,
    canonicalExamplesError,
    loadCanonicalExamples,
    loadingExamples,
    viewerCanManageByTenant,
  };
}

export function useChatMutations(
  queryClient: QueryClient,
  selectedTenant: Tenant | null,
  loadPersistentHistory: (tenantId: string) => Promise<void>,
) {
  const [history, setHistory] = useState<QueryHistoryItem[]>([]);
  const [submitting, setSubmitting] = useState(false);

  const submitQuestion = useCallback(
    async (rawQuestion: string) => {
      const trimmedQuestion = rawQuestion.trim();
      if (!selectedTenant || trimmedQuestion === "" || submitting) {
        return false;
      }

      setSubmitting(true);
      try {
        const response = await queryClient.askQuestion({
          tenantId: selectedTenant.id,
          question: trimmedQuestion,
        });
        setHistory((current) => [
          {
            id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
            tenantId: selectedTenant.id,
            tenantName: selectedTenant.name,
            question: trimmedQuestion,
            createdAt: Date.now(),
            status: "success",
            response,
            error: null,
          },
          ...current,
        ]);
        await loadPersistentHistory(selectedTenant.id);
        return true;
      } catch (err) {
        setHistory((current) => [
          {
            id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
            tenantId: selectedTenant.id,
            tenantName: selectedTenant.name,
            question: trimmedQuestion,
            createdAt: Date.now(),
            status: "error",
            response: extractErrorResult(err),
            error: normalizeError(err),
          },
          ...current,
        ]);
        await loadPersistentHistory(selectedTenant.id);
        return false;
      } finally {
        setSubmitting(false);
      }
    },
    [loadPersistentHistory, queryClient, selectedTenant, submitting],
  );

  const submitFeedback = useCallback(
    async (args: SubmitQueryFeedbackArgs) => {
      await queryClient.submitQueryFeedback({
        tenantId: args.tenantId,
        queryRunId: args.queryRunId,
        rating: args.rating as QueryFeedbackRating,
        comment: args.comment,
        correctedSql: args.correctedSql,
      });
    },
    [queryClient],
  );

  const createCanonicalExample = useCallback(
    async (args: CreateCanonicalExampleArgs) => {
      await queryClient.createCanonicalQueryExample({
        tenantId: args.tenantId,
        queryRunId: args.queryRunId,
        question: args.question,
        sql: args.sql,
        notes: args.notes,
      });
    },
    [queryClient],
  );

  return {
    createCanonicalExample,
    history,
    submitFeedback,
    submitQuestion,
    submitting,
  };
}
