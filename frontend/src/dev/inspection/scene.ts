import { createStructureShowcase } from "../../components/game/board/structureLayout";
import maps from "../../../../backend/assets/maps.json";
import type { GameDto, PlayerDto, TileDto } from "../../types/generated/api-types";
import { tileTypes, label, resolveTarget, type InspectionTarget } from "./catalog";
export type InspectionView = "gameplay" | "close";
export type InspectionState = "settled" | "placement";
export type ClimatePreset = "barren" | "growing" | "terraformed";
export interface SceneOptions {
  target: InspectionTarget;
  scene?: "comparison" | "structures" | "neighbors" | "destruction";
  seed: number;
  color: string;
  climate: ClimatePreset;
  state: InspectionState;
}
export const subjectCoordinate = { q: 1, r: 1, s: -2 };
export const subjectKey = "1,1,-2";
export const climates = {
  barren: { temperature: -30, oxygen: 0, oceans: 0 },
  growing: { temperature: 0, oxygen: 8, oceans: 4 },
  terraformed: { temperature: 8, oxygen: 14, oceans: 9 },
};
const emptyResources = { credits: 0, steel: 0, titanium: 0, plants: 0, energy: 0, heat: 0 };
export function createScene(options: SceneOptions, placed: boolean): GameDto {
  const currentPlayer: PlayerDto = {
    id: "inspector",
    name: "Inspector",
    color: options.color,
    playerType: "human",
    status: "active",
    permanentProduction: { ...emptyResources },
    production: { ...emptyResources },
    resources: { ...emptyResources },
    cardReceipts: [],
    resourceRemovalTargets: [],
    cards: [],
    terraformRating: 20,
    playedCards: [],
    passed: false,
    availableActions: 2,
    totalActions: 2,
    isConnected: true,
    isExited: false,
    effects: [],
    actions: [],
    standardProjects: [],
    resourceConversions: [],
    milestones: [],
    awards: [],
    demoReady: true,
    startingCards: [],
    pendingBehaviorResolutions: [],
    resourceStorage: {},
    paymentSubstitutes: [],
    generationalEvents: [],
    vpGranters: [],
    bonusTags: {},
    actionCosts: [],
  };
  const tiles: TileDto[] = maps[0].rows.flatMap((row, rowIndex) =>
    row.map((tile, column) => {
      const r = rowIndex - 4,
        q = Math.max(-4, -r - 4) + column;
      return {
        coordinates: { q, r, s: -q - r },
        type: tile.type,
        location: "mars",
        tags: [
          ...("tags" in tile ? (tile.tags ?? []) : []),
          ...("volcanic" in tile && tile.volcanic ? ["volcanic"] : []),
        ],
        bonuses: "bonuses" in tile ? (tile.bonuses ?? []) : [],
        displayName: "displayName" in tile ? tile.displayName : undefined,
      };
    }),
  );
  function occupy(
    q: number,
    r: number,
    type: string,
    name?: string,
    style?: InspectionTarget["style"],
  ) {
    const tile = tiles.find((t) => t.coordinates.q === q && t.coordinates.r === r)!;
    tile.bonuses = [];
    tile.displayName = undefined;
    tile.ownerId = type === "ocean" ? undefined : currentPlayer.id;
    tile.occupiedBy = {
      type: `${type}-tile`,
      tags: [],
      displayName: name,
      visual: { seed: options.seed + q * 31 + r * 97, structure: style },
    };
  }
  occupy(0, 1, "city", "Comparison City");
  occupy(0, 2, "greenery");
  occupy(1, 0, "ocean");
  if (placed) {
    if (options.target.tile === "land-claim") {
      tiles.find((t) => t.coordinates.q === 1 && t.coordinates.r === 1)!.reservedBy =
        currentPlayer.id;
    } else {
      occupy(1, 1, options.target.tile, options.target.name, options.target.style);
    }
  } else if (options.target.tile === "nuclear-zone") {
    occupy(1, 1, "city", "Replacement City");
  }
  if (options.scene === "neighbors") {
    occupy(2, 0, options.target.tile, options.target.name, options.target.style);
  }
  if (options.scene === "destruction") {
    if (placed) {
      occupy(1, 1, "nuclear-zone");
    } else {
      occupy(1, 1, options.target.tile, options.target.name, options.target.style);
    }
  }
  if (options.scene === "structures") {
    for (const tile of tiles) {
      tile.occupiedBy = undefined;
      tile.ownerId = undefined;
      tile.reservedBy = undefined;
    }
    const available = tiles.filter((tile) => tile.type !== "ocean" && !tile.displayName);
    for (const [index, city] of createStructureShowcase().entries()) {
      const q = (index % 4) - 2;
      const r = index < 4 ? -1 : 1;
      available.sort(
        (a, b) =>
          Math.abs(a.coordinates.q - q) +
          Math.abs(a.coordinates.r - r) * 3 -
          Math.abs(b.coordinates.q - q) -
          Math.abs(b.coordinates.r - r) * 3,
      );
      const tile = available.shift();
      if (tile) {
        occupy(tile.coordinates.q, tile.coordinates.r, "city", city.name, city.style);
        tile.occupiedBy!.visual!.seed = city.seed + options.seed;
      }
    }
  }
  if (options.scene === "structures") {
    for (const [id, q, r] of [
      ["123", 1, 1],
      ["085", -1, 2],
      ["P37", 2, -2],
    ] as const) {
      const target = resolveTarget(id);
      occupy(q, r, target.tile, target.name, target.style);
    }
  }
  return {
    id: `inspection-${options.seed}`,
    status: "active",
    currentPhase: "action",
    currentPlayer,
    otherPlayers: [],
    hostPlayerId: currentPlayer.id,
    viewingPlayerId: currentPlayer.id,
    currentTurn: currentPlayer.id,
    generation: 1,
    playerOrder: [currentPlayer.id],
    turnOrder: [currentPlayer.id],
    board: { tiles },
    globalParameters: { ...climates[options.climate], maxOceans: 9, venus: 0, bonuses: [] },
    settings: {
      gameModeId: "classic",
      gameModeParameters: {},
      features: [],
      systems: [],
      availableGameModes: [],
      maxPlayers: 1,
      mapId: "tharsis",
      developmentMode: true,
      demoGame: false,
      allowRandomBuy: false,
      hasClaudeOAuthToken: false,
      botSpendCapUsd: 0,
      botSpendUsd: 0,
      availablePlayerColors: [],
      availableMaps: [],
    },
    paymentConstants: { steelValue: 2, titaniumValue: 3 },
    milestones: [],
    awards: [],
    awardResults: [],
    spectators: [],
    chatMessages: [],
    isSpectator: true,
    isLastRound: false,
    placeableTileTypes: tileTypes.map((type) => ({ type, label: label(type), group: "Tiles" })),
  };
}
