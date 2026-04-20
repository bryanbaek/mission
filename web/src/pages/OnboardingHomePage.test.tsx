import { cleanup, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import OnboardingHomePage from "./OnboardingHomePage";
import { WorkspaceRole } from "../gen/onboarding/v1/onboarding_pb";
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

function renderPage(listWorkspaces?: ReturnType<typeof vi.fn>) {
  const onboardingClient = {
    listWorkspaces:
      listWorkspaces ??
      vi.fn().mockResolvedValue({
        workspaces: [
          {
            tenantId: "tenant-1",
            slug: "ecotech",
            name: "에코텍",
            role: WorkspaceRole.OWNER,
            onboardingComplete: false,
            currentStep: 4,
            updatedAt: timestamp("2026-04-20T10:00:00Z"),
          },
          {
            tenantId: "tenant-2",
            slug: "factory",
            name: "팩토리랩",
            role: WorkspaceRole.OWNER,
            onboardingComplete: false,
            currentStep: 2,
            updatedAt: timestamp("2026-04-20T09:00:00Z"),
          },
        ],
      }),
  } as unknown as OnboardingClient;

  return {
    onboardingClient,
    ...renderWithI18n(
      <OnboardingClientContext.Provider value={onboardingClient}>
        <MemoryRouter initialEntries={["/onboarding"]}>
          <Routes>
            <Route path="/onboarding" element={<OnboardingHomePage />} />
            <Route path="/chat" element={<div>chat landed</div>} />
          </Routes>
        </MemoryRouter>
      </OnboardingClientContext.Provider>,
      { locale: "ko" },
    ),
  };
}

describe("OnboardingHomePage", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows the Korean workspace picker when several owner workspaces are incomplete", async () => {
    renderPage();

    expect(
      await screen.findByText("이어서 진행할 작업 공간을 선택하세요"),
    ).toBeInTheDocument();
    expect(screen.getByText("에코텍")).toBeInTheDocument();
    expect(screen.getByText("팩토리랩")).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "작업 공간 열기" }),
    ).toHaveLength(2);
  });
});
