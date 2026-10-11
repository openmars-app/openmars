import cards from "../../../../backend/assets/cards.json";
import type { StructureStyleRequestDto } from "../../types/generated/api-types";

export const tileTypes = [
  "city",
  "greenery",
  "ocean",
  "commercial-district",
  "industrial-center",
  "mining",
  "natural-preserve",
  "ecological-zone",
  "mohole",
  "nuclear-zone",
  "restricted",
  "volcano",
  "world-tree",
  "land-claim",
] as const;
export type InspectionTile = (typeof tileTypes)[number];
export interface InspectionTarget {
  tile: InspectionTile;
  card?: string;
  name: string;
  style?: StructureStyleRequestDto;
}
interface Placement {
  type?: string;
  tileType?: string;
  location?: string;
  tileRestrictions?: { location?: string; boardTags?: string[] };
}
const placementTypes: Record<string, string> = {
  "city-placement": "city",
  "greenery-placement": "greenery",
  "ocean-placement": "ocean",
  "volcano-placement": "volcano",
  "land-claim": "land-claim",
};
const offMarsTags = new Set([
  "phobos-space-haven",
  "ganymede-colony",
  "dawn-city",
  "maxwell-base",
  "stratopolis",
  "luna-metropolis",
]);
function placements(value: unknown): Placement[] {
  if (Array.isArray(value)) {
    return value.flatMap(placements);
  }
  if (!value || typeof value !== "object") {
    return [];
  }
  const object = value as Record<string, unknown>;
  const found =
    typeof object.type === "string" &&
    (placementTypes[object.type] || ["tile-placement", "tile-replacement"].includes(object.type))
      ? [object as Placement]
      : [];
  return [...found, ...Object.values(object).flatMap(placements)];
}
export function label(tile: string) {
  return tile
    .split("-")
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join(" ");
}
export const cardTargets: InspectionTarget[] = cards.flatMap((card) => {
  const types = new Set(
    placements(card.behaviors)
      .filter(
        (p) =>
          (!p.location || p.location === "mars") &&
          (!p.tileRestrictions?.location || p.tileRestrictions.location === "mars") &&
          !p.tileRestrictions?.boardTags?.some((tag) => offMarsTags.has(tag)),
      )
      .map((p) => p.tileType ?? placementTypes[p.type ?? ""])
      .filter((tile): tile is InspectionTile => tileTypes.includes(tile as InspectionTile)),
  );
  return [...types].map((tile) => ({
    tile,
    card: card.id,
    name: card.name,
    style: "style" in card ? (card.style?.tile as StructureStyleRequestDto) : undefined,
  }));
});
export function resolveTarget(card?: string | null, tile?: string | null): InspectionTarget {
  if (card) {
    const choices = cardTargets.filter(
      (target) => target.card === card && (!tile || target.tile === tile),
    );
    if (choices.length === 1) {
      return choices[0];
    }
    if (choices.length > 1) {
      throw new Error(
        `Card ${card} has multiple placements. Select --tile ${choices.map((t) => t.tile).join(" or ")}.`,
      );
    }
    throw new Error(
      `Card ${card} has no supported Mars placement${tile ? ` for ${tile}` : ""}. Use --list to see supported targets.`,
    );
  }
  if (!tile || !tileTypes.includes(tile as InspectionTile)) {
    throw new Error("Select a supported --card ID or --tile TYPE. Use --list to see targets.");
  }
  if (tile !== "city") {
    const styled = cardTargets.filter((target) => target.tile === tile && target.style);
    if (styled.length === 1) {
      return { tile: tile as InspectionTile, name: styled[0].name, style: styled[0].style };
    }
  }
  return { tile: tile as InspectionTile, name: label(tile) };
}
