import path from "node:path";
import { defineConfig } from "@playwright/test";
import dotenv from "dotenv";

dotenv.config({ path: path.resolve(__dirname, ".env") });

export default defineConfig({
  testDir: path.resolve(__dirname, "tests"),
  globalSetup: path.resolve(__dirname, "src/global-setup.ts"),
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: [["html", { outputFolder: "playwright-report", open: "never" }], ["list"]],
  outputDir: "test-results",
  use: {
    baseURL: process.env.E2E_BASE_URL || "http://localhost:18080",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
});
