import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import AppAccessGate from "./components/AppAccessGate";
import Layout from "./components/Layout";
import AgentsPage from "./pages/AgentsPage";
import ChatPage from "./pages/ChatPage";
import OnboardingHomePage from "./pages/OnboardingHomePage";
import OnboardingStepPage from "./pages/OnboardingStepPage";
import QueryPage from "./pages/QueryPage";
import SemanticLayerPage from "./pages/SemanticLayerPage";
import TenantsPage from "./pages/TenantsPage";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="onboarding" element={<OnboardingHomePage />} />
        <Route
          path="onboarding/:tenantId/step-1"
          element={<OnboardingStepPage step={1} />}
        />
        <Route
          path="onboarding/:tenantId/step-2"
          element={<OnboardingStepPage step={2} />}
        />
        <Route
          path="onboarding/:tenantId/step-3"
          element={<OnboardingStepPage step={3} />}
        />
        <Route
          path="onboarding/:tenantId/step-4"
          element={<OnboardingStepPage step={4} />}
        />
        <Route
          path="onboarding/:tenantId/step-5"
          element={<OnboardingStepPage step={5} />}
        />
        <Route
          path="onboarding/:tenantId/step-6"
          element={<OnboardingStepPage step={6} />}
        />
        <Route
          path="onboarding/:tenantId/step-7"
          element={<OnboardingStepPage step={7} />}
        />

        <Route element={<AppAccessGate />}>
          <Route element={<Layout />}>
            <Route index element={<TenantsPage />} />
            <Route path="agents" element={<AgentsPage />} />
            <Route path="chat" element={<ChatPage />} />
            <Route path="queries" element={<ChatPage />} />
            <Route path="queries/debug" element={<QueryPage />} />
            <Route path="semantic-layer" element={<SemanticLayerPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
