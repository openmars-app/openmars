import { useEffect } from "react";
import { createRoot } from "react-dom/client";
import "../../src/index.css";
import EngineBackground from "../../src/engine/EngineBackground";
import LoadingOverlay from "../../src/components/game/view/LoadingOverlay";
import TopMenuBar from "../../src/components/layout/panels/TopMenuBar";
import LayoutModeAttribute from "../../src/components/LayoutModeAttribute";
import { SoundProvider } from "../../src/contexts/SoundContext";
import { createPreviewGame } from "../../src/components/ui/debug/uiDebug/fixtures";
import { useEngineStore } from "../../src/stores/engineStore";
import { useGameStore } from "../../src/stores/gameStore";
import { useAppPhaseStore } from "../../src/stores/appPhaseStore";
import { useUIOverlayStore } from "../../src/stores/uiOverlayStore";
import { useGraphicsSettingsStore } from "../../src/stores/graphicsSettingsStore";
import { useCameraSettingsStore } from "../../src/stores/cameraSettingsStore";
import { useRenderPauseStore, isRenderPaused } from "../../src/stores/renderPauseStore";
import { isGameSceneReady } from "../../src/engine/status";
import { Z_INDEX } from "../../src/constants/zIndex";
import { particleSceneGame } from "./scene";
import type { FocusTarget, GraphicsSettings } from "../../src/engine/scene.generated";

export function mountGraphics(toolbar = true) {
  document.documentElement.style.setProperty("--surface-layer", String(Z_INDEX.CONTROL_DECORATION));
  const game = createPreviewGame();
  game.id = "graphics-acceptance";
  game.board = particleSceneGame().board;
  game.otherPlayers = [];
  useGameStore.setState({ game, currentPlayer: game.currentPlayer, isConnected: true });
  useAppPhaseStore.getState().setPhase({ kind: "playing", gameId: game.id });
  useCameraSettingsStore.getState().update({ smoothing: 0 });
  const host = document.createElement("div");
  document.body.append(host);
  const root = createRoot(host);
  const phases: string[] = [];
  const unsubscribe = useEngineStore.subscribe((s) => phases.push(s.status.phase));
  function Shell() {
    const status = useEngineStore((s) => s.status);
    const paused = useRenderPauseStore(isRenderPaused);
    const performance = useEngineStore((s) => s.performance);
    useEffect(() => {
      performance?.setActive(true);
      return () => performance?.setActive(false);
    }, [performance]);
    return (
      <SoundProvider>
        <LayoutModeAttribute />
        <EngineBackground paused={paused} />
        {toolbar && <TopMenuBar gameState={game} currentPlayer={game.currentPlayer} />}
        <LoadingOverlay
          isLoaded={isGameSceneReady(status, game.id, game.settings.mapId)}
          showDelayMs={0}
          fadeDurationMs={1500}
          message={status.message}
          showProgress
        />
      </SoundProvider>
    );
  }
  root.render(<Shell />);
  return {
    phases,
    inspect() {
      const state = useEngineStore.getState();
      return {
        status: state.status,
        scene: state.inspectScene?.(),
        performance: state.performance?.sample(),
        preferences: useGraphicsSettingsStore.getState().settings,
      };
    },
    focus(focus: FocusTarget) {
      useEngineStore.getState().setFocus(focus);
    },
    zoom(value: number) {
      useEngineStore.getState().camera?.(value);
    },
    settings() {
      useUIOverlayStore.getState().setShowSettings(true);
    },
    apply(patch: Partial<GraphicsSettings>) {
      const store = useGraphicsSettingsStore.getState();
      store.apply({ ...store.settings, ...patch });
    },
    dispose() {
      unsubscribe();
      root.unmount();
      host.remove();
    },
  };
}
