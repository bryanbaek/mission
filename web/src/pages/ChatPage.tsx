import {
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { useSearchParams } from "react-router-dom";

import { useI18n } from "../lib/i18n";
import { useQueryClient } from "../lib/queryClient";
import { useTenantClient } from "../lib/tenantClient";
import {
  useCanonicalExamples,
  useChatMutations,
  useChatTenants,
  usePersistentHistory,
} from "../features/chat/hooks";
import { styles } from "../features/chat/shared";
import {
  CanonicalExamplesPanel,
  PersistentHistoryPanel,
  QueryComposerPanel,
  SessionHistoryPanel,
  TenantSelectionPanel,
} from "../features/chat/ui";

export default function ChatPage() {
  const tenantClient = useTenantClient();
  const queryClient = useQueryClient();
  const { locale, t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();

  const requestedTenantID = searchParams.get("tenant");
  const {
    selectedID,
    selectedTenant,
    setSelectedID,
    tenants,
    tenantsError,
  } = useChatTenants(tenantClient, requestedTenantID);
  const {
    loadPersistentHistory,
    loadingPersistentHistory,
    persistentHistory,
    persistentHistoryError,
  } = usePersistentHistory(queryClient, selectedID);
  const {
    archiveCanonicalExample,
    canonicalExamples,
    canonicalExamplesError,
    loadCanonicalExamples,
    loadingExamples,
    viewerCanManageByTenant,
  } = useCanonicalExamples(queryClient, selectedID);
  const {
    createCanonicalExample,
    history,
    submitFeedback,
    submitQuestion,
    submitting,
  } = useChatMutations(queryClient, selectedTenant, loadPersistentHistory, locale);

  const defaultQuestion = t("chat.form.defaultQuestion");
  const previousDefaultQuestion = useRef(defaultQuestion);
  const [question, setQuestion] = useState(defaultQuestion);
  const autoSubmittedRef = useRef(false);

  useEffect(() => {
    setQuestion((current) =>
      current === previousDefaultQuestion.current ? defaultQuestion : current,
    );
    previousDefaultQuestion.current = defaultQuestion;
  }, [defaultQuestion]);

  useEffect(() => {
    if (searchParams.get("auto") !== "1") {
      autoSubmittedRef.current = false;
    }
  }, [searchParams]);

  const runQuestion = (text: string) => {
    setQuestion(text);
    void submitQuestion(text).then((submitted) => {
      if (submitted) {
        setQuestion("");
      }
    });
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const submitted = await submitQuestion(question);
    if (submitted) {
      setQuestion("");
    }
  };

  useEffect(() => {
    const auto = searchParams.get("auto");
    const queuedQuestion = searchParams.get("q")?.trim() ?? "";
    if (
      auto !== "1" ||
      queuedQuestion === "" ||
      selectedTenant == null ||
      autoSubmittedRef.current
    ) {
      return;
    }

    autoSubmittedRef.current = true;
    setQuestion(queuedQuestion);
    void submitQuestion(queuedQuestion)
      .then((submitted) => {
        if (submitted) {
          setQuestion("");
        }
      })
      .finally(() => {
        const nextParams = new URLSearchParams(searchParams);
        nextParams.delete("auto");
        nextParams.delete("q");
        nextParams.delete("tenant");
        setSearchParams(nextParams, { replace: true });
      });
  }, [searchParams, selectedTenant, setSearchParams, submitQuestion]);

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>{t("common.appLabel")}</p>
        <h1 className="text-3xl font-semibold tracking-tight">
          {t("chat.hero.title")}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">
          {t("chat.hero.subtitle")}
        </p>
      </section>

      <div className={styles.grid}>
        <aside>
          <TenantSelectionPanel
            tenants={tenants}
            selectedID={selectedID}
            tenantsError={tenantsError}
            onSelect={setSelectedID}
          />

          {selectedID ? (
            <CanonicalExamplesPanel
              examples={canonicalExamples}
              canManage={viewerCanManageByTenant[selectedID] ?? false}
              loading={loadingExamples}
              error={canonicalExamplesError}
              onArchive={archiveCanonicalExample}
            />
          ) : null}
        </aside>

        <div className="flex flex-col gap-6">
          <QueryComposerPanel
            history={history}
            question={question}
            selectedTenant={selectedTenant}
            submitting={submitting}
            onQuestionChange={setQuestion}
            onRunQuestion={runQuestion}
            onSubmit={handleSubmit}
          />

          {selectedID ? (
            <PersistentHistoryPanel
              runs={persistentHistory}
              loading={loadingPersistentHistory}
              error={persistentHistoryError}
              onRerun={runQuestion}
              locale={locale}
            />
          ) : null}

          <SessionHistoryPanel
            history={history}
            locale={locale}
            viewerCanManageByTenant={viewerCanManageByTenant}
            onSubmitFeedback={submitFeedback}
            onCreateCanonicalExample={createCanonicalExample}
            onCanonicalExampleChanged={loadCanonicalExamples}
          />
        </div>
      </div>
    </div>
  );
}
