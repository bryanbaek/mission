import { render, type RenderOptions } from "@testing-library/react";
import type { ReactElement } from "react";

import {
  I18nProvider,
  primeLocaleDictionary,
  type Locale,
} from "../lib/i18n";
import { en } from "../lib/i18n-dictionaries/en";
import { ko } from "../lib/i18n-dictionaries/ko";
import { ThemeProvider } from "../lib/theme";
import { type ThemeMode } from "../lib/theme-context";

type Options = Omit<RenderOptions, "wrapper"> & {
  locale?: Locale;
  themeMode?: ThemeMode;
};

export function renderWithI18n(
  ui: ReactElement,
  options?: Options,
) {
  const { locale, themeMode, ...renderOptions } = options ?? {};

  primeLocaleDictionary("en", en);
  primeLocaleDictionary("ko", ko);

  return render(
    <ThemeProvider initialMode={themeMode}>
      <I18nProvider initialLocale={locale}>
        {ui}
      </I18nProvider>
    </ThemeProvider>,
    renderOptions,
  );
}
