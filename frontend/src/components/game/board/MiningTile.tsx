import { useNuclearImpacted } from "./NuclearCollapse";
import { useRef, useState, useMemo, useEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import MiningDust from "./effects/MiningDust";
import DustEffect from "./effects/DustEffect";
import { useTextures } from "../../../hooks/useTextures";
import { addSphereProjectionWithSoftEdges } from "./GreeneryRenderer";
import { easeOutCubic } from "./boardConstants";

interface MiningTileProps {
  isNewlyPlaced?: boolean;
  seed?: number;
  surfaceNormal?: THREE.Vector3;
  worldPosition?: THREE.Vector3;
  sphereCenter?: THREE.Vector3;
  groupInverseMatrix?: THREE.Matrix4;
}

const HEX_RADIUS = 0.166;

// Track runs from the outside (high) down to the cave mouth (low)
const TRACK_LENGTH = 0.13;
const GROUND_Z = 0.004;
const CAVE_Z = -0.02;
const SLOPE_ANGLE = Math.atan2(CAVE_Z - GROUND_Z, TRACK_LENGTH);

// t=0 is outside (top of hill), t=1 is cave mouth (bottom)
function trackY(t: number): number {
  return -TRACK_LENGTH * (1 - t);
}

function trackZ(t: number): number {
  return GROUND_Z + (CAVE_Z - GROUND_Z) * t;
}

function mulberry32(seed: number): () => number {
  return function () {
    let t = (seed += 0x6d2b79f5);
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
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

function createCaveArchGeometry(): THREE.BufferGeometry {
  const archShape = new THREE.Shape();
  const w = 0.035;
  const h = 0.04;

  archShape.moveTo(-w, 0);
  archShape.lineTo(-w, h * 0.5);
  archShape.quadraticCurveTo(-w, h, 0, h);
  archShape.quadraticCurveTo(w, h, w, h * 0.5);
  archShape.lineTo(w, 0);
  archShape.lineTo(-w, 0);

  const geo = new THREE.ExtrudeGeometry(archShape, { depth: 0.04, bevelEnabled: false });
  // Arch face at Y=0, tunnel extends into +Y
  geo.rotateX(Math.PI / 2);
  return geo;
}

// Solid hillside behind the arch — hides the cart when it enters
function createHillGeometry(): THREE.BufferGeometry {
  const shape = new THREE.Shape();
  const hw = 0.05;
  shape.moveTo(-hw, -0.03);
  shape.lineTo(-hw, 0.06);
  shape.lineTo(hw, 0.06);
  shape.lineTo(hw, -0.03);
  shape.lineTo(-hw, -0.03);

  const geo = new THREE.ExtrudeGeometry(shape, { depth: 0.06, bevelEnabled: false });
  geo.rotateX(Math.PI / 2);
  return geo;
}

export default function MiningTile({
  isNewlyPlaced = false,
  seed: seedProp,
  surfaceNormal,
  worldPosition,
  sphereCenter,
  groupInverseMatrix,
}: MiningTileProps) {
  const groupRef = useRef<THREE.Group>(null);
  const cartRef = useRef<THREE.Group>(null);
  const cartTiltRef = useRef<THREE.Group>(null);
  const oreGroupRef = useRef<THREE.Group>(null);
  const emergenceStartRef = useRef<number | null>(null);
  const isEmergingRef = useRef(isNewlyPlaced);
  const nuclearImpacted = useNuclearImpacted();
  const [showDust, setShowDust] = useState(isNewlyPlaced);
  const [emergenceComplete, setEmergenceComplete] = useState(!isNewlyPlaced);

  useEffect(() => {
    if (isNewlyPlaced) {
      isEmergingRef.current = true;
      setShowDust(true);
      setEmergenceComplete(false);
      emergenceStartRef.current = null;
    }
  }, [isNewlyPlaced]);

  const seed = useMemo(() => seedProp ?? Math.random() * 10000, [seedProp]);

  const caveAngle = useMemo(() => {
    const rng = mulberry32(seed + 10);
    return rng() * Math.PI * 2;
  }, [seed]);

  const groundGeometry = useMemo(() => {
    const geo = createRadialDiscGeometry(HEX_RADIUS * 1.8, 10, 36);
    geo.rotateZ(Math.PI / 2);
    return geo;
  }, []);

  const caveArchGeometry = useMemo(() => createCaveArchGeometry(), []);
  const hillGeometry = useMemo(() => createHillGeometry(), []);

  const { noiseMid: noiseTexture, noiseHigh: noiseHighTexture } = useTextures();

  const groundMaterial = useMemo(() => {
    const mat = new THREE.MeshStandardMaterial({
      color: new THREE.Color(0.35, 0.22, 0.12),
      roughness: 0.95,
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

  const emergenceRef = useRef(isNewlyPlaced ? 0 : 1);
  const prevCycleRef = useRef(0.5);
  const goingDownRef = useRef(false);

  useFrame((state) => {
    if (nuclearImpacted) {
      return;
    }
    if (isEmergingRef.current) {
      if (!groupRef.current) return;

      if (emergenceStartRef.current === null) {
        emergenceStartRef.current = state.clock.elapsedTime * 1000;
      }

      const elapsed = state.clock.elapsedTime * 1000 - emergenceStartRef.current;
      const duration = 900;
      const rawT = Math.min(elapsed / duration, 1);
      const eased = easeOutCubic(rawT);
      emergenceRef.current = eased;

      const shakeIntensity = 0.01 * (1 - eased);
      const shakeX = Math.sin(elapsed * 0.137 + seed) * shakeIntensity;
      const shakeY = Math.sin(elapsed * 0.173 + seed * 1.3) * shakeIntensity;
      groupRef.current.position.set(shakeX, shakeY, 0);

      if (rawT >= 1) {
        isEmergingRef.current = false;
        setEmergenceComplete(true);
        groupRef.current.position.set(0, 0, 0);
      }
    }

    // Animate cart along the sloped track
    // Cart goes from outside (t=0) down to cave mouth (t=1) and a bit beyond
    if (cartRef.current) {
      const time = state.clock.elapsedTime;
      // cycle: 0→1→0 smooth ping-pong. At 1 the cart is at the cave mouth
      const cycle = (Math.sin(time * 1.2) + 1) / 2;
      // Extend slightly past t=1 so the cart disappears into the hill
      const t = cycle * 1.3;
      cartRef.current.position.y =
        trackY(Math.min(t, 1)) + (t > 1 ? (t - 1) * TRACK_LENGTH * 0.3 : 0);
      cartRef.current.position.z =
        trackZ(Math.min(t, 1)) + 0.005 + (t > 1 ? (t - 1) * (CAVE_Z - GROUND_Z) * 0.3 : 0);
      cartRef.current.scale.setScalar(emergenceRef.current);

      // Track direction: going down (toward cave) vs coming up (from cave)
      const goingDown = cycle > prevCycleRef.current;
      if (Math.abs(cycle - prevCycleRef.current) > 0.001) {
        goingDownRef.current = goingDown;
      }
      prevCycleRef.current = cycle;

      // Tip cart and dump ore at the top of the track
      if (cartTiltRef.current) {
        const dumpZone = 0.1;
        if (cycle < dumpZone && !goingDownRef.current) {
          const dumpT = 1 - cycle / dumpZone;
          cartTiltRef.current.rotation.y = dumpT * 0.8;
        } else {
          cartTiltRef.current.rotation.y = 0;
        }
      }

      // Show ore only when coming up from cave, hide after dumping at top
      if (oreGroupRef.current) {
        if (goingDownRef.current) {
          oreGroupRef.current.visible = false;
        } else {
          oreGroupRef.current.visible = cycle > 0.12;
        }
      }
    }
  });

  const groundQuaternion = useMemo(() => new THREE.Quaternion(), []);

  const oreRocks = useMemo(() => {
    const rng = mulberry32(seed + 500);
    const rocks: { x: number; z: number; scale: number; color: THREE.Color }[] = [];
    for (let i = 0; i < 5; i++) {
      rocks.push({
        x: (rng() - 0.5) * 0.012,
        z: (rng() - 0.5) * 0.008,
        scale: 0.003 + rng() * 0.003,
        color: new THREE.Color().setHSL(0.07 + rng() * 0.05, 0.4 + rng() * 0.3, 0.3 + rng() * 0.2),
      });
    }
    return rocks;
  }, [seed]);

  return (
    <>
      <group ref={groupRef}>
        {/* Ground */}
        <mesh
          geometry={groundGeometry}
          material={groundMaterial}
          quaternion={groundQuaternion}
          renderOrder={12}
        />

        {/* Everything rotated to random cave direction */}
        <group rotation={[0, 0, caveAngle - Math.PI / 2]}>
          {/* Solid hill behind cave arch — occludes the cart */}
          <mesh geometry={hillGeometry} position={[0, 0.045, CAVE_Z]} renderOrder={12}>
            <meshStandardMaterial
              color={new THREE.Color(0.32, 0.2, 0.11)}
              roughness={0.95}
              metalness={0.0}
            />
          </mesh>

          {/* Cave entrance arch at bottom of slope (Y=0, Z=CAVE_Z) */}
          <mesh geometry={caveArchGeometry} position={[0, 0, CAVE_Z]} renderOrder={14}>
            <meshStandardMaterial
              color={new THREE.Color(0.25, 0.16, 0.09)}
              roughness={0.9}
              metalness={0.05}
              side={THREE.DoubleSide}
            />
          </mesh>

          {/* Dark interior visible through arch */}
          <mesh
            position={[0, 0.038, CAVE_Z + 0.018]}
            rotation={[Math.PI / 2, 0, 0]}
            renderOrder={11}
          >
            <planeGeometry args={[0.065, 0.04]} />
            <meshStandardMaterial color={new THREE.Color(0.03, 0.02, 0.01)} roughness={1.0} />
          </mesh>

          {/* Earthen ramp walls on each side of the track */}
          {[-1, 1].map((side) => {
            const wallVerts: number[] = [];
            const wallIndices: number[] = [];
            const steps = 10;
            for (let i = 0; i <= steps; i++) {
              const t = i / steps;
              const y = trackY(t);
              const z = trackZ(t);
              wallVerts.push(side * 0.018, y, z - 0.003);
              wallVerts.push(side * 0.018, y, Math.max(z + 0.001, GROUND_Z));
            }
            for (let i = 0; i < steps; i++) {
              const bl = i * 2;
              const br = (i + 1) * 2;
              const tl = i * 2 + 1;
              const tr = (i + 1) * 2 + 1;
              if (side > 0) {
                wallIndices.push(bl, br, tl, tl, br, tr);
              } else {
                wallIndices.push(bl, tl, br, tl, tr, br);
              }
            }
            const geo = new THREE.BufferGeometry();
            geo.setAttribute("position", new THREE.Float32BufferAttribute(wallVerts, 3));
            geo.setIndex(wallIndices);
            geo.computeVertexNormals();
            return (
              <mesh key={`wall-${side}`} geometry={geo} renderOrder={13}>
                <meshStandardMaterial
                  color={new THREE.Color(0.3, 0.18, 0.1)}
                  roughness={0.95}
                  metalness={0.0}
                  side={THREE.DoubleSide}
                />
              </mesh>
            );
          })}

          {/* Rail ties (wooden sleepers) along slope */}
          {Array.from({ length: 12 }).map((_, i) => {
            const t = (i + 0.5) / 12;
            const y = trackY(t);
            const z = trackZ(t);
            return (
              <mesh
                key={`tie-${i}`}
                position={[0, y, z]}
                rotation={[SLOPE_ANGLE, 0, 0]}
                renderOrder={13}
              >
                <boxGeometry args={[0.028, 0.004, 0.0015]} />
                <meshStandardMaterial color="#3a2a1a" roughness={0.9} metalness={0.0} />
              </mesh>
            );
          })}

          {/* Rail tracks — two steel rails along slope */}
          {[-0.009, 0.009].map((xOff, i) => {
            const midY = trackY(0.5);
            const midZ = trackZ(0.5);
            return (
              <mesh
                key={`rail-${i}`}
                position={[xOff, midY, midZ + 0.001]}
                rotation={[SLOPE_ANGLE, 0, 0]}
                renderOrder={13}
              >
                <boxGeometry args={[0.003, TRACK_LENGTH, 0.002]} />
                <meshStandardMaterial color="#5a5a5a" roughness={0.3} metalness={0.7} />
              </mesh>
            );
          })}

          {/* Mine cart — animated along slope, disappears into hill */}
          <group ref={cartRef} position={[0, trackY(0), trackZ(0) + 0.005]}>
            <group ref={cartTiltRef}>
              <group rotation={[SLOPE_ANGLE, 0, 0]}>
                <mesh renderOrder={13}>
                  <boxGeometry args={[0.02, 0.016, 0.012]} />
                  <meshStandardMaterial color="#666666" roughness={0.5} metalness={0.5} />
                </mesh>
                {[
                  [-0.009, -0.006],
                  [0.009, -0.006],
                  [-0.009, 0.006],
                  [0.009, 0.006],
                ].map(([wx, wy], wi) => (
                  <mesh
                    key={`wheel-${wi}`}
                    position={[wx, wy, -0.004]}
                    rotation={[0, Math.PI / 2, 0]}
                    renderOrder={13}
                  >
                    <cylinderGeometry args={[0.003, 0.003, 0.003, 6]} />
                    <meshStandardMaterial
                      color="#333333"
                      roughness={0.5}
                      metalness={0.5}
                      side={THREE.DoubleSide}
                    />
                  </mesh>
                ))}
                <group ref={oreGroupRef}>
                  {oreRocks.map((rock, ri) => (
                    <mesh key={`ore-${ri}`} position={[rock.x, rock.z, 0.008]} renderOrder={13}>
                      <dodecahedronGeometry args={[rock.scale, 0]} />
                      <meshStandardMaterial color={rock.color} roughness={0.8} metalness={0.2} />
                    </mesh>
                  ))}
                </group>
              </group>
            </group>
          </group>

          {/* Small ore pile at top of track */}
          <mesh
            position={[0.03, trackY(0) + 0.01, GROUND_Z]}
            rotation={[-Math.PI / 2, 0, 0]}
            renderOrder={13}
          >
            <sphereGeometry args={[0.015, 8, 6, 0, Math.PI * 2, 0, Math.PI / 2]} />
            <meshStandardMaterial
              color={new THREE.Color(0.35, 0.25, 0.15)}
              roughness={0.95}
              metalness={0.1}
            />
          </mesh>
        </group>
      </group>

      {emergenceComplete && <MiningDust seed={seed} />}

      {!nuclearImpacted && showDust && surfaceNormal && worldPosition && (
        <DustEffect
          seed={seed}
          duration={2500}
          particleColor={new THREE.Color(0.45, 0.3, 0.15)}
          onComplete={() => setShowDust(false)}
        />
      )}
    </>
  );
}
