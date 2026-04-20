import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import Layout from "./components/Layout";
import AgentsPage from "./pages/AgentsPage";
import ChatPage from "./pages/ChatPage";
import QueryPage from "./pages/QueryPage";
import SemanticLayerPage from "./pages/SemanticLayerPage";
import TenantsPage from "./pages/TenantsPage";

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<TenantsPage />} />
          <Route path="agents" element={<AgentsPage />} />
          <Route path="queries" element={<ChatPage />} />
          <Route path="queries/debug" element={<QueryPage />} />
          <Route path="semantic-layer" element={<SemanticLayerPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
