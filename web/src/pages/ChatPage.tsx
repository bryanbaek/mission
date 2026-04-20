import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
} from "react";
import { ConnectError } from "@connectrpc/connect";

import {
  AskQuestionResponseSchema,
  type AskQuestionResponse,
  type AttemptDebug,
} from "../gen/query/v1/query_pb";
import type { Tenant } from "../gen/tenant/v1/tenant_pb";
import { useQueryClient } from "../lib/queryClient";
import { useTenantClient } from "../lib/tenantClient";

type QueryHistoryItem = {
  id: string;
  tenantName: string;
  question: string;
  createdAt: number;
  status: "success" | "error";
  response: AskQuestionResponse | null;
  error: string | null;
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
  grid: "grid gap-6 xl:grid-cols-[280px_minmax(0,1fr)]",
  sectionCard: "rounded-3xl border border-slate-200 bg-white p-6 shadow-sm",
  sectionHeader:
    "flex items-center justify-between gap-4 border-b border-slate-200 pb-4",
  row: "flex items-center justify-between gap-3 px-3 py-2",
  rowActive: "rounded-lg bg-slate-950 text-white",
  rowIdle: "rounded-lg hover:bg-slate-100",
  muted: "text-sm text-slate-500",
  textarea: [
    "min-h-[140px] w-full rounded-2xl border border-slate-300 px-4 py-3",
    "text-sm leading-6 text-slate-900",
    "focus:border-slate-950 focus:outline-none",
  ].join(" "),
  primaryButton: [
    "inline-flex items-center justify-center rounded-xl bg-slate-950",
    "px-4 py-2 text-sm font-medium text-white transition",
    "hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-300",
  ].join(" "),
  bannerError: [
    "rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3",
    "text-sm text-rose-700",
  ].join(" "),
  bannerInfo: [
    "rounded-2xl border border-sky-200 bg-sky-50 px-4 py-3",
    "text-sm text-sky-800",
  ].join(" "),
  chipRow: "flex flex-wrap gap-2",
  chip: [
    "rounded-full bg-slate-100 px-3 py-1",
    "text-xs font-medium text-slate-700",
  ].join(" "),
  warningChip: [
    "rounded-full bg-amber-100 px-3 py-1",
    "text-xs font-medium text-amber-800",
  ].join(" "),
  successChip: [
    "rounded-full bg-emerald-100 px-3 py-1",
    "text-xs font-medium text-emerald-800",
  ].join(" "),
  historyItem: "rounded-[28px] border border-slate-200 bg-slate-50 p-5",
  promptCard: [
    "rounded-2xl border border-slate-200 bg-white px-4 py-3",
    "text-sm text-slate-900",
  ].join(" "),
  summaryCard:
    [
      "rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-4",
      "text-sm leading-6 text-emerald-950",
    ].join(" "),
  metaGrid: "grid gap-3 md:grid-cols-3",
  metaTile: "rounded-2xl border border-slate-200 bg-white px-4 py-3",
  metaLabel: "text-xs uppercase tracking-[0.14em] text-slate-400",
  metaValue: "mt-1 text-sm font-medium text-slate-900",
  sqlBox: [
    "overflow-auto rounded-2xl border border-slate-200 bg-slate-950 p-4",
    "font-mono text-xs leading-6 text-emerald-200",
  ].join(" "),
  tableShell: "overflow-x-auto rounded-2xl border border-slate-200 bg-white",
  table:
    "min-w-full border-collapse text-left text-sm text-slate-700",
  th: [
    "border-b border-slate-200 bg-slate-50 px-4 py-3",
    "text-xs font-semibold uppercase tracking-[0.14em] text-slate-500",
  ].join(" "),
  td: "border-b border-slate-100 px-4 py-3 align-top",
  attemptItem: [
    "rounded-2xl border border-slate-200 bg-white px-4 py-4",
    "text-sm text-slate-700",
  ].join(" "),
  details: "rounded-2xl border border-slate-200 bg-white",
  detailsHeader:
    "cursor-pointer list-none px-4 py-3 text-sm font-medium text-slate-900",
  detailsBody: "border-t border-slate-200 px-4 py-4",
};

