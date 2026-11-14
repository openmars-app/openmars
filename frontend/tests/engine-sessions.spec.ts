import { devices, expect, test, type Page, type WebSocketRoute } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import type { GameDto } from "../src/types/generated/api-types";
import type { SceneDiagnostics } from "../src/engine/diagnostics";
import type { EnginePerformance } from "../src/engine/performance";

test.skip(!process.env.E2E_BACKEND, "Requires an explicitly selected Go verification server");
test.beforeEach(() => test.skip(!test.info().project.name.startsWith("engine")));
test.use({ actionTimeout: 10_000, trace: { mode: "retain-on-failure", screenshots: false } });

function captureReactCycle() {
  const report = console.error;
  console.error = (...values) => {
    if (
      String(values[0]).includes("Maximum update depth") &&
      !Reflect.has(window, "reactUpdateDepth")
    ) {
      Reflect.set(window, "reactUpdateDepth", new Error("React update cycle").stack);
    }
    report(...values);
  };
}

// Read production stores; every game mutation below goes through the existing UI/services.
async function state(page: Page) {
  return page.evaluate(async () => {
    const gp = "/src/stores/gameStore.ts";
    const ep = "/src/stores/engineStore.ts";
    const pp = "/src/stores/appPhaseStore.ts";
    const wp = "/src/services/globalWebSocketManager.ts";
    const sp = "/src/utils/sessionStorage.ts";
    const [
      { useGameStore },
      { useEngineStore },
      { useAppPhaseStore, gameIdOf },
      { globalWebSocketManager },
      { getGameSession },
    ] = await Promise.all([
      import(/* @vite-ignore */ gp),
      import(/* @vite-ignore */ ep),
      import(/* @vite-ignore */ pp),
      import(/* @vite-ignore */ wp),
      import(/* @vite-ignore */ sp),
    ]);
    const game = useGameStore.getState();
    const engine = useEngineStore.getState();
    return {
      game: game.game as GameDto | null,
      connected: game.isConnected as boolean,
      transportConnected: globalWebSocketManager.connected as boolean,
      transportGameId: globalWebSocketManager.gameId as string | null,
      hasSession: getGameSession() !== null,
      spectator: game.isSpectator as boolean,
      phase: useAppPhaseStore.getState().phase.kind as string,
      phaseGameId: gameIdOf(useAppPhaseStore.getState().phase) as string | null,
      status: engine.status as { phase: string; backend?: string },
      scene: engine.inspectScene?.() as SceneDiagnostics | null,
      reactUpdateDepth: Reflect.get(window, "reactUpdateDepth") as string | undefined,
    };
  });
}

async function sessionPerformance(page: Page, active?: boolean) {
  return page.evaluate(async (enabled) => {
    const ep = "/src/stores/engineStore.ts";
    const { useEngineStore } = await import(/* @vite-ignore */ ep);
    const source = useEngineStore.getState().performance;
    if (enabled !== undefined) source?.setActive(enabled);
    return source?.sample() as EnginePerformance | null;
  }, active);
}

