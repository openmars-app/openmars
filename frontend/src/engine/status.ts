import type { EngineBackend, RendererCapabilities } from "./backend";

export type EngineStatus = { backend?: EngineBackend } & (
  | { phase: "starting"; message: string }
  | { phase: "loading"; message: string; loaded: number; total: number | null }
  | { phase: "preparing"; message: string }
  | {
      phase: "ready";
      message: string;
      capabilities: RendererCapabilities;
      scene: { gameId: string; mapId: string } | null;
    }
  | { phase: "error"; message: string }
);

/** Menu readiness must never unlock a game that has not finished preparing. */
export function isGameSceneReady(
  status: EngineStatus,
  gameId: string | null | undefined,
  mapId: string | null | undefined,
): boolean {
  return (
    status.phase === "ready" &&
    !!gameId &&
    !!mapId &&
    status.scene?.gameId === gameId &&
    status.scene.mapId === mapId
  );
}

export const INITIAL_ENGINE_STATUS: EngineStatus = {
  phase: "starting",
  message: "Starting renderer",
};
