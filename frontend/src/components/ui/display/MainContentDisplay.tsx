import TileInspectTooltip from "./TileInspectTooltip.tsx";
import PlacementConfirmBar from "../overlay/PlacementConfirmBar.tsx";
import type { GameDto } from "@/types/generated/api-types.ts";

interface MainContentDisplayProps {
  gameState: GameDto;
  showUI?: boolean;
  uiAnimationClass?: string;
}

export default function MainContentDisplay({
  gameState,
  showUI = true,
  uiAnimationClass = "",
}: MainContentDisplayProps) {
  return (
    <>
      <TileInspectTooltip gameState={gameState} visible={showUI} />
      <PlacementConfirmBar
        gameState={gameState}
        visible={showUI}
        uiAnimationClass={uiAnimationClass}
      />
    </>
  );
}
