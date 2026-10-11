import * as THREE from "three";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { HexGrid2D } from "../../src/utils/hex-grid-2d";
import {
  StructureBatchStore,
  createStructureWarmupGeometry,
} from "../../src/components/game/board/structureBatch";
import { createGeometry } from "../../src/components/game/board/structureGeometry";
import {
  generateStructureLayout,
  createStructureShowcase,
  insideBuilding,
  structureWallDistance,
} from "../../src/components/game/board/structureLayout";
import {
  boardCenter,
  projectBoardPoint,
  nearestOnRoad,
} from "../../src/components/game/board/landscapeGeometry";
import {
  LandscapeBuilder,
  createLandscapeSampler,
  FIELD_SIZE,
  FIELD_BORDER,
  FIELD_SAMPLES,
  PATCH_SIZE,
  landscapeNoise,
  basinHeight,
  landscapeHeightAt,
  WATER_LEVEL,
  WATER_RADIUS,
  LAKE_BED_LEVEL,
  BEACH_WIDTH_MIN,
  BEACH_WIDTH_MAX,
  LAKE_MASK_REACH,
  LAKE_GROUND_REACH,
} from "../../src/components/game/board/landscapeFields";
import {
  MOHOLE_STENCIL_BIT,
  NUCLEAR_STENCIL_BIT,
  LAKE_STENCIL_BIT,
} from "../../src/components/game/board/boardConstants";
import type {
  LandscapeSource,
  LandscapeRequest,
  LandscapeDelta,
} from "../../src/components/game/board/landscapeTypes";
import type { StructureStyleRequestDto } from "../../src/types/generated/api-types";
const coord = (q: number, r = 0) => ({ q, r, s: -q - r });
const cards = JSON.parse(
  readFileSync(new URL("../../../backend/assets/cards.json", import.meta.url), "utf8"),
) as { id: string; name: string; style?: { tile: StructureStyleRequestDto } }[];
const requests = cards.filter((c) => c.style?.tile);
assert.equal(requests.length, 12);
const phobosCard = requests.find((c) => c.id === "021")!;
const phobosLayout = generateStructureLayout(42, phobosCard.style!.tile, phobosCard.name);
assert.equal(phobosLayout.style.cover, "dome");
const moonRadius = 0.4;
const moonCenter = { x: 0.06, y: -0.02 };
const moonPosition = projectBoardPoint(
  moonCenter.x,
  moonCenter.y,
  0,
  new THREE.Vector3(),
  moonRadius,
);
const moonNormal = moonPosition.clone().normalize();
const moonRotation = new THREE.Quaternion().setFromUnitVectors(
  new THREE.Vector3(0, 0, 1),
  moonNormal,
);
for (const geometry of createGeometry(phobosLayout, {
  coordinate: coord(1000),
  layout: phobosLayout,
  worldPosition: moonPosition,
  normal: moonNormal,
  surface: { radius: moonRadius, center: moonCenter },
})) {
  if (!geometry) {
    continue;
  }
  const vertices = geometry.getAttribute("position");
  for (let i = 0; i < vertices.count; i++) {
    const point = new THREE.Vector3()
      .fromBufferAttribute(vertices, i)
      .applyQuaternion(moonRotation)
      .add(moonPosition);
    assert.ok(
      point.length() > moonRadius - 0.02 && point.length() < moonRadius + 0.2,
      "Off-Mars city must follow its body instead of Mars coordinates",
    );
  }
  geometry.dispose();
}
for (const card of requests) {
  for (let seed = 1; seed <= 20; seed++) {
    const layout = generateStructureLayout(seed, card.style!.tile, card.name);
    assert.deepEqual(
      layout,
      generateStructureLayout(seed, structuredClone(card.style!.tile), card.name),
    );
    assert.ok(layout.buildings.length > 0, card.name);
    assert.ok(layout.roads.length > 0);
    assert.notEqual(layout.style.perimeter, "open", "Every city must have a perimeter wall");
    if (card.id === "120") {
      assert.ok(layout.entrances.length >= 2 && layout.entrances.length <= 3);
      const balanced = generateStructureLayout(
        seed,
        { ...card.style!.tile, density: "balanced" },
        card.name,
      );
      assert.ok(
        layout.buildings.length > balanced.buildings.length * 1.4,
        "Urbanized Area must have a denser building grid",
      );
      assert.ok(
        layout.roads.every((road) => road.points.length === 2),
        "Urbanized Area must use streets without a perimeter ring road",
      );
      assert.ok(
        layout.buildings.filter((b) => Math.hypot(b.x, b.y) > 0.105).length >= 8,
        "Urbanized Area must populate the outer hex districts",
      );
      for (const b of layout.buildings) {
        for (const dx of [-b.width / 2, b.width / 2]) {
          for (const dy of [-b.depth / 2, b.depth / 2]) {
            const x = b.x + dx * Math.cos(b.rotation) - dy * Math.sin(b.rotation);
            const y = b.y + dx * Math.sin(b.rotation) + dy * Math.cos(b.rotation);
            assert.ok(
              structureWallDistance(layout, x, y) < -0.003,
              "Urbanized building intersects wall",
            );
          }
        }
      }
    }
    if (card.id === "008") {
      assert.equal(layout.style.plan, "grid");
      assert.equal(layout.style.perimeter, "low-wall");
      assert.equal(layout.entrances.length, 3);
      assert.ok(layout.buildings.length >= 18, "Capital must fill its outer districts");
      const tower = layout.buildings.find((b) => b.landmark)!;
      assert.equal(tower.x, 0);
      assert.equal(tower.y, 0);
      assert.ok(tower.round && tower.height >= 0.175);
      assert.equal(layout.style.cover, "flat-glass");
      assert.ok(nearestOnRoad(tower, layout.roads).distance > tower.width / 2 + 0.005);
      for (const road of layout.roads) {
        for (let i = 1; i < road.points.length; i++) {
          assert.ok(
            Math.abs(road.points[i].x - road.points[i - 1].x) < 1e-8 ||
              Math.abs(road.points[i].y - road.points[i - 1].y) < 1e-8,
            "Capital streets must form a square grid",
          );
        }
      }
      for (const b of layout.buildings.filter((b) => !b.landmark)) {
        assert.ok(!b.round && !b.habitat && b.height < tower.height / 2);
        for (const dx of [-b.width / 2, b.width / 2]) {
          for (const dy of [-b.depth / 2, b.depth / 2]) {
            const x = b.x + dx * Math.cos(b.rotation) - dy * Math.sin(b.rotation);
            const y = b.y + dx * Math.sin(b.rotation) + dy * Math.cos(b.rotation);
            assert.ok(
              structureWallDistance(layout, x, y) < -0.004,
              "Capital building intersects wall",
            );
            assert.ok(!insideBuilding(tower, x, y, 0.005), "Building crowds the central tower");
          }
        }
      }
    }
    for (const b of layout.buildings) {
      assert.ok(Number.isFinite(b.x + b.y + b.height + b.rotation));
      if (b.entranceRoad) {
        const dx = b.entranceRoad.x - b.x,
          dy = b.entranceRoad.y - b.y;
        assert.ok(
          (dx * Math.sin(b.rotation) - dy * Math.cos(b.rotation)) / Math.hypot(dx, dy) > 0.999,
        );
      }
    }
    for (const plant of layout.plants) {
      const road = nearestOnRoad(plant, layout.roads);
      assert.ok(road.distance > road.halfWidth);
    }
  }
}
for (const card of requests) {
  const layout = generateStructureLayout(9183, card.style!.tile, card.name);
  const coordinate = { q: 4, r: 0, s: -4 };
  const center = boardCenter(coordinate),
    worldPosition = projectBoardPoint(center.x, center.y);
  const meshes = createGeometry(layout, {
    coordinate,
    worldPosition,
    normal: worldPosition.clone().normalize(),
    layout,
  });
  assert.ok(meshes.some(Boolean));
  for (const mesh of meshes) {
    if (mesh) {
      for (const attr of Object.values(mesh.attributes)) {
        assert.ok(Array.from(attr.array).every(Number.isFinite), `${card.name}: invalid mesh`);
      }
      mesh.dispose();
    }
  }
}
const showcase = createStructureShowcase().map((layout, i) => {
  const coordinate = coord(i % 3, Math.floor(i / 3));
  const center = boardCenter(coordinate),
    worldPosition = projectBoardPoint(center.x, center.y);
  return { layout, coordinate, worldPosition, normal: worldPosition.clone().normalize() };
});
const materials = new Map<string, THREE.MeshStandardMaterial>();
for (const mode of ["normal", "bright"]) {
  for (let i = 0; i < 19; i++) {
    if (mode === "bright" && i >= 3) {
      continue;
    }
    materials.set(`${mode}:${i}`, new THREE.MeshStandardMaterial());
  }
}
const store = new StructureBatchStore(materials);
store.update(showcase, new Map());
assert.equal(store.builds, showcase.length);
assert.ok(store.meshes.size <= 22, "Cities must share material batches");
const unchangedStart = performance.now();
store.update([...showcase], new Map());
const unchangedMs = performance.now() - unchangedStart;
assert.equal(store.builds, showcase.length, "Unchanged cities must retain GPU geometry");
let triangles = 0;
for (const plot of showcase) {
  for (const mesh of createGeometry(plot.layout, plot)) {
    if (!mesh) {
      continue;
    }
    triangles += mesh.getAttribute("position").count / 3;
    mesh.dispose();
  }
}
const batchCount = store.meshes.size;
store.update([], new Map());
store.dispose();
const camera = new THREE.PerspectiveCamera();
const scene = new THREE.Scene();
const structureRenderer = { info: { render: { frame: 0 } } } as THREE.WebGLRenderer;
function prepareStructureBatches() {
  store.beginPreparation();
  let steps = 0;
  while (store.meshes.size === 0) {
    assert.equal(store.prepareNext(), false);
    assert.ok(++steps <= 13, "Preparation must yield between the structure fixtures");
  }
  assert.equal(steps, 13);
  assert.equal(store.meshes.size, 22, "Every production material must have a prepared batch");
  assert.equal(store.ready, false);
  return new Map([...store.meshes].map(([key, mesh]) => [key, { mesh, geometry: mesh.geometry }]));
}
function renderStructurePreparation() {
  for (const mesh of store.meshes.values()) {
    assert.equal(mesh.geometry.index, null, "Warmup must use the production non-indexed draw path");
    assert.doesNotThrow(() =>
      mesh.onBeforeRender(
        structureRenderer,
        scene,
        camera,
        mesh.geometry,
        mesh.material,
        {} as THREE.Group,
      ),
    );
    mesh.onAfterRender(
      structureRenderer,
      scene,
      camera,
      mesh.geometry,
      mesh.material,
      {} as THREE.Group,
    );
  }
}
const abandoned = prepareStructureBatches();
renderStructurePreparation();
store.dispose();
assert.equal(store.group.children.length, 0, "Disposed preparation must leave the scene");
assert.equal(store.ready, false);
const prepared = prepareStructureBatches();
for (const [key, { mesh }] of prepared) {
  assert.notEqual(mesh, abandoned.get(key)!.mesh, "Effect replay must create fresh batches");
}
for (let frame = 0; frame < 2; frame++) {
  structureRenderer.info.render.frame++;
  renderStructurePreparation();
  renderStructurePreparation();
  assert.equal(store.prepareNext(), false, "Repeated draws within a frame are not extra frames");
}
structureRenderer.info.render.frame++;
renderStructurePreparation();
assert.equal(store.prepareNext(), true);
assert.equal(store.ready, true);
const warmupSchema = createStructureWarmupGeometry();
for (const [key, { mesh, geometry }] of prepared) {
  assert.equal(store.meshes.get(key), mesh);
  assert.equal(mesh.geometry, geometry, "Finishing preparation must retain uploaded buffers");
  assert.equal(mesh.unusedVertexCount, mesh.geometry.getAttribute("position").count);
  assert.equal(mesh.instanceCount, 0, "Preparation instances must be removed");
  for (const [name, attribute] of Object.entries(warmupSchema.attributes)) {
    assert.equal(mesh.geometry.getAttribute(name).itemSize, attribute.itemSize);
  }
}
warmupSchema.dispose();
store.update([showcase[0]], new Map());
for (const [key, { mesh, geometry }] of prepared) {
  assert.equal(store.meshes.get(key), mesh, "First placement must reuse the prepared batches");
  assert.equal(mesh.geometry, geometry, "First placement must fit the prepared buffers");
}
store.update([], new Map());
store.update(showcase, new Map());
assert.ok(store.group.children.length > 0, "City batches must survive effect replay too");
assert.ok(
  [...store.meshes].some(([key, mesh]) => mesh.geometry !== prepared.get(key)!.geometry),
  "Larger boards must still grow beyond prepared capacity",
);
store.dispose();
materials.forEach((material) => material.dispose());
assert.ok(
  showcase
    .find((p) => p.layout.style.landmark === "hall")!
    .layout.buildings.find((b) => b.landmark)!.height >= 0.077,
);
console.log(
  JSON.stringify({
    structureCount: showcase.length,
    structureBatches: batchCount,
    structureTriangles: triangles,
    unchangedUpdateMs: unchangedMs,
  }),
);
// Picking must remain correct when planets move, rotate, or use nonuniform scale.
const { sphereRaycast } = await import("../../src/utils/sphereRaycast");
const pickMesh = new THREE.Mesh(new THREE.SphereGeometry(2.02, 128, 64));
pickMesh.position.set(17, -4, 31);
pickMesh.rotation.set(0.3, -0.7, 0.2);
pickMesh.scale.set(1.2, 0.8, 1.1);
pickMesh.updateMatrixWorld();
const localPoint = new THREE.Vector3(0, 0, 2.02);
const expectedPoint = localPoint.clone().applyMatrix4(pickMesh.matrixWorld);
const origin = new THREE.Vector3(0, 0, 10).applyMatrix4(pickMesh.matrixWorld);
const pickRay = new THREE.Raycaster(origin, expectedPoint.clone().sub(origin).normalize());
const hits: THREE.Intersection[] = [];
sphereRaycast.call(pickMesh, pickRay, hits);
assert.equal(hits.length, 1);
assert.ok(hits[0].point.distanceTo(expectedPoint) < 1e-9);
assert.equal(hits[0].object, pickMesh);
pickRay.far = hits[0].distance - 0.01;
hits.length = 0;
sphereRaycast.call(pickMesh, pickRay, hits);
assert.equal(hits.length, 0);
pickRay.far = Infinity;
pickRay.near = origin.distanceTo(expectedPoint) + 0.01;
sphereRaycast.call(pickMesh, pickRay, hits);
assert.equal(hits.length, 0);
pickRay.near = 0;
pickRay.ray.direction.negate();
sphereRaycast.call(pickMesh, pickRay, hits);
assert.equal(hits.length, 0);
pickRay.ray.direction.negate();
const trianglePickStart = performance.now();
for (let i = 0; i < 500; i++) {
  hits.length = 0;
  pickMesh.raycast(pickRay, hits);
}
const trianglePickMs = performance.now() - trianglePickStart;
const spherePickStart = performance.now();
for (let i = 0; i < 500; i++) {
  hits.length = 0;
  sphereRaycast.call(pickMesh, pickRay, hits);
}
console.log(JSON.stringify({ trianglePickMs, spherePickMs: performance.now() - spherePickStart }));
pickMesh.geometry.dispose();
pickMesh.material.dispose();

