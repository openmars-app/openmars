import GameButton from "@/components/ui/buttons/GameButton.tsx";
import React from "react";
import { GameDto } from "@/types/generated/api-types";
import { useJoinGame } from "@/hooks/useJoinGame";
import LoadingOverlay from "../../game/view/LoadingOverlay";
import GameMenuModal from "./GameMenuModal";
import { MAX_PLAYER_NAME_LENGTH } from "@/constants/gameConstants";

interface JoinGameOverlayProps {
  game: GameDto;
  onCancel: () => void;
  visible?: boolean;
  onExited?: () => void;
  title?: string;
  subtitle?: string;
}

const JoinGameOverlay: React.FC<JoinGameOverlayProps> = ({
  game,
  onCancel,
  visible,
  onExited,
  title = "Join game",
  subtitle,
}) => {
  const join = useJoinGame({ game });
  return (
    <JoinGameOverlayView
      onCancel={onCancel}
      visible={visible}
      onExited={onExited}
      title={title}
      subtitle={subtitle}
      {...join}
    />
  );
};

export function JoinGameOverlayView({
  onCancel,
  visible,
  onExited,
  title = "Join game",
  subtitle,
  playerName,
  setPlayerName,
  isLoading,
  handleJoin,
  handleKeyDown,
  loadingMessage,
  errorMessage,
}: Omit<JoinGameOverlayProps, "game"> & ReturnType<typeof useJoinGame>) {
  return (
    <>
      <GameMenuModal
        title={title}
        subtitle={subtitle}
        visible={visible}
        onExited={onExited}
        showBackdrop={true}
        onClose={onCancel}
      >
        <div className="flex flex-col gap-5">
          <input
            type="text"
            value={playerName}
            onChange={(e) => setPlayerName(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Enter your name"
            aria-label="Your name"
            disabled={isLoading}
            spellCheck={false}
            autoComplete="off"
            autoCorrect="off"
            maxLength={MAX_PLAYER_NAME_LENGTH}
            autoFocus
            className="game-input w-full disabled:opacity-60"
          />
          {errorMessage && (
            <p role="alert" className="text-sm text-red-300 text-left">
              {errorMessage}
            </p>
          )}
          <div className="grid grid-cols-2 gap-3">
            <GameButton emphasis="secondary" onClick={onCancel} disabled={isLoading}>
              Return
            </GameButton>
            <GameButton
              onClick={() => void handleJoin()}
              disabled={isLoading || !playerName.trim()}
            >
              {isLoading ? "Joining..." : "Join"}
            </GameButton>
          </div>
        </div>
      </GameMenuModal>

      {isLoading && <LoadingOverlay isLoaded={false} showDelayMs={0} message={loadingMessage} />}
    </>
  );
}

export default JoinGameOverlay;
