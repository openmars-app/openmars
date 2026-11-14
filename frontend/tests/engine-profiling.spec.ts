import { devices, expect, test, type Page, type BrowserContext } from "@playwright/test";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import type { ProfileRecorder } from "../src/engine/profileRecorder";
import type { GameDto } from "../src/types/generated/api-types";
import type { SceneDiagnostics } from "../src/engine/diagnostics";

type Trace = ReturnType<ProfileRecorder["export"]>;
test.beforeEach(() => test.skip(!test.info().project.name.startsWith("engine")));
test.use({ actionTimeout: 15_000, trace: { mode: "retain-on-failure", screenshots: false } });

async function mountPanel(page: Page) {
  await page.evaluate(async () => {
    const module = "/tests/fixtures/profiler.ts";
    const { mountProfilerFixture } = await import(/* @vite-ignore */ module);
    mountProfilerFixture();
  });
  await expect(page.getByRole("dialog", { name: "Performance", exact: true })).toBeVisible();
}

async function recorder(page: Page) {
  return page.evaluate(async () => {
    const module = "/src/stores/engineProfilerStore.ts";
    const { useEngineProfilerStore } = await import(/* @vite-ignore */ module);
    const state = useEngineProfilerStore.getState();
    return {
      phase: state.phase,
      armed: state.armed,
      error: state.error,
      durationMs: state.durationMs,
    };
  });
}

async function recordedTrace(page: Page): Promise<Trace> {
  return page.evaluate(async () => {
    const module = "/src/engine/profiler.ts";
    const { engineProfiler } = await import(/* @vite-ignore */ module);
    return engineProfiler.export();
  });
}

const nativeSpans = (trace: Trace) =>
  trace.traceEvents.filter((event) => event.cat === "native.cpu" && event.ph === "X");

async function downloadTrace(page: Page, filename: string): Promise<Trace> {
  const downloaded = page.waitForEvent("download");
  await page
    .getByRole("region", { name: "Profiler", exact: true })
    .getByRole("button", { name: "Download", exact: true })
    .click();
  const download = await downloaded;
  expect(download.suggestedFilename()).toMatch(/^openmars-trace-.*\.json$/);
  await download.saveAs(filename);
  const trace = JSON.parse(await readFile(filename, "utf8")) as Trace;
  expect(trace.displayTimeUnit).toBe("ms");
  expect(trace.capture.version).toBe(2);
  expect(trace.capture.durationMs).toBeGreaterThan(0);
  expect(trace.capture.dropped).toBeGreaterThanOrEqual(0);
  expect(trace.capture.summary.worstCpuFrameMs).toBeGreaterThanOrEqual(0);
  expect(trace.capture.summary.longestFrameGapMs).toBeGreaterThan(0);
  expect(trace.capture.summary.hottestStage).toBeTruthy();
  expect(trace.capture.summary.hottestStageMs).toBeGreaterThanOrEqual(0);
  expect(trace.traceEvents.some((event) => event.name === "capture.start")).toBe(true);
  expect(trace.traceEvents.some((event) => event.name === "capture.stop")).toBe(true);
  const spans = nativeSpans(trace);
  expect(spans.length).toBeGreaterThan(0);
  for (const event of spans) {
    expect(Number.isFinite(event.ts)).toBe(true);
    expect(Number.isFinite(event.dur)).toBe(true);
    expect(event.dur).toBeGreaterThanOrEqual(0);
  }
  return trace;
}