const sources: LandscapeSource[] = [
  { coordinate: coord(0), kind: "greenery", seed: 1 },
  { coordinate: coord(1), kind: "ocean", seed: 2 },
  { coordinate: coord(0, 1), kind: "structure", seed: 3, layout: generateStructureLayout(3) },
  { coordinate: coord(-1), kind: "special-greenery", seed: 4 },
  { coordinate: coord(2), kind: "ocean", seed: 5 },
];
assert.ok(LAKE_BED_LEVEL < WATER_LEVEL && WATER_LEVEL < 0);
assert.ok(BEACH_WIDTH_MAX < LAKE_MASK_REACH && LAKE_MASK_REACH < LAKE_GROUND_REACH);
for (const width of [BEACH_WIDTH_MIN, BEACH_WIDTH_MAX]) {
  for (const landHeight of [0.0003, 0.0063]) {
    assert.equal(basinHeight(0, width, landHeight), WATER_LEVEL);
    assert.equal(basinHeight(-0.07, width, landHeight), LAKE_BED_LEVEL);
    assert.ok(Math.abs(basinHeight(width, width, landHeight) - landHeight) < 1e-12);
    let previous = LAKE_BED_LEVEL;
    for (let i = -100; i <= 160; i++) {
      const height = basinHeight(i / 1000, width, landHeight);
      assert.ok(height >= previous - 1e-12, "Basin must rise continuously toward land");
      previous = height;
    }
    for (const join of [-0.07, 0, width]) {
      const slope =
        (basinHeight(join + 1e-6, width, landHeight) -
          basinHeight(join - 1e-6, width, landHeight)) /
        2e-6;
      assert.ok(Math.abs(slope) < 0.001, "Bank joins must have smooth slopes");
    }
  }
}
const builder = new LandscapeBuilder();
const capitalCard = requests.find((card) => card.id === "008")!;
const capitalLayout = generateStructureLayout(839, capitalCard.style!.tile, "Capital");
const capitalSample = createLandscapeSampler({
  seed: 42,
  sources: [{ coordinate: coord(0), kind: "structure", seed: 839, layout: capitalLayout }],
});
let capitalGround = 0,
  capitalGrass = 0,
  capitalPaving = 0;
