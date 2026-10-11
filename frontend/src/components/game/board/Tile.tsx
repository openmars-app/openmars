import type { NuclearSite } from "./nuclearDebris";
import { assetUrl } from "@/assets";
import { useRef, useState, useMemo, useEffect, memo, type RefObject } from "react";
import { useFrame } from "@react-three/fiber";
import { Text } from "@react-three/drei";
import * as THREE from "three";
import { HexTile2D } from "../../../utils/hex-grid-2d";
import VolcanoTile from "./VolcanoTile";
import NuclearZoneTile from "./NuclearZoneTile";
import { NuclearCollapse } from "./NuclearCollapse";
import type { NuclearTransition } from "./nuclearTransitions";
import { useTileHighlight } from "./TileHighlightContext";
import MiningTile from "./MiningTile";
import ReservedAreaTile from "./ReservedAreaTile";
import WorldTreeTile from "./WorldTreeTile";
import MoholeTile from "./MoholeTile";
import { useTextures } from "../../../hooks/useTextures";
import {
  sphereProjectionVertex,
  oceanBorderFragment,
  tileBorderVertex,
  tileBorderFragment,
  tileSurfaceVertexSnippet,
  splitSnippet,
} from "./shaders";
import { SPHERE_RADIUS, CHROME_Z_BASE, easeOutCubic } from "./boardConstants";

const BONUS_ICON_TINT = new THREE.Color(0.7, 0.7, 0.7);
const EMPTY_HEX_OPACITY = 0.12;
const ORIGIN = new THREE.Vector3(0, 0, 0);
const _bbWorldPos = new THREE.Vector3();
const _bbNormal = new THREE.Vector3();
const _bbToCamera = new THREE.Vector3();
const HOVER_BORDER_COLOR = new THREE.Color("#ffffff");
let _bbOpacity = 1;

// A shared traverse callback, so fading billboards allocates nothing per frame.
function applyBillboardOpacity(child: THREE.Object3D) {
  if (!(child instanceof THREE.Mesh) || !child.material) {
    return;
  }
  if (Array.isArray(child.material)) {
    for (const mat of child.material) {
      mat.opacity = _bbOpacity;
    }
  } else {
    child.material.opacity = _bbOpacity;
  }
}

const GLOW_CUTOFF = 0.002;

function createSubdividedHexagonGeometry(radius: number, rings: number): THREE.BufferGeometry {
  const geometry = new THREE.BufferGeometry();
  const vertices: number[] = [];
  const uvs: number[] = [];
  const indices: number[] = [];

  vertices.push(0, 0, 0);
  uvs.push(0.5, 0.5);

  for (let ring = 1; ring <= rings; ring++) {
    const ringRadius = (ring / rings) * radius;
    const verticesInRing = 6 * ring;

    for (let i = 0; i < verticesInRing; i++) {
      const edgeIndex = Math.floor(i / ring);
      const posOnEdge = i % ring;

      const angle1 = (edgeIndex * Math.PI) / 3;
      const angle2 = ((edgeIndex + 1) * Math.PI) / 3;

      const t = posOnEdge / ring;
      const x = ringRadius * (Math.cos(angle1) * (1 - t) + Math.cos(angle2) * t);
      const y = ringRadius * (Math.sin(angle1) * (1 - t) + Math.sin(angle2) * t);

      vertices.push(x, y, 0);
      uvs.push(0.5 + (x / radius) * 0.5, 0.5 + (y / radius) * 0.5);
    }
  }

  for (let i = 0; i < 6; i++) {
    const next = (i + 1) % 6;
    indices.push(0, 1 + i, 1 + next);
  }

  let prevRingStart = 1;
  for (let ring = 2; ring <= rings; ring++) {
    const currRingStart = prevRingStart + 6 * (ring - 1);
    const prevRingVerts = 6 * (ring - 1);
    const currRingVerts = 6 * ring;

    let prevIdx = 0;
    let currIdx = 0;

    for (let edge = 0; edge < 6; edge++) {
      for (let i = 0; i < ring; i++) {
        const curr0 = currRingStart + currIdx;
        const curr1 = currRingStart + ((currIdx + 1) % currRingVerts);

        if (i < ring - 1) {
          const prev0 = prevRingStart + prevIdx;
          const prev1 = prevRingStart + ((prevIdx + 1) % prevRingVerts);

          indices.push(prev0, curr0, curr1);
          indices.push(prev0, curr1, prev1);
          prevIdx++;
        } else {
          const prev0 = prevRingStart + (prevIdx % prevRingVerts);
          indices.push(prev0, curr0, curr1);
        }
        currIdx++;
      }
    }

    prevRingStart = currRingStart;
  }

  geometry.setAttribute("position", new THREE.Float32BufferAttribute(vertices, 3));
  geometry.setAttribute("uv", new THREE.Float32BufferAttribute(uvs, 2));
  geometry.setIndex(indices);
  geometry.computeVertexNormals();

  return geometry;
}

