import * as Sentry from "@sentry/react";
import React from "react";
import ReactDOM from "react-dom/client";

import App from "./App";
import { I18nProvider } from "./lib/i18n";
import { ClerkGate } from "./lib/ClerkGate";
import "./index.css";

if (import.meta.env.VITE_SENTRY_DSN) {
  Sentry.init({
    dsn: import.meta.env.VITE_SENTRY_DSN,
    environment: import.meta.env.VITE_SENTRY_ENVIRONMENT,
    release: import.meta.env.VITE_SENTRY_RELEASE,
  });
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18nProvider>
      <ClerkGate>
        <App />
      </ClerkGate>
    </I18nProvider>
  </React.StrictMode>,
);