for (let x = -0.13; x < 0.13; x += 0.004) {
  for (let y = -0.14; y < 0.14; y += 0.004) {
    if (
      structureWallDistance(capitalLayout, x, y) > -0.007 ||
      Math.max(Math.abs(x), Math.abs(y)) < 0.045 ||
      capitalLayout.buildings.some((b) => insideBuilding(b, x, y, 0))
    ) {
      continue;
    }
    const sample = capitalSample(x, y);
    capitalGround++;
    capitalGrass += sample.coverage * (1 - sample.foundation * 0.85);
    capitalPaving += sample.foundation;
  }
}
assert.ok(
  capitalGrass / capitalGround > 0.6,
  "Capital's exposed ground must be predominantly green",
);
assert.ok(
  capitalPaving / capitalGround < 0.4,
  "Capital paving outside its central plaza must stay on roads and pads",
);
assert.equal(capitalSample(0.038, 0.038).foundation, 1, "Capital's inner square must be paved");
assert.ok(capitalSample(0.038, 0.038).blocked, "Capital's paved plaza must exclude vegetation");
const capitalPlants = [
  ...new LandscapeBuilder()
    .build({
      seed: 42,
      sources: [{ coordinate: coord(0), kind: "structure", seed: 839, layout: capitalLayout }],
    })
    .values(),
]
  .flatMap((p) => p.plants)
  .filter((p) => structureWallDistance(capitalLayout, p.x, p.y) < 0);
