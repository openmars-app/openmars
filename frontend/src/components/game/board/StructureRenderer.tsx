import { useNuclearImpacted } from "./NuclearCollapse";
import { memo, useEffect, useMemo, useRef, useState } from "react";
import { useFrame } from "@react-three/fiber";
import StructureEmissions from "./effects/StructureEmissions";
import DustEffect from "./effects/DustEffect";
import { STRUCTURE_EMERGENCE_DURATION } from "./boardConstants";
import { Text } from "@react-three/drei";
import * as THREE from "three";
import { createGeometry, structureEmergenceDepth } from "./structureGeometry";
import { assetUrl } from "@/assets";
import { useTextures } from "../../../hooks/useTextures";
import { addSphereProjectionWithSoftEdges } from "./GreeneryRenderer";

import { randomSequence, type StructureStyle, type StructurePlot } from "./structureLayout";

const ORIGIN = new THREE.Vector3();
const materialCache = new Map<string, THREE.MeshStandardMaterial[]>();

export function getMaterials(
  lighting: StructureStyle["lighting"],
  textures: Pick<
    ReturnType<typeof useTextures>,
    "concrete" | "grass" | "sand" | "structureFacades"
  >,
) {
  const cacheKey =
    lighting +
    textures.structureFacades
      .map((f) => `${f.color.uuid}:${f.normal.uuid}:${f.roughness.uuid}`)
      .join(":");
  const cached = materialCache.get(cacheKey);
  if (cached) {
    return cached;
  }
  const materials = textures.structureFacades.map((facade, variant) => {
    const canvases = Array.from({ length: 4 }, () => {
      const canvas = document.createElement("canvas");
      canvas.width = 512;
      canvas.height = 1024;
      return canvas;
    });
    const [ctx, glow, normals, roughness] = canvases.map((canvas) => {
      const context = canvas.getContext("2d")!;
      context.scale(2, 2);
      return context;
    });
    ctx.drawImage(facade.color.image as CanvasImageSource, 0, 0, 256, 512);
    normals.drawImage(facade.normal.image as CanvasImageSource, 0, 0, 256, 512);
    roughness.drawImage(facade.roughness.image as CanvasImageSource, 0, 0, 256, 512);
    glow.fillStyle = "#000000";
    glow.fillRect(0, 0, 256, 512);
    const rng = randomSequence(475 + variant * 31);
    const width = [30, 36, 22][variant],
      height = [32, 26, 40][variant];
    for (let row = 0; row < 8; row++) {
      for (let column = 0; column < 4; column++) {
        const x = column * 64 + (64 - width) / 2;
        const y = row * 64 + (64 - height) / 2;
        const lit = rng() > (lighting === "bright" ? 0.15 : 0.58);
        ctx.fillStyle = "#52636b";
        ctx.fillRect(x - 2, y - 2, width + 4, height + 4);
        ctx.fillStyle = lit ? "#b9cbd0" : "#233e50";
        ctx.fillRect(x, y, width, height);
        ctx.fillStyle = lit ? "#e7d9a9" : "#416475";
        ctx.fillRect(x + 2, y + 2, width - 4, 5);
        normals.fillStyle = "#8080ff";
        normals.fillRect(x - 2, y - 2, width + 4, height + 4);
        roughness.fillStyle = "#555555";
        roughness.fillRect(x - 2, y - 2, width + 4, height + 4);
        if (lit) {
          glow.fillStyle = "#9caeb8";
          glow.fillRect(x, y, width, height);
          glow.fillStyle = "#f0cf89";
          glow.fillRect(x + 2, y + 2, width - 4, 5);
        }
      }
    }
    const [map, emissiveMap, normalMap, roughnessMap] = canvases.map((canvas, index) => {
      const texture = new THREE.CanvasTexture(canvas);
      texture.wrapS = texture.wrapT = THREE.RepeatWrapping;
      texture.colorSpace = index < 2 ? THREE.SRGBColorSpace : THREE.NoColorSpace;
      texture.anisotropy = 4;
      return texture;
    });
    return new THREE.MeshStandardMaterial({
      color: "#eef2f2",
      map,
      emissiveMap,
      normalMap,
      roughnessMap,
      normalScale: new THREE.Vector2(0.45, 0.45),
      emissive: "#ffffff",
      emissiveIntensity: lighting === "bright" ? 1.4 : 0.6,
      roughness: 0.95,
      metalness: 0.08,
    });
  });
  for (const [color, roughness, metalness] of [
    ["#68777d", 0.85, 0.1],
    ["#b8b6a9", 0.9, 0],
    ["#344650", 0.7, 0.2],
    ["#b67c47", 0.5, 0.6],
    ["#496d38", 0.95, 0],
    ["#244b6d", 0.35, 0.5],
  ] as const) {
    materials.push(
      new THREE.MeshStandardMaterial({
        color,
        roughness,
        metalness,
        emissive: color,
        emissiveIntensity: 0.22,
      }),
    );
  }
  materials.push(
    new THREE.MeshStandardMaterial({
      color: "#79b1bb",
      roughness: 0.25,
      metalness: 0.45,
      emissive: "#264754",
      emissiveIntensity: 0.25,
    }),
  );
  materials.push(
    new THREE.MeshStandardMaterial({
      color: "#8dced9",
      transparent: true,
      opacity: 0.3,
      depthWrite: false,
      roughness: 0.15,
      metalness: 0.25,
      emissive: "#488ba6",
      emissiveIntensity: 0.2,
    }),
  );
  materials.push(
    new THREE.MeshStandardMaterial({
      color: "#ffd899",
      emissive: "#ffc56c",
      emissiveIntensity: 2,
    }),
  );
  materials.push(
    new THREE.MeshStandardMaterial({
      color: "#1b252b",
      roughness: 0.9,
      emissive: "#1b252b",
      emissiveIntensity: 0.15,
      side: THREE.DoubleSide,
    }),
  );
  materials[4].side = THREE.DoubleSide;
  for (const index of [3, 4, 5]) {
    const facade = textures.structureFacades[(index + 2) % 3];
    materials[index].map = facade.color;
    materials[index].normalMap = facade.normal;
    materials[index].normalScale.set(0.35, 0.35);
    materials[index].roughnessMap = facade.roughness;
    materials[index].emissiveMap = facade.color;
  }
  materials[4].color.set("#a8a399");
  materials[7].map = textures.grass;
  materials[7].emissiveMap = textures.grass;
  materials.push(
    new THREE.MeshStandardMaterial({
      map: textures.concrete,
      bumpMap: textures.concrete,
      bumpScale: 0.00025,
      color: "#697175",
      roughness: 0.98,
      emissive: "#697175",
      emissiveMap: textures.concrete,
      emissiveIntensity: 0.25,
    }),
  );
  const pool = document.createElement("canvas");
  pool.width = pool.height = 64;
  const poolContext = pool.getContext("2d")!;
  const gradient = poolContext.createRadialGradient(32, 32, 0, 32, 32, 32);
  gradient.addColorStop(0, "rgba(255,255,255,0.8)");
  gradient.addColorStop(0.35, "rgba(255,255,255,0.4)");
  gradient.addColorStop(1, "rgba(255,255,255,0)");
  poolContext.fillStyle = gradient;
  poolContext.fillRect(0, 0, 64, 64);
  const poolMap = new THREE.CanvasTexture(pool);
  materials.push(
    new THREE.MeshStandardMaterial({
      map: poolMap,
      color: "#000000",
      transparent: true,
      opacity: 0.55,
      depthWrite: false,
    }),
  );
  materials.push(
    new THREE.MeshStandardMaterial({
      map: poolMap,
      color: "#edaf59",
      emissive: "#edaf59",
      emissiveMap: poolMap,
      emissiveIntensity: 1.2,
      transparent: true,
      opacity: 0.3,
      depthWrite: false,
    }),
  );
  materials.push(
    new THREE.MeshStandardMaterial({
      map: textures.sand,
      bumpMap: textures.sand,
      bumpScale: 0.0005,
      color: "#88694f",
      roughness: 1,
      emissive: "#88694f",
      emissiveMap: textures.sand,
      emissiveIntensity: 0.2,
    }),
  );
  materials.push(
    new THREE.MeshStandardMaterial({
      color: "#99aab0",
      roughness: 0.38,
      metalness: 0.75,
    }),
  );
  materials.push(
    new THREE.MeshStandardMaterial({
      color: "#dce5e8",
      roughness: 0.3,
      metalness: 0.25,
      side: THREE.DoubleSide,
    }),
  );
  materials.forEach((material) => {
    material.vertexColors = true;
  });
  materialCache.set(cacheKey, materials);
  return materials;
}

