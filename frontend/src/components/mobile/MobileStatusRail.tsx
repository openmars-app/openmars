import { useRef } from "react";
import {
  ResourceTypeOceanTile,
  ResourceTypeOxygen,
  ResourceTypeTR,
  ResourceTypeTemperature,
  ResourceTypeVenus,
  type GameDto,
  type OtherPlayerDto,
  type PlayerDto,
  type ResourceType,
} from "@/types/generated/api-types.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useMobileUiStore } from "@/stores/mobileUiStore.ts";
import GameIcon from "../ui/display/GameIcon.tsx";
import { useMobileGame } from "./MobileGameContext.tsx";
import MobilePlayerPresence from "./feedback/MobilePlayerPresence.tsx";

const MIN_TEMPERATURE = -30;
const MAX_TEMPERATURE = 8;
const MAX_OXYGEN = 14;
const MAX_VENUS = 30;

interface ParameterRowProps {
  icon: ResourceType;
  label: string;
  value: string;
  progress: number;
}

function ParameterRow({ icon, label, value, progress }: ParameterRowProps) {
  const percent = Math.round(Math.min(1, Math.max(0, progress)) * 100);
  return (
    <div className="flex flex-col gap-0.5 px-2 py-1" aria-label={`${label} ${value}`}>
      <div className="flex items-center gap-1.5">
        <GameIcon iconType={icon} size="small" className="!w-4 !h-4" />
        <span className="font-orbitron text-[12px] font-bold leading-none tabular-nums text-white">
          {value}
        </span>
      </div>
      <div className="h-[3px] w-full bg-white/10" aria-hidden="true">
        <div className="h-full bg-[#8fb8ca]" style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}

function orderedPlayers(gameState: GameDto): (PlayerDto | OtherPlayerDto)[] {
  const byId = new Map<string, PlayerDto | OtherPlayerDto>();
  if (gameState.currentPlayer) {
    byId.set(gameState.currentPlayer.id, gameState.currentPlayer);
  }
  for (const other of gameState.otherPlayers ?? []) {
    byId.set(other.id, other);
  }
  return (gameState.turnOrder ?? [])
    .map((id) => byId.get(id))
    .filter((player): player is PlayerDto | OtherPlayerDto => player !== undefined);
}

interface PlayerChipButtonProps {
  player: PlayerDto | OtherPlayerDto;
  isMe: boolean;
  isTurn: boolean;
}

function PlayerChipButton({ player, isMe, isTurn }: PlayerChipButtonProps) {
  const chipRef = useRef<HTMLButtonElement>(null);
  const color = player.color || "#6496ff";
  const inactive = player.isExited || (player.playerType !== "bot" && !player.isConnected);
  const passed = player.passed && !player.isExited;
  return (
    <button
      ref={chipRef}
      type="button"
      className={`relative w-full min-h-11 shrink-0 flex items-stretch text-left cursor-pointer border-b border-white/5 ${isTurn ? "bg-white/10" : ""} ${inactive ? "opacity-40" : ""}`}
      aria-label={`${player.name}${isMe ? " (you)" : ""}, TR ${player.terraformRating}${isTurn ? ", current turn" : ""}${player.passed ? ", passed" : ""}`}
      onClick={() => useMobileUiStore.getState().open("players", { playerId: player.id })}
    >
      <span
        className={`shrink-0 ${isTurn ? "w-1.5" : "w-1"}`}
        style={{
          backgroundColor: color,
          boxShadow: isTurn ? `0 0 8px ${color}` : undefined,
        }}
        aria-hidden="true"
      />
      <span className="flex-1 min-w-0 flex flex-col justify-center gap-0.5 px-1.5 py-1">
        <span className="flex items-center gap-1 min-w-0">
          {isTurn && (
            <span
              className="shrink-0 w-1.5 h-1.5 rounded-full animate-pulse"
              style={{ backgroundColor: color }}
              aria-hidden="true"
            />
          )}
          <span
            className={`truncate text-[11px] ${isMe ? "font-bold text-white" : "text-white/85"}`}
          >
            {player.name}
          </span>
        </span>
        {passed ? (
          <span className="font-orbitron text-[11px] leading-none text-white/45">Passed</span>
        ) : (
          <span className="flex items-center gap-1">
            <GameIcon iconType={ResourceTypeTR} size="small" className="!w-4 !h-4" />
            <span className="font-orbitron text-[12px] font-bold leading-none tabular-nums text-white">
              {player.terraformRating}
            </span>
          </span>
        )}
      </span>
      <MobilePlayerPresence playerId={player.id} anchorRef={chipRef} />
    </button>
  );
}

interface MobileStatusRailProps {
  turnPlayerId: string;
  showParameters: boolean;
  className?: string;
}

export default function MobileStatusRail({
  turnPlayerId,
  showParameters,
  className = "",
}: MobileStatusRailProps) {
  const { gameState, currentPlayer } = useMobileGame();
  const parameters = gameState.globalParameters;
  const showVenus = !!gameState.settings?.systems.includes("venus");
  const players = orderedPlayers(gameState);

  return (
    <div
      data-testid="mobile-status-rail"
      className={`fixed right-0 flex flex-col bg-[rgba(3,3,4,0.82)] border-l border-white/10 pointer-events-auto ${className}`}
      style={{
        zIndex: Z_INDEX.MOBILE_HUD,
        top: "calc(var(--hud-top-h) + var(--safe-top))",
        bottom: "calc(var(--hud-dock-h) + var(--safe-bottom))",
        width: "calc(var(--hud-rail-right-w) + var(--safe-right))",
        paddingRight: "var(--safe-right)",
      }}
    >
      {showParameters && parameters && (
        <div className="shrink-0 flex flex-col py-1 border-b border-white/15">
          <ParameterRow
            icon={ResourceTypeTemperature}
            label="Temperature"
            value={`${parameters.temperature}°C`}
            progress={
              (parameters.temperature - MIN_TEMPERATURE) / (MAX_TEMPERATURE - MIN_TEMPERATURE)
            }
          />
          <ParameterRow
            icon={ResourceTypeOxygen}
            label="Oxygen"
            value={`${parameters.oxygen}%`}
            progress={parameters.oxygen / MAX_OXYGEN}
          />
          <ParameterRow
            icon={ResourceTypeOceanTile}
            label="Oceans"
            value={`${parameters.oceans}/${parameters.maxOceans}`}
            progress={parameters.maxOceans > 0 ? parameters.oceans / parameters.maxOceans : 0}
          />
          {showVenus && (
            <ParameterRow
              icon={ResourceTypeVenus}
              label="Venus"
              value={`${parameters.venus}%`}
              progress={parameters.venus / MAX_VENUS}
            />
          )}
        </div>
      )}
      <div className="flex-1 min-h-0 flex flex-col overflow-y-auto overscroll-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
        {players.map((player) => (
          <PlayerChipButton
            key={player.id}
            player={player}
            isMe={player.id === currentPlayer?.id}
            isTurn={player.id === turnPlayerId}
          />
        ))}
      </div>
    </div>
  );
}
