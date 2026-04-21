import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  ThemeProvider,
  themeStorageKey,
  useTheme,
} from "./theme";

function installMatchMedia(initialMatches = false) {
  const listeners = new Set<(event: MediaQueryListEvent) => void>();

  const mediaQueryList = {
    addEventListener: vi.fn(
      (_event: string, listener: (event: MediaQueryListEvent) => void) => {
        listeners.add(listener);
      },
    ),
    addListener: vi.fn((listener: (event: MediaQueryListEvent) => void) => {
      listeners.add(listener);
    }),
    dispatch(next: boolean) {
      mediaQueryList.matches = next;
      const event = {
        matches: next,
        media: mediaQueryList.media,
      } as MediaQueryListEvent;
      listeners.forEach((listener) => listener(event));
    },
    matches: initialMatches,
    media: "(prefers-color-scheme: dark)",
    onchange: null,
    removeEventListener: vi.fn(
      (_event: string, listener: (event: MediaQueryListEvent) => void) => {
        listeners.delete(listener);
      },
    ),
    removeListener: vi.fn(
      (listener: (event: MediaQueryListEvent) => void) => {
        listeners.delete(listener);
      },
    ),
  };

  vi.stubGlobal(
    "matchMedia",
    vi.fn().mockImplementation(() => mediaQueryList),
  );

  return mediaQueryList;
}

function Probe() {
  const { resolvedTheme, themeMode, setThemeMode } = useTheme();

  return (
    <div>
      <div data-testid="theme-mode">{themeMode}</div>
      <div data-testid="resolved-theme">{resolvedTheme}</div>
      <button type="button" onClick={() => setThemeMode("system")}>
        System
      </button>
      <button type="button" onClick={() => setThemeMode("light")}>
        Light
      </button>
      <button type="button" onClick={() => setThemeMode("dark")}>
        Dark
      </button>
    </div>
  );
}

describe("ThemeProvider", () => {
  it("defaults to system mode and uses the OS dark preference", () => {
    installMatchMedia(true);

    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );

    expect(screen.getByTestId("theme-mode")).toHaveTextContent("system");
    expect(screen.getByTestId("resolved-theme")).toHaveTextContent("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(document.documentElement.style.colorScheme).toBe("dark");
    expect(window.localStorage.getItem(themeStorageKey)).toBeNull();
  });

  it("persists explicit dark mode and applies the dark root state", () => {
    installMatchMedia(false);

    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Dark" }));

    expect(screen.getByTestId("theme-mode")).toHaveTextContent("dark");
    expect(screen.getByTestId("resolved-theme")).toHaveTextContent("dark");
    expect(window.localStorage.getItem(themeStorageKey)).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(document.documentElement.style.colorScheme).toBe("dark");
  });

  it("persists explicit light mode and clears the dark root state", () => {
    installMatchMedia(true);

    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Light" }));

    expect(screen.getByTestId("theme-mode")).toHaveTextContent("light");
    expect(screen.getByTestId("resolved-theme")).toHaveTextContent("light");
    expect(window.localStorage.getItem(themeStorageKey)).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(document.documentElement.style.colorScheme).toBe("light");
  });

  it("reacts to matchMedia changes while in system mode", () => {
    const mediaQueryList = installMatchMedia(false);

    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );

    expect(screen.getByTestId("resolved-theme")).toHaveTextContent("light");

    act(() => {
      mediaQueryList.dispatch(true);
    });

    expect(screen.getByTestId("theme-mode")).toHaveTextContent("system");
    expect(screen.getByTestId("resolved-theme")).toHaveTextContent("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(document.documentElement.style.colorScheme).toBe("dark");
  });

  it("removes persisted preference when switching back to system", () => {
    installMatchMedia(false);

    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Dark" }));
    expect(window.localStorage.getItem(themeStorageKey)).toBe("dark");

    fireEvent.click(screen.getByRole("button", { name: "System" }));

    expect(screen.getByTestId("theme-mode")).toHaveTextContent("system");
    expect(window.localStorage.getItem(themeStorageKey)).toBeNull();
  });
});
