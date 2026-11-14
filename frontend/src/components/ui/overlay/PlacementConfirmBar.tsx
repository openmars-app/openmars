import React, { useEffect, useMemo, useRef } from "react";
import GameButton from "../buttons/GameButton.tsx";
import GameIcon from "../display/GameIcon.tsx";
import TileInfoContent, { type TileTooltipData } from "../display/TileInfoContent.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { useReducedMotion } from "@/hooks/useReducedMotion.ts";
import { sendTileSelection, usePlacementSelectionStore } from "@/stores/placementSelectionStore.ts";
import { GameDto } from "@/types/generated/api-types.ts";
import { HexGrid2D } from "@/utils/hex-grid-2d.ts";
import { boardBottomOverlayStyle, getTileIconType } from "@/utils/boardOverlay.ts";

const BAR_STYLE = boardBottomOverlayStyle(560);

interface PlacementConfirmBarProps {
  gameState: GameDto;
  visible: boolean;
  uiAnimationClass?: string;
}

const PlacementConfirmBar: React.FC<PlacementConfirmBarProps> = ({
  gameState,
  visible,
  uiAnimationClass = "",
}) => {
  const { isCompact } = useLayoutMode();
  const reducedMotion = useReducedMotion();
  const selectedHex = usePlacementSelectionStore((s) => s.selectedHex);
  const clear = usePlacementSelectionStore((s) => s.clear);
  const pendingTileSelection = gameState.currentPlayer?.pendingTileSelection;
  const availableSignature = pendingTileSelection?.availableHexes.join("|") ?? "";

  useEffect(() => {
    clear();
  }, [availableSignature, isCompact, gameState.id, clear]);

  const info = useMemo((): TileTooltipData | null => {
    if (!selectedHex) {
      return null;
    }
    const tile = gameState.board?.tiles?.find(
      (t) => HexGrid2D.coordinateToKey(t.coordinates) === selectedHex,
    );
    if (!tile) {
      return null;
    }
    const bonuses: { [key: string]: number } = {};
    for (const bonus of tile.bonuses ?? []) {
      bonuses[bonus.type] = bonus.amount;
    }
    return {
      tileType: "empty",
      displayName: tile.displayName,
      isOceanSpace: tile.type === "ocean-space",
      isVolcanic: tile.tags?.includes("volcanic") ?? false,
      bonuses,
    };
  }, [selectedHex, gameState.board?.tiles]);

  const lastInfo = useRef<TileTooltipData | null>(null);
  if (info) {
    lastInfo.current = info;
  }
  const shownInfo = info ?? lastInfo.current;

  if (!visible || !pendingTileSelection) {
    return null;
  }

  if (!isCompact) {
    return (
      <div
        role="group"
        aria-label="Tile placement"
        aria-live="polite"
        className={`pointer-events-none fixed top-[66px] left-1/2 max-w-[calc(100vw-32px)] -translate-x-1/2 game-panel game-panel-clipped game-window px-6 py-3 ${uiAnimationClass}`}
        style={{ zIndex: Z_INDEX.TILE_PLACEMENT_PROMPT }}
      >
        <div className="flex items-center gap-2">
          <span className="font-orbitron text-lg text-white tracking-wider-2xl">Place</span>
          <GameIcon iconType={getTileIconType(pendingTileSelection.tileType)} size="medium" />
        </div>
      </div>
    );
  }

  const expanded = !!selectedHex;
  const transitionClass = reducedMotion
    ? ""
    : "transition-[grid-template-columns,opacity] duration-300 ease-out";

  return (
    <div
      className="fixed w-max -translate-x-1/2 animate-[fadeIn_150ms_ease-in]"
      style={{ ...BAR_STYLE, zIndex: Z_INDEX.PLACEMENT_CONFIRM_BAR }}
    >
      <div
        className="game-panel flex items-center text-white/90 text-xs leading-tight px-3 py-2 min-h-[60px]"
        style={{ "--panel-cut": "14px" } as React.CSSProperties}
      >
        <div className="flex items-center gap-2 shrink-0">
          <span className="font-orbitron text-sm text-white tracking-wider">Place</span>
          <GameIcon iconType={getTileIconType(pendingTileSelection.tileType)} size="medium" />
        </div>
        <div
          className={`grid ${transitionClass} ${expanded ? "opacity-100" : "opacity-0"}`}
          style={{ gridTemplateColumns: expanded ? "1fr" : "0fr" }}
          aria-hidden={!expanded}
          inert={!expanded}
        >
          <div className="min-w-0 overflow-hidden">
            <div className="flex items-center gap-3 pl-3 whitespace-nowrap">
              {shownInfo && (
                <div className="shrink-0">
                  <TileInfoContent data={shownInfo} />
                </div>
              )}
              <div className="flex items-center gap-2 shrink-0">
                <GameButton emphasis="secondary" size="sm" height={44} onClick={clear}>
                  Cancel
                </GameButton>
                <GameButton
                  tone="success"
                  size="sm"
                  height={44}
                  onClick={() => {
                    if (selectedHex) {
                      sendTileSelection(selectedHex);
                    }
                  }}
                >
                  Confirm
                </GameButton>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PlacementConfirmBar;
