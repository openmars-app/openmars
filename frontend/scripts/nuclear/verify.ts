import assert from "node:assert/strict";
import * as THREE from "three";
import type { TileDto } from "../../src/types/generated/api-types";
import {
  reconcileNuclearTransitions,
  advanceNuclearTransitions,
  collapseParameters,
  collapseProgress,
  NUCLEAR_COLLAPSE_SECONDS,
} from "../../src/components/game/board/nuclearTransitions";
import { createGeometry } from "../../src/components/game/board/structureGeometry";
import { generateStructureLayout } from "../../src/components/game/board/structureLayout";
import { StructureBatchStore } from "../../src/components/game/board/structureBatch";
import {
  createNuclearGeometry,
  nuclearSurfaceHeight,
  nuclearGroundHeight,
  nuclearProfile,
  bareNuclearGround,
  nuclearRadius,
  nuclearMoundRadius,
  nuclearPhase,
  nuclearElapsed,
  NUCLEAR_PLAYBACK_RATE,
  NUCLEAR_FLIGHT_SECONDS,
  NUCLEAR_DURATION_SECONDS,
  nuclearShake,
  NUCLEAR_SEGMENTS,
  createNuclearWaveGeometry,
} from "../../src/components/game/board/nuclearGeometry";
import {
  NuclearDebrisBuffer,
  createNuclearSites,
  createNuclearFragments,
  nuclearFragmentMatrix,
  debrisScratch,
  NUCLEAR_DEBRIS_SECONDS,
  nuclearGroundNormal,
  nuclearDirection,
} from "../../src/components/game/board/nuclearDebris";
import { EMPTY_LANDSCAPE } from "../../src/components/game/board/landscapeTypes";
import { sharedRockGeometry } from "../../src/components/game/board/rockGeometry";
import { changedOccupants } from "../../src/hooks/usePreviousTiles";
import {
  NUCLEAR_STENCIL_BIT,
  LAKE_STENCIL_BIT,
  MOHOLE_STENCIL_BIT,
} from "../../src/components/game/board/boardConstants";

