import { cleanup, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import AppAccessGate from "./AppAccessGate";
import { WorkspaceRole } from "../gen/onboarding/v1/onboarding_pb";
import {
  OnboardingClientContext,
  type OnboardingClient,
} from "../lib/onboardingClient";
import { renderWithI18n } from "../test/renderWithI18n";

function workspace(overrides?: Record<string, unknown>) {
  return {
    tenantId: "tenant-1",
    slug: "ecotech",
    name: "Ecotech",
    role: WorkspaceRole.OWNER,
    onboardingComplete: false,
    currentStep: 6,
    ...overrides,
  };
}

function renderGate(workspaces: ReturnType<typeof workspace>[]) {
  const onboardingClient = {
    listWorkspaces: vi.fn().mockResolvedValue({ workspaces }),
  } as unknown as OnboardingClient;

  return {
    onboardingClient,
    ...renderWithI18n(
      <OnboardingClientContext.Provider value={onboardingClient}>
        <MemoryRouter
          initialEntries={["/chat?tenant=tenant-1&q=Revenue&auto=1"]}
        >
          <Routes>
            <Route element={<AppAccessGate />}>
              <Route path="/chat" element={<div>chat landed</div>} />
            </Route>
            <Route
              path="/onboarding/:tenantId/step-5"
              element={<div>step 5 landed</div>}
            />
            <Route path="/onboarding" element={<div>picker landed</div>} />
          </Routes>
        </MemoryRouter>
      </OnboardingClientContext.Provider>,
    ),
  };
}

describe("AppAccessGate", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("allows chat once an owner workspace reaches the starter step", async () => {
    renderGate([workspace({ currentStep: 6 })]);

    expect(await screen.findByText("chat landed")).toBeInTheDocument();
  });

  it("redirects to onboarding while the core setup is incomplete", async () => {
    renderGate([workspace({ currentStep: 5 })]);

    expect(await screen.findByText("step 5 landed")).toBeInTheDocument();
  });

  it("redirects to the workspace picker when multiple owner workspaces are blocked", async () => {
    renderGate([
      workspace({ currentStep: 3 }),
      workspace({ tenantId: "tenant-2", currentStep: 4 }),
    ]);

    expect(await screen.findByText("picker landed")).toBeInTheDocument();
  });
});
