import { sharedRockGeometry } from "./rockGeometry";
import {
  collapseParameters,
  collapseProgress,
  type NuclearTransitions,
} from "./nuclearTransitions";
import { nuclearElapsed, NUCLEAR_FLIGHT_SECONDS } from "./nuclearGeometry";
import { memo, useEffect, useMemo, useRef, useLayoutEffect, useState } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import * as THREE from "three";
import { mergeGeometries } from "three/examples/jsm/utils/BufferGeometryUtils.js";
import { useModels } from "../../../hooks/useModels";
import { useTextures } from "../../../hooks/useTextures";
import { useClimate, type ClimateRuntime } from "../../../contexts/ClimateContext";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext";
import { plantForm } from "./climate";
import type { PlantShadeBlob, PlantShadeMap } from "./plantShade";
import { performanceStore } from "../../../services/performanceStore";
import {
  greeneryGroundVertexSnippet,
  greeneryGroundFragmentSnippet,
  splitSnippet,
} from "./shaders";
import { SPHERE_RADIUS, easeOutCubic } from "./boardConstants";
import type { LandscapePlant, LandscapeState } from "./landscapeTypes";
import {
  landscapeHeightAt,
  landscapeShoreAt,
  lakeBlendWeight,
  TRANSITION_MS,
} from "./landscapeFields";

import { projectBoardPoint } from "./landscapeGeometry";
import { GRAPHICS } from "@/utils/graphicsQuality.ts";
export const TREE_NAMES = ["Tree-01-1", "Tree-01-2", "Tree-01-3", "Tree-01-4"];
export const PINE_NAMES = ["Pine4m"];
export const BUSH_NAMES = ["Bush-01", "Bush-02", "Bush-03", "Bush-04", "Bush-05"];
export const CLOVER_NAMES = ["Clover-01", "Clover-02", "Clover-03", "Clover-04", "Clover-05"];
export const FLOWER_NAMES = ["Circle003", "Circle008", "Circle015"];

// Shared by every plant material so frost follows the climate without recompiling.
const PLANT_CLIMATE_UNIFORMS = { uFrost: { value: 0 } };

export function addPlantFrost(material: THREE.Material) {
  material.onBeforeCompile = (shader) => {
    addInvariantPosition(shader);
    shader.uniforms.uFrost = PLANT_CLIMATE_UNIFORMS.uFrost;
    shader.vertexShader =
      "varying float vFrostUp;\n" +
      shader.vertexShader.replace(
        "#include <beginnormal_vertex>",
        "#include <beginnormal_vertex>\nvFrostUp=objectNormal.z;",
      );
    shader.fragmentShader =
      "uniform float uFrost;\nvarying float vFrostUp;\n" +
      shader.fragmentShader
        .replace(
          "#include <map_fragment>",
          "#include <map_fragment>\ndiffuseColor.rgb=mix(diffuseColor.rgb,vec3(0.86,0.9,0.95),uFrost*smoothstep(0.1,0.7,vFrostUp)*0.85);",
        )
        .replace(
          "#include <emissivemap_fragment>",
          "#include <emissivemap_fragment>\ntotalEmissiveRadiance*=1.0-uFrost*0.6;",
        );
  };
  material.customProgramCacheKey = () => "plant-frost";
}

const FOLIAGE_FILL = 0.5;
// Rock textures are already light, so they need far less fill than leaves.
const ROCK_FILL = 0.15;

// Both foliage passes must produce bit-identical depth for the Equal test of the shading pass.
function addInvariantPosition(shader: { vertexShader: string }) {
  shader.vertexShader = "invariant gl_Position;\n" + shader.vertexShader;
}

// Foliage is thousands of overlapping alpha cards. A depth pre-pass (the material's
// `userData.depthMaterial`) resolves the alpha cut-out first; the Lambert shading pass then only
// runs for the front-most card of each pixel (Equal depth, no discard, so hidden layers are
// rejected before shading).
export function createFoliageMaterial(
  map: THREE.Texture | null,
  color: THREE.Color,
  side: THREE.Side,
) {
  const material = new THREE.MeshLambertMaterial({
    map,
    color,
    side,
    depthFunc: THREE.EqualDepth,
    depthWrite: false,
    // Stands in for the environment lighting Lambert skips: an ambient fill of the plant's own colour.
    emissive: new THREE.Color(1, 1, 1),
    emissiveIntensity: FOLIAGE_FILL,
    emissiveMap: map,
    polygonOffset: true,
    polygonOffsetFactor: 1,
    polygonOffsetUnits: 1,
  });
  material.userData.planetHaze = true;
  addPlantFrost(material);
  const depth = new THREE.MeshBasicMaterial({
    map,
    side,
    alphaTest: 0.15,
    alphaToCoverage: GRAPHICS.antialias,
    colorWrite: false,
    polygonOffset: true,
    polygonOffsetFactor: 1,
    polygonOffsetUnits: 1,
  });
  depth.onBeforeCompile = addInvariantPosition;
  depth.customProgramCacheKey = () => "plant-depth";
  material.userData.depthMaterial = depth;
  material.addEventListener("dispose", () => depth.dispose());
  return material;
}

