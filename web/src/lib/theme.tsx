import {
  createContext,
  createElement,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

export type ThemeMode = "system" | "light" | "dark";
export type ResolvedTheme = "light" | "dark";

export const defaultThemeMode: ThemeMode = "system";
export const themeStorageKey = "mission.frontend.theme";

type ThemeContextValue = {
  resolvedTheme: ResolvedTheme;
  themeMode: ThemeMode;
  setThemeMode: (next: ThemeMode) => void;
};

const systemThemeQuery = "(prefers-color-scheme: dark)";

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readStoredThemeMode(initialMode?: ThemeMode): ThemeMode {
  if (initialMode) {
    return initialMode;
  }
  if (typeof window === "undefined") {
    return defaultThemeMode;
  }
  const stored = window.localStorage.getItem(themeStorageKey);
  if (stored === "system" || stored === "light" || stored === "dark") {
    return stored;
  }
  return defaultThemeMode;
}

function readSystemTheme(): ResolvedTheme {
  if (
    typeof window === "undefined" ||
    typeof window.matchMedia !== "function"
  ) {
    return "light";
  }
  return window.matchMedia(systemThemeQuery).matches ? "dark" : "light";
}

function applyResolvedTheme(theme: ResolvedTheme) {
  if (typeof document === "undefined") {
    return;
  }
  document.documentElement.classList.toggle("dark", theme === "dark");
  document.documentElement.style.colorScheme = theme;
}

export function ThemeProvider({
  children,
  initialMode,
}: PropsWithChildren<{ initialMode?: ThemeMode }>) {
  const [themeMode, setThemeMode] = useState<ThemeMode>(() =>
    readStoredThemeMode(initialMode),
  );
  const [systemTheme, setSystemTheme] = useState<ResolvedTheme>(() =>
    readSystemTheme(),
  );

  useEffect(() => {
    if (
      typeof window === "undefined" ||
      typeof window.matchMedia !== "function"
    ) {
      return;
    }

    const media = window.matchMedia(systemThemeQuery);
    const handleChange = (event: MediaQueryListEvent) => {
      setSystemTheme(event.matches ? "dark" : "light");
    };

    setSystemTheme(media.matches ? "dark" : "light");

    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", handleChange);
      return () => media.removeEventListener("change", handleChange);
    }

    media.addListener(handleChange);
    return () => media.removeListener(handleChange);
  }, []);

  useEffect(() => {
    if (typeof window === "undefined" || initialMode) {
      return;
    }
    if (themeMode === "system") {
      window.localStorage.removeItem(themeStorageKey);
      return;
    }
    window.localStorage.setItem(themeStorageKey, themeMode);
  }, [initialMode, themeMode]);

  const resolvedTheme = themeMode === "system" ? systemTheme : themeMode;

  useLayoutEffect(() => {
    applyResolvedTheme(resolvedTheme);
    return () => {
      if (typeof document === "undefined") {
        return;
      }
      document.documentElement.classList.remove("dark");
      document.documentElement.style.colorScheme = "";
    };
  }, [resolvedTheme]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      resolvedTheme,
      themeMode,
      setThemeMode,
    }),
    [resolvedTheme, themeMode],
  );

  return createElement(ThemeContext.Provider, { value }, children);
}

export function useTheme(): ThemeContextValue {
  const value = useContext(ThemeContext);
  if (!value) {
    throw new Error("useTheme must be used inside a ThemeProvider");
  }
  return value;
}
