import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { GamePopover } from "../GamePopover";
import GameButton from "../buttons/GameButton.tsx";
import { MENU_POPOVER_WIDTH, useMenuPopoverPosition } from "./useMenuPopoverPosition.ts";
import { apiService } from "@/services/apiService";
import { GameDto } from "@/types/generated/api-types";
import { useNotifications } from "@/contexts/NotificationContext";

const UUID_V4_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

interface EnterCodePopoverProps {
  isVisible: boolean;
  onClose: () => void;
  onGameValidated: (game: GameDto) => void;
  initialCode?: string;
  anchorRef: React.RefObject<HTMLButtonElement | null>;
}

const EnterCodePopover: React.FC<EnterCodePopoverProps> = ({
  isVisible,
  onClose,
  onGameValidated,
  initialCode,
  anchorRef,
}) => {
  const navigate = useNavigate();
  const { showNotification } = useNotifications();
  const [gameId, setGameId] = useState(initialCode || "");
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (initialCode) {
      setGameId(initialCode);
    }
  }, [initialCode]);

  useEffect(() => {
    if (isVisible && initialCode) {
      void handleConnect(initialCode);
    }
  }, [isVisible, initialCode]);

  const handleConnect = async (code = gameId) => {
    const trimmed = code.trim();

    if (!trimmed) {
      showNotification({ message: "Please enter a game ID", type: "error" });
      return;
    }

    if (!UUID_V4_REGEX.test(trimmed)) {
      showNotification({ message: "Please enter a valid game ID (UUID format)", type: "error" });
      return;
    }

    setIsLoading(true);

    try {
      const game = await apiService.getGame(trimmed);
      if (!game) {
        throw new Error("Game not found");
      }

      if (game.resumeLobby) {
        onGameValidated(game);
        return;
      }

      if (game.status !== "lobby" && game.status !== "waiting") {
        navigate(`/game/${trimmed}`);
        return;
      }

      if (
        (game.currentPlayer?.id ? 1 : 0) + (game.otherPlayers?.length || 0) >=
        (game.settings?.maxPlayers || 10)
      ) {
        throw new Error("Game is full");
      }

      onGameValidated(game);
    } catch (err) {
      if (err instanceof Error) {
        if (err.message.includes("404") || err.message.includes("not found")) {
          showNotification({
            message: "Game not found. Please check the game ID.",
            type: "error",
          });
        } else if (err.message.includes("full")) {
          showNotification({
            message: "This game is full. Please try another game.",
            type: "error",
          });
        } else {
          showNotification({ message: err.message || "Failed to find game", type: "error" });
        }
      } else {
        showNotification({ message: "Failed to find game", type: "error" });
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <EnterCodePopoverView
      isVisible={isVisible}
      onClose={onClose}
      anchorRef={anchorRef}
      gameId={gameId}
      onCodeChange={setGameId}
      isLoading={isLoading}
      onConnect={() => {
        void handleConnect();
      }}
    />
  );
};

export function EnterCodePopoverView({
  isVisible,
  onClose,
  anchorRef,
  gameId,
  onCodeChange: setGameId,
  isLoading,
  onConnect: handleConnect,
}: Pick<EnterCodePopoverProps, "isVisible" | "onClose" | "anchorRef"> & {
  gameId: string;
  onCodeChange: (code: string) => void;
  isLoading: boolean;
  onConnect: () => void;
}) {
  const position = useMenuPopoverPosition(anchorRef);
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      void handleConnect();
    }
  };

  return (
    <GamePopover
      isVisible={isVisible}
      onClose={onClose}
      position={position}
      theme="menu"
      header={{ title: "Enter game code", showCloseButton: true }}
      width={MENU_POPOVER_WIDTH}
      maxHeight="none"
      animation="slideDown"
    >
      <div className="flex flex-row gap-3 items-center px-4 py-4 min-w-0 overflow-hidden">
        <input
          type="text"
          value={gameId}
          onChange={(e) => setGameId(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Enter game ID"
          disabled={isLoading}
          spellCheck={false}
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="off"
          autoFocus
          className="flex-1 min-w-0 bg-black/50 border border-white/20 rounded-none py-3 px-4 text-white text-base outline-none placeholder:text-white/50 focus:border-white/60 focus:shadow-[0_0_20px_rgba(255,255,255,0.1)] transition-all duration-200 disabled:opacity-60"
        />
        <GameButton
          size="md"
          onClick={() => void handleConnect()}
          disabled={isLoading || !gameId.trim()}
          className="shrink-0"
        >
          {isLoading ? "..." : "Connect"}
        </GameButton>
      </div>
    </GamePopover>
  );
}

export default EnterCodePopover;