interface StructureRendererProps {
  renderBuildings?: boolean;
  plot: StructurePlot;
  sphereCenter?: THREE.Vector3;
  groupInverseMatrix?: THREE.Matrix4;
  showLabel?: boolean;
  connectedGround?: boolean;
  isNewlyPlaced?: boolean;
}

export function StructureGroundPatch({
  x = 0,
  y = 0,
  elevation = 0.0003,
  renderOrder = 11,
  radius,
  surface,
  sphereRadius,
  sphereCenter,
  groupInverseMatrix,
  frustumCulled = true,
}: {
  x?: number;
  y?: number;
  elevation?: number;
  renderOrder?: number;
  radius: number;
  surface: "soil" | "grass" | "paving" | "asphalt";
  sphereRadius?: number;
  sphereCenter: THREE.Vector3;
  groupInverseMatrix?: THREE.Matrix4;
  frustumCulled?: boolean;
}) {
  const { sand, grass, concrete, noiseMid, noiseHigh } = useTextures();
  const geometry = useMemo(() => {
    const geo = new THREE.PlaneGeometry(radius * 3, radius * 3, 24, 24);
    const uv = geo.getAttribute("uv");
    for (let i = 0; i < uv.count; i++) {
      uv.setXY(i, uv.getX(i) * 3, uv.getY(i) * 3);
    }
    return geo;
  }, [radius]);
  const material = useMemo(() => {
    let map = sand;
    let color = "#9a6b4b";
    let opacity = 0.72;
    if (surface === "grass") {
      map = grass;
      color = "#75865c";
      opacity = 0.95;
    } else if (surface === "asphalt") {
      map = concrete;
      color = "#8b9294";
      opacity = 1;
    } else if (surface === "paving") {
      map = concrete;
      color = "#6c7779";
      opacity = 1;
    }
    const mat = new THREE.MeshStandardMaterial({
      map,
      color,
      opacity,
      bumpMap: map,
      bumpScale: 0.0005,
      roughness: 0.95,
      transparent: true,
      depthWrite: false,
      alphaTest: 0.005,
      side: THREE.DoubleSide,
      forceSinglePass: true,
    });
    const paved = surface === "paving";
    addSphereProjectionWithSoftEdges(
      mat,
      elevation,
      noiseMid,
      noiseHigh,
      radius,
      sphereCenter,
      groupInverseMatrix,
      {
        sphereRadius,
        circular: true,
        overflow: paved ? 0 : 0.12,
        bandWidth: paved ? 0.015 : 0.4,
        warp: paved ? 0 : 0.22,
      },
    );
    return mat;
  }, [
    sand,
    grass,
    concrete,
    noiseMid,
    noiseHigh,
    surface,
    sphereRadius,
    radius,
    elevation,
    sphereCenter,
    groupInverseMatrix,
  ]);
  useEffect(
    () => () => {
      geometry.dispose();
      material.dispose();
    },
    [geometry, material],
  );
  return (
    <mesh
      position={[x, y, 0]}
      geometry={geometry}
      material={material}
      renderOrder={renderOrder}
      frustumCulled={frustumCulled}
      raycast={() => {}}
      dispose={null}
    />
  );
}

