import { expect, test, devices, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import sharp from "sharp";
import type { mountGraphics } from "./fixtures/graphics";
import type { BodyId, FocusTarget } from "../src/engine/scene.generated";

type Probe = ReturnType<typeof mountGraphics>;
const out = path.resolve(import.meta.dirname, "../../tmp/final-acceptance");
const backend = () => (test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu");
test.beforeEach(() => test.skip(!test.info().project.name.startsWith("engine")));

async function mount(page: Page, toolbar = true) {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(`/tests/fixtures/engine.html?renderer=${backend()}`);
  await page.evaluate(async (toolbar) => {
    const path = "/tests/fixtures/graphics.tsx";
    const { mountGraphics } = await import(/* @vite-ignore */ path);
    Reflect.set(window, "graphicsProbe", mountGraphics(toolbar));
  }, toolbar);
  await ready(page);
}
const inspect = (page: Page) =>
  page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).inspect());

for (const layout of ["desktop", "iphone", "android"] as const)
  test(`styled graphics selectors remain reachable with keyboard and touch on ${layout}`, async ({
    browser,
  }) => {
    test.setTimeout(120_000);
    const device =
      layout === "iphone" ? devices["iPhone 14 landscape"] : devices["Pixel 7 landscape"];
    const context = await browser.newContext(
      layout === "desktop"
        ? { viewport: { width: 1280, height: 720 } }
        : { ...device, deviceScaleFactor: 1 },
    );
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await mount(page);
      await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).settings());
      await page.getByRole("tab", { name: "Graphics", exact: true }).click();
      for (const name of [
        "Graphics quality preset",
        "Texture quality",
        "Effect detail",
        "Anti-aliasing",
        "FPS limit",
      ]) {
        const trigger = page.getByRole("button", { name, exact: true });
        await trigger.click();
        const list = page.getByRole("listbox", { name, exact: true });
        await expect(list).toBeVisible();
        await expect(list).toBeFocused();
        const bounds = (await list.boundingBox())!;
        expect(bounds.y).toBeGreaterThanOrEqual(0);
        expect(bounds.y + bounds.height).toBeLessThanOrEqual(page.viewportSize()!.height);
        for (const option of await list.getByRole("option").all()) {
          expect((await option.boundingBox())!.height).toBeGreaterThanOrEqual(44);
        }
        await page.screenshot({
          path: path.join(
            out,
            `selectors-${backend()}-${layout}-${name.replaceAll(" ", "-").toLowerCase()}.png`,
          ),
        });
        await list.press("Escape");
        await expect(list).toBeHidden();
        await expect(trigger).toBeFocused();
        await expect(page.getByRole("dialog", { name: "Settings", exact: true })).toBeVisible();
      }
      const preset = page.getByRole("button", { name: "Graphics quality preset", exact: true });
      await preset.press("ArrowDown");
      const list = page.getByRole("listbox", { name: "Graphics quality preset", exact: true });
      await list.press("Home");
      await list.press("Enter");
      await expect(preset).toHaveText("Low");
      await preset.click();
      await page.getByRole("listbox").press("Tab");
      await expect(
        page.getByRole("slider", { name: "Render resolution", exact: true }),
      ).toBeFocused();
      const fps = page.getByRole("button", { name: "FPS limit", exact: true });
      await fps.click();
      await page.getByRole("listbox").press("End");
      await page.getByRole("listbox").press("Enter");
      await expect(fps).toHaveText("240 FPS");
      await expect(preset).toHaveText("Custom");
      await expect(page.getByRole("button", { name: "Apply graphics", exact: true })).toBeVisible();
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });

async function ready(page: Page) {
  await expect
    .poll(
      async () => {
        const { status, scene } = await inspect(page);
        if (status.phase === "error") throw new Error(status.message);
        return status.phase === "ready" && !!scene?.boardReady;
      },
      { timeout: 90000 },
    )
    .toBe(true);
  await expect(page.getByRole("status")).toBeHidden({ timeout: 10000 });
}
async function frames(page: Page, count = 4) {
  const before = (await inspect(page)).performance!.submittedFrames;
  await expect
    .poll(async () => (await inspect(page)).performance!.submittedFrames, { timeout: 10000 })
    .toBeGreaterThanOrEqual(before + count);
}
async function focus(page: Page, target: FocusTarget) {
  await page.evaluate(
    (target) => (Reflect.get(window, "graphicsProbe") as Probe).focus(target),
    target,
  );
  await expect.poll(async () => (await inspect(page)).scene?.focus).toEqual(target);
  await frames(page);
}