for (const [layout, viewport] of [
  ["desktop", { width: 1280, height: 720 }],
  ["iphone", { width: 844, height: 390 }],
  ["android", { width: 915, height: 412 }],
] as const) {
  test(`records and exports Native work with compact profiler controls on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.setTimeout(150_000);
    const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
    const profile =
      layout === "desktop"
        ? {}
        : devices[layout === "iphone" ? "iPhone 14 landscape" : "Pixel 7 landscape"];
    const context = await browser.newContext({
      ...profile,
      baseURL,
      viewport,
      reducedMotion: "reduce",
    });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const directory = path.resolve("../tmp/bevy-scaffold");
    await mkdir(directory, { recursive: true });
    const prefix = path.join(directory, `profiler-ui-${backend}-${layout}`);
    try {
      await page.goto(`/dev/engine?renderer=${backend}${layout === "desktop" ? "" : "&compact"}`);
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 90_000 },
      );
      await expect(page.locator("canvas")).toHaveCount(1);
      await mountPanel(page);
      const profiler = page.getByRole("region", { name: "Profiler", exact: true });
      const record = profiler.getByRole("button", { name: "Record 10s", exact: true });
      const download = profiler.getByRole("button", { name: "Download", exact: true });
      const nextReload = profiler.getByRole("button", { name: "Next reload", exact: true });
      await expect(download).toBeDisabled();
      await expect(nextReload).toHaveAttribute("aria-pressed", "false");
      for (const control of [record, download, nextReload]) {
        // oxlint-disable-next-line no-await-in-loop -- Each control must be measured in the viewport.
        const bounds = await control.boundingBox();
        expect(bounds).not.toBeNull();
        expect(bounds!.height).toBeGreaterThanOrEqual(44);
        expect(bounds!.width).toBeGreaterThanOrEqual(44);
        expect(bounds!.x).toBeGreaterThanOrEqual(0);
        expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width);
        expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(viewport.height);
      }
      await record.focus();
      await expect(record).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(profiler.getByRole("button", { name: "Stop", exact: true })).toBeVisible();
      await expect(download).toBeDisabled();
      await expect
        .poll(async () => nativeSpans(await recordedTrace(page)).length, { timeout: 8000 })
        .toBeGreaterThan(0);
      expect((await recorder(page)).durationMs).toBe(10_000);
      await page.screenshot({ path: `${prefix}-recording.png` });

      const beforeClose = nativeSpans(await recordedTrace(page)).length;
      await page.getByRole("button", { name: "Close Performance", exact: true }).click();
      await expect(page.getByRole("dialog", { name: "Performance", exact: true })).toHaveCount(0);
      await expect
        .poll(async () => nativeSpans(await recordedTrace(page)).length, { timeout: 4000 })
        .toBeGreaterThan(beforeClose);
      expect((await recorder(page)).phase).toBe("recording");
      await page.getByRole("button", { name: "Open performance", exact: true }).click();
      await profiler.getByRole("button", { name: "Stop", exact: true }).click();
      await expect(download).toBeEnabled();
      expect((await recorder(page)).phase).toBe("ready");
      const manual = await downloadTrace(page, `${prefix}-manual.json`);
      expect(manual.capture.mode).toBe("manual");
      expect(manual.traceEvents.some((event) => event.name === "native.readiness")).toBe(false);
      await expect(profiler.getByText("Worst CPU frame", { exact: true })).toBeVisible();
      await expect(profiler.getByText("Longest frame gap", { exact: true })).toBeVisible();
      await expect(profiler.getByText("Main stage", { exact: true })).toBeVisible();
      await page.screenshot({ path: `${prefix}-ready.png` });

      const origin = await page.evaluate(() => performance.timeOrigin);
      await nextReload.click();
      await expect(nextReload).toHaveAttribute("aria-pressed", "true");
      expect(await page.evaluate(() => performance.timeOrigin)).toBe(origin);
      expect((await recorder(page)).phase).toBe("ready");
      await page.reload();
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 90_000 },
      );
      await mountPanel(page);
      await expect(nextReload).toHaveAttribute("aria-pressed", "false");
      expect((await recorder(page)).durationMs).toBe(30_000);
      await expect
        .poll(async () => nativeSpans(await recordedTrace(page)).length)
        .toBeGreaterThan(0);
      await profiler.getByRole("button", { name: "Stop", exact: true }).click();
      const reloaded = await downloadTrace(page, `${prefix}-reload.json`);
      expect(reloaded.capture.mode).toBe("reload");
      expect(reloaded.traceEvents.some((event) => event.name === "wasm.download")).toBe(true);
      expect(reloaded.traceEvents.some((event) => event.name === "engine.create")).toBe(true);
      expect(reloaded.traceEvents.some((event) => event.name === "native.readiness")).toBe(true);
      expect((await recorder(page)).error).toBeNull();
      expect(errors).toEqual([]);
    } finally {
      if (!page.isClosed()) {
        await page
          .evaluate(async () => {
            const module = "/src/engine/profiler.ts";
            const { engineProfiler } = await import(/* @vite-ignore */ module);
            engineProfiler.clear();
            const fixture = "/tests/fixtures/profiler.ts";
            const { unmountProfilerFixture } = await import(/* @vite-ignore */ fixture);
            unmountProfilerFixture();
          })
          .catch(() => {});
      }
      await context.close();
    }
  });
}

test("keeps a failed startup trace downloadable from the renderer error", async ({ page }) => {
  const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
  await page.addInitScript(() => sessionStorage.setItem("openmars.profile-next-reload", "1"));
  await page.route("**/*.wasm*", (route) => route.abort("failed"));
  await page.goto(`/dev/engine?renderer=${backend}`);
  await expect(page.getByRole("heading", { name: "Unable to start the renderer" })).toBeVisible();
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "Download trace", exact: true }).click();
  const target = path.resolve(`../tmp/bevy-scaffold/profiler-error-${backend}.json`);
  await (await download).saveAs(target);
  const trace = JSON.parse(await readFile(target, "utf8")) as Trace;
  expect(trace.traceEvents.some((event) => event.name === "renderer.failure")).toBe(true);
  expect(trace.traceEvents.some((event) => event.name === "capture.stop")).toBe(true);
  await page.screenshot({
    path: path.resolve(`../tmp/bevy-scaffold/profiler-error-${backend}.png`),
  });
});

test("keeps native preparation details when a world asset fails", async ({ page }) => {
  test.setTimeout(90_000);
  const renderer = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
  await page.route("**/*.glb*", (route) => route.abort("failed"));
  await page.goto("/tests/fixtures/engine.html");
  const trace = (await page.evaluate(async (renderer) => {
    const rp = "/src/engine/runtime.ts",
      sp = "/src/engine/scene.ts",
      fp = "/tests/fixtures/scene.ts",
      pp = "/src/engine/profiler.ts";
    const [{ startEngine }, { toSceneSnapshot }, { sceneGame }, { engineProfiler }] =
      await Promise.all([import(rp), import(sp), import(fp), import(pp)]);
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0";
    document.body.append(host);
    let ready = () => {},
      failed = () => {};
    const started = new Promise<void>((resolve) => {
      ready = resolve;
    });
    const failure = new Promise<void>((resolve) => {
      failed = resolve;
    });
    const runtime = startEngine(
      host,
      (status: { phase: string }) => {
        if (status.phase === "ready") ready();
        if (status.phase === "error") failed();
      },
      renderer,
    );
    try {
      await started;
      engineProfiler.start("manual");
      runtime.setScene(toSceneSnapshot(sceneGame("asset-failure-details"), null), "snapshot");
      await failure;
      return engineProfiler.export();
    } finally {
      runtime.dispose();
      engineProfiler.stop();
      host.remove();
    }
  }, renderer)) as Trace;
  const failure = trace.traceEvents.find((event) => event.name === "renderer.failure");
  expect(failure?.args?.backend).toBe(renderer);
  expect(failure?.args?.nativePreparation).not.toBe("unavailable");
  expect(typeof failure?.args?.nativeAssets).toBe("string");
  const assets = JSON.parse(String(failure!.args!.nativeAssets));
  expect(assets).toHaveProperty("activeCameras");
  await writeFile(
    path.resolve(`../tmp/bevy-scaffold/profiler-world-error-${renderer}.json`),
    JSON.stringify(trace),
  );
});

// Observe the actual browser pipeline APIs, including redundant links whose
// generated shader code is identical. Timing thresholds would miss this bug.
async function installSharedPipelineProbe(context: BrowserContext) {
  await context.addInitScript(() => {
    type Family = "foliage" | "bake" | "structures" | "special";
    const counts = { foliage: 0, bake: 0, structures: 0, special: 0 };
    Reflect.set(window, "sharedPipelineCounts", counts);
    const modules = new WeakMap<object, Family>();
    const classify = (module: object, code: string) => {
      if (/vec4\s+frost\s*;|frost\s*:\s*vec4/.test(code)) modules.set(module, "foliage");
      else if (code.includes("struct BakeVertex")) modules.set(module, "bake");
      else if (code.includes("openmars_engine_structures_shaders_animation_Parameters"))
        modules.set(module, "structures");
      else if (
        code.includes("openmars_engine_structures_special_") ||
        code.includes("mohole_color") ||
        code.includes("fence_surface")
      )
        modules.set(module, "special");
    };
    const record = (stages: object[]) => {
      for (const family of new Set(stages.map((stage) => modules.get(stage)))) {
        if (family) counts[family]++;
      }
    };
    const gpu = Reflect.get(globalThis, "GPUDevice")?.prototype;
    if (gpu) {
      const originalModule = gpu.createShaderModule;
      gpu.createShaderModule = function (this: object, descriptor: { code: string }) {
        const module = originalModule.call(this, descriptor);
        classify(module, descriptor.code);
        return module;
      };
      for (const method of ["createRenderPipeline", "createRenderPipelineAsync"]) {
        const original = Reflect.get(gpu, method);
        Reflect.set(
          gpu,
          method,
          function (
            this: object,
            descriptor: { vertex: { module: object }; fragment?: { module: object } },
          ) {
            record([
              descriptor.vertex.module,
              ...(descriptor.fragment ? [descriptor.fragment.module] : []),
            ]);
            return original.call(this, descriptor);
          },
        );
      }
    }
    for (const name of ["WebGLRenderingContext", "WebGL2RenderingContext"]) {
      const gl = Reflect.get(globalThis, name)?.prototype;
      if (!gl) continue;
      const shaders = new WeakMap<object, object[]>();
      const originalSource = gl.shaderSource;
      gl.shaderSource = function (this: object, shader: object, code: string) {
        classify(shader, code);
        return originalSource.call(this, shader, code);
      };
      const originalAttach = gl.attachShader;
      gl.attachShader = function (this: object, program: object, shader: object) {
        shaders.set(program, [...(shaders.get(program) ?? []), shader]);
        return originalAttach.call(this, program, shader);
      };
      const originalLink = gl.linkProgram;
      gl.linkProgram = function (this: object, program: object) {
        record(shaders.get(program) ?? []);
        return originalLink.call(this, program);
      };
    }
  });
}

async function sharedPipelineCounts(
  page: Page,
): Promise<{ foliage: number; bake: number; structures: number; special: number }> {
  return page.evaluate(() => Reflect.get(window, "sharedPipelineCounts"));
}

for (const projectType of ["greenery", "city"] as const) {
  test(`exports a real Go placement with correlated Native terrain work (${projectType})`, async ({
    browser,
    baseURL,
  }) => {
    test.skip(
      !process.env.E2E_BACKEND || !process.env.E2E_SESSION_SAVE,
      "Requires a running Go backend and an exported playable game",
    );
    test.setTimeout(240_000);
    const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
    const viewport = { width: 1280, height: 720 };
    const context = await browser.newContext({ baseURL, viewport, reducedMotion: "reduce" });
    await context.addInitScript((renderer) => {
      const url = new URL(location.href);
      url.searchParams.set("renderer", renderer);
      history.replaceState(history.state, "", url);
    }, backend);
    await installSharedPipelineProbe(context);
    const page = await context.newPage();
    const current = () =>
      page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts",
          ep = "/src/stores/engineStore.ts",
          pp = "/src/stores/appPhaseStore.ts";
        const [{ useGameStore }, { useEngineStore }, { useAppPhaseStore, gameIdOf }] =
          await Promise.all([
            import(/* @vite-ignore */ gp),
            import(/* @vite-ignore */ ep),
            import(/* @vite-ignore */ pp),
          ]);
        return {
          game: useGameStore.getState().game as GameDto | null,
          connected: useGameStore.getState().isConnected as boolean,
          phase: useAppPhaseStore.getState().phase.kind as string,
          phaseGameId: gameIdOf(useAppPhaseStore.getState().phase) as string | null,
          status: useEngineStore.getState().status.phase as string,
          scene: useEngineStore.getState().inspectScene?.() as SceneDiagnostics | null,
        };
      });
    try {
      await page.goto(`/load?renderer=${backend}`);
      await page
        .getByLabel("Select an exported Game file")
        .setInputFiles(process.env.E2E_SESSION_SAVE!);
      await page.getByLabel("Your name").fill(`Profiler placement ${backend}`);
      await page.getByRole("button", { name: "Load game", exact: true }).click();
      await page.getByRole("button", { name: "Resume game", exact: true }).click();
      await expect(async () => {
        expect((await current()).status).toBe("ready");
        expect((await current()).scene?.boardReady).toBe(true);
      }).toPass({ timeout: 120_000, intervals: [500] });
      await expect(page.getByRole("status")).toHaveCount(0, { timeout: 30_000 });
      await expect(
        page.getByRole("heading", { name: "Cards received", exact: true }),
      ).toBeVisible();
      await page.getByRole("button", { name: "Close", exact: true }).click();
      await expect(async () => {
        const state = await current();
        expect(state.connected).toBe(true);
        expect(state.phase).toBe("playing");
        expect(state.phaseGameId).toBe(state.game?.id);
        expect(state.status).toBe("ready");
        expect(state.scene?.boardReady).toBe(true);
        expect(state.scene?.camera.framing).toBe(false);
      }).toPass({ timeout: 60_000, intervals: [500] });
      await expect(page.getByRole("status")).toHaveCount(0);
      // The production game has a real 3D view: stable empty cluster storage must
      // survive subsequent submissions instead of allocating buffers every frame.
      const clusters = await page.evaluate(async () => {
        const module = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ module);
        const engine = useEngineStore.getState();
        const source = engine.performance;
        if (!source) throw new Error("Native performance source is unavailable");
        const read = () => {
          const lighting = engine.inspectScene?.()?.lighting;
          return {
            frames: source.sample()?.submittedFrames ?? -1,
            allocations: Number(Reflect.get(lighting, "clusterBufferAllocations")),
            views: Number(Reflect.get(lighting, "clusterViews")),
          };
        };
        const frames = async (count: number) => {
          const start = read().frames;
          for (let raf = 0; raf < 4000; raf++) {
            // oxlint-disable-next-line no-await-in-loop -- Wait for actual Native submissions.
            await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
            if (read().frames >= start + count) return;
          }
          throw new Error("Cluster persistence check did not receive Native frames");
        };
        source.setActive(true);
        try {
          await frames(4);
          const warmed = read();
          await frames(24);
          return { warmed, later: read() };
        } finally {
          source.setActive(false);
        }
      });
      expect(clusters.warmed.views).toBeGreaterThan(0);
      expect(clusters.warmed.allocations).toBeGreaterThan(0);
      expect(clusters.later.frames).toBeGreaterThanOrEqual(clusters.warmed.frames + 24);
      expect(clusters.later.allocations).toBe(clusters.warmed.allocations);
      expect(clusters.later.views).toBe(clusters.warmed.views);
      const before = await current();
      const project = before.game?.currentPlayer.standardProjects.find(
        (item) => item.projectType === projectType && item.available,
      );
      test.skip(!project, "The supplied save must offer the selected affordable project");

      await page.getByRole("button", { name: "Menu", exact: true }).click();
      await page.getByText("Performance", { exact: true }).click();
      const profiler = page.getByRole("region", { name: "Profiler", exact: true });
      await profiler.getByRole("button", { name: "Record 10s", exact: true }).click();
      await page.getByRole("button", { name: "Close Performance", exact: true }).click();
      // Existing service quotes/pays through Go, which then supplies the valid hexes.
      await page.evaluate(async (kind) => {
        const module = "/src/services/globalWebSocketManager.ts";
        const { globalWebSocketManager } = await import(/* @vite-ignore */ module);
        await globalWebSocketManager.standardProject(kind);
      }, project!.projectType);
      await expect
        .poll(
          async () =>
            (await current()).game?.currentPlayer.pendingTileSelection?.availableHexes.length,
          { timeout: 15_000 },
        )
        .toBeGreaterThan(0);
      const selecting = await current();
      const pending = selecting.game!.currentPlayer.pendingTileSelection!;
      const available = new Set(pending.availableHexes);
      const candidates = selecting.scene!.tiles.filter(
        (tile) =>
          available.has(tile.id) &&
          tile.point &&
          tile.point[0] > viewport.width * 0.2 &&
          tile.point[0] < viewport.width * 0.8 &&
          tile.point[1] > viewport.height * 0.2 &&
          tile.point[1] < viewport.height * 0.8,
      );
      candidates.sort(
        (a, b) =>
          Math.hypot(a.point![0] - viewport.width / 2, a.point![1] - viewport.height / 2) -
          Math.hypot(b.point![0] - viewport.width / 2, b.point![1] - viewport.height / 2),
      );
      expect(candidates.length).toBeGreaterThan(0);
      const target = candidates[0];
      const occupant = `${pending.tileType}-tile`;
      const preparedPipelines = await sharedPipelineCounts(page);
      expect(preparedPipelines.foliage).toBeGreaterThan(0);
      expect(preparedPipelines.bake).toBeGreaterThan(0);
      expect(preparedPipelines.structures).toBeGreaterThan(0);
      await page.mouse.click(...target.point!);
      await expect(async () => {
        const state = await current();
        const tile = state.game?.board.tiles.find(
          (candidate) =>
            `${candidate.coordinates.q},${candidate.coordinates.r},${candidate.coordinates.s}` ===
            target.id,
        );
        expect(state.game?.currentPlayer.pendingTileSelection).toBeFalsy();
        expect(tile?.occupiedBy?.type).toBe(occupant);
      }).toPass({ timeout: 15_000, intervals: [100] });
      await expect(async () => {
        const trace = await recordedTrace(page);
        expect(trace.traceEvents.some((event) => event.name === "state.received")).toBe(true);
        let applied: Trace["traceEvents"][number] | undefined;
        for (const event of trace.traceEvents) {
          if (
            event.name === "scene.applied" &&
            event.args?.kind === "world" &&
            Number(event.args?.context) > 0
          ) {
            applied = event;
          }
        }
        expect(applied).toBeDefined();
        const owner = applied!.args!.context;
        const build = trace.traceEvents.find(
          (event) => event.name === "terrain.build.begin" && event.args?.context === owner,
        );
        expect(build).toBeDefined();
        const publication = trace.traceEvents.find(
          (event) =>
            event.name === "terrain.publish" &&
            event.args?.landscapeRevision === build!.args?.landscapeRevision &&
            event.args?.context === owner,
        );
        expect(publication).toBeDefined();
        expect(publication!.ts).toBeGreaterThanOrEqual(build!.ts);
        expect(
          trace.traceEvents.some(
            (event) =>
              event.name === "Scene update → terrain.publish" &&
              event.args?.context === owner &&
              event.cat === "scene.wait",
          ),
        ).toBe(true);
      }).toPass({ timeout: 15_000, intervals: [100] });
      await page.getByRole("button", { name: "Menu", exact: true }).click();
      await page.getByText("Performance", { exact: true }).click();
      const stop = profiler.getByRole("button", { name: "Stop", exact: true });
      if (await stop.isVisible()) await stop.click();
      const output = path.resolve("../tmp/bevy-scaffold");
      await mkdir(output, { recursive: true });
      const trace = await downloadTrace(
        page,
        path.join(output, `profiler-real-placement-${backend}-${projectType}.json`),
      );
      expect(trace.traceEvents.some((event) => event.name === "state.received")).toBe(true);
      expect(trace.capture.mode).toBe("manual");
      expect(await sharedPipelineCounts(page)).toEqual(preparedPipelines);
    } finally {
      await context.close();
    }
  });
}

async function installSpecialPreparationBufferProbe(context: BrowserContext) {
  await context.addInitScript(() => {
    const counts = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "specialPreparationBuffers", counts);
    const tracked = new WeakSet<object>();
    const device = Reflect.get(globalThis, "GPUDevice")?.prototype;
    const buffer = Reflect.get(globalThis, "GPUBuffer")?.prototype;
    if (!device || !buffer) return;
    const create = device.createBuffer;
    device.createBuffer = function (this: object, descriptor: GPUBufferDescriptor) {
      const value = create.call(this, descriptor);
      if (descriptor.label?.startsWith("special surface preparation ")) {
        tracked.add(value);
        counts.created++;
        counts.live++;
      }
      return value;
    };
    const destroy = buffer.destroy;
    buffer.destroy = function (this: object) {
      if (tracked.delete(this)) {
        counts.destroyed++;
        counts.live--;
      }
      return destroy.call(this);
    };
  });
}

test("first Mohole reuses prepared surface mask and accessory pipelines", async ({
  browser,
  baseURL,
}) => {
  test.setTimeout(180_000);
  const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
  const context = await browser.newContext({ baseURL, viewport: { width: 1280, height: 720 } });
  await installSharedPipelineProbe(context);
  await installSpecialPreparationBufferProbe(context);
  const page = await context.newPage();
  const output = path.resolve("../tmp/mohole-cold-start");
  await mkdir(output, { recursive: true });
  try {
    await page.goto("/tests/fixtures/engine.html");
    const report = await page.evaluate(async (backend) => {
      const rp = "/src/engine/runtime.ts",
        sp = "/src/engine/scene.ts",
        fp = "/tests/fixtures/scene.ts",
        pp = "/src/engine/profiler.ts";
      const [{ startEngine }, { toSceneSnapshot }, { sceneGame }, { engineProfiler }] =
        await Promise.all([import(rp), import(sp), import(fp), import(pp)]);
      const host = document.createElement("div");
      host.style.cssText = "position:fixed;inset:0";
      document.body.append(host);
      let phase = "starting",
        failure: string | undefined;
      const runtime = startEngine(
        host,
        (status: { phase: string; message?: string }) => {
          phase = status.phase;
          if (status.phase === "error") failure = status.message;
        },
        backend,
      );
      Reflect.set(window, "moholeColdProbe", { runtime, host });
      const frames = async (n = 1) => {
        for (let i = 0; i < n; i++) await new Promise(requestAnimationFrame);
      };
      const until = async (predicate: () => boolean) => {
        const deadline = performance.now() + 100_000;
        while (!predicate()) {
          if (failure) throw new Error(failure);
          if (performance.now() > deadline)
            throw new Error("Mohole readiness: " + JSON.stringify(runtime.assetStats()));
          await frames();
        }
      };
      await until(() => phase === "ready");
      const game = sceneGame("first-mohole");
      runtime.setScene(toSceneSnapshot(game, null), "snapshot");
      await frames(2);
      await until(
        () =>
          phase === "ready" &&
          runtime.sceneStats()?.boardReady &&
          !runtime.sceneStats()?.settling &&
          runtime.assetStats().specialSurfaces.ready,
      );
      await frames(5);
      const initial = {
        counts: { ...Reflect.get(window, "sharedPipelineCounts") },
        stats: runtime.assetStats().specialSurfaces,
        buffers: { ...Reflect.get(window, "specialPreparationBuffers") },
      };
      const placements = [];
      for (const [index, [q, r]] of [
        [0, 0],
        [1, -1],
      ].entries()) {
        engineProfiler.start("manual");
        engineProfiler.mark("test.mohole.place", { ordinal: index + 1, q, r });
        game.board.tiles.find(
          (tile: GameDto["board"]["tiles"][number]) =>
            tile.coordinates.q === q && tile.coordinates.r === r,
        )!.occupiedBy = { type: "mohole-tile", tags: [], visual: { seed: 1234 + index } };
        const start = performance.now();
        runtime.setScene(toSceneSnapshot(game, null), "live");
        await until(
          () =>
            runtime.assetStats().specialSurfaces.sites === index + 1 &&
            runtime.assetStats().specialSurfaces.ready,
        );
        await frames(15);
        engineProfiler.stop();
        placements.push({
          counts: { ...Reflect.get(window, "sharedPipelineCounts") },
          stats: runtime.assetStats().specialSurfaces,
          elapsedMs: performance.now() - start,
          trace: engineProfiler.export(),
        });
      }
      return { initial, placements };
    }, backend);
    await writeFile(path.join(output, `first-use-${backend}.json`), JSON.stringify(report));
    await page.screenshot({ path: path.join(output, `first-use-${backend}.png`) });
    expect(report.initial.stats.sites).toBe(0);
    expect(report.initial.stats.uniforms).toBe(0);
    expect(report.initial.stats.geometryBuilds).toBe(0);
    expect(report.initial.counts.special).toBeGreaterThan(0);
    if (backend === "webgpu") {
      expect(report.initial.buffers).toEqual({ created: 2, destroyed: 2, live: 0 });
    }
    for (const placement of report.placements) {
      expect(placement.counts.special).toBe(report.initial.counts.special);
    }
  } finally {
    await page
      .evaluate(() => {
        const probe = Reflect.get(window, "moholeColdProbe");
        if (probe) {
          probe.runtime.dispose();
          probe.host.remove();
        }
      })
      .catch(() => {});
    await context.close();
  }
});

test("cancels shared special preparation across scene replacement", async ({
  browser,
  baseURL,
}) => {
  test.skip(
    test.info().project.name === "engine-webgl2",
    "Raw GPUBuffer instrumentation targets WebGPU",
  );
  test.setTimeout(150_000);
  const context = await browser.newContext({ baseURL });
  await installSpecialPreparationBufferProbe(context);
  const page = await context.newPage();
  try {
    await page.goto("/tests/fixtures/engine.html");
    const buffers = await page.evaluate(async () => {
      const rp = "/src/engine/runtime.ts",
        sp = "/src/engine/scene.ts",
        fp = "/tests/fixtures/scene.ts";
      const [{ startEngine }, { toSceneSnapshot }, { sceneGame }] = await Promise.all([
        import(rp),
        import(sp),
        import(fp),
      ]);
      const host = document.createElement("div");
      document.body.append(host);
      let phase = "starting",
        failure: string | undefined;
      const runtime = startEngine(
        host,
        (s: { phase: string; message?: string }) => {
          phase = s.phase;
          if (s.phase === "error") failure = s.message;
        },
        "webgpu",
      );
      const counts = () =>
        Reflect.get(window, "specialPreparationBuffers") as {
          created: number;
          destroyed: number;
          live: number;
        };
      const until = async (predicate: () => boolean, timeout = 100_000) => {
        const deadline = performance.now() + timeout;
        while (!predicate()) {
          if (failure) throw new Error(failure);
          if (performance.now() > deadline)
            throw new Error("Preparation cancellation: " + JSON.stringify(counts()));
          await new Promise(requestAnimationFrame);
        }
      };
      try {
        await until(() => phase === "ready");
        runtime.setScene(
          toSceneSnapshot(sceneGame("cancel-special-preparation"), null),
          "snapshot",
        );
        await until(() => counts().live > 0);
        const during = { ...counts() };
        // Keep the renderer/device alive: full disposal destroys the device,
        // which releases its resources without individual Buffer.destroy calls.
        runtime.setScene(
          toSceneSnapshot(sceneGame("replacement-after-special-preparation"), null),
          "snapshot",
        );
        await until(
          () =>
            counts().created > during.created &&
            counts().live === 0 &&
            runtime.assetStats().specialSurfaces.ready,
        );
        return { during, after: { ...counts() } };
      } finally {
        runtime.dispose();
        host.remove();
      }
    });
    expect(buffers.during.live).toBe(2);
    expect(buffers.after.created).toBeGreaterThanOrEqual(2);
    expect(buffers.after.destroyed).toBe(buffers.after.created);
    expect(buffers.after.live).toBe(0);
  } finally {
    await context.close();
  }
});
