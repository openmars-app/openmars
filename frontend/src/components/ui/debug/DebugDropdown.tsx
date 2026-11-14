import GameButton from "../buttons/GameButton";
import FloatingWindow from "./FloatingWindow.tsx";
import React, { useState, useEffect } from "react";
import { useWindowDrag, useWindowManager } from "./WindowManager.tsx";
import { GameDto } from "../../../types/generated/api-types.ts";
import SidebarNav, { type ActiveItem } from "./SidebarNav.tsx";
import GameStatePanel from "./panels/GameStatePanel.tsx";
import PlayerResourcesPage from "./panels/PlayerResourcesPage.tsx";
import PlayerBehaviorPage from "./panels/PlayerBehaviorPage.tsx";
import PlaceTilePage from "./panels/PlaceTilePage.tsx";
import GameCommandsPage from "./panels/GameCommandsPage.tsx";
import TurnCommandsPage from "./panels/TurnCommandsPage.tsx";
import BotInspectorPage from "./panels/BotInspectorPage.tsx";

const WINDOW_ID = "admin-tools";
const WINDOW_WIDTH = 780;
const EXCLUDE_SELECTORS = [".tree-expand-toggle", ".tree-node-content", ".debug-content-area"];

interface DebugDropdownProps {
  isVisible: boolean;
  onClose: () => void;
  gameState: GameDto | null;
  changedPaths?: Set<string>;
  onOpenUIDebug?: () => void;
}

const DebugDropdown: React.FC<DebugDropdownProps> = ({
  isVisible,
  onClose,
  gameState,
  changedPaths = new Set(),
  onOpenUIDebug,
}) => {
  const [activeItem, setActiveItem] = useState<ActiveItem>("game-state");
  const [playerSelection, setPlayerSelection] = useState<{
    gameId: string | null;
    playerId: string;
  }>({ gameId: null, playerId: "" });

  const { position, handleMouseDown } = useWindowDrag({
    windowId: WINDOW_ID,
    width: WINDOW_WIDTH,
    height: () => window.innerHeight * 0.7,
    excludeSelectors: EXCLUDE_SELECTORS,
    isVisible,
  });

  const { getZIndex } = useWindowManager();

  const gameId = gameState?.id ?? null;
  const allPlayers = gameState
    ? [gameState.currentPlayer, ...gameState.otherPlayers].filter((player) => player?.id)
    : [];
  const preferredPlayerId = playerSelection.gameId === gameId ? playerSelection.playerId : "";
  const selectedPlayerId =
    allPlayers.find((player) => player.id === preferredPlayerId)?.id ?? allPlayers[0]?.id ?? "";
  const setSelectedPlayerId = (playerId: string) => setPlayerSelection({ gameId, playerId });

  useEffect(() => {
    // Resolve against the current roster before rendering any player commands.
    // A native select can display its first option while still holding a stale ID.
    setPlayerSelection((previous) =>
      previous.gameId === gameId && previous.playerId === selectedPlayerId
        ? previous
        : { gameId, playerId: selectedPlayerId },
    );
  }, [gameId, selectedPlayerId]);

  if (!isVisible) {
    return null;
  }

  const isCommandItem = (item: ActiveItem) => item !== "game-state";
  const contentOverflow = (item: ActiveItem) => {
    if (item === "bots") {
      return "hidden";
    }
    return isCommandItem(item) ? "visible" : "auto";
  };

  const renderContent = () => {
    if (activeItem === "game-state") {
      return <GameStatePanel gameState={gameState} changedPaths={changedPaths} />;
    }

    if (!gameState) {
      return (
        <div style={{ color: "#666", textAlign: "center", padding: "20px" }}>
          No game state available
        </div>
      );
    }

    const playerProps = {
      gameState,
      selectedPlayerId,
      onPlayerChange: setSelectedPlayerId,
    };

    switch (activeItem) {
      case "player-resources":
        return <PlayerResourcesPage {...playerProps} />;
      case "player-behavior":
        return <PlayerBehaviorPage {...playerProps} />;
      case "place-tile":
        return <PlaceTilePage {...playerProps} />;
      case "game-commands":
        return <GameCommandsPage gameState={gameState} />;
      case "turn-commands":
        return <TurnCommandsPage gameState={gameState} />;
      case "bots":
        return <BotInspectorPage gameState={gameState} />;
      default:
        return null;
    }
  };

  return (
    <FloatingWindow
      title="Admin Tools"
      onClose={onClose}
      onMouseDown={handleMouseDown}
      style={{
        top: position.y,
        left: position.x,
        width: WINDOW_WIDTH,
        maxHeight: "70vh",
        zIndex: getZIndex(WINDOW_ID),
      }}
    >
      {gameState?.settings.developmentMode && onOpenUIDebug && (
        <GameButton onClick={onOpenUIDebug} className="self-start mb-3 min-h-11">
          UI Debug
        </GameButton>
      )}
      <div
        style={{
          display: "flex",
          flex: 1,
          overflow: "hidden",
          minHeight: 0,
        }}
      >
        <SidebarNav
          activeItem={activeItem}
          onSelectItem={setActiveItem}
          developmentMode={gameState?.settings.developmentMode ?? false}
        />

        <div
          className="debug-content-area"
          style={{
            flex: 1,
            overflow: contentOverflow(activeItem),
            padding: "16px",
            display: "flex",
            flexDirection: "column",
          }}
          onMouseDown={(e) => e.stopPropagation()}
        >
          {renderContent()}
        </div>
      </div>
    </FloatingWindow>
  );
};

export default DebugDropdown;