function createSubdividedHexRingGeometry(
  innerRadius: number,
  outerRadius: number,
  segmentsPerEdge: number,
): THREE.BufferGeometry {
  const geometry = new THREE.BufferGeometry();
  const vertices: number[] = [];
  const uvs: number[] = [];
  const indices: number[] = [];

  const totalSegments = 6 * segmentsPerEdge;

  for (let ring = 0; ring <= 1; ring++) {
    const radius = ring === 0 ? innerRadius : outerRadius;

    for (let edge = 0; edge < 6; edge++) {
      const angle1 = (edge * Math.PI) / 3;
      const angle2 = ((edge + 1) * Math.PI) / 3;

      const corner1 = { x: Math.cos(angle1) * radius, y: Math.sin(angle1) * radius };
      const corner2 = { x: Math.cos(angle2) * radius, y: Math.sin(angle2) * radius };

      for (let seg = 0; seg < segmentsPerEdge; seg++) {
        const t = seg / segmentsPerEdge;
        const x = corner1.x * (1 - t) + corner2.x * t;
        const y = corner1.y * (1 - t) + corner2.y * t;

        vertices.push(x, y, 0);
        uvs.push(0.5 + (x / outerRadius) * 0.5, 0.5 + (y / outerRadius) * 0.5);
      }
    }
  }

  const innerStart = 0;
  const outerStart = totalSegments;

  for (let i = 0; i < totalSegments; i++) {
    const next = (i + 1) % totalSegments;

    const inner0 = innerStart + i;
    const inner1 = innerStart + next;
    const outer0 = outerStart + i;
    const outer1 = outerStart + next;

    indices.push(inner0, outer0, outer1);
    indices.push(inner0, outer1, inner1);
  }

  geometry.setAttribute("position", new THREE.Float32BufferAttribute(vertices, 3));
  geometry.setAttribute("uv", new THREE.Float32BufferAttribute(uvs, 2));
  geometry.setIndex(indices);
  geometry.computeVertexNormals();

  return geometry;
}

interface TileData3D extends HexTile2D {
  spherePosition: THREE.Vector3;
  normal: THREE.Vector3;
}

