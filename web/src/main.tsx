import React from "react";
import ReactDOM from "react-dom/client";

import App from "./App";
import { ClerkGate } from "./lib/ClerkGate";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ClerkGate>
      <App />
    </ClerkGate>
  </React.StrictMode>,
);
