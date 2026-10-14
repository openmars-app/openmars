import GameButton from "@/components/ui/buttons/GameButton.tsx";
import React, { useState } from "react";
import { GameDto } from "@/types/generated/api-types";
import { globalWebSocketManager } from "@/services/globalWebSocketManager";
import { saveGameSession } from "@/utils/sessionStorage";
import GameMenuModal from "./GameMenuModal";

interface SpectateGameOverlayProps {
  game: GameDto;
  onCancel: () => void;
  onConnected: () => void;
}

const SpectateGameOverlay: React.FC<SpectateGameOverlayProps> = ({
  game,
  onCancel,
  onConnected,
}) => {
  const [spectatorName, setSpectatorName] = useState("");
  const [isConnecting, setIsConnecting] = useState(false);

  const handleSpectate = async () => {
    const name = spectatorName.trim();
    if (!name) return;
    setIsConnecting(true);
    saveGameSession({ gameId: game.id, playerId: "", playerName: name, isSpectator: true });
    await globalWebSocketManager.spectatorConnect(name, game.id);
    onConnected();
  };

  return (
    <SpectateGameOverlayView
      spectatorName={spectatorName}
      onNameChange={setSpectatorName}
      isConnecting={isConnecting}
      onSpectate={() => {
        void handleSpectate();
      }}
      onCancel={onCancel}
    />
  );
};

export function SpectateGameOverlayView({
  spectatorName,
  onNameChange,
  isConnecting,
  onSpectate,
  onCancel,
}: {
  spectatorName: string;
  onNameChange: (name: string) => void;
  isConnecting: boolean;
  onSpectate: () => void;
  onCancel: () => void;
}) {
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      onSpectate();
    }
  };

  return (
    <GameMenuModal
      title="Spectate Game"
      onBack={onCancel}
      backLabel="Main menu"
      showBackdrop={true}
      onClose={onCancel}
      showCloseButton={true}
    >
      <div className="flex flex-row gap-3 items-center px-2">
        <input
          type="text"
          value={spectatorName}
          onChange={(e) => onNameChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Enter your name"
          disabled={isConnecting}
          spellCheck={false}
          autoComplete="off"
          autoCorrect="off"
          maxLength={50}
          autoFocus
          className="flex-1 bg-black/50 border border-white/20 rounded-none py-3 px-4 text-white text-base outline-none placeholder:text-white/50 focus:border-white/60 focus:shadow-[0_0_20px_rgba(255,255,255,0.1)] transition-all duration-200 disabled:opacity-60"
        />
        <GameButton
          emphasis="quiet"
          onClick={onSpectate}
          disabled={isConnecting || !spectatorName.trim()}
          className="font-orbitron bg-white/10 border border-white/20 rounded-none py-3 px-6 text-white text-sm font-medium hover:bg-white/20 transition-colors disabled:opacity-50 disabled:cursor-default"
        >
          {isConnecting ? "Joining..." : "Spectate"}
        </GameButton>
      </div>
    </GameMenuModal>
  );
}

export default SpectateGameOverlay;
