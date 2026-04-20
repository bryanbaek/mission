import {
  cleanup,
  fireEvent,
  screen,
  waitFor,
} from "@testing-library/react";
import { Code, ConnectError } from "@connectrpc/connect";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";

import ChatPage from "./ChatPage";
import { AskQuestionResponseSchema } from "../gen/query/v1/query_pb";
import type { Locale } from "../lib/i18n";
import { QueryClientContext, type QueryClient } from "../lib/queryClient";
import {
  StarterQuestionsClientContext,
  type StarterQuestionsClient,
} from "../lib/starterQuestionsClient";
import { TenantClientContext, type TenantClient } from "../lib/tenantClient";
import { renderWithI18n } from "../test/renderWithI18n";

function renderWithClients(options?: {
  listTenants?: ReturnType<typeof vi.fn>;
  askQuestion?: ReturnType<typeof vi.fn>;
  listStarterQuestions?: ReturnType<typeof vi.fn>;
  regenerateStarterQuestions?: ReturnType<typeof vi.fn>;
  initialEntry?: string;
  locale?: Locale;
}) {
  const tenantClient = {
    listTenants:
      options?.listTenants ??
      vi.fn().mockResolvedValue({
        tenants: [{ id: "tenant-1", slug: "ecotech", name: "에코텍" }],
      }),
  } as unknown as TenantClient;

  const queryClient = {
    askQuestion:
      options?.askQuestion ??
      vi.fn().mockResolvedValue({
        sqlOriginal:
          "SELECT station_id, AVG(ph) AS avg_ph " +
          "FROM readings GROUP BY station_id",
        sqlExecuted:
          "SELECT station_id, AVG(ph) AS avg_ph " +
          "FROM readings GROUP BY station_id",
        limitInjected: false,
        columns: ["station_id", "avg_ph"],
        rows: [{ values: { station_id: "A-01", avg_ph: "7.20" } }],
        rowCount: 1n,
        elapsedMs: 42n,
        summaryKo: "측정소 A-01의 평균 pH는 7.20입니다.",
        warnings: ["승인된 시맨틱 레이어를 사용했습니다."],
        attempts: [
          {
            sql:
              "SELECT station_id, AVG(ph) AS avg_ph " +
              "FROM readings GROUP BY station_id",
            error: "",
            stage: "execution",
          },
        ],
      }),
  } as unknown as QueryClient;

  const starterQuestionsClient = {
    list:
      options?.listStarterQuestions ??
      vi.fn().mockResolvedValue({
        questions: [
          {
            id: "starter-1",
            text: "이번 달 신규 고객 수는 몇 명인가요?",
            category: "count",
            primaryTable: "customers",
            ordinal: 1,
          },
        ],
        setId: "set-1",
      }),
    regenerate:
      options?.regenerateStarterQuestions ??
      vi.fn().mockResolvedValue({
        questions: [
          {
            id: "starter-2",
            text: "최근 주문 10건을 보여주세요.",
            category: "latest",
            primaryTable: "orders",
            ordinal: 1,
          },
        ],
        setId: "set-2",
      }),
  } as unknown as StarterQuestionsClient;

  return renderWithI18n(
    <MemoryRouter initialEntries={[options?.initialEntry ?? "/chat"]}>
      <TenantClientContext.Provider value={tenantClient}>
        <QueryClientContext.Provider value={queryClient}>
          <StarterQuestionsClientContext.Provider value={starterQuestionsClient}>
            <ChatPage />
          </StarterQuestionsClientContext.Provider>
        </QueryClientContext.Provider>
      </TenantClientContext.Provider>
    </MemoryRouter>,
    { locale: options?.locale },
  );
}

