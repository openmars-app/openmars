import React from "react";
import { GamePopover } from "../GamePopover";
import GameButton from "../buttons/GameButton.tsx";
import { MENU_POPOVER_WIDTH, useMenuPopoverPosition } from "./useMenuPopoverPosition.ts";
import { GameDto } from "@/types/generated/api-types";
import { useJoinGame } from "@/hooks/useJoinGame";
import LoadingOverlay from "../../game/view/LoadingOverlay";
import { MAX_PLAYER_NAME_LENGTH } from "@/constants/gameConstants";

interface JoinGamePopoverProps {
  isVisible: boolean;
  onClose: () => void;
  game: GameDto;
  anchorRef: React.RefObject<HTMLElement | null>;
}

const JoinGamePopover: React.FC<JoinGamePopoverProps> = ({
  isVisible,
  onClose,
  game,
  anchorRef,
}) => {
  const join = useJoinGame({ game });
  return (
    <JoinGamePopoverView isVisible={isVisible} onClose={onClose} anchorRef={anchorRef} {...join} />
  );
};

export function JoinGamePopoverView({
  isVisible,
  onClose,
  anchorRef,
  playerName,
  setPlayerName,
  isLoading,
  handleJoin,
  handleKeyDown,
  loadingMessage,
}: Omit<JoinGamePopoverProps, "game"> & ReturnType<typeof useJoinGame>) {
  const position = useMenuPopoverPosition(anchorRef);

  return (
    <>
      <GamePopover
        isVisible={isVisible}
        onClose={onClose}
        position={position}
        theme="menu"
        header={{ title: "Join game", showCloseButton: true }}
        width={MENU_POPOVER_WIDTH}
        maxHeight="none"
        animation="slideDown"
      >
        <div className="flex flex-row gap-3 items-center px-4 py-4 min-w-0 overflow-hidden">
          <input
            type="text"
            value={playerName}
            onChange={(e) => setPlayerName(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Enter your name"
            disabled={isLoading}
            spellCheck={false}
            autoComplete="off"
            autoCorrect="off"
            maxLength={MAX_PLAYER_NAME_LENGTH}
            autoFocus
            className="flex-1 min-w-0 bg-black/50 border border-white/20 rounded-none py-3 px-4 text-white text-base outline-none placeholder:text-white/50 focus:border-white/60 focus:shadow-[0_0_20px_rgba(255,255,255,0.1)] transition-all duration-200 disabled:opacity-60"
          />
          <GameButton
            size="md"
            onClick={() => void handleJoin()}
            disabled={isLoading || !playerName.trim()}
            className="shrink-0"
          >
            {isLoading ? "..." : "Join"}
          </GameButton>
        </div>
      </GamePopover>

      {isLoading && <LoadingOverlay isLoaded={false} message={loadingMessage} />}
    </>
  );
}

export default JoinGamePopover;
