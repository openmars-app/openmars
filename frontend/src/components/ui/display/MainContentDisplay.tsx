import React, { useCallback } from "react";
import { getLayoutMode } from "@/hooks/useLayoutMode";
import { sendTileSelection, usePlacementSelectionStore } from "@/stores/placementSelectionStore";
import Game3DView from "../../game/view/Game3DView.tsx";
import TileInspectTooltip from "./TileInspectTooltip.tsx";
import PlacementConfirmBar from "../overlay/PlacementConfirmBar.tsx";
import { GameDto } from "@/types/generated/api-types.ts";

interface MainContentDisplayProps {
  gameState: GameDto;
  animateHexEntrance?: boolean;
  startDark?: boolean;
  tilesHidden?: boolean;
  onSkyboxReady?: () => void;
  onGpuReady?: () => void;
  showUI?: boolean;
  uiAnimationClass?: string;
}

const MainContentDisplay: React.FC<MainContentDisplayProps> = ({
  gameState,
  animateHexEntrance = false,
  startDark = false,
  tilesHidden = false,
  onSkyboxReady,
  onGpuReady,
  showUI = true,
  uiAnimationClass = "",
}) => {
  const onHexClick = useCallback(
    (coordinate: string) => {
      if (!gameState.currentPlayer.pendingTileSelection?.availableHexes.includes(coordinate)) {
        return;
      }
      if (getLayoutMode().isCompact) {
        const { selectedHex, select } = usePlacementSelectionStore.getState();
        if (selectedHex !== coordinate) {
          select(coordinate);
          return;
        }
      }
      sendTileSelection(coordinate);
    },
    [gameState.currentPlayer],
  );
  return (
    <>
      <Game3DView
        gameState={gameState}
        onHexClick={onHexClick}
        animateHexEntrance={animateHexEntrance}
        startDark={startDark}
        tilesHidden={tilesHidden}
        onSkyboxReady={onSkyboxReady}
        onGpuReady={onGpuReady}
        showUI={showUI}
        uiAnimationClass={uiAnimationClass}
      />
      <TileInspectTooltip gameState={gameState} visible={showUI} />
      <PlacementConfirmBar gameState={gameState} visible={showUI} />
    </>
  );
};

export default MainContentDisplay;
