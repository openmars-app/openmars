import { expect, test } from "bun:test";
import { placeSolarLabels } from "../../src/engine/solarLabelLayout";
import type { BodyId } from "../../src/engine/scene.generated";

const bodies: BodyId[] = [
  "mars",
  "mercury",
  "venus",
  "earth",
  "ceres",
  "jupiter",
  "saturn",
  "uranus",
  "neptune",
];

test("crowded solar destinations keep separate, reachable targets inside the HUD", () => {
  for (const [width, height, compact] of [
    [1280, 720, false],
    [750, 340, true],
    [863, 360, true],
  ] as const) {
    const insets = { top: 40, bottom: 48, left: 88, right: 88 };
    const labels = placeSolarLabels(
      bodies.map((body) => ({ body, point: [width / 2, height / 2], hitSize: 12 })),
      compact,
      width,
      height,
      insets,
    );
    expect(labels).toHaveLength(9);
    for (const label of labels) {
      expect(label.width).toBeGreaterThanOrEqual(44);
      expect(label.height).toBeGreaterThanOrEqual(44);
      expect(label.x - label.width / 2).toBeGreaterThanOrEqual(insets.left);
      expect(label.x + label.width / 2).toBeLessThanOrEqual(width - insets.right);
      expect(label.y - label.height / 2).toBeGreaterThanOrEqual(insets.top);
      expect(label.y + label.height / 2).toBeLessThanOrEqual(height - insets.bottom);
      for (const other of labels) {
        if (other === label) {
          continue;
        }
        expect(
          Math.abs(label.x - other.x) >= label.width || Math.abs(label.y - other.y) >= label.height,
        ).toBe(true);
      }
    }
  }
});

test("an isolated label retains its source anchor offset", () => {
  const marker = { body: "mars" as const, point: [300, 200] as [number, number], hitSize: 16 };
  const labels = placeSolarLabels([marker], false, 800, 500, {
    top: 0,
    bottom: 0,
    left: 0,
    right: 0,
  });
  expect(labels[0].marker).toBe(marker);
  expect([labels[0].x, labels[0].y]).toEqual([300, 186]);
});

test("uneven moving anchors never drop a destination or cover camera controls", () => {
  let seed = 12891;
  const random = () => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    return seed / 4294967296;
  };
  for (let sample = 0; sample < 60; sample++) {
    const width = sample % 2 ? 750 : 863;
    const height = sample % 2 ? 340 : 360;
    const controls = { x: width - 240, y: 56, width: 140, height: 44 };
    const labels = placeSolarLabels(
      bodies.map((body) => ({ body, point: [width * random(), height * random()], hitSize: 12 })),
      true,
      width,
      height,
      { top: 40, bottom: 48, left: 88, right: 88 },
      [controls],
    );
    expect(labels).toHaveLength(9);
    for (const label of labels) {
      expect(
        label.x + label.width / 2 <= controls.x ||
          label.x - label.width / 2 >= controls.x + controls.width ||
          label.y + label.height / 2 <= controls.y ||
          label.y - label.height / 2 >= controls.y + controls.height,
      ).toBe(true);
      for (const other of labels) {
        if (other === label) {
          continue;
        }
        expect(
          Math.abs(label.x - other.x) >= label.width || Math.abs(label.y - other.y) >= label.height,
        ).toBe(true);
      }
    }
  }
});
