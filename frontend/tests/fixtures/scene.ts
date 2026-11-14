import type { GameDto } from "../../src/types/generated/api-types";

/** Public DTO fixture; unrelated game UI fields are not needed by the renderer. */
export function sceneGame(id = "bridge-game"): GameDto {
  const tiles = [];
  for (let q = -4; q <= 4; q++) {
    for (let r = -4; r <= 4; r++) {
      const s = -q - r;
      if (Math.abs(s) > 4) continue;
      tiles.push({
        coordinates: { q, r, s },
        location: "mars",
        type: "land",
        tags: [],
        bonuses: [],
      });
    }
  }
  return {
    id,
    status: "active",
    currentPhase: "action",
    settings: { mapId: "tharsis", systems: ["colonies"] },
    generation: 1,
    globalParameters: { temperature: -30, oxygen: 0, oceans: 0, maxOceans: 9, venus: 0 },
    currentPlayer: {
      id: "player",
      name: "Alice",
      color: "#ffaa00",
      cards: ["private-card"],
      pendingTileSelection: { tileType: "greenery", availableHexes: ["0,0,0"] },
    },
    otherPlayers: [],
    isSpectator: false,
    board: { tiles },
  } as unknown as GameDto;
}

/** Authored public style inputs exercise maps, glass, frost, parks and factories. */
export function citySceneGame(): GameDto {
  const game = sceneGame("city-scenes");
  const styles: NonNullable<
    NonNullable<GameDto["board"]["tiles"][number]["occupiedBy"]>["visual"]
  >[] = [
    {
      seed: 28881,
      structure: {
        plan: "grid",
        density: "dense",
        heights: "tall",
        cover: "flat-glass",
        connections: "skybridges",
        lighting: "bright",
        details: ["pipes", "gatehouses"],
      },
    },
    {
      seed: 2907,
      structure: {
        plan: "clustered",
        landmark: "factory",
        details: ["smokestacks", "tanks", "pipes"],
        density: "dense",
      },
    },
    {
      seed: 73,
      structure: {
        plan: "radial",
        landmark: "observatory",
        cover: "dome",
        landscaping: "parks",
        perimeterRoad: "ring",
      },
    },
    {
      seed: 991,
      structure: {
        plan: "courtyard",
        ground: "recessed",
        exposure: "mostly-buried",
        landmark: "headquarters",
        landscaping: "lush",
      },
    },
  ];
  const coordinates = [
    [0, 0],
    [-1, 0],
    [1, 0],
    [0, 1],
  ];
  for (let i = 0; i < styles.length; i++) {
    const [q, r] = coordinates[i];
    const tile = game.board.tiles.find(
      (tile) => tile.coordinates.q === q && tile.coordinates.r === r,
    )!;
    tile.occupiedBy = {
      type: "city-tile",
      tags: [],
      displayName: ["Bright towers", "Factory", "Observatory", "Courtyard"][i],
      visual: styles[i],
    };
    tile.ownerId = "player";
  }
  return game;
}

/** Only preserves/ecological zones host birds; ordinary greenery is a control. */
export function faunaSceneGame(): GameDto {
  const game = sceneGame("fauna-scenes");
  game.globalParameters.temperature = 8;
  game.globalParameters.oxygen = 14;
  for (const [q, kind] of [
    [0, "ecological-zone-tile"],
    [1, "natural-preserve-tile"],
    [-1, "greenery-tile"],
  ] as const) {
    const tile = game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === 0)!;
    tile.occupiedBy = { type: kind, tags: [], displayName: kind, visual: { seed: 37 } };
    tile.ownerId = "player";
  }
  return game;
}

export function worldTreeSceneGame(): GameDto {
  const game = sceneGame("world-tree-scenes");
  game.globalParameters.temperature = 8;
  game.globalParameters.oxygen = 14;
  for (const [q, seed] of [
    [0, 37],
    [1, 999],
  ] as const) {
    game.board.tiles.find(
      (tile) => tile.coordinates.q === q && tile.coordinates.r === 0,
    )!.occupiedBy = {
      type: "world-tree-tile",
      displayName: "World tree",
      tags: [],
      visual: { seed },
    };
  }
  return game;
}