assert.ok(
  capitalPlants.filter((p) => p.kind === "tree").length >= 20,
  "Capital should have plentiful trees",
);
assert.ok(
  capitalPlants.filter((p) => p.kind === "bush").length >= 10,
  "Capital should have plentiful bushes",
);
const concreteLayout = generateStructureLayout(839, {}, "City");
const concretePlants = [
  ...new LandscapeBuilder()
    .build({
      seed: 42,
      sources: [{ coordinate: coord(0), kind: "structure", seed: 839, layout: concreteLayout }],
    })
    .values(),
].flatMap((p) => p.plants);
assert.equal(
  concretePlants.length,
  0,
  "Concrete city ground and its margin must exclude vegetation",
);
const start = performance.now();
const patches = builder.build({ seed: 42, sources });
console.log({
  patches: patches.size,
  preparationMs: performance.now() - start,
  plants: [...patches.values()].reduce((n, p) => n + p.plants.length, 0),
});
const again = builder.build({ seed: 42, sources: [...sources].reverse() });
for (const [key, p] of patches) {
  assert.equal(again.get(key), p, "Unchanged patches must retain their arrays");
}
const sample = createLandscapeSampler({ seed: 42, sources });
const ids = new Set<string>();
for (const p of patches.values()) {
  for (const plant of p.plants) {
    assert.ok(!ids.has(plant.id), "Duplicate plant across patches");
    ids.add(plant.id);
    const f = sample(plant.x, plant.y);
    assert.ok(!f.blocked && f.shore >= 0.008, "Plant intersects water or construction");
    assert.ok(Math.abs(f.height - plant.height) < 1e-8, "Plant does not follow terrain height");
    assert.ok(
      Math.abs(landscapeHeightAt(patches, plant.x, plant.y) - plant.height) < 0.0005,
      "Plant transition must start on the displayed field",
    );
  }
  const [ix, iy] = p.key.split(":").map(Number);
  for (const [dx, dy] of [
    [1, 0],
    [0, 1],
  ]) {
    const neighbor = patches.get(`${ix + dx}:${iy + dy}`);
    if (!neighbor) {
      continue;
    }
    for (let t = 0; t < FIELD_SAMPLES; t++) {
      const a =
        ((dy ? FIELD_BORDER + FIELD_SAMPLES - 1 : FIELD_BORDER + t) * FIELD_SIZE +
          (dx ? FIELD_BORDER + FIELD_SAMPLES - 1 : FIELD_BORDER + t)) *
        4;
      const b =
        ((dy ? FIELD_BORDER : FIELD_BORDER + t) * FIELD_SIZE +
          (dx ? FIELD_BORDER : FIELD_BORDER + t)) *
        4;
      assert.deepEqual(
        p.terrain.slice(a, a + 4),
        neighbor.terrain.slice(b, b + 4),
        `Terrain seam at ${p.key}`,
      );
      assert.deepEqual(
        p.materials.slice(a, a + 4),
        neighbor.materials.slice(b, b + 4),
        `Material seam at ${p.key}`,
      );
    }
  }
}
for (const boundary of [-2, -1, 0, 1, 2]) {
  assert.ok(
    Math.abs(landscapeNoise(boundary - 1e-7, 0.6, 42) - landscapeNoise(boundary + 1e-7, 0.6, 42)) <
      1e-5,
  );
}
const changed = builder.build({
  seed: 42,
  sources: [...sources, { coordinate: coord(7), kind: "greenery", seed: 10 }],
});
for (const [key, p] of patches) {
  assert.equal(changed.get(key), p, "Distant placement changed cached terrain");
}
assert.deepEqual(
  builder.build({ seed: 42, sources }),
  patches,
  "Removal must restore deterministic landscape",
);
const isolated = createLandscapeSampler({
  seed: 42,
  sources: [{ coordinate: coord(0), kind: "ocean", seed: 1 }],
});
assert.ok(isolated(0, 0).shore < 0);
const oceanRing: LandscapeSource[] = HexGrid2D.getNeighbors(coord(0)).map((coordinate) => ({
  coordinate,
  kind: "ocean",
  seed: 1,
}));
for (const seed of [42, 7, 1234]) {
  const single = createLandscapeSampler({
    seed,
    sources: [{ coordinate: coord(0), kind: "ocean", seed: 1 }],
  });
  for (let i = 0; i < 120; i++) {
    const angle = (i / 120) * Math.PI * 2;
    const radius = WATER_RADIUS + Math.SQRT2 * 0.04 + 0.007;
    assert.ok(
      single(Math.cos(angle) * radius, Math.sin(angle) * radius).shore > 0,
      "Coast distortion must stay inside the existing lake support bounds",
    );
  }
  for (const neighbor of HexGrid2D.getNeighbors(coord(0))) {
    const pair: LandscapeSource[] = [
      { coordinate: coord(0), kind: "ocean", seed: 1 },
      { coordinate: neighbor, kind: "ocean", seed: 2 },
    ];
    const joined = createLandscapeSampler({ seed, sources: pair });
    const reversed = createLandscapeSampler({ seed, sources: pair.toReversed() });
    const center = boardCenter(neighbor);
    for (let i = 0; i <= 40; i++) {
      const x = (center.x * i) / 40,
        y = (center.y * i) / 40;
      assert.ok(joined(x, y).shore < -0.035, "Adjacent oceans must keep an open water channel");
      for (const offset of [-0.14, -0.08, 0, 0.08, 0.14]) {
        assert.equal(
          joined(x + offset, y - offset).shore,
          reversed(x + offset, y - offset).shore,
          "Coast shape must not depend on ocean placement order",
        );
      }
    }
  }
  const lake = createLandscapeSampler({
    seed,
    sources: [{ coordinate: coord(0), kind: "ocean", seed: 1 }, ...oceanRing],
  });
  const island = createLandscapeSampler({ seed, sources: oceanRing });
  assert.ok(island(0, 0).shore > 0, "An empty tile enclosed by oceans must remain an island");
  for (let corner = 0; corner < 6; corner++) {
    const a = boardCenter(oceanRing[corner].coordinate);
    const b = boardCenter(oceanRing[(corner + 1) % 6].coordinate);
    for (const offset of [-0.015, 0, 0.015]) {
      const junction = lake((a.x + b.x) / 3 + offset, (a.y + b.y) / 3 - offset);
      assert.ok(junction.shore < -0.07, "Three ocean tiles must not create an interior shoreline");
      assert.ok(
        Math.abs(junction.height - lake(0, 0).height) < 1e-10,
        "Ocean junctions must have the same deep-water floor as the lake interior",
      );
    }
  }
  for (let i = 0; i < 120; i++) {
    const angle = (i / 120) * Math.PI * 2;
    const x = Math.cos(angle) * 0.42,
      y = Math.sin(angle) * 0.42;
    assert.equal(
      lake(x, y).shore,
      island(x, y).shore,
      "Filling the center of a lake must not change its outer coastline",
    );
  }
  const shoreSources: LandscapeSource[] = [
    ...oceanRing,
    { coordinate: coord(0), kind: "greenery", seed: 3 },
  ];
  const shoreSampler = createLandscapeSampler({ seed, sources: shoreSources });
  const shorePatches = new LandscapeBuilder().build({ seed, sources: shoreSources });
  let shorePlants = 0;
  for (const patch of shorePatches.values()) {
    for (const plant of patch.plants) {
      const terrain = shoreSampler(plant.x, plant.y);
      assert.ok(!terrain.blocked && terrain.shore > 0.008, "Island plants must clear the coast");
      shorePlants++;
    }
    const [ix, iy] = patch.key.split(":").map(Number);
    for (const [dx, dy] of [
      [1, 0],
      [0, 1],
    ]) {
      const neighbor = shorePatches.get(`${ix + dx}:${iy + dy}`);
      if (!neighbor) {
        continue;
      }
      for (let t = 0; t < FIELD_SAMPLES; t++) {
        const a =
          ((dy ? FIELD_BORDER + FIELD_SAMPLES - 1 : FIELD_BORDER + t) * FIELD_SIZE +
            (dx ? FIELD_BORDER + FIELD_SAMPLES - 1 : FIELD_BORDER + t)) *
          4;
        const b =
          ((dy ? FIELD_BORDER : FIELD_BORDER + t) * FIELD_SIZE +
            (dx ? FIELD_BORDER : FIELD_BORDER + t)) *
          4;
        assert.deepEqual(
          patch.terrain.slice(a, a + 2),
          neighbor.terrain.slice(b, b + 2),
          `Ocean height and shore distance must be continuous at ${patch.key} for seed ${seed}`,
        );
      }
    }
  }
  assert.ok(shorePlants > 0, "The island fixture must exercise shoreline vegetation clearance");
}
console.log(
  "Ocean coast bounds, connectivity, islands, placement order, seams, and plants passed.",
);
assert.ok(
  Array.from(
    { length: 100 },
    (_, i) => isolated(Math.cos(i) * 0.14, Math.sin(i) * 0.14).coverage,
  ).some((v) => v > 0.05),
  "Isolated water must support shore grass",
);
assert.ok([...patches.values()].some((p) => p.water));
assert.ok(PATCH_SIZE > 0);
const spreading = createLandscapeSampler({
  seed: 42,
  sources: [{ coordinate: coord(0), kind: "greenery", seed: 1 }],
});
const fringe = Array.from({ length: 120 }, (_, i) => {
  const angle = (i / 120) * Math.PI * 2;
  return spreading(Math.cos(angle) * 0.24, Math.sin(angle) * 0.24).coverage;
});
assert.equal(Math.max(...fringe), 0, "Exposed greenery must not send veins into empty tiles");
assert.equal(spreading(0.5, 0).coverage, 0, "Spread must remain local");
assert.ok(
  spreading(0, 0).lush > spreading(0, 0).litter + spreading(0, 0).mud,
  "Forest floor must favor leafy grass over brown soil and litter",
);
const greeneryPair = createLandscapeSampler({
  seed: 42,
  sources: [0, 1].map((q) => ({ coordinate: coord(q), kind: "greenery", seed: q + 1 })),
});
const sharedGreenEdge = boardCenter(coord(1));
assert.equal(
  greeneryPair(sharedGreenEdge.x / 2, sharedGreenEdge.y / 2).coverage,
  1,
  "Neighboring greeneries must blend across their shared edge",
);
const greenShore = createLandscapeSampler({
  seed: 42,
  sources: [
    { coordinate: coord(0), kind: "greenery", seed: 1 },
    { coordinate: coord(1), kind: "ocean", seed: 2 },
  ],
});
const shoreSamples = Array.from({ length: 160 }, (_, i) =>
  greenShore(0.145 + (i % 40) * 0.004, -0.06 + Math.floor(i / 40) * 0.04),
).filter((f) => f.shore > 0.008 && f.shore < 0.05);
assert.ok(shoreSamples.length > 10);
assert.ok(
  shoreSamples.every((f) => f.coverage > 0.95 && f.lush > f.mud),
  "Greenery beside water must reach the bank without a red or muddy strip",
);
const developed = createLandscapeSampler({
  seed: 42,
  sources: [
    { coordinate: coord(0), kind: "structure", seed: 3, layout: generateStructureLayout(3) },
  ],
});
assert.equal(developed(0, 0).foundation, 1, "City interior must cover the red terrain");
assert.equal(developed(0.4, 0).foundation, 0, "Foundations must stay near the city");
const circularCity = createLandscapeSampler({
  seed: 42,
  sources: [
    {
      coordinate: coord(0),
      kind: "structure",
      seed: 3,
      layout: generateStructureLayout(3, { plan: "radial" }),
    },
  ],
});
for (let i = 0; i < 72; i++) {
  const angle = (i * Math.PI) / 36;
  const inside = circularCity(Math.cos(angle) * 0.122, Math.sin(angle) * 0.122);
  const outside = circularCity(Math.cos(angle) * 0.16, Math.sin(angle) * 0.16);
  assert.ok(inside.foundation > 0.95, "Circular city paving must reach its wall on every side");
  assert.equal(outside.foundation, 0, "Circular cities must not pave the outer hex corners");
  assert.equal(
    outside.coverage,
    0,
    "Circular city ground must not leave a hex-shaped grass fringe",
  );
}
const cupolaCard = requests.find((card) => card.name === "Cupola City")!;
const cupolaSource: LandscapeSource = {
  coordinate: coord(0),
  kind: "structure",
  seed: 2069,
  layout: generateStructureLayout(2069, cupolaCard.style!.tile, cupolaCard.name),
};
const cupola = createLandscapeSampler({ seed: 42, sources: [cupolaSource] });
assert.equal(cupola(0, 0).foundation, 1, "Cupola interior must use the shared city foundation");
for (const neighborKind of ["greenery", "ocean"] as const) {
  for (const coordinate of HexGrid2D.getNeighbors(coord(0))) {
    const sample = createLandscapeSampler({
      seed: 42,
      sources: [cupolaSource, { coordinate, kind: neighborKind, seed: 4 }],
    });
    for (let i = 0; i < 72; i++) {
      const angle = (i * Math.PI) / 36;
      const inner = sample(Math.cos(angle) * 0.12999, Math.sin(angle) * 0.12999);
      const outer = sample(Math.cos(angle) * 0.13001, Math.sin(angle) * 0.13001);
      assert.ok(
        Math.abs(inner.foundation - outer.foundation) < 0.01 &&
          Math.abs(inner.coverage - outer.coverage) < 0.01,
        "Cupola paving and greenery must blend continuously across the former cutout edge",
      );
    }
  }
}
for (let i = 0; i < 72; i++) {
  const angle = (i * Math.PI) / 36;
  assert.ok(
    cupola(Math.cos(angle) * 0.122, Math.sin(angle) * 0.122).foundation > 0.95,
    "Cupola paving must meet its circular collar without exposing Mars",
  );
  assert.equal(
    cupola(Math.cos(angle) * 0.16, Math.sin(angle) * 0.16).foundation,
    0,
    "Cupola paving must remain circular rather than fill the hex corners",
  );
}
const cityPair = createLandscapeSampler({
  seed: 42,
  sources: [0, 1].map((q) => ({ coordinate: coord(q), kind: "structure", seed: q + 3 })),
});
const neighbor = boardCenter(coord(1));
assert.equal(
  cityPair(neighbor.x / 2, neighbor.y / 2).foundation,
  1,
  "Adjacent city foundations must join across the shared edge",
);
const separatedCities = createLandscapeSampler({
  seed: 42,
  sources: [0, 2].map((q) => ({ coordinate: coord(q), kind: "structure", seed: q + 3 })),
});
assert.equal(
  separatedCities(neighbor.x, neighbor.y).foundation,
  0,
  "City foundations must not cross an intervening empty tile",
);
assert.equal(spreading(0, 0).foundation, 0, "Greenery must not receive city foundations");
for (const coordinate of HexGrid2D.getNeighbors(coord(0))) {
  const greenCenter = boardCenter(coordinate);
  const mixedSources: LandscapeSource[] = [
    { coordinate: coord(0), kind: "structure", seed: 3, layout: generateStructureLayout(3) },
    { coordinate, kind: "greenery", seed: 4 },
  ];
  const mixed = createLandscapeSampler({ seed: 42, sources: mixedSources });
  const reversed = createLandscapeSampler({ seed: 42, sources: [...mixedSources].reverse() });
  const x = greenCenter.x / 2,
    y = greenCenter.y / 2;
  const edge = mixed(x, y);
  assert.ok(edge.coverage > 0.5, "City–greenery shared ground must carry grass");
  assert.ok(
    edge.foundation > 0.2 && edge.foundation < 0.8,
    "City–greenery edge must blend paving and grass",
  );
  assert.equal(edge.coverage, reversed(x, y).coverage);
  assert.equal(edge.foundation, reversed(x, y).foundation);
  assert.equal(mixed(0, 0).foundation, 1, "Blending must preserve the city interior");
  assert.equal(
    mixed(greenCenter.x, greenCenter.y).foundation,
    0,
    "Blending must preserve the greenery interior",
  );
  const length = Math.hypot(greenCenter.x, greenCenter.y);
  for (let along = -0.025; along <= 0.025; along += 0.005) {
    for (const across of [-0.07, -0.035, 0, 0.035, 0.07]) {
      const shared = mixed(
        x + (greenCenter.x * along - greenCenter.y * across) / length,
        y + (greenCenter.y * along + greenCenter.x * across) / length,
      );
      assert.ok(
        shared.coverage + shared.foundation >= 1 - 1e-6,
        "Grass and paving must fully cover the city–greenery shared edge",
      );
    }
  }
  for (let distance = 0.15; distance <= 0.6; distance += 0.015) {
    for (const across of [-0.1, -0.05, 0, 0.05, 0.1]) {
      const behindX = (-greenCenter.x * distance - greenCenter.y * across) / length;
      const behindY = (-greenCenter.y * distance + greenCenter.x * across) / length;
      assert.equal(
        mixed(behindX, behindY).foundation,
        developed(behindX, behindY).foundation,
        "A greenery neighbor must not extend paving into the opposite hex or terrain patches",
      );
    }
  }
  const opposite = { q: -coordinate.q, r: -coordinate.r, s: -coordinate.s };
  const ocean: LandscapeSource = { coordinate: opposite, kind: "ocean", seed: 5 };
  const waterBefore = createLandscapeSampler({ seed: 42, sources: [mixedSources[0], ocean] });
  const waterAfter = createLandscapeSampler({ seed: 42, sources: [...mixedSources, ocean] });
  for (let t = 0.6; t <= 1.2; t += 0.1) {
    const waterX = -greenCenter.x * t,
      waterY = -greenCenter.y * t;
    assert.equal(
      waterAfter(waterX, waterY).height,
      waterBefore(waterX, waterY).height,
      "City–greenery blending must not lift the opposite lake bed or water surface",
    );
    assert.equal(waterAfter(waterX, waterY).foundation, waterBefore(waterX, waterY).foundation);
  }
}
const separatedCityGreenery = createLandscapeSampler({
  seed: 42,
  sources: [
    { coordinate: coord(0), kind: "structure", seed: 3, layout: generateStructureLayout(3) },
    { coordinate: coord(2), kind: "greenery", seed: 4 },
  ],
});
assert.equal(
  separatedCityGreenery(neighbor.x, neighbor.y).foundation,
  0,
  "City–greenery blending must not bridge an empty tile",
);
assert.equal(separatedCityGreenery(neighbor.x, neighbor.y).coverage, 0);
assert.ok(spreading(0, 0).height > 0.0025, "Vegetated ground must have real relief");
assert.equal(developed(0, 0).height, 0.0003, "City pads must stay level");
assert.equal(isolated(0, 0).height, LAKE_BED_LEVEL);
for (const kind of ["structure", "excluded", "volcano"] as const) {
  const protectedGround = createLandscapeSampler({
    seed: 42,
    sources: [
      { coordinate: coord(0), kind, seed: 3 },
      { coordinate: coord(1), kind: "ocean", seed: 4 },
    ],
  });
  assert.ok(protectedGround(0, 0).height >= 0.0003, `${kind} must stay above the basin`);
}
for (let i = 1; i < 3000; i++) {
  const previous = isolated((i - 1) * 0.0001, 0);
  const current = isolated(i * 0.0001, 0);
  assert.ok(
    Math.abs(current.height - previous.height) < 0.00015,
    "Shore banks must rise continuously",
  );
}
console.log(
  "Shared shoreline, construction clearances, deterministic plants, and patch seams passed.",
);
const worker = new Worker(
  new URL("../../src/components/game/board/landscape.worker.ts", import.meta.url),
);
const jobs = new Map<number, (r: LandscapeDelta) => void>();
worker.onmessage = ({ data }: MessageEvent<LandscapeDelta>) => {
  jobs.get(data.id)?.(data);
  jobs.delete(data.id);
};
const send = (r: LandscapeRequest) =>
  new Promise<LandscapeDelta>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("Worker timeout")), 15000);
    jobs.set(r.id, (v) => {
      clearTimeout(timer);
      resolve(v);
    });
    worker.postMessage(r);
  });
