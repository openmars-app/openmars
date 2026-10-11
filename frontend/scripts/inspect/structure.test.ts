import { expect, test } from "bun:test";
import { resolveTarget } from "../../src/dev/inspection/catalog";
import { createScene, subjectKey } from "../../src/dev/inspection/scene";
import {
  generateStructureLayout,
  insideStructure,
  insideBuilding,
  insideGatehouse,
  structureWallDistance,
} from "../../src/components/game/board/structureLayout";
import {
  createGeometry,
  structureEmergenceDepth,
} from "../../src/components/game/board/structureGeometry";
import {
  boardCenter,
  nearestOnRoad,
  projectBoardPoint,
} from "../../src/components/game/board/landscapeGeometry";
import {
  createLandscapeSampler,
  LandscapeBuilder,
} from "../../src/components/game/board/landscapeFields";
import type { LandscapeSource } from "../../src/components/game/board/landscapeTypes";
import * as THREE from "three";
import { MARS_RELIEF_DEPTH } from "../../src/components/game/board/marsRelief";
import { SPHERE_RADIUS } from "../../src/components/game/board/boardConstants";
const request = resolveTarget("123").style!;
const coordinate = (q: number, r = 0) => ({ q, r, s: -q - r });

for (const id of ["123", "085", "P37", "182"]) {
  const request = resolveTarget(id).style!;
  test(`${id}: repeatable structures with clear roads and vegetation`, () => {
    for (let seed = 1; seed <= 20; seed++) {
      const layout = generateStructureLayout(seed, request);
      expect(layout).toEqual(generateStructureLayout(seed, structuredClone(request)));
      if (id === "123") {
        expect(layout.buildings.filter((b) => b.landmark)).toHaveLength(6);
        expect(layout.buildings.length).toBeGreaterThanOrEqual(20);
        expect(layout.buildings.length).toBeLessThanOrEqual(28);
        expect(
          layout.buildings.filter((b) => b.landmark && b.width * b.depth > 0.0016).length,
        ).toBeGreaterThanOrEqual(4);
        expect(layout.fixtures.filter((f) => f.kind === "smokestack")).toHaveLength(2);
        expect(layout.fixtures.filter((f) => f.kind === "tank")).toHaveLength(2);
        expect(layout.pipes).toHaveLength(2);
      } else if (id === "085") {
        expect(layout.buildings.filter((b) => b.landmark)).toHaveLength(1);
        expect(layout.buildings.length).toBeGreaterThanOrEqual(45);
        expect(layout.buildings.length).toBeLessThanOrEqual(85);
        expect(layout.fixtures.filter((f) => f.kind === "accent-light")).toHaveLength(4);
        expect(layout.fixtures.some((f) => f.emitter)).toBe(false);
        const globe = layout.buildings.find((b) => b.landmark)!;
        expect(Math.hypot(globe.x, globe.y)).toBeGreaterThan(0.02);
        expect(
          new Set(layout.buildings.map((b) => Math.round(b.height * 1000))).size,
        ).toBeGreaterThan(4);
        for (const b of layout.buildings.filter((b) => !b.landmark)) {
          expect(
            Math.hypot(b.x - globe.x, b.y - globe.y) - Math.hypot(b.width, b.depth) / 2,
          ).toBeGreaterThan(globe.width / 2);
        }
      } else if (id === "182") {
        expect(layout.buildings.filter((b) => b.landmark)).toHaveLength(1);
        expect(layout.buildings.length).toBeGreaterThanOrEqual(23);
        expect(layout.style.perimeter).toBe("high-wall");
        expect(layout.entrances).toHaveLength(2);
        const headquarters = layout.buildings.find((b) => b.landmark)!;
        expect(Math.hypot(headquarters.x, headquarters.y)).toBeGreaterThan(0.013);
        expect(headquarters.width * headquarters.depth).toBeGreaterThan(0.013);
        for (const b of layout.buildings.filter((b) => !b.landmark)) {
          expect(b.height).toBeLessThan(headquarters.height / 2);
          expect(insideGatehouse(layout, b.x, b.y, Math.hypot(b.width, b.depth) / 2)).toBe(false);
          expect(insideBuilding(headquarters, b.x, b.y, Math.hypot(b.width, b.depth) / 2)).toBe(
            false,
          );
        }
        for (const road of layout.roads) {
          for (let segment = 1; segment < road.points.length; segment++) {
            const a = road.points[segment - 1],
              b = road.points[segment];
            for (let i = 0; i <= 10; i++) {
              expect(
                insideBuilding(
                  headquarters,
                  a.x + ((b.x - a.x) * i) / 10,
                  a.y + ((b.y - a.y) * i) / 10,
                  road.width / 2,
                ),
              ).toBe(false);
            }
          }
        }
      } else {
        expect(layout.buildings.length).toBeGreaterThanOrEqual(16);
        expect(layout.buildings.length).toBeLessThanOrEqual(24);
        expect(layout.fixtures.filter((f) => f.kind === "ground-exhaust")).toHaveLength(8);
        expect(layout.fixtures.filter((f) => f.kind === "shaft").length).toBeGreaterThanOrEqual(3);
        const exhausts = layout.fixtures.filter((f) => f.kind === "ground-exhaust");
        for (const exhaust of exhausts) {
          const turns = (exhaust.rotation! - exhausts[0].rotation!) / (Math.PI / 2);
          expect(turns).toBeCloseTo(Math.round(turns), 8);
        }
        expect(layout.fixtures.filter((f) => f.emitter?.kind === "vapor")).toHaveLength(2);
        expect(layout.fixtures.some((f) => f.kind === "smokestack")).toBe(false);
        expect(layout.pipes.length).toBeGreaterThanOrEqual(12);
      }
      for (const b of layout.buildings) {
        for (const dx of [-b.width / 2, b.width / 2]) {
          for (const dy of [-b.depth / 2, b.depth / 2]) {
            const x = b.x + dx * Math.cos(b.rotation) - dy * Math.sin(b.rotation);
            const y = b.y + dx * Math.sin(b.rotation) + dy * Math.cos(b.rotation);
            expect(structureWallDistance(layout, x, y)).toBeLessThan(-0.003);
          }
        }
      }
      for (const fixture of layout.fixtures) {
        expect(structureWallDistance(layout, fixture.x, fixture.y) + fixture.radius).toBeLessThan(
          -0.003,
        );
        if (fixture.kind === "tank" || fixture.kind === "ground-exhaust") {
          const road = nearestOnRoad(fixture, layout.roads);
          expect(road.distance).toBeGreaterThan(road.halfWidth + fixture.radius);
        }
      }
      for (const pipe of layout.pipes) {
        const [a, b] = pipe.points;
        for (let i = 0; i <= 20; i++) {
          const p = { x: a.x + ((b.x - a.x) * i) / 20, y: a.y + ((b.y - a.y) * i) / 20 };
          const road = nearestOnRoad(p, layout.roads);
          expect(road.distance).toBeGreaterThan(road.halfWidth + pipe.radius);
        }
      }
      for (const plant of layout.plants) {
        expect(insideStructure(layout, plant.x, plant.y, 0.005)).toBe(false);
      }
      const c = boardCenter(coordinate(1, 1));
      const position = projectBoardPoint(c.x, c.y);
      const plot = {
        layout,
        coordinate: coordinate(1, 1),
        worldPosition: position,
        normal: position.clone().normalize(),
      };
      const geometries = createGeometry(layout, plot);
      const depth = structureEmergenceDepth(geometries);
      const rotation = new THREE.Quaternion().setFromUnitVectors(
        new THREE.Vector3(0, 0, 1),
        plot.normal,
      );
      const point = new THREE.Vector3();
      for (const geometry of geometries) {
        if (!geometry) {
          continue;
        }
        const positions = geometry.getAttribute("position");
        expect(Array.from(positions.array).every(Number.isFinite)).toBe(true);
        expect(geometry.getAttribute("structureFrostExposure").count).toBe(positions.count);
        expect(geometry.getAttribute("structureCollapsePivot").count).toBe(positions.count);
        let furthest = 0;
        for (let vertex = 0; vertex < positions.count; vertex++) {
          point.fromBufferAttribute(positions, vertex);
          point.z -= depth;
          point.applyQuaternion(rotation).add(position);
          furthest = Math.max(furthest, point.length());
        }
        expect(furthest).toBeLessThan(SPHERE_RADIUS - MARS_RELIEF_DEPTH);
        geometry.dispose();
      }
    }
  });
}

