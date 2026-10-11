import { randomSequence } from "../structureLayout";
import { useRef, useEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import { useTextures } from "../../../../hooks/useTextures";
import { scaledParticleCount } from "@/utils/graphicsQuality.ts";

interface VolcanoSmokeProps {
  seed: number;
  craterHeight: number;
}

interface SmokeParticle {
  sprite: THREE.Sprite;
  velocity: THREE.Vector3;
  rotationSpeed: number;
  age: number;
  maxAge: number;
  initialScale: number;
  delay: number;
}

export default function VolcanoSmoke({ craterHeight, seed }: VolcanoSmokeProps) {
  const random = useRef(randomSequence(seed));
  const groupRef = useRef<THREE.Group>(null);
  const particlesRef = useRef<SmokeParticle[]>([]);
  const { smoke: smokeTexture } = useTextures();

  const resetParticle = (p: SmokeParticle) => {
    const angle = random.current() * Math.PI * 2;
    const offset = random.current() * 0.012;
    p.sprite.position.set(Math.cos(angle) * offset, Math.sin(angle) * offset, 0);
    p.age = 0;
    p.maxAge = 9.0 + random.current() * 6.0;
    p.initialScale = 0.035 + random.current() * 0.025;
    p.sprite.scale.setScalar(p.initialScale);
    const spread = 0.001 + random.current() * 0.002;
    const newAngle = random.current() * Math.PI * 2;
    p.velocity.set(
      Math.cos(newAngle) * spread,
      Math.sin(newAngle) * spread,
      0.012 + random.current() * 0.008,
    );
    p.rotationSpeed = (random.current() - 0.5) * 0.4;
    (p.sprite.material as THREE.SpriteMaterial).opacity = 0;
    (p.sprite.material as THREE.SpriteMaterial).rotation = random.current() * Math.PI * 2;
  };

  useEffect(() => {
    const group = groupRef.current;
    if (!group) return;

    random.current = randomSequence(seed);
    const particles: SmokeParticle[] = [];
    const particleCount = scaledParticleCount(28);

    for (let i = 0; i < particleCount; i++) {
      const material = new THREE.SpriteMaterial({
        map: smokeTexture,
        transparent: true,
        opacity: 0,
        depthWrite: false,
        blending: THREE.NormalBlending,
        color: new THREE.Color(0.03, 0.025, 0.02),
      });

      const sprite = new THREE.Sprite(material);
      sprite.renderOrder = 100;
      group.add(sprite);

      const p: SmokeParticle = {
        sprite,
        velocity: new THREE.Vector3(),
        rotationSpeed: 0,
        age: 0,
        maxAge: 3,
        initialScale: 0.03,
        delay: (i / particleCount) * 3.5,
      };

      resetParticle(p);
      p.delay = (i / particleCount) * 12.0;
      particles.push(p);
    }

    particlesRef.current = particles;

    return () => {
      particles.forEach((p) => {
        group.remove(p.sprite);
        (p.sprite.material as THREE.Material).dispose();
      });
      particlesRef.current = [];
    };
  }, [smokeTexture, seed]);

  useFrame((_state, delta) => {
    for (const p of particlesRef.current) {
      if (p.delay > 0) {
        p.delay -= delta;
        continue;
      }

      p.age += delta;

      if (p.age >= p.maxAge) {
        resetParticle(p);
        continue;
      }

      const lifeRatio = p.age / p.maxAge;

      p.velocity.x += 0.008 * delta;
      p.velocity.y += 0.003 * delta;

      p.sprite.position.x += p.velocity.x * delta;
      p.sprite.position.y += p.velocity.y * delta;
      p.sprite.position.z += p.velocity.z * delta;

      const mat = p.sprite.material as THREE.SpriteMaterial;
      mat.rotation += p.rotationSpeed * delta;

      const fadeIn = Math.min(lifeRatio * 6, 1);
      const fadeOut = 1 - Math.pow(lifeRatio, 1.5);
      mat.opacity = fadeIn * fadeOut * 0.55;

      const scale = p.initialScale * (1 + lifeRatio * 5.0);
      p.sprite.scale.setScalar(scale);

      p.velocity.x *= 1 - 0.3 * delta;
      p.velocity.y *= 1 - 0.3 * delta;
    }
  });

  return <group ref={groupRef} position={[0, 0, craterHeight]} />;
}