test("planet surfaces travel without downloads, preparation or lost scene ownership", async ({
  page,
}) => {
  test.setTimeout(180000);
  await mkdir(out, { recursive: true });
  await mount(page);
  const baseline = (await inspect(page)).scene!;
  const requests: string[] = [];
  page.on("request", (request) => {
    if (request.url().includes("/assets/")) requests.push(request.url());
  });
  await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).phases.splice(0));
  const times: Array<{ destination: string; ms: number }> = [];
  for (const body of [
    "mercury",
    "venus",
    "earth",
    "ceres",
    "jupiter",
    "saturn",
    "uranus",
    "neptune",
    "mars",
  ] as BodyId[]) {
    const elapsed = await page.evaluate(async (body) => {
      const probe = Reflect.get(window, "graphicsProbe") as Probe;
      const start = performance.now();
      probe.focus({ type: "body", body });
      while (
        probe.inspect().scene?.focus.type !== "body" ||
        (probe.inspect().scene?.focus as { body: string }).body !== body
      )
        await new Promise(requestAnimationFrame);
      return performance.now() - start;
    }, body);
    times.push({ destination: body, ms: elapsed });
    expect(elapsed).toBeLessThan(200);
    await expect(page.getByRole("status")).toBeHidden();
    const current = (await inspect(page)).scene!;
    expect(current.tiles.map((tile) => tile.entity)).toEqual(
      baseline.tiles.map((tile) => tile.entity),
    );
    expect(current.space.bodies.map((body) => body.entity)).toEqual(
      baseline.space.bodies.map((body) => body.entity),
    );
    expect(current.space.warmCamera).toBe(false);
  }
  await focus(page, { type: "body", body: "earth" });
  for (let step = 0; step < 3; step++) {
    await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).zoom(2));
    await frames(page, 3);
  }
  const current = (await inspect(page)).scene!;
  await page.screenshot({ path: path.join(out, `picking-${backend()}-before.png`) });
  await writeFile(
    path.join(out, `picking-${backend()}-before.json`),
    JSON.stringify(current, null, 2),
  );
  // Exercise native canvas picking, outside the overview's React label buttons.
  const candidates = current.space.bodies.filter(
    (body) =>
      body.body !== "earth" &&
      body.body !== "luna" &&
      body.point &&
      body.point[0] > 10 &&
      body.point[0] < 1270 &&
      body.point[1] > 50 &&
      body.point[1] < 710,
  );
  expect(candidates.length).toBeGreaterThan(0);
  let travelled = false;
  const hitEvidence: unknown[] = [];
  for (const candidate of candidates) {
    await page.mouse.move(...candidate.point!);
    const hit = await page.locator("canvas").evaluate((canvas) => getComputedStyle(canvas).cursor);
    hitEvidence.push({ body: candidate.body, point: candidate.point, cursor: hit });
    if (hit !== "pointer") continue;
    await page.mouse.click(...candidate.point!);
    await expect
      .poll(async () => (await inspect(page)).scene?.focus)
      .not.toEqual({ type: "body", body: "earth" });
    travelled = true;
    break;
  }
  await writeFile(
    path.join(out, `picking-${backend()}-hits.json`),
    JSON.stringify(hitEvidence, null, 2),
  );
  expect(travelled, "visible planets accept direct canvas clicks").toBe(true);
  expect(requests).toEqual([]);
  const phases = await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).phases);
  expect(phases.every((phase) => phase === "ready")).toBe(true);
  await writeFile(
    path.join(out, `travel-${backend()}.json`),
    JSON.stringify({ times, requests, phases }, null, 2),
  );
  await page.screenshot({ path: path.join(out, `travel-${backend()}.png`) });
  await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).dispose());
});

