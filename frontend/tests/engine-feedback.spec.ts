import { expect, test, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";
import type { mountFeedback } from "./fixtures/feedback";
import type { mountRestoration, mountInterruptedLoading } from "./fixtures/loading";

type Probe = ReturnType<typeof mountFeedback>;
type Restoration = ReturnType<typeof mountRestoration>;
type InterruptedLoading = ReturnType<typeof mountInterruptedLoading>;
test.beforeEach(() => test.skip(!test.info().project.name.startsWith("engine")));

async function mount(page: Page, canvas = false) {
  await page.addInitScript(() => {
    Reflect.set(window, "feedbackPlayed", []);
    HTMLMediaElement.prototype.play = function () {
      Reflect.get(window, "feedbackPlayed").push({ url: this.src, volume: this.volume });
      return Promise.resolve();
    };
  });
  const renderer = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
  await page.goto(`/tests/fixtures/engine.html?renderer=${renderer}`);
  await page.evaluate(async (canvas) => {
    const module = "/tests/fixtures/feedback.tsx";
    const { mountFeedback } = await import(/* @vite-ignore */ module);
    Reflect.set(window, "feedbackProbe", mountFeedback(canvas));
  }, canvas);
}
async function tick(page: Page) {
  await page.evaluate(async () => {
    await new Promise(requestAnimationFrame);
    await new Promise(requestAnimationFrame);
  });
}

test("admin selection follows current player IDs when games or rosters change", async ({
  page,
}) => {
  await mount(page);
  const dialog = page.getByRole("dialog", { name: "Admin Tools" });
  await dialog.getByText("Player Resources", { exact: true }).click();
  const selector = dialog.getByRole("combobox");
  const values = dialog.getByRole("textbox");
  await expect(selector).toHaveValue("first-emil");
  await expect(values.nth(0)).toHaveValue("111");
  await selector.selectOption("first-sam");
  await expect(values.nth(0)).toHaveValue("222");
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).hide());
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).replaceGame());
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).show());
  await expect(selector).toHaveValue("second-emil");
  await expect(values.nth(0)).toHaveValue("111");
  await expect(values.nth(12)).toHaveValue("35");
  await selector.selectOption("second-sam");
  await expect(values.nth(0)).toHaveValue("222");
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).replaceRoster());
  await expect(selector).toHaveValue("third-emil");
  await expect(values.nth(0)).toHaveValue("111");
  await selector.selectOption("third-sam");
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).updateCredits());
  await expect(selector).toHaveValue("third-sam");
  await expect(values.nth(0)).toHaveValue("222");
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).removeSam());
  await expect(selector).toHaveValue("third-emil");
  await expect(values.nth(0)).toHaveValue("333");
  await dialog.getByRole("button", { name: "Set", exact: true }).first().click();
  await expect
    .poll(() =>
      page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).commands.at(-1)?.payload),
    )
    .toMatchObject({ playerId: "third-emil" });
  const out = path.resolve(import.meta.dirname, "../../tmp/city-placement-audio");
  await mkdir(out, { recursive: true });
  for (const [name, width, height] of [
    ["desktop", 1280, 900],
    ["iphone", 844, 390],
    ["android", 915, 412],
  ] as const) {
    await page.setViewportSize({ width, height });
    await expect(values.nth(12)).toHaveValue("35");
    await values.nth(12).scrollIntoViewIfNeeded();
    await expect(values.nth(12)).toBeVisible();
    await selector.scrollIntoViewIfNeeded();
    await expect(selector).toBeVisible();
    await page.screenshot({
      path: path.join(out, `admin-${test.info().project.name}-${name}.png`),
    });
  }
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).dispose());
});

test("restoring a game keeps loading opaque until that game's world is ready", async ({ page }) => {
  test.setTimeout(120000);
  const renderer = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
  const out = path.resolve(import.meta.dirname, "../../tmp/city-placement-audio");
  await mkdir(out, { recursive: true });
  const run = async (capture = false) => {
    await page.evaluate(async () => {
      const path = "/tests/fixtures/loading.tsx";
      const { mountRestoration } = await import(/* @vite-ignore */ path);
      Reflect.set(window, "restoration", mountRestoration());
    });
    const state = () =>
      page.evaluate(() => (Reflect.get(window, "restoration") as Restoration).state());
    await expect.poll(async () => (await state()).status.phase, { timeout: 90000 }).toBe("ready");
    expect((await state()).phase).toBe("checking");
    await expect(page.getByRole("status")).toHaveCSS("opacity", "1");
    if (capture) {
      await page.screenshot({ path: path.join(out, `restoring-${renderer}.png`) });
    }
    await page.evaluate(() => (Reflect.get(window, "restoration") as Restoration).connect());
    await expect.poll(async () => (await state()).phase, { timeout: 90000 }).toBe("playing");
    const ready = (await state()).status;
    expect(ready.phase === "ready" && ready.scene).toEqual({
      gameId: "restored-game",
      mapId: "tharsis",
    });
    await expect(page.getByRole("status")).toBeHidden();
    const frames = await page.evaluate(
      () => (Reflect.get(window, "restoration") as Restoration).frames,
    );
    expect(frames.length).toBeGreaterThan(0);
    const earlyReveal = frames.filter(
      (frame) => frame.readyGame !== "restored-game" && frame.opacity !== 1,
    );
    expect(earlyReveal).toEqual([]);
    expect(
      frames.filter((frame) => frame.phase === "playing" && frame.readyGame !== "restored-game"),
    ).toEqual([]);
    if (capture) {
      for (const [name, width, height] of [
        ["desktop", 1280, 900],
        ["iphone", 844, 390],
        ["android", 915, 412],
      ] as const) {
        await page.setViewportSize({ width, height });
        await tick(page);
        await page.screenshot({ path: path.join(out, `restored-${renderer}-${name}.png`) });
      }
    }
  };
  await page.goto(`/tests/fixtures/engine.html?renderer=${renderer}`);
  await run(true);
  // Recreate the WASM runtime and lifecycle through a real browser reload as well.
  await page.reload();
  await run();
  await page.evaluate(() => (Reflect.get(window, "restoration") as Restoration).dispose());
});

