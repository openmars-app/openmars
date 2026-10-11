import { useNuclearImpacted } from "./NuclearCollapse";
import { useRef, useState, useMemo, useEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import { mergeGeometries } from "three/examples/jsm/utils/BufferGeometryUtils.js";
import DustEffect from "./effects/DustEffect";
import MoholeSmoke from "./effects/MoholeSmoke";
import { createMoholeMaterial, createMoholeMaskMaterial } from "./shaders";
import { easeOutCubic, SPHERE_RADIUS } from "./boardConstants";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext";
import { useTextures } from "../../../hooks/useTextures";
import { useModels } from "../../../hooks/useModels";
import { addSphereProjectionWithSoftEdges } from "./GreeneryRenderer";
import { usePrimitiveInstances } from "./PrimitiveManager";

interface MoholeTileProps {
  isNewlyPlaced?: boolean;
  seed?: number;
  surfaceNormal?: THREE.Vector3;
  worldPosition?: THREE.Vector3;
  sphereCenter?: THREE.Vector3;
  groupInverseMatrix?: THREE.Matrix4;
}

const DISC_RADIUS = 0.14;
const HOLE_RADIUS_UV = 0.4;
const HOLE_DEPTH = 0.06;
const HOLE_WORLD_RADIUS = DISC_RADIUS * HOLE_RADIUS_UV;
const RIM_OUTER_RADIUS = DISC_RADIUS * 0.95;
const RIM_Z = 0.005;
const LINE_WIDTH = 0.007;

// Module-level temp objects for fence matrix computation (avoid per-frame allocation)
const _tmpEuler = new THREE.Euler();
const _tmpQuat = new THREE.Quaternion();
const _tmpPos = new THREE.Vector3();
const _tmpScale = new THREE.Vector3();
const _tmpMatrix = new THREE.Matrix4();

// Module-level cache for fence primitive extracted from GLB
let fenceCache: {
  geometry: THREE.BufferGeometry;
  material: THREE.Material;
  pieceWidth: number;
} | null = null;

function extractFencePrimitive(
  fenceScene: THREE.Group,
): { geometry: THREE.BufferGeometry; material: THREE.Material; pieceWidth: number } | null {
  if (fenceCache) {
    return fenceCache;
  }

  const box = new THREE.Box3().setFromObject(fenceScene);
  const size = box.getSize(new THREE.Vector3());
  const targetHeight = 0.018;
  const scaleF = targetHeight / size.y;

  const geometries: THREE.BufferGeometry[] = [];
  let firstMaterial: THREE.Material | null = null;

  fenceScene.traverse((child) => {
    if (child instanceof THREE.Mesh) {
      child.updateWorldMatrix(true, false);
      const geo = child.geometry.clone();
      geo.applyMatrix4(child.matrixWorld);
      geometries.push(geo);
      if (!firstMaterial) {
        firstMaterial = (child.material as THREE.Material).clone();
      }
    }
  });

  if (geometries.length === 0 || !firstMaterial) {
    return null;
  }

  const merged = mergeGeometries(geometries);
  if (!merged) {
    return null;
  }

  // Bake uniform scale into geometry so instance matrices only need position+rotation
  merged.applyMatrix4(new THREE.Matrix4().makeScale(scaleF, scaleF, scaleF));
  merged.computeVertexNormals();

  const mat = firstMaterial as THREE.MeshStandardMaterial;
  mat.side = THREE.DoubleSide;
  mat.transparent = true;
  mat.forceSinglePass = true;
  mat.alphaTest = 0.5;

  fenceCache = { geometry: merged, material: mat, pieceWidth: size.x * scaleF };
  return fenceCache;
}

function createRadialDiscGeometry(
  radius: number,
  radialSegments: number,
  angularSegments: number,
): THREE.BufferGeometry {
  const geometry = new THREE.BufferGeometry();
  const vertices: number[] = [];
  const uvs: number[] = [];
  const indices: number[] = [];

  vertices.push(0, 0, 0);
  uvs.push(0.5, 0.5);

  for (let r = 1; r <= radialSegments; r++) {
    const ringRadius = (r / radialSegments) * radius;
    for (let a = 0; a < angularSegments; a++) {
      const angle = (a / angularSegments) * Math.PI * 2;
      const x = Math.cos(angle) * ringRadius;
      const y = Math.sin(angle) * ringRadius;
      vertices.push(x, y, 0);
      uvs.push(0.5 + (x / radius) * 0.5, 0.5 + (y / radius) * 0.5);
    }
  }

  for (let a = 0; a < angularSegments; a++) {
    const next = (a + 1) % angularSegments;
    indices.push(0, 1 + a, 1 + next);
  }

  for (let r = 1; r < radialSegments; r++) {
    const innerStart = 1 + (r - 1) * angularSegments;
    const outerStart = 1 + r * angularSegments;
    for (let a = 0; a < angularSegments; a++) {
      const nextA = (a + 1) % angularSegments;
      indices.push(innerStart + a, outerStart + a, outerStart + nextA);
      indices.push(innerStart + a, outerStart + nextA, innerStart + nextA);
    }
  }

  geometry.setAttribute("position", new THREE.Float32BufferAttribute(vertices, 3));
  geometry.setAttribute("uv", new THREE.Float32BufferAttribute(uvs, 2));
  geometry.setIndex(indices);
  geometry.computeVertexNormals();
  return geometry;
}

function stageProgress(elapsed: number, startMs: number, endMs: number): number {
  if (elapsed < startMs) {
    return 0;
  }
  if (elapsed >= endMs) {
    return 1;
  }
  return easeOutCubic((elapsed - startMs) / (endMs - startMs));
}

export default function MoholeTile({
  isNewlyPlaced = false,
  seed: seedProp,
  surfaceNormal,
  worldPosition,
  sphereCenter,
  groupInverseMatrix,
}: MoholeTileProps) {
  const groupRef = useRef<THREE.Group>(null);
  const emergenceStartRef = useRef<number | null>(null);
  const isEmergingRef = useRef(isNewlyPlaced);
  const nuclearImpacted = useNuclearImpacted();
  const [showDust, setShowDust] = useState(isNewlyPlaced);
  const [showSmoke, setShowSmoke] = useState(!isNewlyPlaced);
  const { settings: world3DSettings } = useWorld3DSettings();
  const {
    concrete: concreteTexture,
    noiseMid: noiseTexture,
    noiseHigh: noiseHighTexture,
  } = useTextures();
  const { fenceScene } = useModels();

  useEffect(() => {
    if (isNewlyPlaced) {
      isEmergingRef.current = true;
      setShowDust(true);
      setShowSmoke(false);
      emergenceStartRef.current = null;
    }
  }, [isNewlyPlaced]);

  const seed = useMemo(() => seedProp ?? Math.random() * 10000, [seedProp]);

  const holeGeometry = useMemo(() => {
    return createRadialDiscGeometry(DISC_RADIUS, 28, 56);
  }, []);

  const rimGeometry = useMemo(() => {
    return new THREE.RingGeometry(HOLE_WORLD_RADIUS, RIM_OUTER_RADIUS * 1.2, 56, 8);
  }, []);

  const lineGeometry = useMemo(() => {
    return new THREE.RingGeometry(HOLE_WORLD_RADIUS - 0.001, HOLE_WORLD_RADIUS + LINE_WIDTH, 56, 1);
  }, []);

  const rimConcreteTexture = useMemo(() => {
    if (!concreteTexture) {
      return null;
    }
    const tex = concreteTexture.clone();
    tex.repeat.set(2, 2);
    tex.needsUpdate = true;
    return tex;
  }, [concreteTexture]);

  const rimMaterial = useMemo(() => {
    const mat = new THREE.MeshStandardMaterial({
      map: rimConcreteTexture,
      color: new THREE.Color(0.3, 0.28, 0.26),
      roughness: 0.95,
      metalness: 0.05,
      side: THREE.DoubleSide,
      forceSinglePass: true,
      transparent: true,
      alphaTest: 0.01,
      depthWrite: false,
      opacity: isEmergingRef.current ? 0 : 1,
    });
    const fenceHexR = RIM_OUTER_RADIUS - 0.035;
    addSphereProjectionWithSoftEdges(
      mat,
      0.006,
      noiseTexture,
      noiseHighTexture,
      fenceHexR,
      sphereCenter,
      groupInverseMatrix,
    );
    return mat;
  }, [rimConcreteTexture, noiseTexture, noiseHighTexture]);

  const lineMaterial = useMemo(() => {
    return new THREE.ShaderMaterial({
      uniforms: {
        uConcreteTexture: { value: concreteTexture ?? null },
        uSphereRadius: { value: SPHERE_RADIUS },
        uSphereCenter: { value: sphereCenter || new THREE.Vector3() },
        uOpacity: { value: isEmergingRef.current ? 0.0 : 1.0 },
      },
      vertexShader: `
        uniform float uSphereRadius;
        uniform vec3 uSphereCenter;
        varying vec2 vPos;
        varying vec2 vUv;
        void main() {
          vPos = position.xy;
          vUv = uv;
          vec4 worldPos = modelMatrix * vec4(position, 1.0);
          vec3 sphereDir = normalize(worldPos.xyz - uSphereCenter);
          vec3 projected = uSphereCenter + sphereDir * (uSphereRadius + 0.007);
          gl_Position = projectionMatrix * viewMatrix * vec4(projected, 1.0);
        }
      `,
      fragmentShader: `
        uniform sampler2D uConcreteTexture;
        uniform float uOpacity;
        varying vec2 vPos;
        varying vec2 vUv;
        void main() {
          vec3 concrete = texture2D(uConcreteTexture, vUv * 8.0).rgb;
          float angle = atan(vPos.y, vPos.x);
          float stripe = step(0.0, sin(angle * 24.0));
          vec3 orange = vec3(0.75, 0.45, 0.05);
          vec3 black = vec3(0.08, 0.07, 0.06);
          vec3 paint = mix(black, orange, stripe);
          vec3 color = concrete * 0.3 + paint * 0.6;
          gl_FragColor = vec4(color, uOpacity);
        }
      `,
      side: THREE.DoubleSide,
      forceSinglePass: true,
      transparent: true,
    });
  }, [concreteTexture]);

  // Merge all 3 pipe geometries into one mesh (3 draw calls → 1)
  const pipeData = useMemo(() => {
    const pipeRadius = 0.002;
    const depth = HOLE_DEPTH;
    const geometries: THREE.TubeGeometry[] = [];
    const tipPositions: THREE.Vector3[] = [];

    const s = seed;
    const rng = (i: number) => {
      const x = Math.sin(s * 127.1 + i * 311.7) * 43758.5453;
      return x - Math.floor(x);
    };

    const bendRadius = 0.008;
    const surfaceZ = RIM_Z;

    for (let i = 0; i < 3; i++) {
      const baseAngle = (i * Math.PI * 2) / 3;
      const angleOffset = (rng(i * 3) - 0.5) * 0.8;
      const a = baseAngle + angleOffset;
      const cos_a = Math.cos(a);
      const sin_a = Math.sin(a);

      const wallR = HOLE_WORLD_RADIUS - 0.01;
      const tipR = HOLE_WORLD_RADIUS + 0.018 + rng(i * 3 + 1) * 0.008;
      const path = new THREE.CurvePath<THREE.Vector3>();

      const vBottom = new THREE.Vector3(cos_a * wallR, sin_a * wallR, -depth * 1.2);
      const vTop = new THREE.Vector3(cos_a * wallR, sin_a * wallR, surfaceZ - bendRadius);
      path.add(new THREE.LineCurve3(vBottom, vTop));

      const bendCorner = new THREE.Vector3(cos_a * wallR, sin_a * wallR, surfaceZ);
      const bendEnd = new THREE.Vector3(
        cos_a * (wallR + bendRadius),
        sin_a * (wallR + bendRadius),
        surfaceZ,
      );
      path.add(new THREE.QuadraticBezierCurve3(vTop, bendCorner, bendEnd));

      const tipSurface = new THREE.Vector3(cos_a * tipR, sin_a * tipR, surfaceZ);
      path.add(new THREE.LineCurve3(bendEnd, tipSurface));

      const bend2Corner = new THREE.Vector3(cos_a * tipR, sin_a * tipR, surfaceZ);
      const bend2End = new THREE.Vector3(cos_a * tipR, sin_a * tipR, surfaceZ - bendRadius);
      path.add(new THREE.QuadraticBezierCurve3(tipSurface, bend2Corner, bend2End));

      const vDown = new THREE.Vector3(cos_a * tipR, sin_a * tipR, -depth * 0.5);
      path.add(new THREE.LineCurve3(bend2End, vDown));

      geometries.push(new THREE.TubeGeometry(path, 48, pipeRadius, 5, false));
      tipPositions.push(tipSurface);
    }

    const merged = mergeGeometries(geometries);
    return { mergedGeometry: merged, tipPositions };
  }, [seed]);

  const pipeMaterial = useMemo(() => {
    const mat = new THREE.MeshStandardMaterial({
      color: new THREE.Color("#D4A017"),
      roughness: 0.4,
      metalness: 0.5,
      side: THREE.DoubleSide,
      forceSinglePass: true,
      transparent: true,
      opacity: isEmergingRef.current ? 0 : 1,
    });
    mat.onBeforeCompile = (shader) => {
      shader.uniforms.uSphereRadius = { value: SPHERE_RADIUS };
      shader.uniforms.uSphereCenter = { value: sphereCenter || new THREE.Vector3() };
      shader.vertexShader =
        "uniform float uSphereRadius;\nuniform vec3 uSphereCenter;\nvarying float vDepth;\n" +
        shader.vertexShader;
      shader.vertexShader = shader.vertexShader.replace(
        "#include <begin_vertex>",
        `#include <begin_vertex>
        vec4 pipeWorldPos = modelMatrix * vec4(position, 1.0);
        float pipeR = length(pipeWorldPos.xyz - uSphereCenter);
        vDepth = pipeR - uSphereRadius;`,
      );
      shader.fragmentShader = "varying float vDepth;\n" + shader.fragmentShader;
      shader.fragmentShader = shader.fragmentShader.replace(
        "#include <opaque_fragment>",
        `#include <opaque_fragment>
        float pipeV = clamp((0.005 - vDepth) / 0.045, 0.0, 1.0);
        float pipeFade = mix(1.0, 0.0, pow(pipeV, 2.0));
        gl_FragColor.rgb *= pipeFade;`,
      );
    };
    return mat;
  }, []);

  // --- Fence via PrimitiveManager ---

  const fencePrimitive = useMemo(() => {
    if (!fenceScene) {
      return null;
    }
    return extractFencePrimitive(fenceScene);
  }, [fenceScene]);

  // Compute the tile's world transform (position on sphere + orientation)
  const tileWorldMatrix = useMemo(() => {
    if (!worldPosition || !surfaceNormal) {
      return null;
    }
    const quat = new THREE.Quaternion().setFromUnitVectors(
      new THREE.Vector3(0, 0, 1),
      surfaceNormal,
    );
    return new THREE.Matrix4().compose(worldPosition, quat, new THREE.Vector3(1, 1, 1));
  }, [worldPosition, surfaceNormal]);

  // Compute local fence layout (positions + rotations in tile-local space)
  const fenceLayout = useMemo(() => {
    if (!fencePrimitive) {
      return [];
    }

    const hexRadius = RIM_OUTER_RADIUS - 0.035;
    const sides = 6;
    const hexVerts: [number, number][] = [];
    for (let s = 0; s < sides; s++) {
      const a = (s / sides) * Math.PI * 2 + Math.PI / 6;
      hexVerts.push([Math.cos(a) * hexRadius, Math.sin(a) * hexRadius]);
    }

    const layout: { x: number; y: number; angle: number }[] = [];
    for (let s = 0; s < sides; s++) {
      const [x0, y0] = hexVerts[s];
      const [x1, y1] = hexVerts[(s + 1) % sides];
      const edgeLen = Math.sqrt((x1 - x0) ** 2 + (y1 - y0) ** 2);
      const piecesPerEdge = Math.min(
        4,
        Math.max(1, Math.round(edgeLen / fencePrimitive.pieceWidth)),
      );
      const edgeAngle = Math.atan2(y1 - y0, x1 - x0);

      for (let j = 0; j < piecesPerEdge; j++) {
        const t = (j + 0.5) / piecesPerEdge;
        layout.push({
          x: x0 + (x1 - x0) * t,
          y: y0 + (y1 - y0) * t,
          angle: edgeAngle,
        });
      }
    }
    return layout;
  }, [fencePrimitive]);

  // Pre-allocate matrices array for fence instances
  const fenceMatricesRef = useRef<THREE.Matrix4[]>([]);

  const { setTransforms: setFenceTransforms } = usePrimitiveInstances(
    "mohole-fence",
    fencePrimitive?.geometry ?? null,
    fencePrimitive?.material ?? null,
    16,
  );

  // Compute and submit fence world-space matrices
  const computeFenceMatrices = (scaleY: number): THREE.Matrix4[] => {
    if (!tileWorldMatrix || fenceLayout.length === 0) {
      return [];
    }

    // Ensure we have enough pre-allocated matrices
    while (fenceMatricesRef.current.length < fenceLayout.length) {
      fenceMatricesRef.current.push(new THREE.Matrix4());
    }

    for (let i = 0; i < fenceLayout.length; i++) {
      const { x, y, angle } = fenceLayout[i];
      _tmpPos.set(x, y, RIM_Z);
      _tmpEuler.set(Math.PI / 2, angle + Math.PI / 2, 0);
      _tmpQuat.setFromEuler(_tmpEuler);
      _tmpScale.set(1, scaleY, 1);
      _tmpMatrix.compose(_tmpPos, _tmpQuat, _tmpScale);
      // Local → world
      fenceMatricesRef.current[i].multiplyMatrices(tileWorldMatrix, _tmpMatrix);
    }

    return fenceMatricesRef.current.slice(0, fenceLayout.length);
  };

  // Set initial fence transforms
  useEffect(() => {
    const scaleY = isEmergingRef.current ? 0 : 1;
    const matrices = computeFenceMatrices(scaleY);
    setFenceTransforms(matrices);
  }, [fenceLayout, tileWorldMatrix]);

  const moholeMaterial = useMemo(() => {
    const mat = createMoholeMaterial(seed, concreteTexture, sphereCenter);
    mat.uniforms.uEmergence.value = isEmergingRef.current ? 0.0 : 1.0;
    mat.uniforms.uEmergenceRadius.value = 1.0;
    return mat;
  }, [seed, concreteTexture]);

  const maskMaterial = useMemo(() => {
    const mat = createMoholeMaskMaterial(seed, sphereCenter);
    mat.uniforms.uEmergence.value = isEmergingRef.current ? 0.0 : 1.0;
    mat.uniforms.uEmergenceRadius.value = 1.0;
    return mat;
  }, [seed]);

  useFrame((state) => {
    if (nuclearImpacted) {
      return;
    }
    moholeMaterial.uniforms.uTime.value = state.clock.elapsedTime;
    moholeMaterial.uniforms.uSunDirection.value
      .set(
        world3DSettings.sunDirectionX,
        world3DSettings.sunDirectionY,
        world3DSettings.sunDirectionZ,
      )
      .normalize();
    moholeMaterial.uniforms.uSunIntensity.value = world3DSettings.sunIntensity;
    moholeMaterial.uniforms.uSunColor.value.set(
      world3DSettings.sunColor.r,
      world3DSettings.sunColor.g,
      world3DSettings.sunColor.b,
    );

    if (!isEmergingRef.current) {
      moholeMaterial.uniforms.uEmergence.value = 1.0;
      maskMaterial.uniforms.uEmergence.value = 1.0;
      return;
    }

    if (!groupRef.current) {
      return;
    }

    if (emergenceStartRef.current === null) {
      emergenceStartRef.current = state.clock.elapsedTime * 1000;
    }

    const elapsed = state.clock.elapsedTime * 1000 - emergenceStartRef.current;
    const duration = 1500;
    const t = Math.min(elapsed / duration, 1);

    const rimProgress = stageProgress(elapsed, 0, 400);
    rimMaterial.opacity = rimProgress;
    lineMaterial.uniforms.uOpacity.value = rimProgress;

    const holeProgress = stageProgress(elapsed, 200, 800);
    moholeMaterial.uniforms.uEmergence.value = holeProgress;
    maskMaterial.uniforms.uEmergence.value = holeProgress;

    const pipeProgress = stageProgress(elapsed, 400, 1000);
    pipeMaterial.opacity = pipeProgress;

    // Stage 4: Fences grow up from ground (600-1200ms)
    const fenceProgress = stageProgress(elapsed, 600, 1200);
    setFenceTransforms(computeFenceMatrices(fenceProgress));

    const shakeIntensity = 0.012 * (1 - easeOutCubic(t));
    const shakeX = Math.sin(elapsed * 0.137 + seed) * shakeIntensity;
    const shakeY = Math.sin(elapsed * 0.173 + seed * 1.3) * shakeIntensity;
    groupRef.current.position.set(shakeX, shakeY, 0);

    if (t >= 1) {
      isEmergingRef.current = false;
      setShowSmoke(true);
      groupRef.current.position.set(0, 0, 0);
      rimMaterial.opacity = 1;
      lineMaterial.uniforms.uOpacity.value = 1;
      pipeMaterial.opacity = 1;
      moholeMaterial.uniforms.uEmergence.value = 1;
      maskMaterial.uniforms.uEmergence.value = 1;
      setFenceTransforms(computeFenceMatrices(1));
    }
  });

  return (
    <>
      <mesh
        geometry={holeGeometry}
        material={maskMaterial}
        renderOrder={-1}
        frustumCulled={false}
      />

      <group ref={groupRef}>
        <mesh
          geometry={holeGeometry}
          material={moholeMaterial}
          renderOrder={13}
          frustumCulled={false}
        />

        <mesh
          geometry={rimGeometry}
          material={rimMaterial}
          renderOrder={14}
          frustumCulled={false}
        />

        <mesh
          geometry={lineGeometry}
          material={lineMaterial}
          renderOrder={15}
          frustumCulled={false}
        />

        {pipeData.mergedGeometry && (
          <mesh
            geometry={pipeData.mergedGeometry}
            material={pipeMaterial}
            renderOrder={14}
            frustumCulled={false}
          />
        )}
      </group>

      {!nuclearImpacted && showSmoke && tileWorldMatrix && (
        <MoholeSmoke seed={seed} isNewlyPlaced={isNewlyPlaced} tileWorldMatrix={tileWorldMatrix} />
      )}

      {!nuclearImpacted && showDust && surfaceNormal && worldPosition && (
        <DustEffect
          seed={seed}
          duration={2800}
          particleColor={new THREE.Color(0.35, 0.15, 0.1)}
          onComplete={() => setShowDust(false)}
        />
      )}
    </>
  );
}
