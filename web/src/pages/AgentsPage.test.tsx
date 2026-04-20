import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import AgentsPage from "./AgentsPage";

describe("AgentsPage", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("shows loading health status while requests are in flight", () => {
    fetchMock.mockImplementation(() => new Promise(() => {}));

    render(<AgentsPage />);

    expect(
      screen.getByText("Week 2.1 agent tunnel debug surface"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("loading")).toHaveLength(2);
    expect(fetchMock).toHaveBeenNthCalledWith(1, "/healthz");
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/debug/agents");
  });

  it(
    "renders health and agent session details after successful requests",
    async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ database: "ok", status: "ok" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          agents: [
            {
              agent_version: "",
              connected_at: "2026-04-19T12:00:00Z",
              disconnected_at: "not-a-date",
              hostname: "edge-host",
              last_heartbeat_at: "2026-04-19T12:00:05Z",
              session_id: "session-1",
              status: "online",
              tenant_id: "tenant-1",
              token_id: "token-1",
              token_label: "edge-1",
            },
          ],
        }),
      });

    render(<AgentsPage />);

    expect(await screen.findByText("edge-host")).toBeInTheDocument();
    expect(screen.getByText("edge-1")).toBeInTheDocument();
    expect(screen.getAllByText("ok")).toHaveLength(2);
    expect(screen.getByText("unknown")).toBeInTheDocument();
    expect(screen.getByText("not-a-date")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ping agent" })).toBeEnabled();
  });

  it("renders request errors", async () => {
    fetchMock.mockRejectedValue(new Error("boom"));

    render(<AgentsPage />);

    expect(await screen.findByText("boom")).toBeInTheDocument();
  });

  it("shows status-code errors during polling", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: false,
        status: 503,
        json: async () => ({}),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ agents: [] }),
      });

    render(<AgentsPage />);

    expect(
      await screen.findByText("health check failed with 503"),
    ).toBeInTheDocument();
  });

  it("shows an error when the agent debug API fails", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ database: "ok", status: "ok" }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: async () => ({}),
      });

    render(<AgentsPage />);

    expect(
      await screen.findByText("agent debug API failed with 500"),
    ).toBeInTheDocument();
  });

  it("runs ping and renders the latest ping result", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ database: "ok", status: "ok" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          agents: [
            {
              agent_version: "v1.0.0",
              connected_at: "2026-04-19T12:00:00Z",
              hostname: "edge-host",
              last_heartbeat_at: "2026-04-19T12:00:05Z",
              session_id: "session-1",
              status: "online",
              tenant_id: "tenant-1",
              token_id: "token-1",
              token_label: "edge-1",
            },
          ],
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          command_id: "command-1",
          completed_at: "2026-04-19T12:00:06Z",
          round_trip_ms: 12,
          session_id: "session-1",
        }),
      });

    render(<AgentsPage />);

    const button = await screen.findByRole("button", { name: "Ping agent" });
    fireEvent.click(button);

    expect(await screen.findByText(/12 ms at/)).toBeInTheDocument();
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/debug/agents/session-1/ping",
      { method: "POST" },
    );
  });

  it("renders ping errors from the debug API", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ database: "ok", status: "ok" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          agents: [
            {
              agent_version: "v1.0.0",
              connected_at: "2026-04-19T12:00:00Z",
              hostname: "edge-host",
              last_heartbeat_at: "2026-04-19T12:00:05Z",
              session_id: "session-1",
              status: "online",
              tenant_id: "tenant-1",
              token_id: "token-1",
              token_label: "edge-1",
            },
          ],
        }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: async () => ({ error: "session is not active" }),
      });

    render(<AgentsPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Ping agent" }));

    expect(
      await screen.findByText("session is not active"),
    ).toBeInTheDocument();
  });

  it("falls back to a generic ping error when body has no error", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ database: "ok", status: "ok" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          agents: [
            {
              agent_version: "v1.0.0",
              connected_at: "2026-04-19T12:00:00Z",
              hostname: "edge-host",
              last_heartbeat_at: "2026-04-19T12:00:05Z",
              session_id: "session-1",
              status: "online",
              tenant_id: "tenant-1",
              token_id: "token-1",
              token_label: "edge-1",
            },
          ],
        }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: async () => ({}),
      });

    render(<AgentsPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Ping agent" }));

    expect(
      await screen.findByText("ping failed with 500"),
    ).toBeInTheDocument();
  });

  it("renders the empty state for zero agents", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ database: "ok", status: "ok" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ agents: [] }),
      });

    render(<AgentsPage />);

    expect(
      await screen.findByText("No edge agents connected yet."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 sessions")).toBeInTheDocument();
  });

  it("formats ISO timestamps and reports n/a for missing values", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ database: "ok", status: "ok" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          agents: [
            {
              agent_version: "v1.0.0",
              connected_at: "2026-04-19T12:00:00Z",
              hostname: "edge-host-2",
              last_heartbeat_at: "",
              session_id: "session-offline",
              status: "offline",
              tenant_id: "tenant-1",
              token_id: "token-1",
              token_label: "edge-2",
            },
          ],
        }),
      });

    render(<AgentsPage />);

    expect(await screen.findByText("edge-host-2")).toBeInTheDocument();
    expect(screen.getByText("n/a")).toBeInTheDocument();
    expect(screen.getByText("1 session")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Ping agent" }),
    ).toBeDisabled();
  });
});