function StructureRenderer({
  plot,
  sphereCenter = ORIGIN,
  groupInverseMatrix,
  showLabel = false,
  connectedGround = false,
  isNewlyPlaced = false,
  renderBuildings = true,
}: StructureRendererProps) {
  const nuclearImpacted = useNuclearImpacted();
  const buildings = useRef<THREE.Group>(null);
  const started = useRef<number | null>(null);
  const emerging = useRef(isNewlyPlaced);
  const [dust, setDust] = useState(isNewlyPlaced);
  useEffect(() => {
    if (isNewlyPlaced) {
      started.current = null;
      emerging.current = true;
      setDust(true);
    }
  }, [isNewlyPlaced]);
  useFrame(({ clock }) => {
    if (!buildings.current || !emerging.current) {
      return;
    }
    started.current ??= clock.elapsedTime;
    const t = Math.min(1, (clock.elapsedTime - started.current) / STRUCTURE_EMERGENCE_DURATION);
    buildings.current.position.z = -emergenceDepth * Math.pow(1 - t, 3);
    if (t === 1) {
      emerging.current = false;
    }
  });
  const geometries = useMemo(
    () => (renderBuildings ? createGeometry(plot.layout, plot) : []),
    [plot, renderBuildings],
  );
  const emergenceDepth = useMemo(() => structureEmergenceDepth(geometries), [geometries]);
  useEffect(
    () => () => {
      for (const geometry of geometries) {
        geometry?.dispose();
      }
    },
    [geometries],
  );
  const textures = useTextures();
  const structureMaterials = getMaterials(plot.layout.style.lighting, textures);
  let groundSurface: "soil" | "grass" | "paving" | "asphalt" = "soil";
  if (plot.layout.style.cover === "dome") {
    groundSurface = "paving";
  } else if (plot.layout.style.density === "dense") {
    groundSurface = "asphalt";
  } else if (plot.layout.style.landscaping !== "sparse") {
    groundSurface = "grass";
  }
  const dirtPatches = useMemo(() => {
    if (plot.layout.style.exposure !== "mostly-buried" || plot.layout.style.ground !== "recessed") {
      return [];
    }
    const random = randomSequence(4271);
    return Array.from({ length: 13 }, (_, i) => {
      const angle = (i / 13) * Math.PI * 2 + random() * 0.3;
      const distance = 0.065 + random() * 0.055;
      return {
        x: Math.cos(angle) * distance,
        y: Math.sin(angle) * distance,
        radius: 0.014 + random() * 0.022,
      };
    });
  }, [plot.layout]);
  const quaternion = useMemo(
    () => new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 0, 1), plot.normal),
    [plot.normal],
  );

  return (
    <group position={plot.worldPosition} quaternion={quaternion}>
      {!connectedGround && plot.layout.style.cover === "dome" && (
        <StructureGroundPatch
          radius={0.15}
          surface="asphalt"
          elevation={0.0002}
          renderOrder={10}
          sphereRadius={plot.surface?.radius}
          sphereCenter={sphereCenter}
          groupInverseMatrix={groupInverseMatrix}
        />
      )}
      {(!connectedGround || plot.layout.style.ground === "recessed") && (
        <StructureGroundPatch
          radius={plot.layout.style.cover === "dome" ? 0.113 : 0.155}
          surface={groundSurface}
          sphereRadius={plot.surface?.radius}
          sphereCenter={sphereCenter}
          groupInverseMatrix={groupInverseMatrix}
        />
      )}
      {dirtPatches.map((patch, i) => (
        <StructureGroundPatch
          key={`dirt-${i}`}
          {...patch}
          surface="soil"
          elevation={0.0005}
          sphereRadius={plot.surface?.radius}
          sphereCenter={sphereCenter}
          groupInverseMatrix={groupInverseMatrix}
        />
      ))}
      {groundSurface !== "grass" &&
        plot.layout.parks.map((park, i) => (
          <StructureGroundPatch
            key={i}
            x={park.x}
            y={park.y}
            elevation={0.0004}
            radius={park.radius}
            surface="grass"
            sphereRadius={plot.surface?.radius}
            sphereCenter={sphereCenter}
            groupInverseMatrix={groupInverseMatrix}
          />
        ))}
      <group ref={buildings}>
        {geometries.map(
          (geometry, index) =>
            geometry && (
              <mesh
                key={index}
                geometry={geometry}
                material={structureMaterials[index]}
                renderOrder={15}
                raycast={() => {}}
                dispose={null}
              />
            ),
        )}
      </group>
      {!nuclearImpacted && plot.layout.fixtures.some((fixture) => fixture.emitter != null) && (
        <StructureEmissions plot={plot} emerging={isNewlyPlaced} />
      )}
      {!nuclearImpacted && dust && (
        <DustEffect seed={plot.layout.seed} duration={3000} onComplete={() => setDust(false)} />
      )}
      {!nuclearImpacted && showLabel && (
        <Text
          position={[0, -0.137, 0.005]}
          font={assetUrl("fonts/prototype")}
          fontSize={0.01}
          maxWidth={0.18}
          color="#fff2d5"
          anchorX="center"
          anchorY="middle"
          raycast={() => {}}
        >
          {plot.layout.name}
        </Text>
      )}
    </group>
  );
}

export default memo(StructureRenderer);
