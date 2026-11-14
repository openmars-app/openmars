import type { SceneSnapshot, SceneTile } from "./scene.generated";

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
