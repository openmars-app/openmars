import type { GameDto } from "@/types/generated/api-types.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import GameSetupControls from "./GameSetupControls.tsx";

interface LobbyMapInfoPanelProps {
  game: GameDto;
  playerId: string;
  layout?: "stacked" | "split";
  section?: "all" | "map" | "settings";
}

export default function LobbyMapInfoPanel({
  game,
  playerId,
  layout,
  section,
}: LobbyMapInfoPanelProps) {
  return (
    <GameSetupControls
      layout={layout}
      section={section}
      maps={game.settings.availableMaps ?? []}
      mapId={game.settings.mapId}
      modes={game.settings.availableGameModes}
      gameModeId={game.settings.gameModeId}
      gameModeParameters={game.settings.gameModeParameters}
      features={game.settings.features}
      maxPlayers={game.settings.maxPlayers}
      onChange={
        game.hostPlayerId === playerId
          ? (patch) => {
              void globalWebSocketManager.updateGameSettings(patch);
            }
          : undefined
      }
    />
  );
}
