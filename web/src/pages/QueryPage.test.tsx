import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import QueryPage from "./QueryPage";
import {
  TenantClientContext,
  type TenantClient,
} from "../lib/tenantClient";

const getToken = vi.fn().mockResolvedValue("clerk-token");

vi.mock("@clerk/clerk-react", () => ({
  useAuth: () => ({
    getToken,
  }),
}));

function renderWithClient(
  listTenants = vi.fn().mockResolvedValue({
    tenants: [{ id: "tenant-1", slug: "one", name: "Tenant One" }],
  }),
) {
  const client = {
    listTenants,
  } as unknown as TenantClient;

  return render(
    <TenantClientContext.Provider value={client}>
      <QueryPage />
    </TenantClientContext.Provider>,
  );
}

describe("QueryPage", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
    getToken.mockResolvedValue("clerk-token");
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("loads tenants and shows offline status", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ status: "offline" }),
    });

    renderWithClient();

    expect((await screen.findAllByText("Tenant One"))[0]).toBeInTheDocument();
    expect(
      await screen.findByText("Bring an edge agent online to run SQL."),
    ).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith("/api/debug/tenants/tenant-1/query", {
      headers: expect.any(Headers),
    });
  });

  it("runs a query and renders the JSON result", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "online",
          hostname: "edge-host",
          session_id: "session-1",
          agent_version: "v1",
          token_label: "edge-1",
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          command_id: "command-1",
          completed_at: "2026-04-19T12:00:00Z",
          session_id: "session-1",
          columns: ["1"],
          rows: [{ "1": 1 }],
          elapsed_ms: 15,
          database_user: "mission_ro@%",
          database_name: "mission_app",
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          status: "online",
          hostname: "edge-host",
          session_id: "session-1",
          agent_version: "v1",
          token_label: "edge-1",
        }),
      });

    renderWithClient();

    const button = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);

    expect(await screen.findByText("mission_app")).toBeInTheDocument();
    expect(screen.getByText("15 ms")).toBeInTheDocument();
    expect(screen.getByText(/"1": 1/)).toBeInTheDocument();

    await waitFor(() =>
      expect(fetchMock).toHaveBeenNthCalledWith(
        2,
        "/api/debug/tenants/tenant-1/query",
        {
          method: "POST",
          headers: expect.any(Headers),
          body: JSON.stringify({ sql: "SELECT 1" }),
        },
      ),
    );

    const postHeaders = fetchMock.mock.calls[1][1]?.headers as Headers;
    expect(postHeaders.get("Authorization")).toBe("Bearer clerk-token");
    expect(postHeaders.get("Content-Type")).toBe("application/json");
  });

  it("surfaces backend errors", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "online" }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: async () => ({ error: "no active agent connected for tenant" }),
      });

    renderWithClient();

    const button = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);

    expect(
      await screen.findByText("no active agent connected for tenant"),
    ).toBeInTheDocument();
  });

  it("surfaces explicit status errors from the response body", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 409,
      json: async () => ({ error: "edge agent unavailable" }),
    });

    renderWithClient();

    expect(
      await screen.findByText("edge agent unavailable"),
    ).toBeInTheDocument();
  });

  it("surfaces tenant-loading errors", async () => {
    renderWithClient(
      vi.fn().mockRejectedValue("tenant load failed"),
    );

    expect(await screen.findByText("tenant load failed")).toBeInTheDocument();
  });

  it("switches tenants and updates the SQL input", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "online" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "offline" }),
      });

    renderWithClient(
      vi.fn().mockResolvedValue({
        tenants: [
          { id: "tenant-1", slug: "one", name: "Tenant One" },
          { id: "tenant-2", slug: "two", name: "Tenant Two" },
        ],
      }),
    );

    expect((await screen.findAllByText("Tenant One"))[0]).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Tenant Two/i }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenNthCalledWith(
        2,
        "/api/debug/tenants/tenant-2/query",
        {
          headers: expect.any(Headers),
        },
      ),
    );

    const textarea = screen.getByPlaceholderText("SELECT 1");
    fireEvent.change(textarea, { target: { value: "SELECT * FROM orders" } });

    expect(screen.getByDisplayValue("SELECT * FROM orders")).toBeInTheDocument();
  });

  it("does not issue a query when no tenant is selected", async () => {
    renderWithClient(
      vi.fn().mockResolvedValue({
        tenants: [],
      }),
    );

    expect(
      await screen.findByText("No tenants available yet."),
    ).toBeInTheDocument();

    const textarea = screen.getByPlaceholderText("SELECT 1");
    const form = textarea.closest("form");
    if (!form) {
      throw new Error("expected query form");
    }

    fireEvent.submit(form);

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("falls back to status HTTP errors when the body has no message", async () => {
    getToken.mockResolvedValue(null);
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: async () => ({}),
    });

    renderWithClient();

    expect(
      await screen.findByText("status request failed with 503"),
    ).toBeInTheDocument();

    const firstHeaders = fetchMock.mock.calls[0][1]?.headers as Headers;
    expect(firstHeaders.get("Authorization")).toBeNull();
  });

  it("falls back to query HTTP errors when the body has no message", async () => {
    getToken.mockResolvedValue(null);
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "online" }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: async () => ({}),
      });

    renderWithClient();

    const button = await screen.findByRole("button", { name: "Run query" });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);

    expect(
      await screen.findByText("query failed with 500"),
    ).toBeInTheDocument();

    const firstHeaders = fetchMock.mock.calls[0][1]?.headers as Headers;
    expect(firstHeaders.get("Authorization")).toBeNull();
  });
});
