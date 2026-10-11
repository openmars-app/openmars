import type {
  CardDto,
  GameDto,
  GameHistoryEntryDto,
  GameHistoryPlayerDto,
  OtherPlayerDto,
  PlayerDto,
} from "../types/generated/api-types";

export function historyEntryToGameDto(
  entry: GameHistoryEntryDto,
  liveGame: GameDto,
  cardLookup?: Map<string, CardDto>,
): GameDto {
  const lookup = cardLookup ?? buildCardLookup(liveGame);
  const viewingId = liveGame.viewingPlayerId;

  const historyCurrentPlayer = entry.players[viewingId];
  const currentPlayer = historyCurrentPlayer
    ? historyPlayerToPlayerDto(historyCurrentPlayer, lookup)
    : liveGame.currentPlayer;

  const otherPlayers: OtherPlayerDto[] = Object.values(entry.players)
    .filter((hp) => hp.id !== viewingId)
    .map((hp) => historyPlayerToOtherPlayerDto(hp, lookup));

  return {
    ...liveGame,
    board: entry.board,
    settings: entry.settings,
    modeRuntime: entry.modeRuntime,
    generation: entry.generation,
    currentPhase: entry.phase,
    currentPlayer,
    otherPlayers: otherPlayers.length > 0 ? otherPlayers : liveGame.otherPlayers,
    globalParameters: {
      temperature: entry.temperature,
      oxygen: entry.oxygen,
      oceans: entry.oceans,
      maxOceans: liveGame.globalParameters.maxOceans,
      venus: entry.venus,
      bonuses: liveGame.globalParameters.bonuses,
    },
    milestones: liveGame.milestones.map((m) => {
      const claimed = entry.milestones.find((cm) => cm.type === m.type);
      return { ...m, claimedBy: claimed?.playerId };
    }),
    awards: liveGame.awards.map((a) => {
      const funded = entry.awards.find((fa) => fa.type === a.type);
      return { ...a, fundedBy: funded?.playerId };
    }),
  };
}

export function buildCardLookup(liveGame: GameDto): Map<string, CardDto> {
  const lookup = new Map<string, CardDto>();
  const allPlayers = [
    ...(liveGame.currentPlayer ? [liveGame.currentPlayer] : []),
    ...(liveGame.otherPlayers ?? []),
  ];
  for (const player of allPlayers) {
    if (player.corporation) {
      lookup.set(player.corporation.id, player.corporation);
    }
    for (const card of player.playedCards ?? []) {
      lookup.set(card.id, card);
    }
    if ("cards" in player) {
      for (const card of player.cards ?? []) {
        lookup.set(card.id, card);
      }
    }
  }
  return lookup;
}

function historyCard(
  id: string,
  player: GameHistoryPlayerDto,
  lookup: Map<string, CardDto>,
): CardDto | undefined {
  const card = lookup.get(id);
  if (!card) return undefined;
  return {
    ...card,
    behaviors: card.behaviors?.map((behavior) => ({
      ...behavior,
      dependencyStatus: behavior.dependency
        ? player.dependencyStatuses[`${id}:${behavior.dependency}`]
        : undefined,
    })),
  };
}

function historyPlayerToOtherPlayerDto(
  historyPlayer: GameHistoryPlayerDto,
  cardLookup: Map<string, CardDto>,
): OtherPlayerDto {
  const playedCards: CardDto[] = (historyPlayer.playedCardIds ?? [])
    .map((id) => historyCard(id, historyPlayer, cardLookup))
    .filter((c): c is CardDto => c != null);

  const corporation = historyPlayer.corporationId
    ? (historyCard(historyPlayer.corporationId, historyPlayer, cardLookup) ?? undefined)
    : undefined;

  return {
    id: historyPlayer.id,
    name: historyPlayer.name,
    color: historyPlayer.color,
    playerType: "human",
    status: "playing" as OtherPlayerDto["status"],
    terraformRating: historyPlayer.terraformRating,
    resources: {
      credits: historyPlayer.credits,
      steel: historyPlayer.steel,
      titanium: historyPlayer.titanium,
      plants: historyPlayer.plants,
      energy: historyPlayer.energy,
      heat: historyPlayer.heat,
    },
    production: historyPlayer.production,
    permanentProduction: historyPlayer.permanentProduction,
    corporation,
    handCardCount: historyPlayer.handCardIds?.length ?? 0,
    playedCards,
    passed: false,
    availableActions: 0,
    totalActions: 0,
    isConnected: true,
    isExited: false,
    effects: [],
    actions: [],
    resourceStorage: historyPlayer.resourceStorage ?? {},
    paymentSubstitutes: [],
    vpGranters: [],
    bonusTags: {},
    demoReady: false,
  };
}

export function historyPlayerToPlayerDto(
  historyPlayer: GameHistoryPlayerDto,
  cardLookup: Map<string, CardDto>,
): PlayerDto {
  const playedCards: CardDto[] = (historyPlayer.playedCardIds ?? [])
    .map((id) => historyCard(id, historyPlayer, cardLookup))
    .filter((c): c is CardDto => c != null);

  const handCards = (historyPlayer.handCardIds ?? [])
    .map((id) => historyCard(id, historyPlayer, cardLookup))
    .filter((c): c is CardDto => c != null)
    .map((card) => ({
      ...card,
      available: false,
      errors: [],
      effectiveCost: card.cost,
    }));

  const corporation = historyPlayer.corporationId
    ? (historyCard(historyPlayer.corporationId, historyPlayer, cardLookup) ?? null)
    : null;

  return {
    resourceRemovalTargets: [],
    id: historyPlayer.id,
    name: historyPlayer.name,
    color: historyPlayer.color,
    playerType: "human",
    status: "playing" as PlayerDto["status"],
    terraformRating: historyPlayer.terraformRating,
    resources: {
      credits: historyPlayer.credits,
      steel: historyPlayer.steel,
      titanium: historyPlayer.titanium,
      plants: historyPlayer.plants,
      energy: historyPlayer.energy,
      heat: historyPlayer.heat,
    },
    production: historyPlayer.production,
    permanentProduction: historyPlayer.permanentProduction,
    corporation: corporation ?? undefined,
    cards: handCards,
    playedCards,
    passed: false,
    availableActions: 0,
    totalActions: 0,
    isConnected: true,
    isExited: false,
    effects: [],
    actions: [],
    standardProjects: [],
    resourceConversions: [],
    milestones: [],
    awards: [],
    startingCards: [],
    resourceStorage: historyPlayer.resourceStorage ?? {},
    paymentSubstitutes: [],
    generationalEvents: [],
    vpGranters: [],
    bonusTags: {},
    actionCosts: [],
    demoReady: false,
    pendingBehaviorResolutions: [],
    cardReceipts: [],
  };
}