export function createRockMaterial(map: THREE.Texture) {
  const material = new THREE.MeshLambertMaterial({
    map,
    emissive: new THREE.Color(1, 1, 1),
    emissiveIntensity: ROCK_FILL,
    emissiveMap: map,
  });
  material.userData.planetHaze = true;
  addPlantFrost(material);
  return material;
}

export interface TreePrimitive {
  geometry: THREE.BufferGeometry;
  material: THREE.Material;
}

export interface TreeVariant {
  primitives: TreePrimitive[];
}

// Module-level cache for shared geometry/materials
export const variantCache: {
  trees: TreeVariant[] | null;
  pines: TreeVariant[] | null;
  bushes: TreeVariant[] | null;
  clover: TreeVariant[] | null;
  flowers: TreeVariant[] | null;
  rock: { geometry: THREE.BufferGeometry; material: THREE.Material } | null;
} = {
  trees: null,
  pines: null,
  bushes: null,
  clover: null,
  flowers: null,
  rock: null,
};

export function addSphereProjectionWithSoftEdges(
  material: THREE.Material,
  zOffset: number,
  noiseMap: THREE.Texture,
  noiseMapHigh: THREE.Texture,
  hexRadius: number,
  sphereCenter?: THREE.Vector3,
  groupInverseMatrix?: THREE.Matrix4,
  edges: {
    circular?: boolean;
    overflow?: number;
    bandWidth?: number;
    warp?: number;
    sphereRadius?: number;
  } = {},
): void {
  const grassOverflow = hexRadius * (edges.overflow ?? 0.25);
  const bandWidth = hexRadius * (edges.bandWidth ?? 0.35);
  const warpAmount = hexRadius * (edges.warp ?? 0.2);
  const noiseScale = 1.5 / hexRadius;
  const centerVec = sphereCenter || new THREE.Vector3(0, 0, 0);
  const invMatrix = groupInverseMatrix || new THREE.Matrix4();

  const vertSnippet = splitSnippet(greeneryGroundVertexSnippet);
  const fragSnippet = splitSnippet(greeneryGroundFragmentSnippet);

  material.onBeforeCompile = (shader) => {
    (material as any).__shader = shader;
    shader.uniforms.uSphereRadius = { value: edges.sphereRadius ?? SPHERE_RADIUS };
    shader.uniforms.uZOffset = { value: zOffset };
    shader.uniforms.uSphereCenter = { value: centerVec };
    shader.uniforms.uGroupInverseMatrix = { value: invMatrix };
    shader.uniforms.uNoiseMap = { value: noiseMap };
    shader.uniforms.uNoiseMapHigh = { value: noiseMapHigh };
    shader.uniforms.uHexRadius = { value: hexRadius };
    shader.uniforms.uCircularEdge = { value: edges.circular ?? false };
    shader.uniforms.uGrassOverflow = { value: grassOverflow };
    shader.uniforms.uBandWidth = { value: bandWidth };
    shader.uniforms.uWarpAmount = { value: warpAmount };
    shader.uniforms.uNoiseScale = { value: noiseScale };
    shader.uniforms.uFadeProgress = { value: 1.0 };

    shader.vertexShader =
      vertSnippet.header +
      "\n" +
      shader.vertexShader.replace("#include <begin_vertex>", vertSnippet.body).replace(
        "#include <project_vertex>",
        `vec4 mvPosition = viewMatrix * vec4(projectedPos, 1.0);
           gl_Position = projectionMatrix * mvPosition;`,
      );

    shader.fragmentShader =
      fragSnippet.header +
      "\n" +
      shader.fragmentShader.replace("#include <alphamap_fragment>", fragSnippet.body);
  };
}