test("level buried structures retain full paving with and without a neighboring city", () => {
  const layout = generateStructureLayout(42, resolveTarget("P37").style);
  const settlement: LandscapeSource = {
    coordinate: coordinate(0),
    kind: "structure",
    seed: 42,
    layout,
  };
  const neighbor: LandscapeSource = {
    coordinate: coordinate(1),
    kind: "structure",
    seed: 7,
    layout: generateStructureLayout(7),
  };
  for (const sources of [[settlement], [settlement, neighbor]]) {
    const sample = createLandscapeSampler({ seed: 42, sources });
    for (const x of [-0.075, -0.035, 0, 0.035, 0.075]) {
      for (const y of [-0.075, -0.035, 0, 0.035, 0.075]) {
        expect(sample(x, y).foundation).toBeGreaterThan(0.99);
      }
    }
  }
});

test("recessed underground structures keep access buildings around the covered basin", () => {
  const layout = generateStructureLayout(42, resolveTarget("032").style);
  expect(layout.style.plan).toBe("radial");
  expect(layout.buildings).toHaveLength(5);
  for (const building of layout.buildings) {
    expect(Math.hypot(building.x, building.y)).toBeGreaterThan(0.095);
  }
});

test("factories share neighboring paving and greenery through the landscape", () => {
  const factory: LandscapeSource = {
    coordinate: coordinate(0),
    kind: "structure",
    seed: 42,
    layout: generateStructureLayout(42, request),
  };
  const neighborCenter = boardCenter(coordinate(1));
  for (const other of [generateStructureLayout(7), generateStructureLayout(8, request)]) {
    const sources: LandscapeSource[] = [
      factory,
      { coordinate: coordinate(1), kind: "structure", seed: 7, layout: other },
    ];
    const sample = createLandscapeSampler({ seed: 42, sources });
    const midpoint = sample(neighborCenter.x / 2, neighborCenter.y / 2);
    expect(midpoint.foundation).toBeGreaterThan(0.8);
    const builder = new LandscapeBuilder();
    const initial = builder.build({ seed: 42, sources });
    const known = new Map([...initial].map(([key, p]) => [key, p.signature]));
    const cleared = builder.build({ seed: 42, sources: [sources[1]] });
    expect([...known].some(([key, signature]) => cleared.get(key)?.signature !== signature)).toBe(
      true,
    );
  }
  const withGreen = createLandscapeSampler({
    seed: 42,
    sources: [factory, { coordinate: coordinate(1), kind: "greenery", seed: 2 }],
  });
  const isolated = createLandscapeSampler({ seed: 42, sources: [factory] });
  const x = neighborCenter.x / 2 + 0.025;
  expect(withGreen(x, 0).coverage).toBeGreaterThan(isolated(x, 0).coverage);
});

test("inspection resolves the authored factory for tiles and exercises removal", () => {
  expect(resolveTarget(null, "industrial-center").style).toEqual(request);
  const options = {
    target: resolveTarget("123"),
    seed: 42,
    color: "#e43c32",
    climate: "barren" as const,
    state: "placement" as const,
    scene: "destruction" as const,
  };
  const subject = (placed: boolean) =>
    createScene(options, placed).board.tiles.find(
      (t) => Object.values(t.coordinates).join(",") === subjectKey,
    )!;
  expect(subject(false).occupiedBy?.visual?.structure?.landmark).toBe("factory");
  expect(subject(true).occupiedBy?.type).toBe("nuclear-zone-tile");
  expect(subject(true).occupiedBy?.visual?.structure).toBeUndefined();
});
