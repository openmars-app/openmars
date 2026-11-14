import type { GameDto, LogUpdatePayload } from "../types/generated/api-types";
import { GamePhaseInitApplyCorp, GamePhaseInitApplyPrelude } from "../types/generated/api-types";
import type { SceneCue, SceneSnapshot, SceneTile } from "./scene.generated";

function seed(value: string): number {
  let hash = 2166136261;
  for (const byte of new TextEncoder().encode(value)) {
    hash = Math.imul(hash ^ byte, 16777619) >>> 0;
  }
  return hash;
}

/** Only explicit, public live log records can create a presentation event. */
export function publicSceneCues(payload: LogUpdatePayload, game: GameDto): SceneCue[] {
  if (payload.isHistory) {
    return [];
  }
  const result: SceneCue[] = [];
  const players = [game.currentPlayer, ...game.otherPlayers];
  for (const log of payload.logs) {
    if (
      log.gameId !== game.id ||
      !Number.isSafeInteger(log.sequenceNumber) ||
      log.sequenceNumber < 0
    ) {
      continue;
    }
    const add = (suffix: string) => {
      const id = `log:${log.sequenceNumber}:${suffix}`;
      if (id.length <= 256) {
        result.push({ gameId: game.id, id, seed: seed(`${game.id}:${id}`), kind: "asteroid" });
      }
    };
    if (log.sourceType === "standard_project" && log.source.toLowerCase().includes("asteroid")) {
      add("project");
    }
    if (
      log.sourceType !== "card_play" ||
      game.currentPhase === GamePhaseInitApplyCorp ||
      game.currentPhase === GamePhaseInitApplyPrelude
    ) {
      continue;
    }
    const player = players.find((p) => p?.id === log.playerId);
    const ids = log.changes?.playerChanges?.[log.playerId]?.cardsPlayed ?? [];
    for (const id of ids) {
      const card = player?.playedCards?.find((card) => card.id === id);
      if (card?.name.toLowerCase().includes("asteroid")) {
        add(`card:${id}`);
      }
    }
  }
  return result;
}

type PlacementScene = Pick<SceneSnapshot, "gameId" | "mapId"> & {
  tiles: ReadonlyArray<Pick<SceneTile, "id" | "occupant">>;
};
export type PlacementSound = "construction" | "water";
const constructionTiles = new Set([
  "city-tile",
  "volcano-tile",
  "special-tile",
  "commercial-district-tile",
  "industrial-center-tile",
  "mining-tile",
  "mohole-tile",
  "restricted-tile",
]);

/** Restore the former board's placement sounds without replaying loaded tiles. */
export function placementSounds(
  previous: PlacementScene | null,
  next: PlacementScene | null,
  delivery: "snapshot" | "live",
): PlacementSound[] {
  if (
    delivery !== "live" ||
    !previous ||
    !next ||
    previous.gameId !== next.gameId ||
    previous.mapId !== next.mapId
  )
    return [];
  const occupants = new Map(previous.tiles.map((tile) => [tile.id, tile.occupant]));
  const sounds: PlacementSound[] = [];
  for (const tile of next.tiles) {
    if (!tile.occupant || occupants.get(tile.id) === tile.occupant) continue;
    if (tile.occupant === "ocean-tile") sounds.push("water");
    else if (constructionTiles.has(tile.occupant)) sounds.push("construction");
  }
  return sounds;
}