try {
  const first = await send({ id: 1, seed: 42, sources, known: {} });
  const known = Object.fromEntries(first.patches.map((p) => [p.key, p.signature]));
  const repeat = await send({ id: 2, seed: 42, sources, known });
  assert.equal(repeat.patches.length, 0);
  const transferredAgain = await send({ id: 3, seed: 42, sources, known: {} });
  assert.deepEqual(transferredAgain.patches, first.patches, "Transfer detached worker cache");
  const empty = await send({ id: 4, seed: 42, sources: [], known });
  assert.equal(empty.removed.length, first.patches.length);
  const replaced = await send({ id: 5, seed: 42, sources, known: {} });
  assert.deepEqual(replaced.patches, first.patches);
  console.log("Worker deltas, acknowledged cache reuse, transfer safety, and removal passed.");
} finally {
  worker.terminate();
}

const denseSources: LandscapeSource[] = [];
for (let q = -4; q <= 4; q++) {
  for (let r = Math.max(-4, -q - 4); r <= Math.min(4, -q + 4); r++) {
    denseSources.push({ coordinate: coord(q, r), kind: "greenery", seed: 42 });
  }
}
const denseBuilder = new LandscapeBuilder();
const denseStart = performance.now();
const dense = denseBuilder.build({ seed: 42, sources: denseSources });
assert.ok(dense.size <= 128, "Full board exceeds surface capacity");
const densePlants = [...dense.values()].flatMap((p) => p.plants);
const counts = new Map<string, number>();
for (const p of densePlants) {
  const key = `${p.kind}:${p.seed % 4}`;
  counts.set(key, (counts.get(key) ?? 0) + 1);
}
assert.ok(Math.max(...counts.values()) < 4096, "Full board exceeds plant capacity");
const forest = densePlants.filter((p) => p.kind === "tree");
assert.ok(forest.length > denseSources.length * 120, "Forest interior is too sparse");
console.log({
  densePatches: dense.size,
  densePlants: densePlants.length,
  treesPerTile: forest.length / denseSources.length,
  densePreparationMs: performance.now() - denseStart,
});

