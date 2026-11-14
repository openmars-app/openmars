import type { AppPhase } from "../stores/appPhaseStore";
import { isInGameWorld } from "../stores/appPhaseStore";
import type { GameDto, StructureStyleRequestDto } from "../types/generated/api-types";
import type { BodyId, SceneEvent, SceneSnapshot, StructureStyleRequest } from "./scene.generated";

const PLACEMENT_DESTINATIONS: Record<string, BodyId> = {
  mars: "mars",
  phobos: "mars",
  mercury: "mercury",
  venus: "venus",
  earth: "earth",
  moon: "earth",
  luna: "earth",
  ceres: "ceres",
  jupiter: "jupiter",
  ganymede: "jupiter",
  saturn: "saturn",
  uranus: "uranus",
  neptune: "neptune",
};

/** A single public placement destination may guide navigation, never legality. */
export function placementDestination(scene: SceneSnapshot | null): BodyId | null {
  if (!scene || !scene.selectableTiles.length) {
    return null;
  }
  const available = new Set(scene.selectableTiles);
  const destinations = new Set(
    scene.tiles
      .filter((tile) => available.has(tile.id))
      .map((tile) => PLACEMENT_DESTINATIONS[tile.location] ?? "mars"),
  );
  return destinations.size === 1 ? [...destinations][0] : null;
}

function structureRequest(value: StructureStyleRequestDto): StructureStyleRequest {
  return {
    plan: value.plan,
    density: value.density,
    heights: value.heights,
    ground: value.ground,
    exposure: value.exposure,
    perimeter: value.perimeter,
    cover: value.cover,
    landmark: value.landmark,
    connections: value.connections,
    landscaping: value.landscaping,
    lighting: value.lighting,
    perimeterRoad: value.perimeterRoad,
    entranceMin: value.entranceMin ?? 0,
    entranceMax: value.entranceMax ?? 0,
    details: [...(value.details ?? [])],
  };
}
/** Copy only public presentation data across the WASM boundary. */
export function toSceneSnapshot(
  game: GameDto | null,
  selectedTile: string | null,
): SceneSnapshot | null {
  if (!game) {
    return null;
  }
  const tiles = game.board.tiles.map((tile) => ({
    id: `${tile.coordinates.q},${tile.coordinates.r},${tile.coordinates.s}`,
    ...tile.coordinates,
    location: tile.location,
    terrain: tile.type,
    displayName: tile.displayName ?? null,
    tags: [...(tile.tags ?? [])],
    bonuses: (tile.bonuses ?? []).map(({ type, amount }) => ({ type, amount })),
    occupant: tile.occupiedBy?.type ?? null,
    occupantDisplayName: tile.occupiedBy?.displayName ?? null,
    occupantTags: [...(tile.occupiedBy?.tags ?? [])],
    visual: tile.occupiedBy?.visual
      ? {
          seed: tile.occupiedBy.visual.seed,
          structure: tile.occupiedBy.visual.structure
            ? structureRequest(tile.occupiedBy.visual.structure)
            : null,
        }
      : null,
    ownerId: tile.ownerId ?? null,
    reservedBy: tile.reservedBy ?? null,
  }));
  const ids = new Set(tiles.map((tile) => tile.id));
  const players = [game.currentPlayer, ...game.otherPlayers].map(({ id, color }) => ({
    id,
    color,
  }));
  const playerIds = new Set(players.map((player) => player.id));
  const station = game.projectFunding?.find((project) => project.id === "pf_orbital_station");
  return {
    gameId: game.id,
    mapId: game.settings.mapId,
    generation: game.generation,
    temperature: game.globalParameters.temperature,
    oxygen: game.globalParameters.oxygen,
    oceans: game.globalParameters.oceans,
    maxOceans: game.globalParameters.maxOceans,
    venus: game.globalParameters.venus,
    tiles,
    players,
    offworld: {
      fleetsEnabled: (game.settings.systems ?? []).includes("colonies"),
      colonies: (game.colonies ?? []).map((colony) => ({
        id: colony.id,
        location: colony.location,
        tradedThisGen: colony.tradedThisGen,
        traderId: colony.traderId || null,
      })),
      fleets: Object.entries(game.tradeFleets ?? {})
        .filter(([id]) => playerIds.has(id))
        .map(([playerId, fleet]) => ({ playerId, total: fleet.total, available: fleet.available })),
      station: station
        ? {
            name: station.name,
            filledSeats: (station.seatOwners ?? []).length,
            totalSeats: (station.seats ?? []).length,
            completed: station.isCompleted,
          }
        : null,
    },
    selectedTile: selectedTile && ids.has(selectedTile) ? selectedTile : null,
    selectableTiles: game.isSpectator
      ? []
      : (game.currentPlayer.pendingTileSelection?.availableHexes ?? []).filter((id) => ids.has(id)),
  };
}

/** Ignore stale scenes and never turn a render event into an optimistic board mutation. */
export function routeSceneEvent(
  event: SceneEvent,
  game: GameDto | null,
  handlers: {
    select: (id: string) => void;
    inspect: (id: string, x: number, y: number) => void;
    hover: (id: string | null, x: number, y: number) => void;
    focus: (body: BodyId) => void;
  },
): void {
  if (!game || event.gameId !== game.id) {
    return;
  }
  if (event.type === "body-selected") {
    handlers.focus(event.body);
    return;
  }
  if (event.type === "body-hovered") {
    handlers.hover(null, 0, 0);
    return;
  }
  const snapshot = toSceneSnapshot(game, null)!;
  if (event.type === "tile-hovered" && event.tileId === null) {
    handlers.hover(null, event.x, event.y);
    return;
  }
  if (!snapshot.tiles.some((tile) => tile.id === event.tileId)) {
    return;
  }
  if (event.type === "tile-selected") {
    if (snapshot.selectableTiles.includes(event.tileId)) {
      handlers.select(event.tileId);
    }
  } else if (event.type === "tile-hovered") {
    handlers.hover(event.tileId, event.x, event.y);
  } else {
    handlers.inspect(event.tileId, event.x, event.y);
  }
}

/** The same explicit phase projection as the old animateEntrance/startHidden props. */
export function worldEntryForPhase(phase: AppPhase): import("./scene.generated").WorldEntry {
  if (phase.kind === "loading") {
    return { tiles: "hidden", lighting: "dark" };
  }
  if (phase.kind === "fadeOutLobby") {
    return { tiles: "hidden", lighting: "sunrise" };
  }
  return isInGameWorld(phase)
    ? { tiles: "reveal", lighting: "sunrise" }
    : { tiles: "settled", lighting: "steady" };
}
