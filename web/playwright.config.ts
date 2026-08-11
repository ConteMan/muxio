import { defineConfig } from "@playwright/test";

// The panel is only meaningful when served by the binary that embeds it, so the
// smoke tests drive a real `muxio serve` rather than the Vite dev server.
export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? "list" : "line",
  use: {
    baseURL: process.env.MUXIO_BASE_URL ?? "http://127.0.0.1:9922",
    trace: "retain-on-failure",
  },
});
