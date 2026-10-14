import type {
  CardDto,
  ColonyDto,
  GameDto,
  OtherPlayerDto,
  PlayerDto,
  ResourcesDto,
} from "@/types/generated/api-types";

export const previewResources: ResourcesDto = {
  credits: 42,
  steel: 6,
  titanium: 4,
  plants: 8,
  energy: 3,
  heat: 12,
};
export const previewProduction = {
  credits: 4,
  steel: 2,
  titanium: 1,
  plants: 3,
  energy: 2,
  heat: 4,
};

export function createPreviewPlayer(cards: CardDto[] = []): PlayerDto {
  return {
    id: "preview-player",
    name: "Avery",
    playerType: "human",
    color: "#e76f51",
    status: "active",
    resources: { ...previewResources },
    production: { ...previewProduction },
    permanentProduction: { ...previewProduction },
    terraformRating: 27,
    corporation: cards.find((card) => card.type === "corporation"),
    cards: cards
      .filter((card) => card.type !== "corporation" && card.type !== "prelude")
      .slice(0, 10)
      .map((card) => ({ ...card, available: true, errors: [], effectiveCost: card.cost })),
    playedCards: cards.filter((card) => card.type === "active").slice(0, 4),
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
    demoReady: false,
    startingCards: [],
    pendingBehaviorResolutions: [],
    cardReceipts: [],
    resourceRemovalTargets: [],
    resourceStorage: {},
    paymentSubstitutes: [],
    generationalEvents: [],
    vpGranters: [],
    bonusTags: {},
    actionCosts: [
      {
        actionType: "card-buying",
        costs: [
          { resource: "credit", baseCost: 3, effectiveCost: 3, discount: 0, paymentCapacity: 42 },
        ],
      },
      {
        actionType: "colony-trade",
        costs: [
          { resource: "credit", baseCost: 9, effectiveCost: 9, discount: 0, paymentCapacity: 42 },
          { resource: "energy", baseCost: 3, effectiveCost: 3, discount: 0, paymentCapacity: 3 },
          { resource: "titanium", baseCost: 3, effectiveCost: 3, discount: 0, paymentCapacity: 4 },
        ],
      },
    ],
  };
}

export function createPreviewPlayers(cards: CardDto[] = []): OtherPlayerDto[] {
  return ["Morgan", "Robin", "Sam"].map((name, index) => ({
    ...createPreviewPlayer(cards),
    id: `preview-other-${index + 1}`,
    name,
    color: ["#4fa3df", "#a6ce39", "#b792e8"][index],
    handCardCount: 5 + index,
    isConnected: index !== 1,
    passed: index === 2,
    availableActions: 0,
    corporation: cards.filter((card) => card.type === "corporation")[index + 1],
  }));
}

export function createPreviewColonies(): ColonyDto[] {
  return [
    { id: "pluto", name: "Pluto", resource: "card-draw", color: "#9c8fd2" },
    { id: "miranda", name: "Miranda", resource: "animal", color: "#75b773" },
    { id: "luna", name: "Luna", resource: "credit", color: "#d6b572" },
  ].map(({ id, name, resource, color }, index) => ({
    id,
    name,
    location: id,
    active: true,
    steps: Array.from({ length: 7 }, (_, step) => ({
      outputs: [{ type: resource, amount: Math.max(1, step + index) }],
    })),
    colonyBonus: [{ type: resource, amount: 1 }],
    colonies: Array.from({ length: 3 }, () => ({ reward: [{ type: resource, amount: 2 }] })),
    markerPosition: 3,
    playerColonies: ["preview-other-1"],
    tradedThisGen: false,
    traderId: "",
    style: { color, icon: `colony-${id}` },
    tradeOptions: [
      { trackSteps: 0, markerPosition: 3, outputs: [{ type: resource, amount: 3 + index }] },
    ],
    tradeAvailable: true,
    buildAvailable: true,
    tradeErrors: [],
    buildErrors: [],
  }));
}

export const sampleColonies = createPreviewColonies();
export const samplePlayers = createPreviewPlayers();

export function createPreviewGame(cards: CardDto[] = []): GameDto {
  const currentPlayer = createPreviewPlayer(cards);
  const otherPlayers = createPreviewPlayers(cards);
  const playerOrder = [currentPlayer.id, ...otherPlayers.map((player) => player.id)];
  return {
    id: "preview-game",
    status: "active",
    hostPlayerId: currentPlayer.id,
    currentPhase: "action",
    viewingPlayerId: currentPlayer.id,
    currentPlayer,
    otherPlayers,
    currentTurn: currentPlayer.id,
    generation: 6,
    playerOrder,
    turnOrder: [...playerOrder],
    settings: {
      gameModeId: "classic",
      gameModeParameters: {},
      features: ["prelude"],
      systems: ["colonies", "venus"],
      availableGameModes: [
        {
          id: "classic",
          name: "Classic",
          summary: "Terraform Mars together while building your corporation.",
          scenario: "",
          rules: [
            {
              title: "Terraforming",
              text: "Raise temperature, oxygen and ocean coverage to complete the game.",
            },
          ],
          minPlayers: 1,
          maxPlayers: 5,
          parameters: [],
          features: [
            {
              id: "prelude",
              name: "Prelude",
              description: "Begin with two prelude cards.",
              experimental: false,
              default: true,
            },
            {
              id: "project-funding",
              name: "Project funding",
              description: "Fund shared projects for rewards.",
              experimental: false,
              default: false,
            },
          ],
          systems: ["colonies", "venus"],
          statuses: [],
          stateVisuals: {},
        },
      ],
      maxPlayers: 5,
      mapId: "tharsis",
      developmentMode: false,
      demoGame: false,
      allowRandomBuy: true,
      hasClaudeOAuthToken: false,
      botSpendCapUsd: 5,
      botSpendUsd: 0,
      availablePlayerColors: ["#e76f51", "#4fa3df", "#a6ce39", "#b792e8"],
      availableMaps: [
        {
          id: "tharsis",
          name: "Tharsis",
          description: "The original map of Mars, featuring volcanic regions and ocean basins.",
          tiles: [
            { q: 0, r: 0, s: 0, type: "land" },
            { q: 1, r: -1, s: 0, type: "ocean-space" },
            { q: 0, r: -1, s: 1, type: "land", volcanic: true },
          ],
        },
        {
          id: "hellas",
          name: "Hellas",
          description: "Explore the southern hemisphere of Mars.",
          tiles: [],
        },
      ],
    },
    globalParameters: {
      temperature: -12,
      oxygen: 7,
      oceans: 4,
      maxOceans: 9,
      venus: 12,
      bonuses: [],
    },
    board: { tiles: [] },
    paymentConstants: { steelValue: 2, titaniumValue: 3 },
    milestones: [],
    awards: [],
    awardResults: [],
    placeableTileTypes: [],
    spectators: [],
    chatMessages: [],
    isSpectator: false,
    isLastRound: false,
    colonies: createPreviewColonies(),
    tradeFleets: { [currentPlayer.id]: { total: 1, available: 1 } },
    projectFunding: [],
  };
}