// Exercise actual texture staging without requiring a browser WebGL context.
const { plugin } = await import("bun");
const { resolve, dirname } = await import("node:path");
await plugin({
  name: "landscape-raw",
  setup(build) {
    build.onResolve({ filter: /\.glsl\?raw$/ }, (args) => ({
      path: resolve(dirname(args.importer), args.path.slice(0, -4)),
      namespace: "landscape-glsl",
    }));
    build.onLoad({ filter: /.*/, namespace: "landscape-glsl" }, async (args) => ({
      contents: `export default ${JSON.stringify(await Bun.file(args.path).text())}`,
      loader: "js",
    }));
  },
});
const { LandscapeSurface } = await import("../../src/components/game/board/landscapeSurface");
const texture = new THREE.Texture();
const surface = new LandscapeSurface(
  {
    mars: texture,
    grass: texture,
    sand: texture,
    rock: texture,
    concrete: texture,
    waterNormals: texture,
  } as unknown as ReturnType<typeof import("../../src/hooks/useTextures").useTextures>,
  {
    groundAlbedo: new THREE.DataArrayTexture(),
    groundDetail: new THREE.DataArrayTexture(),
    iceAlbedo: new THREE.DataArrayTexture(),
    iceDetail: new THREE.DataArrayTexture(),
  },
);
let uploads = 0;
const renderer = {
  initTexture(t: THREE.DataArrayTexture) {
    uploads += t.layerUpdates.size;
    t.clearLayerUpdates();
  },
} as unknown as THREE.WebGLRenderer;
const { receivesPlanetHaze, addPlanetHaze, createPlanetHazeUniforms } =
  await import("../../src/components/game/board/shaders");
