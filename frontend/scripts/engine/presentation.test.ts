import { expect, test } from "bun:test";
import { placementSounds } from "../../src/engine/presentation";

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
