import { chromiumDevice } from "./scripts/inspect/devices";
import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.E2E_PORT ?? 3100);
const baseURL = `http://localhost:${port}`;

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL,
    trace: "on",
  },

  projects: [
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },
    {
      name: "mobile-landscape-ios",
      use: chromiumDevice("iPhone 14 landscape"),
    },
    {
      name: "mobile-landscape-android",
      use: chromiumDevice("Pixel 7 landscape"),
    },
    {
      name: "mobile-portrait",
      testMatch: /mobile-menus\.spec\.ts/,
      use: chromiumDevice("iPhone 14"),
    },
  ],

  webServer: {
    command: `bun run assets && bun x vite --port ${port} --strictPort`,
    url: baseURL,
    env: { BROWSER: "none" },
    reuseExistingServer: process.env.E2E_REUSE_SERVER === "1",
    timeout: 120_000,
  },
});
