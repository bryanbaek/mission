import { ConnectError } from "@connectrpc/connect";
import { useEffect, useState } from "react";

import type { StarterQuestion } from "../gen/starter/v1/starter_pb";
import ErrorBanner from "./ErrorBanner";
import { useI18n } from "../lib/i18n";
import { useStarterQuestionsClient } from "../lib/starterQuestionsClient";

const styles = {
  shell: "rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm",
  header: "flex flex-wrap items-start justify-between gap-4",
  subtitle: "mt-2 max-w-2xl text-sm leading-6 text-slate-600",
  regenerateButton: [
    "inline-flex items-center justify-center gap-2 rounded-xl border border-slate-300",
    "bg-white px-4 py-2 text-sm font-medium text-slate-700 transition",
    "hover:bg-slate-50 disabled:cursor-not-allowed disabled:text-slate-300",
  ].join(" "),
  loadingBox: [
    "mt-5 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3",
    "text-sm text-slate-600",
  ].join(" "),
  emptyBox: [
    "mt-5 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3",
    "text-sm text-slate-600",
  ].join(" "),
  grid: "mt-5 grid gap-3 md:grid-cols-2",
  questionButton: [
    "rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4 text-left",
    "text-sm leading-6 text-slate-900 transition hover:border-slate-300 hover:bg-white",
    "disabled:cursor-not-allowed disabled:opacity-60",
  ].join(" "),
};

function normalizeStarterQuestionsError(err: unknown): string {
  const message = ConnectError.from(err).rawMessage.trim();
  return message;
}

function sortQuestions(questions: StarterQuestion[]): StarterQuestion[] {
  return [...questions].sort((left, right) => left.ordinal - right.ordinal);
}

export default function StarterQuestions({
  tenantId,
  onPick,
}: {
  tenantId: string;
  onPick: (text: string) => void;
}) {
  const client = useStarterQuestionsClient();
  const { t, locale } = useI18n();

  const [questions, setQuestions] = useState<StarterQuestion[]>([]);
  const [loading, setLoading] = useState(true);
  const [regenerating, setRegenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    void client
      .list({ tenantId, locale })
      .then((response) => {
        if (cancelled) {
          return;
        }
        setQuestions(sortQuestions(response.questions));
      })
      .catch((err) => {
        if (cancelled) {
          return;
        }
        const detail = normalizeStarterQuestionsError(err);
        setQuestions([]);
        setError(
          detail ? `${t("starterQuestions.error")} (${detail})` : t("starterQuestions.error"),
        );
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [client, locale, t, tenantId]);

  const handleRegenerate = async () => {
    setRegenerating(true);
    setError(null);
    try {
      const response = await client.regenerate({ tenantId, locale });
      setQuestions(sortQuestions(response.questions));
    } catch (err) {
      const detail = normalizeStarterQuestionsError(err);
      setError(
        detail ? `${t("starterQuestions.error")} (${detail})` : t("starterQuestions.error"),
      );
    } finally {
      setRegenerating(false);
    }
  };

  return (
    <section className={styles.shell}>
      <div className={styles.header}>
        <div>
          <h3 className="text-lg font-semibold text-slate-900">
            {t("starterQuestions.title")}
          </h3>
          <p className={styles.subtitle}>{t("starterQuestions.subtitle")}</p>
        </div>
        <button
          type="button"
          className={styles.regenerateButton}
          onClick={() => void handleRegenerate()}
          disabled={loading || regenerating}
        >
          {regenerating ? (
            <span className="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-slate-900" />
          ) : null}
          {t("starterQuestions.regenerate")}
        </button>
      </div>

      {loading ? (
        <div className={styles.loadingBox}>{t("starterQuestions.loading")}</div>
      ) : null}

      {!loading ? <ErrorBanner message={error} className="mt-5" /> : null}

      {!loading && !error && questions.length === 0 ? (
        <div className={styles.emptyBox}>{t("starterQuestions.empty")}</div>
      ) : null}

      {!loading && questions.length > 0 ? (
        <div className={styles.grid}>
          {questions.map((question) => (
            <button
              key={question.id}
              type="button"
              className={styles.questionButton}
              onClick={() => onPick(question.text)}
              disabled={regenerating}
            >
              {question.text}
            </button>
          ))}
        </div>
      ) : null}
    </section>
  );
}
