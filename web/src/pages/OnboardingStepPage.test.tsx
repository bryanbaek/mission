import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
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
    dockerRunCommand: `docker run -d --name ecotech-agent
--restart unless-stopped
-e CONTROL_PLANE_URL=https://mission.example.com
-e TENANT_TOKEN=mssn_plaintext
-e AGENT_VERSION=v0.1.0
-v /etc/ecotech-agent:/etc/agent
-v /var/lib/ecotech-agent:/var/lib/agent
registry.digitalocean.com/mission/edge-agent:v0.1.0`,
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

function renderPage(options?: {
  clientOverrides?: Partial<OnboardingClient>;
  initialState?: Record<string, unknown>;
  routeStep?: number;
}) {
  const routeStep = options?.routeStep ?? 1;
  const getState = vi.fn().mockResolvedValue({
    state: makeState({
      currentStep: routeStep,
      name: "에코텍 워크스페이스",
      ...options?.initialState,
    }),
  });
  const saveWelcome = vi.fn().mockResolvedValue({
    state: makeState({
      currentStep: 2,
      name: "에코텍 워크스페이스",
    }),
  });
  const configureDatabase = vi.fn().mockResolvedValue({
    state: makeState({
      currentStep: 4,
      name: "에코텍 워크스페이스",
      ...options?.initialState,
    }),
  });

  const onboardingClient = {
    getState,
    saveWelcome,
    configureDatabase,
    ...options?.clientOverrides,
  } as unknown as OnboardingClient;

  return {
    getState,
    saveWelcome,
    configureDatabase,
    onboardingClient,
    ...renderWithI18n(
      <OnboardingClientContext.Provider value={onboardingClient}>
        <MemoryRouter
          initialEntries={[`/onboarding/tenant-1/step-${routeStep}`]}
        >
          <Routes>
            <Route
              path={`/onboarding/:tenantId/step-${routeStep}`}
              element={<OnboardingStepPage step={routeStep} />}
            />
            <Route path="/onboarding/:tenantId/step-4" element={<div>step 4</div>} />
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

  it("builds and reuses the suggested DSN on step 3", async () => {
    const { configureDatabase } = renderPage({
      routeStep: 3,
      initialState: {
        dbHost: "db.internal",
        dbPort: 3306,
        dbName: "mysql-1",
        dbUsername: "okta_ai_ro",
        generatedPassword: "X1bXqFdQjYGVEBCvlPgnPRXN",
        dbSetupSql:
          "CREATE USER 'okta_ai_ro'@'%' IDENTIFIED BY 'X1bXqFdQjYGVEBCvlPgnPRXN';",
      },
    });

    expect(
      await screen.findByText(
        "okta_ai_ro:X1bXqFdQjYGVEBCvlPgnPRXN@tcp(db.internal:3306)/mysql-1",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "검증 후 계속" }));

    await waitFor(() => {
      expect(configureDatabase).toHaveBeenCalledWith({
        tenantId: "tenant-1",
        host: "db.internal",
        port: 3306,
        databaseName: "mysql-1",
        connectionString:
          "okta_ai_ro:X1bXqFdQjYGVEBCvlPgnPRXN@tcp(db.internal:3306)/mysql-1",
        locale: "ko",
      });
    });
  });
});
