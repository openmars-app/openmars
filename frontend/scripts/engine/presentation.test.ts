import { expect, test } from "bun:test";
import { placementSounds, publicSceneCues } from "../../src/engine/presentation";
import type { GameDto, StateDiffDto } from "../../src/types/generated/api-types";

const game = {
  id: "game",
  currentPhase: "action",
  currentPlayer: {
    id: "player",
    cards: [{ id: "secret", name: "Secret Asteroid" }],
    playedCards: [{ id: "public", name: "Giant Ice Asteroid" }],
  },
  otherPlayers: [],
} as unknown as GameDto;
const project = {
  gameId: "game",
  sequenceNumber: 7,
  sourceType: "standard_project",
  source: "Asteroid",
  playerId: "player",
} as StateDiffDto;

test("maps only explicit public live asteroid events with stable identities", () => {
  const payload = { logs: [project], isHistory: false };
  const cues = publicSceneCues(payload, game);
  expect(cues).toEqual(publicSceneCues(payload, game));
  expect(cues).toHaveLength(1);
  expect(cues[0].id).toBe("log:7:project");
  expect(cues[0].kind).toBe("asteroid");
  expect(cues[0].seed).toBeGreaterThan(0);
  expect(publicSceneCues({ ...payload, isHistory: true }, game)).toEqual([]);
  expect(
    publicSceneCues({ logs: [{ ...project, gameId: "departed" }], isHistory: false }, game),
  ).toEqual([]);
  expect(
    publicSceneCues(
      { logs: [{ ...project, sequenceNumber: Number.MAX_SAFE_INTEGER + 1 }], isHistory: false },
      game,
    ),
  ).toEqual([]);
});

test("uses publicly played card definitions and never inspects a private hand", () => {
  const log = {
    ...project,
    sourceType: "card_play",
    changes: { playerChanges: { player: { cardsPlayed: ["secret", "public"] } } },
  } as unknown as StateDiffDto;
  const cues = publicSceneCues({ logs: [log], isHistory: false }, game);
  expect(cues.map((cue) => cue.id)).toEqual(["log:7:card:public"]);
  expect(JSON.stringify(cues)).not.toContain("Secret");
  expect(JSON.stringify(cues)).not.toContain("Giant Ice");
  const parameterOnly = { ...project, sourceType: "other", source: "temperature" };
  expect(publicSceneCues({ logs: [parameterOnly], isHistory: false }, game)).toEqual([]);
});

test("construction and ocean placement sounds follow live occupant changes once", () => {
  const scene = (types: Array<string | null>, gameId = "game", mapId = "tharsis") => ({
    gameId,
    mapId,
    tiles: types.map((occupant, id) => ({ id: String(id), occupant })),
  });
  const empty = scene([null, null, null]);
  const city = scene(["city-tile", null, null]);
  expect(placementSounds(null, city, "live")).toEqual([]);
  expect(placementSounds(empty, city, "snapshot")).toEqual([]);
  expect(placementSounds(empty, city, "live")).toEqual(["construction"]);
  expect(placementSounds(city, { ...city }, "live")).toEqual([]);
  expect(placementSounds(city, scene(["city-tile", "ocean-tile", "mining-tile"]), "live")).toEqual([
    "water",
    "construction",
  ]);
  expect(placementSounds(city, scene(["ocean-tile", null, null]), "live")).toEqual(["water"]);
  expect(placementSounds(city, empty, "live")).toEqual([]);
  expect(placementSounds(empty, scene(["city-tile"], "other"), "live")).toEqual([]);
  expect(placementSounds(empty, scene(["city-tile"], "game", "elysium"), "live")).toEqual([]);
  expect(placementSounds(empty, null, "live")).toEqual([]);
  for (const type of [
    "city-tile",
    "volcano-tile",
    "special-tile",
    "commercial-district-tile",
    "industrial-center-tile",
    "mining-tile",
    "mohole-tile",
    "restricted-tile",
  ]) {
    expect(placementSounds(empty, scene([type]), "live")).toEqual(["construction"]);
  }
  // Greenery feedback remains owned by the existing oxygen-change handler.
  expect(
    placementSounds(
      empty,
      scene(["greenery-tile", "ecological-zone-tile", "natural-preserve-tile"]),
      "live",
    ),
  ).toEqual([]);
});
