import { randomSequence } from "../structureLayout";
import { useRef, useLayoutEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import { useTextures } from "../../../../hooks/useTextures";
import { scaledParticleCount } from "@/utils/graphicsQuality.ts";

const DUST_SPRITE_COUNT = scaledParticleCount(45);

interface DustEffectProps {
  seed?: number;
  duration?: number;
  particleColor?: THREE.Color;
  onComplete?: () => void;
}

interface SmokeParticle {
  sprite: THREE.Sprite;
  velocity: THREE.Vector3;
  rotationSpeed: number;
  maxLifetime: number;
  initialScale: number;
}

export default function DustEffect({
  duration = 2600,
  seed = 42,
  particleColor,
  onComplete,
}: DustEffectProps) {
  const particlesRef = useRef<SmokeParticle[]>([]);
  const startTimeRef = useRef<number | null>(null);
  const completedRef = useRef(false);
  const groupRef = useRef<THREE.Group>(null);
  const initialColor = useRef(particleColor ?? new THREE.Color(0.58, 0.43, 0.32));
  const { smoke: smokeTexture } = useTextures();

  useLayoutEffect(() => {
    const group = groupRef.current!;
    const random = randomSequence(seed);
    const particles: SmokeParticle[] = [];
    startTimeRef.current = null;
    completedRef.current = false;
    for (let i = 0; i < DUST_SPRITE_COUNT; i++) {
      const material = new THREE.SpriteMaterial({
        map: smokeTexture,
        transparent: true,
        opacity: 0.08,
        depthWrite: false,
        color: initialColor.current,
        rotation: random() * Math.PI * 2,
      });
      const sprite = new THREE.Sprite(material);
      const angle = random() * Math.PI * 2;
      const radius = Math.sqrt(random()) * 0.13;
      sprite.position.set(Math.cos(angle) * radius, Math.sin(angle) * radius, 0.008);
      sprite.renderOrder = 20;
      const initialScale = 0.045 + random() * 0.06;
      sprite.scale.setScalar(initialScale);
      const spread = 0.015 + random() * 0.025;
      group.add(sprite);
      particles.push({
        sprite,
        velocity: new THREE.Vector3(
          Math.cos(angle) * spread,
          Math.sin(angle) * spread,
          0.025 + random() * 0.035,
        ),
        rotationSpeed: (random() - 0.5) * 0.8,
        maxLifetime: (duration / 1000) * (0.65 + random() * 0.35),
        initialScale,
      });
    }
    particlesRef.current = particles;
    return () => {
      for (const particle of particles) {
        group.remove(particle.sprite);
        particle.sprite.material.dispose();
      }
      particlesRef.current = [];
    };
  }, [smokeTexture, duration, seed]);

  useFrame((state, delta) => {
    startTimeRef.current ??= state.clock.elapsedTime;
    const elapsed = state.clock.elapsedTime - startTimeRef.current;
    for (const particle of particlesRef.current) {
      const lifeRatio = Math.min(1, elapsed / particle.maxLifetime);
      particle.sprite.position.addScaledVector(particle.velocity, delta);
      particle.sprite.material.rotation += particle.rotationSpeed * delta;
      const fadeIn = Math.min(0.12 + lifeRatio * 10, 1);
      const fadeOut = 1 - lifeRatio * lifeRatio;
      particle.sprite.material.opacity = fadeIn * fadeOut * 0.65;
      particle.sprite.scale.setScalar(particle.initialScale * (1 + lifeRatio * 1.5));
      particle.velocity.multiplyScalar(Math.exp(-1.2 * delta));
    }
    if (elapsed >= duration / 1000 && !completedRef.current) {
      completedRef.current = true;
      onComplete?.();
    }
  });

  return <group ref={groupRef} />;
}