export function createVariantsFromScene(
  scene: THREE.Object3D,
  names: string[],
  targetHeight: number,
): TreeVariant[] {
  const variants: TreeVariant[] = [];

  for (const name of names) {
    const obj = scene.getObjectByName(name);
    if (!obj) continue;

    const primitives: TreePrimitive[] = [];
    const allGeometries: THREE.BufferGeometry[] = [];

    obj.traverse((child) => {
      if (child instanceof THREE.Mesh) {
        child.updateWorldMatrix(true, false);
        const baseGeo = child.geometry.clone();
        baseGeo.applyMatrix4(child.matrixWorld);

        if (Array.isArray(child.material)) {
          const groups = child.geometry.groups;
          child.material.forEach((mat, idx) => {
            if (groups && groups[idx]) {
              const group = groups[idx];
              const geo = baseGeo.clone();
              const indices = baseGeo.index!.array.slice(group.start, group.start + group.count);
              geo.setIndex(Array.from(indices));
              primitives.push({ geometry: geo, material: mat.clone() });
              allGeometries.push(geo.clone());
            }
          });
        } else {
          primitives.push({
            geometry: baseGeo,
            material: (child.material as THREE.Material).clone(),
          });
          allGeometries.push(baseGeo.clone());
        }
      }
    });

    if (primitives.length === 0) continue;

    const mergedForBounds = mergeGeometries(allGeometries);
    if (!mergedForBounds) continue;

    const rotationMatrix = new THREE.Matrix4().makeRotationX(Math.PI / 2);
    mergedForBounds.applyMatrix4(rotationMatrix);

    const box = new THREE.Box3().setFromBufferAttribute(
      mergedForBounds.getAttribute("position") as THREE.BufferAttribute,
    );
    const size = box.getSize(new THREE.Vector3());
    const center = box.getCenter(new THREE.Vector3());

    const maxDim = Math.max(size.x, size.y, size.z);
    const scale = targetHeight / maxDim;

    const transform = new THREE.Matrix4()
      .makeScale(scale, scale, scale)
      .multiply(new THREE.Matrix4().makeTranslation(-center.x, -center.y, -box.min.z))
      .multiply(rotationMatrix);

    primitives.forEach((p) => {
      p.geometry.applyMatrix4(transform);
      p.geometry.computeVertexNormals();
      const source = p.material as THREE.MeshStandardMaterial;
      if (source.map) {
        source.map.minFilter = THREE.LinearMipmapLinearFilter;
        source.map.magFilter = THREE.LinearFilter;
        source.map.anisotropy = 4;
      }
      p.material = createFoliageMaterial(source.map, source.color, source.side);
      source.dispose();
    });

    variants.push({ primitives });
  }

  return variants;
}