function quietLandingOwnership(scene: SceneDiagnostics) {
  // These are live ownership/queued-retirement sizes, not monotonic activity counters.
  return {
    tiles: scene.tiles.length,
    celestialGrids: scene.celestialGrids.bodies.length,
    gameBodies: scene.space.bodies.length,
    orbitLines: scene.space.orbitLines,
    structures: scene.structures.plots,
    structureChunks: scene.structures.chunks,
    structurePending: scene.structures.pending,
    structureRetiringMeshes: scene.structures.retiringMeshes,
    structureUniforms: scene.structures.uniforms,
    structureMaterials: scene.structures.materials,
    structureGroups: scene.structures.groups,
    structureImages: scene.structures.groupImages,
    structureMeshIds: scene.structures.meshIds.length,
    structureBufferIds: scene.structures.bufferIds.length,
    trees: scene.worldTrees.trees,
    treeUniforms: scene.worldTrees.uniforms,
    worksites: scene.worksites.sites,
    worksiteUniforms: scene.worksites.uniforms,
    worksiteMeshBytes: scene.worksites.meshBytes,
    specialSurfaces: scene.specialSurfaces.sites,
    specialUniforms: scene.specialSurfaces.uniforms,
    specialMeshBytes: scene.specialSurfaces.meshBytes,
    craters: scene.craters.sites,
    craterPending: scene.craters.pending,
    craterUniforms: scene.craters.uniforms,
    craterMeshBytes: scene.craters.meshBytes,
    blastSites: scene.blast.sites,
    blastRetirementQueue: scene.blast.retired,
    blastBytes: scene.blast.bytes,
    atmosphereBodies: scene.atmosphere.bodies,
    atmosphereUniforms: scene.atmosphere.uniforms,
    atmosphereRetirementQueue: scene.atmosphere.retired,
    nuclearDebris: scene.nuclearDebris.length,
    vegetation: scene.vegetationPoses.length,
    labels: scene.labels.entries.length,
    offworldShips: scene.offworld.ships,
    offworldEffects: scene.offworld.effects.entries.length,
  };
}

function landingResourceCounts(performance: EnginePerformance, scene: SceneDiagnostics) {
  return {
    entities: performance.entities,
    meshes: performance.residentMeshes,
    images: performance.residentImages,
    materials: performance.residentMaterials,
    ownership: quietLandingOwnership(scene),
    // Static menu-owned resources should remain bounded too.
    menuMaterials: scene.menu.materials,
    menuParts: scene.menu.moon?.parts ?? 0,
    labelMaterials: scene.labels.materials,
    labelAtlasPages: scene.labels.atlasPages,
    offworldMaterials: scene.offworld.effects.materials,
  };
}

async function quietLandingBaseline(page: Page, backend: string) {
  type Counts = ReturnType<typeof landingResourceCounts>;
  let candidate: { counts: Counts; frame: number } | undefined;
  let result: { counts: Counts; submittedFrames: number } | undefined;
  await expect(async () => {
    const current = await state(page);
    const performance = await sessionPerformance(page);
    expect(current.phase).toBe("menu");
    expect(current.phaseGameId).toBeNull();
    expect(current.transportConnected).toBe(false);
    expect(current.transportGameId).toBeNull();
    expect(current.hasSession).toBe(false);
    expect(current.status.phase).toBe("ready");
    expect(current.status.backend).toBe(backend);
    expect(current.reactUpdateDepth).toBeUndefined();
    expect(current.scene?.menu.ready).toBe(true);
    const scene = current.scene!;
    expect(scene.menu.root).not.toBeNull();
    expect(scene.menu.warmFrames).toBeGreaterThanOrEqual(2);
    expect(scene.terrain.transitioning).toBe(false);
    expect(scene.craters.building).toBe(false);
    expect(scene.worksites.building).toBe(false);
    expect(scene.specialSurfaces.building).toBe(false);
    expect(performance).toBeTruthy();
    expect(performance!.backend).toBe(backend);
    expect(performance!.state).toBe("rendering");
    const counts = landingResourceCounts(performance!, scene);
    expect(counts.ownership).toEqual(
      Object.fromEntries(
        Object.keys(counts.ownership).map((key) => [
          key,
          ["atmosphereBodies", "atmosphereUniforms"].includes(key) ? 1 : 0,
        ]),
      ),
    );
    // Start/restart a real submitted-frame fence whenever the resource footprint changes.
    if (!candidate || JSON.stringify(counts) !== JSON.stringify(candidate.counts)) {
      candidate = { counts, frame: performance!.submittedFrames };
    }
    expect(performance!.submittedFrames - candidate.frame).toBeGreaterThanOrEqual(2);
    result = { counts, submittedFrames: performance!.submittedFrames };
  }).toPass({ timeout: 120_000, intervals: [200] });
  return result!;
}

