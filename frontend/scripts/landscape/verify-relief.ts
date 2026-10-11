import assert from "node:assert/strict";
import sharp from "sharp";
import * as THREE from "three";
import {
  createMarsRelief,
  createMarsReliefGeometry,
  createMarsReliefRaycast,
  marsReliefAtBoard,
  marsReliefAtDirection,
  marsReliefAtUv,
  MARS_RELIEF_DEPTH,
} from "../../src/components/game/board/marsRelief";
import { SPHERE_RADIUS } from "../../src/components/game/board/boardConstants";
import {
  WATER_LEVEL,
  LAKE_BED_LEVEL,
  basinHeight,
  BEACH_WIDTH_MIN,
  BEACH_WIDTH_MAX,
  LAKE_GROUND_REACH,
} from "../../src/components/game/board/landscapeFields";
import {
  nuclearGroundHeight,
  nuclearSurfaceHeight,
  nuclearProfile,
} from "../../src/components/game/board/nuclearGeometry";
import {
  createLandscapeSampler,
  LandscapeBuilder,
  landscapeHeightAt,
  FIELD_SIZE,
  FIELD_BORDER,
  FIELD_STEP,
} from "../../src/components/game/board/landscapeFields";
import { projectBoardPoint } from "../../src/components/game/board/landscapeGeometry";
import { generateStructureLayout } from "../../src/components/game/board/structureLayout";
import type { LandscapeInput } from "../../src/components/game/board/landscapeTypes";
import { createNuclearSites } from "../../src/components/game/board/nuclearDebris";

const { data, info } = await sharp(
  new URL("../../../assets/original/textures/planets/mars/surface.jpg", import.meta.url).pathname,
)
  .resize(512, 256)
  .ensureAlpha()
  .raw()
  .toBuffer({ resolveWithObject: true });
const relief = createMarsRelief(new Uint8ClampedArray(data), info.width, info.height);
assert.equal(WATER_LEVEL, -0.008, "Ocean elevation is independent of terrain relief strength");
assert.equal(LAKE_BED_LEVEL, -0.024, "Seabed elevation is independent of terrain relief strength");
assert.ok(LAKE_BED_LEVEL < WATER_LEVEL, "Lake beds remain submerged");
for (const width of [BEACH_WIDTH_MIN, BEACH_WIDTH_MAX]) {
  for (const height of [-0.06, -0.018, 0.0003, 0.027]) {
    assert.equal(basinHeight(0, width, height), WATER_LEVEL);
    assert.ok(
      basinHeight(width * 0.5, width, height) > WATER_LEVEL,
      "Dry beaches rise above water even alongside deep valleys",
    );
    assert.ok(
      Math.abs(basinHeight(LAKE_GROUND_REACH, width, height) - height) < 1e-9,
      "Banks rejoin the surrounding relief before the basin ends",
    );
    for (const edge of [0, width, LAKE_GROUND_REACH]) {
      assert.ok(
        Math.abs(
          basinHeight(edge - 1e-7, width, height) - basinHeight(edge + 1e-7, width, height),
        ) < 1e-7,
        "Shore transitions are continuous",
      );
    }
  }
}
const crater = nuclearProfile(42);
const deepGround = () => ({ height: -MARS_RELIEF_DEPTH + 0.0003, coverage: 1, borderDistance: 1 });
assert.ok(
  Math.abs(
    nuclearGroundHeight(crater.center.x, crater.center.y, 42, deepGround) -
      nuclearSurfaceHeight(crater.center.x, crater.center.y, 42) +
      MARS_RELIEF_DEPTH,
  ) < 1e-9,
  "Crater floors follow local terrain depth",
);
assert.ok(
  relief.depths.every((v) => Number.isFinite(v) && v >= 0 && v <= MARS_RELIEF_DEPTH + 1e-9),
);
assert.ok(
  relief.depths.some((v) => v > 0.003),
  "The source must produce visible, bounded valleys",
);
assert.ok(
  relief.depths.some((v) => v < 0.0001),
  "Bright plateaus keep the reference radius",
);
assert.deepEqual(createMarsRelief(new Uint8ClampedArray(data), info.width, info.height), relief);
for (let v = 0; v <= 1; v += 0.1) {
  assert.ok(Math.abs(marsReliefAtUv(relief, -1e-8, v) - marsReliefAtUv(relief, 1e-8, v)) < 1e-7);
}
for (const v of [0, 1]) {
  assert.equal(marsReliefAtUv(relief, 0.1, v), marsReliefAtUv(relief, 0.8, v));
}
const geometry = createMarsReliefGeometry(relief);
const positions = geometry.getAttribute("position"),
  uv = geometry.getAttribute("uv");
