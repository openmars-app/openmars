import { chromiumDevice } from "./tests/devices";
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
    headless: true,
    trace: "on",
  },

  projects: [
    ...["engine", "engine-webgl2"].map((name) => ({
      name,
      testMatch: [
        /engine(?:-sessions|-refinements|-endgame|-pacing|-profiling|-feedback|-graphics)?\.spec\.ts/,
        /mobile-lobby\.spec\.ts/,
      ],
      use: {
        browserName: "chromium" as const,
        launchOptions: {
          args: [
            "--enable-unsafe-webgpu",
            "--enable-gpu",
            "--mute-audio",
            // Select native Vulkan in Linux headless mode rather than SwiftShader.
            ...(process.platform === "linux"
              ? [
                  "--use-vulkan=native",
                  "--use-webgpu-adapter=default",
                  "--use-angle=vulkan",
                  "--enable-features=Vulkan",
                  "--disable-vulkan-surface",
                ]
              : []),
          ],
        },
      },
    })),
    {
      name: "firefox",
      use: {
        ...devices["Desktop Firefox"],
        launchOptions: { firefoxUserPrefs: { "media.volume_scale": "0.0" } },
      },
    },
    {
      name: "mobile-landscape-ios",
      use: { ...chromiumDevice("iPhone 14 landscape"), launchOptions: { args: ["--mute-audio"] } },
    },
    {
      name: "mobile-landscape-android",
      use: { ...chromiumDevice("Pixel 7 landscape"), launchOptions: { args: ["--mute-audio"] } },
    },
    {
      name: "mobile-portrait",
      testMatch: /mobile-menus\.spec\.ts/,
      use: { ...chromiumDevice("iPhone 14"), launchOptions: { args: ["--mute-audio"] } },
    },
  ],

  webServer: {
    command: `bun run engine && bun run assets && bun x --bun vite --port ${port} --strictPort`,
    url: baseURL,
    env: { BROWSER: "none", E2E_DISABLE_HMR: "1" },
    reuseExistingServer: process.env.E2E_REUSE_SERVER === "1",
    timeout: 300_000,
  },
});