assert.ok(receivesPlanetHaze(surface.water), "Blended ocean must receive the planet's haze");
assert.ok(receivesPlanetHaze(surface.basin), "Recessed ground must receive haze");
assert.ok(!receivesPlanetHaze(surface.mask), "Cutout must not receive haze");
assert.equal(surface.ground.transparent, false, "Solid ground must precede transparent overlays");
assert.equal(surface.ground.depthWrite, true, "Solid ground must occlude far-side tiles");
assert.equal(surface.ground.stencilWriteMask, 0, "Ground must not cut holes in Mars");
assert.equal(surface.groundEdge.transparent, true, "Ground fringes must retain alpha blending");
assert.equal(surface.groundEdge.depthWrite, false, "Soft fringes must not become solid occluders");
assert.equal(surface.basin.transparent, false);
assert.equal(surface.basin.depthWrite, true);
assert.equal(surface.mask.colorWrite, false);
assert.equal(surface.mask.depthWrite, false);
assert.equal(surface.mask.stencilWriteMask, LAKE_STENCIL_BIT);
assert.equal(surface.mask.stencilRef, LAKE_STENCIL_BIT);
assert.equal(surface.basin.stencilFuncMask, MOHOLE_STENCIL_BIT | NUCLEAR_STENCIL_BIT);
assert.equal(surface.basin.stencilWriteMask, 0);
assert.equal(surface.group.children.length, 5, "Landscape includes solid ground and soft edges");
assert.ok(surface.group.children[0].renderOrder < 0, "Cutout must precede Mars");
for (const material of [surface.mask, surface.water]) {
  assert.equal(material.uniforms.uTerrain, surface.uniforms.uTerrain);
  assert.equal(material.uniforms.uLandscapeTime, surface.uniforms.uLandscapeTime);
}
const { createMoholeMaskMaterial } = await import("../../src/components/game/board/shaders");
const moholeMask = createMoholeMaskMaterial(1);
assert.equal(moholeMask.stencilWriteMask, MOHOLE_STENCIL_BIT);
assert.equal(moholeMask.stencilRef, MOHOLE_STENCIL_BIT);
let stencil = 0;
for (const mask of [moholeMask, surface.mask]) {
  stencil = (stencil & ~mask.stencilWriteMask) | (mask.stencilRef & mask.stencilWriteMask);
}
assert.equal(stencil, MOHOLE_STENCIL_BIT | LAKE_STENCIL_BIT, "Masks must not overwrite each other");
moholeMask.dispose();
const effectMaterial = new THREE.ShaderMaterial({ transparent: true, depthWrite: false });
assert.ok(!receivesPlanetHaze(effectMaterial), "Transparent effects must remain opt-in");
effectMaterial.dispose();
const restoreHaze = addPlanetHaze(surface.water, createPlanetHazeUniforms());
const waterShader = {
  vertexShader: surface.water.vertexShader,
  fragmentShader: surface.water.fragmentShader,
  uniforms: { ...surface.water.uniforms },
};
surface.water.onBeforeCompile(waterShader, renderer);
assert.match(waterShader.fragmentShader, /gl_FragColor\.rgb = applyPlanetHaze/);
restoreHaze();
const cachedMaterial = new THREE.MeshStandardMaterial();
const programCache = new Map<string, { uniforms: ReturnType<typeof createPlanetHazeUniforms> }>();
cachedMaterial.addEventListener("dispose", () => programCache.clear());
function renderCachedMaterial() {
  const key = cachedMaterial.customProgramCacheKey();
  if (!programCache.has(key)) {
    const shader = {
      vertexShader: THREE.ShaderLib.standard.vertexShader,
      fragmentShader: THREE.ShaderLib.standard.fragmentShader,
      uniforms: {},
    };
    cachedMaterial.onBeforeCompile(shader, renderer);
    programCache.set(key, {
      uniforms: shader.uniforms as ReturnType<typeof createPlanetHazeUniforms>,
    });
  }
  return programCache.get(key)!;
}
const beforeRefresh = createPlanetHazeUniforms();
const detachHaze = addPlanetHaze(cachedMaterial, beforeRefresh);
assert.equal(renderCachedMaterial().uniforms.uHazeCameraWorld, beforeRefresh.uHazeCameraWorld);
detachHaze();
const afterRefresh = createPlanetHazeUniforms();
const detachRefreshedHaze = addPlanetHaze(cachedMaterial, afterRefresh);
const refreshedProgram = renderCachedMaterial();
assert.equal(
  refreshedProgram.uniforms.uHazeCameraWorld,
  afterRefresh.uHazeCameraWorld,
  "Reattaching haze must replace cached camera uniforms after hot reload",
);
for (const distance of [12, 10, 8, 6, 4]) {
  afterRefresh.uHazeCameraWorld.value.makeTranslation(260, 0, distance);
  assert.equal(refreshedProgram.uniforms.uHazeCameraWorld.value.elements[14], distance);
}
detachRefreshedHaze();
cachedMaterial.dispose();
const groundShader = {
  vertexShader: THREE.ShaderLib.standard.vertexShader,
  fragmentShader: THREE.ShaderLib.standard.fragmentShader,
  uniforms: {},
};
surface.ground.onBeforeCompile(groundShader, renderer);
const edgeShader = {
  vertexShader: THREE.ShaderLib.standard.vertexShader,
  fragmentShader: THREE.ShaderLib.standard.fragmentShader,
  uniforms: {},
};
surface.groundEdge.onBeforeCompile(edgeShader, renderer);
assert.equal(edgeShader.vertexShader, groundShader.vertexShader, "Ground passes share geometry");
assert.equal(
  edgeShader.fragmentShader,
  groundShader.fragmentShader,
  "Ground passes classify the same live or baked alpha",
);
assert.doesNotMatch(groundShader.fragmentShader, /gl_FragDepth/, "Ground uses its actual depth");
const coverageSplit = groundShader.fragmentShader.indexOf("if(groundAlpha>=1.0)");
assert.ok(coverageSplit > 0);
assert.doesNotMatch(
  groundShader.fragmentShader.slice(coverageSplit),
  /\b(?:dFdx|dFdy|fwidth)\s*\(/,
  "Coverage splitting must follow material derivatives",
);
const alphaTestOffset = groundShader.fragmentShader.indexOf("#include <alphatest_fragment>");
assert.ok(alphaTestOffset > 0);
assert.match(groundShader.fragmentShader.slice(0, alphaTestOffset), /dFdx\(pavingHeight\)/);
assert.doesNotMatch(
  groundShader.fragmentShader.slice(alphaTestOffset),
  /\b(?:dFdx|dFdy|fwidth)\s*\(/,
  "Landscape derivatives must run before alpha discard to keep fringe lighting valid",
);
const state = {
  id: 1,
  seed: 42,
  sources,
  patches,
  plants: [...patches.values()].flatMap((p) => p.plants),
};
let ready = 0;
let transitionStart = 0;
function flush(next: typeof state, time: number) {
  surface.enqueue(next);
  for (let i = 0; i < 100 && ready !== next.id; i++) {
    surface.tick(renderer, time + i, (s, startedAt) => {
      ready = s.id;
      transitionStart = startedAt;
    });
  }
  assert.equal(ready, next.id, "Staged state never became ready");
}
flush(state, 1000);
assert.equal(
  uploads,
  patches.size * 6,
  "Each changed patch uploads two terrain, two material and two detail layers",
);
const uploaded = uploads;
flush({ ...state, id: 2 }, 1100);
assert.equal(uploads, uploaded, "Unchanged fields reuploaded");
flush({ ...state, id: 3, sources: [], patches: new Map(), plants: [] }, 2000);
assert.ok(transitionStart >= 2000 && transitionStart < 2100);
const births = (surface.group.children[0] as THREE.InstancedMesh).geometry.getAttribute(
  "patchBirth",
);
assert.ok(Math.abs(births.getX(0) * 1000 - transitionStart) < 0.001);
assert.ok(surface.group.children[0].visible, "Cutout must survive the removal transition");
surface.tick(renderer, 3000, () => {});
assert.equal(
  surface.group.children.filter((mesh) => mesh.visible).length,
  2,
  "Boards without oceans must skip lake passes",
);
assert.ok(
  surface.group.children.every(
    (m) => (m as THREE.InstancedMesh).geometry.getAttribute("patchOrigin").getX(0) === 20,
  ),
  "Removed slots remain visible",
);
flush({ ...state, id: 4 }, 4000);
assert.equal(uploads, uploaded * 3, "Reused slots did not upload restored patches");
let disposed = 0;
let disposedPasses = 0;
for (const material of [
  surface.ground,
  surface.groundEdge,
  surface.basin,
  surface.mask,
  surface.water,
]) {
  material.addEventListener("dispose", () => disposedPasses++);
}
surface.terrain.addEventListener("dispose", () => disposed++);
surface.retain();
surface.release();
surface.retain();
await Promise.resolve();
assert.equal(disposed, 0, "Strict Mode replay disposed the live surface");
surface.release();
await Promise.resolve();
assert.equal(disposed, 1);
assert.equal(disposedPasses, 5, "All surface materials must be released exactly once");
texture.dispose();
const {
  climateFromParameters,
  climateMilestones,
  easeClimate,
  CLIMATE_KEYS,
  BARREN_PARAMETERS,
  TERRAFORMED_PARAMETERS,
  plantForm,
} = await import("../../src/components/game/board/climate");
const barren = climateFromParameters(BARREN_PARAMETERS);
const terraformed = climateFromParameters(TERRAFORMED_PARAMETERS);
for (const key of CLIMATE_KEYS) {
  for (const value of [barren[key], terraformed[key]]) {
    assert.ok(value >= 0 && value <= 1, `Climate ${key} must stay within 0..1`);
  }
}
assert.equal(barren.ice, 1, "Barren oceans are frozen");
assert.equal(barren.frost, 1);
assert.equal(barren.tree + barren.bush + barren.grass + barren.birds, 0);
assert.equal(terraformed.ice, 0);
assert.equal(terraformed.frost, 0);
assert.equal(terraformed.tree * terraformed.grass * terraformed.birds, 1);
const thawed = climateFromParameters({ temperature: 0, oxygen: 0, oceans: 0 });
assert.equal(thawed.ice, 0, "Oceans are liquid at 0 °C");
assert.equal(thawed.frost, 0, "Frost is gone at 0 °C");
for (let temperature = -30; temperature < 8; temperature += 2) {
  const before = climateFromParameters({ temperature, oxygen: 7, oceans: 4 });
  const after = climateFromParameters({ temperature: temperature + 2, oxygen: 7, oceans: 4 });
  for (const key of ["grass", "bush", "tree", "greening"] as const) {
    assert.ok(after[key] >= before[key], `${key} must not recede as temperature rises`);
  }
  for (const key of ["frost", "ice"] as const) {
    assert.ok(after[key] <= before[key], `${key} must not grow as temperature rises`);
  }
  // Plants unlock in card order: grass, then bushes, then trees.
  assert.ok(before.grass >= before.bush && before.bush >= before.tree);
}
const eased = { ...barren };
assert.ok(easeClimate(eased, terraformed, 0.016), "Easing reports motion");
for (let i = 0; i < 2000 && easeClimate(eased, terraformed, 0.016); i++) {}
assert.deepEqual(eased, terraformed, "Easing settles on the target");
assert.deepEqual(climateMilestones(BARREN_PARAMETERS, TERRAFORMED_PARAMETERS), [
  "temperature",
  "oceans",
  "terraformed",
]);
assert.deepEqual(climateMilestones(TERRAFORMED_PARAMETERS, TERRAFORMED_PARAMETERS), []);
const climateSources = [{ coordinate: { q: 0, r: 0, s: 0 }, seed: 3, kind: "greenery" as const }];
const climatePlants = [
  ...new LandscapeBuilder().build({ seed: 5, sources: climateSources }).values(),
].flatMap((patch) => patch.plants);
assert.ok(climatePlants.length > 0);
for (const plant of climatePlants) {
  assert.ok(
    plant.rank >= 0 && plant.rank <= 1,
    "Plant rank must stay within its density threshold",
  );
}
for (let temperature = -30; temperature <= 8; temperature += 1) {
  const state = climateFromParameters({ temperature, oxygen: 7, oceans: 4 });
  assert.ok(state.bush >= state.tree, "Saplings follow bushes");
  const warmer = climateFromParameters({ temperature: temperature + 1, oxygen: 7, oceans: 4 });
  for (const kind of ["tree", "pine"]) {
    assert.deepEqual(
      plantForm(kind, warmer),
      plantForm(kind, state),
      "Trees do not grow with warmth",
    );
  }
  assert.ok(warmer.sky >= state.sky, "The sky warms with temperature");
}
const greenSample = createLandscapeSampler({ seed: 5, sources: climateSources })(0, 0);
assert.equal(greenSample.meadow, 0, "Greeneries are not shoreline meadows");
assert.ok(greenSample.greenery > 0.99);
const meadowSources = [{ coordinate: { q: 0, r: 0, s: 0 }, seed: 3, kind: "ocean" as const }];
const meadowSampler = createLandscapeSampler({ seed: 5, sources: meadowSources });
const meadowPeak = Math.max(
  ...Array.from({ length: 60 }, (_, i) => meadowSampler(0.06 + i * 0.002, 0).meadow),
);
assert.ok(meadowPeak > 0.5, "Isolated oceans grow a shoreline meadow");
let shown = new Set<string>();
for (let gate = 0; gate <= 1; gate += 0.05) {
  const next = new Set(climatePlants.filter((p) => p.rank <= gate).map((p) => p.id));
  for (const id of shown) {
    assert.ok(next.has(id), "Raising a climate gate must only add plants");
  }
  shown = next;
}
console.log(
  "Full board capacity, forest density, partial texture uploads, slot reuse, effect replay, and climate passed.",
);
