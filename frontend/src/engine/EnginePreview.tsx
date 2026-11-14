import EngineBackground from "./EngineBackground";
import LoadingOverlay from "@/components/game/view/LoadingOverlay";
import RotateDeviceOverlay from "@/components/ui/overlay/RotateDeviceOverlay";
import { useEngineStore } from "@/stores/engineStore";
import { isRenderPaused, useRenderPauseStore } from "@/stores/renderPauseStore";
import { useGameStore } from "@/stores/gameStore";
import MainContentDisplay from "@/components/ui/display/MainContentDisplay";
import { needsGameScene, useAppPhaseStore } from "@/stores/appPhaseStore";
import LayoutModeAttribute from "@/components/LayoutModeAttribute";

/** Development entry using the same canvas, loader, and lifecycle as the game. */
export default function EnginePreview() {
  const status = useEngineStore((s) => s.status);
  const paused = useRenderPauseStore(isRenderPaused);
  const game = useGameStore((s) => s.game);
  const phase = useAppPhaseStore((s) => s.phase);
  return (
    <>
      <LayoutModeAttribute />
      <EngineBackground paused={paused} menu={false} />
      {game && needsGameScene(phase) && <MainContentDisplay gameState={game} />}
      <LoadingOverlay
        isLoaded={status.phase === "ready" || status.phase === "error"}
        message={status.message}
        showDelayMs={0}
        showProgress
        progress={
          status.phase === "loading" && status.total ? status.loaded / status.total : undefined
        }
      />
      <RotateDeviceOverlay />
    </>
  );
}
