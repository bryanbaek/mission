import { Suspense, lazy } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import AppAccessGate from "./components/AppAccessGate";
import Layout from "./components/Layout";

const AgentsPage = lazy(() => import("./pages/AgentsPage"));
const ChatPage = lazy(() => import("./pages/ChatPage"));
const OnboardingHomePage = lazy(() => import("./pages/OnboardingHomePage"));
const OnboardingStepPage = lazy(() => import("./pages/OnboardingStepPage"));
const QueryPage = lazy(() => import("./pages/QueryPage"));
const SemanticLayerPage = lazy(() => import("./pages/SemanticLayerPage"));
const TenantsPage = lazy(() => import("./pages/TenantsPage"));

export default function App() {
  return (
    <BrowserRouter>
      <Suspense fallback={<div className="p-6 text-sm text-slate-500">Loading...</div>}>
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
      </Suspense>
    </BrowserRouter>
  );
}