assert.equal(NUCLEAR_STENCIL_BIT & (LAKE_STENCIL_BIT | MOHOLE_STENCIL_BIT), 0);
const waveGeometry = createNuclearWaveGeometry();
const wavePositions = waveGeometry.getAttribute("position");
const waveIndices = waveGeometry.index!;
const projectedWave = Array.from({ length: wavePositions.count }, (_, i) =>
  new THREE.Vector3(wavePositions.getX(i), wavePositions.getY(i), 2.03)
    .normalize()
    .multiplyScalar(2.02 + wavePositions.getZ(i)),
);
for (let i = 0; i < waveIndices.count; i += 3) {
  const center = projectedWave[waveIndices.getX(i)]
    .clone()
    .add(projectedWave[waveIndices.getX(i + 1)])
    .add(projectedWave[waveIndices.getX(i + 2)])
    .multiplyScalar(1 / 3);
  assert.ok(
    center.length() > 2.023,
    "Shockwave triangles must follow the planet instead of cutting through its surface",
  );
}
waveGeometry.dispose();
const nuclearProjection = {
  origin: new THREE.Vector3(0, 0, 2.03),
  rotation: new THREE.Quaternion(),
};
for (const seed of [1, 42, 997, 2147483647]) {
  const profile = nuclearProfile(seed);
  const geometry = createNuclearGeometry(seed, bareNuclearGround, nuclearProjection);
  const mask = geometry.mask.getAttribute("position");
  const bowl = geometry.bowl.getAttribute("position");
  const scorch = geometry.scorch.getAttribute("position");
  for (let i = 0; i <= NUCLEAR_SEGMENTS; i++) {
    const angle = (i / NUCLEAR_SEGMENTS) * Math.PI * 2;
    const radius = nuclearRadius(angle, seed);
    const edge = NUCLEAR_SEGMENTS + 1 + i;
    assert.ok(
      Math.abs(
        Math.hypot(mask.getX(edge) - profile.center.x, mask.getY(edge) - profile.center.y) - radius,
      ) < 1e-7,
    );
    assert.ok(
      nuclearSurfaceHeight(mask.getX(edge), mask.getY(edge), seed) > 0,
      "Cutout must end where the wall has met the surface",
    );
    const outer = scorch.count - NUCLEAR_SEGMENTS - 1 + i;
    assert.ok(
      Math.hypot(scorch.getX(outer), scorch.getY(outer)) <= 0.202,
      "Ejecta fans stay bounded",
    );
    assert.ok(
      Math.abs(scorch.getZ(outer) - 0.0009) < 0.00001,
      "Dust settles onto surrounding ground",
    );
    const shoulder = bowl.count - NUCLEAR_SEGMENTS - 1 + i;
    assert.ok(Math.abs(scorch.getX(i) - bowl.getX(shoulder)) < 1e-7);
    assert.ok(Math.abs(scorch.getY(i) - bowl.getY(shoulder)) < 1e-7);
    assert.ok(
      Math.abs(scorch.getZ(i) - bowl.getZ(shoulder)) < 1e-7,
      "Opaque mound joins the apron without a gap",
    );
    const crest = 30 * (NUCLEAR_SEGMENTS + 1) + i;
    assert.ok(Math.abs(bowl.getX(crest) - mask.getX(edge)) < 1e-7);
    assert.ok(
      Math.abs(bowl.getY(crest) - mask.getY(edge)) < 1e-7,
      "Opaque terrain completely covers the stencil perimeter",
    );
    assert.ok(bowl.getZ(crest) > 0.002);
  }
  let minimum = Infinity;
  for (let i = 0; i < bowl.count; i++) {
    const z = bowl.getZ(i);
    assert.ok(Number.isFinite(z));
    assert.ok(z > -0.02 && z < 0.017, "Basin stays shallow with a raised broken rim");
    minimum = Math.min(minimum, z);
    assert.ok(Math.hypot(bowl.getX(i), bowl.getY(i)) < 0.135, "Excavation stays inside the border");
  }
  assert.ok(minimum < -0.009);
  for (const mesh of Object.values(geometry)) {
    const positions = mesh.getAttribute("position");
    const planet = mesh.getAttribute("nuclearPlanetDirection");
    for (let i = 0; i < positions.count; i++) {
      const expected = new THREE.Vector3(positions.getX(i), positions.getY(i), 2.03).normalize();
      assert.ok(
        expected.distanceTo(new THREE.Vector3().fromBufferAttribute(planet, i)) < 1e-7,
        "Mars texture direction follows planet-local coordinates",
      );
    }
    for (let i = 0; i < positions.count; i += NUCLEAR_SEGMENTS + 1) {
      assert.ok(Math.abs(positions.getX(i) - positions.getX(i + NUCLEAR_SEGMENTS)) < 1e-7);
      assert.ok(Math.abs(positions.getZ(i) - positions.getZ(i + NUCLEAR_SEGMENTS)) < 1e-7);
    }
    mesh.dispose();
  }
}
const empty = new Map([["0,0,0", ""]]);
const placed = new Map([["0,0,0", "nuclear-zone-tile"]]);
assert.equal(changedOccupants(null, placed).size, 0, "Reconnect must not replay");
assert.equal(changedOccupants(empty, placed).size, 1);
assert.equal(
  changedOccupants(placed, new Map(placed)).size,
  0,
  "Unrelated board update must not replay",
);
assert.equal(changedOccupants(placed, empty).size, 0);
assert.equal(changedOccupants(empty, placed).size, 1, "Replacement after removal must play");
assert.equal(nuclearPhase(0), "flight");
assert.equal(nuclearPhase(0.8), "blast");
assert.equal(nuclearPhase(6.99), "blast");
assert.equal(nuclearPhase(7), "complete");
assert.equal(nuclearPhase(1000), "complete");
assert.equal(nuclearPhase(nuclearElapsed(0.99)), "flight");
assert.equal(nuclearPhase(nuclearElapsed(1)), "blast");
assert.equal(nuclearPhase(nuclearElapsed(8.74)), "blast");
assert.equal(nuclearPhase(nuclearElapsed(8.75)), "complete");
for (let age = -1; age < 8; age += 0.001) {
  const shake = nuclearShake(age);
  assert.ok(Math.hypot(...shake) <= 1.5, "Shake is bounded in CSS pixels");
  if (age < 0 || age >= 0.35) {
    assert.deepEqual(shake, [0, 0]);
  }
}
console.log("Nuclear geometry, placement detection, timeline, and shake bounds verified.");

