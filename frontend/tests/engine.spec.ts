import type { SceneSound, FocusTarget } from "../src/engine/scene.generated";
import {
  test,
  expect,
  devices,
  type Page,
  type Browser,
  type BrowserContextOptions,
} from "@playwright/test";
import path from "node:path";
import sharp from "sharp";
import { mkdir, writeFile } from "node:fs/promises";
import type {
  SceneDiagnostics,
  FaunaDiagnostics,
  WorldTreeDiagnostics,
  WorksiteDiagnostics,
  SpecialSurfaceDiagnostics,
  ParticleDiagnostics,
  CraterDiagnostics,
} from "../src/engine/diagnostics";
import {
  sceneGame,
  labelSceneGame,
  oceanSceneGame,
  citySceneGame,
  faunaSceneGame,
  worldTreeSceneGame,
  worksiteSceneGame,
  specialSceneGame,
  particleSceneGame,
  nuclearSceneGame,
  nuclearTransitionGame,
  nuclearCollapseGame,
  atmosphereSceneGame,
  offworldSceneGame,
} from "./fixtures/scene";
import { assets, type AssetId } from "../src/assets/generated/registry";
import { gameAssets } from "../src/engine/scene.generated";

// Keep source-visual and lifetime regressions at an explicit quality. The
// graphics suite separately exercises adaptive defaults, every tier and overrides.
const referenceGraphics = {
  textureQuality: "high",
  renderScale: 1,
  antialiasing: true,
  effects: "high",
  frameLimit: 0,
  backgroundBrightness: 0.65,
};
function referencePreferences(settings: typeof referenceGraphics) {
  localStorage.setItem("openmars.graphics.settings", JSON.stringify(settings));
  // Projection fixtures aim with literal pixel/radian math; real preference
  // behavior is covered by engine-refinements and camera controls tests.
  localStorage.setItem(
    "openmars.camera.settings",
    JSON.stringify({ orbitSensitivity: 1, zoomSensitivity: 1, smoothing: 0 }),
  );
}
async function referenceContext(browser: Browser, options?: BrowserContextOptions) {
  const context = await browser.newContext(options);
  await context.addInitScript(referencePreferences, referenceGraphics);
  return context;
}
test.beforeEach(async ({ page }) => {
  await page.addInitScript(referencePreferences, referenceGraphics);
});

const backend = () => (test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu");
const engineUrl = () => `/dev/engine?renderer=${backend()}`;

async function trackTerrainBuffers(page: Page) {
  await page.addInitScript(() => {
    const blastLive = new Set<GPUBuffer>();
    const blastMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "blastBuffers", blastMetrics);
    const craterLive = new Set<GPUBuffer>();
    const craterMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "craterBuffers", craterMetrics);
    const particleLive = new Set<GPUBuffer>();
    const particleMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "particleBuffers", particleMetrics);
    const specialLive = new Set<GPUBuffer>();
    const specialMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "specialBuffers", specialMetrics);
    const worksiteLive = new Set<GPUBuffer>();
    const worksiteMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "worksiteBuffers", worksiteMetrics);
    const worldTreeLive = new Set<GPUBuffer>();
    const worldTreeMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "worldTreeBuffers", worldTreeMetrics);
    const structureLive = new Set<GPUBuffer>();
    const structureMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "structureBuffers", structureMetrics);
    const shadeLive = new Set<GPUBuffer>();
    const shadeMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "shadeBuffers", shadeMetrics);
    const gridLive = new Set<GPUBuffer>();
    const gridMetrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "gridBuffers", gridMetrics);
    const textureMetrics = { created: 0, writes: 0 };
    Reflect.set(window, "gridTextures", textureMetrics);
    const createTexture = GPUDevice.prototype.createTexture;
    GPUDevice.prototype.createTexture = function (descriptor) {
      const texture = createTexture.call(this, descriptor);
      if (descriptor.label === "tile highlight data") {
        textureMetrics.created++;
      }
      return texture;
    };
    const writeTexture = GPUQueue.prototype.writeTexture;
    GPUQueue.prototype.writeTexture = function (...args) {
      if (args[0].texture.label === "tile highlight data") {
        textureMetrics.writes++;
      }
      Reflect.apply(writeTexture, this, args);
    };
    const live = new Set<GPUBuffer>();
    const metrics = { created: 0, destroyed: 0, live: 0 };
    Reflect.set(window, "terrainBuffers", metrics);
    const create = GPUDevice.prototype.createBuffer;
    const destroy = GPUBuffer.prototype.destroy;
    GPUDevice.prototype.createBuffer = function (descriptor) {
      const buffer = create.call(this, descriptor);
      if (descriptor.label === "nuclear compositor parameters") {
        blastLive.add(buffer);
        blastMetrics.created++;
        blastMetrics.live = blastLive.size;
      }
      if (/terrain (parameters|highlights)/.test(descriptor.label ?? "")) {
        live.add(buffer);
        metrics.created++;
        metrics.live = live.size;
      }
      if (/^crater (shared parameters|highlights)$/.test(descriptor.label ?? "")) {
        craterLive.add(buffer);
        craterMetrics.created++;
        craterMetrics.live = craterLive.size;
      }
      if (descriptor.label === "particle palette") {
        particleLive.add(buffer);
        particleMetrics.created++;
        particleMetrics.live = particleLive.size;
      }
      if (/^special surface (parameters|highlights)$/.test(descriptor.label ?? "")) {
        specialLive.add(buffer);
        specialMetrics.created++;
        specialMetrics.live = specialLive.size;
      }
      if (/^worksite (parameters|highlights)$/.test(descriptor.label ?? "")) {
        worksiteLive.add(buffer);
        worksiteMetrics.created++;
        worksiteMetrics.live = worksiteLive.size;
      }
      if (descriptor.label === "world tree parameters") {
        worldTreeLive.add(buffer);
        worldTreeMetrics.created++;
        worldTreeMetrics.live = worldTreeLive.size;
      }
      if (descriptor.label === "structure parameters") {
        structureLive.add(buffer);
        structureMetrics.created++;
        structureMetrics.live = structureLive.size;
      }
      if (descriptor.label === "plant shade instances") {
        shadeLive.add(buffer);
        shadeMetrics.created++;
        shadeMetrics.live = shadeLive.size;
      }
      if (descriptor.label === "grid highlights") {
        gridLive.add(buffer);
        gridMetrics.created++;
        gridMetrics.live = gridLive.size;
      }
      return buffer;
    };
    GPUBuffer.prototype.destroy = function () {
      if (blastLive.delete(this)) {
        blastMetrics.destroyed++;
        blastMetrics.live = blastLive.size;
      }
      if (live.delete(this)) {
        metrics.destroyed++;
        metrics.live = live.size;
      }
      if (craterLive.delete(this)) {
        craterMetrics.destroyed++;
        craterMetrics.live = craterLive.size;
      }
      if (particleLive.delete(this)) {
        particleMetrics.destroyed++;
        particleMetrics.live = particleLive.size;
      }
      if (specialLive.delete(this)) {
        specialMetrics.destroyed++;
        specialMetrics.live = specialLive.size;
      }
      if (worksiteLive.delete(this)) {
        worksiteMetrics.destroyed++;
        worksiteMetrics.live = worksiteLive.size;
      }
      if (worldTreeLive.delete(this)) {
        worldTreeMetrics.destroyed++;
        worldTreeMetrics.live = worldTreeLive.size;
      }
      if (structureLive.delete(this)) {
        structureMetrics.destroyed++;
        structureMetrics.live = structureLive.size;
      }
      if (shadeLive.delete(this)) {
        shadeMetrics.destroyed++;
        shadeMetrics.live = shadeLive.size;
      }
      if (gridLive.delete(this)) {
        gridMetrics.destroyed++;
        gridMetrics.live = gridLive.size;
      }
      destroy.call(this);
    };
  });
}

test.describe("Bevy renderer", () => {
  test.skip(({ browserName }) => browserName !== "chromium", "WebGPU smoke test runs in Chromium");
  test.beforeEach(({}, info) => {
    test.skip(!info.project.name.startsWith("engine"));
  });
  test.setTimeout(120_000);

  for (const [layout, profile] of [
    ["desktop", { viewport: { width: 1280, height: 800 } }],
    ["iphone", devices["iPhone 14 landscape"]],
    ["android", devices["Pixel 7 landscape"]],
  ] as const) {
    test(`shows the active tile placement instruction on ${layout}`, async ({
      browser,
      baseURL,
    }) => {
      const context = await referenceContext(browser, { ...profile, baseURL });
      try {
        const page = await context.newPage();
        await page.goto(engineUrl());
        await expect(page.locator("[data-engine-state]")).toHaveAttribute(
          "data-engine-state",
          "ready",
          { timeout: 90_000 },
        );
        await expect(page.getByText("Place", { exact: true })).toHaveCount(0);
        const game = sceneGame();
        await page.evaluate(async (game) => {
          const gamePath = "/src/stores/gameStore.ts";
          const phasePath = "/src/stores/appPhaseStore.ts";
          const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
            import(/* @vite-ignore */ gamePath),
            import(/* @vite-ignore */ phasePath),
          ]);
          useGameStore.getState().setGame(game);
          useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
        }, game);
        for (const tileType of ["city", "greenery", "ocean", null]) {
          await page.evaluate(async (tileType) => {
            const path = "/src/stores/gameStore.ts";
            const { useGameStore } = await import(/* @vite-ignore */ path);
            const state = useGameStore.getState();
            state.setGame({
              ...state.game,
              currentPlayer: {
                ...state.game.currentPlayer,
                pendingTileSelection: tileType
                  ? { tileType, availableHexes: ["0,0,0"], source: "standard-project" }
                  : undefined,
              },
            });
          }, tileType);
          const instruction = page.getByText("Place", { exact: true });
          if (!tileType) {
            await expect(instruction).toHaveCount(0);
            continue;
          }
          await expect(instruction).toBeVisible();
          await expect(
            page.getByRole("img", { name: `${tileType}-tile`, exact: true }),
          ).toBeVisible();
          const desktopPrompt = page.getByRole("group", { name: "Tile placement" });
          if (layout === "desktop") {
            await expect(desktopPrompt).toBeVisible();
            const bounds = await desktopPrompt.boundingBox();
            expect(bounds!.y).toBe(66);
            expect(
              Math.abs(bounds!.x + bounds!.width / 2 - page.viewportSize()!.width / 2),
            ).toBeLessThan(2);
          } else {
            await expect(desktopPrompt).toHaveCount(0);
          }
          if (tileType === "city") {
            await expect(page.locator("[data-engine-state]")).toHaveAttribute(
              "data-engine-state",
              "ready",
              { timeout: 90_000 },
            );
            await page
              .locator('[role="status"]')
              .filter({
                has: page.getByRole("img", { name: "Open Mars", exact: true }),
              })
              .waitFor({ state: "hidden" });
            const output = path.resolve(import.meta.dirname, "../../tmp/placement-prompt");
            await mkdir(output, { recursive: true });
            await page.screenshot({ path: path.join(output, `${backend()}-${layout}.png`) });
          }
        }
      } finally {
        await context.close();
      }
    });
  }

  for (const crossOrigin of [false, true]) {
    test(`prepares terrain from a ${crossOrigin ? "gateway" : "direct"} page origin`, async ({
      page,
      baseURL,
    }) => {
      const origin = new URL(baseURL!).origin;
      const documentOrigin = crossOrigin ? origin.replace("localhost", "127.0.0.1") : origin;
      await page.goto(`${documentOrigin}/tests/fixtures/engine.html`);
      const result = await page.evaluate(
        async ({ origin, renderer, game }) => {
          const runtimePath = `${origin}/src/engine/runtime.ts`;
          const scenePath = `${origin}/src/engine/scene.ts`;
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ runtimePath),
            import(/* @vite-ignore */ scenePath),
          ]);
          const blobs = new Set<string>();
          let created = 0;
          const create = URL.createObjectURL;
          const revoke = URL.revokeObjectURL;
          URL.createObjectURL = function (value) {
            const url = create.call(this, value);
            blobs.add(url);
            created++;
            return url;
          };
          URL.revokeObjectURL = function (url) {
            blobs.delete(url);
            revoke.call(this, url);
          };
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let phase = "starting";
          let failure: string | undefined;
          const runtime = startEngine(
            host,
            (status: { phase: string; message?: string }) => {
              phase = status.phase;
              if (phase === "error") failure = status.message;
            },
            renderer,
          );
          try {
            runtime.setScene(toSceneSnapshot(game, null), "snapshot");
            const deadline = performance.now() + 100_000;
            while (phase !== "ready" || !runtime.sceneStats()?.boardReady) {
              if (failure) throw new Error(failure);
              if (performance.now() > deadline) throw new Error("Terrain did not become ready");
              await new Promise(requestAnimationFrame);
            }
          } finally {
            runtime.dispose();
            host.remove();
            URL.createObjectURL = create;
            URL.revokeObjectURL = revoke;
          }
          return {
            created,
            live: blobs.size,
            canvases: document.querySelectorAll("canvas").length,
          };
        },
        { origin, renderer: backend(), game: sceneGame() },
      );
      expect(result.created).toBe(crossOrigin ? 1 : 0);
      expect(result.live).toBe(0);
      expect(result.canvases).toBe(0);
    });
  }

  for (const [device, profile] of [
    ["desktop", { viewport: { width: 1280, height: 720 } }],
    ["iphone", devices["iPhone 14 landscape"]],
    ["android", devices["Pixel 7 landscape"]],
  ] as const) {
    test(`renders climate atmosphere with stable shared storage on ${device}`, async ({
      browser,
      baseURL,
    }) => {
      const context = await referenceContext(browser, { ...profile, baseURL });
      const page = await context.newPage();
      test.setTimeout(180_000);
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.addInitScript(() => {
        const live = new Set<GPUBuffer>();
        const metrics = { created: 0, destroyed: 0, live: 0 };
        Reflect.set(window, "atmosphereBuffers", metrics);
        const create = GPUDevice.prototype.createBuffer;
        const destroy = GPUBuffer.prototype.destroy;
        GPUDevice.prototype.createBuffer = function (descriptor) {
          const buffer = create.call(this, descriptor);
          if (descriptor.label === "planet haze parameters") {
            live.add(buffer);
            metrics.created++;
            metrics.live = live.size;
          }
          return buffer;
        };
        GPUBuffer.prototype.destroy = function () {
          if (live.delete(this)) {
            metrics.destroyed++;
            metrics.live = live.size;
          }
          destroy.call(this);
        };
      });
      const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
      await mkdir(output, { recursive: true });
      try {
        await page.goto("/tests/fixtures/engine.html");
        const cold = await page.evaluate(
          async ({ game, renderer }) => {
            const rp = "/src/engine/runtime.ts",
              sp = "/src/engine/scene.ts";
            const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
              import(/* @vite-ignore */ rp),
              import(/* @vite-ignore */ sp),
            ]);
            const host = document.createElement("div");
            host.style.cssText = "position:fixed;inset:0";
            document.body.append(host);
            let resolve = () => {},
              reject = (_error: Error) => {},
              failure: string | null = null;
            const next = () =>
              new Promise<void>((yes, no) => {
                resolve = yes;
                reject = no;
              });
            const first = next();
            const runtime = startEngine(
              host,
              (s: { phase: string; message: string }) => {
                if (s.phase === "ready") resolve();
                if (s.phase === "error") {
                  failure = s.message;
                  reject(new Error(s.message));
                }
              },
              renderer,
            );
            const stats = () => runtime.sceneStats() as SceneDiagnostics;
            const wait = async (condition: () => boolean) => {
              for (let i = 0; i < 6000; i++) {
                if (failure) throw new Error(failure);
                if (condition()) return;
                await new Promise(requestAnimationFrame);
              }
              throw new Error("Atmosphere deadline: " + JSON.stringify(stats()));
            };
            Reflect.set(window, "atmosphereProbe", {
              runtime,
              host,
              game,
              stats,
              wait,
              next,
              toSceneSnapshot,
            });
            await first;
            Reflect.set(window, "atmosphereBaseline", runtime.assetStats());
            const ready = next();
            runtime.setScene(toSceneSnapshot(game, null), "snapshot");
            await ready;
            const writes = stats().atmosphere.writes;
            runtime.camera(0.1);
            await wait(() => stats().atmosphere.writes > writes + 5);
            runtime.setPaused(true);
            return stats().atmosphere;
          },
          {
            game: atmosphereSceneGame(),
            renderer: backend(),
          },
        );
        expect(cold.ready).toBe(true);
        expect(cold.sample.map((body) => body.id).sort()).toEqual(
          [
            "mars",
            "body-phobos",
            "body-mercury",
            "body-venus",
            "body-earth",
            "body-moon",
            "body-ceres",
            "body-jupiter",
            "body-ganymede",
            "body-saturn",
            "body-uranus",
            "body-neptune",
          ].sort(),
        );
        expect(cold.bodies).toBe(cold.sample.length);
        expect(cold.bytes).toBe(1088);
        expect(cold.sample.every((body) => body.warmFrames === 2)).toBe(true);
        expect(cold.profiles![0][0]).toBeCloseTo(0.045, 6);
        const environment = await page.evaluate(
          () =>
            Reflect.get(window, "atmosphereProbe").stats().lighting as SceneDiagnostics["lighting"],
        );
        expect(environment.views).toBe(1);
        expect(
          environment.maps.map(({ size, mips, faces, format }) => ({ size, mips, faces, format })),
        ).toEqual([
          { size: 32, mips: 1, faces: 6, format: "Rgba16Float" },
          { size: 128, mips: 8, faces: 6, format: "Rgba16Float" },
        ]);
        expect(environment.maps.reduce((sum, map) => sum + map.bytes, 0)).toBe(1097712);

        const coldImage = await page.screenshot({
          path: path.join(output, `phase5-atmosphere-${backend()}-${device}-cold.png`),
        });
        if (device === "desktop") {
          // The +Y fill alone leaves this bare lower hemisphere black. This
          // catches WebGL2 silently dropping the sun at its one-light limit.
          const { data, info } = await sharp(coldImage)
            .extract({ left: 600, top: 510, width: 80, height: 60 })
            .removeAlpha()
            .raw()
            .toBuffer({ resolveWithObject: true });
          let red = 0;
          for (let i = 0; i < data.length; i += info.channels) red += data[i];
          expect(red / (info.width * info.height)).toBeGreaterThan(50);
        }
        const transition = await page.evaluate(async () => {
          const { runtime, game, stats, wait, toSceneSnapshot } = Reflect.get(
            window,
            "atmosphereProbe",
          );
          const start = stats().step;
          game.globalParameters.temperature = 8;
          runtime.setScene(toSceneSnapshot(game, null), "live");
          runtime.setPaused(false);
          const samples: [number, number][] = [];
          await wait(() => {
            const p = stats().atmosphere.profiles?.[0];
            if (p && samples.length < 256) samples.push([p[0], p[1]]);
            return stats().step >= start + 120;
          });
          runtime.setPaused(true);
          return { samples, atmosphere: stats().atmosphere as SceneDiagnostics["atmosphere"] };
        });
        expect(transition.atmosphere.buffer).toBe(cold.buffer);
        expect(
          transition.samples.some(([thickness]) => thickness > 0.046 && thickness < 0.069),
        ).toBe(true);
        expect(
          transition.samples.some(([thickness, intensity]) => {
            const base =
              thickness <= 0.055 ? 0.85 + (thickness - 0.045) * 25 : 1.1 + (thickness - 0.055) * 10;
            return intensity > base * 0.65 + 0.02;
          }),
        ).toBe(true);
        const hot = await page.evaluate(async () => {
          const { runtime, game, next, stats, wait, toSceneSnapshot } = Reflect.get(
            window,
            "atmosphereProbe",
          );
          game.globalParameters.temperature = 8;
          game.globalParameters.oxygen = 14;
          game.globalParameters.oceans = 9;
          const ocean = game.board.tiles.find(
            (t: { coordinates: { q: number; r: number } }) =>
              t.coordinates.q === 0 && t.coordinates.r === 0,
          );
          ocean.occupiedBy = { type: "ocean-tile", tags: [] };
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          runtime.setPaused(false);
          await ready;
          await wait(() => stats().atmosphere.profiles?.[0][0] > 0.0699);
          runtime.setPaused(true);
          return stats().atmosphere as SceneDiagnostics["atmosphere"];
        });
        expect(hot.buffer).toBe(cold.buffer);
        expect(hot.allocations).toBe(1);
        expect(hot.profiles![0][0]).toBeCloseTo(0.07, 6);
        expect(hot.sample.find((body) => body.id === "mars")!.entities).toEqual(
          cold.sample.find((body) => body.id === "mars")!.entities,
        );
        await page.screenshot({
          path: path.join(output, `phase5-atmosphere-${backend()}-${device}-hot.png`),
        });
        const actors = await page.evaluate(async () => {
          const { runtime, stats, wait } = Reflect.get(window, "atmosphereProbe");
          const writes = stats().atmosphere.writes;
          runtime.setPaused(false);
          runtime.camera(-0.85);
          await wait(() => stats().atmosphere.writes > writes + 5);
          runtime.setPaused(true);
          const a = runtime.assetStats();
          return {
            atmosphere: stats().atmosphere as SceneDiagnostics["atmosphere"],
            cameraDistance: stats().camera.distance as number,
            birds: a.fauna.birds,
            plants: a.foliagePlants,
            cities: a.structures.plots,
            trees: a.worldTrees.trees,
            worksites: a.worksites.sites,
            special: a.specialSurfaces.sites,
            craters: a.craters.sites,
          };
        });
        expect(actors.atmosphere.buffer).toBe(cold.buffer);
        expect(actors.cameraDistance).toBeGreaterThan(3.1);
        expect(actors.cameraDistance).toBeLessThan(3.5);
        expect(actors.birds).toBeGreaterThan(0);
        expect(actors.plants).toBeGreaterThan(0);
        expect(actors.cities).toBe(1);
        expect(actors.trees).toBe(1);
        expect(actors.worksites).toBe(2);
        expect(actors.special).toBe(2);
        expect(actors.craters).toBe(1);
        await page.screenshot({
          path: path.join(output, `phase5-atmosphere-${backend()}-${device}-surface.png`),
        });
        const size = page.viewportSize()!;
        await page.setViewportSize({ width: size.width + 96, height: size.height + 48 });
        const resized = await page.evaluate(async () => {
          const { runtime, stats, wait } = Reflect.get(window, "atmosphereProbe");
          const writes = stats().atmosphere.writes;
          runtime.setPaused(false);
          await wait(() => stats().atmosphere.writes > writes + 5);
          runtime.setPaused(true);
          return stats().atmosphere as SceneDiagnostics["atmosphere"];
        });
        expect(resized.buffer).toBe(cold.buffer);
        expect(resized.allocations).toBe(1);
        const cleared = await page.evaluate(async () => {
          const { runtime, stats, wait } = Reflect.get(window, "atmosphereProbe");
          runtime.setScene(null);
          runtime.setPaused(false);
          await wait(
            () =>
              stats().atmosphere.uniforms === 0 &&
              stats().atmosphere.retired === 0 &&
              stats().atmosphere.retirements === 1,
          );
          return {
            atmosphere: stats().atmosphere as SceneDiagnostics["atmosphere"],
            lighting: stats().lighting as SceneDiagnostics["lighting"],
            buffers: Reflect.get(window, "atmosphereBuffers"),
          };
        });
        expect(cleared.atmosphere.bodies).toBe(0);
        expect(cleared.atmosphere.sample).toHaveLength(0);
        expect(cleared.lighting).toEqual({
          background: null,
          views: 0,
          maps: [],
          clusterBufferAllocations: expect.any(Number),
          // The persistent main camera remains; retired game/preparation views do not.
          clusterViews: 1,
        });
        if (backend() === "webgpu")
          expect(cleared.buffers).toEqual({ created: 1, destroyed: 1, live: 0 });
        expect(errors).toEqual([]);
      } finally {
        await page
          .evaluate(() => {
            const p = Reflect.get(window, "atmosphereProbe");
            p?.runtime.dispose();
            p?.host.remove();
          })
          .catch(() => {});
        await context.close();
      }
    });

    test(`presents public asteroid cues once with source flight dust and quiet cancellation on ${device}`, async ({
      browser,
      baseURL,
    }) => {
      const context = await referenceContext(browser, { ...profile, baseURL });
      const page = await context.newPage();
      test.setTimeout(240_000);
      const requests: string[] = [];
      const errors: string[] = [];
      page.on("request", (request) => {
        if (request.url().includes("/assets/models/asteroid.")) {
          requests.push(request.url());
        }
      });
      page.on("pageerror", (error) => errors.push(error.message));
      await page.goto("/tests/fixtures/engine.html");
      const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
      await mkdir(output, { recursive: true });
      try {
        const initial = await page.evaluate(
          async ({ game, renderer }) => {
            const rp = "/src/engine/runtime.ts",
              sp = "/src/engine/scene.ts",
              pp = "/src/engine/presentation.ts";
            const [{ startEngine }, { toSceneSnapshot }, { publicSceneCues }] = await Promise.all([
              import(/* @vite-ignore */ rp),
              import(/* @vite-ignore */ sp),
              import(/* @vite-ignore */ pp),
            ]);
            const host = document.createElement("div");
            host.style.cssText = "position:fixed;inset:0";
            document.body.append(host);
            let resolve = () => {},
              reject = (_e: Error) => {},
              failure: string | null = null;
            const next = () =>
              new Promise<void>((yes, no) => {
                resolve = yes;
                reject = no;
              });
            const first = next();
            const runtime = startEngine(
              host,
              (s: { phase: string; message: string }) => {
                if (s.phase === "ready") {
                  resolve();
                }
                if (s.phase === "error") {
                  failure = s.message;
                  reject(new Error(s.message));
                }
              },
              renderer,
            );
            const stats = () => runtime.sceneStats() as SceneDiagnostics;
            const wait = async (condition: () => boolean) => {
              for (let i = 0; i < 18000; i++) {
                if (failure) {
                  throw new Error(failure);
                }
                if (condition()) {
                  return;
                }
                await new Promise(requestAnimationFrame);
              }
              throw new Error("Asteroid deadline: " + JSON.stringify(stats().asteroids));
            };
            const cue = (sequenceNumber: number) =>
              publicSceneCues(
                {
                  isHistory: false,
                  logs: [
                    {
                      gameId: game.id,
                      sequenceNumber,
                      sourceType: "standard_project",
                      source: "Asteroid",
                      playerId: game.currentPlayer.id,
                    },
                  ],
                },
                game,
              )[0];
            Reflect.set(window, "asteroidProbe", {
              runtime,
              host,
              game,
              toSceneSnapshot,
              stats,
              wait,
              next,
              cue,
            });
            await first;
            Reflect.set(window, "asteroidBaseline", runtime.assetStats());
            const ready = next();
            let lastPreparation: unknown = null;
            const preparationProbe = setInterval(() => {
              lastPreparation = { scene: stats(), assets: runtime.assetStats() };
            }, 1000);
            runtime.setScene(toSceneSnapshot(game, null), "snapshot");
            try {
              await ready;
            } catch (error) {
              throw new Error(
                String(error) + "\nLast preparation: " + JSON.stringify(lastPreparation),
              );
            } finally {
              clearInterval(preparationProbe);
            }
            runtime.camera(-0.3);
            return stats();
          },
          { game: sceneGame("asteroid-public"), renderer: backend() },
        );
        expect(initial.asteroids.active).toBeNull();
        expect(initial.asteroids.started).toBe(0);
        const flight = await page.evaluate(async () => {
          const { runtime, stats, wait, cue } = Reflect.get(window, "asteroidProbe");
          if (!runtime.present(cue(3)) || runtime.present(cue(3))) {
            throw new Error("JS cue admission");
          }
          await wait(() => {
            const s = stats();
            return (
              s.asteroids.active?.start !== null &&
              s.step >= (s.asteroids.active?.start ?? Infinity) + 90
            );
          });
          runtime.setPaused(true);
          return stats() as SceneDiagnostics;
        });
        expect(flight.presentation.seen).toBe(1);
        expect(flight.asteroids.active?.warmFrames).toBe(2);
        expect(flight.asteroids.active?.parts).toBe(1);
        expect(flight.asteroids.active?.sprites).toBe(60);
        expect(flight.asteroids.active?.scale).toBeCloseTo(
          0.15 / (1.4869314432144165 + 1.3308839797973633),
          6,
        );
        const start = flight.asteroids.active!.start!;
        expect(flight.step - start).toBeLessThan(108);
        expect(flight.asteroids.active!.dustSample.every((p) => (p.tag & 0x80000000) !== 0)).toBe(
          true,
        );
        await page.screenshot({
          path: path.join(output, `phase5-asteroid-${backend()}-${device}-flight.png`),
        });
        const dust = await page.evaluate(async (start) => {
          const { runtime, stats, wait } = Reflect.get(window, "asteroidProbe");
          runtime.setPaused(false);
          await wait(() => stats().step >= start + 140);
          runtime.setPaused(true);
          return stats() as SceneDiagnostics;
        }, start);
        expect(
          dust.asteroids.active!.dustSample.every((p) => (p.tag & 0x80000000) === 0 && p.tag > 0),
        ).toBe(true);
        await page.screenshot({
          path: path.join(output, `phase5-asteroid-${backend()}-${device}-dust.png`),
        });
        expect(
          (
            await page.evaluate(
              () => Reflect.get(window, "asteroidProbe").stats() as SceneDiagnostics,
            )
          ).asteroids.active,
        ).toEqual(dust.asteroids.active);
        const expired = await page.evaluate(async () => {
          const { runtime, stats, wait, cue } = Reflect.get(window, "asteroidProbe");
          runtime.setPaused(false);
          await wait(() => stats().asteroids.completed === 1 && stats().asteroids.retired === 0);
          runtime.present(cue(3));
          const step = stats().step;
          await wait(() => stats().step >= step + 20);
          return stats() as SceneDiagnostics;
        });
        expect(expired.asteroids.started).toBe(1);
        expect(expired.asteroids.active).toBeNull();
        expect(expired.asteroids.shared).toBe(false);
        const quiet = await page.evaluate(async () => {
          const { runtime, stats, wait, cue, game, toSceneSnapshot, next } = Reflect.get(
            window,
            "asteroidProbe",
          );
          runtime.present(cue(8));
          runtime.present(cue(9));
          await wait(
            () => stats().asteroids.started === 2 && stats().asteroids.active?.warmFrames === 2,
          );
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          await ready;
          await wait(() => stats().asteroids.retired === 0);
          runtime.setPaused(true);
          return stats() as SceneDiagnostics;
        });
        expect(quiet.asteroids.active).toBeNull();
        expect(quiet.asteroids.canceled).toBe(1);
        expect(quiet.presentation.queued).toBe(0);
        expect(quiet.presentation.seen).toBe(3);
        await page.screenshot({
          path: path.join(output, `phase5-asteroid-${backend()}-${device}-quiet.png`),
        });
        await page.emulateMedia({ reducedMotion: "reduce" });
        const reduced = await page.evaluate(async () => {
          const { runtime, stats, wait, cue } = Reflect.get(window, "asteroidProbe");
          runtime.setPaused(false);
          await wait(() => runtime.assetStats().reducedMotion);
          runtime.present(cue(10));
          await wait(() => stats().presentation.seen === 4);
          return stats() as SceneDiagnostics;
        });
        expect(reduced.asteroids.started).toBe(2);
        expect(reduced.asteroids.active).toBeNull();
        await page.emulateMedia({ reducedMotion: "no-preference" });
        const cleared = await page.evaluate(async () => {
          const { runtime, stats, wait, next } = Reflect.get(window, "asteroidProbe");
          const ready = next();
          runtime.setScene(null);
          await ready;
          await wait(() => stats().asteroids.retired === 0);
          const baseline = Reflect.get(window, "asteroidBaseline");
          const keys = ["meshes", "materials", "images", "gpuImages", "gpuMeshes", "gpuMaterials"];
          await wait(() => keys.every((key) => runtime.assetStats()[key] === baseline[key]));
          return { scene: stats(), assets: runtime.assetStats(), baseline };
        });
        expect(cleared.scene.presentation.seen).toBe(0);
        expect(cleared.scene.asteroids.active).toBeNull();
        expect(cleared.scene.asteroids.shared).toBe(false);
        for (const key of [
          "meshes",
          "materials",
          "images",
          "gpuImages",
          "gpuMeshes",
          "gpuMaterials",
        ]) {
          expect(cleared.assets[key]).toBe(cleared.baseline[key]);
        }
        expect(requests).toHaveLength(1);
        expect(errors).toEqual([]);
        await writeFile(
          path.join(output, `phase5-asteroid-${backend()}-${device}-diagnostics.json`),
          JSON.stringify({ initial, flight, dust, expired, quiet, reduced, cleared }, null, 2),
        );
      } finally {
        await page
          .evaluate(() => {
            const p = Reflect.get(window, "asteroidProbe");
            if (p) {
              p.runtime.dispose();
              p.host.remove();
              Reflect.deleteProperty(window, "asteroidProbe");
            }
          })
          .catch(() => {});
        await context.close();
      }
    });
  }

  test("owns effect audio handles across duplicate cues, pause and disposal", async ({ page }) => {
    await page.goto("/tests/fixtures/engine.html");
    const counts = await page.evaluate(async () => {
      const effectPath = "/src/engine/effectAudio.ts";
      const servicePath = "/src/services/audioService.ts";
      const [{ effectAudio }, { audioService }] = await Promise.all([
        import(/* @vite-ignore */ effectPath),
        import(/* @vite-ignore */ servicePath),
      ]);
      const previousPlay = audioService.playNuclearBlastSound;
      const previousStop = audioService.stopNuclearBlastSound;
      const counts = { starts: 0, stops: 0, pauses: 0, resumes: 0 };
      audioService.playNuclearBlastSound = () => {
        counts.starts++;
        const audio = new Audio();
        audio.pause = () => {
          counts.pauses++;
        };
        audio.play = () => {
          counts.resumes++;
          return Promise.resolve();
        };
        return audio;
      };
      audioService.stopNuclearBlastSound = (audio: HTMLAudioElement | null) => {
        if (audio) {
          counts.stops++;
        }
      };
      const audio = effectAudio();
      const event = { gameId: "game", id: "blast-1", sound: "nuclear-blast", action: "start" };
      try {
        audio.dispatch(event);
        audio.dispatch(event);
        audio.pause(true);
        audio.pause(true);
        audio.pause(false);
        audio.dispatch({ ...event, action: "stop" });
        audio.dispatch({ ...event, action: "stop" });
        audio.dispatch({ ...event, id: "blast-2" });
        audio.dispose();
        audio.dispose();
        return counts;
      } finally {
        audio.dispose();
        audioService.playNuclearBlastSound = previousPlay;
        audioService.stopNuclearBlastSound = previousStop;
      }
    });
    expect(counts).toEqual({ starts: 2, stops: 2, pauses: 1, resumes: 1 });
  });

  test.use({ actionTimeout: 15_000 });

  for (const [name, pattern, id] of [
    ["model", "**/assets/models/rock.*.glb", "models/rock"],
    ["cubemap", "**/assets/textures/skyboxes/*/starmap.*.ktx2", "textures/skyboxes/starmap"],
    [
      "texture array",
      "**/assets/textures/terrain/*/color-array.*.ktx2",
      "textures/terrain/color-array",
    ],
  ]) {
    test(`retries a corrupt ${name} without keeping its bytes`, async ({ page }) => {
      let requests = 0;
      await page.route(pattern, async (route) => {
        requests++;
        if (requests === 1) {
          await route.fulfill({
            status: 200,
            body: "Invalid encoded asset",
            contentType: "application/octet-stream",
          });
        } else {
          await route.continue();
        }
      });
      await page.goto(engineUrl());
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 90_000 },
      );
      await page.evaluate(async (game) => {
        const gamePath = "/src/stores/gameStore.ts";
        const phasePath = "/src/stores/appPhaseStore.ts";
        const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
          import(/* @vite-ignore */ gamePath),
          import(/* @vite-ignore */ phasePath),
        ]);
        useGameStore.getState().setGame(game);
        useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
      }, sceneGame());
      await expect(page.getByRole("alert")).toContainText(`Could not decode ${id}`, {
        timeout: 30_000,
      });
      await page.getByRole("button", { name: "Try again" }).click();
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 90_000 },
      );
      expect(requests).toBe(2);
    });
  }

  test("cancels obsolete world downloads when leaving a game", async ({ page }) => {
    let release = () => {};
    let requested = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    const requestStarted = new Promise<void>((resolve) => {
      requested = resolve;
    });
    await page.route("**/assets/models/rock.*.glb", async (route) => {
      requested();
      await gate;
      await route.continue().catch(() => {});
    });
    await page.goto(engineUrl());
    const ready = () =>
      expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready", {
        timeout: 90_000,
      });
    await ready();
    await page.evaluate(async (game) => {
      const gamePath = "/src/stores/gameStore.ts";
      const phasePath = "/src/stores/appPhaseStore.ts";
      const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
        import(/* @vite-ignore */ gamePath),
        import(/* @vite-ignore */ phasePath),
      ]);
      useGameStore.getState().setGame(game);
      useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
    }, sceneGame());
    try {
      await requestStarted;
      const cancelled = page.waitForEvent("requestfailed", (request) =>
        /\/rock\..*\.glb/.test(request.url()),
      );
      await page.evaluate(async () => {
        const path = "/src/stores/appPhaseStore.ts";
        const { useAppPhaseStore } = await import(/* @vite-ignore */ path);
        useAppPhaseStore.getState().resetToMenu();
      });
      await cancelled;
      await ready();
      await expect(page.getByRole("alert")).toHaveCount(0);
      await expect(page.locator("canvas")).toHaveCount(1);
    } finally {
      release();
    }
  });

  test("world assets retry after a failed download", async ({ page }) => {
    let failed = false;
    await page.route("**/assets/models/rock.*.glb", async (route) => {
      if (!failed) {
        failed = true;
        await route.fulfill({ status: 503, body: "Unavailable" });
      } else {
        await route.continue();
      }
    });
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 90_000 },
    );
    await page.evaluate(async (game) => {
      const gamePath = "/src/stores/gameStore.ts";
      const phasePath = "/src/stores/appPhaseStore.ts";
      const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
        import(/* @vite-ignore */ gamePath),
        import(/* @vite-ignore */ phasePath),
      ]);
      useGameStore.getState().setGame(game);
      useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
    }, sceneGame());
    await expect(page.getByRole("alert")).toContainText("World asset download failed (503)");
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 90_000 },
    );
    await expect(page.getByRole("alert")).toHaveCount(0);
  });

  test("compares live and cached city ground", async ({ page }) => {
    await page.goto("/tests/fixtures/engine.html");
    const initial = await page.evaluate(
      async ({ game, renderer }) => {
        const rp = "/src/engine/runtime.ts",
          sp = "/src/engine/scene.ts";
        const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
          import(/* @vite-ignore */ rp),
          import(/* @vite-ignore */ sp),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        let resolve = () => {},
          reject = (_e: Error) => {};
        const next = () =>
          new Promise<void>((r, j) => {
            resolve = r;
            reject = j;
          });
        const first = next();
        const runtime = startEngine(
          host,
          (s: { phase: string; message: string }) => {
            if (s.phase === "ready") resolve();
            if (s.phase === "error") reject(new Error(s.message));
          },
          renderer,
        );
        const wait = async (predicate: () => boolean) => {
          for (let i = 0; i < 4000; i++) {
            await new Promise(requestAnimationFrame);
            if (predicate()) return;
          }
          throw new Error(JSON.stringify(runtime.assetStats()));
        };
        Reflect.set(window, "cityGroundProbe", { runtime, host, wait });
        await first;
        const loaded = next();
        runtime.setScene(toSceneSnapshot(game, null));
        await loaded;
        const before = runtime.assetStats().terrainUniformWrites;
        runtime.camera(-1);
        await wait(
          () =>
            runtime.assetStats().terrainUniformWrites >= before + 2 &&
            runtime.assetStats().bakeValid === 0,
        );
        return {
          stats: runtime.assetStats(),
          point: (runtime.sceneStats() as SceneDiagnostics).tiles.find(
            (tile) => tile.id === "-1,0,1",
          )!.point,
        };
      },
      { game: citySceneGame(), renderer: backend() },
    );
    const output = path.resolve("../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    const live = await page.screenshot({
      path: path.join(output, `phase5-city-ground-${backend()}-live.png`),
    });
    const cachedStats = await page.evaluate(async () => {
      const { runtime, wait } = Reflect.get(window, "cityGroundProbe");
      await wait(() => runtime.assetStats().bakeValid === 1);
      const writes = runtime.assetStats().terrainUniformWrites;
      await wait(() => runtime.assetStats().terrainUniformWrites >= writes + 2);
      return runtime.assetStats();
    });
    const cached = await page.screenshot({
      path: path.join(output, `phase5-city-ground-${backend()}-cached.png`),
    });
    await page.evaluate(() => {
      const { runtime, host } = Reflect.get(window, "cityGroundProbe");
      runtime.dispose();
      host.remove();
      Reflect.deleteProperty(window, "cityGroundProbe");
    });
    // Follow the authored factory courtyard through camera framing changes.
    // A fixed screen rectangle can sample the neighbouring owner's bright border.
    expect(initial.point).not.toBeNull();
    const dimensions = await sharp(live).metadata();
    const viewport = page.viewportSize()!;
    const crop = {
      left: Math.round((initial.point![0] * dimensions.width!) / viewport.width) - 12,
      top: Math.round((initial.point![1] * dimensions.height!) / viewport.height) - 12,
      width: 24,
      height: 24,
    };
    expect(crop.left).toBeGreaterThanOrEqual(0);
    expect(crop.top).toBeGreaterThanOrEqual(0);
    expect(crop.left + crop.width).toBeLessThanOrEqual(dimensions.width!);
    expect(crop.top + crop.height).toBeLessThanOrEqual(dimensions.height!);
    const a = await sharp(live).extract(crop).removeAlpha().raw().toBuffer();
    const b = await sharp(cached).extract(crop).removeAlpha().raw().toBuffer();
    let difference = 0;
    for (let i = 0; i < a.length; i++) difference += Math.abs(a[i] - b[i]);
    difference /= a.length;
    await writeFile(
      path.join(output, `phase5-city-ground-${backend()}-stats.json`),
      JSON.stringify(
        { initial: initial.stats, cached: cachedStats, sample: initial.point, difference },
        null,
        2,
      ),
    );
    // This empty factory courtyard is fully paved in the authored field.
    // Live and cached images can agree while both expose the underlying globe.
    for (const pixels of [a, b]) {
      let red = 0,
        green = 0,
        blue = 0;
      for (let i = 0; i < pixels.length; i += 3) {
        red += pixels[i];
        green += pixels[i + 1];
        blue += pixels[i + 2];
      }
      const count = pixels.length / 3;
      red /= count;
      green /= count;
      blue /= count;
      expect(red).toBeLessThan(140);
      expect(Math.abs(red - blue)).toBeLessThan(45);
      expect(red + green + blue).toBeGreaterThan(60);
    }
    expect(difference).toBeLessThan(10);
  });

  test("renders authored cities and retains unaffected plot geometry", async ({ page }) => {
    const errors: string[] = [];
    page.on("console", (m) => {
      if (m.type() === "error") errors.push(m.text());
    });
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve("../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    const frames: { name: string; scene: SceneDiagnostics }[] = [];
    try {
      const cold = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") resolve();
              if (s.phase === "error") reject(new Error(s.message));
            },
            renderer,
          );
          const stats = () => runtime.sceneStats() as SceneDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 4000; i++) {
              await new Promise(requestAnimationFrame);
              if (condition()) return;
            }
            throw new Error("City frame did not complete: " + JSON.stringify(runtime.assetStats()));
          };
          Reflect.set(window, "cityProbe", {
            runtime,
            host,
            game,
            toSceneSnapshot,
            stats,
            wait,
            next,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          runtime.camera(-1);
          const writes = stats().structures.uniformWrites;
          await wait(() => stats().structures.uniformWrites >= writes + 15);
          return stats();
        },
        { game: citySceneGame(), renderer: backend() },
      );
      frames.push({ name: "cold", scene: cold });
      // Read the compositor's presented frame. WebGL's transient back buffer
      // is allowed to clear between frames, so canvas.toDataURL is unsuitable.
      await page.screenshot({ path: path.join(output, `phase5-cities-${backend()}-cold.png`) });
      const warm = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(window, "cityProbe");
        const before = stats().structures.uniformWrites,
          start = runtime.assetStats().terrainStep;
        game.globalParameters.temperature = 8;
        game.globalParameters.oxygen = 14;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(
          () =>
            runtime.assetStats().terrainStep >= start + 120 &&
            stats().structures.uniformWrites >= before + 25,
        );
        return stats() as SceneDiagnostics;
      });
      frames.push({ name: "warm", scene: warm });
      await page.screenshot({ path: path.join(output, `phase5-cities-${backend()}-warm.png`) });
      const removed = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(window, "cityProbe");
        const tile = game.board.tiles.find(
          (t: ReturnType<typeof citySceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        )!;
        delete tile.occupiedBy;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(
          () =>
            stats().structures.plots === 3 &&
            stats().structures.pending === 0 &&
            stats().structures.retiringMeshes === 0 &&
            !stats().terrain.transitioning,
        );
        const writes = stats().structures.uniformWrites;
        await wait(() => stats().structures.uniformWrites >= writes + 10);
        return stats() as SceneDiagnostics;
      });
      frames.push({ name: "removed", scene: removed });
      await page.screenshot({ path: path.join(output, `phase5-cities-${backend()}-removed.png`) });
      const a = cold.structures,
        b = warm.structures,
        c = removed.structures;
      expect(a.plots).toBe(4);
      expect(c.plots).toBe(3);
      expect(b.meshIds).toEqual(a.meshIds);
      expect(b.bufferIds).toEqual(a.bufferIds);
      expect(b.groupImage).toBe(a.groupImage);
      expect(b.geometryBuilds).toBe(a.geometryBuilds);
      expect(b.textureWrites).toBe(a.textureWrites);
      expect(c.bufferIds).toEqual(a.bufferIds);
      expect(c.groupImage).toBe(a.groupImage);
      expect(c.geometryBuilds).toBe(a.geometryBuilds);
      expect(c.groups).toBeLessThan(a.groups);
      for (const { name } of frames) {
        const { data, info } = await sharp(
          path.join(output, `phase5-cities-${backend()}-${name}.png`),
        )
          .removeAlpha()
          .raw()
          .toBuffer({ resolveWithObject: true });
        let visible = 0;
        for (let i = 0; i < data.length; i += info.channels)
          if (data[i] + data[i + 1] + data[i + 2] > 60) visible++;
        expect(visible).toBeGreaterThan((info.width * info.height) / 2);
      }
      const cleared = await page.evaluate(async () => {
        const { runtime, next, wait } = Reflect.get(window, "cityProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(
          () =>
            runtime.assetStats().structures.uniforms === 0 &&
            runtime.assetStats().structures.groupImages === 0,
        );
        return runtime.assetStats();
      });
      expect(cleared.structures.plots).toBe(0);
      await writeFile(
        path.join(output, `phase5-cities-${backend()}-diagnostics.json`),
        JSON.stringify(frames, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const probe = Reflect.get(window, "cityProbe");
        if (probe) {
          probe.runtime.dispose();
          probe.host.remove();
          Reflect.deleteProperty(window, "cityProbe");
        }
      });
    }
  });

  test("animates real ecological bird skeletons and retires flock instances", async ({ page }) => {
    test.setTimeout(240_000);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const first = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_error: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const initial = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") resolve();
              if (s.phase === "error") reject(new Error(s.message));
            },
            renderer,
          );
          const stats = () => runtime.assetStats().fauna as FaunaDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let frame = 0; frame < 5000; frame++) {
              await new Promise(requestAnimationFrame);
              if (condition()) return;
            }
            throw new Error("Bird preparation did not finish: " + JSON.stringify(stats()));
          };
          Reflect.set(window, "faunaProbe", {
            runtime,
            host,
            game,
            next,
            toSceneSnapshot,
            stats,
            wait,
          });
          await initial;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          runtime.camera(-1);
          const step = runtime.assetStats().terrainStep;
          await wait(() => runtime.assetStats().terrainStep >= step + 45);
          return stats();
        },
        { game: faunaSceneGame(), renderer: backend() },
      );
      expect(first.ready).toBe(true);
      expect(first.flocks).toBe(2);
      expect(first.birds).toBeGreaterThanOrEqual(10);
      expect(first.birds).toBeLessThanOrEqual(12);
      expect(first.gpuSkins).toBe(first.birds);
      expect(first.clipDuration).toBeCloseTo(4.958333492, 5);
      expect(first.skinBufferBytes).toBeLessThan(4 * 1024 * 1024);
      expect(new Set(first.sample.map((b) => b.player)).size).toBe(first.birds);
      expect(new Set(first.sample.map((b) => b.skin)).size).toBe(first.birds);
      expect(
        first.sample.every((b) => b.joints?.length === 7 && b.visible && b.warmFrames >= 2),
      ).toBe(true);
      await page.screenshot({ path: path.join(output, `phase5-fauna-${backend()}-first.png`) });
      const motion = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "faunaProbe");
        const step = runtime.assetStats().terrainStep;
        await wait(() => runtime.assetStats().terrainStep >= step + 60);
        const moving = stats() as FaunaDiagnostics;
        runtime.setPaused(true);
        const paused = stats() as FaunaDiagnostics;
        for (let frame = 0; frame < 20; frame++) await new Promise(requestAnimationFrame);
        const frozen = stats() as FaunaDiagnostics;
        runtime.setPaused(false);
        return { moving, paused, frozen };
      });
      expect(motion.moving.sample[0].position).not.toEqual(first.sample[0].position);
      expect(motion.moving.sample[0].joints).not.toEqual(first.sample[0].joints);
      expect(motion.frozen).toEqual(motion.paused);
      expect(motion.moving.sample.map((b) => b.root)).toEqual(first.sample.map((b) => b.root));
      expect(motion.moving.skinBufferIds?.slice().sort()).toEqual(
        first.skinBufferIds?.slice().sort(),
      );
      await page.screenshot({ path: path.join(output, `phase5-fauna-${backend()}-moving.png`) });
      const removed = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(window, "faunaProbe");
        delete game.board.tiles.find(
          (t: ReturnType<typeof faunaSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        ).occupiedBy;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(
          () => stats().flocks === 1 && stats().ready && stats().gpuSkins === stats().birds,
        );
        return stats() as FaunaDiagnostics;
      });
      expect(removed.sample.map((b) => b.root)).toEqual(
        first.sample.filter((b) => b.tile[0] === 1).map((b) => b.root),
      );
      const lifetime = await page.evaluate(async () => {
        const { runtime, game, next, toSceneSnapshot, stats, wait } = Reflect.get(
          window,
          "faunaProbe",
        );
        const cleared: FaunaDiagnostics[] = [];
        const populated: FaunaDiagnostics[] = [];
        for (let cycle = 0; cycle < 3; cycle++) {
          const empty = next();
          runtime.setScene(null);
          await empty;
          await wait(
            () =>
              stats().birds === 0 &&
              stats().gpuSkins === 0 &&
              stats().graphs === 0 &&
              stats().clips === 0 &&
              stats().worldAssets === 0 &&
              stats().inverseBindposes === 0,
          );
          cleared.push(stats());
          if (cycle < 2) {
            const ready = next();
            runtime.setScene(
              toSceneSnapshot(
                {
                  ...game,
                  id: `fauna-${cycle}`,
                  globalParameters: { ...game.globalParameters, oxygen: cycle === 1 ? 0 : 14 },
                },
                null,
              ),
            );
            await ready;
            await wait(() => stats().gpuSkins === (cycle === 1 ? 0 : stats().birds));
            populated.push(stats());
          }
        }
        return { cleared, populated };
      });
      for (const value of lifetime.cleared) {
        expect(value.birds).toBe(0);
        expect(value.gpuSkins).toBe(0);
        expect(value.skinBufferIds?.slice().sort()).toEqual(first.skinBufferIds?.slice().sort());
      }
      expect(lifetime.populated[0].gpuSkins).toBe(lifetime.populated[0].birds);
      expect(lifetime.populated[1].gpuSkins).toBe(0);
      expect(lifetime.populated[1].sample.every((b) => !b.visible)).toBe(true);
      expect(lifetime.populated[1].birds).toBeGreaterThanOrEqual(5);
      await writeFile(
        path.join(output, `phase5-fauna-${backend()}-diagnostics.json`),
        JSON.stringify({ first, motion, removed, lifetime }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "faunaProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "faunaProbe");
        }
      });
    }
  });

  test("honors initial and live reduced motion without stopping scene updates", async ({
    page,
  }) => {
    // The measured software-GPU freeze/resume phases take four minutes before live placement.
    // Keep production startup/loading deadlines unchanged while allowing the whole scenario.
    test.setTimeout(360_000);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const game = faunaSceneGame();
    game.globalParameters.oceans = 9;
    game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === 1)!.occupiedBy =
      citySceneGame().board.tiles.find(
        (t) => t.coordinates.q === 0 && t.coordinates.r === 0,
      )!.occupiedBy;
    game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === -1)!.occupiedBy = {
      type: "ocean-tile",
      displayName: "Ocean",
      tags: [],
    };
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") resolve();
              if (s.phase === "error") reject(new Error(s.message));
            },
            renderer,
          );
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 4000; i++) {
              await new Promise(requestAnimationFrame);
              if (condition()) return;
            }
            throw new Error("Motion frame did not finish: " + JSON.stringify(runtime.assetStats()));
          };
          Reflect.set(window, "motionProbe", { runtime, host, wait, next, toSceneSnapshot, game });
          await first;
          const start = runtime.assetStats();
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          runtime.camera(-1);
          await wait(() => runtime.assetStats().bakeValid === 1);
          const writes = runtime.assetStats().terrainUniformWrites;
          // Camera changes can replace the temporary bake after the first valid frame.
          // Compare reduced-motion frames only after the current world is published.
          await wait(() => {
            const stats = runtime.assetStats();
            return (
              stats.terrainUniformWrites >= writes + 5 &&
              stats.bakeValid === 1 &&
              stats.bakeRevision === stats.terrainRevision &&
              stats.terrainRevision === stats.terrainRequestedRevision &&
              runtime.sceneStats().boardReady
            );
          });
          return { start, ready: runtime.assetStats() };
        },
        { game, renderer: backend() },
      );
      expect(initial.start.reducedMotion).toBe(true);
      expect(initial.start.motionStep).toBe(0);
      expect(initial.ready.motionStep).toBe(0);
      expect(initial.ready.fauna.gpuSkins).toBe(initial.ready.fauna.birds);
      const a = await page.screenshot({
        path: path.join(output, `phase5-motion-${backend()}-frozen-first.png`),
      });
      const frozen = await page.evaluate(async () => {
        const { runtime, wait } = Reflect.get(window, "motionProbe");
        const first = runtime.assetStats(),
          step = first.terrainStep;
        await wait(() => runtime.assetStats().terrainStep >= step + 90);
        return { first, last: runtime.assetStats() };
      });
      const b = await page.screenshot({
        path: path.join(output, `phase5-motion-${backend()}-frozen-later.png`),
      });
      expect(frozen.last.terrainStep).toBeGreaterThan(frozen.first.terrainStep);
      expect(frozen.last.motionStep).toBe(0);
      expect(frozen.last.fauna.sample).toEqual(frozen.first.fauna.sample);
      expect(frozen.first.bakeValid).toBe(1);
      expect(frozen.last.bakeValid).toBe(1);
      expect(frozen.last.bakeRevision).toBe(frozen.first.bakeRevision);
      expect(frozen.last.bakePublications).toBe(frozen.first.bakePublications);
      const [pixelsA, pixelsB] = await Promise.all(
        [a, b].map((buffer) => sharp(buffer).removeAlpha().raw().toBuffer()),
      );
      let difference = 0;
      for (let i = 0; i < pixelsA.length; i++) difference += Math.abs(pixelsA[i] - pixelsB[i]);
      expect(difference / pixelsA.length).toBeLessThan(0.1);
      await page.emulateMedia({ reducedMotion: "no-preference" });
      const moving = await page.evaluate(async () => {
        const { runtime, wait } = Reflect.get(window, "motionProbe");
        await wait(
          () => !runtime.assetStats().reducedMotion && runtime.assetStats().motionStep >= 90,
        );
        return runtime.assetStats();
      });
      expect(moving.fauna.sample[0].position).not.toEqual(frozen.last.fauna.sample[0].position);
      expect(moving.fauna.sample[0].joints).not.toEqual(frozen.last.fauna.sample[0].joints);
      expect(
        moving.fauna.sample.map((bird: FaunaDiagnostics["sample"][number]) => bird.root),
      ).toEqual(
        frozen.last.fauna.sample.map((bird: FaunaDiagnostics["sample"][number]) => bird.root),
      );
      await page.screenshot({ path: path.join(output, `phase5-motion-${backend()}-resumed.png`) });
      await page.emulateMedia({ reducedMotion: "reduce" });
      const refrozen = await page.evaluate(async () => {
        const { runtime, wait } = Reflect.get(window, "motionProbe");
        await wait(() => runtime.assetStats().reducedMotion);
        const first = runtime.assetStats();
        await wait(() => runtime.assetStats().terrainStep >= first.terrainStep + 45);
        return { first, last: runtime.assetStats() };
      });
      expect(refrozen.last.motionStep).toBe(refrozen.first.motionStep);
      expect(refrozen.last.fauna.sample).toEqual(refrozen.first.fauna.sample);
      const updated = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, wait } = Reflect.get(window, "motionProbe");
        const before = runtime.assetStats();
        game.board.tiles.find(
          (tile: ReturnType<typeof faunaSceneGame>["board"]["tiles"][number]) =>
            tile.coordinates.q === 0 && tile.coordinates.r === 2,
        ).occupiedBy = {
          type: "greenery-tile",
          tags: [],
          displayName: "Live greenery",
          visual: { seed: 71 },
        };
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(
          () =>
            runtime.assetStats().foliagePlants > before.foliagePlants &&
            !runtime.sceneStats().terrain.transitioning,
        );
        return {
          before,
          after: runtime.assetStats(),
          scene: runtime.sceneStats() as SceneDiagnostics,
        };
      });
      expect(updated.after.motionStep).toBe(updated.before.motionStep);
      expect(updated.after.terrainStep).toBeGreaterThan(updated.before.terrainStep);
      expect(updated.scene.tiles.find((tile) => tile.id === "0,2,-2")?.occupant).toBe(
        "greenery-tile",
      );
      await writeFile(
        path.join(output, `phase5-motion-${backend()}-diagnostics.json`),
        JSON.stringify(
          { initial, frozen, moving, refrozen, updated, difference: difference / pixelsA.length },
          null,
          2,
        ),
      );
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "motionProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "motionProbe");
        }
      });
    }
  });

  test("renders seeded world-tree relief and keeps surviving GPU bindings", async ({ page }) => {
    test.setTimeout(240_000);
    if (backend() === "webgpu") await trackTerrainBuffers(page);
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") resolve();
              if (s.phase === "error") reject(new Error(s.message));
            },
            renderer,
          );
          const stats = () => runtime.assetStats().worldTrees as WorldTreeDiagnostics;
          const wait = async (test: () => boolean) => {
            for (let frame = 0; frame < 6000; frame++) {
              await new Promise(requestAnimationFrame);
              if (test()) return;
            }
            throw new Error(
              "World-tree frame did not complete: " + JSON.stringify(runtime.assetStats()),
            );
          };
          Reflect.set(window, "worldTreeProbe", {
            runtime,
            host,
            game,
            next,
            toSceneSnapshot,
            stats,
            wait,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          runtime.camera(-1);
          if (runtime.assetStats().foliagePlants !== 0)
            throw new Error("World-tree tiles must not create ordinary forest vegetation");
          const writes = stats().uniformWrites;
          await wait(() => stats().uniformWrites >= writes + 10);
          return stats();
        },
        { game: worldTreeSceneGame(), renderer: backend() },
      );
      expect(initial.ready).toBe(true);
      expect(initial.trees).toBe(2);
      expect(initial.uniforms).toBe(2);
      expect(initial.geometryBuilds).toBe(1);
      expect(initial.groundMesh).not.toBeNull();
      expect(initial.sample.map((p) => p.seed)).toEqual([37, 999]);
      expect(initial.sample.every((p) => p.born < 0 && p.warmFrames === 2)).toBe(true);
      await page.screenshot({
        path: path.join(output, `phase5-worldtree-${backend()}-initial.png`),
      });
      const emerging = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(
          window,
          "worldTreeProbe",
        );
        game.board.tiles.find(
          (tile: ReturnType<typeof worldTreeSceneGame>["board"]["tiles"][number]) =>
            tile.coordinates.q === -1 && tile.coordinates.r === 0,
        ).occupiedBy = {
          type: "world-tree-tile",
          displayName: "Live world tree",
          tags: [],
          visual: { seed: 2027 },
        };
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(() => stats().trees === 3 && stats().ready);
        return stats() as WorldTreeDiagnostics;
      });
      const old = emerging.sample.filter((p) => p.coordinate[0] !== -1);
      expect(old.map((p) => [p.entity, p.buffer])).toEqual(
        initial.sample.map((p) => [p.entity, p.buffer]),
      );
      expect(emerging.mesh).toBe(initial.mesh);
      expect(emerging.groundMesh).toBe(initial.groundMesh);
      expect(emerging.geometryBuilds).toBe(initial.geometryBuilds);
      expect(emerging.sample.find((p) => p.coordinate[0] === -1)!.born).toBeGreaterThanOrEqual(0);
      await page.screenshot({
        path: path.join(output, `phase5-worldtree-${backend()}-emerging.png`),
      });
      const settled = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "worldTreeProbe");
        const birth = stats().sample.find(
          (p: WorldTreeDiagnostics["sample"][number]) => p.coordinate[0] === -1,
        ).born;
        await wait(() => runtime.assetStats().terrainStep >= Math.ceil((birth + 1.6) * 60));
        return stats();
      });
      await page.screenshot({
        path: path.join(output, `phase5-worldtree-${backend()}-settled.png`),
      });
      const cold = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(
          window,
          "worldTreeProbe",
        );
        const step = runtime.assetStats().terrainStep;
        game.globalParameters.temperature = -30;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(() => runtime.assetStats().terrainStep >= step + 120);
        return stats() as WorldTreeDiagnostics;
      });
      expect(cold.sample.map((p) => [p.entity, p.buffer])).toEqual(
        settled.sample.map((p: WorldTreeDiagnostics["sample"][number]) => [p.entity, p.buffer]),
      );
      await page.screenshot({ path: path.join(output, `phase5-worldtree-${backend()}-cold.png`) });
      const collapsed = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(
          window,
          "worldTreeProbe",
        );
        game.board.tiles.find(
          (tile: ReturnType<typeof worldTreeSceneGame>["board"]["tiles"][number]) =>
            tile.coordinates.q === -1 && tile.coordinates.r === 0,
        ).occupiedBy = { type: "nuclear-zone-tile", displayName: "Nuclear zone", tags: [] };
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(() =>
          stats().sample.some(
            (p: WorldTreeDiagnostics["sample"][number]) =>
              p.coordinate[0] === -1 && p.destroy !== null,
          ),
        );
        const destruction = stats();
        await wait(
          () => stats().trees === 2 && stats().uniforms === 2 && stats().retiredUniforms >= 1,
        );
        return { destruction, finished: stats() as WorldTreeDiagnostics };
      });
      expect(collapsed.destruction.trees).toBe(3);
      expect(collapsed.finished.sample.map((p) => [p.entity, p.buffer])).toEqual(
        initial.sample.map((p) => [p.entity, p.buffer]),
      );
      const reseeded = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(
          window,
          "worldTreeProbe",
        );
        game.board.tiles.find(
          (t: ReturnType<typeof worldTreeSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        ).occupiedBy.visual.seed = 38;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(() =>
          stats().sample.some((p: WorldTreeDiagnostics["sample"][number]) => p.seed === 38),
        );
        const first = stats().sample.find(
          (p: WorldTreeDiagnostics["sample"][number]) => p.seed === 38,
        );
        await wait(() => stats().ready);
        return { first, settled: stats() as WorldTreeDiagnostics };
      });
      expect(reseeded.first.warmFrames).toBe(0);
      expect(reseeded.first.buffer).not.toBe(initial.sample.find((p) => p.seed === 37)!.buffer);
      expect(reseeded.settled.sample.find((p) => p.seed === 999)!.buffer).toBe(
        initial.sample.find((p) => p.seed === 999)!.buffer,
      );
      await page.screenshot({
        path: path.join(output, `phase5-worldtree-${backend()}-reseeded.png`),
      });
      const cleared = await page.evaluate(async () => {
        const { runtime, next, wait, stats } = Reflect.get(window, "worldTreeProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(() => stats().trees === 0 && stats().uniforms === 0);
        return {
          stats: stats() as WorldTreeDiagnostics,
          buffers: Reflect.get(window, "worldTreeBuffers"),
        };
      });
      expect(cleared.stats.mesh).toBeNull();
      expect(cleared.stats.groundMesh).toBeNull();
      if (backend() === "webgpu")
        expect(cleared.buffers).toEqual({ created: 4, destroyed: 4, live: 0 });
      await writeFile(
        path.join(output, `phase5-worldtree-${backend()}-diagnostics.json`),
        JSON.stringify({ initial, emerging, settled, cold, collapsed, reseeded, cleared }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "worldTreeProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "worldTreeProbe");
        }
      });
    }
  });

  test("renders shared local emissions and settles snapshot dust without replay", async ({
    page,
  }) => {
    test.setTimeout(240_000);
    if (backend() === "webgpu") {
      await trackTerrainBuffers(page);
    }
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") {
                resolve();
              }
              if (s.phase === "error") {
                reject(new Error(s.message));
              }
            },
            renderer,
          );
          const stats = () => runtime.assetStats().particles as ParticleDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 10000; i++) {
              await new Promise(requestAnimationFrame);
              if (condition()) {
                return;
              }
            }
            throw new Error("Particle frame: " + JSON.stringify(runtime.assetStats()));
          };
          Reflect.set(window, "particleProbe", {
            runtime,
            host,
            game,
            toSceneSnapshot,
            stats,
            wait,
            next,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          await wait(() => stats().ready);
          runtime.camera(-1);
          const step = runtime.assetStats().motionStep;
          await wait(() => runtime.assetStats().motionStep >= step + 180);
          return stats();
        },
        { game: particleSceneGame(), renderer: backend() },
      );
      expect(initial.ready).toBe(true);
      expect(initial.pools).toBe(5);
      expect(initial.vents).toBe(1);
      expect(initial.sprites).toBeGreaterThan(124);
      expect(initial.uniforms).toBe(1);
      expect(initial.sample.some((p) => p.kind === "Placement")).toBe(false);
      expect(initial.imageMips).toBe(9);
      expect(initial.imageSize).toEqual([377, 329]);
      await page.screenshot({
        path: path.join(output, `phase5-particles-${backend()}-initial.png`),
      });
      await page.emulateMedia({ reducedMotion: "reduce" });
      const frozen = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "particleProbe");
        await wait(() => runtime.assetStats().reducedMotion);
        const first = stats() as ParticleDiagnostics;
        const step = runtime.sceneStats().step;
        await wait(() => runtime.sceneStats().step >= step + 30);
        return { first, after: stats() as ParticleDiagnostics };
      });
      expect(frozen.first.sample).toEqual(frozen.after.sample);
      expect(frozen.first.ventSample).toEqual(frozen.after.ventSample);
      expect(frozen.after.paletteBuffer).toBe(initial.paletteBuffer);
      await page.screenshot({
        path: path.join(output, `phase5-particles-${backend()}-frozen.png`),
      });
      const quiet = await page.evaluate(async () => {
        const { runtime, stats, wait, game, toSceneSnapshot, next } = Reflect.get(
          window,
          "particleProbe",
        );
        const factory = game.board.tiles.find(
          (t: ReturnType<typeof particleSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === -1,
        ).occupiedBy;
        game.board.tiles.find(
          (t: ReturnType<typeof particleSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 2 && t.coordinates.r === 0,
        ).occupiedBy = structuredClone(factory);
        const ready = next();
        runtime.setScene(toSceneSnapshot(game, null), "snapshot");
        await ready;
        await wait(() => stats().ready && stats().vents === 2);
        return stats() as ParticleDiagnostics;
      });
      expect(quiet.sample.some((p) => p.kind === "Placement")).toBe(false);
      expect(quiet.paletteBuffer).toBe(initial.paletteBuffer);
      await page.screenshot({
        path: path.join(output, `phase5-particles-${backend()}-snapshot.png`),
      });

      const reducedLive = await page.evaluate(async () => {
        const { runtime, stats, wait, game, toSceneSnapshot } = Reflect.get(
          window,
          "particleProbe",
        );
        game.board.tiles.find(
          (t: ReturnType<typeof particleSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 2 && t.coordinates.r === -1,
        ).reservedBy = "player";
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(
          () =>
            runtime
              .assetStats()
              .worksites.sample.some(
                (p: WorksiteDiagnostics["sample"][number]) =>
                  p.coordinate[0] === 2 && p.coordinate[1] === -1 && p.warmFrames === 2,
              ) && stats().ready,
        );
        return stats() as ParticleDiagnostics;
      });
      expect(reducedLive.sample.some((p) => p.kind === "Placement")).toBe(false);
      await page.emulateMedia({ reducedMotion: "no-preference" });
      const resumed = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "particleProbe");
        await wait(() => !runtime.assetStats().reducedMotion);
        const step = runtime.assetStats().motionStep;
        await wait(() => runtime.assetStats().motionStep >= step + 60);
        return stats() as ParticleDiagnostics;
      });
      expect(resumed.sample.some((p) => p.kind === "Placement")).toBe(false);
      expect(resumed.sample.map((p) => p.positions)).not.toEqual(
        quiet.sample.map((p) => p.positions),
      );
      expect(resumed.ventSample[0].sample).not.toEqual(quiet.ventSample[0].sample);
      const live = await page.evaluate(async () => {
        const { runtime, stats, wait, game, toSceneSnapshot } = Reflect.get(
          window,
          "particleProbe",
        );
        const target = game.board.tiles.find(
          (t: ReturnType<typeof particleSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 1 && t.coordinates.r === -1,
        );
        target.reservedBy = "player";
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(() =>
          stats().sample.some(
            (p: ParticleDiagnostics["sample"][number]) =>
              p.kind === "Placement" && p.warmFrames === 2,
          ),
        );
        return stats() as ParticleDiagnostics;
      });
      expect(live.sample.filter((p) => p.kind === "Placement")).toHaveLength(1);
      expect(live.paletteBuffer).toBe(initial.paletteBuffer);
      await page.screenshot({ path: path.join(output, `phase5-particles-${backend()}-dust.png`) });
      const expired = await page.evaluate(async () => {
        const { stats, wait } = Reflect.get(window, "particleProbe");
        await wait(
          () =>
            !stats().sample.some(
              (p: ParticleDiagnostics["sample"][number]) => p.kind === "Placement",
            ),
        );
        return stats() as ParticleDiagnostics;
      });
      expect(expired.pools).toBe(5);
      expect(expired.paletteBuffer).toBe(initial.paletteBuffer);
      const cleared = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "particleProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(() => stats().uniforms === 0);
        return {
          stats: stats() as ParticleDiagnostics,
          buffers: Reflect.get(window, "particleBuffers"),
        };
      });
      expect(cleared.stats.sprites).toBe(0);
      expect(cleared.stats.vents).toBe(0);
      expect(cleared.stats.pools).toBe(0);
      expect(cleared.stats.retiredUniforms).toBe(1);
      if (backend() === "webgpu") {
        expect(cleared.buffers).toEqual({ created: 1, destroyed: 1, live: 0 });
      }
      await writeFile(
        path.join(output, `phase5-particles-${backend()}-diagnostics.json`),
        JSON.stringify({ initial, frozen, quiet, resumed, live, expired, cleared }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "particleProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "particleProbe");
        }
      });
    }
  });

  test("collapses every outgoing actor group on the shared nuclear timeline", async ({ page }) => {
    // The software-GPU trace reaches the sixth collapse at nine minutes, before final retirement.
    // Allow the full six-family lifecycle; production startup keeps its own deadline.
    test.setTimeout(660_000);
    if (backend() === "webgpu") {
      await trackTerrainBuffers(page);
    }
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    const targets = [
      { q: 0, r: 0, family: "mining", parts: 31 },
      { q: -1, r: 0, family: "reservation", parts: 25 },
      { q: 0, r: -1, family: "mohole", parts: 5 },
      { q: -1, r: 1, family: "volcano", parts: 1 },
      { q: 1, r: -1, family: "worldtree", parts: 2 },
      { q: -1, r: -1, family: "foliage", parts: 0 },
    ];
    const records = [];
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {},
            failure: string | null = null;
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") {
                resolve();
              }
              if (s.phase === "error") {
                failure = s.message;
                reject(new Error(s.message));
              }
            },
            renderer,
          );
          const stats = () => runtime.sceneStats() as SceneDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 12000; i++) {
              await new Promise(requestAnimationFrame);
              if (failure) {
                throw new Error(failure);
              }
              if (condition()) {
                return;
              }
            }
            throw new Error("Actor collapse: " + JSON.stringify(runtime.assetStats()));
          };
          const same = (c: number[], q: number, r: number) => c[0] === q && c[1] === r;
          Reflect.set(window, "nuclearPartsProbe", {
            runtime,
            host,
            game,
            toSceneSnapshot,
            stats,
            wait,
            next,
            same,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          await ready;
          await wait(
            () =>
              stats().worksites.ready &&
              stats().specialSurfaces.ready &&
              stats().worldTrees.ready &&
              runtime.assetStats().foliagePlants > 100,
          );
          runtime.camera(-2);
          return { scene: stats(), assets: runtime.assetStats() };
        },
        { game: nuclearCollapseGame(), renderer: backend() },
      );
      const control = initial.scene.worksites.sample.find(
        (p) => p.coordinate[0] === 1 && p.coordinate[1] === 0,
      )!;
      expect(initial.scene.worksites.sites).toBe(4);
      expect(initial.scene.specialSurfaces.sites).toBe(2);
      expect(initial.scene.worldTrees.trees).toBe(1);
      const family = process.env.E2E_NUCLEAR_FAMILY;
      const selectedTargets = family
        ? targets.filter((target) => target.family === family)
        : targets;
      expect(selectedTargets.length).toBeGreaterThan(0);
      for (const target of selectedTargets) {
        const active = await page.evaluate(async (target) => {
          const { runtime, game, toSceneSnapshot, stats, wait, same } = Reflect.get(
            window,
            "nuclearPartsProbe",
          );
          const previous = stats() as SceneDiagnostics;
          const previousAssets = runtime.assetStats();
          const baseline = previous.vegetationPoses.filter((p) => same(p.tile, target.q, target.r));
          const tile = game.board.tiles.find(
            (t: ReturnType<typeof nuclearCollapseGame>["board"]["tiles"][number]) =>
              t.coordinates.q === target.q && t.coordinates.r === target.r,
          )!;
          tile.occupiedBy = { type: "nuclear-zone-tile", tags: [] };
          runtime.setScene(toSceneSnapshot(game, null));
          await wait(() =>
            stats().nuclearTimeline.events.some(
              (e: SceneDiagnostics["nuclearTimeline"]["events"][number]) =>
                same(e.coordinate, target.q, target.r) && typeof e.start === "number",
            ),
          );
          const started = stats() as SceneDiagnostics;
          const start = started.nuclearTimeline.events.find((e) =>
            same(e.coordinate, target.q, target.r),
          )!.start!;
          const source = () => {
            const scene = stats() as SceneDiagnostics;
            if (target.family === "mining" || target.family === "reservation") {
              return scene.worksites.sample.find((p) => same(p.coordinate, target.q, target.r));
            }
            if (target.family === "mohole" || target.family === "volcano") {
              return scene.specialSurfaces.sample.find((p) =>
                same(p.coordinate, target.q, target.r),
              );
            }
            return scene.worldTrees.sample.find((p) => same(p.coordinate, target.q, target.r));
          };
          await wait(() => {
            if (target.family === "foliage") {
              return stats().vegetationPoses.some(
                (p: SceneDiagnostics["vegetationPoses"][number]) => {
                  const old = baseline.find((b) => b.entity === p.entity);
                  return old && p.position.some((v, i) => Math.abs(v - old.position[i]) > 0.005);
                },
              );
            }
            const actor = source();
            if (!actor) {
              return false;
            }
            if (target.family === "worldtree") {
              const tree = actor as WorldTreeDiagnostics["sample"][number];
              return tree.collapse[2] > 0.012 && tree.groundCollapse[2] > 0.012;
            }
            const grouped = actor as
              | WorksiteDiagnostics["sample"][number]
              | SpecialSurfaceDiagnostics["sample"][number];
            const motions = target.family === "volcano" ? [grouped.collapse[0]] : grouped.collapse;
            return (
              motions.some((m: number[]) => m[2] > 0.012) &&
              motions.every((m: number[]) => m[2] > 0)
            );
          });
          runtime.setPaused(true);
          const stage = stats() as SceneDiagnostics;
          const actor = source();
          return {
            previous,
            previousAssets,
            baseline,
            started,
            startedAssets: runtime.assetStats(),
            start,
            stage,
            actor,
          };
        }, target);
        expect(
          active.started.craters.sample.find(
            (p) => p.coordinate[0] === target.q && p.coordinate[1] === target.r,
          )?.coverage,
        ).toBe(0);
        expect(active.stage.step).toBeGreaterThanOrEqual(active.start + 60);
        if (target.family === "mining" || target.family === "reservation") {
          const actor = active.actor as WorksiteDiagnostics["sample"][number];
          expect(actor.collapseParts).toBe(target.parts);
          expect(actor.collapse).toHaveLength(target.parts);
          expect(new Set(actor.collapse.map((m) => m.slice(0, 2).join(","))).size).toBe(
            target.parts,
          );
        } else if (target.family === "mohole") {
          const actor = active.actor as SpecialSurfaceDiagnostics["sample"][number];
          expect(actor.collapse).toHaveLength(5);
          expect(new Set(actor.collapse.map((m) => m.slice(0, 2).join(","))).size).toBe(5);
        } else if (target.family === "worldtree") {
          const actor = active.actor as WorldTreeDiagnostics["sample"][number];
          expect(actor.collapse).not.toEqual(actor.groundCollapse);
          expect(actor.depths[0]).not.toBe(actor.depths[1]);
        } else if (target.family === "foliage") {
          expect(active.baseline.length).toBeGreaterThan(0);
          if (family) {
            expect(active.startedAssets.shadeShapes).toBeLessThan(
              active.previousAssets.shadeShapes,
            );
          } else {
            expect(active.startedAssets.shadeShapes).toBe(0);
          }
          const moved = active.stage.vegetationPoses.filter((p) =>
            active.baseline.some((b) => b.entity === p.entity),
          );
          expect(moved).toHaveLength(active.baseline.length);
          expect(
            moved.some((p) => {
              const old = active.baseline.find((b) => b.entity === p.entity)!;
              return p.rotation.some((v, i) => Math.abs(v - old.rotation[i]) > 0.001);
            }),
          ).toBe(true);
          expect(moved.map((p) => p.scale)).toEqual(active.baseline.map((p) => p.scale));
        }
        await page.screenshot({
          path: path.join(output, `phase5-nuclear-${backend()}-${target.family}-collapse.png`),
        });
        const after = await page.evaluate(
          async (target) => {
            const { runtime, stats, wait, same } = Reflect.get(window, "nuclearPartsProbe");
            runtime.setPaused(false);
            await wait(() => {
              const scene = stats() as SceneDiagnostics;
              let gone = false;
              if (target.family === "mining" || target.family === "reservation") {
                gone = !scene.worksites.sample.some((p) => same(p.coordinate, target.q, target.r));
              } else if (target.family === "mohole" || target.family === "volcano") {
                gone = !scene.specialSurfaces.sample.some((p) =>
                  same(p.coordinate, target.q, target.r),
                );
              } else if (target.family === "worldtree") {
                gone = !scene.worldTrees.sample.some((p) => same(p.coordinate, target.q, target.r));
              } else {
                // A filtered run retains other families, including their vegetation.
                gone = target.focused
                  ? runtime.assetStats().foliagePlants < target.plantsBefore &&
                    target.plantIds.every(
                      (id) => !scene.vegetationPoses.some((p) => p.entity === id),
                    )
                  : runtime.assetStats().foliagePlants === 0;
              }
              return gone && scene.craters.ready;
            });
            return { scene: stats() as SceneDiagnostics, assets: runtime.assetStats() };
          },
          {
            ...target,
            focused: !!family,
            plantIds: active.baseline.map((p) => p.entity),
            plantsBefore: active.previousAssets.foliagePlants,
          },
        );
        const survivor = after.scene.worksites.sample.find(
          (p) => p.coordinate[0] === 1 && p.coordinate[1] === 0,
        )!;
        expect([survivor.entity, survivor.mesh, survivor.buffer]).toEqual([
          control.entity,
          control.mesh,
          control.buffer,
        ]);
        expect(after.scene.step).toBeGreaterThanOrEqual(active.start + 128);
        records.push({ target, ...active, after });
        await writeFile(
          path.join(output, `phase5-nuclear-${backend()}-parts-progress.json`),
          JSON.stringify({ initial, records }, null, 2),
        );
      }
      await page.screenshot({
        path: path.join(output, `phase5-nuclear-${backend()}-all-actors-settled.png`),
      });
      const cleared = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "nuclearPartsProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(
          () =>
            stats().craters.uniforms === 0 &&
            stats().craters.meshBytes === 0 &&
            stats().worksites.uniforms === 0 &&
            stats().specialSurfaces.uniforms === 0 &&
            stats().worldTrees.uniforms === 0 &&
            runtime.assetStats().foliagePlants === 0,
        );
        return {
          scene: stats() as SceneDiagnostics,
          assets: runtime.assetStats(),
          craterBuffers: Reflect.get(window, "craterBuffers"),
          worksiteBuffers: Reflect.get(window, "worksiteBuffers"),
          specialBuffers: Reflect.get(window, "specialBuffers"),
        };
      });
      if (backend() === "webgpu") {
        for (const b of [cleared.craterBuffers, cleared.worksiteBuffers, cleared.specialBuffers]) {
          expect(b.live).toBe(0);
          expect(b.created).toBe(b.destroyed);
        }
      }
      await writeFile(
        path.join(output, `phase5-nuclear-${backend()}-parts-diagnostics.json`),
        JSON.stringify({ initial, records, cleared }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "nuclearPartsProbe");
        if (p) {
          p.runtime.setPaused(false);
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "nuclearPartsProbe");
        }
      });
    }
  });

  test("renders the source nuclear cloud and wave through expiry on both backends", async ({
    page,
  }) => {
    test.setTimeout(300_000);
    if (backend() === "webgpu") {
      await trackTerrainBuffers(page);
    }
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {},
            failure: string | null = null;
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const sounds: { action: string; id: string; gameId: string; sound: string }[] = [];
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") {
                resolve();
              }
              if (s.phase === "error") {
                failure = s.message;
                reject(new Error(s.message));
              }
            },
            renderer,
            () => {},
            (event: SceneSound) => sounds.push(event),
          );
          const stats = () => runtime.sceneStats() as SceneDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 12000; i++) {
              if (failure) {
                throw new Error(failure);
              }
              if (condition()) {
                return;
              }
              await new Promise(requestAnimationFrame);
            }
            throw new Error("Blast deadline: " + JSON.stringify(stats()));
          };
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          await ready;
          runtime.camera(-0.3);
          Reflect.set(window, "blastProbe", {
            runtime,
            sounds,
            host,
            game,
            stats,
            wait,
            next,
            toSceneSnapshot,
          });
          return stats();
        },
        { game: sceneGame(), renderer: backend() },
      );
      expect(initial.blast.sites).toBe(0);
      expect(initial.blast.allocations).toBe(0);
      const flight = await page.evaluate(async () => {
        const { runtime, game, stats, wait, toSceneSnapshot } = Reflect.get(window, "blastProbe");
        game.board.tiles.find(
          (t: ReturnType<typeof sceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        )!.occupiedBy = { type: "nuclear-zone-tile", tags: [] };
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(
          () => stats().blast.sample[0]?.missileVisible && stats().blast.sample[0].age < -0.1,
        );
        runtime.setPaused(true);
        return stats() as SceneDiagnostics;
      });
      expect(await page.evaluate(() => Reflect.get(window, "blastProbe").sounds)).toEqual([]);
      expect(flight.blast.ready).toBe(true);
      expect(flight.blast.sample[0].warmFrames).toBe(2);
      expect(flight.blast.target?.extent).toEqual([640, 360]);
      expect(flight.blast.target?.encoded).toBe(true);
      expect(flight.blast.uniformAllocations).toBe(1);
      expect(flight.blast.uniformBytes).toBeLessThan(300_000);
      expect(flight.craters.sample[0].coverage).toBe(0);
      expect(flight.craters.sample[0].fragments).toBeGreaterThan(80);
      expect(flight.craters.sample[0].fragments).toBeLessThanOrEqual(144);
      expect(flight.nuclearDebris.length).toBe(4);
      expect(flight.nuclearDebris.every((p) => (p.tag & 0x80000000) !== 0)).toBe(true);
      const flightImage = await page.screenshot({
        path: path.join(output, `phase5-blast-${backend()}-flight.png`),
      });
      // Hidden fragments must not leave depth-only holes in the untouched tile.
      const flightPixels = await sharp(flightImage)
        .extract({ left: 600, top: 320, width: 80, height: 80 })
        .removeAlpha()
        .raw()
        .toBuffer();
      let darkPixels = 0;
      for (let i = 0; i < flightPixels.length; i += 3) {
        // Source lighting shades the brown grid border down to red27..29.
        // Black depth holes remain below20 (all226 pixels in the retained
        // original broken-prepass capture still fail this cutoff).
        if (Math.max(flightPixels[i], flightPixels[i + 1], flightPixels[i + 2]) < 20) {
          darkPixels++;
        }
      }
      expect(darkPixels).toBeLessThanOrEqual(2);
      const stages = [];
      for (const [name, age] of [
        ["flash", 0.08],
        ["cloud", 1.2],
        ["dissipating", 4.1],
      ] as const) {
        const captured = await page.evaluate(async (age) => {
          const { runtime, stats, wait } = Reflect.get(window, "blastProbe");
          runtime.setPaused(false);
          await wait(() => stats().blast.sample[0]?.age >= age);
          runtime.setPaused(true);
          return stats() as SceneDiagnostics;
        }, age);
        expect(captured.blast.buffer).toBe(flight.blast.buffer);
        expect(captured.blast.allocations).toBe(flight.blast.allocations);
        expect(captured.blast.sample[0].missileVisible).toBe(false);
        expect(captured.craters.sample[0].coverage).toBe(1);
        expect(captured.blast.encodedFrames).toBeGreaterThan(flight.blast.encodedFrames);
        expect(captured.blast.draws).toBe(1);
        expect(captured.nuclearDebris.map((p) => p.entity)).toEqual(
          flight.nuclearDebris.map((p) => p.entity),
        );
        expect(captured.nuclearDebris.map((p) => p.scale)).toEqual(
          flight.nuclearDebris.map((p) => p.scale),
        );
        expect(captured.nuclearDebris.some((p) => (p.tag & 0x80000000) === 0)).toBe(true);
        expect(captured.nuclearDebris.map((p) => p.position)).not.toEqual(
          flight.nuclearDebris.map((p) => p.position),
        );
        if (age > 4) {
          expect(captured.blast.sample[0].waveTag).toBe(0x80000000);
        }
        await page.screenshot({ path: path.join(output, `phase5-blast-${backend()}-${name}.png`) });
        const frozen = await page.evaluate(
          () => (Reflect.get(window, "blastProbe").stats() as SceneDiagnostics).blast,
        );
        expect(frozen.sample).toEqual(captured.blast.sample);
        stages.push({ name, ...captured.blast });
      }
      const expired = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "blastProbe");
        runtime.setPaused(false);
        await wait(
          () =>
            stats().blast.sites === 0 &&
            stats().blast.retired === 0 &&
            stats().blast.uniformAllocations === stats().blast.uniformRetirements,
        );
        return stats() as SceneDiagnostics;
      });
      expect(expired.nuclearTimeline.events).toEqual([]);
      expect(expired.blast.target).toBeNull();
      expect(expired.blast.bytes).toBe(0);
      expect(expired.blast.shake).toBeNull();
      expect(expired.craters.sites).toBe(1);
      expect(expired.nuclearDebris.every((p) => (p.tag & 0x80000000) === 0)).toBe(true);
      for (const pose of expired.nuclearDebris) {
        const source = expired.craters.sample[0].fragmentSample.find(
          (f) => f.entity === pose.entity,
        )!;
        expect(pose.position.every((v, i) => Math.abs(v - source.settledPosition[i]) < 1e-6)).toBe(
          true,
        );
      }
      await page.screenshot({ path: path.join(output, `phase5-blast-${backend()}-expired.png`) });
      const sounds = await page.evaluate(() => Reflect.get(window, "blastProbe").sounds);
      expect(sounds.map((event: { action: string }) => event.action)).toEqual(["start", "stop"]);
      expect(sounds[0].id).toBe(sounds[1].id);
      expect(sounds[0].sound).toBe("nuclear-blast");
      const cleared = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "blastProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(() => (Reflect.get(window, "blastBuffers")?.live ?? 0) === 0);
        return {
          scene: stats(),
          assets: runtime.assetStats(),
          buffers: Reflect.get(window, "blastBuffers"),
        };
      });
      if (backend() === "webgpu") {
        expect(cleared.buffers).toEqual({ created: 1, destroyed: 1, live: 0 });
      }
      await writeFile(
        path.join(output, `phase5-blast-${backend()}-diagnostics.json`),
        JSON.stringify({ initial, flight, stages, expired, sounds, cleared }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page
        .evaluate(() => {
          const p = Reflect.get(window, "blastProbe");
          if (p) {
            p.runtime.dispose();
            p.host.remove();
            Reflect.deleteProperty(window, "blastProbe");
          }
        })
        .catch(() => {});
    }
  });

  for (const [name, profile] of [
    ["desktop", { viewport: { width: 1280, height: 720 } }],
    ["iphone", devices["iPhone 14 landscape"]],
    ["android", devices["Pixel 7 landscape"]],
  ] as const) {
    test(`captures oblique nuclear cloud and settled fragments on ${name}`, async ({
      browser,
      baseURL,
    }) => {
      test.setTimeout(240_000);
      const context = await referenceContext(browser, { ...profile, baseURL });
      const page = await context.newPage();
      const errors: string[] = [];
      page.on("pageerror", (e) => errors.push(e.message));
      const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
      await mkdir(output, { recursive: true });
      try {
        await page.goto("/tests/fixtures/engine.html");
        const game = sceneGame("nuclear-device-capture");
        // This is an orbit/effects scenario, not an active placement prompt.
        // A prompt would auto-frame the tile before the production orbit gesture.
        delete game.currentPlayer.pendingTileSelection;
        await page.evaluate(
          async ({ game, renderer }) => {
            const rp = "/src/engine/runtime.ts",
              sp = "/src/engine/scene.ts";
            const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
              import(/* @vite-ignore */ rp),
              import(/* @vite-ignore */ sp),
            ]);
            const host = document.createElement("div");
            host.style.cssText = "position:fixed;inset:0";
            document.body.append(host);
            const hud = document.createElement("span");
            hud.textContent = "Public game state";
            hud.style.cssText =
              "position:fixed;left:12px;top:12px;color:white;background:#111;padding:8px;font:14px sans-serif";
            document.body.append(hud);
            let resolve = () => {},
              reject = (_e: Error) => {},
              failure: string | null = null;
            const next = () =>
              new Promise<void>((yes, no) => {
                resolve = yes;
                reject = no;
              });
            const first = next();
            const runtime = startEngine(
              host,
              (s: { phase: string; message: string }) => {
                if (s.phase === "ready") {
                  resolve();
                }
                if (s.phase === "error") {
                  failure = s.message;
                  reject(new Error(s.message));
                }
              },
              renderer,
            );
            const stats = () => runtime.sceneStats() as SceneDiagnostics;
            const wait = async (condition: () => boolean) => {
              for (let i = 0; i < 12000; i++) {
                if (failure) {
                  throw new Error(failure);
                }
                if (condition()) {
                  return;
                }
                await new Promise(requestAnimationFrame);
              }
              throw new Error("Nuclear capture deadline: " + JSON.stringify(stats()));
            };
            Reflect.set(window, "nuclearViewProbe", {
              runtime,
              host,
              hud,
              game,
              toSceneSnapshot,
              stats,
              wait,
              next,
            });
            await first;
            const ready = next();
            runtime.setScene(toSceneSnapshot(game, null), "snapshot");
            await ready;
          },
          { game, renderer: backend() },
        );
        // Exercise the production pointer/orbit path before the public live event.
        const bounds = await page.locator("canvas").boundingBox();
        const x = bounds!.x + bounds!.width / 2,
          y = bounds!.y + bounds!.height * 0.35;
        await page.mouse.move(x, y);
        await page.mouse.down();
        await page.mouse.move(x + 12, y + 100, { steps: 8 });
        await page.mouse.up();
        const hud = await page.getByText("Public game state", { exact: true }).boundingBox();
        const cloud = await page.evaluate(async () => {
          const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(
            window,
            "nuclearViewProbe",
          );
          game.board.tiles.find(
            (t: ReturnType<typeof sceneGame>["board"]["tiles"][number]) =>
              t.coordinates.q === 0 && t.coordinates.r === 0,
          )!.occupiedBy = { type: "nuclear-zone-tile", tags: [] };
          runtime.setScene(toSceneSnapshot(game, null));
          await wait(() => stats().blast.sample[0]?.age >= 1.4);
          runtime.setPaused(true);
          return stats() as SceneDiagnostics;
        });
        expect(cloud.camera.pitch).toBeGreaterThan(0.5);
        const target = cloud.tiles.find((tile) => tile.id === "0,0,0")!.point!;
        const viewport = page.viewportSize()!;
        expect(target[0]).toBeGreaterThanOrEqual(0);
        expect(target[0]).toBeLessThan(viewport.width);
        expect(target[1]).toBeGreaterThanOrEqual(0);
        expect(target[1]).toBeLessThan(viewport.height);
        expect(cloud.blast.draws).toBe(1);
        expect(cloud.blast.target?.encoded).toBe(true);
        expect(cloud.blast.target!.extent[0]).toBeLessThanOrEqual(960);
        expect(cloud.nuclearDebris.length).toBe(4);
        expect(cloud.nuclearDebris.every((p) => (p.tag & 0x80000000) === 0)).toBe(true);
        expect(await page.getByText("Public game state", { exact: true }).boundingBox()).toEqual(
          hud,
        );
        await page.screenshot({
          path: path.join(output, `phase5-nuclear-${backend()}-${name}-oblique.png`),
        });
        const settled = await page.evaluate(async () => {
          const { runtime, stats, wait } = Reflect.get(window, "nuclearViewProbe");
          runtime.setPaused(false);
          await wait(
            () =>
              stats().nuclearTimeline.events.length === 0 &&
              stats().blast.sites === 0 &&
              stats().blast.retired === 0,
          );
          runtime.setPaused(true);
          return stats() as SceneDiagnostics;
        });
        expect(settled.craters.sites).toBe(1);
        expect(settled.nuclearDebris.map((p) => p.entity)).toEqual(
          cloud.nuclearDebris.map((p) => p.entity),
        );
        expect(settled.blast.target).toBeNull();
        expect(settled.blast.shake).toBeNull();
        await page.screenshot({
          path: path.join(output, `phase5-nuclear-${backend()}-${name}-oblique-settled.png`),
        });
        await writeFile(
          path.join(output, `phase5-nuclear-${backend()}-${name}-view.json`),
          JSON.stringify({ cloud, settled }, null, 2),
        );
        const clear = await page.evaluate(async () => {
          const { runtime, stats, wait, next } = Reflect.get(window, "nuclearViewProbe");
          runtime.setPaused(false);
          const ready = next();
          runtime.setScene(null);
          await ready;
          await wait(
            () =>
              stats().craters.uniforms === 0 &&
              stats().craters.meshBytes === 0 &&
              stats().blast.retired === 0,
          );
          return stats() as SceneDiagnostics;
        });
        expect(clear.nuclearDebris).toEqual([]);
        expect(errors).toEqual([]);
      } finally {
        await page
          .evaluate(() => {
            const p = Reflect.get(window, "nuclearViewProbe");
            if (p) {
              p.runtime.dispose();
              p.host.remove();
              p.hud.remove();
              Reflect.deleteProperty(window, "nuclearViewProbe");
            }
          })
          .catch(() => {});
        await context.close();
      }
    });
  }

  test("cancels nuclear preparation and retains one compositor across resize and overlap", async ({
    page,
  }) => {
    test.setTimeout(300_000);
    if (backend() === "webgpu") {
      await trackTerrainBuffers(page);
    }
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {},
            failure: string | null = null;
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") {
                resolve();
              }
              if (s.phase === "error") {
                failure = s.message;
                reject(new Error(s.message));
              }
            },
            renderer,
          );
          const stats = () => runtime.sceneStats() as SceneDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 16000; i++) {
              if (failure) {
                throw new Error(failure);
              }
              if (condition()) {
                return;
              }
              await new Promise(requestAnimationFrame);
            }
            throw new Error("Nuclear lifetime deadline: " + JSON.stringify(stats()));
          };
          const replace = (q: number, nuclear = true) => {
            const tile = game.board.tiles.find(
              (t: ReturnType<typeof sceneGame>["board"]["tiles"][number]) =>
                t.coordinates.q === q && t.coordinates.r === 0,
            )!;
            if (nuclear) {
              tile.occupiedBy = { type: "nuclear-zone-tile", tags: [] };
            } else {
              delete tile.occupiedBy;
            }
            runtime.setScene(toSceneSnapshot(game, null));
          };
          Reflect.set(window, "nuclearLifetimeProbe", {
            runtime,
            host,
            game,
            toSceneSnapshot,
            stats,
            wait,
            next,
            replace,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          await ready;
          runtime.camera(-0.3);
        },
        { game: sceneGame("nuclear-lifetime"), renderer: backend() },
      );
      const canceled = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait, next, replace } = Reflect.get(
          window,
          "nuclearLifetimeProbe",
        );
        replace(0);
        await wait(() => stats().craters.building);
        const preparing = stats() as SceneDiagnostics;
        delete game.board.tiles.find(
          (t: ReturnType<typeof sceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        )!.occupiedBy;
        const ready = next();
        runtime.setScene(toSceneSnapshot(game, null), "snapshot");
        await ready;
        await wait(
          () =>
            stats().craters.uniforms === 0 &&
            stats().craters.meshBytes === 0 &&
            stats().blast.sites === 0 &&
            stats().blast.retired === 0 &&
            stats().blast.uniformAllocations === stats().blast.uniformRetirements,
        );
        return { preparing, after: stats() as SceneDiagnostics };
      });
      expect(canceled.preparing.craters.building).toBe(true);
      expect(canceled.after.craters.geometryBuilds).toBe(0);
      expect(canceled.after.nuclearTimeline.events).toEqual([]);
      expect(canceled.after.nuclearDebris).toEqual([]);
      const first = await page.evaluate(async () => {
        const { runtime, stats, wait, replace } = Reflect.get(window, "nuclearLifetimeProbe");
        replace(0);
        await wait(() => stats().blast.sample[0]?.age >= 0.2);
        runtime.setPaused(true);
        return stats() as SceneDiagnostics;
      });
      await page.setViewportSize({ width: 1600, height: 900 });
      const resized = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "nuclearLifetimeProbe");
        runtime.setPaused(false);
        await wait(
          () =>
            stats().blast.target?.extent[0] === 800 &&
            stats().blast.target.encoded &&
            stats().blast.retired === 0,
        );
        runtime.setPaused(true);
        return stats() as SceneDiagnostics;
      });
      expect(resized.blast.target?.extent).toEqual([800, 450]);
      expect(resized.blast.buffer).toBe(first.blast.buffer);
      expect(resized.blast.uniformAllocations).toBe(first.blast.uniformAllocations);
      expect(resized.craters.sample[0].entity).toBe(first.craters.sample[0].entity);
      expect(resized.craters.sample[0].buffer).toBe(first.craters.sample[0].buffer);
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-resized.png`) });
      const overlap = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait, replace } = Reflect.get(
          window,
          "nuclearLifetimeProbe",
        );
        // Deliver both new events together so GPU preparation cannot age one
        // effect out before the other's source field is ready.
        replace(0, false);
        for (const q of [0, 1]) {
          game.board.tiles.find(
            (t: ReturnType<typeof sceneGame>["board"]["tiles"][number]) =>
              t.coordinates.q === q && t.coordinates.r === 0,
          )!.occupiedBy = { type: "nuclear-zone-tile", tags: [] };
        }
        runtime.setScene(toSceneSnapshot(game, null));
        runtime.setPaused(false);
        await wait(
          () =>
            stats().blast.sites === 2 &&
            stats().blast.draws === 2 &&
            stats().blast.sample.every(
              (s: SceneDiagnostics["blast"]["sample"][number]) =>
                s.age !== null && s.age >= 0.2 && s.warmFrames >= 2,
            ),
        );
        runtime.setPaused(true);
        return stats() as SceneDiagnostics;
      });
      expect(overlap.blast.buffer).toBe(first.blast.buffer);
      expect(overlap.blast.uniformAllocations).toBe(first.blast.uniformAllocations);
      expect(overlap.blast.allocations).toBe(resized.blast.allocations);
      expect(overlap.craters.sites).toBe(2);
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-overlap.png`) });
      const quiet = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait, next } = Reflect.get(
          window,
          "nuclearLifetimeProbe",
        );
        const ready = next();
        runtime.setScene(toSceneSnapshot(game, null), "snapshot");
        runtime.setPaused(false);
        await ready;
        await wait(
          () =>
            stats().blast.sites === 0 &&
            stats().blast.retired === 0 &&
            stats().blast.uniformAllocations === stats().blast.uniformRetirements &&
            stats().craters.ready,
        );
        runtime.setPaused(true);
        return stats() as SceneDiagnostics;
      });
      expect(quiet.nuclearTimeline.events).toEqual([]);
      expect(quiet.craters.sample.map((s) => s.entity)).toEqual(
        overlap.craters.sample.map((s) => s.entity),
      );
      expect(quiet.nuclearDebris.every((p) => (p.tag & 0x80000000) === 0)).toBe(true);
      expect(quiet.blast.bytes).toBe(0);
      await page.screenshot({
        path: path.join(output, `phase5-nuclear-${backend()}-overlap-quiet.png`),
      });
      const clear = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "nuclearLifetimeProbe");
        runtime.setPaused(false);
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(
          () =>
            stats().craters.uniforms === 0 &&
            stats().craters.meshBytes === 0 &&
            stats().blast.retired === 0 &&
            (Reflect.get(window, "blastBuffers")?.live ?? 0) === 0 &&
            (Reflect.get(window, "craterBuffers")?.live ?? 0) === 0,
        );
        return {
          scene: stats() as SceneDiagnostics,
          assets: runtime.assetStats(),
          blastBuffers: Reflect.get(window, "blastBuffers"),
          craterBuffers: Reflect.get(window, "craterBuffers"),
        };
      });
      expect(clear.scene.nuclearDebris).toEqual([]);
      if (backend() === "webgpu") {
        expect(clear.blastBuffers.created).toBe(clear.blastBuffers.destroyed);
        expect(clear.craterBuffers.created).toBe(clear.craterBuffers.destroyed);
      }
      await writeFile(
        path.join(output, `phase5-nuclear-${backend()}-lifetime.json`),
        JSON.stringify({ canceled, first, resized, overlap, quiet, clear }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page
        .evaluate(() => {
          const p = Reflect.get(window, "nuclearLifetimeProbe");
          if (p) {
            p.runtime.dispose();
            p.host.remove();
            Reflect.deleteProperty(window, "nuclearLifetimeProbe");
          }
        })
        .catch(() => {});
    }
  });

  test("prepares a complete quiet crater board within the bounded working set", async ({
    page,
  }) => {
    test.setTimeout(180_000);
    if (backend() === "webgpu") {
      await trackTerrainBuffers(page);
    }
    const game = sceneGame("nuclear-complete-board");
    delete game.currentPlayer!.pendingTileSelection;
    for (const tile of game.board.tiles) {
      tile.occupiedBy = { type: "nuclear-zone-tile", tags: [] };
    }
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const loaded = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {},
            failure: string | null = null;
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") {
                resolve();
              }
              if (s.phase === "error") {
                failure = s.message;
                reject(new Error(s.message));
              }
            },
            renderer,
          );
          const stats = () => runtime.sceneStats() as SceneDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 12000; i++) {
              if (failure) {
                throw new Error(failure);
              }
              if (condition()) {
                return;
              }
              await new Promise(requestAnimationFrame);
            }
            throw new Error("Full crater board deadline");
          };
          Reflect.set(window, "nuclearBoundProbe", { runtime, host, stats, wait, next });
          await first;
          const ready = next();
          const started = performance.now();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          let lastProgress = {};
          const progress = setInterval(() => {
            const s = stats();
            lastProgress = {
              elapsed: performance.now() - started,
              craters: { ...s.craters, sample: undefined },
            };
          }, 1000);
          try {
            await ready;
            await wait(() => stats().craters.ready && stats().craters.sites === 61);
          } catch (error) {
            throw new Error(`${String(error)}; last progress: ${JSON.stringify(lastProgress)}`);
          } finally {
            clearInterval(progress);
          }
          runtime.setPaused(true);
          return {
            scene: stats(),
            elapsed: performance.now() - started,
            assets: runtime.assetStats(),
          };
        },
        { game, renderer: backend() },
      );
      expect(loaded.scene.craters.sites).toBe(61);
      expect(loaded.scene.craters.geometryBuilds).toBe(61);
      expect(loaded.scene.craters.uniforms).toBe(62);
      expect(loaded.scene.craters.meshBytes).toBeLessThanOrEqual(64 * 1024 * 1024);
      expect(
        loaded.scene.craters.sample.every(
          (s) => s.coverage === 1 && s.warmFrames === 2 && s.fragments <= 144,
        ),
      ).toBe(true);
      expect(loaded.scene.blast.allocations).toBe(0);
      expect(loaded.scene.nuclearTimeline.events).toEqual([]);
      await page.screenshot({
        path: path.join(output, `phase5-nuclear-${backend()}-complete-board.png`),
      });
      const clear = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "nuclearBoundProbe");
        runtime.setPaused(false);
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(
          () =>
            stats().craters.uniforms === 0 &&
            stats().craters.meshBytes === 0 &&
            (Reflect.get(window, "craterBuffers")?.live ?? 0) === 0,
        );
        return {
          scene: stats(),
          assets: runtime.assetStats(),
          buffers: Reflect.get(window, "craterBuffers"),
        };
      });
      expect(clear.scene.nuclearDebris).toEqual([]);
      if (backend() === "webgpu") {
        expect(clear.buffers).toEqual({ created: 62, destroyed: 62, live: 0 });
      }
      await writeFile(
        path.join(output, `phase5-nuclear-${backend()}-complete-board.json`),
        JSON.stringify({ loaded, clear }, null, 2),
      );
    } finally {
      await page
        .evaluate(() => {
          const p = Reflect.get(window, "nuclearBoundProbe");
          if (p) {
            p.runtime.dispose();
            p.host.remove();
            Reflect.deleteProperty(window, "nuclearBoundProbe");
          }
        })
        .catch(() => {});
    }
  });

  test("aligns live nuclear impact with outgoing actors and quiet reconnects", async ({ page }) => {
    test.setTimeout(240_000);
    if (backend() === "webgpu") {
      await trackTerrainBuffers(page);
    }
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") {
                resolve();
              }
              if (s.phase === "error") {
                reject(new Error(s.message));
              }
            },
            renderer,
          );
          const stats = () => runtime.sceneStats() as SceneDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 9000; i++) {
              await new Promise(requestAnimationFrame);
              if (condition()) {
                return;
              }
            }
            throw new Error("Nuclear timeline: " + JSON.stringify(runtime.assetStats()));
          };
          const replace = (q: number) => {
            game.board.tiles.find(
              (t: ReturnType<typeof nuclearTransitionGame>["board"]["tiles"][number]) =>
                t.coordinates.q === q && t.coordinates.r === 0,
            )!.occupiedBy = { type: "nuclear-zone-tile", tags: [] };
            runtime.setScene(toSceneSnapshot(game, null));
          };
          Reflect.set(window, "nuclearLiveProbe", {
            runtime,
            host,
            game,
            stats,
            wait,
            replace,
            next,
            toSceneSnapshot,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          await ready;
          await wait(() => stats().structures.ready && stats().worldTrees.trees === 2);
          runtime.camera(-2);
          return stats();
        },
        { game: nuclearTransitionGame(), renderer: backend() },
      );
      expect(initial.structures.plots).toBe(1);
      expect(initial.nuclearTimeline.events).toEqual([]);
      const preparing = await page.evaluate(() => {
        const { replace, stats } = Reflect.get(window, "nuclearLiveProbe");
        replace(-1);
        return stats() as SceneDiagnostics;
      });
      expect(preparing.nuclearTimeline.events).toEqual([
        { coordinate: [-1, 0, 1], start: null, motionStart: null },
      ]);
      expect(preparing.structures.plots).toBe(1);
      const flight = await page.evaluate(async () => {
        const { stats, wait } = Reflect.get(window, "nuclearLiveProbe");
        await wait(() =>
          stats().nuclearTimeline.events.some(
            (e: SceneDiagnostics["nuclearTimeline"]["events"][number]) => e.start !== null,
          ),
        );
        return stats() as SceneDiagnostics;
      });
      const start = flight.nuclearTimeline.events[0].start!;
      expect(flight.craters.sites).toBe(1);
      expect(flight.craters.sample[0].coverage).toBe(0);
      expect(flight.structures.plots).toBe(1);
      expect(flight.terrain.revision).toBe(initial.terrain.revision);
      expect(
        flight.structures.sample[0].collapseTimes.every(
          (v) => Math.abs(v - (start / 60 + 1)) < 0.001,
        ),
      ).toBe(true);
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-flight.png`) });
      const impact = await page.evaluate(async () => {
        const { stats, wait } = Reflect.get(window, "nuclearLiveProbe");
        await wait(() => stats().craters.sample[0]?.coverage === 1 && stats().terrain.revision > 1);
        return stats() as SceneDiagnostics;
      });
      expect(impact.step).toBeGreaterThanOrEqual(start + 60);
      expect(impact.structures.plots).toBe(1);
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-impact.png`) });
      const settled = await page.evaluate(async () => {
        const { stats, wait } = Reflect.get(window, "nuclearLiveProbe");
        await wait(
          () =>
            stats().structures.plots === 0 &&
            stats().craters.ready &&
            !stats().terrain.transitioning,
        );
        return stats() as SceneDiagnostics;
      });
      expect(settled.step).toBeGreaterThanOrEqual(start + 128);
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-settled.png`) });
      await page.emulateMedia({ reducedMotion: "reduce" });
      const reduced = await page.evaluate(async () => {
        const { runtime, stats, wait, replace } = Reflect.get(window, "nuclearLiveProbe");
        await wait(() => runtime.assetStats().reducedMotion);
        replace(0);
        await wait(() => stats().craters.sites === 2);
        const first = stats() as SceneDiagnostics;
        await wait(
          () =>
            stats().craters.ready &&
            stats().worldTrees.trees === 1 &&
            stats().craters.sample.every(
              (p: CraterDiagnostics["sample"][number]) => p.coverage === 1,
            ),
        );
        return { first, after: stats() as SceneDiagnostics };
      });
      expect(reduced.first.craters.sample.find((p) => p.coordinate[0] === 0)?.coverage).toBe(0);
      expect(
        reduced.first.worldTrees.sample.find((p) => p.coordinate[0] === 0)?.destroy,
      ).not.toBeNull();
      expect(reduced.after.nuclearTimeline.events).toEqual([]);
      await page.screenshot({
        path: path.join(output, `phase5-nuclear-${backend()}-live-reduced.png`),
      });
      const quiet = await page.evaluate(async () => {
        const { runtime, stats, wait, replace, next, toSceneSnapshot, game } = Reflect.get(
          window,
          "nuclearLiveProbe",
        );
        replace(1);
        const pending = stats() as SceneDiagnostics;
        const ready = next();
        runtime.setScene(toSceneSnapshot(game, null), "snapshot");
        await ready;
        await wait(
          () =>
            stats().craters.sites === 3 && stats().craters.ready && stats().worldTrees.trees === 0,
        );
        return { pending, after: stats() as SceneDiagnostics };
      });
      expect(quiet.pending.nuclearTimeline.events[0].start).toBeNull();
      expect(quiet.after.nuclearTimeline.events).toEqual([]);
      expect(quiet.after.craters.sample.every((p) => p.coverage === 1)).toBe(true);
      await page.emulateMedia({ reducedMotion: "no-preference" });
      const resumed = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "nuclearLiveProbe");
        await wait(() => !runtime.assetStats().reducedMotion);
        const step = stats().step;
        await wait(() => stats().step >= step + 60);
        return stats() as SceneDiagnostics;
      });
      expect(resumed.nuclearTimeline.events).toEqual([]);
      expect(resumed.craters.sample.map((p) => p.buffer)).toEqual(
        quiet.after.craters.sample.map((p) => p.buffer),
      );
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-quiet.png`) });
      const cleared = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "nuclearLiveProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(() => stats().craters.uniforms === 0 && stats().craters.meshBytes === 0);
        return {
          stats: stats() as SceneDiagnostics,
          buffers: Reflect.get(window, "craterBuffers"),
        };
      });
      expect(cleared.stats.nuclearTimeline.events).toEqual([]);
      expect(cleared.stats.craters.sites).toBe(0);
      if (backend() === "webgpu") {
        expect(cleared.buffers.live).toBe(0);
        expect(cleared.buffers.created).toBe(cleared.buffers.destroyed);
      }
      await writeFile(
        path.join(output, `phase5-nuclear-${backend()}-live-diagnostics.json`),
        JSON.stringify(
          { initial, preparing, flight, impact, settled, reduced, quiet, resumed, cleared },
          null,
          2,
        ),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "nuclearLiveProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "nuclearLiveProbe");
        }
      });
    }
  });

  test("renders persisted nuclear craters without replay and retires shared passes", async ({
    page,
  }) => {
    test.setTimeout(240_000);
    if (backend() === "webgpu") {
      await trackTerrainBuffers(page);
    }
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") {
                resolve();
              }
              if (s.phase === "error") {
                reject(new Error(s.message));
              }
            },
            renderer,
          );
          const stats = () => runtime.assetStats().craters as CraterDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 6000; i++) {
              await new Promise(requestAnimationFrame);
              if (condition()) {
                return;
              }
            }
            throw new Error("Crater preparation: " + JSON.stringify(runtime.assetStats()));
          };
          Reflect.set(window, "craterProbe", {
            runtime,
            host,
            game,
            toSceneSnapshot,
            stats,
            wait,
            next,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          await ready;
          await wait(() => stats().ready);
          runtime.camera(-2);
          const writes = stats().uniformWrites;
          await wait(() => stats().uniformWrites >= writes + 20);
          return stats();
        },
        { game: nuclearSceneGame(), renderer: backend() },
      );
      expect(initial.sites).toBe(2);
      expect(initial.uniforms).toBe(3);
      expect(initial.geometryBuilds).toBe(2);
      expect(initial.meshBytes).toBeLessThan(2 * 1024 * 1024);
      expect(
        initial.sample.every(
          (p) => p.warmFrames === 2 && p.coverage === 1 && p.meshes.length === 3,
        ),
      ).toBe(true);
      expect(initial.sample[0].seed).not.toBe(37);
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-initial.png`) });
      const unchanged = await page.evaluate(async () => {
        const { runtime, game, toSceneSnapshot, stats, wait } = Reflect.get(window, "craterProbe");
        game.board.tiles.find(
          (t: ReturnType<typeof nuclearSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        ).occupiedBy.visual.seed = 999;
        runtime.setScene(toSceneSnapshot(game, null), "snapshot");
        const writes = stats().uniformWrites;
        await wait(() => stats().ready && stats().uniformWrites >= writes + 20);
        return stats() as CraterDiagnostics;
      });
      expect(unchanged.sample).toEqual(initial.sample);
      expect(unchanged.geometryBuilds).toBe(2);
      await page.emulateMedia({ reducedMotion: "reduce" });
      const reduced = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "craterProbe");
        await wait(() => runtime.assetStats().reducedMotion);
        const writes = stats().uniformWrites;
        await wait(() => stats().uniformWrites >= writes + 20);
        return stats() as CraterDiagnostics;
      });
      expect(reduced.sample).toEqual(initial.sample);
      await page.screenshot({ path: path.join(output, `phase5-nuclear-${backend()}-reduced.png`) });
      const cleared = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "craterProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(() => stats().uniforms === 0 && stats().meshBytes === 0);
        return {
          stats: stats() as CraterDiagnostics,
          buffers: Reflect.get(window, "craterBuffers"),
        };
      });
      expect(cleared.stats.sites).toBe(0);
      expect(cleared.stats.retiredUniforms).toBe(3);
      if (backend() === "webgpu") {
        expect(cleared.buffers).toEqual({ created: 3, destroyed: 3, live: 0 });
      }
      await writeFile(
        path.join(output, `phase5-nuclear-${backend()}-diagnostics.json`),
        JSON.stringify({ initial, unchanged, reduced, cleared }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "craterProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "craterProbe");
        }
      });
    }
  });

  test("renders seeded volcano and mohole surfaces with stable live bindings", async ({ page }) => {
    test.setTimeout(240_000);
    if (backend() === "webgpu") await trackTerrainBuffers(page);
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") resolve();
              if (s.phase === "error") reject(new Error(s.message));
            },
            renderer,
          );
          const stats = () => runtime.assetStats().specialSurfaces as SpecialSurfaceDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let i = 0; i < 6000; i++) {
              await new Promise(requestAnimationFrame);
              if (condition()) return;
            }
            throw new Error("Special surface preparation: " + JSON.stringify(runtime.assetStats()));
          };
          Reflect.set(window, "specialProbe", {
            runtime,
            host,
            game,
            toSceneSnapshot,
            stats,
            wait,
            next,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          await wait(() => stats().ready);
          runtime.camera(-2);
          const writes = stats().uniformWrites;
          await wait(() => stats().uniformWrites >= writes + 20);
          return stats();
        },
        { game: specialSceneGame(), renderer: backend() },
      );
      expect(initial.sites).toBe(4);
      expect(initial.accessoryMeshes).toBe(2);
      expect(initial.fenceBytes).toBeGreaterThan(10_000);
      expect(initial.uniforms).toBe(5);
      expect(initial.meshBytes).toBeLessThan(4 * 1024 * 1024);
      expect(initial.sample.every((p) => p.born < 0 && p.warmFrames === 2)).toBe(true);
      await page.screenshot({ path: path.join(output, `phase5-special-${backend()}-initial.png`) });
      await page.emulateMedia({ reducedMotion: "reduce" });
      const frozen = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "specialProbe");
        await wait(() => runtime.assetStats().reducedMotion);
        const first = stats() as SpecialSurfaceDiagnostics;
        const at = runtime.sceneStats().step;
        await wait(() => runtime.sceneStats().step >= at + 30);
        return { first, after: stats() as SpecialSurfaceDiagnostics };
      });
      expect(frozen.first.sample.map((p) => p.clock)).toEqual(
        frozen.after.sample.map((p) => p.clock),
      );
      expect(frozen.after.sample.map((p) => [p.mesh, p.buffer])).toEqual(
        initial.sample.map((p) => [p.mesh, p.buffer]),
      );
      await page.screenshot({ path: path.join(output, `phase5-special-${backend()}-frozen.png`) });
      const reseeded = await page.evaluate(async () => {
        const { runtime, stats, wait, game, toSceneSnapshot } = Reflect.get(window, "specialProbe");
        game.board.tiles.find(
          (t: ReturnType<typeof specialSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        ).occupiedBy.visual.seed = 38;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(() =>
          stats().sample.some((p: SpecialSurfaceDiagnostics["sample"][number]) => p.seed === 38),
        );
        const first = stats().sample.find(
          (p: SpecialSurfaceDiagnostics["sample"][number]) => p.seed === 38,
        );
        await wait(
          () =>
            stats().ready &&
            stats().uniforms === 5 &&
            stats().sample.find((p: SpecialSurfaceDiagnostics["sample"][number]) => p.seed === 38)
              ?.clock[2] === 1,
        );
        return { first, after: stats() as SpecialSurfaceDiagnostics };
      });
      expect(reseeded.first.warmFrames).toBe(0);
      expect(reseeded.after.sample.find((p) => p.seed === 38)?.clock[2]).toBe(1);
      expect(
        reseeded.after.sample.filter((p) => p.seed !== 38).map((p) => [p.entity, p.mesh, p.buffer]),
      ).toEqual(
        initial.sample.filter((p) => p.seed !== 37).map((p) => [p.entity, p.mesh, p.buffer]),
      );
      await page.screenshot({
        path: path.join(output, `phase5-special-${backend()}-reseeded.png`),
      });
      const cleared = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "specialProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(() => stats().uniforms === 0 && stats().meshBytes === 0);
        return {
          stats: stats() as SpecialSurfaceDiagnostics,
          buffers: Reflect.get(window, "specialBuffers"),
        };
      });
      expect(cleared.stats.sites).toBe(0);
      expect(cleared.stats.retiredUniforms).toBe(6);
      if (backend() === "webgpu")
        expect(cleared.buffers).toEqual({ created: 6, destroyed: 6, live: 0 });
      await writeFile(
        path.join(output, `phase5-special-${backend()}-diagnostics.json`),
        JSON.stringify({ initial, frozen, reseeded, cleared }, null, 2),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "specialProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "specialProbe");
        }
      });
    }
  });

  test("renders seeded mining and owned fences with bounded live resources", async ({ page }) => {
    test.setTimeout(240_000);
    if (backend() === "webgpu") await trackTerrainBuffers(page);
    const errors: string[] = [],
      requests: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const sourceUrls = new Set(
      gameAssets.flatMap((requirement) =>
        assets[requirement.id as AssetId].variants
          .filter((variant) => variant.url.endsWith(requirement.extension))
          .map((variant) => variant.url),
      ),
    );
    page.on("request", (request) => {
      if (sourceUrls.has(new URL(request.url()).pathname)) requests.push(request.url());
    });
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto("/tests/fixtures/engine.html");
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    try {
      const initial = await page.evaluate(
        async ({ game, renderer }) => {
          const rp = "/src/engine/runtime.ts",
            sp = "/src/engine/scene.ts";
          const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
            import(/* @vite-ignore */ rp),
            import(/* @vite-ignore */ sp),
          ]);
          const host = document.createElement("div");
          host.style.cssText = "position:fixed;inset:0";
          document.body.append(host);
          let resolve = () => {},
            reject = (_e: Error) => {};
          const next = () =>
            new Promise<void>((yes, no) => {
              resolve = yes;
              reject = no;
            });
          const first = next();
          const runtime = startEngine(
            host,
            (s: { phase: string; message: string }) => {
              if (s.phase === "ready") resolve();
              if (s.phase === "error") reject(new Error(s.message));
            },
            renderer,
          );
          const stats = () => runtime.assetStats().worksites as WorksiteDiagnostics;
          const wait = async (condition: () => boolean) => {
            for (let frame = 0; frame < 6000; frame++) {
              await new Promise(requestAnimationFrame);
              if (condition()) return;
            }
            throw new Error(
              "Worksite frame did not complete: " + JSON.stringify(runtime.assetStats()),
            );
          };
          Reflect.set(window, "worksiteRenderStates", {});
          const record = (name: string) => {
            Reflect.get(window, "worksiteRenderStates")[name] = {
              assets: runtime.assetStats(),
              scene: runtime.sceneStats(),
            };
          };
          Reflect.set(window, "worksiteProbe", {
            runtime,
            host,
            game,
            toSceneSnapshot,
            stats,
            wait,
            next,
            record,
          });
          await first;
          const ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          runtime.camera(-2);
          const writes = stats().uniformWrites;
          await wait(() => stats().uniformWrites >= writes + 20);
          record("initial");
          return stats();
        },
        { game: worksiteSceneGame(), renderer: backend() },
      );
      expect(initial.ready).toBe(true);
      expect(initial.sites).toBe(4);
      expect(initial.uniforms).toBe(5);
      expect(initial.geometryBuilds).toBe(4);
      expect(initial.meshBytes).toBeGreaterThan(100_000);
      expect(initial.meshBytes).toBeLessThan(8 * 1024 * 1024);
      expect(initial.sample.filter((p) => p.kind === "Mining")).toHaveLength(2);
      expect(initial.sample.filter((p) => p.kind === "Reserved")).toHaveLength(2);
      expect(initial.sample.every((p) => p.born < 0 && p.warmFrames === 2)).toBe(true);
      await page.screenshot({
        path: path.join(output, `phase5-worksites-${backend()}-initial.png`),
      });
      const moved = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "worksiteProbe");
        const step = runtime.assetStats().motionStep;
        await wait(() => runtime.assetStats().motionStep >= step + 60);
        return stats() as WorksiteDiagnostics;
      });
      expect(moved.sample.find((p) => p.kind === "Mining")!.cart).not.toEqual(
        initial.sample.find((p) => p.kind === "Mining")!.cart,
      );
      expect(moved.sample.map((p) => [p.entity, p.mesh, p.buffer])).toEqual(
        initial.sample.map((p) => [p.entity, p.mesh, p.buffer]),
      );
      await page.screenshot({
        path: path.join(output, `phase5-worksites-${backend()}-moving.png`),
      });
      await page.emulateMedia({ reducedMotion: "reduce" });
      const frozen = await page.evaluate(async () => {
        const { runtime, stats, wait } = Reflect.get(window, "worksiteProbe");
        await wait(() => runtime.assetStats().reducedMotion);
        const first = stats(),
          step = runtime.assetStats().terrainStep;
        await wait(() => runtime.assetStats().terrainStep >= step + 60);
        Reflect.get(window, "worksiteProbe").record("frozen");
        return { first, last: stats() as WorksiteDiagnostics };
      });
      expect(frozen.last.sample.map((p) => p.cart)).toEqual(
        frozen.first.sample.map((p: WorksiteDiagnostics["sample"][number]) => p.cart),
      );
      const frozenImage = await page.screenshot({
        path: path.join(output, `phase5-worksites-${backend()}-frozen.png`),
      });
      const recolored = await page.evaluate(async () => {
        const { runtime, stats, wait, game, toSceneSnapshot } = Reflect.get(
          window,
          "worksiteProbe",
        );
        game.otherPlayers[0].color = "#ee44cc";
        // ReservedAreaTile never uses its visual seed to generate fence geometry.
        game.board.tiles.find(
          (t: ReturnType<typeof worksiteSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === -1 && t.coordinates.r === 0,
        ).occupiedBy.visual.seed = 888;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(
          () =>
            runtime.assetStats().gridReady &&
            stats().sample.find(
              (p: WorksiteDiagnostics["sample"][number]) =>
                p.coordinate[0] === 0 && p.coordinate[1] === 1,
            ).color[0] > 0.8,
        );
        return stats() as WorksiteDiagnostics;
      });
      expect(recolored.sample.map((p) => [p.entity, p.mesh, p.buffer])).toEqual(
        initial.sample.map((p) => [p.entity, p.mesh, p.buffer]),
      );
      expect(recolored.geometryBuilds).toBe(initial.geometryBuilds);
      await page.evaluate(() => Reflect.get(window, "worksiteProbe").record("owner"));
      expect(recolored.highlightBuffer).toBe(initial.highlightBuffer);
      const ownerImage = await page.screenshot({
        path: path.join(output, `phase5-worksites-${backend()}-owner.png`),
      });
      const [outsideBefore, outsideAfter] = await Promise.all(
        [frozenImage, ownerImage].map((buffer) =>
          sharp(buffer)
            .extract({ left: 0, top: 0, width: 160, height: 720 })
            .removeAlpha()
            .raw()
            .toBuffer(),
        ),
      );
      let outsideDifference = 0;
      for (let i = 0; i < outsideBefore.length; i++)
        outsideDifference += Math.abs(outsideBefore[i] - outsideAfter[i]);
      expect(outsideDifference / outsideBefore.length).toBeLessThan(0.1);
      await page.emulateMedia({ reducedMotion: "no-preference" });
      const reseeded = await page.evaluate(async () => {
        const { runtime, stats, wait, game, toSceneSnapshot } = Reflect.get(
          window,
          "worksiteProbe",
        );
        game.board.tiles.find(
          (t: ReturnType<typeof worksiteSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        ).occupiedBy.visual.seed = 38;
        runtime.setScene(toSceneSnapshot(game, null));
        await wait(() =>
          stats().sample.some((p: WorksiteDiagnostics["sample"][number]) => p.seed === 38),
        );
        const first = stats().sample.find(
          (p: WorksiteDiagnostics["sample"][number]) => p.seed === 38,
        );
        await wait(() => stats().ready);
        return { first, settled: stats() as WorksiteDiagnostics };
      });
      expect(reseeded.first.warmFrames).toBe(0);
      expect(reseeded.first.buffer).not.toBe(initial.sample.find((p) => p.seed === 37)!.buffer);
      expect(
        reseeded.settled.sample
          .filter((p) => p.seed !== 38)
          .map((p) => [p.entity, p.mesh, p.buffer]),
      ).toEqual(
        initial.sample.filter((p) => p.seed !== 37).map((p) => [p.entity, p.mesh, p.buffer]),
      );
      expect(reseeded.settled.sample.find((p) => p.seed === 38)!.born).toBeGreaterThanOrEqual(0);
      await page.screenshot({
        path: path.join(output, `phase5-worksites-${backend()}-reseeded.png`),
      });
      const quiet = await page.evaluate(async () => {
        const { runtime, stats, wait, game, toSceneSnapshot, next } = Reflect.get(
          window,
          "worksiteProbe",
        );
        const tile = game.board.tiles.find(
          (t: ReturnType<typeof worksiteSceneGame>["board"]["tiles"][number]) =>
            t.coordinates.q === 0 && t.coordinates.r === 0,
        );
        tile.occupiedBy = { type: "nuclear-zone-tile", displayName: "Missed impact", tags: [] };
        const ready = next();
        runtime.setScene(toSceneSnapshot(game, null), "snapshot");
        await ready;
        await wait(() => stats().uniforms === 4);
        return stats() as WorksiteDiagnostics;
      });
      expect(quiet.sites).toBe(3);
      expect(quiet.sample.every((p) => p.born < 0 && p.destroy === null)).toBe(true);
      expect(quiet.sample.map((p) => [p.entity, p.mesh, p.buffer])).toEqual(
        initial.sample.filter((p) => p.seed !== 37).map((p) => [p.entity, p.mesh, p.buffer]),
      );
      const cleared = await page.evaluate(async () => {
        const { runtime, stats, wait, next } = Reflect.get(window, "worksiteProbe");
        const ready = next();
        runtime.setScene(null);
        await ready;
        await wait(() => stats().uniforms === 0 && stats().meshBytes === 0);
        return {
          stats: stats() as WorksiteDiagnostics,
          buffers: Reflect.get(window, "worksiteBuffers"),
        };
      });
      expect(cleared.stats.sites).toBe(0);
      expect(cleared.stats.highlightBuffer).toBeNull();
      expect(cleared.stats.retiredUniforms).toBe(6);
      if (backend() === "webgpu")
        expect(cleared.buffers).toEqual({ created: 6, destroyed: 6, live: 0 });
      expect(requests).toHaveLength(gameAssets.length);
      await writeFile(
        path.join(output, `phase5-worksites-${backend()}-diagnostics.json`),
        JSON.stringify(
          {
            initial,
            moved,
            frozen,
            recolored,
            reseeded,
            quiet,
            cleared,
            renderStates: await page.evaluate(() => Reflect.get(window, "worksiteRenderStates")),
          },
          null,
          2,
        ),
      );
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => {
        const p = Reflect.get(window, "worksiteProbe");
        if (p) {
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "worksiteProbe");
        }
      });
    }
  });

  test("reuses encoded assets and retires decoded scene resources", async ({ page }) => {
    test.setTimeout(240_000);
    const lifetimeGame = citySceneGame();
    lifetimeGame.board.tiles.find(
      (t) => t.coordinates.q === -1 && t.coordinates.r === 1,
    )!.occupiedBy = worksiteSceneGame().board.tiles.find(
      (t) => t.coordinates.q === 0 && t.coordinates.r === 0,
    )!.occupiedBy;
    lifetimeGame.board.tiles.find(
      (t) => t.coordinates.q === 0 && t.coordinates.r === -1,
    )!.reservedBy = "player";
    lifetimeGame.board.tiles.find(
      (tile) => tile.coordinates.q === 0 && tile.coordinates.r === 2,
    )!.occupiedBy = {
      type: "world-tree-tile",
      displayName: "World tree",
      tags: [],
      visual: { seed: 37 },
    };
    for (const [q, r, type, seed] of [
      [-1, 2, "mohole-tile", 77],
      [1, -1, "volcano-tile", 37],
    ] as const) {
      lifetimeGame.board.tiles.find(
        (tile) => tile.coordinates.q === q && tile.coordinates.r === r,
      )!.occupiedBy = { type, tags: [], visual: { seed } };
    }
    if (backend() === "webgpu") await trackTerrainBuffers(page);
    const requests: string[] = [];
    const sourceUrls = new Set(
      gameAssets.flatMap((requirement) =>
        assets[requirement.id as AssetId].variants
          .filter((variant) => variant.url.endsWith(requirement.extension))
          .map((variant) => variant.url),
      ),
    );
    page.on("request", (request) => {
      if (sourceUrls.has(new URL(request.url()).pathname)) requests.push(request.url());
    });
    await page.goto("/tests/fixtures/engine.html");
    const result = await page.evaluate(
      async ({ renderer, game, assetCount }) => {
        const runtimePath = "/src/engine/runtime.ts";
        const scenePath = "/src/engine/scene.ts";
        const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
          import(/* @vite-ignore */ runtimePath),
          import(/* @vite-ignore */ scenePath),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;left:0;top:0;width:800px;height:600px";
        document.body.append(host);
        let resolveReady = () => {};
        let rejectReady = (_error: Error) => {};
        const nextReady = () =>
          new Promise<void>((resolve, reject) => {
            resolveReady = resolve;
            rejectReady = reject;
          });
        const first = nextReady();
        let measured = false;
        const runtime = startEngine(
          host,
          (status: { phase: string; message: string; loaded?: number }) => {
            if (status.phase === "ready") resolveReady();
            if (status.phase === "error") rejectReady(new Error(status.message));
            if (status.message.startsWith("Loading world assets") && (status.loaded ?? 0) > 0)
              measured = true;
          },
          renderer,
        );
        try {
          await first;
          const baseline = runtime.assetStats();
          const cleared: Record<string, number>[] = [];
          for (let cycle = 0; cycle < 3; cycle++) {
            const loading = nextReady();
            runtime.setScene(toSceneSnapshot(game, null));
            await loading;
            if (runtime.assetStats().sources !== assetCount || runtime.assetStats().models < 1)
              throw new Error("No real model residency");
            const stats = runtime.assetStats();
            if (stats.gpuArrays !== 2 || stats.gpuArrayLayers !== 20 || stats.gpuArrayMips !== 22)
              throw new Error("No real mipmapped terrain array residency");
            if (stats.gridUniforms !== 1 || stats.gridUniformWrites < 1)
              throw new Error("No active native grid");
            if (stats.cpuArrays !== 0)
              throw new Error("Decoded atlas pixels were retained after upload");
            if (
              stats.terrainBatches !== 1 ||
              stats.terrainUniforms !== 2 ||
              stats.terrainUniformWrites < 1
            )
              throw new Error("No active native terrain batch");
            if (
              stats.structureTexturesPrepared !== 34 ||
              stats.structureTextureImages !== 34 ||
              stats.structureTextureCpuBytes !== 0 ||
              stats.structureTextureMipDraws < 300
            )
              throw new Error("Structure textures did not finish real GPU preparation");
            if (
              stats.structures.plots !== 4 ||
              stats.structures.uniforms !== 5 ||
              stats.structures.groupImages !== 1 ||
              stats.structures.groups < 50
            )
              throw new Error("Native city residency missing: " + JSON.stringify(stats.structures));
            if (
              !stats.worksites.ready ||
              stats.worksites.sites !== 2 ||
              stats.worksites.uniforms !== 3
            )
              throw new Error(
                "Native worksite residency missing: " + JSON.stringify(stats.worksites),
              );
            if (
              !stats.specialSurfaces.ready ||
              stats.specialSurfaces.sites !== 2 ||
              stats.specialSurfaces.accessoryMeshes !== 1 ||
              stats.specialSurfaces.uniforms !== 3
            )
              throw new Error("Missing special surfaces: " + JSON.stringify(stats.specialSurfaces));
            if (
              !stats.particles.ready ||
              stats.particles.uniforms !== 1 ||
              stats.particles.sprites < 1 ||
              stats.particles.imageMips !== 9
            )
              throw new Error("Missing native particles: " + JSON.stringify(stats.particles));
            runtime.present({
              gameId: game.id,
              id: `lifetime:${cycle}`,
              kind: "asteroid",
              seed: 37,
            });
            let asteroidReady = false;
            for (let frame = 0; frame < 300; frame++) {
              if (runtime.sceneStats()?.asteroids.active?.warmFrames === 2) {
                asteroidReady = true;
                break;
              }
              await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
            }
            if (!asteroidReady) {
              throw new Error("Asteroid did not reach actual GPU preparation");
            }
            const clearing = nextReady();
            runtime.setScene(null);
            await clearing;
            for (let frame = 0; frame < 300; frame++) {
              const stats = runtime.assetStats();
              if (
                stats.particles.uniforms === 0 &&
                stats.particles.sprites === 0 &&
                stats.specialSurfaces.uniforms === 0 &&
                stats.specialSurfaces.meshBytes === 0 &&
                stats.worksites.uniforms === 0 &&
                stats.worksites.meshBytes === 0 &&
                stats.sources === 0 &&
                stats.models === 0 &&
                stats.meshes === baseline.meshes &&
                stats.materials === baseline.materials &&
                stats.images === baseline.images &&
                stats.gpuImages === baseline.gpuImages &&
                stats.gpuMeshes === baseline.gpuMeshes &&
                stats.gpuMaterials === baseline.gpuMaterials
              )
                break;
              await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
            }
            cleared.push(runtime.assetStats());
          }
          return {
            asteroids: runtime.sceneStats()!.asteroids,
            baseline,
            cleared,
            measured,
            shadeBuffers: Reflect.get(window, "shadeBuffers"),
            structureBuffers: Reflect.get(window, "structureBuffers"),
            worldTreeBuffers: Reflect.get(window, "worldTreeBuffers"),
            worksiteBuffers: Reflect.get(window, "worksiteBuffers"),
            specialBuffers: Reflect.get(window, "specialBuffers"),
            particleBuffers: Reflect.get(window, "particleBuffers"),
          };
        } finally {
          runtime.dispose();
          host.remove();
        }
      },
      {
        renderer: backend(),
        game: lifetimeGame,
        assetCount: gameAssets.length,
      },
    );
    expect(result.measured).toBe(true);
    if (backend() === "webgpu")
      expect(result.shadeBuffers).toEqual({ created: 3, destroyed: 3, live: 0 });
    if (backend() === "webgpu")
      expect(result.structureBuffers).toEqual({ created: 15, destroyed: 15, live: 0 });
    if (backend() === "webgpu")
      expect(result.worldTreeBuffers).toEqual({ created: 3, destroyed: 3, live: 0 });
    if (backend() === "webgpu")
      expect(result.worksiteBuffers).toEqual({ created: 9, destroyed: 9, live: 0 });
    if (backend() === "webgpu")
      expect(result.specialBuffers).toEqual({ created: 9, destroyed: 9, live: 0 });
    if (backend() === "webgpu")
      expect(result.particleBuffers).toEqual({ created: 3, destroyed: 3, live: 0 });
    expect(result.asteroids.started).toBe(3);
    expect(result.asteroids.canceled).toBe(3);
    expect(result.asteroids.active).toBeNull();
    expect(result.asteroids.shared).toBe(false);
    expect(result.asteroids.retired).toBe(0);
    expect(result.baseline.gpuMeshes).toEqual(expect.any(Number));
    expect(result.baseline.gpuMaterials).toEqual(expect.any(Number));
    for (const stats of result.cleared) {
      const particles = stats.particles as unknown as ParticleDiagnostics;
      expect(particles.sprites).toBe(0);
      expect(particles.uniforms).toBe(0);
      expect(particles.paletteBuffer).toBeNull();
      const special = stats.specialSurfaces as unknown as SpecialSurfaceDiagnostics;
      expect(special.sites).toBe(0);
      expect(special.uniforms).toBe(0);
      expect(special.meshBytes).toBe(0);
      expect(special.fenceBytes).toBe(0);
      const worksites = stats.worksites as unknown as WorksiteDiagnostics;
      expect(worksites.sites).toBe(0);
      expect(worksites.uniforms).toBe(0);
      expect(worksites.meshBytes).toBe(0);
      expect(worksites.building).toBe(false);
      expect((stats.worldTrees as unknown as WorldTreeDiagnostics).trees).toBe(0);
      expect((stats.worldTrees as unknown as WorldTreeDiagnostics).uniforms).toBe(0);
      expect(stats.sources).toBe(0);
      expect(stats.models).toBe(0);
      expect(stats.gpuArrays).toBe(0);
      expect(stats.terrainBatches).toBe(0);
      expect(stats.terrainUniforms).toBe(0);
      expect(stats.gridUniforms).toBe(0);
      expect(stats.foliageUniforms).toBe(0);
      expect(stats.foliagePlants).toBe(0);
      expect(stats.foliageTextures).toBe(0);
      expect(stats.shadeBuffers).toBe(0);
      expect(stats.structureTextureImages).toBe(0);
      expect(stats.structureTexturesPrepared).toBe(0);
      expect(stats.structureTextureCpuBytes).toBe(0);
      expect(stats.meshes).toBe(result.baseline.meshes);
      expect(stats.materials).toBe(result.baseline.materials);
      expect(stats.images).toBe(result.baseline.images);
      expect(stats.gpuImages).toBe(result.baseline.gpuImages);
      expect(stats.gpuMeshes).toBe(result.baseline.gpuMeshes);
      expect(stats.gpuMaterials).toBe(result.baseline.gpuMaterials);
    }
    expect(new Set(requests).size).toBe(gameAssets.length);
    expect(requests).toHaveLength(gameAssets.length);
  });

  test("renders approved foliage variants and preserves live plant identities", async ({
    page,
  }) => {
    await page.addInitScript(() => {
      const allocations: { target: number; layers: number }[] = [];
      Reflect.set(window, "terrainArrayStorage", allocations);
      const proto = WebGL2RenderingContext.prototype;
      const storage = proto.texStorage3D;
      proto.texStorage3D = function (target, levels, format, width, height, depth) {
        if (width === 132 && height === 132) allocations.push({ target, layers: depth });
        storage.call(this, target, levels, format, width, height, depth);
      };
    });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(message.text());
    });
    await page.goto("/tests/fixtures/engine.html");
    const game = sceneGame();
    game.globalParameters.temperature = 8;
    game.globalParameters.oxygen = 14;
    for (const tile of game.board.tiles) {
      if (Math.abs(tile.coordinates.q) <= 1 && Math.abs(tile.coordinates.r) <= 1) {
        tile.occupiedBy = { type: "greenery-tile", tags: [] };
      }
    }
    const result = await page.evaluate(
      async ({ renderer, game }) => {
        const runtimePath = "/src/engine/runtime.ts";
        const scenePath = "/src/engine/scene.ts";
        const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
          import(/* @vite-ignore */ runtimePath),
          import(/* @vite-ignore */ scenePath),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0;width:100vw;height:100vh";
        document.body.append(host);
        let resolveReady = () => {};
        let rejectReady = (_error: Error) => {};
        const nextReady = () =>
          new Promise<void>((resolve, reject) => {
            resolveReady = resolve;
            rejectReady = reject;
          });
        const first = nextReady();
        const runtime = startEngine(
          host,
          (status: { phase: string; message: string }) => {
            if (status.phase === "ready") resolveReady();
            if (status.phase === "error") rejectReady(new Error(status.message));
          },
          renderer,
        );
        try {
          await first;
          const loaded = nextReady();
          runtime.setScene(toSceneSnapshot(game, null));
          await loaded;
          const capture = async (condition: (stats: Record<string, number>) => boolean) => {
            let writes = runtime.assetStats()!.terrainUniformWrites;
            for (let i = 0; i < 4000; i++) {
              await new Promise(requestAnimationFrame);
              const stats = runtime.assetStats()!;
              if (stats.terrainUniformWrites > writes && condition(stats))
                return host.querySelector("canvas")!.toDataURL("image/png");
              writes = stats.terrainUniformWrites;
            }
            throw new Error(
              "No completed foliage terrain cache frame: " + JSON.stringify(runtime.assetStats()),
            );
          };
          const live = await capture((stats) => stats.bakeStrip > 0 && stats.bakeValid === 0);
          console.info("Foliage live frame captured");
          const cached = await capture((stats) => stats.bakeValid === 1);
          console.info("Foliage cached frame captured");
          const before = {
            scene: runtime.sceneStats()! as SceneDiagnostics,
            stats: runtime.assetStats()!,
          };
          const writesBeforeZoom = runtime.assetStats()!.terrainUniformWrites;
          runtime.camera(-1);
          await capture(
            (stats) =>
              stats.terrainUniformWrites >= writesBeforeZoom + 2 &&
              runtime.sceneStats()!.camera.distance < before.scene.camera.distance,
          );
          console.info("Foliage zoom frame captured");
          const after = {
            scene: runtime.sceneStats()! as SceneDiagnostics,
            stats: runtime.assetStats()!,
          };
          Reflect.set(window, "foliageProbe", {
            dispose: () => {
              runtime.dispose();
              host.remove();
            },
          });
          return {
            before,
            after,
            live,
            cached,
            textures: renderer === "webgl2" ? Reflect.get(window, "terrainArrayStorage") : [],
          };
        } catch (error) {
          runtime.dispose();
          host.remove();
          throw error;
        }
      },
      { renderer: backend(), game },
    );
    const { before, after } = result;
    const livePNG = Buffer.from(result.live.split(",")[1], "base64");
    const cachedPNG = Buffer.from(result.cached.split(",")[1], "base64");
    const meta = await sharp(livePNG).metadata();
    const region = {
      left: Math.floor(meta.width! * 0.3),
      top: Math.floor(meta.height! * 0.2),
      width: Math.floor(meta.width! * 0.4),
      height: Math.floor(meta.height! * 0.6),
    };
    const livePixels = await sharp(livePNG).extract(region).removeAlpha().raw().toBuffer();
    const cachedPixels = await sharp(cachedPNG).extract(region).removeAlpha().raw().toBuffer();
    expect(livePixels.some((v) => v > 150)).toBe(true);
    expect(cachedPixels.length).toBe(livePixels.length);
    let error = 0;
    for (let i = 0; i < livePixels.length; i++) error += Math.abs(livePixels[i] - cachedPixels[i]);
    expect(error / livePixels.length).toBeLessThan(2);
    expect(before.scene.vegetation.models).toHaveLength(19);
    for (const model of before.scene.vegetation.models) {
      expect(model.vertices).toBeGreaterThan(2);
      expect(model.indices % 3).toBe(0);
      expect(Math.max(...model.max.map((v, i) => v - model.min[i]))).toBeCloseTo(model.extent, 5);
      expect(model.min[2]).toBeCloseTo(0, 5);
    }
    if (backend() === "webgl2") {
      // Initial snapshots need one settled field per patch, without blend history.
      const layers = Math.max(2, before.stats.terrainPatches);
      expect(result.textures).toEqual([
        { target: 35866, layers: layers + Number(layers % 6 === 0) },
      ]);
    }
    expect(before.stats.foliagePlants).toBeGreaterThan(100);
    expect(before.stats.foliageTextures).toBeGreaterThan(10);
    expect(before.stats.foliageMipDraws).toBeGreaterThan(100);
    expect(before.stats.foliageUniforms).toBe(1);
    expect(before.stats.shadeShapes).toBe(before.stats.foliagePlants * 2);
    expect(before.stats.shadeRevision).toBeGreaterThan(0);
    expect(before.stats.shadeBuffers).toBe(1);
    expect(before.scene.vegetation.sample).toHaveLength(32);
    expect(after.scene.camera.distance).toBeLessThan(before.scene.camera.distance);
    expect(after.scene.vegetation.sample).toEqual(before.scene.vegetation.sample);
    expect(after.stats.foliageSpawns).toBe(before.stats.foliageSpawns);
    expect(after.stats.foliageUniforms).toBe(1);
    expect(after.stats.shadeBuffers).toBe(1);
    expect(after.stats.shadeDraws).toBe(before.stats.shadeDraws);
    const output = path.resolve("../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    await writeFile(
      path.join(output, `phase5-foliage-${backend()}-metrics.json`),
      JSON.stringify({ before, after, textures: result.textures }),
    );
    for (const [name, data] of Object.entries({ live: result.live, cached: result.cached })) {
      await writeFile(
        path.join(output, `phase5-foliage-${backend()}-${name}.png`),
        Buffer.from(data.split(",")[1], "base64"),
      );
    }
    const forestImage = await page.screenshot({
      path: path.join(output, `phase5-foliage-${backend()}-warm.png`),
    });
    const forestMeta = await sharp(forestImage).metadata();
    const forestPixels = await sharp(forestImage)
      .extract({
        left: Math.floor(forestMeta.width! * 0.42),
        top: Math.floor(forestMeta.height! * 0.37),
        width: Math.floor(forestMeta.width! * 0.16),
        height: Math.floor(forestMeta.height! * 0.26),
      })
      .removeAlpha()
      .raw()
      .toBuffer();
    let foliagePixels = 0;
    for (let i = 0; i < forestPixels.length; i += 3) {
      if (forestPixels[i] < 100 && forestPixels[i + 1] < 115 && forestPixels[i + 2] < 100)
        foliagePixels++;
    }
    // Residency alone cannot catch an equal-depth pass rejecting every leaf.
    expect(foliagePixels).toBeGreaterThan(20_000);
    await page.evaluate(() => {
      Reflect.get(window, "foliageProbe").dispose();
      Reflect.deleteProperty(window, "foliageProbe");
    });
    expect(errors).toEqual([]);
  });

  test("settles initial and reconnect snapshots while retaining resident assets", async ({
    page,
  }) => {
    test.setTimeout(240_000);
    if (backend() === "webgpu") await trackTerrainBuffers(page);
    const game = worldTreeSceneGame();
    game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === 1)!.occupiedBy =
      citySceneGame().board.tiles.find(
        (t) => t.coordinates.q === 1 && t.coordinates.r === 0,
      )!.occupiedBy;
    const sourceUrls = new Set(
      gameAssets.flatMap((requirement) =>
        assets[requirement.id as AssetId].variants
          .filter((variant) => variant.url.endsWith(requirement.extension))
          .map((variant) => variant.url),
      ),
    );
    const requests: string[] = [],
      errors: string[] = [];
    page.on("request", (request) => {
      if (sourceUrls.has(new URL(request.url()).pathname)) requests.push(request.url());
    });
    page.on("pageerror", (error) => errors.push(error.message));
    let release = () => {};
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    let requested = () => {};
    const downloading = new Promise<void>((resolve) => {
      requested = resolve;
    });
    await page.route("**/assets/models/bird.*.glb", async (route) => {
      requested();
      await gate;
      await route.continue();
    });
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto(engineUrl());
    await page.evaluate(async (game) => {
      const gp = "/src/stores/gameStore.ts",
        pp = "/src/stores/appPhaseStore.ts",
        ep = "/src/stores/engineStore.ts";
      const [{ useGameStore }, { useAppPhaseStore }, { useEngineStore }] = await Promise.all([
        import(/* @vite-ignore */ gp),
        import(/* @vite-ignore */ pp),
        import(/* @vite-ignore */ ep),
      ]);
      const phases: string[] = [];
      Reflect.set(window, "snapshotPhases", phases);
      useEngineStore.subscribe((s: { status: { phase: string } }) =>
        Reflect.get(window, "snapshotPhases").push(s.status.phase),
      );
      useGameStore.getState().setGame(game);
      useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
    }, game);
    await downloading;
    // An authoritative change while source bytes are in flight joins the quiet baseline.
    game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === 2)!.occupiedBy = {
      type: "greenery-tile",
      displayName: "Loaded forest",
      tags: [],
      visual: { seed: 37 },
    };
    await page.evaluate(async (game) => {
      const gp = "/src/stores/gameStore.ts";
      const { useGameStore } = await import(/* @vite-ignore */ gp);
      useGameStore.getState().setGame(game);
    }, game);
    release();
    const stats = () =>
      page.evaluate(async () => {
        const ep = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ ep);
        return useEngineStore.getState().inspectScene() as SceneDiagnostics;
      });
    const ready = async () => {
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 90_000 },
      );
      await expect.poll(async () => (await stats()).settling).toBe(false);
      await expect(page.getByRole("status")).toHaveCount(0);
    };
    await ready();
    const initial = await stats();
    expect(initial.worldTrees.trees).toBe(2);
    expect(initial.worldTrees.sample.every((p) => p.born < 0 && p.destroy === null)).toBe(true);
    expect(initial.structures.sample).toHaveLength(1);
    expect(initial.structures.sample.every((p) => p.born < 0)).toBe(true);
    expect(initial.vegetation.sample.length).toBeGreaterThan(0);
    expect(initial.vegetation.sample.every((p) => p.born < 0)).toBe(true);
    expect(requests).toHaveLength(gameAssets.length);
    const baselineRequests = [...requests];
    const output = path.resolve(import.meta.dirname, "../../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    await page.screenshot({ path: path.join(output, `phase5-snapshot-${backend()}-initial.png`) });
    const connect = async (connected: boolean) => {
      await page.evaluate(async (connected) => {
        const gp = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ gp);
        Reflect.set(window, "snapshotPhases", []);
        useGameStore.getState().setIsConnected(connected);
      }, connected);
      await expect
        .poll(
          () =>
            page.evaluate(() => {
              const phases = Reflect.get(window, "snapshotPhases") as string[];
              return phases.includes("preparing") && phases.includes("ready");
            }),
          { timeout: 60_000 },
        )
        .toBe(true);
    };
    const update = async () =>
      page.evaluate(async (game) => {
        const gp = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ gp);
        useGameStore.getState().setGame(game);
      }, game);
    // First connected delivery can have the exact same JSON; it still establishes a baseline.
    await connect(true);
    await ready();
    const connected = await stats();
    expect(connected.structures.bufferIds).toEqual(initial.structures.bufferIds);
    expect(connected.worldTrees.sample.map((p) => p.buffer)).toEqual(
      initial.worldTrees.sample.map((p) => p.buffer),
    );
    const liveTile = game.board.tiles.find((t) => t.coordinates.q === -1 && t.coordinates.r === 0)!;
    liveTile.occupiedBy = {
      type: "world-tree-tile",
      displayName: "Live tree",
      tags: [],
      visual: { seed: 2027 },
    };
    await update();
    await expect.poll(async () => (await stats()).worldTrees.trees, { timeout: 60_000 }).toBe(3);
    const live = await stats();
    expect(live.worldTrees.sample.find((p) => p.seed === 2027)!.born).toBeGreaterThanOrEqual(0);
    await connect(false);
    // Reconnect missed a nuclear replacement and a new tree. Neither replays its event.
    game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === 0)!.occupiedBy = {
      type: "nuclear-zone-tile",
      displayName: "Missed nuclear event",
      tags: [],
    };
    game.board.tiles.find((t) => t.coordinates.q === -2 && t.coordinates.r === 0)!.occupiedBy = {
      type: "world-tree-tile",
      displayName: "Missed tree",
      tags: [],
      visual: { seed: 997 },
    };
    await update();
    await expect
      .poll(async () => (await stats()).tiles.find((t) => t.id === "0,0,0")!.occupant)
      .toBe("nuclear-zone-tile");
    expect((await stats()).worldTrees.sample.some((p) => p.seed === 37)).toBe(false);
    await ready();
    await expect
      .poll(async () => (await stats()).worldTrees.sample.map((p) => p.seed).sort(), {
        timeout: 60_000,
      })
      .toEqual([2027, 997, 999].sort());
    const resynced = await stats();
    expect(resynced.worldTrees.sample.every((p) => p.born < 0 && p.destroy === null)).toBe(true);
    expect(resynced.structures.bufferIds).toEqual(initial.structures.bufferIds);
    expect(resynced.structures.meshIds).toEqual(initial.structures.meshIds);
    expect(resynced.worldTrees.sample.find((p) => p.seed === 999)!.buffer).toBe(
      initial.worldTrees.sample.find((p) => p.seed === 999)!.buffer,
    );
    expect(resynced.step).toBeGreaterThan(initial.step);
    expect(resynced.terrain.transitioning).toBe(false);
    await connect(true);
    await ready();
    expect(requests).toEqual(baselineRequests);
    await page.screenshot({
      path: path.join(output, `phase5-snapshot-${backend()}-reconnected.png`),
    });
    // A later genuine connected placement is animated again.
    game.board.tiles.find((t) => t.coordinates.q === -3 && t.coordinates.r === 0)!.occupiedBy = {
      type: "world-tree-tile",
      displayName: "Next live tree",
      tags: [],
      visual: { seed: 717 },
    };
    await update();
    await expect.poll(async () => (await stats()).worldTrees.trees, { timeout: 60_000 }).toBe(4);
    expect(
      (await stats()).worldTrees.sample.find((p) => p.seed === 717)!.born,
    ).toBeGreaterThanOrEqual(0);
    expect(requests).toEqual(baselineRequests);
    expect(errors).toEqual([]);
  });

  test("bridges snapshots, requests and authoritative replacements", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    const commands: { type: string; payload: unknown }[] = [];
    let broadcast = (_message: string) => {};
    let releaseDownload = () => {};
    const downloadGate = new Promise<void>((resolve) => {
      releaseDownload = resolve;
    });
    await page.route("**/*_bg.wasm", async (route) => {
      await downloadGate;
      await route.continue();
    });
    await page.routeWebSocket(/\/ws(?:\?|$)/, (socket) => {
      broadcast = (message) => socket.send(message);
      socket.onMessage((message) => commands.push(JSON.parse(String(message))));
    });
    await page.goto(`${engineUrl()}&compact`);
    const ready = () =>
      expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready", {
        timeout: 90_000,
      });
    const game = sceneGame();
    await page.evaluate(async (game) => {
      const gamePath = "/src/stores/gameStore.ts";
      const phasePath = "/src/stores/appPhaseStore.ts";
      const socketPath = "/src/services/webSocketService.ts";
      const [{ useGameStore }, { useAppPhaseStore }, { webSocketService }] = await Promise.all([
        import(/* @vite-ignore */ gamePath),
        import(/* @vite-ignore */ phasePath),
        import(/* @vite-ignore */ socketPath),
      ]);
      webSocketService.on("game-updated", (next: unknown) => useGameStore.getState().setGame(next));
      await webSocketService.connect();
      useGameStore.getState().setGame(game);
      useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
    }, game);
    releaseDownload();
    await ready();
    const canvas = page.locator("canvas");
    const center = async () => {
      const { data } = await sharp(await canvas.screenshot())
        .extract({ left: 720, top: 490, width: 1, height: 1 })
        .removeAlpha()
        .raw()
        .toBuffer({ resolveWithObject: true });
      return [...data];
    };
    const expectTerrain = async (revision: number, previousPixel: number[]) => {
      await expect
        .poll(
          async () => {
            return page.evaluate(async () => {
              const path = "/src/stores/engineStore.ts";
              const { useEngineStore } = await import(/* @vite-ignore */ path);
              return (useEngineStore.getState().inspectScene!()! as SceneDiagnostics).terrain;
            });
          },
          { timeout: 60_000 },
        )
        .toEqual({ revision, transitioning: false });
      await expect
        .poll(
          async () => {
            const pixel = await center();
            return Math.max(
              ...pixel.map((channel, index) => Math.abs(channel - previousPixel[index])),
            );
          },
          { timeout: 15_000 },
        )
        .toBeGreaterThan(10);
    };
    const invalidTarget = await page.evaluate(async () => {
      const path = "/src/stores/engineStore.ts";
      const { useEngineStore } = await import(/* @vite-ignore */ path);
      return (useEngineStore.getState().inspectScene!()! as SceneDiagnostics).tiles.find(
        (tile) => tile.id === "1,0,-1",
      )!.point!;
    });
    await canvas.click({
      position: { x: invalidTarget[0], y: invalidTarget[1] },
    });
    await expect(page.getByRole("tooltip")).toBeVisible();
    await expect(page.getByRole("button", { name: "Confirm", exact: true })).toHaveCount(0);
    expect(commands).toEqual([]);
    const occupant = () =>
      page.evaluate(async () => {
        const path = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ path);
        return (useEngineStore.getState().inspectScene!()! as SceneDiagnostics).tiles.find(
          (tile) => tile.id === "0,0,0",
        )!.occupant;
      });
    expect(await occupant()).toBeNull();
    await canvas.click({ position: { x: 720, y: 500 } });
    await expect(page.getByRole("button", { name: "Confirm", exact: true })).toBeVisible();
    expect(commands).toEqual([]);
    await page.getByRole("button", { name: "Confirm", exact: true }).click();
    await expect
      .poll(() => commands.map((command) => command.type))
      .toEqual(["action.tile-selection.tile-selected"]);
    expect(commands[0].payload).toEqual({ hex: "0,0,0" });
    // Sending a command does not fabricate an occupant. Only the server snapshot does.
    expect(await occupant()).toBeNull();
    const placed = structuredClone(game);
    placed.currentPlayer.pendingTileSelection = undefined;
    placed.generation = 2;
    placed.board.tiles.find(
      (tile) => tile.coordinates.q === 0 && tile.coordinates.r === 0,
    )!.occupiedBy = { type: "ocean-tile", tags: [] };
    const emptyPixel = await center();
    broadcast(JSON.stringify({ type: "game-updated", payload: { game: placed } }));
    await expect.poll(occupant).toBe("ocean-tile");
    await page.screenshot({
      path: path.resolve(`../tmp/bevy-scaffold/phase4-ocean-${backend()}.png`),
    });
    await expectTerrain(2, emptyPixel);
    await canvas.click({ position: { x: 720, y: 500 } });
    await expect(page.getByRole("tooltip")).toContainText("Ocean");
    // Reconnect installs a full snapshot through the same existing service event.
    await page.evaluate(async () => {
      const path = "/src/stores/gameStore.ts";
      const { useGameStore } = await import(/* @vite-ignore */ path);
      useGameStore.getState().setIsReconnecting(true);
    });
    const reconnected = structuredClone(placed);
    reconnected.board.tiles.find(
      (tile) => tile.coordinates.q === 0 && tile.coordinates.r === 0,
    )!.occupiedBy = { type: "greenery-tile", tags: [] };
    const oceanPixel = await center();
    broadcast(JSON.stringify({ type: "game-updated", payload: { game: reconnected } }));
    await expectTerrain(3, oceanPixel);
    await expect(page.getByRole("tooltip")).toContainText("Greenery");
    await page.evaluate(async (game) => {
      const gamePath = "/src/stores/gameStore.ts";
      const phasePath = "/src/stores/appPhaseStore.ts";
      const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
        import(/* @vite-ignore */ gamePath),
        import(/* @vite-ignore */ phasePath),
      ]);
      useGameStore.getState().setGame(game);
      useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
    }, sceneGame("replacement-game"));
    await ready();
    await expect.poll(occupant).toBeNull();
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    await page.evaluate(async () => {
      const path = "/src/stores/appPhaseStore.ts";
      const { useAppPhaseStore } = await import(/* @vite-ignore */ path);
      useAppPhaseStore.getState().resetToMenu();
    });
    await ready();
    await page.screenshot({
      path: path.resolve(`../tmp/bevy-scaffold/${backend()}-bridge-cleared.png`),
    });
    expect(await page.locator("canvas").count()).toBe(1);
  });

  test("keeps spherical picking aligned with HUD, camera gestures and stable tile entities", async ({
    page,
  }) => {
    await page.goto(engineUrl());
    const ready = () =>
      expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready", {
        timeout: 90_000,
      });
    await ready();
    await page.evaluate(async (game) => {
      const gamePath = "/src/stores/gameStore.ts";
      const phasePath = "/src/stores/appPhaseStore.ts";
      const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
        import(/* @vite-ignore */ gamePath),
        import(/* @vite-ignore */ phasePath),
      ]);
      useGameStore.getState().setGame(game);
      useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
      for (const [name, value] of [
        ["left-w", 210],
        ["right-w", 80],
        ["top-h", 50],
        ["dock-h", 110],
      ] as const) {
        const key = name.endsWith("-w") ? `--hud-rail-${name}` : `--hud-${name}`;
        document.documentElement.style.setProperty(key, `${value}px`);
      }
    }, sceneGame());
    await ready();
    const diagnostic = (): Promise<SceneDiagnostics> =>
      page.evaluate(async () => {
        const path = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ path);
        return useEngineStore.getState().inspectScene!()!;
      });
    const center = async () => (await diagnostic()).tiles.find((tile) => tile.id === "0,0,0")!;
    await expect.poll(async () => (await center()).point?.[0]).toBeCloseTo(705, 0);
    await expect.poll(async () => (await center()).point?.[1]).toBeCloseTo(330, 0);
    const initial = await center();
    await expect(page.getByRole("status")).toBeHidden();
    const target = initial.point!;
    await page.mouse.move(target[0], target[1]);
    await expect(page.getByRole("tooltip")).toBeVisible();
    await expect.poll(async () => (await diagnostic()).camera.hovered).toBe("0,0,0");
    await page.mouse.down();
    await page.mouse.move(target[0] + 70, target[1] + 25, { steps: 8 });
    await page.mouse.up();
    await expect.poll(async () => Math.abs((await diagnostic()).camera.yaw)).toBeGreaterThan(0.1);
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    const rotated = (await center()).point!;
    expect(Math.abs(rotated[0] - target[0])).toBeGreaterThan(10);
    await page.mouse.move(rotated[0], rotated[1]);
    await expect.poll(async () => (await diagnostic()).camera.hovered).toBe("0,0,0");
    await page.mouse.wheel(0, -360);
    await expect.poll(async () => (await diagnostic()).camera.distance).toBeLessThan(6.0);
    await page.locator("canvas").focus();
    await page.keyboard.press("Home");
    await expect.poll(async () => (await diagnostic()).camera.distance).toBeCloseTo(6.8);
    await expect.poll(async () => (await diagnostic()).camera.yaw).toBe(0);
    await expect.poll(async () => (await center()).point?.[0]).toBeCloseTo(target[0], 0);
    await page.evaluate(async () => {
      const path = "/src/stores/gameStore.ts";
      const { useGameStore } = await import(/* @vite-ignore */ path);
      const game = structuredClone(useGameStore.getState().game!) as ReturnType<typeof sceneGame>;
      game.board.tiles.find(
        (tile) => tile.coordinates.q === 0 && tile.coordinates.r === 0,
      )!.occupiedBy = { type: "city-tile", tags: [] };
      useGameStore.getState().setGame(game);
    });
    await expect.poll(async () => (await center()).occupant).toBe("city-tile");
    expect((await center()).entity).toBe(initial.entity);
    await ready();
    await expect(page.getByRole("status")).toBeHidden();
    await page.setViewportSize({ width: 1100, height: 700 });
    await expect.poll(async () => (await center()).point?.[0]).toBeCloseTo(615, 0);
    await ready();
    const resized = (await center()).point!;
    await page.mouse.move(resized[0], resized[1]);
    await expect.poll(async () => (await diagnostic()).camera.hovered).toBe("0,0,0");
    expect(
      await page.locator("canvas").evaluate((canvas) => (canvas as HTMLCanvasElement).width),
    ).toBeLessThanOrEqual(16384);
  });

  test("supports touch orbit, pinch zoom, cancellation and tap selection", async ({
    browser,
    baseURL,
  }) => {
    const context = await referenceContext(browser, {
      ...devices["iPhone 14 landscape"],
      baseURL,
    });
    const page = await context.newPage();
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 90_000 },
    );
    await page.evaluate(async (game) => {
      const gamePath = "/src/stores/gameStore.ts";
      const phasePath = "/src/stores/appPhaseStore.ts";
      const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
        import(/* @vite-ignore */ gamePath),
        import(/* @vite-ignore */ phasePath),
      ]);
      useGameStore.getState().setGame(game);
      useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
    }, sceneGame());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 90_000 },
    );
    const diagnostic = (): Promise<SceneDiagnostics> =>
      page.evaluate(async () => {
        const path = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ path);
        return useEngineStore.getState().inspectScene!()!;
      });
    const [x, y] = (await diagnostic()).tiles.find((tile) => tile.id === "0,0,0")!.point!;
    const input = await context.newCDPSession(page);
    await input.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [
        { id: 0, x: x - 40, y },
        { id: 1, x: x + 40, y },
      ],
    });
    await input.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [
        { id: 0, x: x - 70, y },
        { id: 1, x: x + 70, y },
      ],
    });
    await input.send("Input.dispatchTouchEvent", {
      type: "touchEnd",
      touchPoints: [],
    });
    await expect.poll(async () => (await diagnostic()).camera.distance).toBeLessThan(6.4);
    await expect(page.getByRole("button", { name: "Confirm", exact: true })).toHaveCount(0);
    await page.locator("canvas").focus();
    await page.keyboard.press("Home");
    await expect.poll(async () => (await diagnostic()).camera.distance).toBeCloseTo(6.8);
    await input.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ id: 0, x, y }],
    });
    await input.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [{ id: 0, x: x + 55, y: y + 15 }],
    });
    await input.send("Input.dispatchTouchEvent", {
      type: "touchCancel",
      touchPoints: [],
    });
    await expect.poll(async () => Math.abs((await diagnostic()).camera.yaw)).toBeGreaterThan(0.1);
    await expect(page.getByRole("button", { name: "Confirm", exact: true })).toHaveCount(0);
    await page.locator("canvas").focus();
    await page.keyboard.press("Home");
    await expect.poll(async () => (await diagnostic()).camera.yaw).toBe(0);
    await page.touchscreen.tap(x, y);
    await expect(page.getByRole("button", { name: "Confirm", exact: true })).toBeVisible();
    await expect(page.getByRole("group", { name: "Board camera" })).toHaveCount(0);
    await input.detach();
    await context.close();
  });

  for (const [name, profile] of [
    ["desktop", { viewport: { width: 1440, height: 1000 } }],
    ["iphone", devices["iPhone 14 landscape"]],
    ["android", devices["Pixel 7 landscape"]],
  ] as const) {
    test(`renders and resizes on ${name}`, async ({ browser, baseURL }) => {
      const context = await referenceContext(browser, { ...profile, baseURL });
      const page = await context.newPage();
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.goto(engineUrl());
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 90_000 },
      );
      const canvas = page.locator("canvas");
      await expect(canvas).toHaveCount(1);
      await expect(canvas).toHaveAttribute("data-engine-backend", backend());
      const capabilities = await page.evaluate(async () => {
        const path = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ path);
        return useEngineStore.getState().status.capabilities;
      });
      expect(capabilities.computeShaders).toBe(backend() === "webgpu");
      expect(capabilities.storageBuffers).toBe(backend() === "webgpu");
      expect(capabilities.maxTextureDimension2D).toBeGreaterThanOrEqual(2048);
      await expect(page.getByRole("status")).toBeHidden();
      const output = path.resolve("../tmp/bevy-scaffold");
      await mkdir(output, { recursive: true });
      await page.screenshot({
        path: path.join(output, `${backend()}-${name}.png`),
      });
      const { data, info } = await sharp(await canvas.screenshot())
        .removeAlpha()
        .raw()
        .toBuffer({ resolveWithObject: true });
      let brightPixels = 0;
      for (let index = 0; index < data.length; index += info.channels) {
        if (data[index] > 180 && data[index + 1] > 180 && data[index + 2] > 180) brightPixels++;
      }
      expect(brightPixels).toBeGreaterThan(100);
      const game = sceneGame();
      for (const [q, type] of [
        [-1, "ocean-tile"],
        [0, "greenery-tile"],
        [1, "city-tile"],
      ] as const) {
        game.board.tiles.find(
          (tile) => tile.coordinates.q === q && tile.coordinates.r === 0,
        )!.occupiedBy = { type, tags: [] };
      }
      await page.evaluate(async (game) => {
        const gamePath = "/src/stores/gameStore.ts";
        const phasePath = "/src/stores/appPhaseStore.ts";
        const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
          import(/* @vite-ignore */ gamePath),
          import(/* @vite-ignore */ phasePath),
        ]);
        useGameStore.getState().setGame(game);
        useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
      }, game);
      await expect(page.locator("[data-engine-state]")).toHaveAttribute(
        "data-engine-state",
        "ready",
        { timeout: 90_000 },
      );
      await expect(page.getByRole("status")).toBeHidden();
      if (name !== "desktop") {
        await expect(page.getByRole("img", { name: "greenery-tile", exact: true })).toBeVisible();
      }
      await page.screenshot({
        path: path.join(output, `${backend()}-bridge-${name}.png`),
      });
      await page.setViewportSize({ width: 1100, height: 700 });
      await expect
        .poll(() => canvas.evaluate((element) => (element as HTMLCanvasElement).width))
        .toBeGreaterThan(1000);
      expect(errors).toEqual([]);
      await context.close();
    });
  }

  test("falls back when the WebGPU API is unavailable", async ({ page }) => {
    await page.addInitScript(() => Object.defineProperty(navigator, "gpu", { value: undefined }));
    await page.goto("/dev/engine");
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    await expect(page.locator("canvas")).toHaveAttribute("data-engine-backend", "webgl2");
    await expect(page.locator("canvas")).toHaveCount(1);
  });

  test("prefers WebGPU when an adapter is usable", async ({ page }) => {
    await page.goto("/dev/engine");
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    await expect(page.locator("canvas")).toHaveAttribute("data-engine-backend", "webgpu");
  });

  test("cancels initialization without a late canvas or callback", async ({ page }) => {
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    const result = await page.evaluate(async (renderer) => {
      const path = "/src/engine/runtime.ts";
      const { startEngine } = await import(/* @vite-ignore */ path);
      const host = document.createElement("div");
      document.body.append(host);
      const states: string[] = [];
      const runtime = startEngine(
        host,
        (status: { phase: string }) => states.push(status.phase),
        renderer,
      );
      runtime.dispose();
      const count = states.length;
      await new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      );
      const result = {
        unchanged: count === states.length,
        canvases: host.childElementCount,
      };
      host.remove();
      return result;
    }, backend());
    expect(result).toEqual({ unchanged: true, canvases: 0 });
  });

  test("falls back when WebGPU cannot supply an adapter", async ({ page }) => {
    await page.addInitScript(() => {
      const gpu = (navigator as Navigator & { gpu: { requestAdapter: unknown } }).gpu;
      Object.defineProperty(gpu, "requestAdapter", {
        configurable: true,
        value: async () => null,
      });
    });
    await page.goto("/dev/engine");
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    await expect(page.locator("canvas")).toHaveAttribute("data-engine-backend", "webgl2");
    await expect(page.getByRole("alert")).toHaveCount(0);
  });

  test("reports device loss and recreates a fresh renderer on retry", async ({ page }) => {
    test.skip(backend() !== "webgpu", "GPUDevice.destroy targets WebGPU");
    await page.addInitScript(() => {
      const request = GPUAdapter.prototype.requestDevice;
      GPUAdapter.prototype.requestDevice = async function (...args) {
        const device = await request.apply(this, args);
        (window as Window & { loseEngineDevice?: () => void }).loseEngineDevice = () =>
          device.destroy();
        return device;
      };
    });
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 90_000 },
    );
    const first = await page.locator("canvas").elementHandle();
    await page.evaluate(() =>
      (window as Window & { loseEngineDevice?: () => void }).loseEngineDevice!(),
    );
    await expect(page.getByRole("alert")).toBeVisible({ timeout: 15_000 });
    await page.getByRole("button", { name: "Try again", exact: true }).click();
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 90_000 },
    );
    expect(await first!.evaluate((element) => element.isConnected)).toBe(false);
    await expect(page.locator("canvas")).toHaveCount(1);
  });

  test("reports forced backend failure and retries", async ({ page }) => {
    await page.addInitScript(() => {
      const gpu = (navigator as Navigator & { gpu: { requestAdapter: unknown } }).gpu;
      Object.defineProperty(gpu, "requestAdapter", {
        configurable: true,
        value: async () => null,
      });
    });
    await page.goto("/dev/engine?renderer=webgpu");
    await expect(page.getByRole("alert")).toContainText("graphics adapter");
    await page.evaluate(() => Reflect.deleteProperty(navigator.gpu, "requestAdapter"));
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    await expect(page.locator("canvas")).toHaveAttribute("data-engine-backend", "webgpu");
  });

  test("reports when neither backend is available", async ({ page }) => {
    await page.addInitScript(() => {
      Object.defineProperty(navigator, "gpu", { value: undefined });
      const getContext = HTMLCanvasElement.prototype.getContext;
      HTMLCanvasElement.prototype.getContext = function (
        this: HTMLCanvasElement,
        ...args: Parameters<typeof getContext>
      ) {
        if (String(args[0]).startsWith("webgl")) return null;
        return getContext.apply(this, args);
      } as typeof getContext;
    });
    await page.goto("/dev/engine");
    await expect(page.getByRole("alert")).toContainText("webgpu:");
    await expect(page.getByRole("alert")).toContainText("webgl2:");
    await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
    const output = path.resolve("../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    await page.screenshot({ path: path.join(output, "unavailable.png") });
  });

  test("disposes and recreates the renderer without stale callbacks", async ({ page }) => {
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    const results = await page.evaluate(async (renderer) => {
      const path = "/src/engine/runtime.ts";
      const { startEngine } = await import(/* @vite-ignore */ path);
      const results: boolean[] = [];
      for (let cycle = 0; cycle < 3; cycle++) {
        const canvas = document.createElement("div");
        canvas.style.cssText = "position:fixed;width:200px;height:120px";
        document.body.append(canvas);
        const states: string[] = [];
        let finish: () => void;
        let fail: (error: Error) => void;
        const ready = new Promise<void>((resolve, reject) => {
          finish = resolve;
          fail = reject;
        });
        const runtime = startEngine(
          canvas,
          (status: { phase: string; message: string }) => {
            states.push(status.phase);
            if (status.phase === "ready") finish();
            if (status.phase === "error") fail(new Error(status.message));
          },
          renderer,
        );
        try {
          await ready;
        } finally {
          runtime.dispose();
          canvas.remove();
        }
        const count = states.length;
        await new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        );
        results.push(states.length === count && states.at(-1) === "ready");
      }
      return results;
    }, backend());
    expect(results).toEqual([true, true, true]);
    await expect(page.locator("canvas")).toHaveCount(1);
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
  });

  test("bounds submissions while GPU completion is pending", async ({ page }) => {
    test.skip(backend() !== "webgpu", "GPUQueue completion hook targets WebGPU");
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 90_000 },
    );
    const result = await page.evaluate(async () => {
      const originalSubmit = GPUQueue.prototype.submit;
      const originalComplete = GPUQueue.prototype.onSubmittedWorkDone;
      let submits = 0;
      let held = true;
      const releases: (() => void)[] = [];
      GPUQueue.prototype.submit = function (...args) {
        submits++;
        originalSubmit.apply(this, args);
      };
      GPUQueue.prototype.onSubmittedWorkDone = function () {
        return originalComplete
          .call(this)
          .then(() => (held ? new Promise<void>((resolve) => releases.push(resolve)) : undefined));
      };
      const frames = async (count: number) => {
        for (let i = 0; i < count; i++) {
          await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
        }
      };
      try {
        await frames(8);
        const pending = submits;
        await frames(8);
        const bounded = submits === pending && pending > 0;
        held = false;
        for (const release of releases) {
          release();
        }
        await frames(8);
        return { bounded, resumed: submits > pending };
      } finally {
        held = false;
        for (const release of releases) {
          release();
        }
        GPUQueue.prototype.submit = originalSubmit;
        GPUQueue.prototype.onSubmittedWorkDone = originalComplete;
      }
    });
    expect(result).toEqual({ bounded: true, resumed: true });
  });

  test("publishes completed terrain bakes and cancels obsolete stripes", async ({ page }) => {
    if (backend() === "webgpu") {
      await page.addInitScript(() => {
        const live = new Set<GPUBuffer>();
        const metrics = {
          created: 0,
          destroyed: 0,
          live: 0,
          peak: 0,
          hdrAttachments: 0,
          targetsCreated: 0,
          targetsDestroyed: 0,
          liveTargets: 0,
          peakTargets: 0,
        };
        const targets = new Set<GPUTexture>();
        const createTexture = GPUDevice.prototype.createTexture;
        const destroyTexture = GPUTexture.prototype.destroy;
        GPUDevice.prototype.createTexture = function (descriptor) {
          const texture = createTexture.call(this, descriptor);
          const size = descriptor.size as GPUExtent3DDict;
          if (
            size.width === 2048 &&
            size.height === 1024 &&
            descriptor.usage &
              (Reflect.get(globalThis, "GPUTextureUsage") as { RENDER_ATTACHMENT: number })
                .RENDER_ATTACHMENT
          ) {
            if (
              ["main_texture_a", "main_texture_b", "view_depth_texture"].includes(
                descriptor.label ?? "",
              )
            ) {
              metrics.hdrAttachments++;
            }
            if (descriptor.format === "rgba8unorm" || descriptor.format === "rgba8unorm-srgb") {
              targets.add(texture);
              metrics.targetsCreated++;
              metrics.liveTargets = targets.size;
              metrics.peakTargets = Math.max(metrics.peakTargets, metrics.liveTargets);
            }
          }
          return texture;
        };
        GPUTexture.prototype.destroy = function () {
          if (targets.delete(this)) {
            metrics.targetsDestroyed++;
            metrics.liveTargets = targets.size;
          }
          destroyTexture.call(this);
        };
        Reflect.set(window, "bakeBuffers", metrics);
        const create = GPUDevice.prototype.createBuffer;
        const destroy = GPUBuffer.prototype.destroy;
        GPUDevice.prototype.createBuffer = function (descriptor) {
          const buffer = create.call(this, descriptor);
          if (/^terrain direct bake parameters$/.test(descriptor.label ?? "")) {
            live.add(buffer);
            metrics.created++;
            metrics.live = live.size;
            metrics.peak = Math.max(metrics.peak, metrics.live);
          }
          return buffer;
        };
        GPUBuffer.prototype.destroy = function () {
          if (live.delete(this)) {
            metrics.destroyed++;
            metrics.live = live.size;
          }
          destroy.call(this);
        };
      });
    }
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    const game = sceneGame("terrain-bakes");
    for (const [q, type] of [
      [-1, "ocean-tile"],
      [0, "greenery-tile"],
      [1, "city-tile"],
    ] as const) {
      game.board.tiles.find(
        (tile) => tile.coordinates.q === q && tile.coordinates.r === 0,
      )!.occupiedBy = { type, tags: [] };
    }
    game.currentPlayer.pendingTileSelection = undefined;
    const result = await page.evaluate(
      async ({ renderer, game }) => {
        const runtimePath = "/src/engine/runtime.ts";
        const scenePath = "/src/engine/scene.ts";
        const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
          import(/* @vite-ignore */ runtimePath),
          import(/* @vite-ignore */ scenePath),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0;width:800px;height:600px";
        document.body.append(host);
        let resolveReady = () => {};
        let rejectReady = (_error: Error) => {};
        let failure: Error | null = null;
        const nextReady = () =>
          new Promise<void>((resolve, reject) => {
            resolveReady = resolve;
            rejectReady = reject;
          });
        const first = nextReady();
        const runtime = startEngine(
          host,
          (status: { phase: string; message: string }) => {
            if (status.phase === "ready") resolveReady();
            if (status.phase === "error") {
              failure = new Error(status.message);
              rejectReady(failure);
            }
          },
          renderer,
        );
        const frames = async (count: number) => {
          for (let i = 0; i < count; i++) {
            if (failure) throw failure;
            await new Promise(requestAnimationFrame);
          }
        };
        const until = async (condition: () => boolean) => {
          for (let i = 0; i < 4000; i++) {
            if (condition()) return;
            await frames(1);
          }
          throw new Error("Bake did not settle: " + JSON.stringify(runtime.assetStats()));
        };
        const capture = async (condition: (stats: Record<string, number>) => boolean) => {
          let writes = runtime.assetStats().terrainUniformWrites;
          await until(() => {
            const stats = runtime.assetStats();
            const fresh = stats.terrainUniformWrites > writes;
            writes = stats.terrainUniformWrites;
            return fresh && condition(stats);
          });
          return host.querySelector("canvas")!.toDataURL("image/png");
        };
        const mark = (stage: string) => {
          const stats = runtime.assetStats();
          console.info(
            "Bake lifecycle " +
              stage +
              " " +
              JSON.stringify({
                step: stats.terrainStep,
                revision: stats.terrainRevision,
                valid: stats.bakeValid,
                publications: stats.bakePublications,
                strip: stats.bakeStrip,
                cancellations: stats.bakeCancellations,
              }),
          );
        };
        try {
          await first;
          const baseline = runtime.assetStats();
          const loading = nextReady();
          runtime.setScene(toSceneSnapshot(game, null));
          await loading;
          mark("initial ready");
          const before = await capture((stats) => stats.bakeStrip > 0 && stats.bakeValid === 0);
          await until(
            () => runtime.assetStats().bakeValid === 1 && runtime.assetStats().bakeUniforms === 1,
          );
          const baked = runtime.assetStats();
          mark("initial published");
          const after = await capture((stats) => stats.bakeValid === 1);
          const stationary = runtime.assetStats();
          await frames(12);
          const stillStationary = runtime.assetStats();
          runtime.setScene(toSceneSnapshot(game, "0,0,0"));
          await until(
            () =>
              runtime.assetStats().gridTextureWrites > baked.gridTextureWrites &&
              runtime.assetStats().terrainUniformWrites >= baked.terrainUniformWrites + 2,
          );
          const highlighted = runtime.assetStats();
          mark("highlight uploaded");
          const tile = game.board.tiles.find(
            (tile) => tile.coordinates.q === 1 && tile.coordinates.r === 0,
          )!;
          tile.occupiedBy = { type: "ocean-tile", tags: [] };
          runtime.setScene(toSceneSnapshot(game, null));
          await until(
            () =>
              runtime.assetStats().bakeStrip > 0 &&
              runtime.assetStats().terrainRevision === 2 &&
              runtime.assetStats().bakeStripes > baked.bakeStripes,
          );
          const drawing = runtime.assetStats();
          mark("replacement stripes");
          runtime.setPaused(true);
          await frames(2);
          const paused = runtime.assetStats();
          await frames(8);
          const stillPaused = runtime.assetStats();
          mark("paused stable");
          tile.occupiedBy = { type: "greenery-tile", tags: [] };
          // The published field remains valid while replacement geometry prepares.
          // A real ground-climate edge makes these in-flight stripes obsolete now.
          game.globalParameters.temperature = 8;
          runtime.setScene(toSceneSnapshot(game, null));
          runtime.setPaused(false);
          await until(
            () =>
              runtime.assetStats().bakeCancellations > drawing.bakeCancellations &&
              runtime.assetStats().bakeValid === 0,
          );
          const cancelled = runtime.assetStats();
          mark("obsolete cancelled");
          game.globalParameters.temperature = -30;
          runtime.setScene(toSceneSnapshot(game, null));
          await until(
            () =>
              runtime.assetStats().bakeRevision === 3 &&
              runtime.assetStats().bakeValid === 1 &&
              runtime.assetStats().bakeUniforms === 1,
          );
          const replaced = runtime.assetStats();
          mark("replacement published");
          game.globalParameters.temperature = 8;
          game.globalParameters.oxygen = 14;
          game.globalParameters.oceans = 9;
          runtime.setScene(toSceneSnapshot(game, null));
          const warmBefore = await capture(
            (stats) => stats.bakeStrip > 0 && stats.bakePublications === replaced.bakePublications,
          );
          mark("warm stripes");
          await until(
            () => runtime.assetStats().bakeValid === 1 && runtime.assetStats().bakeUniforms === 1,
          );
          const warm = runtime.assetStats();
          mark("warm published");
          const warmAfter = await capture((stats) => stats.bakeValid === 1);
          Reflect.set(window, "bakeCapture", {
            before,
            after,
            warmBefore,
            warmAfter,
          });
          const clearing = nextReady();
          runtime.setScene(null);
          await clearing;
          await until(
            () =>
              runtime.assetStats().gpuImages === baseline.gpuImages &&
              runtime.assetStats().gpuMaterials === baseline.gpuMaterials &&
              runtime.assetStats().bakeUniforms === 0,
          );
          mark("cleared");
          return {
            baked,
            highlighted,
            stationary,
            stillStationary,
            drawing,
            paused,
            stillPaused,
            cancelled,
            replaced,
            warm,
            cleared: runtime.assetStats(),
            buffers: renderer === "webgpu" ? Reflect.get(window, "bakeBuffers") : null,
          };
        } finally {
          runtime.dispose();
          host.remove();
        }
      },
      { renderer: backend(), game },
    );
    expect(result.baked.bakeWidth).toBe(2048);
    expect(result.baked.bakeHeight).toBe(1024);
    // Startup may publish a valid earlier shade/view cache while other owners warm.
    // Measure controlled changes relative to the completed current cache.
    expect(result.baked.bakeStripes).toBeGreaterThanOrEqual(4);
    expect(result.baked.bakeMipDraws).toBeGreaterThanOrEqual(24);
    expect(result.baked.bakePublications).toBeGreaterThanOrEqual(1);
    expect(result.baked.bakeUniforms).toBe(1);
    expect(result.baked.bakeDrawCalls).toBe(result.baked.bakeStripes * 2);
    expect(result.baked.bakeViewCacheHits).toBeGreaterThan(0);
    expect(result.stillStationary.bakeViewMeasurements).toBe(
      result.stationary.bakeViewMeasurements,
    );
    expect(result.stillStationary.bakeViewCacheHits).toBeGreaterThan(
      result.stationary.bakeViewCacheHits,
    );
    expect(result.highlighted.bakePublications).toBe(result.baked.bakePublications);
    expect(result.highlighted.bakeMipDraws).toBe(result.baked.bakeMipDraws);
    expect(result.paused.bakeUniformWrites).toBe(result.stillPaused.bakeUniformWrites);
    expect(result.paused.bakeStripes).toBe(result.stillPaused.bakeStripes);
    expect(result.cancelled.bakeValid).toBe(0);
    expect(result.cancelled.bakeCancellations).toBeGreaterThan(result.drawing.bakeCancellations);
    expect(result.replaced.bakeRevision).toBe(3);
    expect(result.replaced.bakePublications).toBeGreaterThan(result.baked.bakePublications);
    expect(result.warm.bakeRevision).toBe(3);
    expect(result.warm.bakePublications).toBe(result.replaced.bakePublications + 1);
    expect(result.warm.bakeMipDraws).toBe(result.replaced.bakeMipDraws + 24);
    expect(result.warm.bakeUniforms).toBe(1);
    expect(result.warm.bakeDrawCalls).toBe(result.replaced.bakeDrawCalls + 8);
    expect(result.cleared.bakeValid).toBe(0);
    expect(result.cleared.bakeUniforms).toBe(0);
    if (backend() === "webgpu") {
      expect(result.buffers.live).toBe(0);
      expect(result.buffers.created).toBe(result.buffers.destroyed);
      expect(result.buffers.created).toBe(1);
      expect(result.buffers.peak).toBe(1);
      expect(result.buffers.hdrAttachments).toBe(0);
      expect(result.buffers.targetsCreated).toBe(4);
      expect(result.buffers.targetsDestroyed).toBe(4);
      expect(result.buffers.liveTargets).toBe(0);
      expect(result.buffers.peakTargets).toBe(4);
    }
    const captures = await page.evaluate(
      () =>
        Reflect.get(window, "bakeCapture") as {
          before: string;
          after: string;
          warmBefore: string;
          warmAfter: string;
        },
    );
    const { writeFile } = await import("node:fs/promises");
    const pixels: Buffer[] = [];
    for (const [name, data] of Object.entries(captures)) {
      const bytes = Buffer.from(data.split(",")[1], "base64");
      await writeFile(
        path.resolve(`../tmp/bevy-scaffold/phase5-bake-${backend()}-${name}.png`),
        bytes,
      );
      const crop = await sharp(bytes)
        .extract({ left: 270, top: 240, width: 260, height: 120 })
        .removeAlpha()
        .raw()
        .toBuffer();
      pixels.push(crop);
    }
    const differences: number[] = [];
    for (let pair = 0; pair < 2; pair++) {
      let difference = 0;
      const before = pixels[pair * 2];
      const after = pixels[pair * 2 + 1];
      for (let i = 0; i < before.length; i++) difference += Math.abs(before[i] - after[i]);
      differences.push(difference / before.length);
      expect(difference / before.length).toBeLessThan(2);
    }
    await writeFile(
      path.resolve(`../tmp/bevy-scaffold/phase5-bake-${backend()}-metrics.json`),
      JSON.stringify({ differences, ...result }, null, 2),
    );
  });

  test("publishes live landscape replacements without starving additive work", async ({ page }) => {
    if (backend() === "webgpu") await trackTerrainBuffers(page);
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    const game = sceneGame("live-landscape");
    game.globalParameters.temperature = 8;
    game.globalParameters.oxygen = 14;
    game.globalParameters.oceans = 9;
    game.board.tiles.find(
      (tile) => tile.coordinates.q === 0 && tile.coordinates.r === 0,
    )!.occupiedBy = { type: "greenery-tile", tags: [] };
    const result = await page.evaluate(
      async ({ renderer, game }) => {
        const runtimePath = "/src/engine/runtime.ts";
        const scenePath = "/src/engine/scene.ts";
        const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
          import(/* @vite-ignore */ runtimePath),
          import(/* @vite-ignore */ scenePath),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0;width:800px;height:600px";
        document.body.append(host);
        let resolveReady = () => {};
        let rejectReady = (_error: Error) => {};
        let failure: Error | null = null;
        const nextReady = () =>
          new Promise<void>((resolve, reject) => {
            resolveReady = resolve;
            rejectReady = reject;
          });
        const first = nextReady();
        const runtime = startEngine(
          host,
          (status: { phase: string; message: string }) => {
            if (status.phase === "ready") resolveReady();
            if (status.phase === "error") {
              failure = new Error(status.message);
              rejectReady(failure);
            }
          },
          renderer,
        );
        const frames = async (count: number) => {
          for (let i = 0; i < count; i++) {
            if (failure) throw failure;
            await new Promise(requestAnimationFrame);
          }
        };
        const until = async (condition: () => boolean) => {
          for (let i = 0; i < 2400; i++) {
            if (condition()) return;
            await frames(1);
          }
          throw new Error(
            "Terrain publication did not settle: " + JSON.stringify(runtime.assetStats()),
          );
        };
        const tile = (q: number) =>
          game.board.tiles.find((tile) => tile.coordinates.q === q && tile.coordinates.r === 0)!;
        const metrics = () =>
          renderer === "webgpu" ? { ...Reflect.get(window, "terrainBuffers") } : null;
        try {
          await first;
          const loading = nextReady();
          runtime.setScene(toSceneSnapshot(game, null));
          await loading;
          const initial = runtime.assetStats();
          tile(1).occupiedBy = { type: "ocean-tile", tags: [] };
          runtime.setScene(toSceneSnapshot(game, null));
          await until(
            () =>
              runtime.assetStats().terrainRevision === 2 &&
              runtime.assetStats().terrainTransitioning === 0 &&
              runtime.assetStats().terrainUniforms === 2,
          );
          const replaced = runtime.assetStats();
          tile(1).occupiedBy = undefined;
          runtime.setScene(toSceneSnapshot(game, null));
          await until(() => runtime.assetStats().terrainRequestedRevision === 3);
          const preparing = runtime.assetStats();
          tile(-1).occupiedBy = { type: "ocean-tile", tags: [] };
          runtime.setScene(toSceneSnapshot(game, null));
          // The new addition queues behind the already prepared removal.
          // Continuing to place must not cancel useful in-flight publications.
          await until(() => runtime.assetStats().terrainRevision === 3);
          const intermediate = runtime.assetStats();
          await until(
            () =>
              runtime.assetStats().terrainRevision === 4 &&
              runtime.assetStats().terrainTransitioning === 0 &&
              runtime.assetStats().terrainUniforms === 2,
          );
          const final = runtime.assetStats();
          const buffers = metrics();
          const gridBeforeSelection = runtime.assetStats();
          runtime.setScene(toSceneSnapshot(game, "0,0,0"));
          await frames(15);
          const selected = runtime.assetStats();
          const beforeResize = runtime.assetStats().terrainUniformWrites;
          host.style.width = "1100px";
          host.style.height = "700px";
          await until(() => runtime.assetStats().terrainUniformWrites >= beforeResize + 3);
          const resized = runtime.assetStats();
          const afterResize = metrics();
          Reflect.set(
            window,
            "terrainLiveCapture",
            host.querySelector("canvas")!.toDataURL("image/png"),
          );
          const clearing = nextReady();
          runtime.setScene(null);
          await clearing;
          await frames(3);
          return {
            initial,
            replaced,
            preparing,
            intermediate,
            final,
            selected,
            resized,
            gridBeforeSelection,
            gridBuffers: renderer === "webgpu" ? Reflect.get(window, "gridBuffers") : null,
            gridTextures: renderer === "webgpu" ? Reflect.get(window, "gridTextures") : null,
            buffers,
            afterResize,
            retired: metrics(),
            cleared: runtime.assetStats(),
          };
        } finally {
          runtime.dispose();
          host.remove();
        }
      },
      { renderer: backend(), game },
    );
    expect(result.initial.terrainRevision).toBe(1);
    expect(result.initial.terrainPatches).toBeGreaterThan(0);
    expect(result.replaced.terrainRevision).toBe(2);
    expect(result.preparing.terrainRevision).toBe(2);
    expect(result.intermediate.terrainRevision).toBe(3);
    expect(result.intermediate.terrainBatches).toBeLessThanOrEqual(2);
    expect(result.final.terrainRevision).toBe(4);
    expect(result.final.terrainRetiredUniforms).toBe(6);
    expect(result.selected.gridMeshBuilds).toBe(result.gridBeforeSelection.gridMeshBuilds);
    expect(result.selected.gridUniforms).toBe(1);
    expect(result.selected.gridTextureWrites).toBeGreaterThan(
      result.gridBeforeSelection.gridTextureWrites,
    );
    expect(result.resized.gridMeshBuilds).toBe(result.selected.gridMeshBuilds);
    expect(result.cleared.gridUniforms).toBe(0);
    expect(result.selected.terrainRequestedRevision).toBe(4);
    expect(result.selected.terrainRevision).toBe(4);
    expect(result.resized.terrainRevision).toBe(4);
    expect(result.cleared.terrainBatches).toBe(0);
    expect(result.cleared.terrainUniforms).toBe(0);
    expect(result.cleared.terrainRetiredUniforms).toBe(8);
    if (backend() === "webgpu") {
      expect(result.gridBuffers).toEqual({ created: 1, destroyed: 1, live: 0 });
      expect(result.gridTextures?.created).toBe(1);
      expect(result.gridTextures!.writes).toBeGreaterThan(0);
      expect(result.buffers).toEqual({ created: 8, destroyed: 6, live: 2 });
      expect(result.afterResize).toEqual(result.buffers);
      expect(result.retired).toEqual({ created: 8, destroyed: 8, live: 0 });
    }
    const dataURL = await page.evaluate(() => Reflect.get(window, "terrainLiveCapture") as string);
    await mkdir(path.resolve("../tmp/bevy-scaffold"), { recursive: true });
    await import("node:fs/promises").then(({ writeFile }) =>
      writeFile(
        path.resolve("../tmp/bevy-scaffold", "phase5-live-" + backend() + "-warm.png"),
        Buffer.from(dataURL.split(",")[1], "base64"),
      ),
    );
  });

  test("publishes rapid placements through the terrain worker without waiting for silence", async ({
    page,
  }) => {
    test.setTimeout(180_000);
    await page.goto("/tests/fixtures/engine.html");
    const game = sceneGame("rapid-worker-publication");
    game.globalParameters.temperature = 8;
    game.globalParameters.oxygen = 14;
    game.globalParameters.oceans = 9;
    game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === 0)!.occupiedBy = {
      type: "city-tile",
      tags: [],
      visual: { seed: 17 },
    };
    const result = await page.evaluate(
      async ({ game, renderer }) => {
        const runtimePath = "/src/engine/runtime.ts",
          scenePath = "/src/engine/scene.ts",
          profilerPath = "/src/engine/profiler.ts";
        const [{ startEngine }, { toSceneSnapshot }, { engineProfiler }] = await Promise.all([
          import(/* @vite-ignore */ runtimePath),
          import(/* @vite-ignore */ scenePath),
          import(/* @vite-ignore */ profilerPath),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        let phase = "starting",
          failure: string | null = null;
        const runtime = startEngine(
          host,
          (status: { phase: string; message?: string }) => {
            phase = status.phase;
            if (phase === "error") failure = status.message || "Renderer failed";
          },
          renderer,
        );
        const frames = async (count = 1) => {
          for (let i = 0; i < count; i++) await new Promise(requestAnimationFrame);
        };
        const until = async (condition: () => boolean) => {
          const deadline = performance.now() + 100_000;
          while (!condition()) {
            if (failure) throw new Error(failure);
            if (performance.now() > deadline)
              throw new Error("Terrain worker deadline: " + JSON.stringify(runtime.assetStats()));
            await frames();
          }
        };
        try {
          await until(() => phase === "ready");
          runtime.setScene(toSceneSnapshot(game, null), "snapshot");
          // The initial snapshot is intentionally quiet until its GPU acknowledgement.
          // Begin live placements after that baseline has finished settling.
          await until(
            () =>
              phase === "ready" &&
              runtime.sceneStats()?.boardReady &&
              !runtime.sceneStats()?.settling &&
              !runtime.sceneStats()?.terrain.transitioning,
          );
          engineProfiler.start("reload");
          const coordinates = [
            [-1, 0],
            [1, 0],
            [0, -1],
            [0, 1],
            [-1, 1],
            [1, -1],
            [-2, 0],
            [2, 0],
            [0, 2],
            [0, -2],
          ];
          for (let i = 0; i < coordinates.length; i++) {
            const [q, r] = coordinates[i];
            game.board.tiles.find(
              (t) => t.coordinates.q === q && t.coordinates.r === r,
            )!.occupiedBy = { type: "greenery-tile", tags: [], visual: { seed: 17 + i } };
            engineProfiler.mark("test.placement", { ordinal: i + 1, q, r });
            runtime.setScene(toSceneSnapshot(game, null), "live");
            await new Promise((resolve) => setTimeout(resolve, 500));
          }
          await until(
            () =>
              runtime.assetStats().terrainRevision === 11 &&
              runtime.sceneStats()?.boardReady &&
              !runtime.sceneStats()?.terrain.transitioning,
          );
          await frames(5);
          engineProfiler.stop();
          return engineProfiler.export();
        } finally {
          engineProfiler.stop();
          runtime.dispose();
          host.remove();
        }
      },
      { game, renderer: backend() },
    );
    type Event = { name: string; ts: number; args?: Record<string, unknown> };
    const events = result.traceEvents as Event[];
    const placements = events.filter((e) => e.name === "test.placement");
    const generated = events.filter((e) => e.name === "terrain.worker.generate");
    const published = events.filter((e) => e.name === "terrain.publish");
    const animated = events.filter((e) => e.name === "vegetation.animation.frame");
    expect(placements).toHaveLength(10);
    expect(generated).toHaveLength(10);
    expect(published).toHaveLength(10);
    const output = path.resolve("../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    await writeFile(
      path.join(output, `rapid-placements-${backend()}.json`),
      JSON.stringify(result),
    );
    expect(generated.some((e) => Number(e.args?.patchesReused) > 0)).toBe(true);
    for (const [index, placement] of placements.entries()) {
      const frame = animated.find(
        (e) =>
          e.ts >= placement.ts &&
          e.args?.q === placement.args?.q &&
          e.args?.r === placement.args?.r,
      );
      expect(frame).toBeDefined();
      // Every placement must begin independently, before another human-paced action.
      expect(frame!.ts - placement.ts).toBeLessThan(500_000);
      if (index + 1 < placements.length) {
        expect(frame!.ts).toBeLessThan(placements[index + 1].ts);
      }
      const owner = published.find(
        (e) => e.args?.landscapeRevision === frame!.args?.landscapeRevision,
      );
      expect(frame!.args?.context).toBe(owner!.args?.context);
      const generation = generated.find(
        (e) => e.args?.landscapeRevision === frame!.args?.landscapeRevision,
      );
      expect(generation!.args?.context).toBe(owner!.args?.context);
    }
    const first = animated.find((e) => e.args?.q === -1 && e.args?.r === 0)!;
    expect(first.ts).toBeLessThan(placements.at(-1)!.ts);
  });

  test("reuses terrain uniforms and destroys them after scene retirement", async ({ page }) => {
    test.skip(backend() !== "webgpu", "Raw GPUBuffer instrumentation targets WebGPU");
    await trackTerrainBuffers(page);
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    const result = await page.evaluate(async (game) => {
      const runtimePath = "/src/engine/runtime.ts";
      const scenePath = "/src/engine/scene.ts";
      const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
        import(/* @vite-ignore */ runtimePath),
        import(/* @vite-ignore */ scenePath),
      ]);
      const host = document.createElement("div");
      host.style.cssText = "position:fixed;inset:0;width:800px;height:600px";
      document.body.append(host);
      let resolveReady = () => {};
      let rejectReady = (_error: Error) => {};
      const nextReady = () =>
        new Promise<void>((resolve, reject) => {
          resolveReady = resolve;
          rejectReady = reject;
        });
      const first = nextReady();
      const runtime = startEngine(
        host,
        (status: { phase: string; message: string }) => {
          if (status.phase === "ready") resolveReady();
          if (status.phase === "error") rejectReady(new Error(status.message));
        },
        "webgpu",
      );
      const frames = async (count: number) => {
        for (let i = 0; i < count; i++) await new Promise(requestAnimationFrame);
      };
      const metrics = () => ({ ...Reflect.get(window, "terrainBuffers") });
      try {
        await first;
        const loading = nextReady();
        runtime.setScene(toSceneSnapshot(game, null));
        await loading;
        const initial = metrics();
        const writes = runtime.assetStats().terrainUniformWrites;
        for (
          let frame = 0;
          frame < 240 && runtime.assetStats().terrainUniformWrites < writes + 12;
          frame++
        )
          await frames(1);
        const animated = metrics();
        const progressed = runtime.assetStats().terrainUniformWrites > writes;
        runtime.setPaused(true);
        await frames(3);
        const pausedStep = runtime.assetStats().terrainStep;
        await frames(20);
        const afterPause = runtime.assetStats().terrainStep;
        runtime.setPaused(false);
        await frames(1);
        const resumedStep = runtime.assetStats().terrainStep;
        const clearing = nextReady();
        runtime.setScene(null);
        await clearing;
        await frames(3);
        return {
          initial,
          animated,
          retired: metrics(),
          progressed,
          pausedStep,
          afterPause,
          resumedStep,
          stats: runtime.assetStats(),
        };
      } finally {
        runtime.dispose();
        host.remove();
      }
    }, sceneGame("terrain-buffer-game"));
    expect(result.initial).toEqual({ created: 2, destroyed: 0, live: 2 });
    expect(result.animated).toEqual(result.initial);
    expect(result.progressed).toBe(true);
    expect(result.afterPause).toBe(result.pausedStep);
    expect(result.resumedStep - result.afterPause).toBeLessThanOrEqual(1);
    expect(result.retired).toEqual({ created: 2, destroyed: 2, live: 0 });
    expect(result.stats.terrainBatches).toBe(0);
    expect(result.stats.terrainRetiredUniforms).toBe(2);
  });

  test("reuses lighting buffers during rendering and destroys them on disposal", async ({
    page,
  }) => {
    test.skip(backend() !== "webgpu", "GPU buffer ownership hook targets WebGPU");
    await page.addInitScript(() => {
      const buffers = new WeakSet<GPUBuffer>();
      const metrics = { created: 0, live: 0, peak: 0 };
      Reflect.set(window, "lightingBuffers", metrics);
      const create = GPUDevice.prototype.createBuffer;
      const destroy = GPUBuffer.prototype.destroy;
      GPUDevice.prototype.createBuffer = function (descriptor) {
        const buffer = create.call(this, descriptor);
        if (/GpuCluster(?:ableObjectIndexLists|OffsetsAndCounts)/.test(descriptor.label ?? "")) {
          buffers.add(buffer);
          metrics.created++;
          metrics.live++;
          metrics.peak = Math.max(metrics.peak, metrics.live);
        }
        return buffer;
      };
      GPUBuffer.prototype.destroy = function () {
        if (buffers.delete(this)) {
          metrics.live--;
        }
        destroy.call(this);
      };
    });
    await page.goto("/tests/fixtures/engine.html");
    await page.evaluate(
      async ({ renderer, game }) => {
        const runtimePath = "/src/engine/runtime.ts",
          scenePath = "/src/engine/scene.ts";
        const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
          import(/* @vite-ignore */ runtimePath),
          import(/* @vite-ignore */ scenePath),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        let status = { phase: "starting", message: "" };
        const runtime = startEngine(
          host,
          (next: typeof status) => {
            status = next;
          },
          renderer,
        );
        runtime.setPerformanceActive(true);
        runtime.setScene(toSceneSnapshot(game, null));
        Reflect.set(window, "lightingProbe", { runtime, status: () => status });
      },
      { renderer: backend(), game: sceneGame() },
    );
    const read = () =>
      page.evaluate(() => {
        const probe = Reflect.get(window, "lightingProbe");
        const status = probe.status();
        if (status.phase === "error") throw new Error(status.message);
        return {
          phase: status.phase,
          frames: probe.runtime.performance()?.completedFrames ?? 0,
          lighting: probe.runtime.sceneStats()?.lighting,
          metrics: { ...Reflect.get(window, "lightingBuffers") },
        };
      });
    await expect.poll(async () => (await read()).phase, { timeout: 90000 }).toBe("ready");
    const prepared = await read();
    await expect
      .poll(async () => (await read()).frames)
      .toBeGreaterThanOrEqual(prepared.frames + 8);
    const initial = await read();
    expect(initial.metrics.created).toBeGreaterThan(0);
    expect(initial.metrics.live).toBe(initial.lighting.clusterViews * 2);
    await expect
      .poll(async () => (await read()).frames, { timeout: 15000 })
      .toBeGreaterThanOrEqual(initial.frames + 120);
    const after = await read();
    expect(after.metrics).toEqual(initial.metrics);
    expect(after.lighting.clusterBufferAllocations).toBe(initial.lighting.clusterBufferAllocations);
    await page.evaluate(() => Reflect.get(window, "lightingProbe").runtime.dispose());
    expect(await page.evaluate(() => Reflect.get(window, "lightingBuffers").live)).toBe(0);
  });

  test("pauses GPU submission and resumes", async ({ page }) => {
    await page.goto(engineUrl());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    const result = await page.evaluate(async (renderer) => {
      Reflect.set(window, "renderCalls", 0);
      const prototype =
        renderer === "webgpu"
          ? Reflect.get(globalThis, "GPUQueue").prototype
          : WebGL2RenderingContext.prototype;
      const methods =
        renderer === "webgpu"
          ? ["submit"]
          : ["drawElements", "drawArrays", "drawElementsInstanced", "drawArraysInstanced"];
      for (const name of methods) {
        const original = Reflect.get(prototype, name);
        Reflect.set(prototype, name, function (this: unknown, ...args: unknown[]) {
          Reflect.set(window, "renderCalls", Number(Reflect.get(window, "renderCalls")) + 1);
          return Reflect.apply(original, this, args);
        });
      }
      const frames = async (count: number) => {
        for (let i = 0; i < count; i++) await new Promise(requestAnimationFrame);
      };
      const path = "/src/stores/renderPauseStore.ts";
      const { useRenderPauseStore } = await import(/* @vite-ignore */ path);
      await frames(5);
      const before = Number(Reflect.get(window, "renderCalls"));
      useRenderPauseStore.getState().pause("test");
      await frames(3);
      const paused = Number(Reflect.get(window, "renderCalls"));
      await frames(5);
      const after = Number(Reflect.get(window, "renderCalls"));
      useRenderPauseStore.getState().resume("test");
      await frames(5);
      return {
        before,
        paused,
        after,
        resumed: Number(Reflect.get(window, "renderCalls")),
      };
    }, backend());
    expect(result.before).toBeGreaterThan(0);
    expect(result.after).toBe(result.paused);
    expect(result.resumed).toBeGreaterThan(result.after);
  });

  test("recovers from a failed renderer download", async ({ page }) => {
    await page.route(/openmars_engine_bg\.wasm/, (route) =>
      route.fulfill({ status: 503, body: "Unavailable" }),
    );
    await page.goto(engineUrl());
    await expect(page.getByRole("alert")).toContainText("Renderer download failed (503)");
    await page.unroute(/openmars_engine_bg\.wasm/);
    await page.getByRole("button", { name: "Try again" }).click();
    await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready");
    await expect(page.getByRole("alert")).toHaveCount(0);
    await expect(page.locator("canvas")).toHaveCount(1);
  });

  test("preserves portrait guidance while the engine loads", async ({ browser, baseURL }) => {
    const context = await referenceContext(browser, {
      ...devices["iPhone 14"],
      baseURL,
    });
    const page = await context.newPage();
    await page.route(/openmars_engine_bg\.wasm/, async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 800));
      await route.continue();
    });
    await page.goto(engineUrl());
    await expect(page.getByText("Open Mars is played in landscape.")).toBeVisible();
    const output = path.resolve("../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    await page.screenshot({ path: path.join(output, "portrait.png") });
    await context.close();
  });
});

for (const [layout, viewport] of [
  ["desktop", { width: 1280, height: 720 }],
  ["iphone", { width: 844, height: 390 }],
  ["android", { width: 915, height: 412 }],
] as const) {
  test(`warms the authored menu and retires it across routes and worlds on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.skip(!test.info().project.name.startsWith("engine"));
    test.setTimeout(180_000);
    const context = await referenceContext(browser, { baseURL, viewport, reducedMotion: "reduce" });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.goto("/tests/fixtures/engine.html");
      await page.evaluate(async (renderer) => {
        const rp = "/src/engine/runtime.ts";
        const { startEngine } = await import(/* @vite-ignore */ rp);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        let resolve = () => {};
        let reject = (_error: Error) => {};
        const next = () =>
          new Promise<void>((yes, no) => {
            resolve = yes;
            reject = no;
          });
        const ready = next();
        const runtime = startEngine(
          host,
          (s: { phase: string; message: string }) => {
            if (s.phase === "ready") resolve();
            if (s.phase === "error") reject(new Error(s.message));
          },
          renderer,
        );
        runtime.setMenu("landing");
        Reflect.set(window, "menuProbe", { runtime, next, host });
        await ready;
      }, backend());
      const initial = await page.evaluate(
        () => Reflect.get(window, "menuProbe").runtime.sceneStats() as SceneDiagnostics,
      );
      expect(initial.menu.ready).toBe(true);
      expect(initial.menu.warmFrames).toBe(2);
      expect(initial.menu.warmCamera).toBe(false);
      expect(initial.menu.moon!.parts).toBeGreaterThan(0);
      expect(initial.menu.moon!.scale).toBeGreaterThan(0);
      expect(initial.menu.materials).toBeGreaterThan(0);
      expect(initial.atmosphere.bytes).toBe(1088);
      expect(initial.atmosphere.bodies).toBe(1);
      expect(initial.lighting.background).toEqual({
        size: 2048,
        mips: 12,
        faces: 6,
        format: "Rgb9e5Ufloat",
        bytes: 134217720,
        cpuBytes: 0,
      });
      expect(initial.menu.position).toEqual(
        (layout === "desktop" ? [3.9, 0, 0] : [4.6, -1.4, -1.5]).map(Math.fround),
      );
      for (const placement of ["landing", "panel", "lobby"] as const) {
        const current = await page.evaluate(async (placement) => {
          const { runtime } = Reflect.get(window, "menuProbe");
          const writes = runtime.sceneStats().atmosphere.writes;
          runtime.setMenu(placement);
          for (let i = 0; i < 300 && runtime.sceneStats().atmosphere.writes < writes + 4; i++)
            await new Promise(requestAnimationFrame);
          return runtime.sceneStats() as SceneDiagnostics;
        }, placement);
        expect(current.menu.root).toBe(initial.menu.root);
        expect(current.menu.rotation).toEqual(initial.menu.rotation);
        expect(current.atmosphere.buffer).toBe(initial.atmosphere.buffer);
        const desktopPose = {
          landing: [3.9, 0, 0],
          panel: [5.4, -0.8, -0.5],
          lobby: [3.9, -3.7, 1.4],
        };
        expect(current.menu.position).toEqual(
          (layout === "desktop" ? desktopPose[placement] : [4.6, -1.4, -1.5]).map(Math.fround),
        );
        await writeFile(
          path.resolve(
            "../tmp/bevy-scaffold",
            `phase5-menu-${backend()}-${layout}-${placement}.json`,
          ),
          JSON.stringify(current, null, 2),
        );
        const capture = await page.screenshot({
          path: path.resolve(
            "../tmp/bevy-scaffold",
            `phase5-menu-${backend()}-${layout}-${placement}.png`,
          ),
        });
        const { data, info } = await sharp(capture)
          .extract({
            left: Math.floor(viewport.width * 0.8),
            top: Math.floor(
              viewport.height * (layout === "desktop" && placement !== "lobby" ? 0.6 : 0.85),
            ),
            width: 16,
            height: 16,
          })
          .raw()
          .toBuffer({ resolveWithObject: true });
        let red = 0;
        for (let i = 0; i < data.length; i += info.channels) red += data[i];
        expect(red / (16 * 16), "Mars remains visible after the route change").toBeGreaterThan(50);
      }
      if (layout === "desktop") {
        const result = await page.evaluate(async (game) => {
          const sp = "/src/engine/scene.ts";
          const { toSceneSnapshot } = await import(/* @vite-ignore */ sp);
          const { runtime, next } = Reflect.get(window, "menuProbe");
          let ready = next();
          runtime.setScene(toSceneSnapshot(game, null));
          await ready;
          const gameStats = runtime.sceneStats() as SceneDiagnostics;
          ready = next();
          runtime.setScene(null);
          await ready;
          const returned = runtime.sceneStats() as SceneDiagnostics;
          ready = next();
          runtime.setMenu(null);
          await ready;
          for (let i = 0; i < 12; i++) await new Promise(requestAnimationFrame);
          return {
            gameStats,
            returned,
            cleared: runtime.sceneStats() as SceneDiagnostics,
            assets: runtime.assetStats(),
          };
        }, sceneGame("menu-handoff"));
        expect(result.gameStats.menu.root).toBeNull();
        expect(result.returned.menu.ready).toBe(true);
        expect(result.returned.menu.root).not.toBe(initial.menu.root);
        expect(result.cleared.menu.root).toBeNull();
        expect(result.cleared.menu.materials).toBe(0);
        expect(result.cleared.atmosphere.uniforms).toBe(0);
        expect(result.cleared.atmosphere.retired).toBe(0);
      }
      expect(errors).toEqual([]);
    } finally {
      await page.evaluate(() => Reflect.get(window, "menuProbe")?.runtime.dispose());
      await context.close();
    }
  });
}

test("reports a menu model decode failure and recovers with a fresh renderer", async ({ page }) => {
  test.skip(!test.info().project.name.startsWith("engine"));
  test.setTimeout(120_000);
  const phobos = assets["models/phobos"].variants[0].url;
  await page.route(
    (url) => url.pathname === phobos,
    (route) =>
      route.fulfill({ status: 200, contentType: "model/gltf-binary", body: "invalid glb" }),
  );
  await page.goto("/tests/fixtures/engine.html");
  const message = await page.evaluate(async (renderer) => {
    const rp = "/src/engine/runtime.ts";
    const { startEngine } = await import(/* @vite-ignore */ rp);
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0";
    document.body.append(host);
    let resolve = (_message: string) => {};
    const failed = new Promise<string>((yes) => {
      resolve = yes;
    });
    const runtime = startEngine(
      host,
      (s: { phase: string; message: string }) => {
        if (s.phase === "error") resolve(s.message);
      },
      renderer,
    );
    runtime.setMenu("landing");
    Reflect.set(window, "failedMenu", { runtime, host });
    return failed;
  }, backend());
  expect(message).toContain("models/phobos");
  expect(message).toContain("Could not decode");
  await page.unrouteAll();
  await page.evaluate(async (renderer) => {
    const previous = Reflect.get(window, "failedMenu");
    previous.runtime.dispose();
    previous.host.remove();
    const rp = "/src/engine/runtime.ts";
    const { startEngine } = await import(/* @vite-ignore */ rp);
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0";
    document.body.append(host);
    let resolve = () => {};
    let reject = (_error: Error) => {};
    const ready = new Promise<void>((yes, no) => {
      resolve = yes;
      reject = no;
    });
    const runtime = startEngine(
      host,
      (s: { phase: string; message: string }) => {
        if (s.phase === "ready") resolve();
        if (s.phase === "error") reject(new Error(s.message));
      },
      renderer,
    );
    runtime.setMenu("landing");
    try {
      await ready;
    } finally {
      runtime.dispose();
      host.remove();
    }
  }, backend());
});

test("warms all authored space bodies before travel and retires world resources", async ({
  page,
}) => {
  test.skip(!test.info().project.name.startsWith("engine"));
  test.setTimeout(240_000);
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/tests/fixtures/engine.html");
  await page.evaluate(async (renderer) => {
    const rp = "/src/engine/runtime.ts";
    const { startEngine } = await import(/* @vite-ignore */ rp);
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0";
    document.body.append(host);
    const states: { phase: string; message: string }[] = [];
    const runtime = startEngine(
      host,
      (state: { phase: string; message: string }) => {
        states.push(state);
      },
      renderer,
    );
    Reflect.set(window, "spaceProbe", { runtime, host, states });
  }, backend());
  const inspect = () =>
    page.evaluate(() => {
      const { runtime, states } = Reflect.get(window, "spaceProbe");
      return {
        scene: runtime.sceneStats() as SceneDiagnostics | null,
        assets: runtime.assetStats() as Record<string, number> | null,
        state: states.at(-1) as { phase: string; message: string } | undefined,
      };
    });
  const waitReady = async (focus?: FocusTarget) => {
    await expect
      .poll(
        async () => {
          const result = await inspect();
          if (result.state?.phase === "error") throw new Error(result.state.message);
          return result.state?.phase;
        },
        { timeout: 70_000 },
      )
      .toBe("ready");
    if (focus) await expect.poll(async () => (await inspect()).scene?.space.focus).toEqual(focus);
  };
  await waitReady();
  const empty = await inspect();
  await page.evaluate(async (game) => {
    const sp = "/src/engine/scene.ts";
    const { toSceneSnapshot } = await import(/* @vite-ignore */ sp);
    Reflect.get(window, "spaceProbe").runtime.setScene(toSceneSnapshot(game, null));
  }, sceneGame("space-focus"));
  await waitReady();
  const initial = await inspect();
  expect(initial.scene!.space.ready).toBe(true);
  expect(initial.scene!.space.bodies).toHaveLength(11);
  expect(initial.scene!.space.bodies.every((body) => body.warmFrames === 2 && body.parts > 0)).toBe(
    true,
  );
  expect(initial.scene!.space.sunParts).toBe(6);
  expect(initial.scene!.space.sunWarmFrames).toBe(2);
  expect(initial.scene!.space.orbitLines).toBe(9);
  expect(initial.scene!.space.orbitWarmFrames).toBe(2);
  expect(initial.scene!.space.warmCamera).toBe(false);
  expect(initial.scene!.space.phobos!.vertices).toBeGreaterThan(100);
  expect(initial.scene!.space.phobos!.contentRadius).toBeGreaterThan(0);
  expect(initial.scene!.space.seconds).toBe(0);
  const tileIds = initial.scene!.tiles.map((tile) => tile.entity);
  const bodies = initial.scene!.space.bodies.map((body) => body.entity);
  const output = path.resolve("../tmp/bevy-scaffold");
  await mkdir(output, { recursive: true });
  for (const focus of [
    { type: "body", body: "mercury" },
    { type: "body", body: "venus" },
    { type: "body", body: "earth" },
    { type: "body", body: "ceres" },
    { type: "body", body: "jupiter" },
    { type: "body", body: "saturn" },
    { type: "body", body: "uranus" },
    { type: "body", body: "neptune" },
    { type: "solar-system" },
    { type: "body", body: "phobos" },
    { type: "body", body: "mars" },
  ] as const) {
    await page.evaluate(
      (focus) => Reflect.get(window, "spaceProbe").runtime.setFocus(focus),
      focus,
    );
    await waitReady(focus);
    const current = await inspect();
    expect(current.scene!.space.focus).toEqual(focus);
    expect(current.scene!.focus).toEqual(focus);
    expect(current.scene!.tiles.map((tile) => tile.entity)).toEqual(tileIds);
    expect(current.scene!.space.bodies.map((body) => body.entity)).toEqual(bodies);
    const name = focus.type === "body" ? focus.body : "solar-system";
    await page.screenshot({ path: path.join(output, `phase5-space-${backend()}-${name}.png`) });
    await writeFile(
      path.join(output, `phase5-space-${backend()}-${name}.json`),
      JSON.stringify(current, null, 2),
    );
  }
  await expect
    .poll(async () => (await inspect()).assets!.gpuImages, { timeout: 15_000 })
    .toBe(initial.assets!.gpuImages);
  await page.evaluate(() => {
    const { runtime } = Reflect.get(window, "spaceProbe");
    runtime.setFocus({ type: "body", body: "earth" });
    runtime.setFocus({ type: "solar-system" });
  });
  await waitReady({ type: "solar-system" });
  expect((await inspect()).scene!.space.focus).toEqual({ type: "solar-system" });
  await page.evaluate(() => Reflect.get(window, "spaceProbe").runtime.setScene(null));
  await waitReady();
  await expect
    .poll(
      async () => {
        const { assets } = await inspect();
        return [
          assets!.images,
          assets!.meshes,
          assets!.materials,
          assets!.gpuImages,
          assets!.gpuMeshes,
          assets!.gpuMaterials,
        ];
      },
      { timeout: 15_000 },
    )
    .toEqual([
      empty.assets!.images,
      empty.assets!.meshes,
      empty.assets!.materials,
      empty.assets!.gpuImages,
      empty.assets!.gpuMeshes,
      empty.assets!.gpuMaterials,
    ]);
  expect((await inspect()).scene!.space.bodies).toHaveLength(0);
  expect(errors).toEqual([]);
  await page.evaluate(() => Reflect.get(window, "spaceProbe").runtime.dispose());
});

test("keeps the React menu usable over the authored Bevy background", async ({ page }) => {
  test.skip(!test.info().project.name.startsWith("engine"));
  test.setTimeout(90_000);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(`/?renderer=${backend()}`);
  await expect(page.locator("[data-engine-state]")).toHaveAttribute("data-engine-state", "ready", {
    timeout: 70_000,
  });
  await expect(page.getByRole("status")).toBeHidden();
  const inspect = () =>
    page.evaluate(async () => {
      const ep = "/src/stores/engineStore.ts";
      const { useEngineStore } = await import(/* @vite-ignore */ ep);
      return useEngineStore.getState().inspectScene!() as SceneDiagnostics;
    });
  const initial = await inspect();
  expect(initial.menu.ready).toBe(true);
  await expect(page.getByRole("link", { name: "New game", exact: true })).toBeVisible();
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-menu-${backend()}-react-landing.png`),
  });
  await page.getByRole("link", { name: "New game", exact: true }).click();
  await expect.poll(async () => (await inspect()).menu.position[0]).toBeCloseTo(5.4, 4);
  expect((await inspect()).menu.root).toBe(initial.menu.root);
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-menu-${backend()}-react-panel.png`),
  });
});

async function mountTravelScene(
  page: Page,
  game: ReturnType<typeof sceneGame>,
  compact: boolean,
  initialPhase: "playing" | "loading" | "menu" = "playing",
) {
  await page.evaluate(
    async ({ game, compact, initialPhase }) => {
      const paths = [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/engine/EngineBackground.tsx",
        "/src/components/layout/panels/TopMenuBar.tsx",
        "/src/components/mobile/MobileTopBar.tsx",
        "/src/components/mobile/MobileGameContext.tsx",
        "/src/components/ui/debug/uiDebug/fixtures.ts",
        "/src/stores/gameStore.ts",
        "/src/stores/appPhaseStore.ts",
        "/src/constants/zIndex.ts",
        "/src/components/LayoutModeAttribute.tsx",
      ];
      const [
        React,
        DOM,
        engine,
        desktop,
        mobile,
        mobileContext,
        fixtures,
        games,
        phases,
        layers,
        layoutMode,
      ] = await Promise.all(paths.map((p) => import(/* @vite-ignore */ p)));
      document.documentElement.style.setProperty(
        "--surface-layer",
        String(layers.Z_INDEX.CONTROL_DECORATION),
      );
      const ap = "/src/services/audioService.ts";
      const { audioService } = await import(/* @vite-ignore */ ap);
      let travelSounds = 0;
      const playTravel = audioService.playTravelSound.bind(audioService);
      audioService.playTravelSound = () => {
        travelSounds++;
        return playTravel();
      };
      Reflect.set(window, "travelAudioCount", () => travelSounds);
      const css = "/src/index.css";
      await import(/* @vite-ignore */ css);
      const dto = fixtures.createPreviewGame();
      dto.id = game.id;
      dto.board = game.board;
      dto.settings = { ...dto.settings, ...game.settings };
      dto.currentPlayer.id = game.currentPlayer.id;
      dto.currentPlayer.name = game.currentPlayer.name;
      dto.currentPlayer.color = game.currentPlayer.color;
      dto.otherPlayers = game.otherPlayers;
      dto.globalParameters = game.globalParameters;
      dto.currentPlayer.pendingTileSelection = game.currentPlayer.pendingTileSelection;
      dto.isSpectator = game.isSpectator;
      dto.colonies = game.colonies ?? [];
      dto.projectFunding = game.projectFunding ?? [];
      dto.tradeFleets = game.tradeFleets ?? {};
      games.useGameStore.getState().setGame(dto);
      if (initialPhase === "menu") {
        phases.useAppPhaseStore.getState().setPhase({ kind: "menu", route: "landing" });
      } else {
        phases.useAppPhaseStore.getState().setPhase({ kind: initialPhase, gameId: dto.id });
      }
      const host = document.createElement("div");
      host.dataset.uiDebugPreview = "true";
      host.style.cssText = `position:fixed;inset:0;--hud-top-h:${compact ? 44 : 60}px`;
      document.body.append(host);
      const h = React.default.createElement,
        noop = () => {};
      function App() {
        const current = games.useGameStore((s: { game: typeof dto }) => s.game);
        let bar = null;
        if (current) {
          bar = compact
            ? h(
                mobileContext.MobileGameContext.Provider,
                {
                  value: {
                    gameState: current,
                    currentPlayer: current.currentPlayer,
                    isSpectator: false,
                    isReplay: false,
                    playerColorMap: new Map(),
                    onPlayCard: noop,
                    onInspectCard: noop,
                    onStandardProjectSelect: noop,
                    onActionSelect: noop,
                    onConvertPlantsToGreenery: noop,
                    onConvertHeatToTemperature: noop,
                    onPlayerClick: noop,
                    onSendChatMessage: noop,
                    chatMessages: [],
                    onLeaveGame: noop,
                    onEndGame: noop,
                  },
                },
                h(mobile.default, { showStatus: true, showActions: true }),
              )
            : h(desktop.default, { gameState: current, currentPlayer: current.currentPlayer });
        }
        return h(
          React.default.Fragment,
          null,
          h(layoutMode.default),
          h(engine.default, { menu: false }),
          bar,
        );
      }
      const root = DOM.default.createRoot(host);
      root.render(h(App));
      Reflect.set(window, "unmountTravelScene", () => root.unmount());
    },
    { game, compact, initialPhase },
  );
}

for (const [layout, profile] of [
  ["desktop", { viewport: { width: 1280, height: 720 } }],
  ["iphone", devices["iPhone 14 landscape"]],
  ["android", devices["Pixel 7 landscape"]],
] as const) {
  test(`travels through the retained React controls on ${layout}`, async ({ browser, baseURL }) => {
    test.skip(!test.info().project.name.startsWith("engine"));
    test.setTimeout(180_000);
    const context = await referenceContext(browser, {
      ...profile,
      baseURL,
      reducedMotion: "reduce",
    });
    const page = await context.newPage();
    const errors: string[] = [],
      mutations: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("request", (request) => {
      if (request.method() !== "GET") mutations.push(request.method() + " " + request.url());
    });
    try {
      await page.goto(`/tests/fixtures/engine.html?renderer=${backend()}`);
      await mountTravelScene(page, sceneGame("react-travel"), layout !== "desktop");
      const inspect = () =>
        page.evaluate(async () => {
          const ep = "/src/stores/engineStore.ts";
          const { useEngineStore } = await import(/* @vite-ignore */ ep);
          const store = useEngineStore.getState();
          return {
            status: store.status,
            focus: store.focus,
            scene: store.inspectScene?.() as SceneDiagnostics | null,
          };
        });
      const ready = async () => {
        await expect
          .poll(
            async () => {
              const { status, focus, scene } = await inspect();
              if (status.phase === "error") throw new Error(status.message);
              const native = scene?.space.focus;
              return (
                status.phase === "ready" &&
                native?.type === focus.type &&
                (focus.type !== "body" || (native?.type === "body" && native.body === focus.body))
              );
            },
            { timeout: 70_000 },
          )
          .toBe(true);
      };
      await ready();
      const original = await inspect();
      await page.getByRole("button", { name: "TRAVEL", exact: true }).click();
      await expect(page.getByRole("button", { name: "Mars", exact: true })).toHaveAttribute(
        "aria-current",
        "location",
      );
      const neptune = page.getByRole("button", { name: "Neptune", exact: true });
      await neptune.scrollIntoViewIfNeeded();
      const box = await neptune.boundingBox();
      expect(box!.height).toBeGreaterThanOrEqual(44);
      expect(box!.y + box!.height).toBeLessThanOrEqual(page.viewportSize()!.height);
      await page.screenshot({
        path: path.resolve("../tmp/bevy-scaffold", `phase5-travel-${backend()}-${layout}-menu.png`),
      });
      await page.getByRole("button", { name: "Venus", exact: true }).click();
      await ready();
      expect((await inspect()).scene!.space.focus).toEqual({ type: "body", body: "venus" });
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-travel-${backend()}-${layout}-venus.png`,
        ),
      });
      await page.getByRole("button", { name: "TRAVEL", exact: true }).click();
      await page.getByRole("button", { name: "Solar System", exact: true }).click();
      await ready();
      const solar = await inspect();
      expect(solar.scene!.space.focus).toEqual({ type: "solar-system" });
      expect(solar.scene!.tiles.map((tile) => tile.entity)).toEqual(
        original.scene!.tiles.map((tile) => tile.entity),
      );
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-travel-${backend()}-${layout}-solar.png`,
        ),
      });
      const markers = page.locator("[data-solar-body]");
      await expect(markers).toHaveCount(9);
      const markerBounds = await markers.evaluateAll((buttons) =>
        buttons.map((button) => {
          const bounds = button.getBoundingClientRect();
          const target = document.elementFromPoint(
            bounds.x + bounds.width / 2,
            bounds.y + bounds.height / 2,
          );
          return {
            body: button.getAttribute("data-solar-body"),
            width: bounds.width,
            height: bounds.height,
            clickable: target?.closest("[data-solar-body]") === button,
          };
        }),
      );
      for (const bounds of markerBounds) {
        expect(bounds.width).toBeGreaterThanOrEqual(44);
        expect(bounds.height).toBeGreaterThanOrEqual(44);
        expect(bounds.clickable, `${bounds.body} label must have an unobstructed target`).toBe(
          true,
        );
      }
      const destination = page.getByRole("button", { name: "Travel to Neptune", exact: true });
      if (layout !== "desktop") {
        await destination.tap();
        await expect(destination).toHaveAttribute("aria-pressed", "true");
        expect((await inspect()).focus).toEqual({ type: "solar-system" });
        expect(await page.evaluate(() => Reflect.get(window, "travelAudioCount")())).toBe(2);
        await page.screenshot({
          path: path.resolve(
            "../tmp/bevy-scaffold",
            `phase5-travel-${backend()}-${layout}-solar-selected.png`,
          ),
        });
        await destination.tap();
      } else {
        await destination.hover();
        await expect(destination.getByText("Click to travel")).toHaveCSS("opacity", "1");
        await destination.click();
      }
      await ready();
      expect((await inspect()).scene!.space.focus).toEqual({ type: "body", body: "neptune" });
      await expect(markers).toHaveCount(0);
      await page.getByRole("button", { name: "TRAVEL", exact: true }).click();
      await page.getByRole("button", { name: "Solar System", exact: true }).click();
      await ready();
      await expect(markers).toHaveCount(9);
      const bodyMarker = page.locator('[data-solar-marker="neptune"]');
      const markerBox = (await bodyMarker.boundingBox())!;
      const point = { x: markerBox.x + markerBox.width / 2, y: markerBox.y + markerBox.height - 1 };
      expect(
        await page.evaluate(
          ({ x, y }) => document.elementFromPoint(x, y)?.getAttribute("data-solar-marker"),
          point,
        ),
      ).toBe("neptune");
      if (layout === "desktop") {
        await page.mouse.click(point.x, point.y);
      } else {
        await expect(destination).toHaveAttribute("aria-pressed", "false");
        await page.touchscreen.tap(point.x, point.y);
        await expect(destination).toHaveAttribute("aria-pressed", "true");
        expect(await page.evaluate(() => Reflect.get(window, "travelAudioCount")())).toBe(4);
        await page.touchscreen.tap(point.x, point.y);
      }
      await ready();
      expect((await inspect()).scene!.space.focus).toEqual({ type: "body", body: "neptune" });
      await expect(markers).toHaveCount(0);
      await page.getByRole("button", { name: "TRAVEL", exact: true }).click();
      await page.getByRole("button", { name: "Solar System", exact: true }).click();
      await ready();
      await expect(markers).toHaveCount(9);
      await page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts",
          pp = "/src/stores/appPhaseStore.ts";
        const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
          import(/* @vite-ignore */ gp),
          import(/* @vite-ignore */ pp),
        ]);
        const game = { ...useGameStore.getState().game, id: "react-travel-replacement" };
        useGameStore.getState().setGame(game);
        useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
      });
      await ready();
      const replacement = await inspect();
      expect(replacement.focus).toEqual({ type: "body", body: "mars" });
      expect(replacement.scene!.space.focus).toEqual({ type: "body", body: "mars" });
      expect(replacement.scene!.tiles[0].entity).not.toBe(original.scene!.tiles[0].entity);
      await expect(markers).toHaveCount(0);
      expect(await page.evaluate(() => Reflect.get(window, "travelAudioCount")())).toBe(6);
      expect(mutations).toEqual([]);
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });
}

for (const [layout, profile] of [
  ["desktop", { viewport: { width: 1280, height: 720 } }],
  ["iphone", devices["iPhone 14 landscape"]],
  ["android", devices["Pixel 7 landscape"]],
] as const) {
  test(`renders and picks five moving-frame offworld grids with bounded ownership on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    const context = await referenceContext(browser, { ...profile, baseURL });
    const page = await context.newPage();
    try {
      test.skip(!test.info().project.name.startsWith("engine"));
      test.setTimeout(240_000);
      const viewport = page.viewportSize()!;
      await page.emulateMedia({ reducedMotion: "reduce" });
      await trackTerrainBuffers(page);
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.goto("/tests/fixtures/engine.html");
      await page.evaluate(async (renderer) => {
        const path = "/src/engine/runtime.ts";
        const { startEngine } = await import(/* @vite-ignore */ path);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        const events: { type: string; tileId?: string }[] = [];
        let status: { phase: string; message: string } = { phase: "starting", message: "" };
        const runtime = startEngine(
          host,
          (next: typeof status) => {
            status = next;
          },
          renderer,
          (event: { type: string; tileId?: string }) => events.push(event),
        );
        Reflect.set(window, "offworldProbe", { runtime, host, events, status: () => status });
      }, backend());
      const inspect = () =>
        page.evaluate(() => {
          const probe = Reflect.get(window, "offworldProbe");
          return {
            scene: probe.runtime.sceneStats() as SceneDiagnostics | null,
            assets: probe.runtime.assetStats() as Record<string, number>,
            status: probe.status() as { phase: string; message: string },
            selected: probe.events.filter((e: { type: string }) => e.type === "tile-selected") as {
              tileId: string;
            }[],
          };
        });
      const ready = async () => {
        await expect
          .poll(
            async () => {
              const p = await inspect();
              if (p.status.phase === "error") throw new Error(p.status.message);
              return { phase: p.status.phase, framing: p.scene?.camera.framing ?? false };
            },
            { timeout: 70_000 },
          )
          .toEqual({ phase: "ready", framing: false });
      };
      await ready();
      const baseline = await inspect();
      await page.evaluate(async (game) => {
        const path = "/src/engine/scene.ts";
        const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
        const p = Reflect.get(window, "offworldProbe");
        p.game = game;
        p.runtime.setScene(toSceneSnapshot(game, null));
      }, offworldSceneGame());
      await ready();
      const first = await inspect();
      expect(first.scene!.celestialGrids.bodies).toHaveLength(5);
      expect(first.assets.gridUniforms).toBe(6);
      expect(first.scene!.celestialGrids.bodies.every((b) => b.tiles === 7 && !b.pending)).toBe(
        true,
      );
      const roots = first.scene!.celestialGrids.bodies.map((b) => [b.body, b.root, b.buffer]);
      for (const [focus, body, id] of [
        ["mercury", "mercury", "400,0,-400"],
        ["venus", "venus", "100,0,-100"],
        ["earth", "luna", "300,0,-300"],
        ["jupiter", "ganymede", "200,0,-200"],
        ["mars", "phobos", "500,0,-500"],
      ] as const) {
        await page.evaluate(
          (body) => Reflect.get(window, "offworldProbe").runtime.setFocus({ type: "body", body }),
          focus,
        );
        await expect
          .poll(async () => (await inspect()).scene?.space.focus)
          .toEqual({ type: "body", body: focus });
        await ready();
        const current = await inspect();
        expect(current.scene!.celestialGrids.bodies.map((b) => [b.body, b.root, b.buffer])).toEqual(
          roots,
        );
        const grid = current.scene!.celestialGrids.bodies.find((b) => b.body === body)!;
        expect(grid.visibility).toBe(1);
        const point = grid.sample.find((t) => t.id === id)!.point!;
        expect(point[0]).toBeGreaterThan(0);
        expect(point[0]).toBeLessThan(viewport.width);
        expect(point[1]).toBeGreaterThan(0);
        expect(point[1]).toBeLessThan(viewport.height);
        const writes = current.assets.gridUniformWrites;
        if (layout === "desktop") {
          await page.mouse.move(...point);
          await page.mouse.click(...point);
        } else {
          await page.touchscreen.tap(...point);
        }
        await expect.poll(async () => (await inspect()).selected.at(-1)?.tileId).toBe(id);
        await expect
          .poll(async () => (await inspect()).assets.gridUniformWrites, { timeout: 15_000 })
          .toBeGreaterThan(writes + 6 * 12);
        const capture = await page.screenshot({
          path: path.resolve(
            "../tmp/bevy-scaffold",
            `phase5-offworld-grid-${backend()}-${layout}-${body}.png`,
          ),
        });
        if (body === "venus" && layout === "desktop") {
          // Native captions occupy the centre. Sample exposed surface toward the
          // two upper neighbours, keeping the probe inside the same hex.
          const upper = grid.sample
            .filter((tile) => tile.id !== id && tile.point)
            .map((tile) => tile.point!)
            .sort((a, b) => a[1] - b[1])
            .slice(0, 2);
          expect(upper).toHaveLength(2);
          const sample = point.map(
            (value, axis) => value + 0.3 * ((upper[0][axis] + upper[1][axis]) / 2 - value),
          );
          const surfaceRegion = {
            left: Math.round(sample[0]) - 4,
            top: Math.round(sample[1]) - 4,
            width: 8,
            height: 8,
          };
          expect(surfaceRegion.left).toBeGreaterThanOrEqual(0);
          expect(surfaceRegion.top).toBeGreaterThanOrEqual(0);
          expect(surfaceRegion.left + surfaceRegion.width).toBeLessThanOrEqual(viewport.width);
          expect(surfaceRegion.top + surfaceRegion.height).toBeLessThanOrEqual(viewport.height);
          const { data, info } = await sharp(capture)
            .extract(surfaceRegion)
            .raw()
            .toBuffer({ resolveWithObject: true });
          let red = 0,
            green = 0;
          for (let i = 0; i < data.length; i += info.channels) {
            red += data[i];
            green += data[i + 1];
          }
          const highlighted = { red, green };
          await page.mouse.move(2, 2);
          await page.evaluate(async () => {
            const path = "/src/engine/scene.ts";
            const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
            const p = Reflect.get(window, "offworldProbe");
            p.available = p.game.currentPlayer.pendingTileSelection.availableHexes;
            p.game.currentPlayer.pendingTileSelection.availableHexes = [];
            p.runtime.setScene(toSceneSnapshot(p.game, null));
          });
          const start = (await inspect()).assets.gridUniformWrites;
          await expect
            .poll(
              async () => {
                const result = await inspect();
                if (result.status.phase === "error") {
                  throw new Error(result.status.message);
                }
                return result.assets.gridUniformWrites;
              },
              { timeout: 15_000 },
            )
            .toBeGreaterThan(start + 6 * 45);
          const plain = await page.screenshot({
            path: path.resolve(
              "../tmp/bevy-scaffold",
              `phase5-offworld-grid-${backend()}-${layout}-venus-plain.png`,
            ),
          });
          const pixels = await sharp(plain)
            .extract(surfaceRegion)
            .raw()
            .toBuffer({ resolveWithObject: true });
          red = 0;
          green = 0;
          for (let i = 0; i < pixels.data.length; i += pixels.info.channels) {
            red += pixels.data[i];
            green += pixels.data[i + 1];
          }
          expect(
            highlighted.green / highlighted.red - green / red,
            "The PBR surface gains the source green placement tint",
          ).toBeGreaterThan(0.025);
          expect((highlighted.green - green) / 64).toBeGreaterThan(2);
          await page.evaluate(async () => {
            const path = "/src/engine/scene.ts";
            const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
            const p = Reflect.get(window, "offworldProbe");
            p.game.currentPlayer.pendingTileSelection.availableHexes = p.available;
            p.runtime.setScene(toSceneSnapshot(p.game, null));
          });
        }
      }
      await page.evaluate(() =>
        Reflect.get(window, "offworldProbe").runtime.setFocus({ type: "body", body: "venus" }),
      );
      await expect
        .poll(async () => (await inspect()).scene?.space.focus)
        .toEqual({ type: "body", body: "venus" });
      await ready();
      const stopped = (await inspect()).scene!.space;
      await page.emulateMedia({ reducedMotion: "no-preference" });
      await expect
        .poll(async () => (await inspect()).scene!.space.seconds)
        .toBeGreaterThan(stopped.seconds + 0.5);
      const moving = await inspect();
      expect(
        moving.scene!.space.bodies.find((body) => body.body === "venus")!.position,
      ).not.toEqual(stopped.bodies.find((body) => body.body === "venus")!.position);
      const movingPoint = moving
        .scene!.celestialGrids.bodies.find((body) => body.body === "venus")!
        .sample.find((tile) => tile.id === "100,0,-100")!.point!;
      if (layout === "desktop") {
        await page.mouse.click(...movingPoint);
      } else {
        await page.touchscreen.tap(...movingPoint);
      }
      await expect.poll(async () => (await inspect()).selected.at(-1)?.tileId).toBe("100,0,-100");
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.evaluate(async () => {
        const path = "/src/engine/scene.ts";
        const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
        const p = Reflect.get(window, "offworldProbe");
        p.game.board.tiles.find(
          (t: { location: string; coordinates: { q: number; r: number } }) =>
            t.location === "venus" && t.coordinates.q === 100 && t.coordinates.r === 0,
        ).reservedBy = "player";
        p.runtime.setScene(toSceneSnapshot(p.game, null));
      });
      const venus = first.scene!.celestialGrids.bodies.find((b) => b.body === "venus")!;
      await expect
        .poll(
          async () =>
            (await inspect()).scene!.celestialGrids.bodies.find((b) => b.body === "venus")!
              .revision,
        )
        .toBeGreaterThan(venus.revision);
      expect(
        (await inspect()).scene!.celestialGrids.bodies.map((b) => [b.body, b.root, b.buffer]),
      ).toEqual(roots);
      await page.evaluate(async () => {
        const path = "/src/engine/scene.ts";
        const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
        const p = Reflect.get(window, "offworldProbe");
        p.game.board.tiles = p.game.board.tiles.filter(
          (t: { location: string }) => t.location !== "mercury",
        );
        p.game.currentPlayer.pendingTileSelection.availableHexes =
          p.game.currentPlayer.pendingTileSelection.availableHexes.filter(
            (id: string) => id !== "400,0,-400",
          );
        p.runtime.setScene(toSceneSnapshot(p.game, null));
      });
      await expect.poll(async () => (await inspect()).scene!.celestialGrids.bodies.length).toBe(4);
      await expect.poll(async () => (await inspect()).assets.gridUniforms).toBe(5);
      await page.evaluate(() => Reflect.get(window, "offworldProbe").runtime.setScene(null));
      await ready();
      await expect
        .poll(
          async () => {
            const { assets } = await inspect();
            return [
              assets.images,
              assets.meshes,
              assets.materials,
              assets.gpuImages,
              assets.gpuMeshes,
              assets.gpuMaterials,
            ];
          },
          { timeout: 15_000 },
        )
        .toEqual([
          baseline.assets.images,
          baseline.assets.meshes,
          baseline.assets.materials,
          baseline.assets.gpuImages,
          baseline.assets.gpuMeshes,
          baseline.assets.gpuMaterials,
        ]);
      expect((await inspect()).scene!.celestialGrids.bodies).toHaveLength(0);
      if (backend() === "webgpu")
        expect(await page.evaluate(() => Reflect.get(window, "gridBuffers"))).toEqual({
          created: 6,
          destroyed: 6,
          live: 0,
        });
      expect(errors).toEqual([]);
      await page.evaluate(() => Reflect.get(window, "offworldProbe").runtime.dispose());
    } finally {
      await context.close();
    }
  });
}

for (const [layout, profile] of [
  ["desktop", { viewport: { width: 1280, height: 720 } }],
  ["iphone", devices["iPhone 14 landscape"]],
  ["android", devices["Pixel 7 landscape"]],
] as const) {
  test(`frames public placement destinations and respects manual navigation on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.skip(!test.info().project.name.startsWith("engine"));
    test.setTimeout(240_000);
    const context = await referenceContext(browser, {
      ...profile,
      baseURL,
      reducedMotion: "reduce",
    });
    const page = await context.newPage();
    const errors: string[] = [],
      mutations: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("request", (request) => {
      if (request.method() !== "GET") {
        mutations.push(request.method() + " " + request.url());
      }
    });
    try {
      await page.goto(`/tests/fixtures/engine.html?renderer=${backend()}`);
      const game = offworldSceneGame();
      game.currentPlayer.pendingTileSelection!.availableHexes = game.board.tiles
        .filter((tile) => tile.location === "mercury")
        .map((tile) => `${tile.coordinates.q},${tile.coordinates.r},${tile.coordinates.s}`);
      await mountTravelScene(page, game, layout !== "desktop");
      const inspect = () =>
        page.evaluate(async () => {
          const path = "/src/stores/engineStore.ts";
          const { useEngineStore } = await import(/* @vite-ignore */ path);
          const state = useEngineStore.getState();
          const css = getComputedStyle(document.documentElement);
          return {
            status: state.status,
            focus: state.focus,
            scene: state.inspectScene?.() as SceneDiagnostics | null,
            viewport: {
              width: innerWidth,
              height: innerHeight,
              top: parseFloat(css.getPropertyValue("--hud-top-h")),
              bottom: parseFloat(css.getPropertyValue("--hud-dock-h")),
              left: parseFloat(css.getPropertyValue("--hud-rail-left-w")),
              right: parseFloat(css.getPropertyValue("--hud-rail-right-w")),
            },
          };
        });
      const ready = async (body: string) => {
        await expect
          .poll(
            async () => {
              const current = await inspect();
              if (current.status.phase === "error") throw new Error(current.status.message);
              return {
                phase: current.status.phase,
                focus: current.scene?.space.focus,
                framing: current.scene?.camera.framing,
              };
            },
            { timeout: 70_000 },
          )
          .toEqual({ phase: "ready", focus: { type: "body", body }, framing: false });
      };
      const update = (location: string | null) =>
        page.evaluate(async (location) => {
          const path = "/src/stores/gameStore.ts";
          const { useGameStore } = await import(/* @vite-ignore */ path);
          const state = useGameStore.getState();
          const game = state.game;
          const availableHexes = location
            ? game.board.tiles
                .filter((tile: { location: string }) => tile.location === location)
                .map(
                  (tile: { coordinates: { q: number; r: number; s: number } }) =>
                    `${tile.coordinates.q},${tile.coordinates.r},${tile.coordinates.s}`,
                )
            : [];
          state.setGame({
            ...game,
            currentPlayer: {
              ...game.currentPlayer,
              pendingTileSelection: location ? { tileType: "greenery", availableHexes } : null,
            },
          });
        }, location);
      const waitFrames = async (step: number, count: number) => {
        await expect
          .poll(async () => (await inspect()).scene!.step, { timeout: 15_000 })
          .toBeGreaterThan(step + count);
      };
      for (const [location, body, grid, home] of [
        ["mercury", "mercury", "mercury", 6],
        ["moon", "earth", "luna", 4.7],
        ["ganymede", "jupiter", "ganymede", 8.6],
      ] as const) {
        if (location !== "mercury") {
          await update(location);
        }
        await ready(body);
        const current = await inspect();
        expect(current.focus).toEqual({ type: "body", body });
        if (layout === "desktop") {
          expect(current.scene!.camera.distance).toBeCloseTo(home, 4);
        } else {
          const sample = current.scene!.celestialGrids.bodies.find(
            (entry) => entry.body === grid,
          )!.sample;
          expect(sample).toHaveLength(7);
          for (const tile of sample) {
            expect(tile.point).not.toBeNull();
            expect(tile.point![0]).toBeGreaterThanOrEqual(current.viewport.left);
            expect(tile.point![0]).toBeLessThanOrEqual(
              current.viewport.width - current.viewport.right,
            );
            expect(tile.point![1]).toBeGreaterThanOrEqual(current.viewport.top);
            expect(tile.point![1]).toBeLessThanOrEqual(
              current.viewport.height - current.viewport.bottom,
            );
          }
        }
        await page.screenshot({
          path: path.resolve(
            "../tmp/bevy-scaffold",
            `phase5-placement-${backend()}-${layout}-${location}.png`,
          ),
        });
      }
      await page.getByRole("button", { name: "TRAVEL", exact: true }).click();
      await page.getByRole("button", { name: "Mars", exact: true }).click();
      await ready("mars");
      const before = await inspect();
      const sounds = await page.evaluate(() => Reflect.get(window, "travelAudioCount")());
      await page.evaluate(async () => {
        const path = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ path);
        const state = useGameStore.getState();
        state.setGame({ ...state.game, generation: state.game.generation + 1 });
      });
      await waitFrames(before.scene!.step, 30);
      expect((await inspect()).focus).toEqual({ type: "body", body: "mars" });
      expect(await page.evaluate(() => Reflect.get(window, "travelAudioCount")())).toBe(sounds);
      await update(null);
      await waitFrames((await inspect()).scene!.step, 2);
      await update("mercury");
      await ready("mercury");
      if (layout !== "desktop") {
        await update(null);
        await waitFrames((await inspect()).scene!.step, 2);
        await update("mercury");
        await expect
          .poll(async () => (await inspect()).scene?.camera.framing, {
            intervals: [20],
            timeout: 10_000,
          })
          .toBe(true);
        await page.locator("canvas").focus();
        await page.keyboard.press("-");
        const cancelled = await inspect();
        expect(cancelled.scene!.camera.framing).toBe(false);
        await waitFrames(cancelled.scene!.step, 30);
        expect((await inspect()).scene!.camera.distance).toBeCloseTo(
          cancelled.scene!.camera.distance,
          5,
        );
      }
      await page.evaluate(() => Reflect.get(window, "unmountTravelScene")());
      await expect
        .poll(async () =>
          page.evaluate(async () => {
            const path = "/src/stores/engineStore.ts";
            const { useEngineStore } = await import(/* @vite-ignore */ path);
            const state = useEngineStore.getState();
            return { overlay: state.overlay, camera: state.camera, inspector: state.inspectScene };
          }),
        )
        .toEqual({ overlay: null, camera: null, inspector: null });
      expect(errors).toEqual([]);
      expect(mutations).toEqual([]);
    } finally {
      await context.close();
    }
  });
}

test("accepts the public offworld contract and recovers from invalid fleet input", async ({
  page,
}) => {
  test.skip(!test.info().project.name.startsWith("engine"));
  test.setTimeout(180_000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto(`/tests/fixtures/engine.html?renderer=${backend()}`);
  const playerId = await page.evaluate(async () => {
    const path = "/src/components/ui/debug/uiDebug/fixtures.ts";
    const { createPreviewGame } = await import(/* @vite-ignore */ path);
    return createPreviewGame().currentPlayer.id as string;
  });
  const game = sceneGame("offworld-contract");
  game.colonies = [
    { id: "titan", location: "venus", tradedThisGen: true, traderId: playerId },
  ] as NonNullable<typeof game.colonies>;
  game.tradeFleets = { [playerId]: { total: 3, available: 2 } };
  game.projectFunding = [
    {
      id: "pf_orbital_station",
      name: "Orbital station",
      seats: [{}, {}, {}],
      seatOwners: [{ playerId: "player" }],
      isCompleted: false,
    },
  ] as NonNullable<typeof game.projectFunding>;
  await mountTravelScene(page, game, false);
  const ready = async () => {
    await expect
      .poll(
        async () =>
          page.evaluate(async () => {
            const path = "/src/stores/engineStore.ts";
            const { useEngineStore } = await import(/* @vite-ignore */ path);
            const status = useEngineStore.getState().status;
            if (status.phase === "error") throw new Error(status.message);
            return status.phase;
          }),
        { timeout: 70_000 },
      )
      .toBe("ready");
  };
  await ready();
  await page.evaluate(async () => {
    const path = "/src/stores/gameStore.ts";
    const { useGameStore } = await import(/* @vite-ignore */ path);
    const state = useGameStore.getState();
    state.setGame({
      ...state.game,
      tradeFleets: { [state.game.currentPlayer.id]: { total: 3, available: 4 } },
    });
  });
  await expect(page.getByRole("alert")).toContainText("Invalid offworld fleet presentation");
  await page.evaluate(async () => {
    const path = "/src/stores/gameStore.ts";
    const { useGameStore } = await import(/* @vite-ignore */ path);
    const state = useGameStore.getState();
    state.setGame({
      ...state.game,
      tradeFleets: { [state.game.currentPlayer.id]: { total: 3, available: 1 } },
    });
  });
  await page.getByRole("button", { name: "Try again", exact: true }).click();
  await ready();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await page.evaluate(() => Reflect.get(window, "unmountTravelScene")());
  expect(errors).toEqual([]);
});

test("renders authored offworld hulls pads and the orbital station with bounded ownership", async ({
  page,
}) => {
  test.skip(!test.info().project.name.startsWith("engine"));
  test.setTimeout(180_000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/tests/fixtures/engine.html");
  await page.evaluate(async (renderer) => {
    const path = "/src/engine/runtime.ts";
    const { startEngine } = await import(/* @vite-ignore */ path);
    const host = document.createElement("div");
    host.style.cssText = "position:fixed;inset:0";
    document.body.append(host);
    let status = { phase: "starting", message: "" };
    const runtime = startEngine(
      host,
      (next: typeof status) => {
        status = next;
      },
      renderer,
    );
    Reflect.set(window, "hullProbe", { runtime, status: () => status });
  }, backend());
  const inspect = () =>
    page.evaluate(() => {
      const p = Reflect.get(window, "hullProbe");
      return {
        status: p.status() as { phase: string; message: string },
        scene: p.runtime.sceneStats() as SceneDiagnostics | null,
        assets: p.runtime.assetStats() as Record<string, number>,
      };
    });
  const ready = async () => {
    await expect
      .poll(
        async () => {
          const current = await inspect();
          if (current.status.phase === "error") throw new Error(current.status.message);
          return current.status.phase;
        },
        { timeout: 70_000 },
      )
      .toBe("ready");
  };
  await ready();
  const baseline = (await inspect()).assets;
  const game = sceneGame("offworld-hulls");
  game.currentPlayer.pendingTileSelection = undefined;
  game.tradeFleets = { player: { total: 2, available: 2 } };
  game.projectFunding = [
    {
      id: "pf_orbital_station",
      name: "Orbital station",
      seats: [{}, {}, {}],
      seatOwners: [{}, {}, {}],
      isCompleted: true,
    },
  ] as NonNullable<typeof game.projectFunding>;
  await page.evaluate(async (game) => {
    const path = "/src/engine/scene.ts";
    const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
    Reflect.get(window, "hullProbe").runtime.setScene(toSceneSnapshot(game, null), "snapshot");
  }, game);
  await ready();
  const current = await inspect();
  const world = current.scene!.offworld;
  await writeFile(
    path.resolve("../tmp/bevy-scaffold", `phase5-offworld-${backend()}-input.json`),
    JSON.stringify(current, null, 2),
  );
  expect(world.ready).toBe(true);
  expect(world.warmFrames).toBe(2);
  expect(world.warmCamera).toBe(false);
  expect(world.ships).toBe(2);
  expect(world.stationParts).toBe(8);
  expect(
    world.stationMaterials.some((material) => Math.abs(material.transmission - 0.3) < 1e-5),
  ).toBe(true);
  expect(world.stationMaterials.some((material) => Math.abs(material.ior - 1.45) < 1e-5)).toBe(
    true,
  );
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-offworld-${backend()}-board.png`),
  });
  await page.mouse.move(640, 360);
  await page.mouse.down();
  await page.mouse.move(640 - 0.74 / 0.006, 360 - 0.394 / 0.006, { steps: 3 });
  await page.mouse.up();
  await page.evaluate(() => {
    const r = Reflect.get(window, "hullProbe").runtime;
    r.camera(-1);
    r.camera(-1);
  });
  const writes = (await inspect()).assets.gridUniformWrites;
  await expect
    .poll(async () => (await inspect()).assets.gridUniformWrites, { timeout: 15_000 })
    .toBeGreaterThan(writes + 1);
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-offworld-${backend()}-hulls.png`),
  });
  await page.evaluate(() =>
    Reflect.get(window, "hullProbe").runtime.setFocus({ type: "orbital-station" }),
  );
  await expect
    .poll(async () => (await inspect()).scene?.space.focus)
    .toEqual({ type: "orbital-station" });
  await ready();
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-offworld-${backend()}-station.png`),
  });
  expect((await inspect()).scene!.space.focus).toEqual({ type: "orbital-station" });
  await page.evaluate(() => Reflect.get(window, "hullProbe").runtime.setScene(null));
  await ready();
  await expect
    .poll(
      async () => {
        const stats = (await inspect()).assets;
        return [
          stats.models,
          stats.images,
          stats.meshes,
          stats.materials,
          stats.gpuImages,
          stats.gpuMeshes,
          stats.gpuMaterials,
        ];
      },
      { timeout: 15_000 },
    )
    .toEqual([
      baseline.models,
      baseline.images,
      baseline.meshes,
      baseline.materials,
      baseline.gpuImages,
      baseline.gpuMeshes,
      baseline.gpuMaterials,
    ]);
  expect((await inspect()).scene!.offworld.ships).toBe(0);
  await page.evaluate(() => Reflect.get(window, "hullProbe").runtime.dispose());
  expect(errors).toEqual([]);
});

for (const [layout, profile] of [
  ["desktop", { viewport: { width: 1280, height: 720 } }],
  ["iphone", devices["iPhone 14 landscape"]],
  ["android", devices["Pixel 7 landscape"]],
] as const) {
  test(`interacts with the authored orbital station on ${layout}`, async ({ browser, baseURL }) => {
    test.skip(!test.info().project.name.startsWith("engine"));
    test.setTimeout(150_000);
    const context = await referenceContext(browser, {
      ...profile,
      baseURL,
      reducedMotion: "reduce",
    });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.goto(`/tests/fixtures/engine.html?renderer=${backend()}`);
      const game = sceneGame("station-interaction");
      game.currentPlayer.pendingTileSelection = undefined;
      game.projectFunding = [
        {
          id: "pf_orbital_station",
          name: "Orbital station",
          seats: [{}, {}, {}],
          seatOwners: [{}, {}, {}],
          isCompleted: true,
        },
      ] as NonNullable<typeof game.projectFunding>;
      await mountTravelScene(page, game, layout !== "desktop");
      await page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ gp);
        useGameStore.setState({ isConnected: true });
      });
      const state = () =>
        page.evaluate(async () => {
          const ep = "/src/stores/engineStore.ts";
          const { useEngineStore } = await import(/* @vite-ignore */ ep);
          const s = useEngineStore.getState();
          if (s.status.phase === "error") {
            throw new Error(s.status.message);
          }
          return { status: s.status.phase, focus: s.focus, overlay: s.overlay };
        });
      await expect.poll(async () => (await state()).status, { timeout: 70_000 }).toBe("ready");
      const marker = page.locator("[data-station-marker]");
      await expect(marker).toHaveCount(1);
      const box = (await marker.boundingBox())!;
      expect(box.width).toBeGreaterThanOrEqual(44);
      expect(box.height).toBeGreaterThanOrEqual(44);
      if (layout === "desktop") {
        await marker.hover();
      } else {
        await marker.tap();
        expect((await state()).focus).toEqual({ type: "body", body: "mars" });
        await expect(marker).toHaveAttribute("aria-pressed", "true");
      }
      await expect(page.getByRole("tooltip")).toContainText("Completed");
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-station-${backend()}-${layout}-details.png`,
        ),
      });
      if (layout !== "desktop") {
        await page.touchscreen.tap(120, 150);
        await expect(page.getByRole("tooltip")).toHaveCount(0);
        await marker.tap();
        await expect(marker).toHaveAttribute("aria-pressed", "true");
      }
      await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
      await page.mouse.down();
      await page.mouse.move(box.x + box.width / 2 + 12, box.y + box.height / 2);
      await page.mouse.up();
      expect((await state()).focus).toEqual({ type: "body", body: "mars" });
      const longName = "Orbital station " + "M".repeat(112);
      await page.evaluate(async (name) => {
        const gp = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ gp);
        const current = useGameStore.getState().game;
        useGameStore.getState().setGame({
          ...current,
          projectFunding: current.projectFunding.map((p: { id: string }) => ({ ...p, name })),
        });
      }, longName);
      await expect(page.getByRole("tooltip")).toContainText(longName);
      const tooltipBox = (await page.getByRole("tooltip").boundingBox())!;
      const viewport = page.viewportSize()!;
      expect(tooltipBox.x).toBeGreaterThanOrEqual(0);
      expect(tooltipBox.y).toBeGreaterThanOrEqual(layout === "desktop" ? 60 : 40);
      expect(tooltipBox.x + tooltipBox.width).toBeLessThanOrEqual(viewport.width);
      expect(tooltipBox.y + tooltipBox.height).toBeLessThanOrEqual(viewport.height);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-station-${backend()}-${layout}-long.png`,
        ),
      });
      if (layout === "desktop") {
        await marker.click();
      } else {
        await marker.tap();
      }
      await expect.poll(async () => (await state()).focus).toEqual({ type: "orbital-station" });
      await expect.poll(async () => (await state()).status).toBe("ready");
      expect(await page.evaluate(() => Reflect.get(window, "travelAudioCount")())).toBe(1);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-station-${backend()}-${layout}-focused.png`,
        ),
      });
      await marker.focus();
      await expect(page.getByRole("tooltip")).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      await page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ gp);
        const current = useGameStore.getState().game;
        useGameStore.getState().setGame({ ...current, projectFunding: [] });
      });
      await expect(marker).toHaveCount(0);
      await page.evaluate(() => Reflect.get(window, "unmountTravelScene")());
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });
}

test("preserves live trade flights across fleet changes and settles public history", async ({
  page,
}) => {
  test.skip(!test.info().project.name.startsWith("engine"));
  test.setTimeout(180_000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/tests/fixtures/engine.html");
  await page.evaluate(
    async ({ renderer, wasmPath }) => {
      const module = await import(/* @vite-ignore */ wasmPath);
      const create = module.Engine.create;
      module.Engine.create = async (canvas: HTMLCanvasElement) => {
        const native = await create.call(module.Engine, canvas);
        Reflect.set(window, "fleetNative", { native, canvas });
        return native;
      };
      const path = "/src/engine/runtime.ts";
      const { startEngine } = await import(/* @vite-ignore */ path);
      const host = document.createElement("div");
      host.style.cssText = "position:fixed;inset:0";
      document.body.append(host);
      let status = { phase: "starting", message: "" };
      const runtime = startEngine(
        host,
        (next: typeof status) => {
          status = next;
        },
        renderer,
      );
      Reflect.set(window, "fleetProbe", { runtime, status: () => status });
    },
    {
      renderer: backend(),
      wasmPath: "/@fs" + path.resolve("../engine/pkg", backend(), "openmars_engine.js"),
    },
  );
  const inspect = () =>
    page.evaluate(() => {
      const p = Reflect.get(window, "fleetProbe");
      const status = p.status() as { phase: string; message: string };
      if (status.phase === "error") {
        throw new Error(status.message);
      }
      return {
        status: status.phase,
        scene: p.runtime.sceneStats() as SceneDiagnostics | null,
        assets: p.runtime.assetStats() as Record<string, number>,
      };
    });
  const ready = async () => {
    await expect.poll(async () => (await inspect()).status, { timeout: 70_000 }).toBe("ready");
  };
  const deliver = (game: ReturnType<typeof sceneGame>, delivery: "snapshot" | "live" = "live") =>
    page.evaluate(
      async ({ game, delivery }) => {
        const path = "/src/engine/scene.ts";
        const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
        Reflect.get(window, "fleetProbe").runtime.setScene(toSceneSnapshot(game, null), delivery);
      },
      { game, delivery },
    );
  await ready();
  const baseline = (await inspect()).assets;
  const game = sceneGame("live-fleet");
  game.currentPlayer.pendingTileSelection = undefined;
  game.tradeFleets = { player: { total: 2, available: 2 } };
  game.colonies = [
    { id: "Titan", location: "venus", tradedThisGen: false, traderId: "" },
  ] as NonNullable<typeof game.colonies>;
  await deliver(game, "snapshot");
  await ready();
  const preparedEffects = (await inspect()).scene!.offworld.effects;
  expect(preparedEffects.warmNodes).toBe(0);
  expect(preparedEffects.materials).toBe(5);
  expect(preparedEffects.entries).toEqual([]);
  const first = (await inspect()).scene!.offworld.fleet;
  expect(first.map((ship) => ship.status)).toEqual(["Parked", "Parked"]);
  const entity = first[0].entity;
  // The same production camera input exposes the authored launch pads.
  await page.mouse.move(640, 360);
  await page.mouse.down();
  await page.mouse.move(640 - 0.74 / 0.006, 360 - 0.394 / 0.006, { steps: 3 });
  await page.mouse.up();
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.camera(-20, false);
  });
  const writes = (await inspect()).assets.gridUniformWrites;
  await expect
    .poll(async () => (await inspect()).assets.gridUniformWrites)
    .toBeGreaterThan(writes + 2);
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-fleet-${backend()}-parked.png`),
  });
  const publishFrozen = () =>
    page.evaluate(async () => {
      const { native, canvas } = Reflect.get(window, "fleetNative");
      const before = JSON.parse(native.asset_stats());
      for (let i = 0; i < 1200; i++) {
        const ready = native.frame(canvas.width, canvas.height, 0);
        const after = JSON.parse(native.asset_stats());
        if (ready && after.gridUniformWrites > before.gridUniformWrites + 2) {
          if (after.motionStep !== before.motionStep) {
            throw new Error("Paused view advanced the flight clock");
          }
          return JSON.parse(native.scene_stats()) as SceneDiagnostics;
        }
        await new Promise(requestAnimationFrame);
      }
      throw new Error("Frozen native view did not become ready");
    });
  const aimAtHull = async (
    view: SceneDiagnostics,
    distance: number,
    frozen = false,
    position = view.offworld.fleet[0].position,
  ) => {
    const radius = Math.hypot(...position);
    const yaw = Math.atan2(position[0], position[2]);
    const pitch = Math.asin(position[1] / radius);
    if (frozen) {
      await page.evaluate(
        async ({ yaw, pitch, zoom }) => {
          const { native, canvas } = Reflect.get(window, "fleetNative");
          const before = JSON.parse(native.asset_stats());
          native.camera_input(yaw, pitch, 0, false);
          let remaining = zoom;
          while (Math.abs(remaining) > 1) {
            const part = Math.sign(remaining);
            native.camera_input(0, 0, part, false);
            remaining -= part;
          }
          native.camera_input(0, 0, remaining, false);
          // The production native camera API publishes the paused pose at delta0.
          for (let i = 0; i < 600; i++) {
            native.frame(canvas.width, canvas.height, 0);
            const after = JSON.parse(native.asset_stats());
            if (after.gridUniformWrites > before.gridUniformWrites + 2) {
              if (after.motionStep !== before.motionStep) {
                throw new Error("Paused capture advanced the flight clock");
              }
              return;
            }
            await new Promise(requestAnimationFrame);
          }
          throw new Error("Paused native camera did not publish");
        },
        {
          yaw: yaw - view.camera.yaw,
          pitch: pitch - view.camera.pitch,
          zoom: Math.log(distance / view.camera.distance),
        },
      );
      return;
    }
    await page.mouse.move(640, 360);
    await page.mouse.down();
    await page.mouse.move(
      640 - (yaw - view.camera.yaw) / 0.006,
      360 + (pitch - view.camera.pitch) / 0.006,
      { steps: 3 },
    );
    await page.mouse.up();
    await page.evaluate(
      (zoom) => {
        Reflect.get(window, "fleetProbe").runtime.camera(zoom);
      },
      Math.log(distance / view.camera.distance),
    );
    const writes = (await inspect()).assets.gridUniformWrites;
    await expect
      .poll(async () => (await inspect()).assets.gridUniformWrites)
      .toBeGreaterThan(writes + 2);
  };
  await aimAtHull((await inspect()).scene!, 2.5);
  await page.emulateMedia({ reducedMotion: "no-preference" });
  game.colonies[0].tradedThisGen = true;
  game.colonies[0].traderId = "player";
  game.tradeFleets.player.available = 1;
  await deliver(game);
  await expect.poll(async () => (await inspect()).scene!.offworld.fleet[0].status).toBe("Flying");
  await expect.poll(async () => (await inspect()).scene!.offworld.fleet[0].started).not.toBeNull();
  const departure = (await inspect()).scene!.offworld.fleet[0];
  expect(departure.entity).toBe(entity);
  expect(departure.destination).toBe("venus");
  expect(departure.started).not.toBeNull();
  const effectEntry = (await inspect()).scene!.offworld.effects.entries[0];
  expect(effectEntry.complete).toBe(true);
  expect(effectEntry.groups.map((g) => g.capacity)).toEqual([600, 100, 60, 1, 20]);
  expect(effectEntry.groups.map((g) => g.instances)).toEqual([600, 100, 60, 1, 20]);
  const pauseAt = (age: number) =>
    page.evaluate(
      async ({ age, start }) => {
        const { runtime } = Reflect.get(window, "fleetProbe");
        for (let i = 0; i < 6000; i++) {
          const step = runtime.assetStats().motionStep;
          if (step >= start + age) {
            runtime.setPaused(true);
            return { age: step - start, scene: runtime.sceneStats() as SceneDiagnostics };
          }
          await new Promise(requestAnimationFrame);
        }
        throw new Error("Live ship did not reach the requested capture phase");
      },
      { age, start: departure.started! },
    );
  const liftoff = await pauseAt(35);
  expect(
    liftoff.scene.offworld.effects.entries[0].groups.find((g) => g.kind === "Hover")!.visible,
  ).toBeGreaterThan(0);
  expect(liftoff.age).toBeLessThan(72);
  expect(Math.hypot(...liftoff.scene.offworld.fleet[0].position)).toBeCloseTo(
    Math.hypot(...first[0].position) + 0.15 * (liftoff.age / 72) ** 2,
    10,
  );
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-fleet-${backend()}-liftoff.png`),
  });
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.setPaused(false);
  });
  game.tradeFleets.player.total = 3;
  game.tradeFleets.player.available = 2;
  game.currentPlayer.color = "#00ff00";
  await deliver(game);
  await expect.poll(async () => (await inspect()).scene!.offworld.fleet.length).toBe(3);
  const continued = (await inspect()).scene!.offworld.fleet[0];
  expect(continued.status).toBe("Flying");
  expect(continued.entity).toBe(entity);
  expect(continued.started).toBe(departure.started);
  expect(continued.serial).toBe(departure.serial);
  const arc = await pauseAt(180);
  const arcGroups = arc.scene.offworld.effects.entries[0].groups;
  expect(arcGroups.find((g) => g.kind === "Trail")!.visible).toBeGreaterThan(0);
  expect(arcGroups.find((g) => g.kind === "Flame")!.visible).toBeGreaterThan(0);
  expect(arcGroups.find((g) => g.kind === "Core")!.visible).toBe(1);
  await aimAtHull(arc.scene, Math.hypot(...arc.scene.offworld.fleet[0].position) + 0.7, true);
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-fleet-${backend()}-flight.png`),
  });
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.setPaused(false);
  });
  const exhaust = await pauseAt(1155);
  expect(
    exhaust.scene.offworld.effects.entries[0].groups.find((g) => g.kind === "Exhaust")!.visible,
  ).toBeGreaterThan(0);
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.setFocus({ type: "body", body: "venus" });
  });
  const exhaustView = await publishFrozen();
  const destination = exhaustView.space.bodies.find((b) => b.body === "venus")!;
  const direction = exhaustView.offworld.fleet[0].position.map(
    (v, i) => v - destination.position[i],
  );
  const [vx, vy, vz] = direction;
  const [dx, dy, dz, dw] = destination.rotation.map((v, i) => (i < 3 ? -v : v));
  const ux = 2 * (dy * vz - dz * vy),
    uy = 2 * (dz * vx - dx * vz),
    uz = 2 * (dx * vy - dy * vx);
  const localDirection = [
    vx + dw * ux + dy * uz - dz * uy,
    vy + dw * uy + dz * ux - dx * uz,
    vz + dw * uz + dx * uy - dy * ux,
  ];
  expect(Math.hypot(...direction)).toBeGreaterThan(3);
  expect(Math.hypot(...direction)).toBeLessThan(3.021);
  await aimAtHull(exhaustView, 4, true, localDirection);
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-ship-effects-${backend()}-landing.png`),
  });
  await writeFile(
    path.resolve("../tmp/bevy-scaffold", `phase5-ship-effects-${backend()}-landing.json`),
    JSON.stringify(await inspect(), null, 2),
  );
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.setPaused(false);
  });
  await expect
    .poll(async () => (await inspect()).scene!.offworld.fleet[0].status, { timeout: 45_000 })
    .toBe("Landed");
  const landed = (await inspect()).scene!.offworld.fleet[0];
  expect(landed.entity).toBe(entity);
  expect(landed.destination).toBe("venus");
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.setFocus({ type: "body", body: "venus" });
  });
  await ready();
  const landingView = (await inspect()).scene!;
  const venus = landingView.space.bodies.find((b) => b.body === "venus")!;
  const vector = landingView.offworld.fleet[0].position.map((v, i) => v - venus.position[i]);
  const [x, y, z] = vector;
  const [qx, qy, qz, qw] = venus.rotation.map((v, i) => (i < 3 ? -v : v));
  const tx = 2 * (qy * z - qz * y),
    ty = 2 * (qz * x - qx * z),
    tz = 2 * (qx * y - qy * x);
  const local = [
    x + qw * tx + qy * tz - qz * ty,
    y + qw * ty + qz * tx - qx * tz,
    z + qw * tz + qx * ty - qy * tx,
  ];
  const yaw = Math.atan2(local[0], local[2]);
  const pitch = Math.asin(local[1] / Math.hypot(...local));
  await page.mouse.move(640, 360);
  await page.mouse.down();
  await page.mouse.move(
    640 - (yaw - landingView.camera.yaw) / 0.006,
    360 + (pitch - landingView.camera.pitch) / 0.006,
    { steps: 4 },
  );
  await page.mouse.up();
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.camera(-20, false);
  });
  const landingWrites = (await inspect()).assets.gridUniformWrites;
  await expect
    .poll(async () => (await inspect()).assets.gridUniformWrites)
    .toBeGreaterThan(landingWrites + 2);
  await page.screenshot({
    path: path.resolve("../tmp/bevy-scaffold", `phase5-fleet-${backend()}-landed.png`),
  });
  await writeFile(
    path.resolve("../tmp/bevy-scaffold", `phase5-fleet-${backend()}-landed.json`),
    JSON.stringify(await inspect(), null, 2),
  );
  await deliver(game, "snapshot");
  await ready();
  const history = (await inspect()).scene!.offworld.fleet[0];
  expect(history.status).toBe("Landed");
  expect(history.started).toBeNull();
  expect((await inspect()).scene!.offworld.effects.entries).toEqual([]);
  expect(history.entity).toBe(entity);
  game.generation += 1;
  game.colonies[0].tradedThisGen = false;
  game.colonies[0].traderId = "";
  game.tradeFleets.player.available = 3;
  await deliver(game);
  await expect
    .poll(async () => (await inspect()).scene!.offworld.fleet.map((s) => s.status))
    .toEqual(["Parked", "Parked", "Parked"]);
  game.colonies[0].tradedThisGen = true;
  game.colonies[0].traderId = "player";
  game.tradeFleets.player.available = 2;
  await deliver(game);
  await expect.poll(async () => (await inspect()).scene!.offworld.fleet[0].status).toBe("Flying");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect.poll(async () => (await inspect()).scene!.offworld.fleet[0].status).toBe("Landed");
  expect((await inspect()).scene!.offworld.effects.entries).toEqual([]);
  const quiet = (await inspect()).scene!.offworld.fleet[0];
  expect(quiet.started).toBeNull();
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await expect
    .poll(async () => (await inspect()).assets.motionStep)
    .toBeGreaterThan((await inspect()).assets.motionStep + 5);
  expect((await inspect()).scene!.offworld.fleet[0].status).toBe("Landed");
  // An unavailable hull with no public trade destination is hidden, never sent somewhere invented.
  game.colonies = [];
  game.tradeFleets.player.available = 2;
  await deliver(game, "snapshot");
  await ready();
  expect((await inspect()).scene!.offworld.fleet.map((s) => s.status)).toEqual([
    "Parked",
    "Parked",
    "Unavailable",
  ]);
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.setScene(null);
  });
  await ready();
  await expect
    .poll(
      async () => {
        const a = (await inspect()).assets;
        return [
          a.models,
          a.images,
          a.meshes,
          a.materials,
          a.gpuImages,
          a.gpuMeshes,
          a.gpuMaterials,
        ];
      },
      { timeout: 15_000 },
    )
    .toEqual([
      baseline.models,
      baseline.images,
      baseline.meshes,
      baseline.materials,
      baseline.gpuImages,
      baseline.gpuMeshes,
      baseline.gpuMaterials,
    ]);
  await page.evaluate(() => {
    Reflect.get(window, "fleetProbe").runtime.dispose();
  });
  expect(errors).toEqual([]);
});

for (const [layout, profile] of [
  ["desktop", { viewport: { width: 1280, height: 720 } }],
  ["iphone", devices["iPhone 14 landscape"]],
  ["android", devices["Pixel 7 landscape"]],
] as const) {
  test(`reveals ships and completes stations from real React phases on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.skip(!test.info().project.name.startsWith("engine"));
    test.setTimeout(240_000);
    const context = await referenceContext(browser, {
      ...profile,
      baseURL,
      reducedMotion: "no-preference",
    });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.goto(
        `/tests/fixtures/engine.html?renderer=${backend()}${layout === "desktop" ? "" : "&compact"}`,
      );
      // Hold only the animation clock. React, asset loading, GPU frames and readiness remain real.
      await page.evaluate(
        async (wasmPath) => {
          const module = await import(/* @vite-ignore */ wasmPath);
          const create = module.Engine.create;
          module.Engine.create = async (canvas: HTMLCanvasElement) => {
            const native = await create.call(module.Engine, canvas);
            const frame = native.frame.bind(native);
            native.frame = (width: number, height: number) => frame(width, height, 0);
            Reflect.set(window, "entryNative", { native, canvas, frame });
            return native;
          };
        },
        "/@fs" + path.resolve("../engine/pkg", backend(), "openmars_engine.js"),
      );
      const game = labelSceneGame();
      game.id = "offworld-entry";
      // Published special children share the Tile scale; their GPU warm draws do not.
      for (const [q, r, type, seed] of [
        [-1, 1, "mining-tile", 37],
        [-1, 2, "mohole-tile", 77],
        [1, -1, "volcano-tile", 37],
        [0, 2, "world-tree-tile", 37],
        [2, -2, "nuclear-zone-tile", 37],
      ] as const) {
        const tile = game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === r)!;
        tile.occupiedBy = { type, tags: [], visual: { seed } };
      }
      game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === -1)!.reservedBy =
        "player";
      game.board.tiles.push({
        coordinates: { q: 200, r: 0, s: -200 },
        location: "venus",
        type: "land",
        tags: [],
        bonuses: [{ type: "steel", amount: 1 }],
        displayName: "Venus Outpost",
      });
      game.currentPlayer.pendingTileSelection = undefined;
      const captionTile = game.board.tiles.find(
        (t) => t.coordinates.q === 0 && t.coordinates.r === 0,
      )!;
      captionTile.displayName = "Arsia Mons";
      captionTile.bonuses = [
        { type: "credit", amount: 3 },
        { type: "steel", amount: 1 },
      ];
      game.tradeFleets = { player: { total: 2, available: 2 } };
      game.projectFunding = [
        {
          id: "pf_orbital_station",
          name: "Orbital station",
          seats: [{}, {}, {}],
          seatOwners: [{}, {}, {}],
          isCompleted: false,
        },
      ] as NonNullable<typeof game.projectFunding>;
      await mountTravelScene(page, game, layout !== "desktop", "menu");
      await page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ gp);
        useGameStore.setState({ isConnected: true });
      });
      const inspect = () =>
        page.evaluate(async () => {
          const ep = "/src/stores/engineStore.ts";
          const { useEngineStore } = await import(/* @vite-ignore */ ep);
          const state = useEngineStore.getState();
          if (state.status.phase === "error") {
            throw new Error(state.status.message);
          }
          const n = Reflect.get(window, "entryNative");
          return {
            status: state.status.phase,
            scene: n ? (JSON.parse(n.native.scene_stats()) as SceneDiagnostics) : null,
            assets: n ? (JSON.parse(n.native.asset_stats()) as Record<string, number>) : null,
          };
        });
      const ready = async () => {
        await expect.poll(async () => (await inspect()).status, { timeout: 70_000 }).toBe("ready");
      };
      const publish = () =>
        page.evaluate(async () => {
          const { native, canvas, frame } = Reflect.get(window, "entryNative");
          const before = JSON.parse(native.asset_stats());
          for (let i = 0; i < 1200; i++) {
            const ready = frame(canvas.width, canvas.height, 0);
            const after = JSON.parse(native.asset_stats());
            if (ready && after.gridUniformWrites > before.gridUniformWrites + 2) {
              if (after.motionStep !== before.motionStep) {
                throw new Error("Publication advanced animation time");
              }
              return JSON.parse(native.scene_stats()) as SceneDiagnostics;
            }
            await new Promise(requestAnimationFrame);
          }
          throw new Error("Entry view did not publish");
        });
      const advance = async (steps: number) => {
        await page.evaluate(async (steps) => {
          const { native, canvas, frame } = Reflect.get(window, "entryNative");
          const target = JSON.parse(native.asset_stats()).motionStep + steps;
          for (let i = 0; i < steps * 8 + 120; i++) {
            const current = JSON.parse(native.asset_stats()).motionStep;
            if (current === target) {
              return;
            }
            if (current > target) {
              throw new Error("Animation clock overshot its target");
            }
            frame(canvas.width, canvas.height, Math.min(6, target - current) / 60);
            await new Promise(requestAnimationFrame);
          }
          throw new Error("Entry clock did not advance");
        }, steps);
        return publish();
      };
      const phase = (kind: "loading" | "fadeOutLobby" | "marsRevealed" | "playing") =>
        page.evaluate(async (kind) => {
          const pp = "/src/stores/appPhaseStore.ts",
            gp = "/src/stores/gameStore.ts";
          const [{ useAppPhaseStore }, { useGameStore }] = await Promise.all([
            import(/* @vite-ignore */ pp),
            import(/* @vite-ignore */ gp),
          ]);
          useAppPhaseStore.getState().setPhase({ kind, gameId: useGameStore.getState().game.id });
        }, kind);
      const complete = (value: boolean, connected = true) =>
        page.evaluate(
          async ({ value, connected }) => {
            const gp = "/src/stores/gameStore.ts";
            const { useGameStore } = await import(/* @vite-ignore */ gp);
            const game = useGameStore.getState().game;
            useGameStore.setState({
              isConnected: connected,
              game: {
                ...game,
                projectFunding: game.projectFunding.map((p: { isCompleted: boolean }) => ({
                  ...p,
                  isCompleted: value,
                })),
              },
            });
          },
          { value, connected },
        );
      await ready();
      const baseline = (await inspect()).assets!;
      await phase("loading");
      await expect
        .poll(async () => (await inspect()).scene!.offworld.ships, { timeout: 70_000 })
        .toBe(2);
      await ready();
      let view = await publish();
      expect(view.offworld.entranceScale).toBe(0);
      expect(view.offworld.entranceStart).toBeNull();
      expect(view.offworld.stationScale).toBe(0);
      expect(view.offworld.ships).toBe(2);
      expect(view.structures.plots).toBe(1);
      expect(view.worldTrees.trees).toBe(1);
      expect(view.worksites.sites).toBe(2);
      expect(view.specialSurfaces.sites).toBe(2);
      expect(view.craters.sites).toBe(1);
      const actorRoots = (value: SceneDiagnostics) => [
        ...value.worldTrees.sample,
        ...value.worksites.sample,
        ...value.specialSurfaces.sample,
      ];
      for (const actor of actorRoots(view)) {
        expect(view.entryActors.find((e) => e.entity === actor.entity)).toMatchObject({
          scale: 0,
          hidden: true,
        });
      }
      expect(view.craters.sample[0].entryScale).toBe(0);
      expect(view.entry.light).toBe(0);
      expect(view.entry.tileScales).toEqual([0, 0, 0]);
      expect(view.labels.entries.find((e) => e.id === "200,0,-200")!.scale).toBe(1);
      const dark = await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-world-entry-${backend()}-${layout}-dark.png`,
        ),
      });
      await phase("fadeOutLobby");
      await expect.poll(async () => (await inspect()).scene!.entry.lightStart).not.toBeNull();
      view = await advance(75);
      expect(view.entry.light).toBeCloseTo(0.75, 6);
      expect(view.entry.tileScales).toEqual([0, 0, 0]);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-world-entry-${backend()}-${layout}-sunrise.png`,
        ),
      });
      view = await advance(75);
      expect(view.entry.light).toBe(1);
      expect(view.entry.tileScales).toEqual([0, 0, 0]);
      const lit = await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-world-entry-${backend()}-${layout}-lit.png`,
        ),
      });
      const centerPoint = view.tiles.find((t) => t.id === "0,0,0")!.point!;
      const brightness = async (bytes: Buffer) => {
        const metadata = await sharp(bytes).metadata();
        const scale = metadata.width! / page.viewportSize()!.width;
        const pixels = await sharp(bytes)
          .extract({
            left: Math.round(centerPoint[0] * scale) - 6,
            top: Math.round(centerPoint[1] * scale) - 6,
            width: 12,
            height: 12,
          })
          .removeAlpha()
          .raw()
          .toBuffer();
        return pixels.reduce((sum, n) => sum + n, 0) / pixels.length;
      };
      expect(await brightness(lit)).toBeGreaterThan((await brightness(dark)) + 10);
      const caption = (view: SceneDiagnostics) =>
        view.labels.entries.find((e) => e.id === "0,0,0")!;
      expect(caption(view).scale).toBe(0);
      expect(caption(view).name).toBe(layout === "desktop" ? "Arsia Mons" : null);
      expect(caption(view).icons).toBe(2);
      const captionRoot = caption(view).root;
      const owner = view.offworld.root,
        station = view.offworld.stationEntity;
      await page.evaluate(() => {
        const { native } = Reflect.get(window, "entryNative");
        native.camera_input(0.74, -0.394, 0, false);
        native.camera_input(0, 0, -1, false);
        native.camera_input(0, 0, -1, false);
      });
      await publish();
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-entry-${backend()}-${layout}-hidden.png`,
        ),
      });
      await phase("marsRevealed");
      await expect.poll(async () => (await inspect()).scene!.offworld.entranceStart).not.toBeNull();
      // Board ordinal30:450ms delay,400ms cubic growth. Ship delay is separate.
      expect(caption(await advance(12)).scale).toBe(0);
      view = await advance(18);
      expect(
        caption(view).scale,
        JSON.stringify({
          entry: view.entry,
          settling: view.settling,
          caption: caption(view),
          ships: view.offworld.entranceStart,
          actors: view.entryActors,
          labelsReady: view.labels.ready,
          treesReady: view.worldTrees.ready,
          sitesReady: view.worksites.ready,
          specialReady: view.specialSurfaces.ready,
          cratersReady: view.craters.ready,
          camera: view.camera,
          terrain: view.terrain,
        }),
      ).toBeCloseTo(1 - 0.875 ** 3, 5);
      expect(view.entry.tileScales[1]).toBeCloseTo(caption(view).scale!, 6);
      const marsTiles = game.board.tiles.filter((t) => t.location === "mars" && t.type !== "empty");
      for (const actor of actorRoots(view)) {
        const ordinal = marsTiles.findIndex(
          (t) => t.coordinates.q === actor.coordinate[0] && t.coordinates.r === actor.coordinate[1],
        );
        const progress = Math.max(0, Math.min(1, (500 - ordinal * 15) / 400));
        const scale = 1 - (1 - progress) ** 3;
        const actual = view.entryActors.find((e) => e.entity === actor.entity)!;
        expect(actual.scale).toBeCloseTo(scale, 5);
        expect(actual.hidden).toBe(scale === 0);
      }
      const restoreCaptionCamera = view.camera;
      await page.evaluate(() => {
        const { native } = Reflect.get(window, "entryNative");
        native.camera_input(0, 0, 0, true);
        native.camera_input(0, 0, -0.7, false);
      });
      await publish();
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-label-entry-${backend()}-${layout}-growing.png`,
        ),
      });
      view = await advance(12);
      const volcano = view.specialSurfaces.sample.find((s) => s.kind === "Volcano")!;
      // Original ordinal38:570ms delay, then130ms of its400ms scale-in.
      expect(view.entryActors.find((a) => a.entity === volcano.entity)!.scale).toBeCloseTo(
        1 - 0.675 ** 3,
        5,
      );
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-world-entry-${backend()}-${layout}-growing-actors.png`,
        ),
      });
      await page.evaluate((camera) => {
        const { native } = Reflect.get(window, "entryNative");
        const view = JSON.parse(native.scene_stats()).camera;
        native.camera_input(camera.yaw - view.yaw, camera.pitch - view.pitch, 0, false);
        let zoom = Math.log(camera.distance / view.distance);
        while (Math.abs(zoom) > 1) {
          native.camera_input(0, 0, Math.sign(zoom), false);
          zoom -= Math.sign(zoom);
        }
        native.camera_input(0, 0, zoom, false);
      }, restoreCaptionCamera);
      await publish();
      view = await advance(30);
      expect(view.offworld.entranceScale).toBe(0);
      expect(caption(view).scale).toBe(1);
      view = await advance(12);
      expect(view.offworld.entranceScale).toBeCloseTo(0.875, 5);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-entry-${backend()}-${layout}-growing.png`,
        ),
      });
      view = await advance(12);
      expect(view.offworld.entranceScale).toBe(1);
      await phase("playing");
      view = await publish();
      for (const actor of actorRoots(view)) {
        expect(view.entryActors.find((e) => e.entity === actor.entity)).toMatchObject({
          scale: 1,
          hidden: false,
        });
      }
      expect(view.craters.sample[0].entryScale).toBe(1);
      const restoreWorldCamera = view.camera;
      await page.evaluate(() => {
        const { native } = Reflect.get(window, "entryNative");
        native.camera_input(0, 0, 0, true);
        native.camera_input(0, 0, -0.3, false);
      });
      await publish();
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-world-entry-${backend()}-${layout}-actors.png`,
        ),
      });
      await page.evaluate((camera) => {
        const { native } = Reflect.get(window, "entryNative");
        const current = JSON.parse(native.scene_stats()).camera;
        native.camera_input(camera.yaw - current.yaw, camera.pitch - current.pitch, 0, false);
        let zoom = Math.log(camera.distance / current.distance);
        while (Math.abs(zoom) > 1) {
          native.camera_input(0, 0, Math.sign(zoom), false);
          zoom -= Math.sign(zoom);
        }
        native.camera_input(0, 0, zoom, false);
      }, restoreWorldCamera);
      await publish();
      expect(view.offworld.entranceStart).toBeNull();
      expect(view.offworld.root).toBe(owner);
      expect(caption(view).root).toBe(captionRoot);
      expect(caption(view).scale).toBe(1);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-entry-${backend()}-${layout}-settled.png`,
        ),
      });
      await complete(true);
      await expect.poll(async () => (await inspect()).scene!.offworld.stationStart).not.toBeNull();
      expect((await publish()).offworld.stationScale).toBe(0);
      await page.evaluate(async () => {
        const ep = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ ep);
        useEngineStore.getState().setFocus({ type: "orbital-station" });
      });
      await ready();
      await publish();
      view = await advance(25);
      expect(view.offworld.stationScale).toBeCloseTo(0.875, 5);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-completion-${backend()}-${layout}-growing.png`,
        ),
      });
      view = await advance(25);
      expect(view.offworld.stationScale).toBe(1);
      expect(view.offworld.stationEntity).toBe(station);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-completion-${backend()}-${layout}-settled.png`,
        ),
      });
      await complete(false);
      await expect.poll(async () => (await inspect()).scene!.offworld.stationScale).toBe(0);
      await complete(true, false);
      await expect.poll(async () => (await inspect()).scene!.offworld.stationScale).toBe(1);
      await ready();
      view = await publish();
      expect(view.offworld.stationScale).toBe(1);
      expect(view.offworld.stationStart).toBeNull();
      await complete(false, true);
      await ready();
      await expect.poll(async () => (await inspect()).scene!.offworld.stationScale).toBe(0);
      await page.emulateMedia({ reducedMotion: "reduce" });
      await complete(true);
      await expect.poll(async () => (await inspect()).scene!.offworld.stationScale).toBe(1);
      expect((await publish()).offworld.stationStart).toBeNull();
      await page.emulateMedia({ reducedMotion: "no-preference" });
      expect((await advance(6)).offworld.stationStart).toBeNull();
      const colonies = (enabled: boolean) =>
        page.evaluate(async (enabled) => {
          const gp = "/src/stores/gameStore.ts";
          const { useGameStore } = await import(/* @vite-ignore */ gp);
          const game = useGameStore.getState().game;
          useGameStore.getState().setGame({
            ...game,
            settings: { ...game.settings, systems: enabled ? ["colonies"] : [] },
          });
        }, enabled);
      await colonies(false);
      await expect.poll(async () => (await inspect()).scene!.offworld.ships).toBe(0);
      await ready();
      view = await publish();
      expect(view.offworld.fleet).toEqual([]);
      expect(view.offworld.stationParts).toBe(8);
      expect(view.offworld.stationEntity).toBe(station);
      expect(view.offworld.root).toBe(owner);
      expect(view.offworld.stationScale).toBe(1);
      await colonies(true);
      await expect.poll(async () => (await inspect()).scene!.offworld.ships).toBe(2);
      await ready();
      view = await publish();
      expect(view.offworld.fleet.map((s) => s.status)).toEqual(["Parked", "Parked"]);
      expect(view.offworld.fleet.every((s) => s.started === null)).toBe(true);
      expect(view.offworld.stationEntity).toBe(station);
      // Replacement owns new objects; already-completed public history never replays a birth.
      await page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts",
          pp = "/src/stores/appPhaseStore.ts";
        const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
          import(/* @vite-ignore */ gp),
          import(/* @vite-ignore */ pp),
        ]);
        const game = { ...useGameStore.getState().game, id: "offworld-replacement" };
        useGameStore.getState().setGame(game);
        useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
      });
      await expect
        .poll(async () => (await inspect()).scene!.offworld.root, { timeout: 70_000 })
        .not.toBe(owner);
      await ready();
      view = await publish();
      expect(view.offworld.root).not.toBeNull();
      expect(view.offworld.stationEntity).not.toBe(station);
      expect(view.offworld.stationScale).toBe(1);
      expect(view.offworld.stationStart).toBeNull();
      expect(view.offworld.entranceScale).toBe(0);
      view = await advance(12);
      expect(view.entry.tileScales[0]).toBeCloseTo(0.875, 6);
      // A reconnect cancels the partial world entrance without replaying it.
      await complete(false, false);
      await ready();
      view = await publish();
      expect(view.entry.tileScales).toEqual([1, 1, 1]);
      expect(view.entry.light).toBe(1);
      for (const actor of actorRoots(view)) {
        expect(view.entryActors.find((e) => e.entity === actor.entity)!.scale).toBe(1);
      }
      await complete(true, true);
      await ready();
      // Reduced motion settles an entrance already underway, including sunrise.
      await phase("loading");
      await expect.poll(async () => (await inspect()).scene!.entry.light).toBe(0);
      await phase("marsRevealed");
      await expect.poll(async () => (await inspect()).scene!.offworld.entranceStart).not.toBeNull();
      view = await advance(6);
      expect(view.entry.tileScales[0]).toBeCloseTo(0.578125, 6);
      expect(view.entry.light).toBeGreaterThan(0);
      expect(view.entry.light).toBeLessThan(1);
      await page.emulateMedia({ reducedMotion: "reduce" });
      await expect.poll(async () => (await inspect()).scene!.entry.light).toBe(1);
      view = await publish();
      expect(view.entry.tileScales).toEqual([1, 1, 1]);
      expect(view.offworld.entranceScale).toBe(1);
      expect(view.craters.sample[0].entryScale).toBe(1);
      for (const actor of actorRoots(view)) {
        expect(view.entryActors.find((e) => e.entity === actor.entity)).toMatchObject({
          scale: 1,
          hidden: false,
        });
      }
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-world-entry-${backend()}-${layout}-reduced.png`,
        ),
      });
      await page.emulateMedia({ reducedMotion: "no-preference" });
      view = await advance(6);
      expect(view.entry.tileScales).toEqual([1, 1, 1]);
      expect(view.entry.light).toBe(1);
      expect(view.offworld.entranceScale).toBe(1);
      await phase("playing");
      await publish();
      const aim = async (view: SceneDiagnostics, position: number[], distance: number) => {
        const radius = Math.hypot(...position);
        await page.evaluate(
          ({ yaw, pitch, zoom }) => {
            const { native } = Reflect.get(window, "entryNative");
            native.camera_input(yaw, pitch, 0, false);
            let remaining = zoom;
            while (Math.abs(remaining) > 1) {
              const part = Math.sign(remaining);
              native.camera_input(0, 0, part, false);
              remaining -= part;
            }
            native.camera_input(0, 0, remaining, false);
          },
          {
            yaw: Math.atan2(position[0], position[2]) - view.camera.yaw,
            pitch: Math.asin(position[1] / radius) - view.camera.pitch,
            zoom: Math.log(distance / view.camera.distance),
          },
        );
        return publish();
      };
      const startTrade = async () => {
        await page.evaluate(async () => {
          const gp = "/src/stores/gameStore.ts";
          const { useGameStore } = await import(/* @vite-ignore */ gp);
          const game = useGameStore.getState().game;
          useGameStore.getState().setGame({
            ...game,
            colonies: [{ id: "Titan", location: "venus", tradedThisGen: false, traderId: "" }],
            tradeFleets: { [game.currentPlayer.id]: { total: 2, available: 2 } },
          });
        });
        await expect
          .poll(async () => (await inspect()).scene!.offworld.fleet.map((s) => s.status))
          .toEqual(["Parked", "Parked"]);
        await publish();
        await page.evaluate(async () => {
          const gp = "/src/stores/gameStore.ts";
          const { useGameStore } = await import(/* @vite-ignore */ gp);
          const game = useGameStore.getState().game;
          useGameStore.getState().setGame({
            ...game,
            colonies: game.colonies.map((c: object) => ({
              ...c,
              tradedThisGen: true,
              traderId: game.currentPlayer.id,
            })),
            tradeFleets: { [game.currentPlayer.id]: { total: 2, available: 1 } },
          });
        });
        await expect
          .poll(async () => (await inspect()).scene!.offworld.fleet[0].started, { timeout: 30_000 })
          .not.toBeNull();
        const view = await publish();
        expect(view.offworld.effects.entries).toHaveLength(1);
        expect(view.offworld.effects.entries[0].groups.map((g) => g.instances)).toEqual([
          600, 100, 60, 1, 20,
        ]);
        return view;
      };
      await startTrade();
      const liftoff = await advance(35);
      expect(
        liftoff.offworld.effects.entries[0].groups.find((g) => g.kind === "Hover")!.visible,
      ).toBeGreaterThan(0);
      await aim(
        liftoff,
        liftoff.offworld.fleet[0].position,
        Math.hypot(...liftoff.offworld.fleet[0].position) + 0.6,
      );
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-offworld-${backend()}-${layout}-liftoff.png`,
        ),
      });
      const arc = await advance(145);
      for (const kind of ["Trail", "Flame", "Core"]) {
        expect(
          arc.offworld.effects.entries[0].groups.find((g) => g.kind === kind)!.visible,
        ).toBeGreaterThan(0);
      }
      await aim(
        arc,
        arc.offworld.fleet[0].position,
        Math.hypot(...arc.offworld.fleet[0].position) + 0.7,
      );
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-offworld-${backend()}-${layout}-arc.png`,
        ),
      });
      const landing = await advance(975);
      expect(
        landing.offworld.effects.entries[0].groups.find((g) => g.kind === "Exhaust")!.visible,
      ).toBeGreaterThan(0);
      await page.evaluate(async () => {
        const ep = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ ep);
        useEngineStore.getState().setFocus({ type: "body", body: "venus" });
      });
      await ready();
      const destinationView = await publish();
      const destination = destinationView.space.bodies.find((b) => b.body === "venus")!;
      const vector = destinationView.offworld.fleet[0].position.map(
        (v, i) => v - destination.position[i],
      );
      const [x, y, z] = vector;
      const [qx, qy, qz, qw] = destination.rotation.map((v, i) => (i < 3 ? -v : v));
      const tx = 2 * (qy * z - qz * y),
        ty = 2 * (qz * x - qx * z),
        tz = 2 * (qx * y - qy * x);
      const local = [
        x + qw * tx + qy * tz - qz * ty,
        y + qw * ty + qz * tx - qx * tz,
        z + qw * tz + qx * ty - qy * tx,
      ];
      await aim(destinationView, local, 4);
      await page.screenshot({
        path: path.resolve(
          "../tmp/bevy-scaffold",
          `phase5-offworld-${backend()}-${layout}-landing.png`,
        ),
      });
      // Replace a live 781-instance pool, then clear a second live pool.
      const activeRoot = destinationView.offworld.root;
      await page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts",
          pp = "/src/stores/appPhaseStore.ts";
        const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
          import(/* @vite-ignore */ gp),
          import(/* @vite-ignore */ pp),
        ]);
        const current = useGameStore.getState().game;
        const game = {
          ...current,
          id: "offworld-active-replacement",
          colonies: [],
          tradeFleets: { [current.currentPlayer.id]: { total: 2, available: 2 } },
        };
        useGameStore.getState().setGame(game);
        useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
      });
      await expect
        .poll(async () => (await inspect()).scene!.offworld.root, { timeout: 70_000 })
        .not.toBe(activeRoot);
      await ready();
      view = await publish();
      expect(view.offworld.root).not.toBeNull();
      expect(view.offworld.effects.entries).toEqual([]);
      expect(view.offworld.fleet.every((s) => s.status === "Parked")).toBe(true);
      expect((await advance(96)).offworld.entranceScale).toBe(1);
      await startTrade();
      expect((await advance(35)).offworld.effects.entries).toHaveLength(1);
      await page.evaluate(async () => {
        const gp = "/src/stores/gameStore.ts",
          pp = "/src/stores/appPhaseStore.ts";
        const [{ useGameStore }, { useAppPhaseStore }] = await Promise.all([
          import(/* @vite-ignore */ gp),
          import(/* @vite-ignore */ pp),
        ]);
        useAppPhaseStore.getState().setPhase({ kind: "menu", route: "landing" });
        useGameStore.getState().setGame(null);
      });
      await ready();
      await expect.poll(async () => (await inspect()).scene!.offworld.root).toBeNull();
      const stores = [
        "models",
        "images",
        "meshes",
        "materials",
        "gpuImages",
        "gpuMeshes",
        "gpuMaterials",
      ];
      await expect
        .poll(
          async () => {
            const assets = (await inspect()).assets!;
            return stores.map((key) => assets[key]);
          },
          { timeout: 15_000 },
        )
        .toEqual(stores.map((key) => baseline[key]));
      const cleared = await inspect();
      expect(cleared.scene!.offworld.stationEntity).toBeNull();
      expect(cleared.scene!.offworld.effects.entries).toEqual([]);
      await writeFile(
        path.resolve("../tmp/bevy-scaffold", `phase5-entry-${backend()}-${layout}.json`),
        JSON.stringify({ view, cleared }, null, 2),
      );
      await page.evaluate(() => Reflect.get(window, "unmountTravelScene")());
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });
}

for (const [layout, profile] of [
  ["desktop", { viewport: { width: 1280, height: 720 } }],
  ["iphone", devices["iPhone 14 landscape"]],
  ["android", devices["Pixel 7 landscape"]],
] as const) {
  test(`renders native tile typography and bonus icons on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.skip(!test.info().project.name.startsWith("engine"));
    test.setTimeout(180_000);
    const context = await referenceContext(browser, {
      ...profile,
      baseURL,
      reducedMotion: "reduce",
    });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.goto(
        `/tests/fixtures/engine.html?renderer=${backend()}${layout === "desktop" ? "" : "&compact"}`,
      );
      await page.evaluate(async (renderer) => {
        const path = "/src/engine/runtime.ts";
        const { startEngine } = await import(/* @vite-ignore */ path);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        let status = { phase: "starting", message: "" };
        const runtime = startEngine(
          host,
          (next: typeof status) => {
            status = next;
          },
          renderer,
        );
        Reflect.set(window, "labelProbe", { runtime, status: () => status });
      }, backend());
      const inspect = () =>
        page.evaluate(() => {
          const p = Reflect.get(window, "labelProbe");
          const status = p.status() as { phase: string; message: string };
          if (status.phase === "error") {
            throw new Error(status.message);
          }
          return {
            status: status.phase,
            scene: p.runtime.sceneStats() as SceneDiagnostics | null,
            assets: p.runtime.assetStats() as Record<string, number>,
          };
        });
      const ready = async () => {
        await expect.poll(async () => (await inspect()).status, { timeout: 70_000 }).toBe("ready");
      };
      await ready();
      const baseline = (await inspect()).assets;
      const game = labelSceneGame();
      const deliver = (
        game: ReturnType<typeof sceneGame> | null,
        delivery: "snapshot" | "live" = "snapshot",
      ) =>
        page.evaluate(
          async ({ game, delivery }) => {
            const path = "/src/engine/scene.ts";
            const { toSceneSnapshot } = await import(/* @vite-ignore */ path);
            Reflect.get(window, "labelProbe").runtime.setScene(
              toSceneSnapshot(game, null),
              delivery,
            );
          },
          { game, delivery },
        );
      await deliver(game);
      await ready();
      let scene = (await inspect()).scene!;
      expect(scene.labels.ready).toBe(true);
      expect(scene.labels.warmFrames).toBe(2);
      expect(scene.labels.warmCamera).toBe(false);
      expect(scene.labels.atlasPages).toBe(1);
      if (layout === "desktop") {
        expect(scene.labels.glyphs).toBeGreaterThan(10);
      } else {
        expect(scene.labels.glyphs).toBe(2);
      }
      const center = scene.labels.entries.find((e) => e.id === "0,0,0")!;
      expect(center.icons).toBe(3);
      if (layout === "desktop") {
        expect(center.name).toBe("Arsia Mons");
        expect(center.credits).toEqual([3]);
        expect(center.fontParts).toBe(2);
        expect(scene.labels.entries.find((e) => e.id === "1,0,-1")!.name).toBe("New Ares");
      } else {
        expect(center.name).toBeNull();
        expect(center.credits).toEqual([]);
        expect(center.fontParts).toBe(0);
        expect(scene.labels.entries.every((e) => e.name === null)).toBe(true);
      }
      expect(scene.labels.entries.find((e) => e.id === "-1,0,1")!.icons).toBe(3);
      await page.evaluate(() => Reflect.get(window, "labelProbe").runtime.camera(-0.7));
      const writes = (await inspect()).assets.gridUniformWrites;
      await expect
        .poll(async () => (await inspect()).assets.gridUniformWrites)
        .toBeGreaterThan(writes + 2);
      await page.screenshot({
        path: path.resolve("../tmp/bevy-scaffold", `phase5-labels-${backend()}-${layout}.png`),
      });
      scene = (await inspect()).scene!;
      await writeFile(
        path.resolve("../tmp/bevy-scaffold", `phase5-labels-${backend()}-${layout}.json`),
        JSON.stringify(await inspect(), null, 2),
      );
      if (layout === "desktop") {
        const stable = scene.labels.entries.find((e) => e.id === "1,0,-1")!.root;
        const glyphs = scene.labels.glyphs;
        const target = game.board.tiles.find(
          (t) => t.coordinates.q === 0 && t.coordinates.r === 0,
        )!;
        target.displayName = "Zephyr Quartz 947";
        target.bonuses = [
          { type: "credit", amount: 97 },
          { type: "steel", amount: 2 },
        ];
        await deliver(game, "live");
        await expect
          .poll(
            async () => (await inspect()).scene!.labels.entries.find((e) => e.id === "0,0,0")?.name,
          )
          .toBe("Zephyr Quartz 947");
        await ready();
        scene = (await inspect()).scene!;
        expect(scene.labels.entries.find((e) => e.id === "0,0,0")!.name).toBe("Zephyr Quartz 947");
        expect(scene.labels.entries.find((e) => e.id === "0,0,0")!.credits).toEqual([97]);
        expect(scene.labels.entries.find((e) => e.id === "1,0,-1")!.root).toBe(stable);
        expect(scene.labels.glyphs).toBeGreaterThan(glyphs);
        expect(scene.labels.atlasPages).toBe(1);
        await expect.poll(async () => (await inspect()).scene!.labels.ready).toBe(true);
        const beforePublication = (await inspect()).assets.gridUniformWrites;
        await expect
          .poll(async () => (await inspect()).assets.gridUniformWrites)
          .toBeGreaterThan(beforePublication + 2);
        await page.screenshot({
          path: path.resolve("../tmp/bevy-scaffold", `phase5-labels-${backend()}-live.png`),
        });
        const sameRoot = scene.labels.entries.find((e) => e.id === "0,0,0")!.root;
        const beforeMetadata = (await inspect()).assets.gridUniformWrites;
        target.reservedBy = "player";
        await deliver(game, "live");
        await expect
          .poll(async () => (await inspect()).assets.gridUniformWrites)
          .toBeGreaterThan(beforeMetadata + 2);
        await ready();
        expect((await inspect()).scene!.labels.entries.find((e) => e.id === "0,0,0")!.root).toBe(
          sameRoot,
        );
        await page.emulateMedia({ reducedMotion: "no-preference" });
        target.occupiedBy = { type: "nuclear-zone-tile", visual: { seed: 123 } } as NonNullable<
          typeof target.occupiedBy
        >;
        await deliver(game, "live");
        await expect
          .poll(
            async () =>
              typeof (await inspect()).scene!.nuclearTimeline.events[0]?.start === "number",
            {
              timeout: 40_000,
            },
          )
          .toBe(true);
        await page.evaluate(() => Reflect.get(window, "labelProbe").runtime.setPaused(true));
        scene = (await inspect()).scene!;
        expect(scene.step - scene.nuclearTimeline.events[0].start!).toBeLessThan(60);
        expect(scene.labels.entries.find((e) => e.id === "0,0,0")!.name).toBe("Zephyr Quartz 947");
        await page.evaluate(() => Reflect.get(window, "labelProbe").runtime.setPaused(false));
        await expect
          .poll(async () => (await inspect()).scene!.labels.entries.some((e) => e.id === "0,0,0"), {
            timeout: 15_000,
          })
          .toBe(false);
        await page.screenshot({
          path: path.resolve("../tmp/bevy-scaffold", `phase5-labels-${backend()}-impact.png`),
        });
        // A reconnect is a new authoritative baseline, not another live impact.
        await deliver(game, "snapshot");
        await expect
          .poll(
            async () => (await inspect()).scene!.labels.entries.find((e) => e.id === "0,0,0")?.name,
            { timeout: 40_000 },
          )
          .toBe("Zephyr Quartz 947");
        await ready();
        expect((await inspect()).scene!.nuclearTimeline.events).toEqual([]);
        expect((await inspect()).scene!.labels.entries.find((e) => e.id === "0,0,0")!.name).toBe(
          "Zephyr Quartz 947",
        );
      }
      await deliver(null);
      await ready();
      const stores = [
        "models",
        "images",
        "meshes",
        "materials",
        "gpuImages",
        "gpuMeshes",
        "gpuMaterials",
      ];
      await expect
        .poll(
          async () => {
            const a = (await inspect()).assets;
            return stores.map((key) => a[key]);
          },
          { timeout: 15_000 },
        )
        .toEqual(stores.map((key) => baseline[key]));
      expect((await inspect()).scene!.labels.entries).toEqual([]);
      expect((await inspect()).scene!.labels.atlasPages).toBe(0);
      await page.evaluate(() => Reflect.get(window, "labelProbe").runtime.dispose());
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });
}

for (const [layout, profile] of [
  ["desktop", { viewport: { width: 1280, height: 720 } }],
  ["iphone", devices["iPhone 14 landscape"]],
  ["android", devices["Pixel 7 landscape"]],
] as const) {
  test(`renders connected ocean basins through freeze thaw and live placement on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.skip(!test.info().project.name.startsWith("engine"));
    test.setTimeout(240_000);
    const context = await referenceContext(browser, {
      ...profile,
      baseURL,
      reducedMotion: "no-preference",
    });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    try {
      await page.goto(
        `/tests/fixtures/engine.html?renderer=${backend()}${layout === "desktop" ? "" : "&compact"}`,
      );
      await page.evaluate(
        async (wasmPath) => {
          const module = await import(/* @vite-ignore */ wasmPath);
          const create = module.Engine.create;
          module.Engine.create = async (canvas: HTMLCanvasElement) => {
            const native = await create.call(module.Engine, canvas);
            const frame = native.frame.bind(native);
            native.frame = (width: number, height: number) => frame(width, height, 0);
            const acceptTerrain = native.accept_terrain.bind(native);
            let workerJobs = 0;
            let workerBytes = 0;
            native.accept_terrain = (bytes: Uint8Array) => {
              const accepted = acceptTerrain(bytes);
              if (accepted) {
                workerJobs++;
                workerBytes += bytes.byteLength;
              }
              return accepted;
            };
            Reflect.set(window, "oceanNative", {
              native,
              canvas,
              frame,
              workerResults: () => ({ jobs: workerJobs, bytes: workerBytes }),
            });
            return native;
          };
        },
        "/@fs" + path.resolve("../engine/pkg", backend(), "openmars_engine.js"),
      );
      await page.evaluate(async (renderer) => {
        const p = "/src/engine/runtime.ts";
        const { startEngine } = await import(/* @vite-ignore */ p);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        let status = { phase: "starting", message: "" };
        const runtime = startEngine(
          host,
          (next: typeof status) => {
            status = next;
          },
          renderer,
        );
        Reflect.set(window, "oceanProbe", { runtime, status: () => status });
      }, backend());
      const inspect = () =>
        page.evaluate(() => {
          const p = Reflect.get(window, "oceanProbe");
          const status = p.status();
          if (status.phase === "error") {
            throw Error(status.message);
          }
          return {
            status: status.phase as string,
            scene: p.runtime.sceneStats() as SceneDiagnostics | null,
            assets: p.runtime.assetStats() as Record<string, number>,
          };
        });
      await expect.poll(async () => (await inspect()).status, { timeout: 70_000 }).toBe("ready");
      const baseline = (await inspect()).assets;
      const publish = () =>
        page.evaluate(async () => {
          const { native, canvas, frame } = Reflect.get(window, "oceanNative");
          const before = JSON.parse(native.asset_stats());
          const deadline = Date.now() + 65_000;
          while (Date.now() < deadline) {
            const p = Reflect.get(window, "oceanProbe");
            if (p.status().phase === "error") {
              throw Error(p.status().message);
            }
            const ready = frame(canvas.width, canvas.height, 0);
            const after = JSON.parse(native.asset_stats());
            const dynamicSurface =
              after.terrainTransitioning === 1 &&
              after.terrainRevision === after.terrainRequestedRevision;
            if (
              (ready || dynamicSurface) &&
              after.gridUniformWrites > before.gridUniformWrites + 2
            ) {
              if (
                after.terrainStep !== before.terrainStep ||
                after.motionStep !== before.motionStep
              ) {
                throw Error("Frozen publication advanced a clock");
              }
              return {
                scene: JSON.parse(native.scene_stats()) as SceneDiagnostics,
                assets: after as Record<string, number>,
              };
            }
            await new Promise(requestAnimationFrame);
          }
          throw Error("Ocean publication deadline: " + native.asset_stats());
        });
      const deliver = (
        game: ReturnType<typeof sceneGame> | null,
        delivery: "snapshot" | "live" = "snapshot",
      ) =>
        page.evaluate(
          async ({ game, delivery }) => {
            const p = "/src/engine/scene.ts";
            const { toSceneSnapshot } = await import(/* @vite-ignore */ p);
            Reflect.get(window, "oceanProbe").runtime.setScene(
              toSceneSnapshot(game, null),
              delivery,
            );
          },
          { game, delivery },
        );
      const advance = async (steps: number) => {
        await page.evaluate(async (steps) => {
          const { native, canvas, frame } = Reflect.get(window, "oceanNative");
          const target = JSON.parse(native.asset_stats()).terrainStep + steps;
          for (let i = 0; i < steps * 20 + 240; i++) {
            const now = JSON.parse(native.asset_stats()).terrainStep;
            if (now === target) {
              return;
            }
            if (now > target) {
              throw Error("Ocean clock overshot");
            }
            frame(canvas.width, canvas.height, Math.min(6, target - now) / 60);
            await new Promise(requestAnimationFrame);
          }
          throw Error("Ocean clock deadline");
        }, steps);
        return publish();
      };
      const capture = async (stage: string) => {
        await publish();
        return page.screenshot({
          path: path.resolve(
            "../tmp/bevy-scaffold",
            `phase5-oceans-${backend()}-${layout}-${stage}.png`,
          ),
        });
      };
      const game = oceanSceneGame();
      await deliver(game);
      let view = await publish();
      expect(view.scene.tiles.filter((t) => t.occupant === "ocean-tile")).toHaveLength(8);
      expect(view.assets.terrainPatches).toBeGreaterThan(8);
      expect(view.assets.terrainTransitioning).toBe(0);
      // The dynamic surface is drawable before its optional ground cache finishes.
      await expect
        .poll(async () => (await inspect()).assets.bakeValid, { timeout: 20_000 })
        .toBe(1);
      view = await publish();
      expect(view.assets.bakeValid).toBe(1);
      const revision = view.scene.terrain.revision;
      await page.evaluate(() => Reflect.get(window, "oceanProbe").runtime.camera(-0.7));
      const cold = await capture("frozen");
      game.globalParameters.temperature = -2;
      game.globalParameters.oxygen = 6;
      await deliver(game);
      view = await publish();
      expect(view.scene.terrain.revision).toBe(revision);
      await capture("melting");
      game.globalParameters.temperature = 8;
      game.globalParameters.oxygen = 14;
      await deliver(game);
      view = await publish();
      expect(view.scene.terrain.revision).toBe(revision);
      const warm = await capture("water");
      const center = view.scene.tiles.find((t) => t.id === "0,0,0")!.point!;
      const sample = async (bytes: Buffer) => {
        const metadata = await sharp(bytes).metadata();
        const viewport = page.viewportSize()!;
        const scale = metadata.width! / viewport.width;
        const { data } = await sharp(bytes)
          .extract({
            left: Math.round(center[0] * scale) - 4,
            top: Math.round(center[1] * scale) - 4,
            width: 8,
            height: 8,
          })
          .removeAlpha()
          .raw()
          .toBuffer({ resolveWithObject: true });
        return data.reduce((sum, n) => sum + n, 0) / data.length;
      };
      expect(await sample(cold)).toBeGreaterThan((await sample(warm)) + 10);
      await advance(120);
      const waves = await capture("waves");
      const wavePixels = await sharp(waves).resize(640).removeAlpha().raw().toBuffer();
      const stillPixels = await sharp(warm).resize(640).removeAlpha().raw().toBuffer();
      expect(Buffer.compare(wavePixels, stillPixels)).not.toBe(0);
      const frozenMotion = await capture("paused");
      expect(
        Buffer.compare(
          await sharp(frozenMotion).raw().toBuffer(),
          await sharp(waves).raw().toBuffer(),
        ),
      ).toBe(0);
      const ninth = game.board.tiles.find((t) => t.coordinates.q === 2 && t.coordinates.r === -1)!;
      await expect
        .poll(async () => (await inspect()).assets.bakeValid, { timeout: 20_000 })
        .toBe(1);
      ninth.occupiedBy = { type: "ocean-tile", tags: [] };
      game.globalParameters.oceans = 9;
      const liveStarted = Date.now();
      const beforeWorker = await page.evaluate(
        () => Reflect.get(window, "oceanNative").workerResults() as { jobs: number; bytes: number },
      );
      await deliver(game, "live");
      await expect
        .poll(
          async () =>
            page.evaluate((previous) => {
              const { native } = Reflect.get(window, "oceanNative");
              const scene = JSON.parse(native.scene_stats());
              const assets = JSON.parse(native.asset_stats());
              if (
                scene.terrain.revision === previous &&
                assets.terrainRequestedRevision > previous &&
                assets.bakeValid !== 1
              ) {
                throw Error(
                  "Preparing an ocean replacement discarded the unchanged published ground cache",
                );
              }
              return scene.terrain.revision > previous ? "published" : native.preparation();
            }, revision),
          { timeout: 40_000 },
        )
        .toBe("published");
      view = await publish();
      expect(view.scene.terrain.revision).toBeGreaterThan(revision);
      expect(view.scene.tiles.filter((t) => t.occupant === "ocean-tile")).toHaveLength(9);
      const livePreparationMs = Date.now() - liveStarted;
      const afterWorker = await page.evaluate(
        () => Reflect.get(window, "oceanNative").workerResults() as { jobs: number; bytes: number },
      );
      const workerResults = {
        jobs: afterWorker.jobs - beforeWorker.jobs,
        bytes: afterWorker.bytes - beforeWorker.bytes,
      };
      expect(workerResults.jobs).toBe(1);
      expect(workerResults.bytes).toBeGreaterThan(0);
      const newRevision = view.scene.terrain.revision;
      await advance(18);
      await capture("placement");
      view = await advance(150);
      expect(view.scene.terrain.transitioning).toBe(false);
      expect(view.scene.terrain.revision).toBe(newRevision);
      await capture("settled");
      await writeFile(
        path.resolve("../tmp/bevy-scaffold", `phase5-oceans-${backend()}-${layout}.json`),
        JSON.stringify({ livePreparationMs, workerResults, ...view }, null, 2),
      );
      await deliver(null);
      const stores = [
        "models",
        "images",
        "meshes",
        "materials",
        "gpuImages",
        "gpuMeshes",
        "gpuMaterials",
      ];
      await expect
        .poll(
          async () => {
            const a = (await inspect()).assets;
            return stores.map((key) => a[key]);
          },
          { timeout: 20_000 },
        )
        .toEqual(stores.map((key) => baseline[key]));
      expect((await inspect()).scene!.labels.atlasPages).toBe(0);
      await page.evaluate(() => Reflect.get(window, "oceanProbe").runtime.dispose());
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });
}

// SCRATCH ONLY. Append this test to frontend/tests/engine.spec.ts, which already imports
// test/expect/path/mkdir/writeFile/citySceneGame and defines backend(). No new imports.
// Run --grep "profiles first and second live city preparation" for each engine project.
// E2E_CITY_LATENCY_LABEL=baseline|candidate keeps artifacts separate.
// This is a DTO-shaped graphics fixture delivered through the production bridge;
// it is not an authoritative Go action or optimistic gameplay test.
test("profiles first and second live city preparation", async ({ page }) => {
  test.setTimeout(120_000);
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await page.setViewportSize({ width: 1280, height: 720 });
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const renderer = backend();
  const label = (process.env.E2E_CITY_LATENCY_LABEL ?? "baseline").replace(/[^a-zA-Z0-9_-]/g, "-");
  const output = path.resolve("../tmp/bevy-scaffold");
  await mkdir(output, { recursive: true });
  const prefix = `city-latency-${renderer}-${label}`;
  const evidence: Record<string, unknown> = {
    renderer,
    label,
    note:
      "Wall times measure actual fixture delivery/preparation, not first visible pixels. " +
      "Publication age uses scene.step/60 minus structures.sample.born, the actual structure clock. " +
      "Screenshots are compositor captures; before/after age is recorded because capture itself takes time.",
  };
  try {
    await page.goto("/tests/fixtures/engine.html");
    evidence.empty = await page.evaluate(
      async ({ authored, renderer }) => {
        const rp = "/src/engine/runtime.ts",
          sp = "/src/engine/scene.ts";
        const [{ startEngine }, { toSceneSnapshot }] = await Promise.all([
          import(/* @vite-ignore */ rp),
          import(/* @vite-ignore */ sp),
        ]);
        const host = document.createElement("div");
        host.style.cssText = "position:fixed;inset:0";
        document.body.append(host);
        let resolve = () => {},
          reject = (_error: Error) => {};
        let failure: string | null = null;
        const nextReady = () =>
          new Promise<void>((yes, no) => {
            resolve = yes;
            reject = no;
          });
        const initialReady = nextReady();
        const runtime = startEngine(
          host,
          (status: { phase: string; message: string }) => {
            if (status.phase === "ready") resolve();
            if (status.phase === "error") {
              failure = status.message;
              reject(new Error(status.message));
            }
          },
          renderer,
        );
        const game = structuredClone(authored);
        for (const tile of game.board.tiles) {
          delete tile.occupiedBy;
          delete tile.ownerId;
        }
        const stats = () => runtime.sceneStats();
        const wait = async (condition: () => boolean) => {
          for (let frame = 0; frame < 4000; frame++) {
            // oxlint-disable-next-line no-await-in-loop -- Inspect successive real rendered frames.
            await new Promise<void>((yes) => requestAnimationFrame(() => yes()));
            if (failure) throw new Error(failure);
            if (condition()) return;
          }
          throw new Error("Live city measurement exhausted 4000 actual rAF callbacks");
        };
        const read = (coordinate: number[]) => {
          const scene = stats();
          const plot = scene?.structures.sample.find((sample: { coordinate: number[] }) =>
            sample.coordinate.every((value: number, index: number) => value === coordinate[index]),
          );
          const performance = runtime.performance();
          return {
            wall: window.performance.now(),
            step: scene?.step,
            born: plot?.born ?? null,
            age: plot?.born == null ? null : scene.step / 60 - plot.born,
            plot: plot ?? null,
            pending: scene?.structures.pending,
            ready: scene?.structures.ready,
            warmFrames: scene?.structures.warmFrames,
            plots: scene?.structures.plots,
            boardReady: scene?.boardReady,
            transitioning: scene?.terrain.transitioning,
            native: performance,
          };
        };
        Reflect.set(window, "liveCityLatencyProbe", {
          runtime,
          host,
          game,
          authored,
          toSceneSnapshot,
          stats,
          wait,
          read,
        });
        await initialReady;
        const boardReady = nextReady();
        runtime.setScene(toSceneSnapshot(game, null), "snapshot");
        await boardReady;
        runtime.setPerformanceActive(true);
        await wait(
          () =>
            stats()?.boardReady &&
            !stats()?.settling &&
            stats()?.structures.ready &&
            stats()?.structures.pending === 0 &&
            !stats()?.terrain.transitioning,
        );
        const before = runtime.performance().submittedFrames;
        await wait(() => runtime.performance().submittedFrames >= before + 2);
        return read([0, 0, 0]);
      },
      { authored: citySceneGame(), renderer },
    );
    const cityReports: unknown[] = [];
    evidence.cities = cityReports;
    for (const [ordinal, coordinate] of [
      [0, [0, 0, 0]],
      [1, [-1, 0, 1]],
    ] as const) {
      const preparation = await page.evaluate(async (target) => {
        const p = Reflect.get(window, "liveCityLatencyProbe");
        const coordinate = target.slice();
        const source = p.authored.board.tiles.find(
          (tile: { coordinates: { q: number; r: number; s: number } }) =>
            [tile.coordinates.q, tile.coordinates.r, tile.coordinates.s].every(
              (value, index) => value === coordinate[index],
            ),
        );
        const tile = p.game.board.tiles.find(
          (item: { coordinates: { q: number; r: number; s: number } }) =>
            [item.coordinates.q, item.coordinates.r, item.coordinates.s].every(
              (value, index) => value === coordinate[index],
            ),
        );
        if (!source?.occupiedBy || !tile)
          throw new Error("Authored city fixture coordinate missing");
        tile.occupiedBy = structuredClone(source.occupiedBy);
        tile.ownerId = source.ownerId;
        const snapshot = p.toSceneSnapshot(p.game, null);
        const delivery = p.read(coordinate);
        const started = performance.now();
        p.runtime.setScene(snapshot, "live");
        const synchronousDeliveryMs = performance.now() - started;
        let firstPlot: unknown = null;
        let firstPublication: unknown = null;
        let previous = started;
        let lastProgress = started;
        let submitted = delivery.native.submittedFrames;
        let maxRafGapMs = 0;
        let maxNativeSubmissionGapMs = 0;
        let rafCount = 0;
        const nativeSamples: unknown[] = [];
        await p.wait(() => {
          const sample = p.read(coordinate);
          rafCount++;
          maxRafGapMs = Math.max(maxRafGapMs, sample.wall - previous);
          previous = sample.wall;
          maxNativeSubmissionGapMs = Math.max(maxNativeSubmissionGapMs, sample.wall - lastProgress);
          if (sample.native.submittedFrames > submitted) {
            submitted = sample.native.submittedFrames;
            lastProgress = sample.wall;
          }
          if (rafCount === 1 || rafCount % 20 === 0) nativeSamples.push(sample);
          if (!firstPlot && sample.plot)
            firstPlot = { ...sample, sinceDeliveryMs: sample.wall - started };
          if (sample.plot && sample.pending === 0 && sample.ready) {
            firstPublication = { ...sample, sinceDeliveryMs: sample.wall - started };
            return true;
          }
          return false;
        });
        return {
          coordinate,
          delivery,
          synchronousDeliveryMs,
          firstPlot,
          firstPublication,
          maxRafGapMs,
          maxNativeSubmissionGapMs,
          rafCount,
          nativeSamples,
        };
      }, coordinate);
      const report = { ordinal: ordinal + 1, ...preparation, captures: [] as unknown[] };
      cityReports.push(report);
      const publication = preparation.firstPublication as {
        born: number | null;
        age: number | null;
      };
      expect(publication).not.toBeNull();
      expect(
        publication.born,
        "Live fixture city keeps a real non-settled birth clock",
      ).not.toBeNull();
      expect(publication.born).toBeGreaterThanOrEqual(0);
      for (const [stage, age] of [
        ["postpublication", 0],
        ["early-rise", 0.5],
        ["settled", 2.0],
      ] as const) {
        const before = await page.evaluate(
          async ({ coordinate, age }) => {
            const p = Reflect.get(window, "liveCityLatencyProbe");
            await p.wait(() => {
              const sample = p.read(coordinate);
              return (
                sample.age !== null && sample.age >= age && sample.pending === 0 && sample.ready
              );
            });
            return p.read(coordinate);
          },
          { coordinate, age },
        );
        await page.screenshot({
          path: path.join(output, `${prefix}-city-${ordinal + 1}-${stage}.png`),
        });
        const after = await page.evaluate(
          (coordinate) => Reflect.get(window, "liveCityLatencyProbe").read(coordinate),
          coordinate,
        );
        report.captures.push({
          stage,
          requestedAge: age,
          before,
          after,
          requestedAgeAlreadyPassedAtPublication:
            age > 0 && publication.age !== null && publication.age >= age,
        });
      }
    }
    const final = await page.evaluate(() => Reflect.get(window, "liveCityLatencyProbe").stats());
    evidence.final = final;
    expect(final.structures.plots).toBe(2);
    expect(final.structures.pending).toBe(0);
    expect(final.structures.ready).toBe(true);
    expect(errors).toEqual([]);
  } finally {
    evidence.errors = errors;
    await writeFile(path.join(output, `${prefix}.json`), JSON.stringify(evidence, null, 2) + "\n");
    await page
      .evaluate(() => {
        const p = Reflect.get(window, "liveCityLatencyProbe");
        if (p) {
          p.runtime.setPerformanceActive(false);
          p.runtime.dispose();
          p.host.remove();
          Reflect.deleteProperty(window, "liveCityLatencyProbe");
        }
      })
      .catch(() => {});
  }
});