test("Sun corona preserves the sky and background brightness leaves the Sun unchanged", async ({
  page,
}) => {
  test.setTimeout(150000);
  await mkdir(out, { recursive: true });
  await mount(page, false);
  await focus(page, { type: "solar-system" });
  for (let step = 0; step < 2; step++) {
    await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).zoom(-2));
    await frames(page, 3);
  }
  await expect.poll(async () => (await inspect(page)).scene?.camera.distance).toBeLessThan(301);
  await frames(page, 8);
  const sun = (await inspect(page)).scene!.space.sun;
  expect(sun.radius).toBeGreaterThan(25);
  expect(sun.point).not.toBeNull();
  const capture = async (brightness: number) => {
    await page.evaluate(
      (brightness) =>
        (Reflect.get(window, "graphicsProbe") as Probe).apply({ backgroundBrightness: brightness }),
      brightness,
    );
    await frames(page, 8);
    return page.screenshot({ path: path.join(out, `sun-${backend()}-${brightness}.png`) });
  };
  const normal = await capture(0.65),
    dark = await capture(0.1);
  const a = await sharp(normal).removeAlpha().raw().toBuffer({ resolveWithObject: true });
  const b = await sharp(dark).removeAlpha().raw().toBuffer();
  const [cx, cy] = sun.point!,
    radius = sun.radius!;
  let core = 0,
    coreCount = 0,
    halo = 0,
    haloCount = 0,
    outside = 0,
    outsideCount = 0,
    changedHaloPixels = 0;
  for (let y = 0; y < a.info.height; y++)
    for (let x = 0; x < a.info.width; x++) {
      const r = Math.hypot(x - cx, y - cy) / radius;
      const index = (y * a.info.width + x) * 3;
      const difference =
        (Math.abs(a.data[index] - b[index]) +
          Math.abs(a.data[index + 1] - b[index + 1]) +
          Math.abs(a.data[index + 2] - b[index + 2])) /
        3;
      if (r < 0.7) {
        core += difference;
        coreCount++;
      }
      if (r > 1.25 && r < 1.45) {
        halo += difference;
        haloCount++;
        if (difference > 2) {
          changedHaloPixels++;
        }
      }
      if (r > 1.6 && r < 1.8) {
        outside += difference;
        outsideCount++;
      }
    }
  expect(core / coreCount).toBeLessThan(0.5);
  // Stars are sparse: compare the sky's brightness response inside and outside
  // the corona. An opaque disk has zero response inside this annulus.
  expect(changedHaloPixels).toBeGreaterThan(25);
  expect(halo / haloCount).toBeGreaterThan(0.05);
  expect(halo / haloCount / (outside / outsideCount)).toBeGreaterThan(0.25);
  await writeFile(
    path.join(out, `sun-${backend()}.json`),
    JSON.stringify(
      {
        sun,
        core: core / coreCount,
        halo: halo / haloCount,
        outside: outside / outsideCount,
        changedHaloPixels,
      },
      null,
      2,
    ),
  );
  await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).dispose());
});