const outgoing: TileDto = {
  coordinates: { q: 0, r: 0, s: 0 },
  location: "mars",
  type: "land",
  tags: [],
  bonuses: [],
  occupiedBy: { type: "city-tile", tags: [], visual: { seed: 42 } },
};
const incoming = { ...outgoing, occupiedBy: { type: "nuclear-zone-tile", tags: [] } };
const key = "0,0,0";
let transitions = reconcileNuclearTransitions(
  { gameId: "game", tiles: [], serial: 0, active: new Map() },
  [incoming],
  "game",
  new Set([key]),
  new Map([[key, outgoing]]),
  10,
);
const first = transitions.active.get(key)!;
assert.equal(first.outgoing, outgoing, "Retain the exact appearance instead of regenerating it");
const impactTime = first.start + NUCLEAR_FLIGHT_SECONDS / NUCLEAR_PLAYBACK_RATE;
const collapseEnd =
  first.start + (NUCLEAR_FLIGHT_SECONDS + NUCLEAR_COLLAPSE_SECONDS) / NUCLEAR_PLAYBACK_RATE;
const effectEnd = first.start + NUCLEAR_DURATION_SECONDS / NUCLEAR_PLAYBACK_RATE;
assert.equal(advanceNuclearTransitions(transitions.active, impactTime - 0.01), transitions.active);
let stages = advanceNuclearTransitions(transitions.active, impactTime + 0.01);
assert.equal(stages.get(key)!.impacted, true);
assert.equal(stages.get(key)!.collapsed, false);
assert.equal(stages.get(key)!.outgoing, outgoing);
assert.equal(advanceNuclearTransitions(stages, collapseEnd - 0.01), stages);
stages = advanceNuclearTransitions(stages, collapseEnd + 0.01);
assert.equal(stages.get(key)!.collapsed, true);
assert.equal(stages.get(key)!.outgoing, undefined, "Release outgoing snapshots after sinking");
assert.equal(advanceNuclearTransitions(stages, effectEnd - 0.01), stages);
assert.equal(advanceNuclearTransitions(stages, effectEnd).size, 0);
transitions = reconcileNuclearTransitions(
  transitions,
  [{ ...incoming }],
  "game",
  new Set(),
  new Map(),
  10.4,
);
assert.equal(transitions.active.get(key), first, "Unrelated updates must not restart flight");
const other = { ...incoming, coordinates: { q: 1, r: 0, s: -1 } };
const simultaneous = reconcileNuclearTransitions(
  transitions,
  [incoming, other],
  "game",
  new Set(["1,0,-1"]),
  new Map(),
  10.4,
);
assert.equal(simultaneous.active.size, 2);
assert.equal(simultaneous.active.get(key)!.start, 10);
assert.equal(simultaneous.active.get("1,0,-1")!.start, 10.4);
assert.equal(
  reconcileNuclearTransitions(transitions, [outgoing], "game", new Set(), new Map(), 11).active
    .size,
  0,
  "Another replacement cancels retained visuals",
);
assert.equal(
  reconcileNuclearTransitions(transitions, [incoming], "other-game", new Set(), new Map(), 11)
    .active.size,
  0,
);
assert.equal(
  reconcileNuclearTransitions(
    { gameId: "game", tiles: undefined, serial: 0, active: new Map() },
    [incoming],
    "game",
    new Set(),
    new Map(),
    11,
  ).active.size,
  0,
  "Reconnect does not replay",
);

