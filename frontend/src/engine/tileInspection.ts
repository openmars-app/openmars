import type { GameDto } from "../types/generated/api-types";

export function describeTile(game: GameDto | null, id: string) {
  const tile = game?.board.tiles.find(
    (candidate) =>
      `${candidate.coordinates.q},${candidate.coordinates.r},${candidate.coordinates.s}` === id,
  );
  if (!game || !tile) {
    return null;
  }
  const players = [game.currentPlayer, ...game.otherPlayers];
  const owner = players.find((player) => player.id === tile.ownerId);
  return {
    tileType: tile.occupiedBy?.type.replace(/-tile$/, "") ?? "empty",
    displayName: tile.displayName,
    placedName: tile.occupiedBy?.displayName,
    ownerName: owner?.name,
    ownerColor: owner?.color,
    reservedByName: players.find((player) => player.id === tile.reservedBy)?.name,
    isOceanSpace: tile.type === "ocean-space",
    isVolcanic: (tile.tags ?? []).includes("volcanic"),
    bonuses: Object.fromEntries((tile.bonuses ?? []).map((bonus) => [bonus.type, bonus.amount])),
  };
}