function GreeneryRenderer({
  landscape,
  previousLandscape,
  transitionStart,
  nuclearTransitions = EMPTY_NUCLEAR_TRANSITIONS,
  nuclearTiles = EMPTY_NUCLEAR_TILES,
  shade,
}: {
  shade?: PlantShadeMap;
  nuclearTransitions?: NuclearTransitions;
  nuclearTiles?: ReadonlySet<string>;
  landscape: LandscapeState;
  previousLandscape: LandscapeState;
  transitionStart: number;
}) {
  const { treesScene, pinesScene, rockScene, flowersScene } = useModels();
  const { rock: rockTexture } = useTextures();
  const { runtime, target } = useClimate();
  const gl = useThree((s) => s.gl);
  const { settings } = useWorld3DSettings();
  const baked = useRef({
    groups: null as unknown,
    size: -1,
    tiles: null as unknown,
    sx: NaN,
    sy: NaN,
    sz: NaN,
  });
  const blobs = useRef<PlantShadeBlob[]>([]);
  const gates = useMemo(
    () => ({
      tree: quantize(target.tree),
      pine: 1,
      bush: quantize(target.bush),
      clover: quantize(target.grass),
      flower: quantize(target.flower),
      rock: 1,
    }),
    [target.tree, target.bush, target.grass, target.flower],
  );
  useFrame(() => {
    if (performanceStore.sectionDue("Plants")) {
      const totals = new Map<string, { active: number; drawn: number; retiring: number }>();
      for (const { kind, active, drawn, retiring } of batchStats.values()) {
        const total = totals.get(kind) ?? { active: 0, drawn: 0, retiring: 0 };
        total.active += active;
        total.drawn += drawn;
        total.retiring += retiring;
        totals.set(kind, total);
      }
      const section: Record<string, string> = {};
      for (const [kind, t] of totals) {
        section[kind] = `${t.active} active · ${t.drawn} drawn · ${t.retiring} retiring`;
      }
      performanceStore.setSection("Plants (instances summed over mesh parts)", section);
    }
    PLANT_CLIMATE_UNIFORMS.uFrost.value = runtime.current.current.plantFrost;
  });
  // Create variants once (cached)
  const treeVariants = useMemo(() => {
    if (!variantCache.trees) {
      variantCache.trees = createVariantsFromScene(treesScene, TREE_NAMES, 0.08);
    }
    return variantCache.trees;
  }, [treesScene]);

  const pineVariants = useMemo(() => {
    if (!variantCache.pines) {
      variantCache.pines = createVariantsFromScene(pinesScene, PINE_NAMES, 0.075);
    }
    return variantCache.pines;
  }, [pinesScene]);

  const bushVariants = useMemo(() => {
    if (!variantCache.bushes) {
      variantCache.bushes = createVariantsFromScene(treesScene, BUSH_NAMES, 0.035);
    }
    return variantCache.bushes;
  }, [treesScene]);

  const cloverVariants = useMemo(() => {
    if (!variantCache.clover) {
      variantCache.clover = createVariantsFromScene(treesScene, CLOVER_NAMES, 0.012);
    }
    return variantCache.clover;
  }, [treesScene]);

  const flowerVariants = useMemo(() => {
    if (!variantCache.flowers) {
      variantCache.flowers = createVariantsFromScene(flowersScene, FLOWER_NAMES, 0.025);
    }
    return variantCache.flowers;
  }, [flowersScene]);

  const { geometry: rockGeometry, material: rockMaterial } = useMemo(() => {
    if (variantCache.rock) {
      return variantCache.rock;
    }

    const geo = sharedRockGeometry(rockScene);

    variantCache.rock = { geometry: geo, material: createRockMaterial(rockTexture) };
    return variantCache.rock;
  }, [rockScene, rockTexture]);

  const groups = useMemo(() => {
    const result = new Map<string, LandscapePlant[]>();
    for (const p of landscape.plants) {
      if (p.rank > gates[p.kind]) {
        continue;
      }
      if (!keptAtFoliageDensity(p.seed)) {
        continue;
      }
      const counts = {
        tree: TREE_NAMES.length,
        pine: PINE_NAMES.length,
        bush: BUSH_NAMES.length,
        clover: CLOVER_NAMES.length,
        flower: FLOWER_NAMES.length,
        rock: 1,
      };
      const key = `${p.kind}:${p.seed % counts[p.kind]}`;
      const list = result.get(key) ?? [];
      list.push(p);
      result.set(key, list);
    }
    return result;
  }, [landscape, gates]);
  useFrame(() => {
    if (!shade) {
      return;
    }
    const climate = runtime.current.current;
    const last = baked.current;
    const { sunDirectionX: sx, sunDirectionY: sy, sunDirectionZ: sz } = settings;
    if (
      !shade.contentLost &&
      last.groups === groups &&
      last.tiles === nuclearTiles &&
      last.sx === sx &&
      last.sy === sy &&
      last.sz === sz &&
      Math.abs(climate.treeScale - last.size) < 0.002
    ) {
      return;
    }
    baked.current = {
      groups,
      tiles: nuclearTiles,
      size: climate.treeScale,
      sx,
      sy,
      sz,
    };
    // Shadows fall away from the sun, longer the lower it sits above the board.
    const across = Math.hypot(sx, sy);
    const slope = Math.min(2, across / Math.max(Math.abs(sz), 0.05));
    const shadowX = across > 1e-4 ? -sx / across : 1;
    const shadowY = across > 1e-4 ? -sy / across : 0;
    const list = blobs.current;
    list.length = 0;
    for (const plants of groups.values()) {
      for (const p of plants) {
        if (nuclearTiles.has(p.tileKey)) {
          continue;
        }
        const form = plantForm(p.kind, climate);
        list.push({
          x: p.x,
          y: p.y,
          radius: SHADE_RADIUS[p.kind] * p.scale * form.width,
          strength: SHADE_STRENGTH[p.kind],
          length: PLANT_HEIGHT[p.kind] * p.scale * form.height * slope,
        });
      }
    }
    shade.bake(gl, list, shadowX, shadowY);
  });
  const variants = [
    ...treeVariants.map((v, i) => ({ kind: "tree", i, v })),
    ...bushVariants.map((v, i) => ({ kind: "bush", i, v })),
    ...cloverVariants.map((v, i) => ({ kind: "clover", i, v })),
    ...flowerVariants.map((v, i) => ({ kind: "flower", i, v })),
    ...pineVariants.map((v, i) => ({ kind: "pine", i, v })),
    { kind: "rock", i: 0, v: { primitives: [{ geometry: rockGeometry, material: rockMaterial }] } },
  ];
  return (
    <group>
      {variants.flatMap(({ kind, i, v }) =>
        v.primitives.map((p, j) => (
          <PlantBatch
            key={`${kind}:${i}:${j}`}
            kind={kind}
            climate={runtime}
            geometry={p.geometry}
            material={p.material}
            plants={groups.get(`${kind}:${i}`) ?? EMPTY_PLANTS}
            landscape={landscape}
            transitionStart={transitionStart}
            previousLandscape={previousLandscape}
            nuclearTransitions={nuclearTransitions}
            nuclearTiles={nuclearTiles}
          />
        )),
      )}
    </group>
  );
}
// Thins plants by a hash of their seed, so the same plants are kept on every render.
function keptAtFoliageDensity(seed: number) {
  if (GRAPHICS.foliageDensity >= 1) {
    return true;
  }
  let h = Math.imul(seed ^ (seed >>> 16), 0x45d9f3b);
  h = Math.imul(h ^ (h >>> 16), 0x45d9f3b);
  h ^= h >>> 16;
  return (h >>> 0) / 4294967296 < GRAPHICS.foliageDensity;
}
const EMPTY_NUCLEAR_TRANSITIONS: NuclearTransitions = new Map();
const EMPTY_NUCLEAR_TILES: ReadonlySet<string> = new Set();
const EMPTY_PLANTS: LandscapePlant[] = [];
const UP = new THREE.Vector3(0, 0, 1);
function quantize(value: number) {
  return Math.round(value * 20) / 20;
}
// Shade footprints in board units at plant scale 1, slightly wider than the canopy.
const SHADE_RADIUS: Record<LandscapePlant["kind"], number> = {
  tree: 0.03,
  pine: 0.026,
  bush: 0.016,
  clover: 0.006,
  flower: 0.008,
  rock: 0.018,
};
// Approximate plant heights in board units at scale 1, matching createVariantsFromScene targets.
const PLANT_HEIGHT: Record<LandscapePlant["kind"], number> = {
  tree: 0.08,
  pine: 0.075,
  bush: 0.035,
  clover: 0.012,
  flower: 0.025,
  rock: 0.03,
};
const SHADE_STRENGTH: Record<LandscapePlant["kind"], number> = {
  tree: 0.8,
  pine: 0.75,
  bush: 0.6,
  clover: 0.3,
  flower: 0.25,
  rock: 0.55,
};
// Rough plant heights in board units, used to sink bases into sloped ground.
const RETIRE_MS = 1100;
// Per-batch instance counts for the performance window, keyed by batch.
const batchStats = new Map<
  object,
  { kind: string; active: number; drawn: number; retiring: number }
