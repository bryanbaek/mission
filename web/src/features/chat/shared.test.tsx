import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it } from "vitest";

import {
  AskQuestionResponseSchema,
  QueryPromptContextSource,
  QueryRunStatus,
} from "../../gen/query/v1/query_pb";
import { useI18n } from "../../lib/i18n";
import { renderWithI18n } from "../../test/renderWithI18n";
import { errorMessage } from "../../lib/errorUtils";
import {
  extractErrorResult,
  historyCountLabel,
  queryPromptContextLabel,
  queryRunStatusChipClass,
  queryRunStatusLabel,
  ratingButtonClass,
  renderCell,
  stageLabel,
  timestampToMillis,
} from "./shared";

function TranslationProbe({
  onReady,
}: {
  onReady: (t: ReturnType<typeof useI18n>["t"]) => void;
}) {
  const { t } = useI18n();
  onReady(t);
  return <div data-testid="probe" />;
}

function getTranslator(locale: "en" | "ko" = "en") {
  let translate: ReturnType<typeof useI18n>["t"] | null = null;
  renderWithI18n(
    <TranslationProbe
      onReady={(t) => {
        translate = t;
      }}
    />,
    { locale },
  );
  if (!translate) {
    throw new Error("translator was not initialized");
  }
  return translate;
}

function makeTimestamp(iso: string): Timestamp {
  const date = new Date(iso);
  return {
    $typeName: "google.protobuf.Timestamp",
    seconds: BigInt(Math.floor(date.getTime() / 1000)),
    nanos: (date.getTime() % 1000) * 1_000_000,
  } as unknown as Timestamp;
}

describe("chat shared helpers", () => {
  it("formats history counts for English and Korean", () => {
    const enT = getTranslator("en");
    const koT = getTranslator("ko");

    expect(historyCountLabel(1, "en", "1", enT)).toBe("1 item");
    expect(historyCountLabel(3, "en", "3", enT)).toBe("3 items");
    expect(historyCountLabel(4, "ko", "4", koT)).toBe("4개");
  });

  it("maps stage and status labels with unknown fallbacks", () => {
    const t = getTranslator("en");

    expect(stageLabel("generation", t)).toBe("generation");
    expect(stageLabel("validation", t)).toBe("validation");
    expect(stageLabel("execution", t)).toBe("execution");
    expect(stageLabel("other", t)).toBe("other");

    expect(queryRunStatusLabel(QueryRunStatus.RUNNING, t)).toBe("Running");
    expect(queryRunStatusLabel(QueryRunStatus.SUCCEEDED, t)).toBe("Completed");
    expect(queryRunStatusLabel(QueryRunStatus.FAILED, t)).toBe("Failed");
    expect(queryRunStatusLabel(QueryRunStatus.UNSPECIFIED, t)).toBe("unknown");
  });

  it("maps prompt context labels and status chip classes", () => {
    const t = getTranslator("en");

    expect(
      queryPromptContextLabel(QueryPromptContextSource.APPROVED, t),
    ).toBe("Approved semantic layer");
    expect(queryPromptContextLabel(QueryPromptContextSource.DRAFT, t)).toBe(
      "Draft semantic layer",
    );
    expect(
      queryPromptContextLabel(QueryPromptContextSource.RAW_SCHEMA, t),
    ).toBe("Raw schema fallback");
    expect(
      queryPromptContextLabel(QueryPromptContextSource.UNSPECIFIED, t),
    ).toBe("unknown");

    expect(queryRunStatusChipClass(QueryRunStatus.SUCCEEDED)).toContain(
      "emerald",
    );
    expect(queryRunStatusChipClass(QueryRunStatus.FAILED)).toContain("amber");
    expect(queryRunStatusChipClass(QueryRunStatus.RUNNING)).toContain("slate");
  });

  it("handles timestamps, rating classes, and cell rendering", () => {
    expect(timestampToMillis(undefined)).toBeNull();
    expect(
      timestampToMillis({
        seconds: 0n,
        nanos: 0,
      } as Timestamp),
    ).toBeNull();
    expect(timestampToMillis(makeTimestamp("2026-04-20T10:00:00Z"))).toBe(
      1_776_679_200_000,
    );

    expect(ratingButtonClass(true)).toContain("bg-slate-950");
    expect(ratingButtonClass(false)).toContain("bg-white");

    expect(
      renderCell(
        {
          rows: [{ values: { avg_ph: "7.20" } }],
        } as never,
        0,
        "avg_ph",
      ),
    ).toBe("7.20");
    expect(
      renderCell(
        {
          rows: [{ values: {} }],
        } as never,
        0,
        "missing",
      ),
    ).toBe("NULL");
  });

  it("normalizes connect errors and extracts error details", () => {
    const err = new ConnectError(
      "query failed",
      Code.FailedPrecondition,
      undefined,
      [
        {
          desc: AskQuestionResponseSchema,
          value: {
            queryRunId: "run-1",
            sqlOriginal: "SELECT 1",
            sqlExecuted: "SELECT 1",
            limitInjected: false,
            columns: [],
            rows: [],
            rowCount: 0n,
            elapsedMs: 0n,
            summaryKo: "",
            warnings: [],
            attempts: [],
          },
        },
      ],
    );

    expect(errorMessage(err)).toBe("query failed");
    expect(extractErrorResult(err)?.queryRunId).toBe("run-1");
    expect(extractErrorResult(new Error("plain error"))).toBeNull();
  });
});