for (let seed = 0; seed < 100; seed++) {
  const part = collapseParameters(seed);
  assert.ok(part.delay >= 0 && part.delay <= 0.12);
  assert.ok(part.tilt >= (2 * Math.PI) / 180 && part.tilt <= (6 * Math.PI) / 180);
  assert.equal(collapseProgress(-0.001, part.delay), 0);
  assert.equal(collapseProgress(0, part.delay), 0);
  assert.equal(collapseProgress(0.9, part.delay), 1);
  const profile = nuclearProfile(seed);
  assert.equal(profile, nuclearProfile(seed));
  assert.ok(profile.aspect >= 1.08 && profile.aspect <= 1.2);
  assert.equal(profile.gouges.length, 5);
  assert.equal(profile.fans.length, 3);
  assert.ok(profile.breaches.length >= 3 && profile.breaches.length <= 5);
  assert.ok(Math.max(...profile.rimHeights) >= 0.01155);
  assert.ok(Math.max(...profile.rimHeights) <= 0.015675);
  assert.ok(
    Math.max(...profile.rimHeights) - Math.min(...profile.rimHeights) > 0.002,
    "Crest has visible collapsed sections",
  );
  let peaks = 0;
  for (let i = 0; i < profile.radii.length; i++) {
    assert.ok(profile.radii[i] >= 0.079 && profile.radii[i] <= 0.121);
    if (
      profile.radii[i] > profile.radii[(i + 127) % 128] &&
      profile.radii[i] > profile.radii[(i + 1) % 128]
    ) {
      peaks++;
    }
    assert.ok(
      Math.abs(profile.radii[i] - profile.radii[(i + 1) % 128]) < 0.004,
      "No needle-like tips",
    );
    const angle = (i * Math.PI * 2) / profile.radii.length;
    assert.ok(
      nuclearMoundRadius(angle, seed) > profile.radii[i] + 0.005,
      "Every crest has an outer slope",
    );
  }
  assert.ok(peaks <= 9, "Broad irregular rim must not become a starburst");
  assert.notDeepEqual(profile.radii, nuclearProfile(seed + 1).radii);
}

const coordinates = { q: 0, r: 0, s: 0 };
const neighbors = [
  { q: 1, r: 0, s: -1 },
  { q: 1, r: -1, s: 0 },
  { q: 0, r: -1, s: 1 },
  { q: -1, r: 0, s: 1 },
  { q: -1, r: 1, s: 0 },
  { q: 0, r: 1, s: -1 },
];
const board = [
  { coordinate: coordinates, kind: "nuclear-zone" },
  ...neighbors.map((coordinate) => ({ coordinate, kind: "empty" })),
];
let spillCount = 0;
for (let seed = 0; seed < 30; seed++) {
  const site = createNuclearSites(board, `debris-${seed}`, EMPTY_LANDSCAPE).get(key)!;
  const fragments = createNuclearFragments(site);
  assert.deepEqual(fragments, createNuclearFragments(site), "Landing pattern is deterministic");
  assert.ok(fragments.length >= 120 && fragments.length <= 144);
  assert.equal(fragments.filter((fragment) => fragment.zone === "rim").length, 120);
  const matrix = new THREE.Matrix4(),
    scratch = debrisScratch();
  for (const fragment of fragments) {
    const profile = nuclearProfile(site.seed);
    const dx = fragment.x - profile.center.x,
      dy = fragment.y - profile.center.y;
    assert.ok(
      Math.hypot(dx, dy) >= nuclearRadius(Math.atan2(dy, dx), site.seed) * 0.99 - 1e-10,
      "The dark crater interior stays free of rocks",
    );
    assert.ok(Math.hypot(fragment.x, fragment.y) < 0.202);
    assert.ok(site.ground(fragment.x, fragment.y).coverage >= 0.99);
    assert.ok(site.ground(fragment.x, fragment.y).borderDistance >= 0.006 + fragment.size * 0.5);
    assert.ok(
      fragment.end.length() <
        2.02 + nuclearGroundHeight(fragment.x, fragment.y, site.seed, site.ground),
    );
    nuclearFragmentMatrix(fragment, -0.001, matrix, scratch);
    assert.equal(matrix.determinant(), 0, "Debris is hidden before impact");
    nuclearFragmentMatrix(fragment, NUCLEAR_DEBRIS_SECONDS, matrix, scratch);
    assert.ok(new THREE.Vector3().setFromMatrixPosition(matrix).distanceTo(fragment.end) < 1e-10);
    const settled = matrix.clone();
    nuclearFragmentMatrix(fragment, Infinity, matrix, scratch);
    assert.deepEqual(matrix, settled, "Reconnect uses exactly the settled burst pose");
    assert.ok(fragment.delay + fragment.duration <= NUCLEAR_DEBRIS_SECONDS);
    const normal = nuclearGroundNormal(site, fragment.x, fragment.y, new THREE.Vector3());
    const rockUp = new THREE.Vector3(0, 0, 1).applyQuaternion(fragment.rotation);
    assert.ok(normal.dot(rockUp) > 0.98, "Rubble follows the actual wall slope");
    spillCount += Number(fragment.zone === "spill");
  }
}
assert.ok(spillCount > 0, "Some fragments must escape the hex");
const tiltedSite = createNuclearSites(
  [{ coordinate: neighbors[0], kind: "nuclear-zone" }],
  "tilted",
  EMPTY_LANDSCAPE,
)
  .values()
  .next().value!;
