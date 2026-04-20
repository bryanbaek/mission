import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

const __dirname = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  // Load env vars from the repo root so a single .env drives both Go and Vite.
  envDir: resolve(__dirname, ".."),
  server: {
    port: 5173,
    proxy: {
      "/healthz": "http://localhost:8080",
      "/api": "http://localhost:8080",
      "/tenant.v1.TenantService": "http://localhost:8080",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    coverage: {
      provider: "v8",
      reporter: ["text", "html"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/gen/**",
        "src/main.tsx",
        "src/vite-env.d.ts",
        "src/App.tsx",
        "src/lib/**",
        "src/components/**",
        "src/test/**",
        "**/*.test.ts",
        "**/*.test.tsx",
      ],
      thresholds: {
        branches: 80,
        functions: 80,
        lines: 80,
        perFile: true,
        statements: 80,
      },
    },
  },
});