/** Seeded workings and both forms of publicly reserved land. */
export function worksiteSceneGame(): GameDto {
  const game = sceneGame("worksite-scenes");
  game.globalParameters.temperature = 8;
  game.globalParameters.oxygen = 14;
  game.otherPlayers = [
    { id: "blue", name: "Bob", color: "#33aaff", playedCards: [] },
  ] as unknown as GameDto["otherPlayers"];
  for (const [q, kind, seed] of [
    [0, "mining-tile", 37],
    [1, "mining-tile", 999],
    [-1, "restricted-tile", 77],
  ] as const) {
    const tile = game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === 0)!;
    tile.occupiedBy = { type: kind, displayName: kind, tags: [], visual: { seed } };
    tile.ownerId = "player";
  }
  game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === 1)!.reservedBy = "blue";
  return game;
}

/** Seeded custom surfaces; no special appearance is inferred from display text. */
export function specialSceneGame(): GameDto {
  const game = sceneGame("special-surfaces");
  delete game.currentPlayer!.pendingTileSelection;
  for (const [q, r, type, seed] of [
    [0, 0, "volcano-tile", 37],
    [1, 0, "volcano-tile", 999],
    [-1, 0, "mohole-tile", 77],
    [0, 1, "mohole-tile", 123],
  ] as const) {
    const tile = game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === r)!;
    tile.ownerId = "player";
    tile.occupiedBy = {
      type,
      tags: [],
      visual: { seed },
    };
  }
  return game;
}

/** Ongoing source emissions plus a real authored factory, without initial dust. */
export function particleSceneGame(): GameDto {
  const game = specialSceneGame();
  game.id = "particle-scenes";
  game.board.tiles.find((t) => t.coordinates.q === -1 && t.coordinates.r === 1)!.occupiedBy = {
    type: "mining-tile",
    tags: [],
    visual: { seed: 73 },
  };
  game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === -1)!.occupiedBy = {
    type: "city-tile",
    tags: [],
    visual: {
      seed: 37,
      structure: {
        plan: "grid",
        landmark: "factory",
        cover: "none",
        details: ["smokestacks", "ground-exhausts", "tanks", "pipes"],
        density: "dense",
      },
    },
  };
  return game;
}

/** Quiet persisted craters use the game/coordinate seed, never visual.seed. */
export function nuclearSceneGame(): GameDto {
  const game = sceneGame("nuclear-scenes");
  delete game.currentPlayer!.pendingTileSelection;
  for (const q of [0, 2]) {
    const tile = game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === 0)!;
    tile.occupiedBy = { type: "nuclear-zone-tile", tags: [], visual: { seed: 37 } };
  }
  return game;
}

/** Public city plus two trees exercise shared full, reduced and quiet ownership. */
export function nuclearTransitionGame(): GameDto {
  const game = worldTreeSceneGame();
  game.id = "nuclear-transitions";
  delete game.currentPlayer!.pendingTileSelection;
  game.board.tiles.find((t) => t.coordinates.q === -1 && t.coordinates.r === 0)!.occupiedBy = {
    type: "city-tile",
    tags: [],
    visual: { seed: 37, structure: { plan: "grid", density: "sparse" } },
  };
  return game;
}

/** Full original actor groups and one forest, plus a surviving control mine. */
export function nuclearCollapseGame(): GameDto {
  const game = worksiteSceneGame();
  game.id = "nuclear-collapse";
  delete game.currentPlayer!.pendingTileSelection;
  for (const [q, r, type, seed] of [
    [0, -1, "mohole-tile", 77],
    [-1, 1, "volcano-tile", 37],
    [1, -1, "world-tree-tile", 37],
    [-1, -1, "greenery-tile", 37],
  ] as const) {
    game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === r)!.occupiedBy = {
      type,
      tags: [],
      visual: { seed },
    };
  }
  return game;
}