tiltedSite.ground = (x, y) => ({
  height: 0.003 + x * 0.03 + y * 0.02,
  coverage: 1,
  borderDistance: 1,
});
const tiltedGeometry = createNuclearGeometry(tiltedSite.seed, tiltedSite.ground, tiltedSite);
for (const geometry of Object.values(tiltedGeometry)) {
  const positions = geometry.getAttribute("position");
  const directions = geometry.getAttribute("nuclearPlanetDirection");
  for (let i = 0; i < positions.count; i++) {
    const direction = nuclearDirection(
      tiltedSite,
      positions.getX(i),
      positions.getY(i),
      new THREE.Vector3(),
    );
    assert.ok(
      direction.distanceTo(new THREE.Vector3().fromBufferAttribute(directions, i)) < 1e-7,
      "Texture projection and debris agree away from the planet pole",
    );
  }
}
const tiltedApron = tiltedGeometry.scorch.getAttribute("position");
for (let i = 0; i <= NUCLEAR_SEGMENTS; i++) {
  const ground = tiltedSite.ground(tiltedApron.getX(i), tiltedApron.getY(i));
  assert.ok(
    Math.abs(tiltedApron.getZ(i) - ground.height - 0.0006) < 1e-7,
    "Outer mound meets elevated surrounding terrain",
  );
}
Object.values(tiltedGeometry).forEach((geometry) => geometry.dispose());
for (const kind of ["city", "ocean", "special", "nuclear-zone"]) {
  const blocked = createNuclearSites(
    [board[0], ...neighbors.map((coordinate) => ({ coordinate, kind }))],
    "blocked",
    EMPTY_LANDSCAPE,
  ).get(key)!;
  assert.equal(blocked.ground(0.18, 0).coverage, 0, `${kind} must reject spill`);
  assert.ok(createNuclearFragments(blocked).every((fragment) => fragment.zone !== "spill"));
}
const isolatedSite = createNuclearSites([board[0]], "isolated", EMPTY_LANDSCAPE).get(key)!;
assert.ok(
  createNuclearFragments(isolatedSite).every((fragment) => fragment.zone !== "spill"),
  "No fragments outside the board",
);
assert.equal(createNuclearSites([], "empty", EMPTY_LANDSCAPE).size, 0);
const doubleSites = createNuclearSites(
  [board[0], { coordinate: neighbors[0], kind: "nuclear-zone" }],
  "double",
  EMPTY_LANDSCAPE,
);
assert.equal(doubleSites.size, 2);
const batchGeometry = new THREE.BoxGeometry(0.04, 0.04, 0.04);
const batchMaterial = new THREE.MeshLambertMaterial();
const debrisBatch = new THREE.InstancedMesh(batchGeometry, batchMaterial, 1024);
let instanceWrites = 0;
const setMatrix = debrisBatch.setMatrixAt.bind(debrisBatch);
debrisBatch.setMatrixAt = (index, matrix) => {
  instanceWrites++;
  setMatrix(index, matrix);
};
const debrisBuffer = new NuclearDebrisBuffer(doubleSites);
const simultaneousDebris = new Map(
  [...doubleSites.keys()].map((tileKey, i) => [
    tileKey,
    { id: i, start: 10 + i * 0.2, impacted: false, collapsed: false },
  ]),
);
debrisBuffer.write(debrisBatch, simultaneousDebris, 10, true);
assert.ok(debrisBatch.count >= 240, "Multiple craters occupy the same instance batch");
const initializedWrites = instanceWrites;
debrisBuffer.write(debrisBatch, simultaneousDebris, 10.1);
assert.equal(instanceWrites, initializedWrites, "No uploads while waiting for impact");
debrisBuffer.write(debrisBatch, simultaneousDebris, 11.5);
assert.ok(instanceWrites > initializedWrites, "Only active bursts update poses");
debrisBuffer.write(debrisBatch, simultaneousDebris, 13);
const settledVersion = debrisBatch.instanceMatrix.version,
  settledWrites = instanceWrites;
