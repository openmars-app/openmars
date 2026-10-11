import { useEffect, useMemo, useRef } from "react";
import { useFrame } from "@react-three/fiber";
import * as THREE from "three";
import { useTextures } from "@/hooks/useTextures";
import { scaledParticleCount } from "@/utils/graphicsQuality";
import { STRUCTURE_EMERGENCE_DURATION } from "../boardConstants";
import { boardCenter, projectBoardPoint } from "../landscapeGeometry";

import { randomSequence, type StructurePlot } from "../structureLayout";

export default function StructureEmissions({
  plot,
  emerging,
}: {
  plot: StructurePlot;
  emerging: boolean;
}) {
  const { smoke } = useTextures();
  const started = useRef<number | null>(null);
  const effect = useMemo(() => {
    const group = new THREE.Group();
    const random = randomSequence(plot.layout.seed ^ 0x88351);
    const center = plot.surface?.center ?? boardCenter(plot.coordinate);
    const inverse = new THREE.Quaternion()
      .setFromUnitVectors(new THREE.Vector3(0, 0, 1), plot.normal)
      .invert();
    const outlets = plot.layout.fixtures.flatMap((fixture) => {
      const emitter = fixture.emitter;
      if (!emitter) {
        return [];
      }
      return [
        {
          kind: emitter.kind,
          position: projectBoardPoint(
            center.x + emitter.x,
            center.y + emitter.y,
            emitter.z - 0.007,
            new THREE.Vector3(),
            plot.surface?.radius,
          )
            .sub(plot.worldPosition)
            .applyQuaternion(inverse),
        },
      ];
    });
    const particles = outlets.flatMap((outlet) =>
      Array.from({ length: scaledParticleCount(outlet.kind === "vapor" ? 5 : 8) }, () => {
        const material = new THREE.SpriteMaterial({
          map: smoke,
          color: outlet.kind === "vapor" ? "#cad5d8" : "#899097",
          opacity: 0,
          transparent: true,
          depthWrite: false,
        });
        const sprite = new THREE.Sprite(material);
        sprite.renderOrder = 100;
        group.add(sprite);
        return { sprite, outlet: outlet.position, vapor: outlet.kind === "vapor", phase: random() };
      }),
    );
    return { group, particles };
  }, [plot, smoke]);
  useEffect(() => {
    started.current = null;
    for (const { sprite } of effect.particles) {
      effect.group.add(sprite);
    }
    return () => {
      for (const { sprite } of effect.particles) {
        effect.group.remove(sprite);
        sprite.material.dispose();
      }
    };
  }, [effect]);
  useFrame(({ clock }) => {
    started.current ??= clock.elapsedTime;
    const age = clock.elapsedTime - started.current;
    const fade = emerging ? Math.max(0, Math.min(1, age - STRUCTURE_EMERGENCE_DURATION)) : 1;
    effect.group.visible = fade > 0;
    for (const { sprite, outlet, vapor, phase } of effect.particles) {
      const life = (clock.elapsedTime / (vapor ? 3.5 : 6) + phase) % 1;
      sprite.position.set(
        outlet.x + (vapor ? 0.01 : 0.017) * life * life,
        outlet.y + 0.006 * life,
        outlet.z + (vapor ? 0.025 : 0.055) * life,
      );
      sprite.scale.setScalar((vapor ? 0.005 : 0.007) + (vapor ? 0.013 : 0.021) * life);
      sprite.material.opacity = Math.sin(Math.PI * life) * (vapor ? 0.13 : 0.19) * fade;
      sprite.material.rotation = phase * Math.PI + life * 0.2;
    }
  });
  return <primitive object={effect.group} />;
}
