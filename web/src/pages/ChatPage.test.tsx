import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { Code, ConnectError } from "@connectrpc/connect";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import ChatPage from "./ChatPage";
import { AskQuestionResponseSchema } from "../gen/query/v1/query_pb";
import { QueryClientContext, type QueryClient } from "../lib/queryClient";
import { TenantClientContext, type TenantClient } from "../lib/tenantClient";

function renderWithClients(options?: {
  listTenants?: ReturnType<typeof vi.fn>;
  askQuestion?: ReturnType<typeof vi.fn>;
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

  return render(
    <TenantClientContext.Provider value={tenantClient}>
      <QueryClientContext.Provider value={queryClient}>
        <ChatPage />
      </QueryClientContext.Provider>
    </TenantClientContext.Provider>,
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
      screen.getByText(/첫 질문을 보내면 한국어 요약, 생성된 SQL/),
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

    const textarea = await screen.findByLabelText("질문");
    fireEvent.change(textarea, {
      target: { value: "지난 30일 동안 측정소별 평균 pH를 보여줘" },
    });
    fireEvent.click(screen.getByRole("button", { name: "질문 보내기" }));

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
      screen.getByText("LIMIT 1000 자동 적용"),
    ).toBeInTheDocument();
    expect(screen.getByText("실행 SQL")).toBeInTheDocument();
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

    fireEvent.click(await screen.findByRole("button", { name: "질문 보내기" }));

    expect(
      await screen.findByText("all SQL generation attempts failed"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("시맨틱 레이어가 없어 원본 스키마만 사용했습니다."),
    ).toBeInTheDocument();
    expect(screen.getByText("시도 1 · 검증")).toBeInTheDocument();
    expect(
      screen.getByText("read-only SELECT만 허용됩니다."),
    ).toBeInTheDocument();
    expect(screen.getByText("시도 2 · 실행")).toBeInTheDocument();
    expect(screen.getByText("Unknown column 'bad_column'")).toBeInTheDocument();
  });

  it("surfaces tenant loading errors", async () => {
    renderWithClients({
      listTenants: vi.fn().mockRejectedValue(new Error("tenant load failed")),
    });

    expect(await screen.findByText("tenant load failed")).toBeInTheDocument();
  });
});
