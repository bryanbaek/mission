import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import TenantsPage from "./TenantsPage";
import {
  TenantClientContext,
  type TenantClient,
} from "../lib/tenantClient";
import { renderWithI18n } from "../test/renderWithI18n";
import type { Locale } from "../lib/i18n";

function makeTimestamp(iso: string): Timestamp {
  const ms = Date.parse(iso);
  return {
    $typeName: "google.protobuf.Timestamp",
    seconds: BigInt(Math.floor(ms / 1000)),
    nanos: 0,
  } as unknown as Timestamp;
}

type ClientMocks = {
  listTenants: ReturnType<typeof vi.fn>;
  createTenant: ReturnType<typeof vi.fn>;
  listAgentTokens: ReturnType<typeof vi.fn>;
  issueAgentToken: ReturnType<typeof vi.fn>;
  revokeAgentToken: ReturnType<typeof vi.fn>;
};

function renderWithClient(
  mocks: Partial<ClientMocks> = {},
  locale?: Locale,
) {
  const fake: ClientMocks = {
    listTenants:
      mocks.listTenants ?? vi.fn().mockResolvedValue({ tenants: [] }),
    createTenant:
      mocks.createTenant ??
      vi.fn().mockResolvedValue({
        tenant: {
          id: "t-new",
          slug: "new",
          name: "New",
          createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
        },
      }),
    listAgentTokens:
      mocks.listAgentTokens ?? vi.fn().mockResolvedValue({ tokens: [] }),
    issueAgentToken:
      mocks.issueAgentToken ??
      vi.fn().mockResolvedValue({
        token: {
          id: "tok-new",
          label: "new",
          createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
        },
        plaintext: "plain-secret",
      }),
    revokeAgentToken:
      mocks.revokeAgentToken ?? vi.fn().mockResolvedValue({}),
  };

  const client = fake as unknown as TenantClient;
  const utils = renderWithI18n(
    <TenantClientContext.Provider value={client}>
      <TenantsPage />
    </TenantClientContext.Provider>,
    { locale },
  );
  return { ...utils, mocks: fake };
}