/** One quiet public board exercises every haze material and the excluded effects. */
export function atmosphereSceneGame(): GameDto {
  const game = citySceneGame();
  game.id = "atmosphere-climate";
  game.isSpectator = true;
  for (const [q, r, type, seed] of [
    [0, 1, "world-tree-tile", 37],
    [1, 0, "mohole-tile", 77],
    [1, -1, "volcano-tile", 37],
    [-1, 1, "mining-tile", 73],
    [0, 2, "ecological-zone-tile", 37],
    [-1, 2, "greenery-tile", 37],
    [2, -1, "nuclear-zone-tile", 37],
  ] as const) {
    const tile = game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === r)!;
    tile.occupiedBy = { type, tags: [], visual: { seed } };
    tile.ownerId = "player";
  }
  game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === -1)!.reservedBy =
    "player";
  return game;
}

/** Five authored offworld locations use their actual absolute cube offsets. */
export function offworldSceneGame(): GameDto {
  const game = sceneGame("offworld-grids");
  const centers: string[] = [];
  for (const [location, offset] of [
    ["venus", 100],
    ["ganymede", 200],
    ["moon", 300],
    ["mercury", 400],
    ["phobos", 500],
  ] as const) {
    centers.push(`${offset},0,${-offset}`);
    for (const [q, r] of [
      [0, 0],
      [1, 0],
      [0, 1],
      [-1, 1],
      [-1, 0],
      [0, -1],
      [1, -1],
    ]) {
      game.board.tiles.push({
        coordinates: { q: offset + q, r, s: -offset - q - r },
        location,
        type: "land",
        tags: [],
        bonuses: [],
        displayName: `${location} ${q},${r}`,
      });
    }
  }
  game.currentPlayer!.pendingTileSelection!.availableHexes = centers;
  return game;
}

/** Original board names, credit numerals and repeated placement bonus icons. */
export function labelSceneGame(): GameDto {
  const game = sceneGame("native-labels");
  game.currentPlayer.pendingTileSelection = undefined;
  const tile = (q: number, r: number) =>
    game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === r)!;
  tile(0, 0).displayName = "Arsia Mons";
  tile(0, 0).bonuses = [
    { type: "credit", amount: 3 },
    { type: "steel", amount: 2 },
  ];
  tile(-1, 0).bonuses = [
    { type: "plant", amount: 2 },
    { type: "card-draw", amount: 1 },
  ];
  tile(1, 0).displayName = "New Ares";
  tile(1, 0).occupiedBy = {
    type: "city-tile",
    displayName: "Different occupant name",
    visual: { seed: 123 },
  } as NonNullable<GameDto["board"]["tiles"][number]["occupiedBy"]>;
  tile(1, 0).bonuses = [{ type: "plant", amount: 2 }];
  tile(-1, 1).occupiedBy = { type: "commercial-district-tile" } as NonNullable<
    GameDto["board"]["tiles"][number]["occupiedBy"]
  >;
  tile(0, -1).displayName = "Arsia Mons Research Outpost";
  tile(0, -1).bonuses = [
    { type: "titanium", amount: 1 },
    { type: "energy-production", amount: 1 },
  ];
  return game;
}

/** Connected lakes include the enclosed shore pockets that retain the last ice. */
export function oceanSceneGame(): GameDto {
  const game = sceneGame("ocean-climate");
  game.currentPlayer.pendingTileSelection = undefined;
  game.globalParameters.oceans = 8;
  for (const [q, r] of [
    [0, 0],
    [1, 0],
    [0, 1],
    [-1, 1],
    [-1, 0],
    [0, -1],
    [1, -1],
    [2, 0],
  ]) {
    const tile = game.board.tiles.find((t) => t.coordinates.q === q && t.coordinates.r === r)!;
    tile.occupiedBy = { type: "ocean-tile", tags: [] };
  }
  return game;
}
