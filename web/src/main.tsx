import React from "react";
import ReactDOM from "react-dom/client";

import App from "./App";
import { I18nProvider } from "./lib/i18n";
import { ClerkGate } from "./lib/ClerkGate";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <I18nProvider>
      <ClerkGate>
        <App />
      </ClerkGate>
    </I18nProvider>
  </React.StrictMode>,
);