function ClampedBillboard({
  children,
  damping = 0.175,
  sphereCenter,
  ...groupProps
}: {
  children: React.ReactNode;
  damping?: number;
  sphereCenter?: THREE.Vector3;
} & React.JSX.IntrinsicElements["group"]) {
  const ref = useRef<THREE.Group>(null);
  const billboardQuat = useMemo(() => new THREE.Quaternion(), []);
  const flatQuat = useMemo(() => new THREE.Quaternion(), []);
  const depthFixedRef = useRef(false);
  useFrame(({ camera }) => {
    const group = ref.current;
    if (!group) return;

    if (!depthFixedRef.current) {
      group.traverse((child) => {
        if (child instanceof THREE.Mesh && child.material) {
          const materials = Array.isArray(child.material) ? child.material : [child.material];
          for (const mat of materials) {
            mat.depthTest = false;
            mat.depthWrite = false;
            mat.transparent = true;
          }
        }
      });
      depthFixedRef.current = true;
    }

    group.getWorldPosition(_bbWorldPos);
    if (sphereCenter) {
      _bbNormal.copy(_bbWorldPos).sub(sphereCenter).normalize();
    } else {
      _bbNormal.copy(_bbWorldPos).normalize();
    }
    _bbToCamera.copy(camera.position).sub(_bbWorldPos).normalize();
    const dot = _bbNormal.dot(_bbToCamera);

    const opacity = THREE.MathUtils.smoothstep(dot, -0.15, 0.15);
    group.visible = opacity > 0.001;
    if (!group.visible) return;

    _bbOpacity = opacity;
    group.traverse(applyBillboardOpacity);

    group.lookAt(camera.position);
    billboardQuat.copy(group.quaternion);

    const angle = flatQuat.angleTo(billboardQuat);
    if (angle > 0.001) {
      const dampedAngle = damping * Math.atan(angle / damping);
      const t = dampedAngle / angle;
      group.quaternion.copy(flatQuat).slerp(billboardQuat, t);
    }
  });

  return (
    <group ref={ref} {...groupProps}>
      {children}
    </group>
  );
}

interface TileProps {
  nuclearSite?: NuclearSite;
  nuclearTransition?: NuclearTransition;
  outgoingType?: TileProps["tileType"];
  outgoingOwnerColor?: string;
  tileData: TileData3D;
  tileType:
    | "empty"
    | "ocean"
    | "greenery"
    | "city"
    | "special"
    | "volcano"
    | "nuclear-zone"
    | "industrial-center"
    | "mining"
    | "restricted"
    | "ecological-zone"
    | "natural-preserve"
    | "world-tree"
    | "mohole";
  ownerId?: string | null;
  ownerColor?: string;
  reservedById?: string | null;
  reservedByColor?: string;
  displayName?: string;
  isOceanSpace?: boolean;
  bonuses?: { [key: string]: number };
  onClick: () => void;
  isAvailableForPlacement?: boolean;
  isSelectedForPlacement?: boolean;
  hideLabels?: boolean;
  animateEntrance?: boolean;
  startHidden?: boolean;
  entranceDelay?: number;
  isNewlyPlaced?: boolean;
  visualSeed?: number;
  isVolcanic?: boolean;
  isHovered?: boolean;
  isInspected?: boolean;
  sphereRadius?: number;
  sphereCenter?: THREE.Vector3;
  groupInverseMatrix?: THREE.Matrix4;
  tileOpacity?: RefObject<number>;
  hasStructure?: boolean;
  vpHighlightIntensity?: number;
  vpHighlightColor?: [number, number, number];
}

