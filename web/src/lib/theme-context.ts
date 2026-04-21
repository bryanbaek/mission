import { createContext, useContext } from "react";

export type ThemeMode = "system" | "light" | "dark";
export type ResolvedTheme = "light" | "dark";

export const defaultThemeMode: ThemeMode = "system";
export const themeStorageKey = "mission.frontend.theme";

export type ThemeContextValue = {
  resolvedTheme: ResolvedTheme;
  themeMode: ThemeMode;
  setThemeMode: (next: ThemeMode) => void;
};

export const ThemeContext = createContext<ThemeContextValue | null>(null);

export function useTheme(): ThemeContextValue {
  const value = useContext(ThemeContext);
  if (!value) {
    throw new Error("useTheme must be used inside a ThemeProvider");
  }
  return value;
}
