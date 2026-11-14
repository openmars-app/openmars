import { devices, expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import type { SceneDiagnostics } from "../src/engine/diagnostics";
import type { PerformanceSnapshot } from "../src/services/performanceStore";

test.skip(
  !process.env.E2E_BACKEND || !process.env.E2E_SESSION_SAVE,
  "Provide a running Go backend and an exported game with E2E_BACKEND/E2E_SESSION_SAVE",
);
test.beforeEach(() => test.skip(!test.info().project.name.startsWith("engine")));
test.use({ actionTimeout: 15_000, trace: { mode: "retain-on-failure", screenshots: false } });

async function scene(page: Page): Promise<SceneDiagnostics> {
  return page.evaluate(async () => {
    const module = "/src/stores/engineStore.ts";
    const { useEngineStore } = await import(/* @vite-ignore */ module);
    return useEngineStore.getState().inspectScene!();
  });
}

async function performanceSample(page: Page): Promise<PerformanceSnapshot | undefined> {
  return page.evaluate(async () => {
    const module = "/src/services/performanceStore.ts";
    const { performanceStore } = await import(/* @vite-ignore */ module);
    return performanceStore.getSnapshots().at(-1);
  });
}

for (const [layout, viewport] of [
  ["desktop", { width: 1280, height: 720 }],
  ["iphone", { width: 844, height: 390 }],
  ["android", { width: 915, height: 412 }],
] as const) {
  test(`verifies camera settings audio and real engine counters on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.setTimeout(360_000);
    const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
    const profile =
      layout === "desktop"
        ? {}
        : devices[layout === "iphone" ? "iPhone 14 landscape" : "Pixel 7 landscape"];
    const context = await browser.newContext({ ...profile, baseURL, viewport });
    await context.addInitScript((renderer) => {
      const url = new URL(location.href);
      url.searchParams.set("renderer", renderer);
      history.replaceState(history.state, "", url);
    }, backend);
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const directory = path.resolve("../tmp/bevy-scaffold");
    await mkdir(directory, { recursive: true });
    const prefix = path.join(directory, `user-refinements-${backend}-${layout}`);
    const evidence: Record<string, unknown> = {};
    const capture = (stage: string) => page.screenshot({ path: `${prefix}-${stage}.png` });
    const ready = async (playing = true) => {
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 120_000 },
      );
      await expect(page.getByRole("status")).toHaveCount(0, { timeout: 120_000 });
      await expect
        .poll(async () => (await scene(page)).boardReady, { timeout: 120_000 })
        .toBe(true);
      if (playing) {
        await expect
          .poll(
            () =>
              page.evaluate(async () => {
                const module = "/src/stores/appPhaseStore.ts";
                const { useAppPhaseStore } = await import(/* @vite-ignore */ module);
                return useAppPhaseStore.getState().phase.kind;
              }),
            { timeout: 120_000 },
          )
          .toBe("playing");
      }
      await expect(page.getByRole("status")).toHaveCount(0, { timeout: 15_000 });
      await expect(page.getByRole("group", { name: "Board camera" })).toHaveCount(0);
    };
    const menu = async (name: string) => {
      await page.getByRole("button", { name: "Menu", exact: true }).click();
      await page.getByText(name, { exact: true }).click();
    };
    try {
      await page.goto(`/load?renderer=${backend}`);
      await page
        .getByLabel("Select an exported Game file")
        .setInputFiles(process.env.E2E_SESSION_SAVE!);
      await page.getByLabel("Your name").fill(`Refinement ${backend} ${layout}`);
      await page.getByRole("button", { name: "Load game", exact: true }).click();
      await page.getByRole("button", { name: "Resume game", exact: true }).click();
      await ready(false);
      const received = page.getByRole("heading", { name: "Cards received", exact: true });
      await expect(received).toBeVisible({ timeout: 30_000 });
      await page.getByRole("button", { name: "Close", exact: true }).click();
      await expect(received).toHaveCount(0);
      await ready();
      await capture("world");
      const initial = (await scene(page)).camera;
      const canvas = page.locator("canvas");
      await canvas.focus();
      await page.mouse.move(viewport.width / 2, viewport.height / 2);
      await page.mouse.down();
      await page.mouse.move(viewport.width / 2 + 90, viewport.height / 2 + 20, { steps: 5 });
      await page.mouse.up();
      const yawDelta = async () => {
        const difference = (await scene(page)).camera.yaw - initial.yaw;
        return Math.abs(Math.atan2(Math.sin(difference), Math.cos(difference)));
      };
      await expect.poll(yawDelta).toBeGreaterThan(0.03);
      expect(await yawDelta()).toBeLessThan(0.15);
      await canvas.focus();
      await page.keyboard.press("Home");
      await expect.poll(async () => (await scene(page)).camera.yaw).toBe(0);
      await page.mouse.wheel(0, -180);
      await expect
        .poll(async () => (await scene(page)).camera.distance)
        .toBeLessThan(initial.distance - 0.15);
      expect((await scene(page)).camera.distance).toBeGreaterThan(initial.distance * 0.8);
      await canvas.focus();
      await page.keyboard.press("Home");

      await menu("Performance");
      const performance = page.getByRole("dialog", { name: "Performance", exact: true });
      await expect(performance.getByText("Browser / UI", { exact: true })).toBeVisible();
      await expect
        .poll(async () => (await performanceSample(page))?.engine?.drawCalls, { timeout: 15_000 })
        .toBeGreaterThan(0);
      await expect
        .poll(() =>
          page.evaluate(async () => {
            const module = "/src/services/performanceStore.ts";
            const { performanceStore } = await import(/* @vite-ignore */ module);
            return performanceStore.getSnapshots().length;
          }),
        )
        .toBeGreaterThanOrEqual(6);
      const sample = await performanceSample(page);
      expect(sample?.engine?.backend).toBe(backend);
      expect(sample?.engine?.fps).toBeGreaterThan(0);
      expect(sample?.engine?.wasmBytes).toBeGreaterThan(0);
      expect(sample?.engine?.residentMeshes).toBeGreaterThan(0);
      evidence.performance = sample;
      await capture("performance");
      await performance.getByText("WASM memory capacity", { exact: true }).scrollIntoViewIfNeeded();
      await capture("engine-counters");
      await performance.getByRole("button", { name: "Close Performance", exact: true }).click();

      await page.getByRole("button", { name: "Menu", exact: true }).click();
      await expect(page.getByText(/^(Hide|Show) bot thoughts$/)).toHaveCount(0);
      await page.getByText("Settings", { exact: true }).click();
      const settings = page.getByRole("dialog", { name: "Settings", exact: true });
      await expect(settings.getByRole("tab", { name: "Controls", exact: true })).toHaveAttribute(
        "aria-selected",
        "true",
      );
      await expect(settings.getByText("Drag to orbit Mars. Scroll or pinch to zoom.")).toHaveCount(
        0,
      );
      await expect(
        settings.getByText("Applies to scrolling, pinch gestures and keyboard zoom."),
      ).toHaveCount(0);
      const sensitivity = settings.getByRole("slider", { name: /^Drag sensitivity/ });
      await expect(sensitivity).toHaveValue("0.15");
      await sensitivity.focus();
      await page.keyboard.press("Home");
      await page.keyboard.press("ArrowRight");
      await page.keyboard.press("ArrowRight");
      await expect(sensitivity).toHaveValue("0.2");
      await capture("controls");
      await settings.getByRole("tab", { name: "Audio", exact: true }).click();
      await settings.getByRole("checkbox", { name: "Music", exact: true }).check();
      const volume = settings.getByRole("slider", { name: /^Music volume/ });
      await volume.focus();
      await page.keyboard.press("Home");
      await page.keyboard.press("ArrowRight");
      await page.keyboard.press("ArrowRight");
      await expect(volume).toHaveValue("0.1");
      await capture("audio");
      await settings.getByRole("tab", { name: "AI", exact: true }).click();
      await settings.getByRole("checkbox", { name: "Show bot thoughts", exact: true }).uncheck();
      await capture("ai");
      await settings.getByRole("button", { name: "Done", exact: true }).click();
      await ready();
      await page.reload();
      await ready();
      await menu("Settings");
      await expect(page.getByRole("slider", { name: /^Drag sensitivity/ })).toHaveValue("0.2");
      await page.getByRole("tab", { name: "Audio", exact: true }).click();
      await expect(page.getByRole("slider", { name: /^Music volume/ })).toHaveValue("0.1");
      await page.getByRole("tab", { name: "AI", exact: true }).click();
      const thoughts = page.getByRole("checkbox", { name: "Show bot thoughts", exact: true });
      await expect(thoughts).not.toBeChecked();
      await page.getByRole("button", { name: "Reset AI", exact: true }).click();
      await expect(thoughts).toBeChecked();
      await page.getByRole("button", { name: "Done", exact: true }).click();
      expect(errors).toEqual([]);
      evidence.camera = (await scene(page)).camera;
    } finally {
      if (!page.isClosed()) await capture("final").catch(() => {});
      await writeFile(`${prefix}.json`, JSON.stringify(evidence, null, 2) + "\n");
      await context.close();
    }
  });
}
