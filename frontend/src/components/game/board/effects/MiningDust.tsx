import { randomSequence } from "../structureLayout";
import { useRef, useEffect } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import { useTextures } from "../../../../hooks/useTextures";
import { scaledParticleCount } from "@/utils/graphicsQuality.ts";

interface MiningDustProps {
  seed: number;
  count?: number;
}

interface DustParticle {
  sprite: THREE.Sprite;
  velocity: THREE.Vector3;
  age: number;
  maxAge: number;
  initialScale: number;
  delay: number;
}

export default function MiningDust({ count = 18, seed }: MiningDustProps) {
  const random = useRef(randomSequence(seed));
  const groupRef = useRef<THREE.Group>(null);
  const particlesRef = useRef<DustParticle[]>([]);
  const { smoke: smokeTexture } = useTextures();

  const resetParticle = (p: DustParticle) => {
    const angle = random.current() * Math.PI * 2;
    const offset = random.current() * 0.08;
    p.sprite.position.set(Math.cos(angle) * offset, Math.sin(angle) * offset, 0.005);
    p.age = 0;
    p.maxAge = 4.0 + random.current() * 5.0;
    p.initialScale = 0.02 + random.current() * 0.015;
    p.sprite.scale.setScalar(p.initialScale);
    const spread = 0.001 + random.current() * 0.002;
    const newAngle = random.current() * Math.PI * 2;
    p.velocity.set(
      Math.cos(newAngle) * spread,
      Math.sin(newAngle) * spread,
      0.003 + random.current() * 0.004,
    );
    (p.sprite.material as THREE.SpriteMaterial).opacity = 0;
    (p.sprite.material as THREE.SpriteMaterial).rotation = random.current() * Math.PI * 2;
  };

  useEffect(() => {
    const group = groupRef.current;
    if (!group) return;

    random.current = randomSequence(seed);
    const particles: DustParticle[] = [];

    const spriteCount = scaledParticleCount(count);
    for (let i = 0; i < spriteCount; i++) {
      const material = new THREE.SpriteMaterial({
        map: smokeTexture,
        transparent: true,
        opacity: 0,
        depthWrite: false,
        blending: THREE.NormalBlending,
        color: new THREE.Color(0.45, 0.35, 0.2),
      });

      const sprite = new THREE.Sprite(material);
      sprite.renderOrder = 100;
      group.add(sprite);

      const p: DustParticle = {
        sprite,
        velocity: new THREE.Vector3(),
        age: 0,
        maxAge: 3,
        initialScale: 0.02,
        delay: (i / spriteCount) * 6.0,
      };

      resetParticle(p);
      p.delay = (i / spriteCount) * 8.0;
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
  }, [smokeTexture, count, seed]);

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

      p.sprite.position.x += p.velocity.x * delta;
      p.sprite.position.y += p.velocity.y * delta;
      p.sprite.position.z += p.velocity.z * delta;

      const mat = p.sprite.material as THREE.SpriteMaterial;
      mat.rotation += 0.2 * delta;

      const fadeIn = Math.min(lifeRatio * 5, 1);
      const fadeOut = 1 - Math.pow(lifeRatio, 1.5);
      mat.opacity = fadeIn * fadeOut * 0.35;

      const scale = p.initialScale * (1 + lifeRatio * 3.0);
      p.sprite.scale.setScalar(scale);
    }
  });

  return <group ref={groupRef} position={[0, 0, 0]} />;
}
