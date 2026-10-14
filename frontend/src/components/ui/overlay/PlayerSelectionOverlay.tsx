import GameButton from "@/components/ui/buttons/GameButton.tsx";
import React, { useState } from "react";
import { GameDto, OtherPlayerDto, PlayerDto } from "../../../types/generated/api-types.ts";
import { getCorporationLogo } from "../../../utils/corporationLogos.tsx";
import { globalWebSocketManager } from "../../../services/globalWebSocketManager.ts";
import { saveGameSession } from "../../../utils/sessionStorage.ts";
import GameMenuModal from "./GameMenuModal.tsx";

interface PlayerSelectionOverlayProps {
  game: GameDto;
  onSelectPlayer: (playerId: string, playerName: string) => void;
  onSpectate: () => void;
  onCancel: () => void;
  visible?: boolean;
  onExited?: () => void;
}

const PlayerSelectionOverlay: React.FC<PlayerSelectionOverlayProps> = ({
  game,
  onSelectPlayer,
  onSpectate,
  onCancel,
  visible,
  onExited,
}) => {
  const [spectatorName, setSpectatorName] = useState("");
  const [isConnecting, setIsConnecting] = useState(false);

  const handleSpectate = async () => {
    const name = spectatorName.trim();
    if (!name) return;
    setIsConnecting(true);
    saveGameSession({ gameId: game.id, playerId: "", playerName: name, isSpectator: true });
    await globalWebSocketManager.spectatorConnect(name, game.id);
    onSpectate();
  };
  return (
    <PlayerSelectionOverlayView
      game={game}
      onSelectPlayer={onSelectPlayer}
      onCancel={onCancel}
      visible={visible}
      onExited={onExited}
      spectatorName={spectatorName}
      onNameChange={setSpectatorName}
      isConnecting={isConnecting}
      onSpectate={() => {
        void handleSpectate();
      }}
    />
  );
};

export function PlayerSelectionOverlayView({
  game,
  onSelectPlayer,
  onSpectate,
  onCancel,
  visible,
  onExited,
  spectatorName,
  onNameChange,
  isConnecting,
}: PlayerSelectionOverlayProps & {
  spectatorName: string;
  onNameChange: (name: string) => void;
  isConnecting: boolean;
}) {
  const allPlayers: (PlayerDto | OtherPlayerDto)[] = [
    ...(game.currentPlayer ? [game.currentPlayer] : []),
    ...(game.otherPlayers || []),
  ];

  const orderedPlayers =
    game.turnOrder && game.turnOrder.length > 0
      ? game.turnOrder
          .map((pid) => allPlayers.find((p) => p.id === pid))
          .filter((player): player is PlayerDto | OtherPlayerDto => player !== undefined)
      : allPlayers;

  return (
    <GameMenuModal
      title="Reconnect to Game"
      subtitle="Select a player to reconnect as"
      onBack={onCancel}
      backLabel="Main menu"
      visible={visible}
      onExited={onExited}
    >
      <div className="mb-6">
        <h3 className="text-white text-sm font-semibold mb-2 uppercase tracking-wide">Players</h3>
        <div className="flex flex-col gap-2">
          {orderedPlayers.map((player) => {
            const isExited = player.isExited;
            const isConnected = player.isConnected;
            const canSelect = !isConnected && !isExited;

            return (
              <GameButton
                emphasis="quiet"
                key={player.id}
                onClick={() => canSelect && onSelectPlayer(player.id, player.name)}
                disabled={!canSelect}
                className={`flex justify-between items-center py-3 px-4 bg-black/40 rounded-none border transition-all text-left w-full ${
                  canSelect
                    ? "border-space-blue-600/50 hover:border-space-blue-400 hover:bg-black/60 cursor-pointer"
                    : "border-white/10 opacity-50 cursor-default"
                }`}
              >
                <div className="flex items-center gap-3">
                  {player.corporation && (
                    <div className="w-[80px] h-6 flex-shrink-0 flex items-center justify-start overflow-hidden">
                      {getCorporationLogo(player.corporation.name, "w-20 h-6", "80px")}
                    </div>
                  )}
                  <span className="text-white text-sm font-medium">{player.name}</span>
                </div>
                {isExited ? (
                  <span className="px-1.5 py-px rounded-none text-[8px] compact:text-[11px] font-semibold uppercase tracking-[0.3px] shadow-[0_1px_2px_rgba(0,0,0,0.2)] bg-[linear-gradient(135deg,#e74c3c,#c0392b)] text-white border border-[rgba(231,76,60,0.5)]">
                    EXITED
                  </span>
                ) : !isConnected ? (
                  <span className="px-1.5 py-px rounded-none text-[8px] compact:text-[11px] font-semibold uppercase tracking-[0.3px] shadow-[0_1px_2px_rgba(0,0,0,0.2)] bg-[linear-gradient(135deg,#e74c3c,#c0392b)] text-white border border-[rgba(231,76,60,0.5)]">
                    DISCONNECTED
                  </span>
                ) : (
                  <span className="px-1.5 py-px rounded-none text-[8px] compact:text-[11px] font-semibold uppercase tracking-[0.3px] shadow-[0_1px_2px_rgba(0,0,0,0.2)] bg-[linear-gradient(135deg,#3498db,#2980b9)] text-white border border-[rgba(52,152,219,0.5)]">
                    CONNECTED
                  </span>
                )}
              </GameButton>
            );
          })}
        </div>
      </div>

      <div className="flex items-center gap-3 my-5">
        <div className="flex-1 h-px bg-white/15" />
        <span className="text-white/30 text-xs font-orbitron uppercase tracking-wider">or</span>
        <div className="flex-1 h-px bg-white/15" />
      </div>

      <div className="flex flex-row gap-3 items-center">
        <input
          type="text"
          value={spectatorName}
          onChange={(e) => onNameChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") onSpectate();
          }}
          placeholder="Enter your name"
          disabled={isConnecting}
          spellCheck={false}
          autoComplete="off"
          autoCorrect="off"
          maxLength={50}
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

export default PlayerSelectionOverlay;