for (const layout of ["desktop", "iphone", "android"] as const)
  test(`graphics presets persist and change real rendering costs on ${layout}`, async ({
    browser,
  }) => {
    test.setTimeout(240000);
    const device =
      layout === "iphone" ? devices["iPhone 14 landscape"] : devices["Pixel 7 landscape"];
    const context = await browser.newContext(
      layout === "desktop"
        ? { viewport: { width: 1280, height: 720 } }
        : { ...device, deviceScaleFactor: 1 },
    );
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await mkdir(out, { recursive: true });
      // Seed corrupt storage before loading production stores, then verify a
      // device-appropriate fallback and persistence after the later reload.
      await page.goto(`/tests/fixtures/engine.html?renderer=${backend()}`);
      await page.evaluate(() => localStorage.setItem("openmars.graphics.settings", "invalid-json"));
      await mount(page);
      expect((await inspect(page)).preferences.textureQuality).toBe(
        layout === "desktop" ? "high" : "medium",
      );
      const evidence = [];
      for (const preset of ["high", "medium", "low"] as const) {
        await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).settings());
        await page.getByRole("tab", { name: "Graphics", exact: true }).click();
        if (preset === "low") {
          await page.getByRole("button", { name: "Reset Graphics", exact: true }).click();
          await expect(
            page.getByRole("button", { name: "Graphics quality preset", exact: true }),
          ).toHaveText(layout === "desktop" ? "High" : "Medium");
        }
        await page.getByRole("button", { name: "Graphics quality preset", exact: true }).click();
        await page
          .getByRole("option", { name: preset[0].toUpperCase() + preset.slice(1), exact: true })
          .click();
        await page.screenshot({
          path: path.join(out, `graphics-${backend()}-${layout}-${preset}-settings.png`),
        });
        const performanceButton = page.getByRole("button", { name: "Performance", exact: true });
        await performanceButton.scrollIntoViewIfNeeded();
        const controlBounds = (await performanceButton.boundingBox())!;
        const panelBounds = (await page.getByRole("tabpanel").boundingBox())!;
        expect(controlBounds.height).toBeGreaterThanOrEqual(44);
        expect(controlBounds.y).toBeGreaterThanOrEqual(panelBounds.y);
        expect(controlBounds.y + controlBounds.height).toBeLessThanOrEqual(
          panelBounds.y + panelBounds.height + 1,
        );
        if (preset === "low")
          await page.screenshot({
            path: path.join(out, `graphics-${backend()}-${layout}-scrolled-settings.png`),
          });

        const button = page.getByRole("button", { name: /^(Apply graphics|Done)$/ });
        await button.click();
        await ready(page);
        await frames(page, 8);
        const result = await inspect(page);
        expect(result.scene!.graphics).toEqual({
          ...result.preferences,
          renderScale: expect.closeTo(result.preferences.renderScale, 6),
          backgroundBrightness: expect.closeTo(result.preferences.backgroundBrightness, 6),
        });
        const dimensions = await page.locator("canvas").evaluate((canvas) => ({
          width: (canvas as HTMLCanvasElement).width,
          height: (canvas as HTMLCanvasElement).height,
        }));
        evidence.push({
          preset,
          dimensions,
          entities: result.performance!.entities,
          performance: result.performance,
          lighting: result.scene!.lighting,
          graphics: result.scene!.graphics,
        });
        await page.screenshot({
          path: path.join(out, `graphics-${backend()}-${layout}-${preset}-world.png`),
        });
      }
      expect(evidence[2].dimensions.width * evidence[2].dimensions.height).toBeLessThan(
        evidence[0].dimensions.width * evidence[0].dimensions.height * 0.5,
      );
      await writeFile(
        path.join(out, `graphics-${backend()}-${layout}.json`),
        JSON.stringify(evidence, null, 2),
      );
      expect(evidence[2].entities).toBeLessThan(evidence[0].entities);
      expect(evidence[2].lighting.background!.size).toBe(512);
      await page.reload();
      await mount(page);
      expect((await inspect(page)).preferences.textureQuality).toBe("low");
      expect((await inspect(page)).scene!.graphics.effects).toBe("low");
      await writeFile(
        path.join(out, `graphics-${backend()}-${layout}.json`),
        JSON.stringify(evidence, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });

