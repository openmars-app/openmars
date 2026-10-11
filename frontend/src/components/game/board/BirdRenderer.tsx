import { assetUrl } from "@/assets";
import { useEffect, useRef, useState } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import * as THREE from "three";
import { SkeletonUtils } from "three-stdlib";
import { useModels } from "../../../hooks/useModels";
import { useClimate } from "../../../contexts/ClimateContext";

const BIRD_SCALE = 0.0015;
const BIRDS_PER_TILE_MIN = 5;
const BIRDS_PER_TILE_EXTRA = 2;
const CLONES_PER_FRAME = 1;
const NOISE_SCROLL_SPEED = 0.03;
const NOISE_AMPLITUDE = 0.025;
const BOB_AMPLITUDE = 0.008;

function mulberry32(seed: number): () => number {
  return function () {
    let t = (seed += 0x6d2b79f5);
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function tileKey(q: number, r: number, s: number): string {
  return `${q},${r},${s}`;
}

function tileSeed(q: number, r: number, s: number): number {
  return Math.abs(q * 73856093) ^ (r * 19349663) ^ (s * 83492791);
}

// Load noise texture as CPU-readable pixel data (one-time)
let noiseData: { pixels: Uint8ClampedArray; width: number; height: number } | null = null;
let noiseLoadPromise: Promise<void> | null = null;

function loadNoiseTexture(): Promise<void> {
  if (noiseData) {
    return Promise.resolve();
  }
  if (noiseLoadPromise) {
    return noiseLoadPromise;
  }
  noiseLoadPromise = new Promise((resolve) => {
    const img = new Image();
    // The bundle may be served from another origin than the page; without CORS the canvas is tainted
    img.crossOrigin = "anonymous";
    img.onload = () => {
      const canvas = document.createElement("canvas");
      canvas.width = img.width;
      canvas.height = img.height;
      const ctx = canvas.getContext("2d")!;
      ctx.drawImage(img, 0, 0);
      const imageData = ctx.getImageData(0, 0, img.width, img.height);
      noiseData = { pixels: imageData.data, width: img.width, height: img.height };
      resolve();
    };
    img.src = assetUrl("textures/terrain/noise-mid");
  });
  return noiseLoadPromise;
}

// Sample noise texture with bilinear interpolation, returns 0..1
function sampleNoise(u: number, v: number): number {
  if (!noiseData) {
    return 0.5;
  }
  const { pixels, width, height } = noiseData;

  // Wrap to [0, 1)
  u = ((u % 1) + 1) % 1;
  v = ((v % 1) + 1) % 1;

  const fx = u * (width - 1);
  const fy = v * (height - 1);
  const x0 = Math.floor(fx);
  const y0 = Math.floor(fy);
  const x1 = (x0 + 1) % width;
  const y1 = (y0 + 1) % height;
  const dx = fx - x0;
  const dy = fy - y0;

  const p00 = pixels[(y0 * width + x0) * 4];
  const p10 = pixels[(y0 * width + x1) * 4];
  const p01 = pixels[(y1 * width + x0) * 4];
  const p11 = pixels[(y1 * width + x1) * 4];

  const top = p00 + (p10 - p00) * dx;
  const bot = p01 + (p11 - p01) * dx;
  return (top + (bot - top) * dy) / 255;
}

interface BirdTileData {
  coordinate: { q: number; r: number; s: number };
  worldPosition: THREE.Vector3;
  normal: THREE.Vector3;
}

interface BirdState {
  radius: number;
  radiusY: number;
  speed: number;
  phase: number;
  height: number;
  noiseU: number;
  noiseV: number;
  bobPhase: number;
  bobSpeed: number;
  center: THREE.Vector3;
  normal: THREE.Vector3;
  tx: THREE.Vector3;
  ty: THREE.Vector3;
  clone: THREE.Object3D;
  mixer: THREE.AnimationMixer;
  tileKey: string;
  index: number;
  count: number;
}

interface PendingBird {
  radius: number;
  radiusY: number;
  speed: number;
  phase: number;
  height: number;
  noiseU: number;
  noiseV: number;
  bobPhase: number;
  bobSpeed: number;
  animTimeScale: number;
  animStartTime: number;
  center: THREE.Vector3;
  normal: THREE.Vector3;
  tx: THREE.Vector3;
  ty: THREE.Vector3;
  tileKey: string;
  index: number;
  count: number;
}

interface BirdRendererProps {
  livingGreeneryTiles: BirdTileData[];
}

function buildTangentFrame(normal: THREE.Vector3) {
  const ref = Math.abs(normal.dot(_yAxis)) > 0.9 ? _xAxis : _yAxis;
  const tx = new THREE.Vector3().crossVectors(normal, ref).normalize();
  const ty = new THREE.Vector3().crossVectors(normal, tx).normalize();
  return { tx, ty };
}

export default function BirdRenderer({ livingGreeneryTiles }: BirdRendererProps) {
  const clock = useThree((s) => s.clock);
  const { birdScene, birdAnimations } = useModels();
  const groupRef = useRef<THREE.Group>(null);
  const knownTilesRef = useRef<Set<string>>(new Set());
  const allBirdsRef = useRef<BirdState[]>([]);
  const spawnQueueRef = useRef<PendingBird[]>([]);
  const [noiseReady, setNoiseReady] = useState(!!noiseData);
  const { runtime } = useClimate();

  useEffect(() => {
    if (!noiseReady) {
      void loadNoiseTexture().then(() => setNoiseReady(true));
    }
  }, [noiseReady]);

  useEffect(() => {
    const group = groupRef.current;
    if (!group) {
      return;
    }

    const nextKeys = new Set<string>();
    for (const tile of livingGreeneryTiles) {
      nextKeys.add(tileKey(tile.coordinate.q, tile.coordinate.r, tile.coordinate.s));
    }

    const removedKeys = new Set<string>();
    for (const key of knownTilesRef.current) {
      if (!nextKeys.has(key)) {
        removedKeys.add(key);
      }
    }
    if (removedKeys.size > 0) {
      const kept: BirdState[] = [];
      for (const bird of allBirdsRef.current) {
        if (removedKeys.has(bird.tileKey)) {
          bird.mixer.stopAllAction();
          bird.clone.removeFromParent();
        } else {
          kept.push(bird);
        }
      }
      allBirdsRef.current = kept;
      spawnQueueRef.current = spawnQueueRef.current.filter((p) => !removedKeys.has(p.tileKey));
    }

    for (const tile of livingGreeneryTiles) {
      const { q, r, s } = tile.coordinate;
      const key = tileKey(q, r, s);
      if (knownTilesRef.current.has(key)) {
        continue;
      }

      const rng = mulberry32(tileSeed(q, r, s) + 55555);
      const count = BIRDS_PER_TILE_MIN + Math.floor(rng() * BIRDS_PER_TILE_EXTRA);
      const { tx, ty } = buildTangentFrame(tile.normal);
      const clipDuration = birdAnimations[0]?.duration ?? 1;

      for (let i = 0; i < count; i++) {
        spawnQueueRef.current.push({
          radius: 0.04 + rng() * 0.06,
          radiusY: 0.03 + rng() * 0.05,
          speed: 0.6 + rng() * 0.8,
          phase: rng() * Math.PI * 2,
          height: 0.04 + rng() * 0.06,
          noiseU: rng(),
          noiseV: rng(),
          bobPhase: rng() * Math.PI * 2,
          bobSpeed: 0.4 + rng() * 0.6,
          animTimeScale: 3.2 + rng() * 1.6,
          animStartTime: rng() * clipDuration,
          center: tile.worldPosition.clone(),
          normal: tile.normal.clone(),
          tx: tx.clone(),
          ty: ty.clone(),
          tileKey: key,
          index: i,
          count,
        });
      }
    }

    knownTilesRef.current = nextKeys;
  }, [livingGreeneryTiles, birdScene, birdAnimations]);

  useFrame((_, delta) => {
    const group = groupRef.current;
    const queue = spawnQueueRef.current;

    if (group && queue.length > 0) {
      const batch = queue.splice(0, CLONES_PER_FRAME);
      for (const p of batch) {
        const clone = SkeletonUtils.clone(birdScene);
        clone.scale.setScalar(BIRD_SCALE);
        group.add(clone);

        const mixer = new THREE.AnimationMixer(clone);
        if (birdAnimations.length > 0) {
          const clip = birdAnimations[0];
          const action = mixer.clipAction(clip);
          action.timeScale = p.animTimeScale;
          action.time = p.animStartTime;
          action.play();
        }

        allBirdsRef.current.push({
          radius: p.radius,
          radiusY: p.radiusY,
          speed: p.speed,
          phase: p.phase,
          height: p.height,
          noiseU: p.noiseU,
          noiseV: p.noiseV,
          bobPhase: p.bobPhase,
          bobSpeed: p.bobSpeed,
          center: p.center,
          normal: p.normal,
          tx: p.tx,
          ty: p.ty,
          clone,
          mixer,
          tileKey: p.tileKey,
          index: p.index,
          count: p.count,
        });
      }
    }

    const t = clock.elapsedTime;
    const density = runtime.current.current.birds;

    for (const bird of allBirdsRef.current) {
      // Hiding by spawn index keeps each tile's flock stable as oxygen rises.
      bird.clone.visible = bird.index < Math.round(bird.count * density);
      if (!bird.clone.visible) {
        continue;
      }
      bird.mixer.update(delta);

      const angle = t * bird.speed + bird.phase;

      // Sample perlin noise at scrolling UV to perturb the orbit
      const noiseX = (sampleNoise(bird.noiseU + t * NOISE_SCROLL_SPEED, bird.noiseV) - 0.5) * 2;
      const noiseY =
        (sampleNoise(bird.noiseU, bird.noiseV + t * NOISE_SCROLL_SPEED * 0.7) - 0.5) * 2;
      const noiseH =
        (sampleNoise(bird.noiseU + 0.5, bird.noiseV + t * NOISE_SCROLL_SPEED * 0.5) - 0.5) * 2;

      const ox = Math.cos(angle) * bird.radius + noiseX * NOISE_AMPLITUDE;
      const oy = Math.sin(angle) * bird.radiusY + noiseY * NOISE_AMPLITUDE;
      const bob = Math.sin(t * bird.bobSpeed + bird.bobPhase) * BOB_AMPLITUDE;

      bird.clone.position
        .copy(bird.center)
        .addScaledVector(bird.normal, bird.height + noiseH * BOB_AMPLITUDE + bob)
        .addScaledVector(bird.tx, ox)
        .addScaledVector(bird.ty, oy);

      // Forward = derivative of position (tangent to path)
      // For the base ellipse: dx/dt = -sin(angle)*speed*radius, dy/dt = cos(angle)*speed*radiusY
      const fwdX = -Math.sin(angle) * bird.speed * bird.radius;
      const fwdY = Math.cos(angle) * bird.speed * bird.radiusY;

      const forward = _v0
        .copy(bird.tx)
        .multiplyScalar(fwdX)
        .addScaledVector(bird.ty, fwdY)
        .normalize();

      const right = _v1.crossVectors(bird.normal, forward).normalize();
      const up = _v2.crossVectors(forward, right).normalize();
      _basis.makeBasis(right, up, forward);
      bird.clone.quaternion.setFromRotationMatrix(_basis);
    }
  });

  return <group ref={groupRef} userData={{ perfGroup: "birds", tileHighlights: false }} />;
}

const _v0 = new THREE.Vector3();
const _v1 = new THREE.Vector3();
const _v2 = new THREE.Vector3();
const _basis = new THREE.Matrix4();
const _xAxis = new THREE.Vector3(1, 0, 0);
const _yAxis = new THREE.Vector3(0, 1, 0);
