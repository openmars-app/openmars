import { useRef } from "react";
import { createRoot } from "react-dom/client";
import "../../src/index.css";
import EngineBackground from "../../src/engine/EngineBackground";
import LoadingOverlay from "../../src/components/game/view/LoadingOverlay";
import TopMenuBar from "../../src/components/layout/panels/TopMenuBar";
import { createPreviewGame } from "../../src/components/ui/debug/uiDebug/fixtures";
import { Z_INDEX } from "../../src/constants/zIndex";
import { useGameTransitions } from "../../src/hooks/useGameTransitions";
import { useGameStore } from "../../src/stores/gameStore";
import { useAppPhaseStore, isInitializing } from "../../src/stores/appPhaseStore";
import { useEngineStore } from "../../src/stores/engineStore";
import { isGameSceneReady } from "../../src/engine/status";
import { sceneGame } from "./scene";

const noSound = async () => {};

export function mountRestoration() {
  document.documentElement.style.setProperty("--surface-layer", String(Z_INDEX.CONTROL_DECORATION));
  const game = createPreviewGame();
  game.id = "restored-game";
  game.board = sceneGame().board;
  game.board.tiles[0].occupiedBy = { type: "city-tile", tags: [] };
  useAppPhaseStore.getState().setPhase({ kind: "checking", gameId: game.id });
  useGameStore.setState({ game: null, currentPlayer: null, isConnected: false });
  const host = document.createElement("div");
  document.body.append(host);
  const root = createRoot(host);
  function Restoration() {
    const queue = useRef(0);
    useGameTransitions(noSound, queue);
    const current = useGameStore((s) => s.game);
    const phase = useAppPhaseStore((s) => s.phase);
    const status = useEngineStore((s) => s.status);
    const loaded =
      !isInitializing(phase) && isGameSceneReady(status, current?.id, current?.settings.mapId);
    return (
      <>
        <EngineBackground />
        {current && <TopMenuBar gameState={current} currentPlayer={current.currentPlayer} />}
        <LoadingOverlay
          isLoaded={loaded}
          showDelayMs={0}
          minDurationMs={0}
          fadeDurationMs={1500}
          message={status.message}
          showProgress
        />
      </>
    );
  }
  root.render(<Restoration />);
  const frames: Array<{ phase: string; readyGame: string | null; opacity: number | null }> = [];
  let frame = 0;
  const sample = () => {
    // Start observing once React has committed the fixture, before any game is restored.
    if (!host.firstChild) {
      frame = requestAnimationFrame(sample);
      return;
    }
    const status = useEngineStore.getState().status;
    const overlay = host.querySelector('[role="status"]');
    frames.push({
      phase: useAppPhaseStore.getState().phase.kind,
      readyGame: status.phase === "ready" ? (status.scene?.gameId ?? null) : null,
      opacity: overlay ? Number(getComputedStyle(overlay).opacity) : null,
    });
    frame = requestAnimationFrame(sample);
  };
  frame = requestAnimationFrame(sample);
  return {
    frames,
    connect() {
      useGameStore.setState({ game, currentPlayer: game.currentPlayer, isConnected: true });
    },
    state() {
      return {
        phase: useAppPhaseStore.getState().phase.kind,
        status: useEngineStore.getState().status,
      };
    },
    dispose() {
      cancelAnimationFrame(frame);
      root.unmount();
      host.remove();
    },
  };
}

export function mountInterruptedLoading() {
  const host = document.createElement("div");
  document.body.append(host);
  const root = createRoot(host);
  let loaded = false;
  const render = () =>
    root.render(
      <LoadingOverlay
        isLoaded={loaded}
        showDelayMs={0}
        minDurationMs={0}
        fadeDurationMs={1000}
        message="Preparing world"
      />,
    );
  render();
  return {
    setLoaded(value: boolean) {
      loaded = value;
      render();
    },
    dispose() {
      root.unmount();
      host.remove();
    },
  };
}
