import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/healthz": "http://localhost:8080",
      "/api": "http://localhost:8080",
      "/tenant.v1.TenantService": "http://localhost:8080",
    },
  },
});