async function ready(page: Page, backend: string, id?: string) {
  await expect(async () => {
    const current = await state(page);
    expect(current.status.phase).toBe("ready");
    expect(current.status.backend).toBe(backend);
    expect(current.connected).toBe(true);
    expect(current.phase).toBe("playing");
    expect(current.phaseGameId).toBe(current.game?.id);
    expect(current.scene?.boardReady).toBe(true);
    expect(current.scene?.camera.framing).toBe(false);
    if (id) expect(current.game?.id).toBe(id);
  }).toPass({ timeout: 120_000, intervals: [500] });
  await expect(page.getByRole("status").filter({ hasText: "Connecting to game" })).toHaveCount(0);
  await expect(page.locator("canvas")).toHaveCount(1);
  await expect(page.getByRole("group", { name: "Board camera" })).toHaveCount(0);
}

async function advanceOpening(page: Page) {
  const step = page.getByRole("list", { name: "Steps" }).locator("[aria-current='step']");
  const game = (await state(page)).game;
  const cards = game?.currentPlayer.selectCorporationPhase?.availableCorporations ?? [];
  const cardsVisible = await page.locator(".corporation-card").first().isVisible();
  if (
    game?.currentPhase === "starting_selection" &&
    cards.length > 0 &&
    cardsVisible &&
    (!(await step.isVisible()) || (await step.textContent())?.includes("Corp"))
  ) {
    const selected = page.locator(".corporation-card[data-module-state='armed']");
    if ((await selected.count()) === 0) {
      const card = cards.find(
        (candidate) =>
          !candidate.behaviors?.some((behavior) =>
            behavior.triggers?.some((trigger) => trigger.type === "auto-corporation-first-action"),
          ),
      );
      expect(card, "Offered corporation without a forced first action").toBeDefined();
      await page.locator(`.corporation-card[data-card-name=${JSON.stringify(card!.name)}]`).click();
    }
  }
  for (const name of ["Close", "Start game", "Next", "Confirm Selection", "Confirm Skip"]) {
    const button = page.getByRole("button", { name, exact: true });
    if ((await button.count()) === 1 && (await button.isVisible()) && (await button.isEnabled())) {
      await button.click();
      return;
    }
  }
}