test("unfinished loading cancels an in-progress fade immediately", async ({ page }) => {
  await page.goto("/tests/fixtures/engine.html");
  await page.evaluate(async () => {
    const path = "/tests/fixtures/loading.tsx";
    const { mountInterruptedLoading } = await import(/* @vite-ignore */ path);
    Reflect.set(window, "interruptedLoading", mountInterruptedLoading());
  });
  const overlay = page.getByRole("status");
  await expect(overlay).toHaveCSS("opacity", "1");
  await page.evaluate(() =>
    (Reflect.get(window, "interruptedLoading") as InterruptedLoading).setLoaded(true),
  );
  await expect
    .poll(async () => Number(await overlay.evaluate((el) => getComputedStyle(el).opacity)))
    .toBeLessThan(0.8);
  await page.evaluate(() =>
    (Reflect.get(window, "interruptedLoading") as InterruptedLoading).setLoaded(false),
  );
  await expect(overlay).toHaveCSS("opacity", "1");
  expect(
    await overlay.evaluate(
      (el) => el.getAnimations().filter((animation) => animation.playState === "running").length,
    ),
  ).toBe(0);
  const out = path.resolve(import.meta.dirname, "../../tmp/city-placement-audio");
  for (const [name, width, height] of [
    ["desktop", 1280, 900],
    ["iphone", 844, 390],
    ["android", 915, 412],
  ] as const) {
    await page.setViewportSize({ width, height });
    await expect(overlay).toHaveCSS("opacity", "1");
    await page.screenshot({
      path: path.join(out, `loading-${test.info().project.name}-${name}.png`),
    });
  }
  await page.evaluate(() =>
    (Reflect.get(window, "interruptedLoading") as InterruptedLoading).dispose(),
  );
});

test("live city placement plays construction audio once and quiet snapshots do not replay", async ({
  page,
}) => {
  test.setTimeout(120000);
  await mount(page, true);
  await expect
    .poll(() => page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).status.phase), {
      timeout: 90000,
    })
    .toBe("ready");
  const played = () =>
    page.evaluate(() => Reflect.get(window, "feedbackPlayed") as { url: string; volume: number }[]);
  expect(await played()).toEqual([]);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).place(1, 0));
  await expect.poll(async () => (await played()).length).toBe(1);
  const urls = await page.evaluate(() =>
    (Reflect.get(window, "feedbackProbe") as Probe).soundUrls(),
  );
  expect(urls.slice(0, 2)).toContain((await played())[0].url);
  expect((await played())[0].volume).toBeCloseTo(0.4);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).rerender());
  await tick(page);
  expect(await played()).toHaveLength(1);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).reconnect(false));
  await tick(page);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).place(2, 0));
  await tick(page);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).reconnect(true));
  await tick(page);
  expect(await played()).toHaveLength(1);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).place(3, 0));
  await expect.poll(async () => (await played()).length).toBe(2);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).enabled(false));
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).place(-1, 0));
  await tick(page);
  expect(await played()).toHaveLength(2);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).enabled(true));
  await page.evaluate(() =>
    (Reflect.get(window, "feedbackProbe") as Probe).place(0, 1, "ocean-tile"),
  );
  await expect.poll(async () => (await played()).length).toBe(3);
  expect((await played())[2].url).toBe(urls[2]);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).replaceGame());
  await tick(page);
  expect(await played()).toHaveLength(3);
  const decoded = await page.evaluate(async (urls) => {
    const context = new AudioContext();
    try {
      return await Promise.all(
        urls.map(async (url) => {
          const response = await fetch(url);
          if (!response.ok) throw new Error("Audio asset failed");
          const audio = await context.decodeAudioData(await response.arrayBuffer());
          return audio.duration;
        }),
      );
    } finally {
      await context.close();
    }
  }, urls);
  expect(decoded.every((duration) => duration > 0)).toBe(true);
  await page.evaluate(() => (Reflect.get(window, "feedbackProbe") as Probe).dispose());
});
