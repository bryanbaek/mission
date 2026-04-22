import * as Sentry from "@sentry/react";
import React from "react";
import ReactDOM from "react-dom/client";

import App from "./App";
import {
  I18nProvider,
  loadLocaleDictionary,
  readStoredLocale,
} from "./lib/i18n";
import { ClerkGate } from "./lib/ClerkGate";
import { getRuntimeConfig, loadRuntimeConfig } from "./lib/runtimeConfig";
import { ThemeProvider } from "./lib/theme";
import "./index.css";

void bootstrap();

async function bootstrap() {
  const initialLocale = readStoredLocale();

  await Promise.all([
    loadRuntimeConfig(),
    loadLocaleDictionary(initialLocale),
  ]);
  const runtimeConfig = getRuntimeConfig();

  if (runtimeConfig.sentryDsn) {
    Sentry.init({
      dsn: runtimeConfig.sentryDsn,
      environment: runtimeConfig.sentryEnvironment,
      release: runtimeConfig.sentryRelease,
    });
  }

  ReactDOM.createRoot(document.getElementById("root")!).render(
    <React.StrictMode>
      <ThemeProvider>
        <I18nProvider initialLocale={initialLocale}>
          <ClerkGate>
            <App />
          </ClerkGate>
        </I18nProvider>
      </ThemeProvider>
    </React.StrictMode>,
  );
}