describe("ChatPage", () => {
  beforeEach(() => {
    vi.spyOn(Date, "now").mockReturnValue(1_713_600_000_000);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("loads tenants and shows the empty state", async () => {
    renderWithClients();

    expect(await screen.findByText("에코텍")).toBeInTheDocument();
    expect(
      screen.getByText(/Send your first question to see the summary/),
    ).toBeInTheDocument();
  });

  it("submits a question and renders the successful response", async () => {
    const askQuestion = vi.fn().mockResolvedValue({
      sqlOriginal:
        "SELECT station_id, AVG(ph) AS avg_ph " +
        "FROM readings GROUP BY station_id",
      sqlExecuted:
        "SELECT station_id, AVG(ph) AS avg_ph " +
        "FROM readings GROUP BY station_id LIMIT 1000",
      limitInjected: true,
      columns: ["station_id", "avg_ph"],
      rows: [{ values: { station_id: "A-01", avg_ph: "7.20" } }],
      rowCount: 1n,
      elapsedMs: 42n,
      summaryKo: "측정소 A-01의 평균 pH는 7.20입니다.",
      warnings: ["안전을 위해 LIMIT 1000을(를) 자동 적용했습니다."],
      attempts: [
        {
          sql:
            "SELECT station_id, AVG(ph) AS avg_ph " +
            "FROM readings GROUP BY station_id",
          error: "",
          stage: "execution",
        },
      ],
    });

    renderWithClients({ askQuestion });

    const textarea = await screen.findByLabelText("Question");
    fireEvent.change(textarea, {
      target: { value: "지난 30일 동안 측정소별 평균 pH를 보여줘" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ask question" }));

    await waitFor(() =>
      expect(askQuestion).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        question: "지난 30일 동안 측정소별 평균 pH를 보여줘",
      }),
    );

    expect(
      await screen.findByText("측정소 A-01의 평균 pH는 7.20입니다."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("안전을 위해 LIMIT 1000을(를) 자동 적용했습니다."),
    ).toBeInTheDocument();
    expect(screen.getByText("A-01")).toBeInTheDocument();
    expect(screen.getByText("7.20")).toBeInTheDocument();
    expect(
      screen.getByText("LIMIT 1000 applied automatically"),
    ).toBeInTheDocument();
    expect(screen.getByText("Executed SQL")).toBeInTheDocument();
  });

  it("surfaces backend attempt details on failed generation", async () => {
    const askQuestion = vi.fn().mockRejectedValue(
      new ConnectError(
        "all SQL generation attempts failed",
        Code.FailedPrecondition,
        undefined,
        [
          {
            desc: AskQuestionResponseSchema,
            value: {
              sqlOriginal: "",
              sqlExecuted: "",
              limitInjected: false,
              columns: [],
              rows: [],
              rowCount: 0n,
              elapsedMs: 0n,
              summaryKo: "",
              warnings: ["시맨틱 레이어가 없어 원본 스키마만 사용했습니다."],
              attempts: [
                {
                  sql: "DELETE FROM readings",
                  error: "read-only SELECT만 허용됩니다.",
                  stage: "validation",
                },
                {
                  sql: "SELECT bad_column FROM readings",
                  error: "Unknown column 'bad_column'",
                  stage: "execution",
                },
              ],
            },
          },
        ],
      ),
    );

    renderWithClients({ askQuestion });

    fireEvent.click(await screen.findByRole("button", { name: "Ask question" }));

    expect(
      await screen.findByText("all SQL generation attempts failed"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("시맨틱 레이어가 없어 원본 스키마만 사용했습니다."),
    ).toBeInTheDocument();
    expect(screen.getByText("Attempt 1 · validation")).toBeInTheDocument();
    expect(
      screen.getByText("read-only SELECT만 허용됩니다."),
    ).toBeInTheDocument();
    expect(screen.getByText("Attempt 2 · execution")).toBeInTheDocument();
    expect(screen.getByText("Unknown column 'bad_column'")).toBeInTheDocument();
  });

  it("surfaces tenant loading errors", async () => {
    renderWithClients({
      listTenants: vi.fn().mockRejectedValue(new Error("tenant load failed")),
    });

    expect(await screen.findByText("tenant load failed")).toBeInTheDocument();
  });

  it("renders Korean page-owned content when locale is ko", async () => {
    renderWithClients({ locale: "ko" });

    expect(await screen.findByText("질문 작성")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "질문 보내기" })).toBeInTheDocument();
  });

  it("runs a starter question when a suggestion is clicked", async () => {
    const askQuestion = vi.fn().mockResolvedValue({
      sqlOriginal: "SELECT COUNT(*) AS customer_count FROM customers",
      sqlExecuted: "SELECT COUNT(*) AS customer_count FROM customers",
      limitInjected: false,
      columns: ["customer_count"],
      rows: [{ values: { customer_count: "42" } }],
      rowCount: 1n,
      elapsedMs: 18n,
      summaryKo: "이번 달 신규 고객은 42명입니다.",
      warnings: [],
      attempts: [{ sql: "SELECT COUNT(*) AS customer_count FROM customers", error: "", stage: "execution" }],
    });

    renderWithClients({ askQuestion });

    fireEvent.click(
      await screen.findByRole("button", {
        name: "이번 달 신규 고객 수는 몇 명인가요?",
      }),
    );

    await waitFor(() =>
      expect(askQuestion).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        question: "이번 달 신규 고객 수는 몇 명인가요?",
      }),
    );
  });

  it("auto-submits a queued question from search params", async () => {
    const askQuestion = vi.fn().mockResolvedValue({
      sqlOriginal: "SELECT * FROM orders ORDER BY placed_at DESC LIMIT 10",
      sqlExecuted: "SELECT * FROM orders ORDER BY placed_at DESC LIMIT 10",
      limitInjected: false,
      columns: ["id"],
      rows: [{ values: { id: "1001" } }],
      rowCount: 1n,
      elapsedMs: 21n,
      summaryKo: "최근 주문 10건을 조회했습니다.",
      warnings: [],
      attempts: [{ sql: "SELECT * FROM orders ORDER BY placed_at DESC LIMIT 10", error: "", stage: "execution" }],
    });

    renderWithClients({
      askQuestion,
      initialEntry:
        "/chat?tenant=tenant-1&q=" +
        encodeURIComponent("최근 주문 10건을 보여주세요.") +
        "&auto=1",
    });

    await waitFor(() =>
      expect(askQuestion).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        question: "최근 주문 10건을 보여주세요.",
      }),
    );
  });
});
