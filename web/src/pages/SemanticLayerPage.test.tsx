import {
  cleanup,
  fireEvent,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import {
  SemanticLayerStatus,
  type SemanticLayerContent,
} from "../gen/semantic/v1/semantic_pb";
import SemanticLayerPage from "./SemanticLayerPage";
import type { Locale } from "../lib/i18n";
import {
  SemanticClientContext,
  type SemanticClient,
} from "../lib/semanticClient";
import {
  TenantClientContext,
  type TenantClient,
} from "../lib/tenantClient";
import { renderWithI18n } from "../test/renderWithI18n";

function makeTimestamp(iso: string): Timestamp {
  const ms = Date.parse(iso);
  return {
    $typeName: "google.protobuf.Timestamp",
    seconds: BigInt(Math.floor(ms / 1000)),
    nanos: 0,
  } as unknown as Timestamp;
}

function makeContent(
  tableDescription = "고객 마스터 데이터",
): SemanticLayerContent {
  return {
    tables: [
      {
        tableSchema: "mission_app",
        tableName: "customers",
        tableType: "BASE TABLE",
        tableComment: "Customer master data",
        description: tableDescription,
        columns: [
          {
            tableSchema: "mission_app",
            tableName: "customers",
            columnName: "name",
            ordinalPosition: 1,
            dataType: "varchar",
            columnType: "varchar(255)",
            isNullable: false,
            columnComment: "Customer display name",
            description: "고객명",
          },
          {
            tableSchema: "mission_app",
            tableName: "customers",
            columnName: "customer_code",
            ordinalPosition: 2,
            dataType: "varchar",
            columnType: "varchar(64)",
            isNullable: false,
            columnComment: "External customer code",
            description: "외부 고객 코드",
          },
        ],
      },
    ],
    entities: [
      {
        name: "고객",
        description: "핵심 고객 엔터티",
        sourceTables: ["mission_app.customers"],
      },
    ],
    candidateMetrics: [
      {
        name: "고객 수",
        description: "전체 고객 건수",
        sourceTables: ["mission_app.customers"],
      },
    ],
  } as unknown as SemanticLayerContent;
}

function makeEmptyContent(tableDescription = ""): SemanticLayerContent {
  return {
    tables: [
      {
        tableSchema: "mission_app",
        tableName: "customers",
        tableType: "BASE TABLE",
        tableComment: "Customer master data",
        description: tableDescription,
        columns: [
          {
            tableSchema: "mission_app",
            tableName: "customers",
            columnName: "name",
            ordinalPosition: 1,
            dataType: "varchar",
            columnType: "varchar(255)",
            isNullable: false,
            columnComment: "Customer display name",
            description: "",
          },
        ],
      },
    ],
    entities: [],
    candidateMetrics: [],
  } as unknown as SemanticLayerContent;
}

function makeLayer(id: string, status: SemanticLayerStatus, content = makeContent()) {
  return {
    id,
    tenantId: "tenant-1",
    schemaVersionId: "schema-1",
    status,
    content,
    createdAt: makeTimestamp("2026-04-20T10:00:00Z"),
    approvedAt:
      status === SemanticLayerStatus.APPROVED
        ? makeTimestamp("2026-04-20T10:30:00Z")
        : undefined,
    approvedByUserId:
      status === SemanticLayerStatus.APPROVED
        ? "user_123"
        : "",
  };
}

function renderPage(options?: {
  getSemanticLayer?: ReturnType<typeof vi.fn>;
  draftSemanticLayer?: ReturnType<typeof vi.fn>;
  updateSemanticLayer?: ReturnType<typeof vi.fn>;
  approveSemanticLayer?: ReturnType<typeof vi.fn>;
  listTenants?: ReturnType<typeof vi.fn>;
}, locale?: Locale) {
  const tenantClient = {
    listTenants:
      options?.listTenants ??
      vi.fn().mockResolvedValue({
        tenants: [
          {
            id: "tenant-1",
            slug: "tenant-one",
            name: "테스트 테넌트",
            createdAt: makeTimestamp("2026-04-20T09:00:00Z"),
          },
        ],
      }),
  } as unknown as TenantClient;

  const semanticClient = {
    getSemanticLayer:
      options?.getSemanticLayer ??
      vi.fn().mockResolvedValue({
        hasSchema: false,
        needsDraft: false,
      }),
    draftSemanticLayer:
      options?.draftSemanticLayer ??
      vi.fn().mockResolvedValue({
        layer: makeLayer(
          "layer-draft",
          SemanticLayerStatus.DRAFT,
        ),
        usage: {
          provider: "anthropic",
          model: "claude-sonnet-4-6",
          cacheReadInputTokens: 0,
        },
      }),
    updateSemanticLayer:
      options?.updateSemanticLayer ??
      vi.fn().mockResolvedValue({
        layer: makeLayer(
          "layer-draft-2",
          SemanticLayerStatus.DRAFT,
        ),
      }),
    approveSemanticLayer:
      options?.approveSemanticLayer ??
      vi.fn().mockResolvedValue({
        layer: makeLayer(
          "layer-approved",
          SemanticLayerStatus.APPROVED,
        ),
      }),
  } as unknown as SemanticClient;

  return {
    tenantClient,
    semanticClient,
    ...renderWithI18n(
      <TenantClientContext.Provider value={tenantClient}>
        <SemanticClientContext.Provider value={semanticClient}>
          <SemanticLayerPage />
        </SemanticClientContext.Provider>
      </TenantClientContext.Provider>,
      { locale },
    ),
  };
}

describe("SemanticLayerPage", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows the no-schema empty state", async () => {
    renderPage({
      getSemanticLayer: vi.fn().mockResolvedValue({
        hasSchema: false,
        needsDraft: false,
      }),
    });

    expect(
      await screen.findByText("Schema not captured yet"),
    ).toBeInTheDocument();
  });

  it("shows the generate button when the latest schema has no draft", async () => {
    const getSemanticLayer = vi
      .fn()
      .mockResolvedValueOnce({
        hasSchema: true,
        needsDraft: true,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
      })
      .mockResolvedValueOnce({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-draft",
          SemanticLayerStatus.DRAFT,
        ),
      });
    const draftSemanticLayer = vi.fn().mockResolvedValue({
      layer: makeLayer(
        "layer-draft",
        SemanticLayerStatus.DRAFT,
      ),
      usage: {
        provider: "anthropic",
        model: "claude-sonnet-4-6",
        cacheReadInputTokens: 12,
      },
    });

    renderPage({ getSemanticLayer, draftSemanticLayer });

    const button = await screen.findByRole("button", {
      name: "Generate draft",
    });
    fireEvent.click(button);

    await waitFor(() =>
      expect(draftSemanticLayer).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        schemaVersionId: "schema-1",
      }),
    );
    expect(await screen.findByText("Semantic Layer")).toBeInTheDocument();
    expect(await screen.findByDisplayValue("고객 마스터 데이터")).toBeInTheDocument();
  });

  it("saves edits as a new draft row", async () => {
    const getSemanticLayer = vi
      .fn()
      .mockResolvedValueOnce({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-draft",
          SemanticLayerStatus.DRAFT,
        ),
      })
      .mockResolvedValueOnce({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-draft-2",
          SemanticLayerStatus.DRAFT,
          makeContent("수정된 고객 설명"),
        ),
      });
    const updateSemanticLayer = vi.fn().mockResolvedValue({
      layer: makeLayer(
        "layer-draft-2",
        SemanticLayerStatus.DRAFT,
        makeContent("수정된 고객 설명"),
      ),
    });

    renderPage({ getSemanticLayer, updateSemanticLayer });

    const tableTextarea = await screen.findByDisplayValue("고객 마스터 데이터");
    fireEvent.change(tableTextarea, {
      target: { value: "수정된 고객 설명" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(updateSemanticLayer).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        id: "layer-draft",
        content: expect.objectContaining({
          tables: [
            expect.objectContaining({
              description: "수정된 고객 설명",
            }),
          ],
        }),
      }),
    );
    expect(
      await screen.findByText("Saved your edits as a new draft."),
    ).toBeInTheDocument();
  });

  it("saves first and then approves when the form is dirty", async () => {
    const getSemanticLayer = vi
      .fn()
      .mockResolvedValueOnce({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-draft",
          SemanticLayerStatus.DRAFT,
        ),
      })
      .mockResolvedValueOnce({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-draft-2",
          SemanticLayerStatus.DRAFT,
          makeContent("승인 전 수정"),
        ),
      })
      .mockResolvedValueOnce({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-approved",
          SemanticLayerStatus.APPROVED,
          makeContent("승인 전 수정"),
        ),
      });

    const updateSemanticLayer = vi.fn().mockResolvedValue({
      layer: makeLayer(
        "layer-draft-2",
        SemanticLayerStatus.DRAFT,
        makeContent("승인 전 수정"),
      ),
    });
    const approveSemanticLayer = vi.fn().mockResolvedValue({
      layer: makeLayer(
        "layer-approved",
        SemanticLayerStatus.APPROVED,
        makeContent("승인 전 수정"),
      ),
    });

    renderPage({ getSemanticLayer, updateSemanticLayer, approveSemanticLayer });

    const tableTextarea = await screen.findByDisplayValue("고객 마스터 데이터");
    fireEvent.change(tableTextarea, {
      target: { value: "승인 전 수정" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() => expect(updateSemanticLayer).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(approveSemanticLayer).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        id: "layer-draft-2",
      }),
    );
    expect(
      await screen.findByText("Semantic layer approved."),
    ).toBeInTheDocument();
  });

  it("renders diff entries for changed descriptions", async () => {
    renderPage({
      getSemanticLayer: vi.fn().mockResolvedValue({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-draft",
          SemanticLayerStatus.DRAFT,
          makeContent("새 고객 설명"),
        ),
        approvedBaseline: makeLayer(
          "layer-approved",
          SemanticLayerStatus.APPROVED,
          makeContent("이전 고객 설명"),
        ),
      }),
    });

    expect(await screen.findByText("Changed")).toBeInTheDocument();
    expect(screen.getAllByText("mission_app.customers").length).toBeGreaterThan(0);
    expect(screen.getByText("이전 고객 설명")).toBeInTheDocument();
    expect(screen.getAllByText("새 고객 설명").length).toBeGreaterThan(0);
    expect(screen.getByText("고객명")).toBeInTheDocument();
  });

  it("renders empty read-only states and no diff when nothing changed", async () => {
    renderPage({
      getSemanticLayer: vi.fn().mockResolvedValue({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-approved",
          SemanticLayerStatus.APPROVED,
          makeEmptyContent("동일 설명"),
        ),
        approvedBaseline: makeLayer(
          "layer-approved",
          SemanticLayerStatus.APPROVED,
          makeEmptyContent("동일 설명"),
        ),
      }),
    });

    expect(await screen.findByText("No entities to display.")).toBeInTheDocument();
    expect(screen.getByText("No candidate metrics to display.")).toBeInTheDocument();
    expect(screen.getByText("No differences to compare.")).toBeInTheDocument();
  });

  it("renders removed diff entries when a current description is cleared", async () => {
    renderPage({
      getSemanticLayer: vi.fn().mockResolvedValue({
        hasSchema: true,
        needsDraft: false,
        latestSchema: {
          id: "schema-1",
          capturedAt: makeTimestamp("2026-04-20T10:00:00Z"),
          schemaHash: "hash-123",
          databaseName: "mission_app",
        },
        currentLayer: makeLayer(
          "layer-draft",
          SemanticLayerStatus.DRAFT,
          makeEmptyContent(""),
        ),
        approvedBaseline: makeLayer(
          "layer-approved",
          SemanticLayerStatus.APPROVED,
          makeContent("이전에 있던 설명"),
        ),
      }),
    });

    await waitFor(() =>
      expect(screen.getAllByText("Removed").length).toBeGreaterThan(0),
    );
    expect(screen.getByText("이전에 있던 설명")).toBeInTheDocument();
  });

  it("renders Korean page-owned content when locale is ko", async () => {
    renderPage(
      {
        getSemanticLayer: vi.fn().mockResolvedValue({
          hasSchema: false,
          needsDraft: false,
        }),
      },
      "ko",
    );

    expect(await screen.findByText("시맨틱 레이어")).toBeInTheDocument();
    expect(screen.getByText("스키마가 아직 없습니다")).toBeInTheDocument();
  });
});
