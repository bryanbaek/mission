import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import Layout from "./Layout";
import { localeStorageKey, useI18n } from "../lib/i18n";
import { themeStorageKey } from "../lib/theme-context";
import { renderWithI18n } from "../test/renderWithI18n";

vi.mock("@clerk/clerk-react", () => ({
  UserButton: () => <div data-testid="user-button">user</div>,
}));

function Probe() {
  const { formatDateTime, formatNumber } = useI18n();

  return (
    <div>
      <div data-testid="month">
        {formatDateTime(new Date("2026-04-20T00:00:00Z"), {
          month: "long",
          timeZone: "UTC",
        })}
      </div>
      <div data-testid="unit">
        {formatNumber(1234, {
          style: "unit",
          unit: "byte",
          unitDisplay: "long",
        })}
      </div>
    </div>
  );
}

function renderLayout() {
  return renderWithI18n(
    <MemoryRouter initialEntries={["/"]}>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Probe />} />
          <Route path="queries" element={<div>queries</div>} />
          <Route path="semantic-layer" element={<div>semantic</div>} />
          <Route path="agents" element={<div>agents</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("Layout", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.stubGlobal(
      "matchMedia",
      vi.fn().mockImplementation(() => ({
        addEventListener: vi.fn(),
        addListener: vi.fn(),
        matches: false,
        media: "(prefers-color-scheme: dark)",
        onchange: null,
        removeEventListener: vi.fn(),
        removeListener: vi.fn(),
      })),
    );
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("defaults to English, shows theme controls, and toggles nav plus formatting to Korean", () => {
    renderLayout();

    expect(screen.getByText("Tenants")).toBeInTheDocument();
    expect(screen.getByText("Ask")).toBeInTheDocument();
    expect(screen.getByText("Semantic Layer")).toBeInTheDocument();
    expect(screen.getByText("Agents")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "System" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "Light" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Dark" })).toBeInTheDocument();

    const englishMonth = screen.getByTestId("month").textContent;
    const englishUnit = screen.getByTestId("unit").textContent;

    fireEvent.click(screen.getByRole("button", { name: "Dark" }));
    fireEvent.click(screen.getByRole("button", { name: "KO" }));

    expect(screen.getByText("테넌트")).toBeInTheDocument();
    expect(screen.getByText("질문하기")).toBeInTheDocument();
    expect(screen.getByText("시맨틱 레이어")).toBeInTheDocument();
    expect(screen.getByText("에이전트")).toBeInTheDocument();
    expect(document.documentElement.classList.contains("dark")).toBe(true);

    expect(screen.getByTestId("month").textContent).not.toBe(englishMonth);
    expect(screen.getByTestId("unit").textContent).not.toBe(englishUnit);
    expect(window.localStorage.getItem(localeStorageKey)).toBe("ko");
    expect(window.localStorage.getItem(themeStorageKey)).toBe("dark");
  });

  it("restores the persisted locale and theme on remount", () => {
    const firstRender = renderLayout();

    fireEvent.click(screen.getByRole("button", { name: "Dark" }));
    fireEvent.click(screen.getByRole("button", { name: "KO" }));
    expect(window.localStorage.getItem(localeStorageKey)).toBe("ko");
    expect(window.localStorage.getItem(themeStorageKey)).toBe("dark");

    firstRender.unmount();

    renderLayout();

    expect(screen.getByText("테넌트")).toBeInTheDocument();
    expect(screen.getByText("질문하기")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "KO" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "다크" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });
});