const point = new THREE.Vector3();
for (let i = 513; i < positions.count - 513; i += 71) {
  point.fromBufferAttribute(positions, i);
  const expected = SPHERE_RADIUS + marsReliefAtUv(relief, uv.getX(i), uv.getY(i));
  assert.ok(
    Math.abs(point.length() - expected) < 4e-7,
    "Geometry relief aligns with Mars texture UVs",
  );
}
const empty = createLandscapeSampler({ seed: 42, sources: [], relief });
for (let x = -1; x <= 1; x += 0.07) {
  for (let y = -1; y <= 1; y += 0.09) {
    projectBoardPoint(x, y, 0, point);
    const baseline = marsReliefAtDirection(relief, point.x, point.y, point.z);
    assert.ok(Math.abs(empty(x, y).height - baseline - 0.0003) < 1e-9);
    assert.ok(Math.abs(landscapeHeightAt(new Map(), x, y, relief) - baseline - 0.0003) < 1e-9);
  }
}
const coordinate = { q: 0, r: 0, s: 0 };
const input: LandscapeInput = {
  seed: 42,
  relief,
  sources: [{ coordinate, kind: "greenery", seed: 42 }],
};
const ground = createLandscapeSampler(input),
  flat = createLandscapeSampler({ ...input, relief: undefined });
for (const x of [-0.12, 0, 0.12]) {
  assert.ok(
    Math.abs(ground(x, 0).height - flat(x, 0).height - marsReliefAtBoard(relief, x, 0)) < 1e-9,
  );
}
const lake = { ...input, sources: [{ coordinate, kind: "ocean" as const, seed: 42 }] };
assert.equal(
  createLandscapeSampler(lake)(0, 0).height,
  createLandscapeSampler({ ...lake, relief: undefined })(0, 0).height,
  "Lake floors retain their level below fixed water",
);
const city = createLandscapeSampler({
  ...input,
  sources: [{ coordinate, kind: "structure", seed: 42, layout: generateStructureLayout(42) }],
});
assert.equal(city(0, 0).height, 0.0003, "City foundations remain level and grounded");
const builder = new LandscapeBuilder();
const patches = builder.build(input);
assert.deepEqual(
  builder.build(structuredClone(input)),
  patches,
  "Worker clones preserve the same height data and cache signatures",
);
for (const patch of patches.values()) {
  for (const plant of patch.plants) {
    assert.ok(
      Math.abs(landscapeHeightAt(patches, plant.x, plant.y, relief) - plant.height) < 0.0004,
      "Plants follow the rendered height field",
    );
  }
  for (let y = FIELD_BORDER; y < FIELD_SIZE - FIELD_BORDER; y += 29) {
    for (let x = FIELD_BORDER; x < FIELD_SIZE - FIELD_BORDER; x += 31) {
      const baseline = marsReliefAtBoard(
        relief,
        patch.x + (x - FIELD_BORDER) * FIELD_STEP,
        patch.y + (y - FIELD_BORDER) * FIELD_STEP,
      );
      const encoded = (-patch.detail[(y * FIELD_SIZE + x) * 4 + 3] / 255) * MARS_RELIEF_DEPTH;
      assert.ok(
        Math.abs(baseline - encoded) <= MARS_RELIEF_DEPTH / 510 + 1e-9,
        "Stencil depth tracks the same baseline",
      );
    }
  }
}
const flatPatches = builder.build({ ...input, relief: undefined });
assert.notEqual(
  flatPatches.values().next().value!.signature,
  patches.values().next().value!.signature,
  "Relief changes invalidate cached patches",
);
const darkRelief = { ...relief, depths: new Float32Array(relief.depths.length).fill(0.003) };
const sites = createNuclearSites([{ coordinate, kind: "nuclear-zone" }], "relief", {
  id: 1,
  seed: 42,
  relief: darkRelief,
  sources: [],
  patches: new Map(),
  plants: [],
});
assert.ok(
  sites.values().next().value!.ground(0, 0).height < 0.0003,
  "Crater skirts inherit relief even without landscape patches",
);
const mesh = new THREE.Mesh(geometry, new THREE.MeshBasicMaterial());
mesh.position.set(4, -2, 1);
mesh.rotation.set(0.2, 0.3, 0.1);
mesh.updateMatrixWorld(true);
mesh.raycast = createMarsReliefRaycast(relief);
for (const x of [-0.9, 0, 0.9]) {
  const local = projectBoardPoint(x, 0);
  const camera = local.clone().multiplyScalar(3).applyMatrix4(mesh.matrixWorld);
  const target = local.clone().applyMatrix4(mesh.matrixWorld);
  const hits = new THREE.Raycaster(camera, target.sub(camera).normalize()).intersectObject(mesh);
  assert.equal(hits.length, 1);
  const hit = mesh.worldToLocal(hits[0].point.clone());
  assert.ok(
    Math.abs(hit.length() - SPHERE_RADIUS - marsReliefAtDirection(relief, hit.x, hit.y, hit.z)) <
      1e-6,
    "Picking follows the rotated displaced surface",
  );
}
geometry.dispose();
mesh.material.dispose();
console.log(
  "Mars relief: UV alignment, bounds, seams, worker fields, plant grounding, level lakes/foundations, crater skirts, stencil depth, cache invalidation and picking verified.",
);
