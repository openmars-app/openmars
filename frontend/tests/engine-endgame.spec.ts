import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import type { GameDto } from "../src/types/generated/api-types";
import type { SceneDiagnostics } from "../src/engine/diagnostics";
import type { EngineStatus } from "../src/engine/status";

test.skip(
  !process.env.E2E_BACKEND || !process.env.E2E_SESSION_SAVE,
  "Requires an owned Go server and a single-player development-mode exported active game",
);
test.beforeEach(() => test.skip(!["engine", "engine-webgl2"].includes(test.info().project.name)));
test.use({ actionTimeout: 15_000, trace: { mode: "retain-on-failure", screenshots: false } });

interface EndgameState {
  game: GameDto | null;
  connected: boolean;
  phase: string;
  status: EngineStatus;
  scene: SceneDiagnostics | null;
  productionOpen: boolean;
  reactUpdateDepth: string | undefined;
}

async function state(page: Page): Promise<EndgameState> {
  return page.evaluate(async () => {
    const gp = "/src/stores/gameStore.ts";
    const ep = "/src/stores/engineStore.ts";
    const pp = "/src/stores/appPhaseStore.ts";
    const up = "/src/stores/uiOverlayStore.ts";
    const [{ useGameStore }, { useEngineStore }, { useAppPhaseStore }, { useUIOverlayStore }] =
      await Promise.all([
        import(/* @vite-ignore */ gp),
        import(/* @vite-ignore */ ep),
        import(/* @vite-ignore */ pp),
        import(/* @vite-ignore */ up),
      ]);
    const game = useGameStore.getState();
    const engine = useEngineStore.getState();
    return {
      game: game.game as GameDto | null,
      connected: game.isConnected as boolean,
      phase: useAppPhaseStore.getState().phase.kind as string,
      status: engine.status as EngineStatus,
      scene: engine.inspectScene?.() as SceneDiagnostics | null,
      productionOpen: useUIOverlayStore.getState().showProductionPhaseModal as boolean,
      reactUpdateDepth: Reflect.get(window, "reactUpdateDepth") as string | undefined,
    };
  });
}

// Every mutation below is sent to Go through the actual production service.
// The admin command seeds a near-complete condition; it never writes React state.
async function seedLastRound(page: Page) {
  await page.evaluate(async () => {
    const sp = "/src/services/globalWebSocketManager.ts";
    const gp = "/src/stores/gameStore.ts";
    const [{ globalWebSocketManager: manager }, { useGameStore }] = await Promise.all([
      import(/* @vite-ignore */ sp),
      import(/* @vite-ignore */ gp),
    ]);
    const game = useGameStore.getState().game as GameDto;
    await manager.sendAdminCommand({
      commandType: "set-resources",
      payload: {
        playerId: game.currentPlayer.id,
        resources: { ...game.currentPlayer.resources, plants: 8 },
      },
    });
    await manager.sendAdminCommand({
      commandType: "set-global-params",
      payload: { globalParameters: { temperature: 8, oxygen: 14, oceans: 9, venus: 0 } },
    });
  });
}

async function action(page: Page, kind: "pass" | "greenery") {
  await page.evaluate(async (kind) => {
    const sp = "/src/services/globalWebSocketManager.ts";
    const { globalWebSocketManager: manager } = await import(/* @vite-ignore */ sp);
    if (kind === "pass") await manager.skipAction();
    else await manager.convertPlantsToGreenery();
  }, kind);
}

