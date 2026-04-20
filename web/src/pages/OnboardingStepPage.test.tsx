import { cleanup, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import OnboardingStepPage from "./OnboardingStepPage";
import {
  OnboardingClientContext,
  type OnboardingClient,
} from "../lib/onboardingClient";
import { renderWithI18n } from "../test/renderWithI18n";

function timestamp(iso: string): Timestamp {
  const ms = Date.parse(iso);
  return {
    $typeName: "google.protobuf.Timestamp",
    seconds: BigInt(Math.floor(ms / 1000)),
    nanos: 0,
  } as unknown as Timestamp;
}

function makeState(overrides?: Record<string, unknown>) {
  return {
    tenantId: "tenant-1",
    slug: "ecotech",
    name: "에코텍",
    role: 1,
    onboardingComplete: false,
    currentStep: 2,
    primaryLanguage: "ko",
    installSlug: "ecotech",
    dockerRunCommand: "docker run -d --name ecotech-agent",
    agentTokenId: "token-1",
    agentTokenPlaintext: "mssn_plaintext",
    agentSessionId: "",
    agentConnected: false,
    agentWaitStartedAt: timestamp("2026-04-20T10:00:00Z"),
    agentConnectedAt: undefined,
    agentConnectionTimedOut: false,
    dbHost: "",
    dbPort: 3306,
    dbName: "",
    dbUsername: "",
    generatedPassword: "",
    dbSetupSql: "",
    dbVerifiedAt: undefined,
    dbErrorCode: "",
    dbErrorMessageKo: "",
    schemaVersionId: "",
    schemaTableCount: 0,
    schemaColumnCount: 0,
    schemaForeignKeyCount: 0,
    semanticLayerId: "",
    semanticApprovedAt: undefined,
    updatedAt: timestamp("2026-04-20T10:00:00Z"),
    canEdit: true,
    waitingForOwner: false,
    invites: [],
    ...overrides,
  };
}

function renderPage(clientOverrides?: Partial<OnboardingClient>) {
  const getState = vi.fn().mockResolvedValue({
    state: makeState({
      currentStep: 1,
      name: "에코텍 워크스페이스",
    }),
  });
  const saveWelcome = vi.fn().mockResolvedValue({
    state: makeState({
      currentStep: 2,
      name: "에코텍 워크스페이스",
    }),
  });

  const onboardingClient = {
    getState,
    saveWelcome,
    ...clientOverrides,
  } as unknown as OnboardingClient;

  return {
    getState,
    saveWelcome,
    onboardingClient,
    ...renderWithI18n(
      <OnboardingClientContext.Provider value={onboardingClient}>
        <MemoryRouter initialEntries={["/onboarding/tenant-1/step-1"]}>
          <Routes>
            <Route
              path="/onboarding/:tenantId/step-1"
              element={<OnboardingStepPage step={1} />}
            />
          </Routes>
        </MemoryRouter>
      </OnboardingClientContext.Provider>,
      { locale: "ko" },
    ),
  };
}

describe("OnboardingStepPage", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("renders the Korean step 1 onboarding form", async () => {
    const { getState } = renderPage();

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "환영합니다. 작업 공간 이름을 정하세요",
      }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("작업 공간 이름")).toBeInTheDocument();
    expect(
      screen.getByText("이 작업 공간의 기본 언어가 한국어임을 확인합니다."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "다음" })).toBeInTheDocument();

    await waitFor(() => {
      expect(getState).toHaveBeenCalledWith({ tenantId: "tenant-1" });
    });
  });
});
