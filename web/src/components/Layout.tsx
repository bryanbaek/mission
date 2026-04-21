import { NavLink, Outlet } from "react-router-dom";
import { UserButton } from "@clerk/clerk-react";

import { useI18n } from "../lib/i18n";
import { useTheme, type ThemeMode } from "../lib/theme-context";

export default function Layout() {
  const { locale, setLocale, t } = useI18n();
  const { themeMode, setThemeMode } = useTheme();

  const nav = [
    { to: "/", label: t("layout.nav.tenants") },
    { to: "/chat", label: t("layout.nav.questions") },
    { to: "/semantic-layer", label: t("layout.nav.semanticLayer") },
    { to: "/agents", label: t("layout.nav.agents") },
  ];
  const themeOptions: ThemeMode[] = ["system", "light", "dark"];

  return (
    <main className="min-h-screen bg-slate-100 p-6 text-slate-900">
      <div className="mx-auto flex max-w-6xl flex-col gap-6">
        <header
          className={[
            "flex flex-col gap-3 rounded-3xl sm:flex-row",
            "sm:items-center sm:justify-between",
            "border border-slate-200 bg-white px-6 py-3 shadow-sm",
          ].join(" ")}
        >
          <nav className="flex flex-wrap items-center gap-2">
            {nav.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  [
                    "rounded-xl px-3 py-1.5 text-sm font-medium",
                    isActive
                      ? "bg-slate-950 text-white"
                      : "text-slate-600 hover:bg-slate-100",
                  ].join(" ")
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className="flex flex-wrap items-center justify-between gap-3 sm:justify-end">
            <div
              role="group"
              aria-label={t("layout.theme.label")}
              className="inline-flex rounded-xl bg-slate-100 p-1"
            >
              {themeOptions.map((nextThemeMode) => (
                <button
                  key={nextThemeMode}
                  type="button"
                  aria-pressed={themeMode === nextThemeMode}
                  onClick={() => setThemeMode(nextThemeMode)}
                  className={[
                    "rounded-lg px-3 py-1.5 text-xs font-semibold tracking-[0.18em]",
                    themeMode === nextThemeMode
                      ? "bg-slate-950 text-white"
                      : "text-slate-600 hover:bg-white",
                  ].join(" ")}
                >
                  {t(`layout.theme.${nextThemeMode}`)}
                </button>
              ))}
            </div>
            <div
              role="group"
              aria-label={t("common.language")}
              className="inline-flex rounded-xl bg-slate-100 p-1"
            >
              {(["en", "ko"] as const).map((nextLocale) => (
                <button
                  key={nextLocale}
                  type="button"
                  aria-pressed={locale === nextLocale}
                  onClick={() => setLocale(nextLocale)}
                  className={[
                    "rounded-lg px-3 py-1.5 text-xs font-semibold tracking-[0.18em]",
                    locale === nextLocale
                      ? "bg-slate-950 text-white"
                      : "text-slate-600 hover:bg-white",
                  ].join(" ")}
                >
                  {nextLocale.toUpperCase()}
                </button>
              ))}
            </div>
            <UserButton afterSignOutUrl="/" />
          </div>
        </header>
        <Outlet />
      </div>
    </main>
  );
}
