import { render, type RenderOptions } from "@testing-library/react";
import type { ReactElement } from "react";

import { I18nProvider, type Locale } from "../lib/i18n";

type Options = Omit<RenderOptions, "wrapper"> & {
  locale?: Locale;
};

export function renderWithI18n(
  ui: ReactElement,
  options?: Options,
) {
  const { locale, ...renderOptions } = options ?? {};
  return render(
    <I18nProvider initialLocale={locale}>
      {ui}
    </I18nProvider>,
    renderOptions,
  );
}