for (let frame = 0; frame < 120; frame++) {
  debrisBuffer.write(debrisBatch, simultaneousDebris, 14 + frame / 60);
}
assert.equal(instanceWrites, settledWrites, "Settled debris must perform no instance writes");
assert.equal(
  debrisBatch.instanceMatrix.version,
  settledVersion,
  "Settled debris must stop GPU uploads",
);
new NuclearDebrisBuffer(new Map()).write(debrisBatch, new Map(), 20, true);
assert.equal(debrisBatch.count, 0, "Removing all craters releases every draw instance");
let disposed = 0;
debrisBatch.addEventListener("dispose", () => disposed++);
debrisBatch.dispose();
assert.equal(disposed, 1);
batchGeometry.dispose();
batchMaterial.dispose();
const fakeRock = new THREE.Group();
const sourceGeometry = new THREE.BoxGeometry(2, 3, 4);
fakeRock.add(new THREE.Mesh(sourceGeometry, new THREE.MeshBasicMaterial()));
const preparedRock = sharedRockGeometry(fakeRock);
assert.equal(
  sharedRockGeometry(fakeRock),
  preparedRock,
  "All renderers share one prepared rock geometry",
);
assert.equal(sourceGeometry.boundingBox, null, "Model geometry stays untouched");
assert.ok(Math.abs(preparedRock.boundingBox!.min.z) < 1e-9);
assert.ok(
  Math.abs(Math.max(...preparedRock.boundingBox!.getSize(new THREE.Vector3()).toArray()) - 0.04) <
    1e-7,
);
preparedRock.dispose();
sourceGeometry.dispose();
console.log(
  "Seeded rubble grounding, border clearance, protected neighbors, impact timing and settled poses verified.",
);