for (const [layout, viewport] of [
  ["desktop", { width: 1280, height: 720 }],
  ["iphone", { width: 844, height: 390 }],
  ["android", { width: 915, height: 412 }],
] as const) {
  test(`runs authoritative game save reconnect placement and replacement on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.setTimeout(600_000);
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
    await context.addInitScript(captureReactCycle);
    await context.addInitScript((renderer) => {
      const url = new URL(location.href);
      url.searchParams.set("renderer", renderer);
      history.replaceState(history.state, "", url);
    }, backend);
    let serverSocket: WebSocketRoute | undefined;
    let clientSocket: WebSocketRoute | undefined;
    await context.routeWebSocket("**/ws", (route) => {
      clientSocket = route;
      serverSocket = route.connectToServer();
    });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const output = path.resolve("../tmp/bevy-scaffold");
    await mkdir(output, { recursive: true });
    const evidence: Record<string, unknown> = {};
    const playerName = `Engine ${backend} ${layout} ${Date.now().toString(36)}`;
    try {
      await page.goto(`/create?renderer=${backend}${layout === "desktop" ? "" : "&compact"}`);
      await page.getByLabel("Your name").fill(playerName);
      await page.getByRole("button", { name: "Options", exact: true }).click();
      await page.getByRole("checkbox", { name: /^Preludes/ }).uncheck();
      await page.getByRole("button", { name: "Game options", exact: true }).click();
      await page.getByLabel("Players", { exact: true }).fill("1");
      await page.getByRole("button", { name: "Create lobby", exact: true }).click();
      await expect(async () => {
        await advanceOpening(page);
        expect((await state(page)).game?.currentPhase).toBe("action");
      }).toPass({ timeout: 180_000, intervals: [500] });
      await ready(page, backend);
      const created = await state(page);
      const gameId = created.game!.id;
      evidence.created = created;
      console.info("Real session: new game ready", { backend, layout, gameId });
      await page.screenshot({
        path: path.join(output, `phase6-session-${backend}-${layout}-new.png`),
      });

      // This service quotes and pays using Go's authoritative payment contract.
      const projects = created.game!.currentPlayer.standardProjects ?? [];
      const project = ["city", "greenery", "aquifer"]
        .map((type) => projects.find((item) => item.projectType === type && item.available))
        .find(Boolean);
      expect(project, "Go offers an affordable tile project").toBeDefined();
      await page.evaluate(async (projectId) => {
        const mp = "/src/services/globalWebSocketManager.ts";
        const { globalWebSocketManager } = await import(/* @vite-ignore */ mp);
        await globalWebSocketManager.standardProject(projectId);
      }, project!.projectType);
      await expect(async () => {
        expect(
          (await state(page)).game?.currentPlayer.pendingTileSelection?.availableHexes.length,
        ).toBeGreaterThan(0);
      }).toPass({ timeout: 15_000, intervals: [200] });
      await ready(page, backend, gameId);
      const placement = await state(page);
      const expectedOccupant = `${placement.game!.currentPlayer.pendingTileSelection!.tileType}-tile`;
      const available = new Set(placement.game!.currentPlayer.pendingTileSelection!.availableHexes);
      const tiles = placement.scene!.tiles.filter(
        (tile) =>
          available.has(tile.id) &&
          tile.point &&
          tile.point[0] > viewport.width * 0.2 &&
          tile.point[0] < viewport.width * 0.8 &&
          tile.point[1] > viewport.height * 0.2 &&
          tile.point[1] < viewport.height * 0.8,
      );
      tiles.sort(
        (a, b) =>
          Math.hypot(a.point![0] - viewport.width / 2, a.point![1] - viewport.height / 2) -
          Math.hypot(b.point![0] - viewport.width / 2, b.point![1] - viewport.height / 2),
      );
      expect(tiles.length).toBeGreaterThan(0);
      const tile = tiles[0];
      const hit = await page.evaluate(
        ([x, y]) =>
          document
            .elementsFromPoint(x, y)
            .slice(0, 8)
            .map((element) => ({
              tag: element.tagName,
              className: element.getAttribute("class"),
              pointerEvents: getComputedStyle(element).pointerEvents,
            })),
        tile.point!,
      );
      console.info("Real session: placement hit stack", hit);
      expect(hit[0]?.tag, "Uncovered board point reaches the renderer canvas").toBe("CANVAS");
      await page.mouse.click(...tile.point!);
      if (layout !== "desktop") await page.mouse.click(...tile.point!);
      await expect(async () => {
        const current = await state(page);
        expect(current.game?.currentPlayer.pendingTileSelection).toBeFalsy();
        expect(current.scene?.tiles.find((item) => item.id === tile.id)?.occupant).toBe(
          expectedOccupant,
        );
      }).toPass({ timeout: 90_000, intervals: [500] });
      await ready(page, backend, gameId);
      evidence.placed = await state(page);
      console.info("Real session: tile placed", { tile: tile.id, project: project!.projectType });
      await page.screenshot({
        path: path.join(output, `phase6-session-${backend}-${layout}-placed.png`),
      });

      await context.setOffline(true);
      expect(serverSocket).toBeDefined();
      expect(clientSocket).toBeDefined();
      const interruption = { code: 1012, reason: "Verification reconnect" };
      await serverSocket!.close(interruption);
      await clientSocket!.close(interruption);
      await expect.poll(async () => (await state(page)).connected).toBe(false);
      await context.setOffline(false);
      await ready(page, backend, gameId);
      await page.reload();
      await ready(page, backend, gameId);
      expect((await state(page)).scene?.tiles.find((item) => item.id === tile.id)?.occupant).toBe(
        expectedOccupant,
      );
      evidence.reconnected = await state(page);
      console.info("Real session: offline and reload recovered");

      await page.getByRole("button", { name: "Menu", exact: true }).click();
      const downloaded = page.waitForEvent("download");
      await page.getByText("Save game", { exact: true }).click();
      const save = await downloaded;
      const savePath = path.join(output, `phase6-session-${backend}-${layout}-save.json`);
      await save.saveAs(savePath);
      await page.goto(`/load?renderer=${backend}${layout === "desktop" ? "" : "&compact"}`);
      await page.getByLabel("Select an exported Game file").setInputFiles(savePath);
      const resumedName = `${playerName} resumed`;
      await page.getByLabel("Your name").fill(resumedName);
      await page.getByRole("button", { name: "Load game", exact: true }).click();
      await expect(page.getByRole("button", { name: "Resume game", exact: true })).toBeEnabled();
      await page.getByRole("button", { name: "Resume game", exact: true }).click();
      await ready(page, backend);
      const resumed = await state(page);
      expect(resumed.game!.id).not.toBe(gameId);
      expect(resumed.scene?.tiles.find((item) => item.id === tile.id)?.occupant).toBe(
        expectedOccupant,
      );
      evidence.loaded = resumed;
      console.info("Real session: saved game resumed", { gameId: resumed.game!.id });
      await page.screenshot({
        path: path.join(output, `phase6-session-${backend}-${layout}-loaded.png`),
      });
      expect(errors).toEqual([]);
      expect((await state(page)).reactUpdateDepth).toBeUndefined();

      // Close the first GPU context before opening the independent spectator.
      await context.close();
      const observer = await browser.newContext({
        ...profile,
        baseURL,
        viewport,
        reducedMotion: "reduce",
      });
      await observer.addInitScript(captureReactCycle);
      await observer.addInitScript((renderer) => {
        const url = new URL(location.href);
        url.searchParams.set("renderer", renderer);
        history.replaceState(history.state, "", url);
      }, backend);
      const observing = await observer.newPage();
      const observerErrors: string[] = [];
      observing.on("pageerror", (error) => observerErrors.push(error.message));
      try {
        await observing.goto(`/game/${gameId}?renderer=${backend}`);
        await observing.getByPlaceholder("Enter your name").fill(`Observer ${layout}`);
        await observing.getByPlaceholder("Enter your name").press("Enter");
        await ready(observing, backend, gameId);
        const watched = await state(observing);
        expect(watched.spectator).toBe(true);
        expect(watched.scene?.tiles.find((item) => item.id === tile.id)?.occupant).toBe(
          expectedOccupant,
        );
        evidence.spectator = watched;
        const firstEntity = watched.scene!.tiles.find((item) => item.id === tile.id)!.entity;
        const canvas = await observing.locator("canvas").elementHandle();
        await observing.screenshot({
          path: path.join(output, `phase6-session-${backend}-${layout}-spectator.png`),
        });

        const baselines: Awaited<ReturnType<typeof quietLandingBaseline>>[] = [];
        const cycles: unknown[] = [];
        const destinations = [
          { id: resumed.game!.id, host: resumedName },
          { id: gameId, host: playerName },
          { id: resumed.game!.id, host: resumedName },
        ];
        let previousEntity = firstEntity;
        await sessionPerformance(observing, true);
        try {
          for (const [index, destination] of destinations.entries()) {
            await observing.getByRole("button", { name: "Menu", exact: true }).click();
            await observing.getByText("Leave game", { exact: true }).click();
            await observing.getByRole("button", { name: "Leave", exact: true }).click();
            const browse = observing.getByRole("link", { name: "Browse", exact: true });
            await expect(browse).toBeVisible();
            await expect(observing.locator("canvas")).toHaveCount(1);
            expect(
              await observing.evaluate(
                (element) => element === document.querySelector("canvas"),
                canvas,
              ),
            ).toBe(true);
            baselines.push(await quietLandingBaseline(observing, backend));

            await browse.click();
            const row = observing
              .locator("div")
              .filter({ has: observing.getByText(destination.host, { exact: true }) })
              .filter({ has: observing.getByRole("button", { name: "Spectate", exact: true }) })
              .last();
            await row.getByRole("button", { name: "Spectate", exact: true }).click();
            const name = observing.getByPlaceholder("Enter your name");
            await name.fill(`Observer ${layout}`);
            await name.press("Enter");
            await ready(observing, backend, destination.id);
            const switched = await state(observing);
            expect(switched.phaseGameId).toBe(destination.id);
            expect(switched.spectator).toBe(true);
            const liveTile = switched.scene!.tiles.find((item) => item.id === tile.id);
            expect(liveTile?.occupant).toBe(expectedOccupant);
            expect(liveTile?.entity).toBeDefined();
            expect(liveTile!.entity).not.toBe(previousEntity);
            previousEntity = liveTile!.entity;
            expect(
              await observing.evaluate(
                (element) => element === document.querySelector("canvas"),
                canvas,
              ),
            ).toBe(true);
            expect(switched.reactUpdateDepth).toBeUndefined();
            expect(observerErrors).toEqual([]);
            cycles.push({ cycle: index + 1, destination: destination.id, state: switched });
            evidence.switched = switched;
            console.info("Real session: repeated spectator switch on one canvas", {
              cycle: index + 1,
              gameId: destination.id,
            });
          }
          // Cycle 1 warms teardown/menu resources; only the final two warmed quiet
          // landing projections must match. Frame IDs and counters intentionally differ.
          expect(baselines).toHaveLength(3);
          expect(baselines[2].counts).toEqual(baselines[1].counts);
          await observing.screenshot({
            path: path.join(output, `phase6-session-${backend}-${layout}-switched.png`),
          });
        } finally {
          evidence.spectatorCycles = cycles;
          evidence.landingBaselines = baselines;
          await sessionPerformance(observing, false);
        }

        if (layout !== "desktop") {
          await observing.setViewportSize({ width: viewport.height, height: viewport.width });
          await expect(observing.getByText("Open Mars is played in landscape.")).toBeVisible();
          await observing.screenshot({
            path: path.join(output, `phase6-session-${backend}-${layout}-portrait.png`),
          });
        } else {
          await observing.setViewportSize({ width: 1024, height: 650 });
          await ready(observing, backend, resumed.game!.id);
          await observing.screenshot({
            path: path.join(output, `phase6-session-${backend}-${layout}-resized.png`),
          });
        }
        await observing.setViewportSize(viewport);
        await ready(observing, backend, resumed.game!.id);
        expect((await state(observing)).reactUpdateDepth).toBeUndefined();
        expect(observerErrors).toEqual([]);
      } finally {
        if (!observing.isClosed()) {
          evidence.observerFinal = await state(observing).catch(() => null);
          await observing
            .screenshot({
              path: path.join(output, `phase6-session-${backend}-${layout}-observer-final.png`),
            })
            .catch(() => {});
        }
        await observer.close();
      }
    } finally {
      if (!page.isClosed()) {
        evidence.final = await state(page).catch(() => null);
        await page
          .screenshot({ path: path.join(output, `phase6-session-${backend}-${layout}-final.png`) })
          .catch(() => {});
      }
      await writeFile(
        path.join(output, `phase6-session-${backend}-${layout}.json`),
        JSON.stringify(evidence, null, 2) + "\n",
      );
      await context.close();
    }
  });
}
