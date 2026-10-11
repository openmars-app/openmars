import { sharedRockGeometry, createNuclearDebrisMaterial } from "./rockGeometry";
import { useCallback, useMemo, useRef, useLayoutEffect, useEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import {
  createVolcanoMaterial,
  createNuclearZoneMaterial,
  createNuclearMaskMaterial,
  createNuclearWaveMaterial,
  createWorldTreeMaterial,
  createSunSurfaceMaterial,
  createSunCoronaMaterial,
  createSunProminenceMaterial,
} from "./shaders";
import { computeFlowMap } from "./volcanoFlowMap";
import { bareNuclearGround, createNuclearGeometry } from "./nuclearGeometry";
import { SPHERE_RADIUS } from "./boardConstants";
import {
  variantCache,
  createVariantsFromScene,
  TREE_NAMES,
  PINE_NAMES,
  BUSH_NAMES,
  CLOVER_NAMES,
  FLOWER_NAMES,
  createRockMaterial,
  type TreeVariant,
} from "./GreeneryRenderer";
import { useModels } from "../../../hooks/useModels";
import { usePlanetSurface, usePlanetSurfaces, useTextures } from "../../../hooks/useTextures";
import { GRAPHICS } from "@/utils/graphicsQuality.ts";
import { coldStartTrace } from "@/services/performanceStore.ts";

import { StructureGroundPatch, getMaterials } from "./StructureRenderer";
import { createStructureWarmupGeometry } from "./structureBatch";
import LandscapeRenderer from "./LandscapeRenderer";
import { LandscapeBuilder } from "./landscapeFields";
import type { LandscapeState } from "./landscapeTypes";
const warmupSources = [
  { coordinate: { q: 0, r: 0, s: 0 }, seed: 1, kind: "greenery" as const },
  { coordinate: { q: 1, r: 0, s: -1 }, seed: 1, kind: "ocean" as const },
];
const warmupPatches = new LandscapeBuilder().build({ seed: 1, sources: warmupSources });
const landscapeWarmup: LandscapeState = {
  id: 1,
  seed: 1,
  sources: warmupSources,
  patches: warmupPatches,
  plants: [],
};
const WARMUP_SCALE = 0.001;
const WARMUP_FRAMES = 3;
const ORIGIN = new THREE.Vector3();

const PLANET_WARMUP_KEYS = [
  "venus",
  "earth",
  "jupiter",
  "mercury",
  "saturn",
  "neptune",
  "uranus",
  "ceres",
  "moon",
  "ganymede",
  "earthClouds",
] as const;

type PlanetWarmupTextures = Record<(typeof PLANET_WARMUP_KEYS)[number], THREE.Texture>;

function PlanetWarmupMeshes({
  geometry,
  textures,
}: {
  geometry: THREE.BufferGeometry;
  textures: PlanetWarmupTextures;
}) {
  const materials = useMemo(
    () =>
      PLANET_WARMUP_KEYS.map(
        (key) =>
          new THREE.MeshStandardMaterial({
            map: textures[key],
            roughness: 0.8,
            metalness: 0.05,
            fog: false,
            transparent: key === "earthClouds",
            depthWrite: key !== "earthClouds",
          }),
      ),
    [textures],
  );
  return (
    <>
      {materials.map((mat, i) => (
        <mesh key={`planet-warmup-${i}`} geometry={geometry} material={mat} frustumCulled={false} />
      ))}
    </>
  );
}

function PlanetSurfaceWarmup({ geometry }: { geometry: THREE.BufferGeometry }) {
  const surfaces = usePlanetSurfaces();
  return <PlanetWarmupMeshes geometry={geometry} textures={surfaces} />;
}

interface GpuWarmupProps {
  onReady?: () => void;
}

export default function GpuWarmup({ onReady }: GpuWarmupProps) {
  const { treesScene, pinesScene, rockScene, flowersScene } = useModels();
  const textures = useTextures();
  const structureMaterials = useMemo(
    () => [...getMaterials("normal", textures), ...getMaterials("bright", textures).slice(0, 3)],
    [textures.concrete, textures.grass, textures.sand, textures.structureFacades],
  );
  const structureGeometry = useMemo(() => createStructureWarmupGeometry(), []);
  useEffect(() => () => structureGeometry.dispose(), [structureGeometry]);
  const { rock: rockTexture, smoke: smokeTexture, grass: grassTexture } = textures;

  const warmupRoot = useRef<THREE.Group>(null);
  const frameCount = useRef(0);
  const readyFired = useRef(false);
  // Water, basin and lake mask only draw once the warmup landscape commits its patches.
  const landscapeReady = useRef(false);
  const handleLandscapeReady = useCallback(() => {
    landscapeReady.current = true;
  }, []);

  useFrame(() => {
    if (readyFired.current) {
      if (warmupRoot.current) {
        warmupRoot.current.visible = false;
      }
      return;
    }
    if (!landscapeReady.current) {
      return;
    }
    frameCount.current++;
    if (frameCount.current > WARMUP_FRAMES) {
      readyFired.current = true;
      if (warmupRoot.current) {
        warmupRoot.current.visible = false;
      }
      coldStartTrace.mark("warmup:ready", { frames: frameCount.current });
      onReady?.();
    }
  });

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
    if (variantCache.rock) return variantCache.rock;

    const geo = sharedRockGeometry(rockScene);

    variantCache.rock = { geometry: geo, material: createRockMaterial(rockTexture) };
    return variantCache.rock;
  }, [rockScene, rockTexture]);

  const nuclearDebrisMaterial = useMemo(
    () => createNuclearDebrisMaterial(rockTexture),
    [rockTexture],
  );
  useEffect(() => () => nuclearDebrisMaterial.dispose(), [nuclearDebrisMaterial]);

  const smokeWarmupMaterial = useMemo(() => {
    return new THREE.MeshBasicMaterial({
      map: smokeTexture,
      transparent: true,
      opacity: 0,
      depthWrite: false,
      depthTest: false,
      blending: THREE.NormalBlending,
      side: THREE.DoubleSide,
      forceSinglePass: true,
    });
  }, [smokeTexture]);

  const smokeGeometry = useMemo(() => new THREE.PlaneGeometry(0.3, 0.3), []);

  const volcanoFlowTexture = useMemo(() => computeFlowMap(42), []);
  const volcanoWarmupMaterial = useMemo(() => {
    return createVolcanoMaterial(grassTexture, volcanoFlowTexture, 42);
  }, [grassTexture, volcanoFlowTexture]);

  const volcanoGeometry = useMemo(() => new THREE.CircleGeometry(0.15, 32), []);

  const nuclearZoneWarmupMaterial = useMemo(() => {
    return createNuclearZoneMaterial(42, textures.mars).material;
  }, [textures.mars]);

  const nuclearMaskMaterial = useMemo(() => createNuclearMaskMaterial(), []);
  const nuclearWaveMaterial = useMemo(() => createNuclearWaveMaterial(), []);
  const nuclearZoneGeometry = useMemo(
    () =>
      createNuclearGeometry(42, bareNuclearGround, {
        origin: new THREE.Vector3(0, 0, SPHERE_RADIUS + 0.01),
        rotation: new THREE.Quaternion(),
      }),
    [],
  );
  useEffect(
    () => () => {
      Object.values(nuclearZoneGeometry).forEach((geometry) => geometry.dispose());
    },
    [nuclearZoneGeometry],
  );
  useEffect(
    () => () => {
      nuclearMaskMaterial.dispose();
      nuclearWaveMaterial.dispose();
      nuclearZoneWarmupMaterial.dispose();
    },
    [nuclearMaskMaterial, nuclearWaveMaterial, nuclearZoneWarmupMaterial],
  );

  const worldTreeWarmupMaterial = useMemo(() => {
    return createWorldTreeMaterial(42);
  }, []);

  const worldTreeGeometry = useMemo(() => new THREE.CircleGeometry(0.14, 32), []);

  const volcanoSmokeWarmupMaterial = useMemo(() => {
    return new THREE.SpriteMaterial({
      map: smokeTexture,
      transparent: true,
      opacity: 0,
      depthWrite: false,
      blending: THREE.NormalBlending,
      color: new THREE.Color(0.03, 0.025, 0.02),
    });
  }, [smokeTexture]);

  // --- Solar system warmup materials ---
  const solarSphereGeometry = useMemo(() => new THREE.SphereGeometry(WARMUP_SCALE, 8, 4), []);

  const sunTexture = usePlanetSurface("sun", true);
  const sunSurfaceMaterial = useMemo(() => createSunSurfaceMaterial(sunTexture), [sunTexture]);
  const sunCoronaMaterial = useMemo(() => createSunCoronaMaterial(), []);
  const sunProminenceMaterial = useMemo(() => createSunProminenceMaterial(), []);
  const {
    venusLod,
    earthLod,
    jupiterLod,
    mercuryLod,
    saturnLod,
    neptuneLod,
    uranusLod,
    ceresLod,
    moonLod,
    ganymedeLod,
    earthCloudsLod,
  } = textures;
  const planetLods = useMemo(
    () => ({
      venus: venusLod,
      earth: earthLod,
      jupiter: jupiterLod,
      mercury: mercuryLod,
      saturn: saturnLod,
      neptune: neptuneLod,
      uranus: uranusLod,
      ceres: ceresLod,
      moon: moonLod,
      ganymede: ganymedeLod,
      earthClouds: earthCloudsLod,
    }),
    [
      venusLod,
      earthLod,
      jupiterLod,
      mercuryLod,
      saturnLod,
      neptuneLod,
      uranusLod,
      ceresLod,
      moonLod,
      ganymedeLod,
      earthCloudsLod,
    ],
  );

  const orbitLineMaterial = useMemo(
    () => new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, fog: false }),
    [],
  );

  const orbitLineObj = useMemo(() => {
    const points = [new THREE.Vector3(0, 0, 0), new THREE.Vector3(WARMUP_SCALE, 0, 0)];
    const geo = new THREE.BufferGeometry().setFromPoints(points);
    geo.setAttribute(
      "color",
      new THREE.Float32BufferAttribute([0.25, 0.35, 0.5, 0.25, 0.35, 0.5], 3),
    );
    const line = new THREE.Line(geo, orbitLineMaterial);
    line.frustumCulled = false;
    return line;
  }, [orbitLineMaterial]);

  const treeRefs = useRef<Map<string, THREE.InstancedMesh | null>>(new Map());
  const bushRefs = useRef<Map<string, THREE.InstancedMesh | null>>(new Map());
  const cloverRefs = useRef<Map<string, THREE.InstancedMesh | null>>(new Map());
  const flowerRefs = useRef<Map<string, THREE.InstancedMesh | null>>(new Map());
  const rockRef = useRef<THREE.InstancedMesh>(null);

  useLayoutEffect(() => {
    const matrix = new THREE.Matrix4().compose(
      new THREE.Vector3(0, 0, 0),
      new THREE.Quaternion(),
      new THREE.Vector3(WARMUP_SCALE, WARMUP_SCALE, WARMUP_SCALE),
    );
    for (const [, mesh] of treeRefs.current) {
      if (!mesh) continue;
      mesh.setMatrixAt(0, matrix);
      mesh.instanceMatrix.needsUpdate = true;
      mesh.setColorAt(0, new THREE.Color(1, 1, 1));
      if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    }
  }, [treeVariants, pineVariants]);

  useLayoutEffect(() => {
    const matrix = new THREE.Matrix4().compose(
      new THREE.Vector3(0, 0, 0),
      new THREE.Quaternion(),
      new THREE.Vector3(WARMUP_SCALE, WARMUP_SCALE, WARMUP_SCALE),
    );
    for (const [, mesh] of bushRefs.current) {
      if (!mesh) continue;
      mesh.setMatrixAt(0, matrix);
      mesh.instanceMatrix.needsUpdate = true;
      mesh.setColorAt(0, new THREE.Color(1, 1, 1));
      if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    }
  }, [bushVariants]);

  useLayoutEffect(() => {
    const matrix = new THREE.Matrix4().compose(
      new THREE.Vector3(0, 0, 0),
      new THREE.Quaternion(),
      new THREE.Vector3(WARMUP_SCALE, WARMUP_SCALE, WARMUP_SCALE),
    );
    for (const [, mesh] of cloverRefs.current) {
      if (!mesh) continue;
      mesh.setMatrixAt(0, matrix);
      mesh.instanceMatrix.needsUpdate = true;
      mesh.setColorAt(0, new THREE.Color(1, 1, 1));
      if (mesh.instanceColor) {
        mesh.instanceColor.needsUpdate = true;
      }
    }
  }, [cloverVariants]);

  useLayoutEffect(() => {
    const matrix = new THREE.Matrix4().compose(
      new THREE.Vector3(0, 0, 0),
      new THREE.Quaternion(),
      new THREE.Vector3(WARMUP_SCALE, WARMUP_SCALE, WARMUP_SCALE),
    );
    for (const [, mesh] of flowerRefs.current) {
      if (!mesh) continue;
      mesh.setMatrixAt(0, matrix);
      mesh.instanceMatrix.needsUpdate = true;
      mesh.setColorAt(0, new THREE.Color(1, 1, 1));
      if (mesh.instanceColor) {
        mesh.instanceColor.needsUpdate = true;
      }
    }
  }, [flowerVariants]);

  useLayoutEffect(() => {
    if (!rockRef.current) return;
    const matrix = new THREE.Matrix4().compose(
      new THREE.Vector3(0, 0, 0),
      new THREE.Quaternion(),
      new THREE.Vector3(WARMUP_SCALE, WARMUP_SCALE, WARMUP_SCALE),
    );
    rockRef.current.setMatrixAt(0, matrix);
    rockRef.current.instanceMatrix.needsUpdate = true;
    rockRef.current.setColorAt(0, new THREE.Color(1, 1, 1));
  }, [rockGeometry]);

  const renderVariants = (
    variants: TreeVariant[],
    prefix: string,
    refs: React.RefObject<Map<string, THREE.InstancedMesh | null>>,
    materialOverrides?: THREE.Material[][],
  ) =>
    variants.map((variant, vIdx) =>
      variant.primitives.map((prim, pIdx) => {
        const key = `${prefix}-${vIdx}-${pIdx}`;
        const mat = materialOverrides ? materialOverrides[vIdx][pIdx] : prim.material;
        const depth = mat.userData.depthMaterial as THREE.Material | undefined;
        return (
          <group key={key}>
            {depth && (
              <instancedMesh
                ref={(el) => {
                  refs.current.set(`${key}-depth`, el);
                }}
                args={[prim.geometry, depth, 1]}
                frustumCulled={false}
              />
            )}
            <instancedMesh
              ref={(el) => {
                refs.current.set(key, el);
              }}
              args={[prim.geometry, mat, 1]}
              frustumCulled={false}
            />
          </group>
        );
      }),
    );

  return (
    <group ref={warmupRoot}>
      <group scale={WARMUP_SCALE}>
        <LandscapeRenderer capacity={16} state={landscapeWarmup} onReady={handleLandscapeReady} />
      </group>
      <group scale={WARMUP_SCALE} dispose={null}>
        {structureMaterials.map((material) => (
          <mesh
            key={material.uuid}
            geometry={structureGeometry}
            material={material}
            frustumCulled={false}
          />
        ))}
        <StructureGroundPatch
          radius={0.155}
          surface="soil"
          sphereCenter={ORIGIN}
          frustumCulled={false}
        />
      </group>
      {renderVariants(treeVariants, "warmup-tree", treeRefs)}
      {renderVariants(pineVariants, "warmup-pine", treeRefs)}
      {renderVariants(bushVariants, "warmup-bush", bushRefs)}
      {renderVariants(cloverVariants, "warmup-clover", cloverRefs)}
      {renderVariants(flowerVariants, "warmup-flower", flowerRefs)}
      <instancedMesh
        ref={(mesh) => {
          if (mesh) {
            mesh.setColorAt(0, new THREE.Color(1, 1, 1));
          }
        }}
        args={[rockGeometry, nuclearDebrisMaterial, 1]}
        scale={WARMUP_SCALE}
        frustumCulled={false}
      />
      <instancedMesh ref={rockRef} args={[rockGeometry, rockMaterial, 1]} frustumCulled={false} />
      <mesh geometry={smokeGeometry} material={smokeWarmupMaterial} frustumCulled={false} />
      <mesh geometry={volcanoGeometry} material={volcanoWarmupMaterial} frustumCulled={false} />
      <mesh
        geometry={nuclearZoneGeometry.bowl}
        material={nuclearZoneWarmupMaterial}
        frustumCulled={false}
      />
      <mesh geometry={nuclearZoneGeometry.bowl} frustumCulled={false}>
        <meshStandardMaterial color="#59633d" roughness={0.85} />
      </mesh>
      <mesh
        geometry={nuclearZoneGeometry.mask}
        material={nuclearMaskMaterial}
        frustumCulled={false}
      />
      <mesh
        geometry={nuclearZoneGeometry.bowl}
        material={nuclearWaveMaterial}
        frustumCulled={false}
      />
      <mesh geometry={worldTreeGeometry} material={worldTreeWarmupMaterial} frustumCulled={false} />
      <sprite
        material={volcanoSmokeWarmupMaterial}
        scale={[WARMUP_SCALE, WARMUP_SCALE, WARMUP_SCALE]}
        frustumCulled={false}
      />

      {/* Solar system warmup */}
      <mesh geometry={solarSphereGeometry} material={sunSurfaceMaterial} frustumCulled={false} />
      <mesh geometry={solarSphereGeometry} material={sunCoronaMaterial} frustumCulled={false} />
      <mesh geometry={solarSphereGeometry} material={sunProminenceMaterial} frustumCulled={false} />
      {GRAPHICS.warmupAllPlanets ? (
        <PlanetSurfaceWarmup geometry={solarSphereGeometry} />
      ) : (
        <PlanetWarmupMeshes geometry={solarSphereGeometry} textures={planetLods} />
      )}
      <primitive object={orbitLineObj} />
    </group>
  );
}