function Tile({
  nuclearSite,
  tileData,
  tileType,
  ownerId,
  ownerColor,
  reservedById,
  reservedByColor,
  displayName,
  isOceanSpace: _isOceanSpace = false,
  bonuses = tileData.bonuses,
  onClick: _onClick,
  isAvailableForPlacement = false,
  isSelectedForPlacement = false,
  hideLabels = false,
  animateEntrance = false,
  startHidden = false,
  entranceDelay = 0,
  isNewlyPlaced = false,
  visualSeed = 1,
  nuclearTransition,
  outgoingType,
  outgoingOwnerColor,
  isVolcanic = false,
  isHovered: isHoveredProp = false,
  isInspected = false,
  sphereRadius = SPHERE_RADIUS,
  sphereCenter = ORIGIN,
  groupInverseMatrix,
  tileOpacity,
  hasStructure = false,
  vpHighlightIntensity = 0,
  vpHighlightColor = [0.95, 0.95, 1.0],
}: TileProps) {
  const contentType = outgoingType ?? tileType;
  const shownName =
    hideLabels || (hasStructure && contentType !== "city") ? undefined : displayName;
  const contentNewlyPlaced = nuclearTransition?.outgoing ? false : isNewlyPlaced;
  const tileGroupRef = useRef<THREE.Group>(null);
  const meshRef = useRef<THREE.Mesh>(null);

  const extraMatsRef = useRef<THREE.Material[] | null>(null);
  const hovered = isHoveredProp;
  const highlight = useTileHighlight(
    tileData,
    hovered,
    isInspected,
    isAvailableForPlacement,
    isSelectedForPlacement,
    vpHighlightIntensity,
    vpHighlightColor,
  );

  const [entranceScale, setEntranceScale] = useState(animateEntrance || startHidden ? 0 : 1);
  const entranceStartRef = useRef<number | null>(null);
  const entranceDoneRef = useRef(!animateEntrance);

  const { noiseMid: borderNoiseTexture, getResourceIcon } = useTextures();

  useEffect(() => {
    if (animateEntrance && entranceDoneRef.current) {
      setEntranceScale(0);
      entranceStartRef.current = null;
      entranceDoneRef.current = false;
    }
  }, [animateEntrance]);

  const hexGeometry = useMemo(() => {
    const radius = 0.166;
    const geometry = createSubdividedHexagonGeometry(radius, 6);
    geometry.rotateZ(Math.PI / 2);
    return geometry;
  }, []);

  const borderGeometry = useMemo(() => {
    const geometry = createSubdividedHexRingGeometry(0.156, 0.166, 4);
    geometry.rotateZ(Math.PI / 2);
    return geometry;
  }, []);

  const overlayGeometry = useMemo(() => {
    const geometry = createSubdividedHexagonGeometry(0.166, 6);
    geometry.rotateZ(Math.PI / 2);
    return geometry;
  }, []);

  const volcanicTintMaterial = useMemo(() => {
    return new THREE.ShaderMaterial({
      vertexShader: sphereProjectionVertex,
      fragmentShader: `
        precision highp float;
        varying vec2 vUv;
        void main() {
          vec2 center = vUv - 0.5;
          float distFromCenter = length(center);
          float gradient = smoothstep(0.2, 0.45, distFromCenter);
          vec3 color = vec3(0.75, 0.12, 0.05);
          float alpha = gradient * 0.4;
          gl_FragColor = vec4(color, alpha);
        }
      `,
      uniforms: {
        uSphereRadius: { value: sphereRadius },
        uZOffset: { value: CHROME_Z_BASE + 0.004 },
        uSphereCenter: { value: sphereCenter },
      },
      transparent: true,
      depthWrite: false,
      side: THREE.DoubleSide,
      // Flat overlays: one pass. Two-pass transparent DoubleSide doubles draws and re-validates the
      // material every frame.
      forceSinglePass: true,
    });
  }, [sphereRadius, sphereCenter]);

  const oceanBorderMaterial = useMemo(() => {
    return new THREE.ShaderMaterial({
      vertexShader: sphereProjectionVertex,
      fragmentShader: oceanBorderFragment,
      uniforms: {
        time: { value: 0.0 },
        uSphereRadius: { value: sphereRadius },
        uZOffset: { value: CHROME_Z_BASE + 0.004 },
        uSphereCenter: { value: sphereCenter },
      },
      transparent: true,
      depthWrite: false,
      side: THREE.DoubleSide,
      // Flat overlays: one pass. Two-pass transparent DoubleSide doubles draws and re-validates the
      // material every frame.
      forceSinglePass: true,
    });
  }, [sphereRadius, sphereCenter]);

  useFrame((state) => {
    oceanBorderMaterial.uniforms.time.value = state.clock.elapsedTime;
    if (highlight.current) {
      highlight.current.visibility = entranceScale * (tileOpacity?.current ?? 1);
    }

    // Update colors for hover state (avoids recreating materials)
    if (hovered || isInspected) {
      borderMaterial.uniforms.uColor.value.copy(HOVER_BORDER_COLOR);
    } else {
      borderMaterial.uniforms.uColor.value.copy(baseBorderColor);
    }

    if (animateEntrance && !entranceDoneRef.current) {
      if (entranceStartRef.current === null) {
        entranceStartRef.current = state.clock.elapsedTime;
      }
      const elapsed = (state.clock.elapsedTime - entranceStartRef.current) * 1000;
      if (elapsed >= entranceDelay) {
        const animDuration = 400;
        const t = Math.min((elapsed - entranceDelay) / animDuration, 1);
        const eased = easeOutCubic(t);
        setEntranceScale(eased);
        if (t >= 1) {
          entranceDoneRef.current = true;
        }
      }
    }

    if (meshRef.current) {
      meshRef.current.visible = hexTileMaterial.opacity > GLOW_CUTOFF;
    }

    if (tileOpacity && tileGroupRef.current) {
      const o = tileOpacity.current;
      hexTileMaterial.opacity = baseHexOpacity * o;
      borderMaterial.uniforms.uOpacity.value = 0.9 * o;

      if (!extraMatsRef.current) {
        const knownMats = new Set<THREE.Material>([
          hexTileMaterial,
          borderMaterial,
          volcanicTintMaterial,
          oceanBorderMaterial,
        ]);
        const extras: THREE.Material[] = [];
        tileGroupRef.current.traverse((child) => {
          if (!(child instanceof THREE.Mesh)) return;
          const mats = Array.isArray(child.material) ? child.material : [child.material];
          for (const mat of mats) {
            if (knownMats.has(mat)) continue;
            if (!mat.transparent) {
              mat.transparent = true;
              mat.needsUpdate = true;
            }
            extras.push(mat);
          }
        });
        extraMatsRef.current = extras;
      }

      for (const mat of extraMatsRef.current) {
        mat.opacity = o;
      }
    }
  });

  const surfaceQuaternion = useMemo(() => {
    const up = new THREE.Vector3(0, 0, 1);
    const quaternion = new THREE.Quaternion();
    quaternion.setFromUnitVectors(up, tileData.normal);
    return quaternion;
  }, [tileData.normal]);

  const adjustedPosition = useMemo(() => {
    return tileData.spherePosition.clone().add(tileData.normal.clone().multiplyScalar(0.01));
  }, [tileData.spherePosition, tileData.normal]);

  const baseTileColor = useMemo(() => {
    switch (tileType) {
      case "ocean":
        return new THREE.Color("#1e88e5");
      case "greenery":
        return new THREE.Color("#43a047");
      case "city":
        return new THREE.Color("#ff6f00");
      case "special":
        return new THREE.Color("#8e24aa");
      case "volcano":
        return new THREE.Color("#4a3728");
      case "nuclear-zone":
        return new THREE.Color("#2a1a0f");
      case "mining":
        return new THREE.Color("#5c3a1e");
      case "restricted":
        return new THREE.Color("#5a4030");
      case "ecological-zone":
      case "natural-preserve":
        return new THREE.Color("#2d6e2e");
      case "world-tree":
        return new THREE.Color("#1a3a12");
      case "mohole":
        return new THREE.Color("#3a2a1a");
      default:
        return new THREE.Color("#6d4c41").multiplyScalar(0.8);
    }
  }, [tileType]);

  const baseBorderColor = useMemo(() => {
    if (ownerColor) {
      return new THREE.Color(ownerColor);
    }
    if (tileType === "empty") {
      return new THREE.Color("#67432e");
    }
    return baseTileColor.clone().multiplyScalar(0.25);
  }, [baseTileColor, ownerColor, tileType]);

  const hexTileMaterial = useMemo(() => {
    const isGreenery = tileType === "greenery";
    const material = new THREE.MeshStandardMaterial({
      color: baseTileColor,
      transparent: true,
      opacity:
        hasStructure ||
        isGreenery ||
        tileType === "ocean" ||
        tileType === "city" ||
        tileType === "volcano" ||
        tileType === "nuclear-zone" ||
        tileType === "industrial-center" ||
        tileType === "mining" ||
        tileType === "restricted" ||
        tileType === "ecological-zone" ||
        tileType === "natural-preserve" ||
        tileType === "world-tree" ||
        tileType === "mohole"
          ? 0
          : tileType === "empty"
            ? EMPTY_HEX_OPACITY
            : 0.7,
      depthWrite: false,
      roughness: 0.7,
      metalness: 0.1,
      side: THREE.DoubleSide,
      // Flat overlays: one pass. Two-pass transparent DoubleSide doubles draws and re-validates the
      // material every frame.
      forceSinglePass: true,
    });

    const snippet = splitSnippet(tileSurfaceVertexSnippet);
    material.onBeforeCompile = (shader) => {
      shader.uniforms.uSphereRadius = { value: sphereRadius };
      shader.uniforms.uZOffset = { value: CHROME_Z_BASE + 0.002 };
      shader.uniforms.uSphereCenter = { value: sphereCenter };
      shader.vertexShader =
        snippet.header +
        "\n" +
        shader.vertexShader.replace("#include <begin_vertex>", snippet.body).replace(
          "#include <project_vertex>",
          `vec4 mvPosition = viewMatrix * vec4(projectedPos, 1.0);
             gl_Position = projectionMatrix * mvPosition;`,
        );
    };

    return material;
  }, [baseTileColor, tileType, hasStructure, sphereRadius, sphereCenter]);

  const baseHexOpacity = useMemo(() => {
    if (
      hasStructure ||
      tileType === "greenery" ||
      tileType === "city" ||
      tileType === "volcano" ||
      tileType === "nuclear-zone"
    ) {
      return 0;
    }
    if (tileType === "empty") {
      return EMPTY_HEX_OPACITY;
    }
    return 0.7;
  }, [tileType, hasStructure]);

  const borderMaterial = useMemo(() => {
    return new THREE.ShaderMaterial({
      vertexShader: tileBorderVertex,
      fragmentShader: tileBorderFragment,
      uniforms: {
        uSphereRadius: { value: sphereRadius },
        uZOffset: { value: CHROME_Z_BASE + 0.0025 },
        uColor: { value: new THREE.Color(baseBorderColor.r, baseBorderColor.g, baseBorderColor.b) },
        uOpacity: { value: 0.9 },
        uOpacityScale: { value: 1 },
        uNoiseTex: { value: borderNoiseTexture },
        uSphereCenter: { value: sphereCenter },
        uGroupInverseMatrix: { value: groupInverseMatrix || new THREE.Matrix4() },
      },
      transparent: true,
      depthWrite: false,
      side: THREE.FrontSide,
    });
  }, [baseBorderColor, borderNoiseTexture, sphereRadius, sphereCenter]);

  // Where trees or terrain stand in front of the border, it shows through dimmed, so the grid stays
  // readable without flattening the forest.
  const occludedBorderMaterial = useMemo(() => {
    return new THREE.ShaderMaterial({
      vertexShader: tileBorderVertex,
      fragmentShader: tileBorderFragment,
      uniforms: { ...borderMaterial.uniforms, uOpacityScale: { value: 0.4 } },
      transparent: true,
      depthWrite: false,
      depthFunc: THREE.GreaterDepth,
      side: THREE.FrontSide,
    });
  }, [borderMaterial]);

  interface BonusIconGroup {
    type: string;
    texture: THREE.Texture;
    count: number;
    isCredits: boolean;
  }

  const bonusIconGroups = useMemo((): BonusIconGroup[] => {
    const entries = Object.entries(bonuses);
    if (entries.length === 0) return [];

    return entries.map(([key, value]) => ({
      type: key,
      texture: getResourceIcon(key),
      count: value,
      isCredits: key === "credit",
    }));
  }, [bonuses, getResourceIcon]);

  const calculateIconPositions = (groups: BonusIconGroup[]) => {
    const ICON_GAP = 0.005;
    const GROUP_GAP = 0.01;
    const ICON_SIZE = 0.05;

    const positions: { x: number; group: BonusIconGroup; indexInGroup: number }[] = [];

    let totalWidth = 0;
    groups.forEach((group, groupIndex) => {
      if (groupIndex > 0) totalWidth += GROUP_GAP;
      const iconCount = group.isCredits ? 1 : group.count;
      totalWidth += iconCount * ICON_SIZE + Math.max(0, iconCount - 1) * ICON_GAP;
    });

    let currentX = -totalWidth / 2;
    groups.forEach((group, groupIndex) => {
      if (groupIndex > 0) currentX += GROUP_GAP;

      const iconCount = group.isCredits ? 1 : group.count;
      for (let i = 0; i < iconCount; i++) {
        if (i > 0) currentX += ICON_GAP;
        positions.push({ x: currentX + ICON_SIZE / 2, group, indexInGroup: i });
        currentX += ICON_SIZE;
      }
    });

    return positions;
  };

  return (
    <group
      ref={tileGroupRef}
      position={adjustedPosition}
      quaternion={surfaceQuaternion}
      scale={[entranceScale, entranceScale, entranceScale]}
    >
      {/* Main hex tile - hidden for ocean (water mesh handles rendering) */}
      {tileType !== "ocean" &&
        (tileType !== "nuclear-zone" ||
          (nuclearTransition?.outgoing && !nuclearTransition.impacted)) && (
          <mesh
            ref={meshRef}
            geometry={hexGeometry}
            material={hexTileMaterial}
            renderOrder={10}
            raycast={() => {}}
          />
        )}

      {/* Hex border - hidden for ocean tiles */}
      {tileType !== "ocean" && (
        <>
          <mesh geometry={borderGeometry} material={borderMaterial} renderOrder={20} />
          <mesh geometry={borderGeometry} material={occludedBorderMaterial} renderOrder={20} />
        </>
      )}

      {/* Ocean space border indicator (for empty ocean-reserved tiles) */}
      {tileType === "empty" && _isOceanSpace && (
        <mesh geometry={overlayGeometry} material={oceanBorderMaterial} renderOrder={21} />
      )}

      {/* Volcanic space indicator - red tint for unoccupied volcanic tiles */}
      {tileType === "empty" && isVolcanic && (
        <mesh geometry={overlayGeometry} material={volcanicTintMaterial} renderOrder={21} />
      )}

      {/* Nuclear Zone 3D tile */}
      {tileType === "nuclear-zone" && nuclearSite && (
        <NuclearZoneTile
          key={visualSeed}
          isNewlyPlaced={!!nuclearTransition || isNewlyPlaced}
          startTime={nuclearTransition?.start}
          site={nuclearSite}
          sphereCenter={sphereCenter}
        />
      )}

      <NuclearCollapse transition={nuclearTransition} seed={visualSeed}>
        {/* Volcano 3D tile */}
        {contentType === "volcano" && (
          <VolcanoTile
            seed={visualSeed}
            isNewlyPlaced={contentNewlyPlaced}
            surfaceNormal={tileData.normal}
            worldPosition={adjustedPosition}
            sphereCenter={sphereCenter}
          />
        )}

        {/* Mohole 3D tile */}
        {contentType === "mohole" && (
          <MoholeTile
            seed={visualSeed}
            isNewlyPlaced={contentNewlyPlaced}
            surfaceNormal={tileData.normal}
            worldPosition={adjustedPosition}
            sphereCenter={sphereCenter}
            groupInverseMatrix={groupInverseMatrix}
          />
        )}

        {/* Mining 3D tile */}
        {contentType === "mining" && (
          <MiningTile
            seed={visualSeed}
            isNewlyPlaced={contentNewlyPlaced}
            surfaceNormal={tileData.normal}
            worldPosition={adjustedPosition}
            sphereCenter={sphereCenter}
            groupInverseMatrix={groupInverseMatrix}
          />
        )}

        {/* Reserved Area fence tile */}
        {contentType === "restricted" && (
          <ReservedAreaTile
            isNewlyPlaced={contentNewlyPlaced}
            ownerColor={outgoingOwnerColor ?? ownerColor}
            surfaceNormal={tileData.normal}
            worldPosition={adjustedPosition}
            sphereCenter={sphereCenter}
            groupInverseMatrix={groupInverseMatrix}
          />
        )}

        {/* World Tree 3D tile */}
        {contentType === "world-tree" && (
          <WorldTreeTile
            seed={visualSeed}
            isNewlyPlaced={contentNewlyPlaced}
            surfaceNormal={tileData.normal}
            worldPosition={adjustedPosition}
            sphereCenter={sphereCenter}
            groupInverseMatrix={groupInverseMatrix}
          />
        )}
      </NuclearCollapse>
      {/* Special tile label (rendered via displayName below) */}

      {/* Billboard display name and/or bonus icons */}
      {!nuclearTransition?.impacted &&
        (shownName ||
          (tileType !== "greenery" &&
            tileType !== "ecological-zone" &&
            tileType !== "natural-preserve" &&
            tileType !== "world-tree" &&
            bonusIconGroups.length > 0)) && (
          <ClampedBillboard position={[0, 0, 0.02]} renderOrder={110} sphereCenter={sphereCenter}>
            {shownName && (
              <Text
                fontSize={0.045}
                font={assetUrl("fonts/prototype")}
                color="white"
                outlineWidth={0.004}
                outlineColor="black"
                anchorX="center"
                anchorY="middle"
                textAlign="center"
                maxWidth={0.18}
                renderOrder={110}
              >
                {shownName}
              </Text>
            )}
            {tileType !== "greenery" &&
              tileType !== "ecological-zone" &&
              tileType !== "natural-preserve" &&
              tileType !== "world-tree" &&
              bonusIconGroups.length > 0 && (
                <group position={[0, shownName ? -0.08 : 0, 0]}>
                  {calculateIconPositions(bonusIconGroups).map((pos) => (
                    <BonusIcon
                      key={`${pos.group.type}-${pos.indexInGroup}`}
                      texture={pos.group.texture}
                      position={[pos.x, 0, 0]}
                      isCredits={pos.group.isCredits}
                      creditAmount={
                        pos.group.isCredits && !hideLabels ? pos.group.count : undefined
                      }
                    />
                  ))}
                </group>
              )}
          </ClampedBillboard>
        )}

      {/* Reserved tile marker (land claim) */}
      {/* Reserved tile marker (fallback for non-fence reserved tiles) */}
      {reservedById && !ownerId && tileType !== "restricted" && (
        <group position={[0, 0, 0.01]}>
          <mesh position={[0.08, 0.05, 0]}>
            <boxGeometry args={[0.004, 0.06, 0.004]} />
            <meshBasicMaterial color="#333333" />
          </mesh>
          <mesh position={[0.1, 0.07, 0]}>
            <circleGeometry args={[0.025, 3]} />
            <meshBasicMaterial color={reservedByColor ?? "#67432e"} />
          </mesh>
        </group>
      )}
    </group>
  );
}