>();
const BATCH_CAPACITY = 4096;
// All pines share one model, so their single batch must hold every pine on a full board.
const PINE_CAPACITY = 16384;
// Culling is redone only when the view moves noticeably (Mars drifts along its orbit every frame);
// the margin keeps plants near the screen edge from popping between re-culls.
const CULL_VIEW_TOLERANCE = 2e-3;
const CULL_MARGIN = 0.03;
const BASE_SINK: Record<string, number> = { tree: 0.0024, pine: 0.0022, bush: 0.001 };
function PlantBatch({
  kind,
  climate,
  geometry,
  material,
  plants,
  landscape,
  transitionStart,
  previousLandscape,
  nuclearTransitions,
  nuclearTiles,
}: {
  kind: string;
  climate: React.RefObject<ClimateRuntime>;
  nuclearTransitions: NuclearTransitions;
  nuclearTiles: ReadonlySet<string>;
  geometry: THREE.BufferGeometry;
  material: THREE.Material;
  plants: LandscapePlant[];
  landscape: LandscapeState;
  transitionStart: number;
  previousLandscape: LandscapeState;
}) {
  const clock = useThree((s) => s.clock);
  const revision = landscape.id;
  const capacity = kind === "pine" ? PINE_CAPACITY : BATCH_CAPACITY;
  const statsKey = useMemo(() => ({}), []);
  useEffect(() => () => void batchStats.delete(statsKey), [statsKey]);
  const mesh = useRef<THREE.InstancedMesh>(null);
  // Foliage depth pre-pass: draws the same instance buffer, so it shares the main mesh's matrices.
  const depthMesh = useRef<THREE.InstancedMesh>(null);
  const depthMaterial = material.userData.depthMaterial as THREE.Material | undefined;
  useLayoutEffect(() => {
    if (depthMesh.current && mesh.current) {
      depthMesh.current.instanceMatrix = mesh.current.instanceMatrix;
      depthMesh.current.count = mesh.current.count;
    }
  }, [depthMaterial, geometry]);
  const slots = useRef(new Map<string, { slot: number; plant: LandscapePlant }>());
  const animations = useRef(
    new Map<
      number,
      {
        plant: LandscapePlant;
        fromHeight: number;
        birth: number;
        grow: boolean;
        shores: { previous: number; next: number } | null;
      }
    >(),
  );
  const seen = useRef(false);
  const retiring = useRef(new Map<number, { plant: LandscapePlant; start: number }>());
  const landscapeRevision = useRef(revision);
  const appliedSize = useRef(-1);
  const sized = kind === "tree" || kind === "pine" || kind === "bush";
  // Every instance lives here; only the ones inside the view are copied to the GPU buffer.
  const [source] = useState(() => ({
    matrices: new Float32Array(capacity * 16),
    colors: new Float32Array(capacity * 3),
    highest: 0,
    dirty: true,
    view: new Float32Array(16),
    frustum: new THREE.Frustum(),
    local: new THREE.Matrix4(),
    sphere: new THREE.Sphere(),
  }));
  const scratch = useMemo(
    () => ({
      matrix: new THREE.Matrix4(),
      q: new THREE.Quaternion(),
      turn: new THREE.Quaternion(),
      scale: new THREE.Vector3(),
      position: new THREE.Vector3(),
      normal: new THREE.Vector3(),
      axis: new THREE.Vector3(),
      color: new THREE.Color(),
    }),
    [],
  );
  const hide = (slot: number) => {
    source.matrices.fill(0, slot * 16, slot * 16 + 16);
    source.dirty = true;
  };
  const write = (
    slot: number,
    plant: LandscapePlant,
    growth: number,
    height: number,
    age?: number,
  ) => {
    projectBoardPoint(plant.x, plant.y, height, scratch.position);
    scratch.normal.copy(scratch.position).normalize();
    scratch.q.setFromUnitVectors(UP, scratch.normal);
    scratch.turn.setFromAxisAngle(UP, (plant.seed % 6283) / 1000);
    scratch.q.multiply(scratch.turn);
    if (age !== undefined) {
      const part = collapseParameters(plant.seed);
      const progress = collapseProgress(age, part.delay);
      const depth = geometry.boundingSphere!.radius * 2 * plant.scale + 0.065;
      scratch.position.addScaledVector(scratch.normal, -depth * progress);
      scratch.axis.set(Math.cos(part.direction), Math.sin(part.direction), 0);
      scratch.turn.setFromAxisAngle(scratch.axis, part.tilt * progress);
      scratch.q.multiply(scratch.turn);
    }
    const form = plantForm(kind, climate.current.current);
    const base = plant.scale * growth;
    scratch.position.addScaledVector(scratch.normal, -(BASE_SINK[kind] ?? 0) * base);
    scratch.scale.set(base * form.width, base * form.width, base * form.height);
    scratch.matrix.compose(scratch.position, scratch.q, scratch.scale);
    scratch.matrix.toArray(source.matrices, slot * 16);
    source.dirty = true;
  };
  const updateHighest = () => {
    let highest = 0;
    for (const { slot } of slots.current.values()) {
      highest = Math.max(highest, slot + 1);
    }
    for (const slot of retiring.current.keys()) {
      highest = Math.max(highest, slot + 1);
    }
    source.highest = highest;
    source.dirty = true;
  };
  const colorize = (slot: number, plant: LandscapePlant) => {
    const tint = 0.65 + (plant.seed % 100) / 400;
    source.colors[slot * 3] = source.colors[slot * 3 + 1] = source.colors[slot * 3 + 2] = tint;
    source.dirty = true;
  };
  useLayoutEffect(() => {
    if (!geometry.boundingSphere) {
      geometry.computeBoundingSphere();
    }
    const allowed = (p: LandscapePlant) =>
      !nuclearTiles.has(p.tileKey) ||
      (!!nuclearTransitions.get(p.tileKey)?.outgoing &&
        !nuclearTransitions.get(p.tileKey)?.collapsed);
    const ids = new Set(plants.filter(allowed).map((p) => p.id));
    // Climate changes keep the landscape revision; plants they reveal grow in now and plants they
    // remove shrink away, instead of following the last terrain transition.
    const climateChange = seen.current && revision === landscapeRevision.current;
    const birth = climateChange ? clock.elapsedTime * 1000 : transitionStart;
    landscapeRevision.current = revision;
    for (const [id, entry] of slots.current) {
      if (!ids.has(id)) {
        const transition = nuclearTransitions.get(entry.plant.tileKey);
        if (transition?.outgoing && !transition.collapsed) {
          continue;
        }
        animations.current.delete(entry.slot);
        slots.current.delete(id);
        if (climateChange && !nuclearTiles.has(entry.plant.tileKey)) {
          retiring.current.set(entry.slot, { plant: entry.plant, start: clock.elapsedTime * 1000 });
          continue;
        }
        hide(entry.slot);
      }
    }
    const used = new Set([...slots.current.values()].map((e) => e.slot));
    for (const slot of retiring.current.keys()) {
      used.add(slot);
    }
    let next = 0;
    for (const p of plants) {
      if (!allowed(p)) {
        continue;
      }
      const prior = slots.current.get(p.id);
      if (nuclearTransitions.has(p.tileKey)) {
        continue;
      }
      if (prior?.plant === p) {
        continue;
      }
      while (used.has(next)) {
        next++;
      }
      const slot = prior?.slot ?? next;
      if (slot >= capacity) {
        throw new Error("Vegetation batch capacity exceeded");
      }
      used.add(slot);
      const grow = seen.current && !prior;
      const previousHeight = previousLandscape.id
        ? landscapeHeightAt(previousLandscape.patches, p.x, p.y, previousLandscape.relief)
        : p.height;
      write(slot, p, grow ? 0 : 1, previousHeight);
      if (grow || previousHeight !== p.height) {
        animations.current.set(slot, {
          plant: p,
          fromHeight: previousHeight,
          birth,
          grow,
          shores: climateChange
            ? null
            : {
                previous: landscapeShoreAt(previousLandscape.patches, p.x, p.y),
                next: landscapeShoreAt(landscape.patches, p.x, p.y),
              },
        });
      }
      colorize(slot, p);
      slots.current.set(p.id, { slot, plant: p });
    }
    updateHighest();
    if (revision > 0) {
      seen.current = true;
    }
  }, [plants, landscape, transitionStart, previousLandscape, nuclearTransitions, nuclearTiles]);
  useFrame(({ clock, camera }) => {
    let collapsing = false;
    if (nuclearTransitions.size) {
      for (const entry of slots.current.values()) {
        const transition = nuclearTransitions.get(entry.plant.tileKey);
        if (!transition) {
          continue;
        }
        const age = nuclearElapsed(clock.elapsedTime - transition.start) - NUCLEAR_FLIGHT_SECONDS;
        if (age < 0) {
          continue;
        }
        animations.current.delete(entry.slot);
        write(entry.slot, entry.plant, 1, entry.plant.height, age);
        collapsing = true;
      }
    }
    if (sized) {
      const { treeScale } = climate.current.current;
      if (Math.abs(treeScale - appliedSize.current) > 0.002) {
        appliedSize.current = treeScale;
        for (const entry of slots.current.values()) {
          if (!animations.current.has(entry.slot) && !nuclearTransitions.has(entry.plant.tileKey)) {
            write(entry.slot, entry.plant, 1, entry.plant.height);
          }
        }
      }
    }
    if (animations.current.size || collapsing || retiring.current.size) {
      const now = clock.elapsedTime * 1000;
      let retired = false;
      for (const [slot, r] of retiring.current) {
        const t = Math.min(1, (now - r.start) / RETIRE_MS);
        write(slot, r.plant, 1 - t * t * t, r.plant.height - t * 0.004);
        if (t === 1) {
          hide(slot);
          retiring.current.delete(slot);
          retired = true;
        }
      }
      if (retired) {
        updateHighest();
      }
      for (const [slot, a] of animations.current) {
        const t = Math.max(0, Math.min(1, (now - a.birth) / TRANSITION_MS));
        const progress = easeOutCubic(t);
        const heightProgress = a.shores
          ? lakeBlendWeight(a.shores.previous, a.shores.next, t)
          : t * t * (3 - 2 * t);
        write(
          slot,
          a.plant,
          a.grow ? progress : 1,
          a.fromHeight + (a.plant.height - a.fromHeight) * heightProgress,
        );
        if (progress === 1) {
          animations.current.delete(slot);
        }
      }
    }
    cull(camera);
    if (performanceStore.watching && mesh.current) {
      const stats = batchStats.get(statsKey) ?? { kind, active: 0, drawn: 0, retiring: 0 };
      stats.active = slots.current.size;
      stats.drawn = mesh.current.count;
      stats.retiring = retiring.current.size;
      batchStats.set(statsKey, stats);
    }
  });
  // Frustum culling: copy only instances whose bounds touch the view into the GPU buffer. It runs
  // only when the plants or the camera's view of the planet change, so a still frame costs nothing.
  const cull = (camera: THREE.Camera) => {
    const target = mesh.current;
    if (!target) {
      return;
    }
    source.local
      .multiplyMatrices(camera.projectionMatrix, camera.matrixWorldInverse)
      .multiply(target.matrixWorld);
    const view = source.local.elements;
    let moved = false;
    for (let i = 0; i < 16; i++) {
      if (Math.abs(view[i] - source.view[i]) > CULL_VIEW_TOLERANCE) {
        moved = true;
        break;
      }
    }
    if (!moved && !source.dirty) {
      return;
    }
    source.view.set(view);
    source.dirty = false;
    source.frustum.setFromProjectionMatrix(source.local);
    const bounds = geometry.boundingSphere!;
    const reach = bounds.center.length() + bounds.radius;
    const matrices = source.matrices;
    const out = target.instanceMatrix.array as Float32Array;
    const colors = source.colors;
    const outColors = target.instanceColor?.array as Float32Array | undefined;
    let visible = 0;
    for (let slot = 0; slot < source.highest; slot++) {
      const m = slot * 16;
      const sx =
        matrices[m] * matrices[m] +
        matrices[m + 1] * matrices[m + 1] +
        matrices[m + 2] * matrices[m + 2];
      const sz =
        matrices[m + 8] * matrices[m + 8] +
        matrices[m + 9] * matrices[m + 9] +
        matrices[m + 10] * matrices[m + 10];
      const scale = Math.sqrt(Math.max(sx, sz));
      if (scale === 0) {
        continue;
      }
      source.sphere.center.set(matrices[m + 12], matrices[m + 13], matrices[m + 14]);
      source.sphere.radius = reach * scale + CULL_MARGIN;
      if (!source.frustum.intersectsSphere(source.sphere)) {
        continue;
      }
      out.set(matrices.subarray(m, m + 16), visible * 16);
      if (outColors) {
        outColors.set(colors.subarray(slot * 3, slot * 3 + 3), visible * 3);
      }
      visible++;
    }
    target.count = visible;
    if (depthMesh.current) {
      depthMesh.current.count = visible;
    }
    target.instanceMatrix.clearUpdateRanges();
    target.instanceMatrix.addUpdateRange(0, visible * 16);
    target.instanceMatrix.needsUpdate = true;
    if (target.instanceColor) {
      target.instanceColor.clearUpdateRanges();
      target.instanceColor.addUpdateRange(0, visible * 3);
      target.instanceColor.needsUpdate = true;
    }
  };
  return (
    <>
      {depthMaterial && (
        <instancedMesh
          ref={depthMesh}
          args={[geometry, depthMaterial, 1]}
          userData={{ perfGroup: `plants: ${kind}` }}
          frustumCulled={false}
          raycast={() => {}}
          renderOrder={14}
        />
      )}
      <instancedMesh
        ref={mesh}
        args={[geometry, material, capacity]}
        userData={{ perfGroup: `plants: ${kind}` }}
        frustumCulled={false}
        raycast={() => {}}
        renderOrder={15}
      >
        <instancedBufferAttribute
          attach="instanceColor"
          args={[new Float32Array(capacity * 3), 3]}
        />
      </instancedMesh>
    </>
  );
}
export default memo(GreeneryRenderer);
