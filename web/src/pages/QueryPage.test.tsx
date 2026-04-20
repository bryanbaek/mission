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
});