const layout = generateStructureLayout(42);
const plot = {
  coordinate: outgoing.coordinates,
  layout,
  worldPosition: new THREE.Vector3(0, 0, 2.02),
  normal: new THREE.Vector3(0, 0, 1),
};
const groups = new Map<string, string>();
for (const geometry of createGeometry(layout, plot)) {
  if (!geometry) {
    continue;
  }
  const positions = geometry.getAttribute("position");
  const pivots = geometry.getAttribute("structureCollapsePivot");
  const motions = geometry.getAttribute("structureCollapseMotion");
  for (let i = 0; i < positions.count; i++) {
    const pivot = new THREE.Vector3().fromBufferAttribute(pivots, i);
    const motion = new THREE.Vector4().fromBufferAttribute(motions, i);
    const id = pivot.toArray().join(":");
    const params = motion.toArray().join(":");
    if (groups.has(id)) {
      assert.equal(
        groups.get(id),
        params,
        "Building attachments must share collapse motion across materials",
      );
    }
    groups.set(id, params);
    const position = new THREE.Vector3().fromBufferAttribute(positions, i).sub(pivot);
    const rotation = new THREE.Euler(motion.y, motion.z, 0, "YXZ");
    position.applyEuler(rotation).add(pivot);
    position.z -= motion.w;
    assert.ok(position.z < -0.03, "Every city vertex must finish below the crater floor");
  }
  geometry.dispose();
}
assert.ok(groups.size > 2, "Buildings must not collapse as one elevator");
const materials = new Map<string, THREE.MeshStandardMaterial>();
for (const mode of ["normal", "bright"]) {
  for (let i = 0; i < 19; i++) {
    materials.set(`${mode}:${i}`, new THREE.MeshStandardMaterial());
  }
}
const store = new StructureBatchStore(materials);
store.update([plot], new Map([[key, -1000]]));
const builds = store.builds;
const meshes = [...store.meshes.values()];
const instances = meshes.reduce((sum, mesh) => sum + mesh.instanceCount, 0);
store.setCollapses(transitions.active);
for (const mesh of meshes) {
  const starts = mesh.geometry.getAttribute("structureCollapseStart");
  assert.ok(
    Array.from(starts.array).some((value) => Math.abs(value - impactTime) < 1e-6),
    "City collapse must start at the slowed missile impact",
  );
}
store.update([plot], new Map([[key, -1000]]));
assert.equal(store.builds, builds, "Starting collapse must not rebuild the city");
assert.deepEqual([...store.meshes.values()], meshes, "Reuse existing draw batches");
assert.equal(
  meshes.reduce((sum, mesh) => sum + mesh.instanceCount, 0),
  instances,
);
store.update([], new Map());
assert.equal(
  meshes.reduce((sum, mesh) => sum + mesh.instanceCount, 0),
  0,
  "Release every outgoing batch instance",
);
store.dispose();
materials.forEach((material) => material.dispose());
console.log(
  "Nuclear replacement retention, collapse bounds, batch cleanup, and irregular rim verified.",
);

const { plugin } = await import("bun");
const { resolve, dirname } = await import("node:path");
await plugin({
  name: "nuclear-raw",
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
const { createNuclearCollapseMaterial } = await import("../../src/components/game/board/shaders");
const {
  addPlanetHaze,
  createPlanetHazeUniforms,
  receivesPlanetHaze,
  createWorldTreeMaterial,
  createVolcanoMaterial,
} = await import("../../src/components/game/board/shaders");
const texture = new THREE.Texture();
for (const source of [
  new THREE.MeshStandardMaterial(),
  createWorldTreeMaterial(1),
  createVolcanoMaterial(texture, texture, 1),
]) {
  const restoreHaze = addPlanetHaze(source, createPlanetHazeUniforms());
  const transform = { value: new THREE.Matrix4() };
  const material = createNuclearCollapseMaterial(
    source,
    transform,
    { value: new THREE.Matrix3() },
    { value: new THREE.Matrix3() },
  );
  assert.equal(
    receivesPlanetHaze(material),
    false,
    "A retained material must not receive a duplicate haze wrapper",
  );
  const raw = source instanceof THREE.ShaderMaterial;
  const shader = {
    vertexShader: raw ? source.vertexShader : THREE.ShaderLib.standard.vertexShader,
    fragmentShader: raw ? source.fragmentShader : THREE.ShaderLib.standard.fragmentShader,
    uniforms: raw ? { ...source.uniforms } : {},
  } as THREE.WebGLProgramParametersWithUniforms;
  material.onBeforeCompile(shader, {} as THREE.WebGLRenderer);
  assert.equal(shader.uniforms.uNuclearClipTransform, transform);
  assert.equal((shader.vertexShader.match(/void\s+main\s*\(/g) ?? []).length, 1);
  assert.equal((shader.vertexShader.match(/void\s+atmosphereSourceVertex\s*\(/g) ?? []).length, 1);
  assert.equal((shader.vertexShader.match(/void\s+nuclearOriginalMain\s*\(/g) ?? []).length, 1);
  if (source instanceof THREE.ShaderMaterial && material instanceof THREE.ShaderMaterial) {
    assert.equal(
      material.uniforms.uEmergence,
      source.uniforms.uEmergence,
      "Retained shaders must continue receiving the component's uniform updates",
    );
  }
  material.dispose();
  restoreHaze();
  source.dispose();
}
texture.dispose();
console.log(
  "Projected collapse shaders compose with lighting and haze without duplicate wrappers.",
);
