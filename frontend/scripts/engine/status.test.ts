import { expect, test } from "bun:test";
import { isGameSceneReady, type EngineStatus } from "../../src/engine/status";

test("readiness belongs to a specific rendered game and map", () => {
  const ready: EngineStatus = {
    phase: "ready",
    message: "Ready",
    scene: null,
    capabilities: { computeShaders: true, storageBuffers: true, maxTextureDimension2D: 8192 },
  };
  expect(isGameSceneReady(ready, "game", "tharsis")).toBe(false);
  const world = { ...ready, scene: { gameId: "game", mapId: "tharsis" } };
  expect(isGameSceneReady(world, "game", "tharsis")).toBe(true);
  expect(isGameSceneReady(world, "other", "tharsis")).toBe(false);
  expect(isGameSceneReady(world, "game", "hellas")).toBe(false);
  expect(isGameSceneReady(world, undefined, undefined)).toBe(false);
  expect(isGameSceneReady({ phase: "preparing", message: "Preparing" }, "game", "tharsis")).toBe(
    false,
  );
});
