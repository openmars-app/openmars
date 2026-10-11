import { expect, test } from "bun:test";
import { cardTargets, resolveTarget } from "../../src/dev/inspection/catalog";
import { createScene, subjectKey } from "../../src/dev/inspection/scene";

test("card targets preserve the authored placement and city style", () => {
  expect(resolveTarget("123")).toMatchObject({
    tile: "industrial-center",
    name: "Industrial Center",
  });
  const city = cardTargets.find((target) => target.tile === "city" && target.style)!;
  expect(resolveTarget(city.card, "city").style).toEqual(city.style);
  expect(() => resolveTarget("does-not-exist")).toThrow("no supported Mars placement");
  expect(() => resolveTarget("123", "ocean")).toThrow();
  expect(() => resolveTarget(null, "invented-tile")).toThrow();
  expect(cardTargets.some((target) => target.name === "Phobos Space Haven")).toBe(false);
});

test("comparison fixtures are stable and placement changes only the subject", () => {
  const options = {
    target: resolveTarget("123"),
    state: "placement" as const,
    seed: 42,
    color: "#e43c32",
    climate: "growing" as const,
  };
  const before = createScene(options, false);
  const after = createScene(options, true);
  expect(after).toEqual(createScene(options, true));
  expect(after.board.tiles).toHaveLength(61);
  const changed = after.board.tiles.filter(
    (tile, i) => JSON.stringify(tile) !== JSON.stringify(before.board.tiles[i]),
  );
  expect(changed).toHaveLength(1);
  expect(Object.values(changed[0].coordinates).join(",")).toBe(subjectKey);
  expect(changed[0].bonuses).toEqual([]);
  expect(changed[0].occupiedBy?.type).toBe("industrial-center-tile");
  expect(after.board.tiles.filter((tile) => tile.occupiedBy)).toHaveLength(4);
});

test("city styles are ordinary city tiles and nuclear placement replaces a city", () => {
  const options = {
    target: resolveTarget(null, "nuclear-zone"),
    state: "placement" as const,
    seed: 42,
    color: "#e43c32",
    climate: "barren" as const,
  };
  const subject = (placed: boolean) =>
    createScene(options, placed).board.tiles.find(
      (tile) => Object.values(tile.coordinates).join(",") === subjectKey,
    )!;
  expect(subject(false).occupiedBy?.type).toBe("city-tile");
  expect(subject(true).occupiedBy?.type).toBe("nuclear-zone-tile");
  const styles = createScene(
    { ...options, scene: "structures", state: "settled" },
    true,
  ).board.tiles.filter((tile) => tile.occupiedBy);
  expect(styles.length).toBeGreaterThan(4);
  expect(styles.every((tile) => tile.occupiedBy?.visual?.structure)).toBe(true);
});