test("live graphics overrides keep the scene resident and cap only engine submissions", async ({
  page,
}) => {
  test.setTimeout(120000);
  await mount(page);
  await frames(page, 8);
  const baseline = await inspect(page);
  const canvas = await page.locator("canvas").elementHandle();
  const requests: string[] = [];
  page.on("request", (request) => {
    if (request.url().includes("/assets/")) requests.push(request.url());
  });
  await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).phases.splice(0));
  await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).settings());
  await page.getByRole("tab", { name: "Graphics", exact: true }).click();
  await page.getByRole("button", { name: "Anti-aliasing", exact: true }).click();
  await page.getByRole("option", { name: "Off", exact: true }).click();
  await page.getByRole("button", { name: "FPS limit", exact: true }).click();
  await page.getByRole("option", { name: "30 FPS", exact: true }).click();
  const resolution = page.getByRole("slider", { name: "Render resolution", exact: true });
  await resolution.press("Home");
  for (let step = 0; step < 5; step++) await resolution.press("ArrowRight");
  const brightness = page.getByRole("slider", { name: "Space brightness", exact: true });
  await brightness.press("Home");
  for (let step = 0; step < 6; step++) await brightness.press("ArrowRight");
  await expect(
    page.getByRole("button", { name: "Graphics quality preset", exact: true }),
  ).toHaveText("Custom");
  await page.getByRole("button", { name: "Apply graphics", exact: true }).click();
  await frames(page, 8);
  const after = await inspect(page);
  expect(after.scene!.graphics).toMatchObject({
    antialiasing: false,
    frameLimit: 30,
    renderScale: 0.75,
    backgroundBrightness: expect.closeTo(0.4, 6),
  });
  expect(after.scene!.tiles.map((tile) => tile.entity)).toEqual(
    baseline.scene!.tiles.map((tile) => tile.entity),
  );
  expect(
    await page.locator("canvas").evaluate((current, previous) => current === previous, canvas),
  ).toBe(true);
  const dimensions = await page.locator("canvas").evaluate((current) => {
    const box = current.getBoundingClientRect();
    return {
      width: (current as HTMLCanvasElement).width,
      height: (current as HTMLCanvasElement).height,
      cssWidth: box.width,
      cssHeight: box.height,
    };
  });
  expect(dimensions.width).toBe(Math.round(dimensions.cssWidth * 0.75));
  expect(dimensions.height).toBe(Math.round(dimensions.cssHeight * 0.75));
  const cadence = await page.evaluate(async () => {
    const probe = Reflect.get(window, "graphicsProbe") as Probe;
    const start = performance.now();
    const initial = probe.inspect().performance!.submittedFrames;
    let browserFrames = 0;
    while (performance.now() - start < 2000) {
      await new Promise(requestAnimationFrame);
      browserFrames++;
    }
    const seconds = (performance.now() - start) / 1000;
    return {
      engine: (probe.inspect().performance!.submittedFrames - initial) / seconds,
      browser: browserFrames / seconds,
    };
  });
  expect(cadence.engine).toBeGreaterThan(24);
  expect(cadence.engine).toBeLessThan(34);
  expect(cadence.browser).toBeGreaterThan(cadence.engine * 1.3);
  expect(requests).toEqual([]);
  expect(
    await page.evaluate(() =>
      (Reflect.get(window, "graphicsProbe") as Probe).phases.every((phase) => phase === "ready"),
    ),
  ).toBe(true);
  if (backend() === "webgpu")
    expect(after.performance!.renderPasses).toBeLessThan(baseline.performance!.renderPasses!);
  await writeFile(
    path.join(out, `graphics-live-${backend()}.json`),
    JSON.stringify(
      { baseline: baseline.performance, after: after.performance, dimensions, cadence },
      null,
      2,
    ),
  );
  await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).dispose());
});

test("cold renderer diagnostics remain unavailable until native initialization completes", async ({
  page,
}) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto(`/tests/fixtures/engine.html?renderer=${backend()}`);
  await page.evaluate(
    async (modulePath) => {
      const module = await import(/* @vite-ignore */ modulePath);
      const create = module.Engine.create;
      module.Engine.create = async (canvas: HTMLCanvasElement) => {
        const native = await create.call(module.Engine, canvas);
        Reflect.set(window, "coldSceneStats", JSON.parse(native.scene_stats()));
        Reflect.set(window, "coldFrameCount", native.submitted_frames());
        return native;
      };
      const fixturePath = "/tests/fixtures/graphics.tsx";
      const { mountGraphics } = await import(/* @vite-ignore */ fixturePath);
      Reflect.set(window, "graphicsProbe", mountGraphics());
    },
    "/@fs" + path.resolve(import.meta.dirname, "../../engine/pkg", backend(), "openmars_engine.js"),
  );
  await ready(page);
  expect(await page.evaluate(() => Reflect.get(window, "coldSceneStats"))).toBeNull();
  expect(await page.evaluate(() => Reflect.get(window, "coldFrameCount"))).toBe(0);
  expect((await inspect(page)).scene!.boardReady).toBe(true);
  expect(errors).toEqual([]);
  await page.evaluate(() => (Reflect.get(window, "graphicsProbe") as Probe).dispose());
});