function formatDateTime(value: number): string {
  return new Intl.DateTimeFormat("ko-KR", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatInteger(value: bigint): string {
  return new Intl.NumberFormat("ko-KR").format(Number(value));
}

function normalizeError(err: unknown): string {
  return ConnectError.from(err).rawMessage;
}

function extractErrorResult(err: unknown): AskQuestionResponse | null {
  const connectErr = ConnectError.from(err);
  const [detail] = connectErr.findDetails(AskQuestionResponseSchema);
  return detail ?? null;
}

function stageLabel(stage: string): string {
  switch (stage) {
    case "generation":
      return "생성";
    case "validation":
      return "검증";
    case "execution":
      return "실행";
    default:
      return stage || "알 수 없음";
  }
}

function renderCell(
  response: AskQuestionResponse,
  rowIndex: number,
  column: string,
): string {
  const value = response.rows[rowIndex]?.values[column];
  if (value === undefined || value === "") {
    return "NULL";
  }
  return value;
}

function AttemptList({ attempts }: { attempts: AttemptDebug[] }) {
  if (attempts.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3">
      {attempts.map((attempt, index) => (
        <article
          key={`${attempt.stage}-${index}`}
          className={styles.attemptItem}
        >
          <div className="flex items-center justify-between gap-3">
            <p className="font-semibold text-slate-900">
              시도 {index + 1} · {stageLabel(attempt.stage)}
            </p>
            {attempt.error ? (
              <span className={styles.warningChip}>실패</span>
            ) : (
              <span className={styles.successChip}>성공</span>
            )}
          </div>
          {attempt.error ? (
            <p className="mt-3 rounded-xl bg-rose-50 px-3 py-2 text-rose-700">
              {attempt.error}
            </p>
          ) : (
            <p className="mt-3 text-slate-500">
              검증과 실행을 통과한 SQL입니다.
            </p>
          )}
          {attempt.sql ? (
            <pre
              className={[
                "mt-3 overflow-auto rounded-xl bg-slate-950 p-3",
                "text-xs leading-6 text-emerald-200",
              ].join(" ")}
            >
              <code>{attempt.sql}</code>
            </pre>
          ) : null}
        </article>
      ))}
    </div>
  );
}

function QueryResultCard({ item }: { item: QueryHistoryItem }) {
  const response = item.response;

  return (
    <article className={styles.historyItem}>
      <div
        className={[
          "flex flex-col gap-3",
          "md:flex-row md:items-start md:justify-between",
        ].join(" ")}
      >
        <div className="min-w-0">
          <p
            className={[
              "text-xs font-semibold uppercase tracking-[0.18em]",
              "text-slate-400",
            ].join(" ")}
          >
            {item.tenantName} · {formatDateTime(item.createdAt)}
          </p>
          <div className="mt-2">
            <p className="text-sm font-medium text-slate-500">질문</p>
            <div className={`${styles.promptCard} mt-2`}>{item.question}</div>
          </div>
        </div>
        <span
          className={
            item.status === "success" ? styles.successChip : styles.warningChip
          }
        >
          {item.status === "success" ? "완료" : "실패"}
        </span>
      </div>

      {item.error ? (
        <div className={`${styles.bannerError} mt-4`}>{item.error}</div>
      ) : null}

      {response?.summaryKo ? (
        <div className={`${styles.summaryCard} mt-4`}>
          <p
            className={[
              "text-xs font-semibold uppercase tracking-[0.18em]",
              "text-emerald-700",
            ].join(" ")}
          >
            한국어 요약
          </p>
          <p className="mt-2">{response.summaryKo}</p>
        </div>
      ) : null}

      {response?.warnings.length ? (
        <div className={`${styles.chipRow} mt-4`}>
          {response.warnings.map((warning, index) => (
            <span key={`${warning}-${index}`} className={styles.warningChip}>
              {warning}
            </span>
          ))}
        </div>
      ) : null}

      {response ? (
        <>
          <div className={`${styles.metaGrid} mt-4`}>
            <div className={styles.metaTile}>
              <p className={styles.metaLabel}>행 수</p>
              <p className={styles.metaValue}>
                {formatInteger(response.rowCount)}
              </p>
            </div>
            <div className={styles.metaTile}>
              <p className={styles.metaLabel}>실행 시간</p>
              <p className={styles.metaValue}>
                {formatInteger(response.elapsedMs)} ms
              </p>
            </div>
            <div className={styles.metaTile}>
              <p className={styles.metaLabel}>안전 제한</p>
              <p className={styles.metaValue}>
                {response.limitInjected
                  ? "LIMIT 1000 자동 적용"
                  : "추가 제한 없음"}
              </p>
            </div>
          </div>

          <details className={`${styles.details} mt-4`} open>
            <summary className={styles.detailsHeader}>결과 데이터</summary>
            <div className={styles.detailsBody}>
              {response.rows.length === 0 ? (
                <p className={styles.muted}>조건에 맞는 결과가 없습니다.</p>
              ) : (
                <div className={styles.tableShell}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        {response.columns.map((column) => (
                          <th key={column} className={styles.th} scope="col">
                            {column}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {response.rows.map((row, rowIndex) => (
                        <tr key={`${item.id}-${rowIndex}`}>
                          {response.columns.map((column) => (
                            <td
                              key={`${rowIndex}-${column}`}
                              className={styles.td}
                            >
                              {row.values[column] ??
                                renderCell(response, rowIndex, column)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </details>

          <details className={`${styles.details} mt-4`}>
            <summary className={styles.detailsHeader}>생성된 SQL과 시도 기록</summary>
            <div className={`${styles.detailsBody} flex flex-col gap-4`}>
              {response.sqlOriginal ? (
                <div>
                  <p className="text-sm font-medium text-slate-900">원본 SQL</p>
                  <pre className={`${styles.sqlBox} mt-2`}>
                    <code>{response.sqlOriginal}</code>
                  </pre>
                </div>
              ) : null}
              {response.sqlExecuted ? (
                <div>
                  <p className="text-sm font-medium text-slate-900">실행 SQL</p>
                  <pre className={`${styles.sqlBox} mt-2`}>
                    <code>{response.sqlExecuted}</code>
                  </pre>
                </div>
              ) : null}
              <div>
                <p className="text-sm font-medium text-slate-900">시도 기록</p>
                <div className="mt-2">
                  <AttemptList attempts={response.attempts} />
                </div>
              </div>
            </div>
          </details>
        </>
      ) : null}
    </article>
  );
}

export default function ChatPage() {
  const tenantClient = useTenantClient();
  const queryClient = useQueryClient();

  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [tenantsError, setTenantsError] = useState<string | null>(null);
  const [question, setQuestion] = useState(
    "지난 30일 동안 측정소별 평균 pH를 보여줘",
  );
  const [submitting, setSubmitting] = useState(false);
  const [history, setHistory] = useState<QueryHistoryItem[]>([]);

  const selectedTenant = useMemo(
    () => tenants.find((tenant) => tenant.id === selectedID) ?? null,
    [selectedID, tenants],
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
      setTenantsError(normalizeError(err));
    }
  }, [selectedID, tenantClient]);

  useEffect(() => {
    void loadTenants();
  }, [loadTenants]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedQuestion = question.trim();
    if (!selectedTenant || trimmedQuestion === "") {
      return;
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
          tenantName: selectedTenant.name,
          question: trimmedQuestion,
          createdAt: Date.now(),
          status: "success",
          response,
          error: null,
        },
        ...current,
      ]);
      setQuestion("");
    } catch (err) {
      setHistory((current) => [
        {
          id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
          tenantName: selectedTenant.name,
          question: trimmedQuestion,
          createdAt: Date.now(),
          status: "error",
          response: extractErrorResult(err),
          error: normalizeError(err),
        },
        ...current,
      ]);
    } finally {
      setSubmitting(false);
    }
  };

  const canSubmit =
    selectedTenant !== null &&
    question.trim() !== "" &&
    !submitting;

  return (
    <div className={styles.shell}>
      <section className={styles.heroCard}>
        <p className={styles.introLabel}>Mission</p>
        <h1 className="text-3xl font-semibold tracking-tight">질문하기</h1>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">
          한국어 질문을 읽기 전용 SQL로 변환하고, 실행 결과를 다시 한국어로
          요약합니다. 승인된 시맨틱 레이어가 있으면 우선 사용하고, 없으면
          초안이나 원본 스키마로 안전하게 폴백합니다.
        </p>
      </section>

      <div className={styles.grid}>
        <aside className={styles.sectionCard}>
          <div className={styles.sectionHeader}>
            <div>
              <h2 className="text-lg font-semibold">테넌트 선택</h2>
              <p className={styles.muted}>질문을 보낼 작업 공간을 고르세요.</p>
            </div>
          </div>

          {tenantsError ? (
            <div className={`${styles.bannerError} mt-4`}>{tenantsError}</div>
          ) : null}

          <ul className="mt-4 flex flex-col gap-1">
            {tenants.length === 0 ? (
              <li className="px-3 py-6 text-center text-sm text-slate-500">
                사용 가능한 테넌트가 없습니다.
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
                            active ? "text-slate-300" : "text-slate-400",
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

          <div className={`${styles.bannerInfo} mt-4`}>
            SELECT, WITH, SHOW만 허용됩니다. 위험한 구문은 차단되고, 필요하면
            LIMIT 1000이 자동 적용됩니다.
          </div>
        </aside>

        <div className="flex flex-col gap-6">
          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className="text-lg font-semibold">질문 작성</h2>
                <p className={styles.muted}>
                  {selectedTenant
                    ? `${selectedTenant.name}에 대해 자연어로 물어보세요.`
                    : "먼저 테넌트를 선택하세요."}
                </p>
              </div>
              {selectedTenant ? (
                <span className={styles.chip}>{selectedTenant.slug}</span>
              ) : null}
            </div>

            <form onSubmit={handleSubmit} className="mt-5 flex flex-col gap-4">
              <label
                className="text-sm font-medium text-slate-900"
                htmlFor="chat-question"
              >
                질문
              </label>
              <textarea
                id="chat-question"
                value={question}
                onChange={(event) => setQuestion(event.target.value)}
                className={styles.textarea}
                placeholder={
                  "예: 지난 분기 공정별 평균 수질 점수와 " +
                  "가장 문제가 많은 측정소를 보여줘"
                }
              />
              <div
                className={[
                  "flex flex-col gap-3",
                  "md:flex-row md:items-center md:justify-between",
                ].join(" ")}
              >
                <p className={styles.muted}>
                  결과에는 SQL 원문, 실제 실행 SQL, 시도 기록이 함께 표시됩니다.
                </p>
                <button
                  type="submit"
                  className={styles.primaryButton}
                  disabled={!canSubmit}
                >
                  {submitting ? "질문 처리 중..." : "질문 보내기"}
                </button>
              </div>
            </form>
          </section>

          <section className={styles.sectionCard}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className="text-lg font-semibold">최근 질문</h2>
                <p className={styles.muted}>
                  같은 브라우저 세션 안에서 보낸 질문과 응답을 임시로 보여줍니다.
                </p>
              </div>
              {history.length > 0 ? (
                <span className={styles.chip}>{history.length}개</span>
              ) : null}
            </div>

            {history.length === 0 ? (
              <div className={`${styles.bannerInfo} mt-4`}>
                첫 질문을 보내면 한국어 요약, 생성된 SQL, 결과 테이블이 여기에
                나타납니다.
              </div>
            ) : (
              <div className="mt-4 flex flex-col gap-4">
                {history.map((item) => (
                  <QueryResultCard key={item.id} item={item} />
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}
