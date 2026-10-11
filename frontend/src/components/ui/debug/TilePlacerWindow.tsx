import FloatingWindow from "./FloatingWindow.tsx";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import React from "react";
import { globalWebSocketManager } from "../../../services/globalWebSocketManager.ts";
import {
  AdminCommandRequest,
  AdminCommandTypeStartTileSelection,
  StartTileSelectionAdminCommand,
  PlaceableTileTypeDto,
} from "../../../types/generated/api-types.ts";
import { useWindowDrag, useWindowManager } from "./WindowManager.tsx";

interface TilePlacerWindowProps {
  tileTypes: PlaceableTileTypeDto[];
  playerId: string;
  playerName: string;
  onClose: () => void;
}

const WINDOW_ID = "tile-placer";
const WINDOW_WIDTH = 220;

const TilePlacerWindow: React.FC<TilePlacerWindowProps> = ({
  tileTypes,
  playerId,
  playerName,
  onClose,
}) => {
  const { position, handleMouseDown } = useWindowDrag({
    windowId: WINDOW_ID,
    width: WINDOW_WIDTH,
    height: 600,
    defaultPosition:
      typeof window !== "undefined"
        ? { x: window.innerWidth - WINDOW_WIDTH - 40, y: 60 }
        : undefined,
  });

  const { getZIndex } = useWindowManager();

  const handleTileClick = async (tileType: string) => {
    const command: StartTileSelectionAdminCommand = {
      playerId: playerId,
      tileType: tileType,
    };
    const adminRequest: AdminCommandRequest = {
      commandType: AdminCommandTypeStartTileSelection as any,
      payload: command,
    };
    try {
      await globalWebSocketManager.sendAdminCommand(adminRequest);
    } catch (error) {
      console.error("Failed to send tile placer command:", error);
    }
  };

  return (
    <FloatingWindow
      title="Tile Placer"
      subtitle={playerName}
      onClose={onClose}
      onMouseDown={handleMouseDown}
      style={{
        top: position.y,
        left: position.x,
        width: WINDOW_WIDTH,
        maxHeight: "calc(100dvh - 100px)",
        zIndex: getZIndex(WINDOW_ID),
      }}
    >
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "4px",
          maxHeight: "400px",
          overflowY: "auto",
        }}
      >
        {tileTypes.map((tile) => (
          <GameButton
            emphasis="secondary"
            key={tile.type}
            onClick={() => void handleTileClick(tile.type)}
            onMouseDown={(e) => e.stopPropagation()}
            size="sm"
            className="!justify-start"
          >
            {tile.label}
          </GameButton>
        ))}
      </div>
    </FloatingWindow>
  );
};

export default TilePlacerWindow;
