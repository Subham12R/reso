import { defineConfig } from "playwright/test";

export default defineConfig({
  testDir: "./tests",
  use: { baseURL: "http://localhost:3000" },
  webServer: { command: "node -e \"require('fs').cpSync('.next/static', '.next/standalone/.next/static', { recursive: true, force: true })\" && node .next/standalone/server.js", url: "http://localhost:3000", reuseExistingServer: false },
});
