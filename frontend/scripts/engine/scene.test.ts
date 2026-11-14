import { describe, expect, test } from "bun:test";
import type { ColonyDto, GameDto, ProjectFundingDto } from "../../src/types/generated/api-types";
import {
  placementDestination,
  routeSceneEvent,
  toSceneSnapshot,
  worldEntryForPhase,
} from "../../src/engine/scene";
import { describeTile } from "../../src/engine/tileInspection";

function game(): GameDto {
  return {
    id: "game",
    settings: { mapId: "tharsis", systems: ["colonies"] },
    generation: 1,
    globalParameters: { temperature: -30, oxygen: 0, oceans: 0, maxOceans: 9, venus: 0 },
    currentPlayer: {
      id: "player",
      color: "#ffaa00",
      cards: ["private-card"],
      pendingTileSelection: { availableHexes: ["0,0,0", "missing"] },
    },
    otherPlayers: [],
    isSpectator: false,
    board: {
      tiles: [{ coordinates: { q: 0, r: 0, s: 0 }, location: "mars", type: "land", bonuses: [] }],
    },
  } as unknown as GameDto;
}

describe("rendering scene bridge", () => {
  test("entry choreography comes from the explicit app phase", () => {
    expect(worldEntryForPhase({ kind: "loading", gameId: "game" })).toEqual({
      tiles: "hidden",
      lighting: "dark",
    });
    expect(worldEntryForPhase({ kind: "fadeOutLobby", gameId: "game" })).toEqual({
      tiles: "hidden",
      lighting: "sunrise",
    });
    for (const kind of ["marsRevealed", "showcase", "animateUI", "playing", "completed"] as const) {
      expect(worldEntryForPhase({ kind, gameId: "game" })).toEqual({
        tiles: "reveal",
        lighting: "sunrise",
      });
    }
    expect(worldEntryForPhase({ kind: "menu", route: "landing" })).toEqual({
      tiles: "settled",
      lighting: "steady",
    });
    expect(worldEntryForPhase({ kind: "lobby", gameId: "game" })).toEqual({
      tiles: "settled",
      lighting: "steady",
    });
  });
  test("copies only public offworld visuals and retains authored trade locations", () => {
    const value = game();
    value.colonies = [
      {
        id: "titan",
        location: "venus",
        tradedThisGen: true,
        traderId: "player",
        tradeOptions: ["do-not-copy"],
      },
    ] as unknown as ColonyDto[];
    value.tradeFleets = { player: { total: 3, available: 1 }, unknown: { total: 1, available: 1 } };
    value.projectFunding = [
      {
        id: "pf_solar_forge",
        name: "Other UI project",
        seats: [],
        seatOwners: [],
        isCompleted: true,
      },
      {
        id: "pf_orbital_station",
        name: "Orbital station",
        seats: [{}, {}, {}],
        seatOwners: [{ playerId: "player" }],
        isCompleted: false,
        description: "do-not-copy",
      },
    ] as unknown as ProjectFundingDto[];
    const scene = toSceneSnapshot(value, null)!;
    expect(scene.offworld).toEqual({
      fleetsEnabled: true,
      colonies: [{ id: "titan", location: "venus", tradedThisGen: true, traderId: "player" }],
      fleets: [{ playerId: "player", total: 3, available: 1 }],
      station: { name: "Orbital station", filledSeats: 1, totalSeats: 3, completed: false },
    });
    expect(JSON.stringify(scene)).not.toContain("do-not-copy");
    value.settings.systems = [];
    expect(toSceneSnapshot(value, null)!.offworld.fleetsEnabled).toBe(false);
    expect(toSceneSnapshot(value, null)!.offworld.fleets).toEqual(scene.offworld.fleets);
    value.settings.systems = ["colonies"];
    scene.offworld.colonies[0].traderId = "changed";
    scene.offworld.fleets[0].total = 9;
    expect(value.colonies[0].traderId).toBe("player");
    expect(value.tradeFleets.player.total).toBe(3);
    Reflect.set(value.projectFunding[1], "seats", null);
    Reflect.set(value.projectFunding[1], "seatOwners", null);
    value.colonies[0].traderId = "";
    expect(toSceneSnapshot(value, null)!.offworld.station).toEqual({
      name: "Orbital station",
      filledSeats: 0,
      totalSeats: 0,
      completed: false,
    });
    expect(toSceneSnapshot(value, null)!.offworld.colonies[0].traderId).toBeNull();
    expect(toSceneSnapshot(game(), null)!.offworld).toEqual({
      fleetsEnabled: true,
      colonies: [],
      fleets: [],
      station: null,
    });
  });
  test("placement navigation uses public body groups and leaves mixed choices open", () => {
    for (const [location, destination] of [
      ["mercury", "mercury"],
      ["moon", "earth"],
      ["ganymede", "jupiter"],
      ["phobos", "mars"],
    ] as const) {
      const value = game();
      value.board.tiles[0].location = location;
      expect(placementDestination(toSceneSnapshot(value, null))).toBe(destination);
      value.isSpectator = true;
      expect(placementDestination(toSceneSnapshot(value, null))).toBeNull();
    }
    const value = game();
    value.board.tiles.push({
      coordinates: { q: 400, r: 0, s: -400 },
      location: "mercury",
      type: "land",
      bonuses: [],
      tags: [],
    });
    value.currentPlayer.pendingTileSelection!.availableHexes.push("400,0,-400");
    expect(placementDestination(toSceneSnapshot(value, null))).toBeNull();
    expect(placementDestination(null)).toBeNull();
  });
  test("adapts actual DTO occupant labels and Go nil lists for inspection", () => {
    const value = game();
    value.board.tiles[0].occupiedBy = { type: "city-tile", tags: [], displayName: "Capital" };
    // Go may encode an empty slice as null even though generated TS uses arrays.
    Reflect.set(value.board.tiles[0], "tags", null);
    Reflect.set(value.board.tiles[0], "bonuses", null);
    const info = describeTile(value, "0,0,0")!;
    expect(info.tileType).toBe("city");
    expect(info.placedName).toBe("Capital");
    expect(info.bonuses).toEqual({});
    expect(info.isVolcanic).toBe(false);
  });
  test("copies authored appearance and public tile metadata without sharing state", () => {
    const value = game();
    value.globalParameters.maxOceans = 14;
    const tile = value.board.tiles[0];
    tile.tags = ["volcanic"];
    tile.displayName = "Arsia Mons";
    tile.bonuses = [{ type: "plant", amount: 2 }];
    tile.occupiedBy = {
      type: "city-tile",
      tags: ["capital"],
      displayName: "Capital",
      visual: { seed: 4294967295, structure: { landmark: "hall", details: ["pipes"] } },
    };
    Reflect.set(tile.occupiedBy.visual!, "private", "hidden");
    const scene = toSceneSnapshot(value, null)!;
    expect(scene.maxOceans).toBe(14);
    expect(scene.tiles[0].visual?.seed).toBe(4294967295);
    expect(scene.tiles[0].visual?.structure?.landmark).toBe("hall");
    expect(scene.tiles[0].displayName).toBe("Arsia Mons");
    expect(scene.tiles[0].occupantDisplayName).toBe("Capital");
    expect(JSON.stringify(scene)).not.toContain("hidden");
    scene.tiles[0].tags.push("changed");
    scene.tiles[0].bonuses[0].amount = 99;
    scene.tiles[0].visual!.structure!.details.push("tanks");
    expect(tile.tags).toEqual(["volcanic"]);
    expect(tile.bonuses[0].amount).toBe(2);
    expect(tile.occupiedBy.visual!.structure!.details).toEqual(["pipes"]);
  });
  test("maps public fields without private player data", () => {
    const scene = toSceneSnapshot(game(), null)!;
    expect(scene.players).toEqual([{ id: "player", color: "#ffaa00" }]);
    expect(scene.selectableTiles).toEqual(["0,0,0"]);
    expect(JSON.stringify(scene)).not.toContain("private-card");
  });
  test("clears scenes and stale selections", () => {
    expect(toSceneSnapshot(null, null)).toBeNull();
    expect(toSceneSnapshot(game(), "missing")!.selectedTile).toBeNull();
    expect(toSceneSnapshot(game(), "0,0,0")!.selectedTile).toBe("0,0,0");
  });
  test("routes only current legal selections without changing board state", () => {
    const value = game();
    const before = JSON.stringify(value);
    const selected: string[] = [];
    const handlers = {
      focus: () => {},
      hover: () => {},
      select: (id: string) => selected.push(id),
      inspect: () => {},
    };
    routeSceneEvent({ type: "tile-selected", gameId: "old", tileId: "0,0,0" }, value, handlers);
    routeSceneEvent({ type: "tile-selected", gameId: "game", tileId: "missing" }, value, handlers);
    routeSceneEvent({ type: "tile-selected", gameId: "game", tileId: "0,0,0" }, value, handlers);
    expect(selected).toEqual(["0,0,0"]);
    expect(JSON.stringify(value)).toBe(before);
  });
  test("spectators cannot select placements but can inspect", () => {
    const value = game();
    value.isSpectator = true;
    const selected: string[] = [];
    const inspected: string[] = [];
    const handlers = {
      focus: () => {},
      hover: () => {},
      select: (id: string) => selected.push(id),
      inspect: (id: string) => inspected.push(id),
    };
    routeSceneEvent({ type: "tile-selected", gameId: "game", tileId: "0,0,0" }, value, handlers);
    routeSceneEvent(
      { type: "tile-inspected", gameId: "game", tileId: "0,0,0", x: 0.5, y: 0.5 },
      value,
      handlers,
    );
    expect(selected).toEqual([]);
    expect(inspected).toEqual(["0,0,0"]);
  });
  test("hover clears without requiring a tile and ignores stale games", () => {
    const hovered: (string | null)[] = [];
    const handlers = {
      focus: () => {},
      select: () => {},
      inspect: () => {},
      hover: (id: string | null) => hovered.push(id),
    };
    routeSceneEvent(
      { type: "tile-hovered", gameId: "game", tileId: "0,0,0", x: 10, y: 20 },
      game(),
      handlers,
    );
    routeSceneEvent(
      { type: "tile-hovered", gameId: "old", tileId: null, x: 0, y: 0 },
      game(),
      handlers,
    );
    routeSceneEvent(
      { type: "tile-hovered", gameId: "game", tileId: "missing", x: 0, y: 0 },
      game(),
      handlers,
    );
    routeSceneEvent(
      { type: "tile-hovered", gameId: "game", tileId: null, x: 0, y: 0 },
      game(),
      handlers,
    );
    expect(hovered).toEqual(["0,0,0", null]);
  });
  test("planet travel is local and rejects events from departed games", () => {
    const value = game();
    const before = JSON.stringify(value);
    const destinations: string[] = [];
    const handlers = {
      focus: (body: string) => destinations.push(body),
      select: () => {},
      hover: () => {},
      inspect: () => {},
    };
    routeSceneEvent({ type: "body-selected", gameId: "old", body: "earth" }, value, handlers);
    routeSceneEvent({ type: "body-selected", gameId: "game", body: "earth" }, value, handlers);
    expect(destinations).toEqual(["earth"]);
    expect(JSON.stringify(value)).toBe(before);
  });
});
