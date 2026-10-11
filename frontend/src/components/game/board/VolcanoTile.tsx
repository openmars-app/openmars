import { useNuclearImpacted } from "./NuclearCollapse";
import { useRef, useState, useMemo, useEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import VolcanoSmoke from "./effects/VolcanoSmoke";
import DustEffect from "./effects/DustEffect";
import { useTextures } from "../../../hooks/useTextures";
import { createVolcanoMaterial } from "./shaders";
import { easeOutCubic } from "./boardConstants";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext";
import { useClimate } from "../../../contexts/ClimateContext";
import { computeFlowMap } from "./volcanoFlowMap";

interface VolcanoTileProps {
  isNewlyPlaced?: boolean;
  seed?: number;
  surfaceNormal?: THREE.Vector3;
  worldPosition?: THREE.Vector3;
  sphereCenter?: THREE.Vector3;
}

const VOLCANO_HEIGHT = 0.14;

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

export default function VolcanoTile({
  isNewlyPlaced = false,
  seed: seedProp,
  surfaceNormal,
  worldPosition,
  sphereCenter,
}: VolcanoTileProps) {
  const groupRef = useRef<THREE.Group>(null);
  const emergenceStartRef = useRef<number | null>(null);
  const isEmergingRef = useRef(isNewlyPlaced);
  const [showSmoke, setShowSmoke] = useState(!isNewlyPlaced);
  const nuclearImpacted = useNuclearImpacted();
  const [showDust, setShowDust] = useState(isNewlyPlaced);
  const { settings: world3DSettings } = useWorld3DSettings();
  const { runtime: climate } = useClimate();

  useEffect(() => {
    if (isNewlyPlaced) {
      isEmergingRef.current = true;
      setShowDust(true);
      setShowSmoke(false);
      emergenceStartRef.current = null;
    }
  }, [isNewlyPlaced]);

  const seed = useMemo(() => seedProp ?? Math.random() * 10000, [seedProp]);
  const flowTexture = useMemo(() => computeFlowMap(seed), [seed]);

  const volcanoGeometry = useMemo(() => {
    return createRadialDiscGeometry(0.15, 32, 64);
  }, []);

  const { grass: grassTexture } = useTextures();

  const volcanoMaterial = useMemo(() => {
    const mat = createVolcanoMaterial(grassTexture, flowTexture, seed, sphereCenter);
    mat.uniforms.uEmergence.value = isEmergingRef.current ? 0.0 : 1.0;
    return mat;
  }, [grassTexture, flowTexture, seed]);

  useFrame((state) => {
    if (nuclearImpacted) {
      return;
    }
    volcanoMaterial.uniforms.uTime.value = state.clock.elapsedTime;
    volcanoMaterial.uniforms.uVegetation.value = climate.current.current.grass;
    volcanoMaterial.uniforms.uFrost.value = climate.current.current.frost;
    volcanoMaterial.uniforms.uSunDirection.value
      .set(
        world3DSettings.sunDirectionX,
        world3DSettings.sunDirectionY,
        world3DSettings.sunDirectionZ,
      )
      .normalize();
    volcanoMaterial.uniforms.uSunIntensity.value = world3DSettings.sunIntensity;
    volcanoMaterial.uniforms.uSunColor.value.set(
      world3DSettings.sunColor.r,
      world3DSettings.sunColor.g,
      world3DSettings.sunColor.b,
    );

    if (!isEmergingRef.current) {
      volcanoMaterial.uniforms.uEmergence.value = 1.0;
      return;
    }

    if (!groupRef.current) return;

    if (emergenceStartRef.current === null) {
      emergenceStartRef.current = state.clock.elapsedTime * 1000;
    }

    const elapsed = state.clock.elapsedTime * 1000 - emergenceStartRef.current;
    const duration = 1200;
    const t = Math.min(elapsed / duration, 1);
    const eased = easeOutCubic(t);

    volcanoMaterial.uniforms.uEmergence.value = eased;

    const shakeIntensity = 0.008 * (1 - eased);
    const shakeX = Math.sin(elapsed * 0.137 + seed) * shakeIntensity;
    const shakeY = Math.sin(elapsed * 0.173 + seed * 1.3) * shakeIntensity;
    groupRef.current.position.set(shakeX, shakeY, 0);

    if (t >= 1) {
      isEmergingRef.current = false;
      setShowSmoke(true);
      groupRef.current.position.set(0, 0, 0);
    }
  });

  return (
    <>
      <group ref={groupRef}>
        <mesh
          geometry={volcanoGeometry}
          material={volcanoMaterial}
          renderOrder={13}
          frustumCulled={false}
        />
      </group>

      {!nuclearImpacted && showSmoke && (
        <VolcanoSmoke seed={seed} craterHeight={VOLCANO_HEIGHT * 0.85} />
      )}

      {!nuclearImpacted && showDust && surfaceNormal && worldPosition && (
        <DustEffect
          seed={seed}
          duration={3000}
          particleColor={new THREE.Color(0.55, 0.18, 0.08)}
          onComplete={() => setShowDust(false)}
        />
      )}
    </>
  );
}