describe("TenantsPage", () => {
  beforeEach(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows the empty state when the user has no tenants", async () => {
    const { mocks } = renderWithClient();
    await waitFor(() => expect(mocks.listTenants).toHaveBeenCalled());
    expect(
      await screen.findByText("No tenants yet. Create one below."),
    ).toBeInTheDocument();
  });

  it("lists tenants and selects the first one by default", async () => {
    const tenants = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
      {
        id: "t2",
        slug: "two",
        name: "Tenant Two",
        createdAt: makeTimestamp("2026-04-19T11:00:00Z"),
      },
    ];
    const listTokens = vi.fn().mockImplementation(async (req) => {
      return {
        tokens: [
          {
            id: req.tenantId === "t1" ? "tok-1" : "tok-2",
            label: req.tenantId === "t1" ? "edge-1" : "edge-2",
            createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
          },
        ],
      };
    });

    const { mocks } = renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants }),
      listAgentTokens: listTokens,
    });

    expect((await screen.findAllByText("Tenant One"))[0]).toBeInTheDocument();
    expect(screen.getByText("Tenant Two")).toBeInTheDocument();
    await waitFor(() =>
      expect(mocks.listAgentTokens).toHaveBeenCalledWith({ tenantId: "t1" }),
    );
    expect(await screen.findByText("edge-1")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Tenant Two"));
    await waitFor(() =>
      expect(mocks.listAgentTokens).toHaveBeenCalledWith({ tenantId: "t2" }),
    );
    expect(await screen.findByText("edge-2")).toBeInTheDocument();
  });

  it("surfaces an error when listing tenants fails", async () => {
    const { mocks } = renderWithClient({
      listTenants: vi.fn().mockRejectedValue(new Error("unauthenticated")),
    });
    expect(await screen.findByText("unauthenticated")).toBeInTheDocument();
    expect(mocks.listAgentTokens).not.toHaveBeenCalled();
  });

  it("surfaces an error when listing tokens fails", async () => {
    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({
        tenants: [
          {
            id: "t1",
            slug: "one",
            name: "Tenant One",
            createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
          },
        ],
      }),
      listAgentTokens: vi.fn().mockRejectedValue(new Error("tokens boom")),
    });

    expect(await screen.findByText("tokens boom")).toBeInTheDocument();
  });

  it("renders Korean page-owned content when locale is ko", async () => {
    const { mocks } = renderWithClient(
      {
        listTenants: vi.fn().mockResolvedValue({ tenants: [] }),
      },
      "ko",
    );

    await waitFor(() => expect(mocks.listTenants).toHaveBeenCalled());
    expect(
      await screen.findByText("테넌트 및 에이전트 토큰"),
    ).toBeInTheDocument();
  });

  it("creates a tenant and reloads the list", async () => {
    const tenantsBefore = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
    ];
    const tenantsAfter = [
      ...tenantsBefore,
      {
        id: "t2",
        slug: "two",
        name: "Tenant Two",
        createdAt: makeTimestamp("2026-04-19T11:00:00Z"),
      },
    ];
    const listTenants = vi
      .fn()
      .mockResolvedValueOnce({ tenants: tenantsBefore })
      .mockResolvedValue({ tenants: tenantsAfter });
    const createTenant = vi.fn().mockResolvedValue({
      tenant: tenantsAfter[1],
    });

    const { mocks } = renderWithClient({ listTenants, createTenant });

    expect((await screen.findAllByText("Tenant One"))[0]).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("ecotech-demo"), {
      target: { value: "two" },
    });
    fireEvent.change(screen.getByPlaceholderText("Ecotech demo tenant"), {
      target: { value: "Tenant Two" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create tenant" }));

    await waitFor(() =>
      expect(mocks.createTenant).toHaveBeenCalledWith({
        slug: "two",
        name: "Tenant Two",
      }),
    );
    await waitFor(() =>
      expect(mocks.listAgentTokens).toHaveBeenCalledWith({ tenantId: "t2" }),
    );
    expect(
      (await screen.findAllByText("Tenant Two")).length,
    ).toBeGreaterThan(0);
  });

  it("reports creation errors without clearing the form", async () => {
    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants: [] }),
      createTenant: vi.fn().mockRejectedValue(new Error("slug taken")),
    });

    await screen.findByText("No tenants yet. Create one below.");

    fireEvent.change(screen.getByPlaceholderText("ecotech-demo"), {
      target: { value: "dupe" },
    });
    fireEvent.change(screen.getByPlaceholderText("Ecotech demo tenant"), {
      target: { value: "Dupe" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create tenant" }));

    expect(await screen.findByText("slug taken")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("ecotech-demo")).toHaveValue("dupe");
  });

  it("issues a token, shows plaintext once, and supports copy", async () => {
    const tenants = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
    ];
    const issue = vi.fn().mockResolvedValue({
      token: {
        id: "tok-99",
        label: "edge-99",
        createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
      },
      plaintext: "tenant_live_abc123",
    });
    const listTokens = vi
      .fn()
      .mockResolvedValueOnce({ tokens: [] })
      .mockResolvedValueOnce({
        tokens: [
          {
            id: "tok-99",
            label: "edge-99",
            createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
          },
        ],
      });

    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants }),
      listAgentTokens: listTokens,
      issueAgentToken: issue,
    });

    await screen.findAllByText("Tenant One");
    await waitFor(() => expect(listTokens).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByPlaceholderText("edge-01"), {
      target: { value: "edge-99" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Issue token" }));

    expect(await screen.findByText("tenant_live_abc123")).toBeInTheDocument();
    expect(issue).toHaveBeenCalledWith({
      tenantId: "t1",
      label: "edge-99",
    });

    fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    await waitFor(() =>
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        "tenant_live_abc123",
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    await waitFor(() =>
      expect(screen.queryByText("tenant_live_abc123")).not.toBeInTheDocument(),
    );
  });

  it("reports clipboard failures", async () => {
    const tenants = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
    ];
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: vi.fn().mockRejectedValue(new Error("denied")),
      },
    });

    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants }),
      issueAgentToken: vi.fn().mockResolvedValue({
        token: {
          id: "tok-1",
          label: "edge-1",
          createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
        },
        plaintext: "secret",
      }),
    });

    await screen.findAllByText("Tenant One");
    fireEvent.change(screen.getByPlaceholderText("edge-01"), {
      target: { value: "edge-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Issue token" }));

    await screen.findByText("secret");
    fireEvent.click(screen.getByRole("button", { name: "Copy" }));

    expect(await screen.findByText("denied")).toBeInTheDocument();
  });

  it("no-ops copy when clipboard is not available", async () => {
    const tenants = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
    ];
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });

    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants }),
      issueAgentToken: vi.fn().mockResolvedValue({
        token: {
          id: "tok-1",
          label: "edge-1",
          createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
        },
        plaintext: "secret",
      }),
    });

    await screen.findAllByText("Tenant One");
    fireEvent.change(screen.getByPlaceholderText("edge-01"), {
      target: { value: "edge-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Issue token" }));
    await screen.findByText("secret");

    fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    // Nothing to assert other than absence of an action error.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("reports token issuance errors", async () => {
    const tenants = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
    ];
    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants }),
      issueAgentToken: vi.fn().mockRejectedValue("string error"),
    });

    await screen.findAllByText("Tenant One");
    fireEvent.change(screen.getByPlaceholderText("edge-01"), {
      target: { value: "edge-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Issue token" }));

    expect(await screen.findByText("string error")).toBeInTheDocument();
  });

  it("revokes a token and reflects the revoked state", async () => {
    const tenants = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
    ];
    const listTokens = vi
      .fn()
      .mockResolvedValueOnce({
        tokens: [
          {
            id: "tok-1",
            label: "edge-1",
            createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
            lastUsedAt: makeTimestamp("2026-04-19T12:05:00Z"),
          },
          {
            id: "tok-2",
            label: "edge-2",
            createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
            revokedAt: makeTimestamp("2026-04-19T12:10:00Z"),
          },
        ],
      })
      .mockResolvedValueOnce({
        tokens: [
          {
            id: "tok-1",
            label: "edge-1",
            createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
            lastUsedAt: makeTimestamp("2026-04-19T12:05:00Z"),
            revokedAt: makeTimestamp("2026-04-19T12:30:00Z"),
          },
          {
            id: "tok-2",
            label: "edge-2",
            createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
            revokedAt: makeTimestamp("2026-04-19T12:10:00Z"),
          },
        ],
      });
    const revoke = vi.fn().mockResolvedValueOnce({});

    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants }),
      listAgentTokens: listTokens,
      revokeAgentToken: revoke,
    });

    await screen.findByText("edge-1");
    expect(screen.getByText("Revoked")).toBeDisabled();

    const revokeButton = screen.getByRole("button", { name: "Revoke" });
    fireEvent.click(revokeButton);

    await waitFor(() =>
      expect(revoke).toHaveBeenCalledWith({
        tenantId: "t1",
        tokenId: "tok-1",
      }),
    );
    await waitFor(() =>
      expect(screen.getAllByText("Revoked")).toHaveLength(2),
    );
  });

  it("reports an error when revoke fails", async () => {
    const tenants = [
      {
        id: "t1",
        slug: "one",
        name: "Tenant One",
        createdAt: makeTimestamp("2026-04-19T10:00:00Z"),
      },
    ];
    renderWithClient({
      listTenants: vi.fn().mockResolvedValue({ tenants }),
      listAgentTokens: vi.fn().mockResolvedValue({
        tokens: [
          {
            id: "tok-1",
            label: "edge-1",
            createdAt: makeTimestamp("2026-04-19T12:00:00Z"),
          },
        ],
      }),
      revokeAgentToken: vi.fn().mockRejectedValue(new Error("denied")),
    });

    await screen.findByText("edge-1");
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    expect(await screen.findByText("denied")).toBeInTheDocument();
  });

  it("throws a clear error when used outside a provider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<TenantsPage />)).toThrow(
      /useTenantClient must be used inside a TenantClientProvider/,
    );
    spy.mockRestore();
  });
});