test("finishes a validated Go save through final production greenery and scoring", async ({
  browser,
  baseURL,
}) => {
  test.setTimeout(480_000);
  const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
  const viewport = { width: 1280, height: 720 };
  const context = await browser.newContext({ baseURL, viewport, reducedMotion: "reduce" });
  await context.addInitScript((renderer) => {
    const url = new URL(location.href);
    url.searchParams.set("renderer", renderer);
    history.replaceState(history.state, "", url);
    const report = console.error;
    console.error = (...values) => {
      if (String(values[0]).includes("Maximum update depth")) {
        Reflect.set(window, "reactUpdateDepth", String(values[0]));
      }
      report(...values);
    };
  }, backend);
  const page = await context.newPage();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const output = path.resolve("../tmp/bevy-scaffold");
  await mkdir(output, { recursive: true });
  const prefix = path.join(output, `phase6-endgame-${backend}-desktop`);
  const evidence: Record<string, unknown> = {
    setup:
      "Go validates an actual exported save; development admin seeds final global conditions and eight plants. Final production, placement and scoring use normal authoritative actions.",
  };
  try {
    await page.goto(`/load?renderer=${backend}`);
    await page
      .getByLabel("Select an exported Game file")
      .setInputFiles(process.env.E2E_SESSION_SAVE!);
    await page.getByLabel("Your name").fill(`Endgame ${backend}`);
    await page.getByRole("button", { name: "Load game", exact: true }).click();
    await page.getByRole("button", { name: "Resume game", exact: true }).click();
    // The captured real fixture has an unacknowledged Cards received showcase.
    await expect(page.getByRole("heading", { name: "Cards received", exact: true })).toBeVisible({
      timeout: 120_000,
    });
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      {
        timeout: 120_000,
      },
    );
    await expect(page.getByRole("status")).toHaveCount(0, { timeout: 120_000 });
    await page.getByRole("button", { name: "Close", exact: true }).click();
    await expect(async () => {
      const current = await state(page);
      expect(current.connected).toBe(true);
      expect(current.phase).toBe("playing");
      expect(current.status.phase).toBe("ready");
      expect(current.status.backend).toBe(backend);
      expect(current.scene?.boardReady).toBe(true);
      expect(current.game?.currentPhase).toBe("action");
    }).toPass({ timeout: 120_000, intervals: [500] });
    await expect(page.getByRole("status")).toHaveCount(0, { timeout: 120_000 });
    const initial = await state(page);
    expect(initial.game!.settings.developmentMode).toBe(true);
    expect(initial.game!.otherPlayers).toHaveLength(0);
    expect(initial.game!.currentTurn).toBe(initial.game!.currentPlayer.id);
    const gameId = initial.game!.id;
    const playerId = initial.game!.currentPlayer.id;
    const canvas = await page.locator("canvas").elementHandle();
    evidence.initial = initial;

    await seedLastRound(page);
    await expect(async () => {
      const current = (await state(page)).game!;
      expect(current.isLastRound).toBe(true);
      expect(current.currentPlayer.resources.plants).toBe(8);
      expect(current.globalParameters).toMatchObject({ temperature: 8, oxygen: 14, oceans: 9 });
    }).toPass({ timeout: 20_000, intervals: [200] });
    await action(page, "pass");
    await expect(async () => {
      const current = await state(page);
      expect(current.game?.currentPhase).toBe("production_and_card_draw");
      expect(current.game?.currentPlayer.productionPhase?.selectionComplete).toBe(false);
      expect(current.productionOpen).toBe(true);
    }).toPass({ timeout: 20_000, intervals: [200] });
    await expect(page.getByRole("heading", { name: "Production", exact: true })).toBeVisible();
    await page.screenshot({ path: `${prefix}-production.png` });
    // Real Next calls confirmProductionCards([]), including Go's payment quote.
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await expect(async () => {
      const current = await state(page);
      expect(current.game?.currentPhase).toBe("final_phase");
      expect(current.game?.currentTurn).toBe(playerId);
      expect(current.productionOpen).toBe(false);
      expect(current.status.phase).toBe("ready");
    }).toPass({ timeout: 120_000, intervals: [500] });
    await expect(page.getByRole("heading", { name: "Production", exact: true })).toHaveCount(0);

    await action(page, "greenery");
    await expect(async () => {
      const current = await state(page);
      expect(
        current.game?.currentPlayer.pendingTileSelection?.availableHexes.length,
      ).toBeGreaterThan(0);
      expect(current.status.phase).toBe("ready");
      expect(current.scene?.camera.framing).toBe(false);
    }).toPass({ timeout: 120_000, intervals: [500] });
    const pending = await state(page);
    const available = new Set(pending.game!.currentPlayer.pendingTileSelection!.availableHexes);
    const targets = pending.scene!.tiles.filter(
      (tile) =>
        available.has(tile.id) &&
        tile.point &&
        tile.point[0] > viewport.width * 0.2 &&
        tile.point[0] < viewport.width * 0.8 &&
        tile.point[1] > viewport.height * 0.2 &&
        tile.point[1] < viewport.height * 0.8,
    );
    targets.sort(
      (a, b) =>
        Math.hypot(a.point![0] - viewport.width / 2, a.point![1] - viewport.height / 2) -
        Math.hypot(b.point![0] - viewport.width / 2, b.point![1] - viewport.height / 2),
    );
    expect(targets.length).toBeGreaterThan(0);
    const tile = targets[0];
    expect(
      await page.evaluate(([x, y]) => document.elementFromPoint(x, y)?.tagName, tile.point!),
    ).toBe("CANVAS");
    await page.mouse.click(...tile.point!);
    await expect(async () => {
      const current = await state(page);
      expect(current.game?.currentPlayer.pendingTileSelection).toBeFalsy();
      expect(current.scene?.tiles.find((item) => item.id === tile.id)?.occupant).toBe(
        "greenery-tile",
      );
      expect(current.status.phase).toBe("ready");
    }).toPass({ timeout: 120_000, intervals: [500] });
    const placed = await state(page);
    evidence.placed = placed;
    await page.screenshot({ path: `${prefix}-greenery.png` });
    // Some final-greenery flows score automatically; otherwise pass normally.
    if (placed.game!.currentPhase === "final_phase") await action(page, "pass");

    const completed = async () => {
      const current = await state(page);
      expect(current.game?.id).toBe(gameId);
      expect(current.game?.currentPlayer.id).toBe(playerId);
      expect(current.connected).toBe(true);
      expect(current.game?.status).toBe("completed");
      expect(current.game?.currentPhase).toBe("complete");
      expect(current.phase).toBe("completed");
      expect(current.game?.finalScores).toHaveLength(1);
      expect(current.game?.finalScores?.[0].playerId).toBe(playerId);
      expect(current.game?.finalScores?.[0].isWinner).toBe(true);
      expect(current.game?.finalScores?.[0].placement).toBe(1);
      expect(current.productionOpen).toBe(false);
      expect(current.status.phase).toBe("ready");
      expect(current.status.backend).toBe(backend);
      expect(current.scene?.boardReady).toBe(true);
      expect(current.scene?.tiles.find((item) => item.id === tile.id)?.occupant).toBe(
        "greenery-tile",
      );
      expect(current.reactUpdateDepth).toBeUndefined();
    };
    await expect(completed).toPass({ timeout: 120_000, intervals: [500] });
    const final = await state(page);
    evidence.completed = final;
    const scores = final.game!.finalScores!;
    const totalVP = scores[0].vpBreakdown.totalVP;
    expect(totalVP).toBeGreaterThan(0);
    const scoreButton = page.getByRole("button", { name: "SCORE", exact: true });
    await expect(scoreButton).toBeVisible({ timeout: 30_000 });
    const skip = page.getByRole("button", { name: "Skip", exact: true });
    if (await skip.isVisible()) await skip.click();
    await expect(page.getByText(`${totalVP} VP`, { exact: true }).first()).toBeVisible({
      timeout: 60_000,
    });
    await page.screenshot({ path: `${prefix}-score-world.png` });
    await page.getByRole("button", { name: "GRAPHS", exact: true }).click();
    await expect(page.locator(".recharts-wrapper").first()).toBeVisible({ timeout: 30_000 });
    await expect
      .poll(() =>
        page
          .locator(".recharts-wrapper")
          .first()
          .evaluate((element) => getComputedStyle(element.closest(".fixed")!).opacity),
      )
      .toBe("1");
    await page.screenshot({ path: `${prefix}-graphs.png` });
    await scoreButton.click();
    expect(
      await page.evaluate((element) => element === document.querySelector("canvas"), canvas),
    ).toBe(true);

    // Completed states cannot be reimported; reload the actual persisted Go game.
    await page.reload();
    await expect(completed).toPass({ timeout: 120_000, intervals: [500] });
    await expect(page.getByRole("status")).toHaveCount(0, { timeout: 120_000 });
    expect((await state(page)).game!.finalScores).toEqual(scores);
    await expect(page.getByRole("button", { name: "SCORE", exact: true })).toBeVisible({
      timeout: 30_000,
    });
    await page.screenshot({ path: `${prefix}-reloaded.png` });
    expect(errors).toEqual([]);
  } finally {
    evidence.final = await state(page).catch(() => null);
    await page.screenshot({ path: `${prefix}-final.png` }).catch(() => {});
    await writeFile(`${prefix}.json`, JSON.stringify(evidence, null, 2) + "\n");
    await context.close();
  }
});
