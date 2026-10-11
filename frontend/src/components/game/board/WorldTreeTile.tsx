import { useNuclearImpacted } from "./NuclearCollapse";
import { useRef, useState, useMemo, useEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import DustEffect from "./effects/DustEffect";
import { useTextures } from "../../../hooks/useTextures";
import { createWorldTreeMaterial } from "./shaders";
import { addSphereProjectionWithSoftEdges } from "./GreeneryRenderer";
import { easeOutCubic } from "./boardConstants";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext";

interface WorldTreeTileProps {
  isNewlyPlaced?: boolean;
  seed?: number;
  surfaceNormal?: THREE.Vector3;
  worldPosition?: THREE.Vector3;
  sphereCenter?: THREE.Vector3;
  groupInverseMatrix?: THREE.Matrix4;
}

const HEX_RADIUS = 0.166;

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

export default function WorldTreeTile({
  isNewlyPlaced = false,
  seed: seedProp,
  surfaceNormal,
  worldPosition,
  sphereCenter,
  groupInverseMatrix,
}: WorldTreeTileProps) {
  const groupRef = useRef<THREE.Group>(null);
  const emergenceStartRef = useRef<number | null>(null);
  const isEmergingRef = useRef(isNewlyPlaced);
  const nuclearImpacted = useNuclearImpacted();
  const [showDust, setShowDust] = useState(isNewlyPlaced);
  const { settings: world3DSettings } = useWorld3DSettings();

  useEffect(() => {
    if (isNewlyPlaced) {
      isEmergingRef.current = true;
      setShowDust(true);
      emergenceStartRef.current = null;
    }
  }, [isNewlyPlaced]);

  const seed = useMemo(() => seedProp ?? Math.random() * 10000, [seedProp]);

  const treeGeometry = useMemo(() => {
    return createRadialDiscGeometry(0.14, 28, 56);
  }, []);

  const groundGeometry = useMemo(() => {
    const geo = createRadialDiscGeometry(HEX_RADIUS * 1.8, 10, 36);
    geo.rotateZ(Math.PI / 2);
    return geo;
  }, []);

  const { noiseMid: noiseTexture, noiseHigh: noiseHighTexture } = useTextures();

  const worldTreeMaterial = useMemo(() => {
    const mat = createWorldTreeMaterial(seed, sphereCenter);
    mat.uniforms.uEmergence.value = isEmergingRef.current ? 0.0 : 1.0;
    return mat;
  }, [seed]);

  const groundMaterial = useMemo(() => {
    const mat = new THREE.MeshStandardMaterial({
      color: new THREE.Color(0.12, 0.2, 0.06),
      roughness: 0.92,
      metalness: 0.0,
      side: THREE.DoubleSide,
      forceSinglePass: true,
      transparent: true,
      alphaTest: 0.01,
      depthWrite: false,
    });
    addSphereProjectionWithSoftEdges(
      mat,
      0.003,
      noiseTexture,
      noiseHighTexture,
      HEX_RADIUS,
      sphereCenter,
      groupInverseMatrix,
    );
    return mat;
  }, [noiseTexture, noiseHighTexture]);

  useFrame((state) => {
    if (nuclearImpacted) {
      return;
    }
    worldTreeMaterial.uniforms.uTime.value = state.clock.elapsedTime;
    worldTreeMaterial.uniforms.uSunDirection.value
      .set(
        world3DSettings.sunDirectionX,
        world3DSettings.sunDirectionY,
        world3DSettings.sunDirectionZ,
      )
      .normalize();
    worldTreeMaterial.uniforms.uSunIntensity.value = world3DSettings.sunIntensity;
    worldTreeMaterial.uniforms.uSunColor.value.set(
      world3DSettings.sunColor.r,
      world3DSettings.sunColor.g,
      world3DSettings.sunColor.b,
    );

    if (!isEmergingRef.current) {
      worldTreeMaterial.uniforms.uEmergence.value = 1.0;
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
    const eased = easeOutCubic(t);

    worldTreeMaterial.uniforms.uEmergence.value = eased;

    const shakeIntensity = 0.006 * (1 - eased);
    const shakeX = Math.sin(elapsed * 0.137 + seed) * shakeIntensity;
    const shakeY = Math.sin(elapsed * 0.173 + seed * 1.3) * shakeIntensity;
    groupRef.current.position.set(shakeX, shakeY, 0);

    if (t >= 1) {
      isEmergingRef.current = false;
      groupRef.current.position.set(0, 0, 0);
    }
  });

  const groundQuaternion = useMemo(() => new THREE.Quaternion(), []);

  return (
    <>
      <group ref={groupRef}>
        <mesh
          geometry={groundGeometry}
          material={groundMaterial}
          quaternion={groundQuaternion}
          renderOrder={12}
        />

        <mesh
          geometry={treeGeometry}
          material={worldTreeMaterial}
          renderOrder={13}
          frustumCulled={false}
        />
      </group>

      {!nuclearImpacted && showDust && surfaceNormal && worldPosition && (
        <DustEffect
          seed={seed}
          duration={3000}
          particleColor={new THREE.Color(0.15, 0.3, 0.08)}
          onComplete={() => setShowDust(false)}
        />
      )}
    </>
  );
}
