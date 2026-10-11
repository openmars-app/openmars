import { randomSequence } from "../structureLayout";
import { useRef, useMemo } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import { useTextures } from "../../../../hooks/useTextures";
import { usePrimitiveInstances } from "../PrimitiveManager";
import { scaledParticleCount } from "@/utils/graphicsQuality.ts";

interface MoholeSmokeProps {
  seed: number;
  isNewlyPlaced?: boolean;
  tileWorldMatrix: THREE.Matrix4;
}

interface SmokeParticle {
  position: THREE.Vector3;
  velocity: THREE.Vector3;
  rotationSpeed: number;
  rotation: number;
  age: number;
  maxAge: number;
  initialScale: number;
  delay: number;
  wavePhase: number;
}

const PARTICLE_COUNT = scaledParticleCount(25);

function createSmokeMaterial(smokeTexture: THREE.Texture | undefined): THREE.ShaderMaterial {
  return new THREE.ShaderMaterial({
    uniforms: {
      uTexture: { value: smokeTexture ?? null },
    },
    vertexShader: `
      varying vec2 vUv;
      varying vec3 vInstanceColor;

      void main() {
        vUv = uv;
        vInstanceColor = instanceColor;

        vec3 instancePos = vec3(
          instanceMatrix[3][0],
          instanceMatrix[3][1],
          instanceMatrix[3][2]
        );
        float scaleX = length(vec3(instanceMatrix[0][0], instanceMatrix[0][1], instanceMatrix[0][2]));

        vec4 mvInstancePos = modelViewMatrix * vec4(instancePos, 1.0);

        float rot = vInstanceColor.g * 6.283185;
        float cr = cos(rot);
        float sr = sin(rot);
        vec2 centered = position.xy;
        vec2 rotated = vec2(
          centered.x * cr - centered.y * sr,
          centered.x * sr + centered.y * cr
        );

        mvInstancePos.xy += rotated * scaleX;
        gl_Position = projectionMatrix * mvInstancePos;
      }
    `,
    fragmentShader: `
      uniform sampler2D uTexture;
      varying vec2 vUv;
      varying vec3 vInstanceColor;

      void main() {
        vec4 tex = texture2D(uTexture, vUv);
        float opacity = vInstanceColor.r;
        vec3 smokeColor = vec3(0.2, 0.18, 0.15);
        gl_FragColor = vec4(smokeColor * tex.rgb, tex.a * opacity);
      }
    `,
    transparent: true,
    depthWrite: false,
    side: THREE.DoubleSide,
    forceSinglePass: true,
  });
}

const _tmpLocalMatrix = new THREE.Matrix4();
const _tmpWorldPos = new THREE.Vector3();

export default function MoholeSmoke({ isNewlyPlaced, tileWorldMatrix, seed }: MoholeSmokeProps) {
  const { smoke: smokeTexture } = useTextures();
  const random = useRef(randomSequence(seed));
  const timeRef = useRef(0);

  const planeGeometry = useMemo(() => new THREE.PlaneGeometry(1, 1), []);
  const smokeMaterial = useMemo(() => createSmokeMaterial(smokeTexture), [smokeTexture]);

  const { setTransforms, setColors } = usePrimitiveInstances(
    "mohole-smoke",
    planeGeometry,
    smokeMaterial,
    100,
  );

  const matricesRef = useRef<THREE.Matrix4[]>([]);
  const colorsRef = useRef<THREE.Color[]>([]);
  const particlesRef = useRef<SmokeParticle[]>([]);

  if (particlesRef.current.length === 0) {
    const matrices: THREE.Matrix4[] = [];
    const colors: THREE.Color[] = [];
    const particles: SmokeParticle[] = [];

    for (let i = 0; i < PARTICLE_COUNT; i++) {
      matrices.push(new THREE.Matrix4());
      colors.push(new THREE.Color(0, 0, 0));

      const p: SmokeParticle = {
        position: new THREE.Vector3(),
        velocity: new THREE.Vector3(),
        rotationSpeed: 0,
        rotation: 0,
        age: 0,
        maxAge: 6.0 + random.current() * 4.0,
        initialScale: 0.025 + random.current() * 0.02,
        delay: 0,
        wavePhase: 0,
      };

      resetParticle(p, 0, random.current);
      if (isNewlyPlaced) {
        p.delay = 1.0 + (i / PARTICLE_COUNT) * 2.0;
      } else {
        p.delay = 0;
        p.age = random.current() * p.maxAge;
        p.wavePhase = -p.age;
      }
      particles.push(p);
    }

    matricesRef.current = matrices;
    colorsRef.current = colors;
    particlesRef.current = particles;
  }

  useFrame((_state, delta) => {
    timeRef.current += delta;
    const time = timeRef.current;

    const wave = Math.sin(time * 1.6) * 0.5 + 0.5;
    const waveBoost = 0.4 + wave * 0.6;

    const particles = particlesRef.current;
    const matrices = matricesRef.current;
    const colors = colorsRef.current;

    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];

      if (p.delay > 0) {
        p.delay -= delta;
        matrices[i].makeScale(0, 0, 0);
        colors[i].setRGB(0, 0, 0);
        continue;
      }

      p.age += delta;

      if (p.age >= p.maxAge) {
        resetParticle(p, time, random.current);
        continue;
      }

      const lifeRatio = p.age / p.maxAge;

      p.position.x += p.velocity.x * delta;
      p.position.y += p.velocity.y * delta;
      p.position.z += p.velocity.z * delta;

      p.rotation += p.rotationSpeed * delta;

      const birthWave = Math.sin(p.wavePhase * 1.6) * 0.5 + 0.5;
      const particleThickness = 0.4 + birthWave * 0.6;

      const fadeIn = Math.min(lifeRatio * 5, 1);
      const fadeOut = 1 - Math.pow(lifeRatio, 1.5);
      const opacity = fadeIn * fadeOut * 0.35 * particleThickness * waveBoost;

      const scale = p.initialScale * (1 + lifeRatio * 2.5);

      // Transform local particle position to world space
      _tmpWorldPos.copy(p.position);
      _tmpWorldPos.applyMatrix4(tileWorldMatrix);

      // Build instance matrix: world position + uniform scale
      _tmpLocalMatrix.makeScale(scale, scale, scale);
      _tmpLocalMatrix.setPosition(_tmpWorldPos);
      matrices[i].copy(_tmpLocalMatrix);

      const normalizedRotation = ((p.rotation % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);
      colors[i].setRGB(opacity, normalizedRotation / (Math.PI * 2), 0);

      p.velocity.x *= 1 - 0.3 * delta;
      p.velocity.y *= 1 - 0.3 * delta;
    }

    setTransforms(matrices);
    setColors(colors);
  });

  return null;
}

function resetParticle(p: SmokeParticle, time: number, random: () => number) {
  const spread = 0.018;
  const angle = random() * Math.PI * 2;
  const r = random() * spread;
  p.position.set(Math.cos(angle) * r, Math.sin(angle) * r, 0.005);
  p.age = 0;
  p.maxAge = 6.0 + random() * 4.0;
  p.initialScale = 0.025 + random() * 0.02;
  const drift = 0.0002 + random() * 0.0004;
  const driftAngle = random() * Math.PI * 2;
  p.velocity.set(
    Math.cos(driftAngle) * drift,
    Math.sin(driftAngle) * drift,
    0.005 + random() * 0.004,
  );
  p.rotationSpeed = (random() - 0.5) * 0.4;
  p.rotation = random() * Math.PI * 2;
  p.wavePhase = time;
}
