import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";

import ReviewPage from "./ReviewPage";
import {
  QueryFeedbackRating,
  QueryPromptContextSource,
  QueryRunStatus,
  type ReviewQueueFilter,
} from "../gen/query/v1/query_pb";
import { OnboardingClientContext, type OnboardingClient } from "../lib/onboardingClient";
import { QueryClientContext, type QueryClient } from "../lib/queryClient";
import { TenantClientContext, type TenantClient } from "../lib/tenantClient";
import { renderWithI18n } from "../test/renderWithI18n";

function makeTimestamp(iso: string): Timestamp {
  const date = new Date(iso);
  return {
    $typeName: "google.protobuf.Timestamp",
    seconds: BigInt(Math.floor(date.getTime() / 1000)),
    nanos: (date.getTime() % 1000) * 1_000_000,
  } as unknown as Timestamp;
}

function makeReviewItem() {
  return {
    run: {
      id: "run-1",
      question: "What is the average pH?",
      status: QueryRunStatus.FAILED,
      promptContextSource: QueryPromptContextSource.RAW_SCHEMA,
      sqlOriginal: "SELECT missing FROM readings",
      sqlExecuted: "",
      rowCount: 0n,
      elapsedMs: 18n,
      errorStage: "execution",
      errorMessage: "Unknown column",
      warnings: ["raw schema fallback"],
      attempts: [
        {
          sql: "SELECT missing FROM readings",
          error: "Unknown column",
          stage: "execution",
        },
      ],
      createdAt: makeTimestamp("2026-04-22T10:00:00Z"),
    },
    hasFeedback: true,
    latestFeedback: {
      queryRunId: "run-1",
      rating: QueryFeedbackRating.DOWN,
      comment: "Use the approved aggregate instead.",
      correctedSql: "SELECT AVG(ph) FROM readings",
      createdAt: makeTimestamp("2026-04-22T10:01:00Z"),
      updatedAt: makeTimestamp("2026-04-22T10:02:00Z"),
    },
    hasActiveCanonicalExample: false,
  };
}

function renderReviewPage(options?: {
  listReviewQueue?: ReturnType<typeof vi.fn>;
  createCanonicalQueryExample?: ReturnType<typeof vi.fn>;
  markQueryRunReviewed?: ReturnType<typeof vi.fn>;
}) {
  const tenantClient = {
    listTenants: vi.fn().mockResolvedValue({
      tenants: [
        { id: "tenant-1", slug: "ecotech", name: "Ecotech" },
        { id: "tenant-2", slug: "member-space", name: "Member Space" },
      ],
    }),
  } as unknown as TenantClient;

  const onboardingClient = {
    listWorkspaces: vi.fn().mockResolvedValue({
      workspaces: [
        {
          tenantId: "tenant-1",
          slug: "ecotech",
          name: "Ecotech",
          role: 1,
          onboardingComplete: true,
          currentStep: 7,
        },
        {
          tenantId: "tenant-2",
          slug: "member-space",
          name: "Member Space",
          role: 2,
          onboardingComplete: true,
          currentStep: 7,
        },
      ],
    }),
  } as unknown as OnboardingClient;

  const queryClient = {
    listReviewQueue:
      options?.listReviewQueue ??
      vi.fn().mockResolvedValue({
        items: [makeReviewItem()],
      }),
    createCanonicalQueryExample:
      options?.createCanonicalQueryExample ??
      vi.fn().mockResolvedValue({
        example: {
          id: "example-1",
          sourceQueryRunId: "run-1",
          schemaVersionId: "schema-1",
          question: "What is the average pH?",
          sql: "SELECT AVG(ph) FROM readings",
          notes: "",
          createdAt: makeTimestamp("2026-04-22T10:05:00Z"),
        },
      }),
    markQueryRunReviewed:
      options?.markQueryRunReviewed ??
      vi.fn().mockResolvedValue({
        queryRunId: "run-1",
        reviewedAt: makeTimestamp("2026-04-22T10:06:00Z"),
      }),
  } as unknown as QueryClient;

  return renderWithI18n(
    <MemoryRouter initialEntries={["/review"]}>
      <OnboardingClientContext.Provider value={onboardingClient}>
        <TenantClientContext.Provider value={tenantClient}>
          <QueryClientContext.Provider value={queryClient}>
            <ReviewPage />
          </QueryClientContext.Provider>
        </TenantClientContext.Provider>
      </OnboardingClientContext.Provider>
    </MemoryRouter>,
  );
}

