import React, { useEffect, useRef } from "react";
import TileTooltip from "./TileTooltip.tsx";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { useTileInspectStore, type ScreenPoint } from "@/stores/tileInspectStore.ts";
import { GameDto } from "@/types/generated/api-types.ts";

interface TileInspectTooltipProps {
  gameState: GameDto;
  visible: boolean;
}

/** On touch, a tapped hex shows the same tooltip desktop shows on hover, next to the tap. */
const TileInspectTooltip: React.FC<TileInspectTooltipProps> = ({ gameState, visible }) => {
  const { isCompact } = useLayoutMode();
  const info = useTileInspectStore((s) => s.info);
  const position = useTileInspectStore((s) => s.position);
  const clear = useTileInspectStore((s) => s.clear);
  const positionRef = useRef<ScreenPoint>({ x: 0, y: 0 });
  const hasPendingPlacement = Boolean(gameState.currentPlayer?.pendingTileSelection);

  useEffect(() => {
    clear();
  }, [hasPendingPlacement, gameState.currentPhase, gameState.id, isCompact, clear]);

  if (position) {
    positionRef.current = position;
  }

  if (!visible) {
    return null;
  }

  return <TileTooltip data={info} positionRef={positionRef} />;
};

export default TileInspectTooltip;