const MemoizedTile = memo(Tile);
export default MemoizedTile;

interface BonusIconProps {
  texture: THREE.Texture;
  position: [number, number, number];
  isCredits?: boolean;
  creditAmount?: number;
}

function BonusIcon({ texture, position, isCredits, creditAmount }: BonusIconProps) {
  const dimensions = useMemo((): [number, number] => {
    const image = texture.image;
    if (!(image instanceof HTMLImageElement)) {
      return [0.05, 0.05];
    }

    const aspect = image.width / image.height;
    const maxSize = 0.05;

    if (aspect > 1) {
      return [maxSize, maxSize / aspect];
    } else {
      return [maxSize * aspect, maxSize];
    }
  }, [texture]);

  return (
    <group position={position}>
      <mesh renderOrder={110}>
        <planeGeometry args={dimensions} />
        <meshBasicMaterial
          transparent
          alphaTest={0.1}
          map={texture}
          toneMapped={false}
          color={BONUS_ICON_TINT}
          depthTest={false}
          depthWrite={false}
        />
      </mesh>

      {isCredits && creditAmount !== undefined && (
        <Text
          position={[0, 0, 0.002]}
          fontSize={0.025}
          font={assetUrl("fonts/prototype")}
          color="black"
          anchorX="center"
          anchorY="middle"
          renderOrder={111}
        >
          {creditAmount}
        </Text>
      )}
    </group>
  );
}