describe("ReviewPage", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("loads owner-scoped tenants and renders the review queue", async () => {
    renderReviewPage();

    expect(await screen.findByText("Ecotech")).toBeInTheDocument();
    expect(screen.queryByText("Member Space")).not.toBeInTheDocument();
    expect(await screen.findByText("What is the average pH?")).toBeInTheDocument();
    expect(screen.getByText("Use the approved aggregate instead.")).toBeInTheDocument();

    const openInChat = screen.getByRole("link", { name: "Open in chat" });
    expect(openInChat).toHaveAttribute(
      "href",
      "/chat?auto=1&q=What+is+the+average+pH%3F&tenant=tenant-1",
    );
  });

  it("marks a review item as reviewed and refreshes the open queue", async () => {
    const listReviewQueue = vi
      .fn()
      .mockResolvedValueOnce({ items: [makeReviewItem()] })
      .mockResolvedValueOnce({ items: [] });
    const markQueryRunReviewed = vi.fn().mockResolvedValue({
      queryRunId: "run-1",
      reviewedAt: makeTimestamp("2026-04-22T10:06:00Z"),
    });

    renderReviewPage({
      listReviewQueue,
      markQueryRunReviewed,
    });

    expect(await screen.findByText("What is the average pH?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Mark reviewed" }));

    await waitFor(() =>
      expect(markQueryRunReviewed).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        queryRunId: "run-1",
      }),
    );
    expect(
      await screen.findByText(/No unresolved review items right now/),
    ).toBeInTheDocument();
  });

  it("creates a canonical example using corrected SQL and refreshes the queue", async () => {
    const listReviewQueue = vi
      .fn()
      .mockResolvedValueOnce({ items: [makeReviewItem()] })
      .mockResolvedValueOnce({ items: [] });
    const createCanonicalQueryExample = vi.fn().mockResolvedValue({
      example: {
        id: "example-1",
        sourceQueryRunId: "run-1",
        schemaVersionId: "schema-1",
        question: "What is the average pH?",
        sql: "SELECT AVG(ph) FROM readings",
        notes: "",
        createdAt: makeTimestamp("2026-04-22T10:05:00Z"),
      },
    });

    renderReviewPage({
      listReviewQueue,
      createCanonicalQueryExample,
    });

    expect(await screen.findByDisplayValue("SELECT AVG(ph) FROM readings")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save approved example" }));

    await waitFor(() =>
      expect(createCanonicalQueryExample).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        queryRunId: "run-1",
        question: "What is the average pH?",
        sql: "SELECT AVG(ph) FROM readings",
        notes: "",
      }),
    );
    expect(
      await screen.findByText(/No unresolved review items right now/),
    ).toBeInTheDocument();
  });

  it("switches the queue filter to all recent", async () => {
    const listReviewQueue = vi
      .fn()
      .mockResolvedValueOnce({ items: [makeReviewItem()] })
      .mockResolvedValueOnce({ items: [] });

    renderReviewPage({ listReviewQueue });

    expect(await screen.findByText("What is the average pH?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "All recent" }));

    await waitFor(() =>
      expect(listReviewQueue).toHaveBeenLastCalledWith({
        tenantId: "tenant-1",
        filter: 2 as ReviewQueueFilter,
        limit: 50,
      }),
    );
  });
});
