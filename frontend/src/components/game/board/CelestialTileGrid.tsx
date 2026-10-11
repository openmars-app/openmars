import { sphereRaycast } from "../../../utils/sphereRaycast";
import { useEffect, useCallback, useMemo, useRef, useState, type RefObject } from "react";
import { useCardDragStore } from "@/stores/cardDragStore.ts";
import * as THREE from "three";
import { HexGrid2D } from "../../../utils/hex-grid-2d";
import Tile from "./Tile";
import StructureRenderer from "./StructureRenderer";
import { generateStructureLayout, type StructurePlot } from "./structureLayout";
import { BOARD_SCALE, hashSeed } from "./landscapeGeometry";
import { GameDto, TileDto, TileBonusDto } from "../../../types/generated/api-types";
import { usePreviousTiles } from "../../../hooks/usePreviousTiles";
import TileTooltip from "../../ui/display/TileTooltip";
import type { TileTooltipData } from "../../ui/display/TileInfoContent";
import { toggleInspectedHex, useTileInspectStore } from "@/stores/tileInspectStore.ts";
import { usePlacementSelectionStore } from "@/stores/placementSelectionStore.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { Html } from "@react-three/drei";
import { usePlanetFocus } from "../../../contexts/PlanetFocusContext";
import { useVPCounting } from "../../../contexts/VPCountingContext";
import { TileHighlightBoardContext, useTileHighlightBoard } from "./TileHighlightContext";
import { isDragClick } from "../controls/PanControls";
import { useAutoFramePlacement } from "../controls/useAutoFramePlacement";

interface CelestialTileGridProps {
  highlightRoot: RefObject<THREE.Group | null>;
  gameState?: GameDto;
  onHexClick?: (hexCoordinate: string) => void;
  tileOpacity?: RefObject<number>;
  location: string;
  radius: number;
  contentRadius?: number;
  coordOffset: { q: number; r: number; s: number };
  worldCenter: THREE.Vector3;
  activePlanetId: string;
  groupInverseMatrix?: THREE.Matrix4;
}

interface ProjectedTile {
  backendTile: TileDto;
  coordinate: { q: number; r: number; s: number };
  position: { x: number; y: number };
  spherePosition: THREE.Vector3;
  normal: THREE.Vector3;
  isOceanSpace: boolean;
  bonuses: { [key: string]: number };
}

type TileType = "city" | "empty" | "special";

function hexToPixel(
  coord: { q: number; r: number; s: number },
  offset: { q: number; r: number; s: number },
) {
  const q = coord.q - offset.q;
  const r = coord.r - offset.r;
  const size = 0.3;
  const x = size * Math.sqrt(3) * (q + r / 2);
  const y = ((size * 3) / 2) * r;
  return { x, y };
}

function projectToSphere(position2D: { x: number; y: number }, radius: number): THREE.Vector3 {
  const scale = 0.4;
  const x = position2D.x * scale;
  const y = position2D.y * scale;

  const r = Math.sqrt(x * x + y * y);

  if (r === 0) {
    return new THREE.Vector3(0, 0, radius);
  }

  const theta = Math.atan2(y, x);
  const phi = (r / radius) * (Math.PI / 2);

  const sphereX = radius * Math.sin(phi) * Math.cos(theta);
  const sphereY = radius * Math.sin(phi) * Math.sin(theta);
  const sphereZ = radius * Math.cos(phi);

  return new THREE.Vector3(sphereX, sphereY, sphereZ);
}

const convertBonuses = (bonuses: TileBonusDto[] | undefined) => {
  const converted: { [key: string]: number } = {};
  if (bonuses) {
    bonuses.forEach((bonus) => {
      converted[bonus.type] = bonus.amount;
    });
  }
  return converted;
};

export default function CelestialTileGrid({
  highlightRoot,
  gameState,
  onHexClick,
  tileOpacity,
  location,
  radius,
  contentRadius = radius,
  coordOffset,
  worldCenter,
  activePlanetId,
  groupInverseMatrix,
}: CelestialTileGridProps) {
  const highlightBoard = useTileHighlightBoard(
    highlightRoot,
    radius,
    1,
    gameState?.id,
    coordOffset.q,
    coordOffset.r,
  );
  const { state: vpCounting } = useVPCounting();
  const { activePlanet } = usePlanetFocus();

  const filteredTiles = useMemo(
    () => gameState?.board?.tiles?.filter((t) => t.location === location),
    [gameState?.board?.tiles, location],
  );
  const { newlyPlaced: newlyPlacedTiles } = usePreviousTiles(filteredTiles, gameState?.id);

  const playerColorMap = useMemo(() => {
    const map = new Map<string, string>();
    if (gameState) {
      if (gameState.currentPlayer?.color) {
        map.set(gameState.currentPlayer.id, gameState.currentPlayer.color);
      }
      gameState.otherPlayers?.forEach((p) => {
        if (p.color) {
          map.set(p.id, p.color);
        }
      });
    }
    return map;
  }, [gameState]);

  const playerNameMap = useMemo(() => {
    const map = new Map<string, string>();
    if (gameState) {
      if (gameState.currentPlayer) {
        map.set(gameState.currentPlayer.id, gameState.currentPlayer.name);
      }
      gameState.otherPlayers?.forEach((p) => {
        map.set(p.id, p.name);
      });
    }
    return map;
  }, [gameState]);

  const [tooltipData, setTooltipData] = useState<TileTooltipData | null>(null);
  const tooltipPositionRef = useRef<{ x: number; y: number }>({ x: 0, y: 0 });
  const hoverTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleTileHoverInfo = useCallback(
    (info: TileTooltipData, position: { x: number; y: number }) => {
      tooltipPositionRef.current = position;
      if (hoverTimerRef.current) {
        clearTimeout(hoverTimerRef.current);
      }
      hoverTimerRef.current = setTimeout(() => {
        setTooltipData(info);
      }, 200);
    },
    [],
  );

  const handleTileHoverMove = useCallback((position: { x: number; y: number }) => {
    tooltipPositionRef.current = position;
  }, []);

  const handleTileHoverLeave = useCallback(() => {
    if (hoverTimerRef.current) {
      clearTimeout(hoverTimerRef.current);
      hoverTimerRef.current = null;
    }
    setTooltipData(null);
  }, []);

  const projectedTiles = useMemo((): ProjectedTile[] => {
    if (!gameState?.board?.tiles) {
      return [];
    }

    return gameState.board.tiles
      .filter((tile: TileDto) => tile.location === location)
      .map((tile: TileDto): ProjectedTile => {
        const position2D = hexToPixel(tile.coordinates, coordOffset);
        const spherePosition = projectToSphere(position2D, radius);

        return {
          backendTile: tile,
          coordinate: tile.coordinates,
          position: position2D,
          spherePosition,
          normal: spherePosition.clone().normalize(),
          isOceanSpace: false,
          bonuses: convertBonuses(tile.bonuses),
        };
      });
  }, [gameState?.board?.tiles, location, coordOffset, radius]);

  const structurePlots = useMemo((): StructurePlot[] => {
    return projectedTiles
      .filter(
        (tile) =>
          tile.backendTile.occupiedBy?.type === "city-tile" ||
          tile.backendTile.occupiedBy?.visual?.structure != null,
      )
      .map((tile) => {
        const visual = tile.backendTile.occupiedBy!.visual;
        const key = HexGrid2D.coordinateToKey(tile.coordinate);
        return {
          coordinate: tile.coordinate,
          worldPosition: projectToSphere(tile.position, contentRadius),
          normal: tile.normal,
          surface: {
            radius: contentRadius,
            center: { x: tile.position.x * BOARD_SCALE, y: tile.position.y * BOARD_SCALE },
          },
          layout: generateStructureLayout(
            visual?.seed ?? hashSeed(`${gameState?.id}:${key}`),
            { cover: "dome", landscaping: "sparse", heights: "low", ...visual?.structure },
            tile.backendTile.occupiedBy?.displayName ?? tile.backendTile.displayName ?? "Structure",
          ),
        };
      });
  }, [projectedTiles, contentRadius, gameState?.id]);

  const availableHexes = gameState?.currentPlayer?.pendingTileSelection?.availableHexes || [];

  const getTileType = (tile: ProjectedTile): TileType => {
    if (tile.backendTile.occupiedBy) {
      if (tile.backendTile.occupiedBy.type === "city-tile") {
        return "city";
      }
      return "special";
    }
    return "empty";
  };

  const describeHex = useCallback(
    (key: string): TileTooltipData | null => {
      const tile = projectedTiles.find((t) => HexGrid2D.coordinateToKey(t.coordinate) === key);
      if (!tile) {
        return null;
      }
      const ownerId = tile.backendTile.ownerId || null;
      const reservedById = tile.backendTile.reservedBy || null;
      return {
        tileType: getTileType(tile),
        displayName: tile.backendTile.displayName,
        placedName: tile.backendTile.occupiedBy?.displayName,
        ownerName: ownerId ? playerNameMap.get(ownerId) : undefined,
        ownerColor: ownerId ? playerColorMap.get(ownerId) : undefined,
        reservedByName: reservedById ? playerNameMap.get(reservedById) : undefined,
        isOceanSpace: false,
        isVolcanic: false,
        bonuses: tile.bonuses,
      };
    },
    [projectedTiles, playerNameMap, playerColorMap],
  );

  const { isCompact } = useLayoutMode();
  const inspectedHex = useTileInspectStore((s) => s.inspectedHex);
  const selectedPlacementHex = usePlacementSelectionStore((s) => s.selectedHex);
  const hasPendingPlacement = Boolean(gameState?.currentPlayer?.pendingTileSelection);

  useEffect(() => {
    const { inspectedHex: hex, position, inspect } = useTileInspectStore.getState();
    if (!hex || !position) {
      return;
    }
    const info = describeHex(hex);
    if (info) {
      inspect(hex, info, position);
    }
  }, [describeHex]);

  const interactionSphereGeometry = useMemo(
    () => new THREE.SphereGeometry(radius + 0.02, 32, 32),
    [radius],
  );

  const findNearestHex = useCallback(
    (hitPoint: THREE.Vector3): string | null => {
      let bestKey: string | null = null;
      let bestDist = Infinity;
      const HEX_HIT_RADIUS = 0.17;
      for (const tile of projectedTiles) {
        const dist = hitPoint.distanceTo(tile.spherePosition);
        if (dist < bestDist && dist < HEX_HIT_RADIUS) {
          bestDist = dist;
          bestKey = HexGrid2D.coordinateToKey(tile.coordinate);
        }
      }
      return bestKey;
    },
    [projectedTiles],
  );

  const [hoveredHexKey, setHoveredHexKey] = useState<string | null>(null);
  const hoveredHexKeyRef = useRef<string | null>(null);
  useEffect(() => {
    hoveredHexKeyRef.current = null;
    setHoveredHexKey(null);
    handleTileHoverLeave();
  }, [activePlanet, gameState?.id]);
  const isDraggingCard = useCardDragStore((s) => s.isDraggingCard);

  const availableSignature = availableHexes.join("|");
  const availablePoints = useMemo(() => {
    const available = new Set(availableSignature ? availableSignature.split("|") : []);
    return projectedTiles
      .filter((tile) => available.has(HexGrid2D.coordinateToKey(tile.coordinate)))
      .map((tile) => tile.spherePosition);
  }, [projectedTiles, availableSignature]);
  useAutoFramePlacement(availablePoints, highlightRoot, activePlanetId);

  useEffect(() => {
    if (isDraggingCard) {
      hoveredHexKeyRef.current = null;
      setHoveredHexKey(null);
      handleTileHoverLeave();
    }
  }, [isDraggingCard, handleTileHoverLeave]);

  const handleSpherePointerMove = useCallback(
    (
      event: THREE.Event & {
        point: THREE.Vector3;
        nativeEvent: PointerEvent;
        object: THREE.Object3D;
      },
    ) => {
      if (isCompact && event.nativeEvent.pointerType === "touch") {
        return;
      }
      if (useCardDragStore.getState().isDraggingCard) {
        return;
      }
      const localPoint = event.object.worldToLocal(event.point.clone());
      const key = findNearestHex(localPoint);
      if (key !== hoveredHexKeyRef.current) {
        hoveredHexKeyRef.current = key;
        setHoveredHexKey(key);
        if (key) {
          const info = describeHex(key);
          if (info) {
            handleTileHoverInfo(info, {
              x: event.nativeEvent.clientX,
              y: event.nativeEvent.clientY,
            });
          }
        } else {
          handleTileHoverLeave();
        }
      } else if (key) {
        handleTileHoverMove({ x: event.nativeEvent.clientX, y: event.nativeEvent.clientY });
      }
    },
    [
      isCompact,
      findNearestHex,
      describeHex,
      handleTileHoverInfo,
      handleTileHoverMove,
      handleTileHoverLeave,
    ],
  );

  const handleSpherePointerLeave = useCallback(() => {
    hoveredHexKeyRef.current = null;
    setHoveredHexKey(null);
    handleTileHoverLeave();
  }, [handleTileHoverLeave]);

  const handleSphereClick = useCallback(
    (
      event: THREE.Event & {
        point: THREE.Vector3;
        object: THREE.Object3D;
        stopPropagation: () => void;
        nativeEvent: MouseEvent;
      },
    ) => {
      event.stopPropagation();
      if (isDragClick() || useCardDragStore.getState().isDraggingCard) {
        return;
      }
      const localPoint = event.object.worldToLocal(event.point.clone());
      const key = findNearestHex(localPoint);
      if (isCompact && !hasPendingPlacement) {
        toggleInspectedHex(key, describeHex, {
          x: event.nativeEvent.clientX,
          y: event.nativeEvent.clientY,
        });
        return;
      }
      if (key) {
        onHexClick?.(key);
      }
    },
    [findNearestHex, onHexClick, isCompact, hasPendingPlacement, describeHex],
  );

  const handleSpherePointerMissed = useCallback(() => {
    if (isCompact && !isDragClick()) {
      useTileInspectStore.getState().clear();
    }
  }, [isCompact]);

  return (
    <TileHighlightBoardContext.Provider value={highlightBoard}>
      {activePlanet === activePlanetId && (
        <mesh
          geometry={interactionSphereGeometry}
          raycast={sphereRaycast}
          onPointerMove={handleSpherePointerMove}
          onPointerLeave={handleSpherePointerLeave}
          onClick={handleSphereClick}
          onPointerMissed={handleSpherePointerMissed}
          visible={false}
        />
      )}

      {tooltipData && (
        <Html>
          <TileTooltip data={tooltipData} positionRef={tooltipPositionRef} />
        </Html>
      )}
      {structurePlots.map((plot) => {
        const key = HexGrid2D.coordinateToKey(plot.coordinate);
        return (
          <StructureRenderer
            key={key}
            plot={plot}
            sphereCenter={worldCenter}
            groupInverseMatrix={groupInverseMatrix}
            isNewlyPlaced={newlyPlacedTiles.has(key)}
          />
        );
      })}
      {projectedTiles.map((tile) => {
        const hexKey = HexGrid2D.coordinateToKey(tile.coordinate);
        const tileType = getTileType(tile);
        const isAvailable = availableHexes.includes(hexKey);
        const ownerId = tile.backendTile.ownerId || null;

        return (
          <Tile
            key={hexKey}
            tileData={tile}
            tileType={tileType}
            ownerId={ownerId}
            ownerColor={ownerId ? playerColorMap.get(ownerId) : undefined}
            reservedById={tile.backendTile.reservedBy || null}
            displayName={tile.backendTile.displayName}
            isVolcanic={false}
            onClick={() => {
              onHexClick?.(hexKey);
            }}
            isAvailableForPlacement={isAvailable}
            hideLabels={isCompact}
            isSelectedForPlacement={isAvailable && selectedPlacementHex === hexKey}
            isNewlyPlaced={newlyPlacedTiles.has(hexKey)}
            sphereRadius={radius}
            sphereCenter={worldCenter}
            groupInverseMatrix={groupInverseMatrix}
            tileOpacity={tileOpacity}
            isHovered={!isDraggingCard && hoveredHexKey === hexKey}
            isInspected={isCompact && inspectedHex === hexKey}
            vpHighlightIntensity={
              vpCounting.highlightedTiles.has(hexKey)
                ? 0.5
                : Number(vpCounting.secondaryHighlightedTiles.has(hexKey)) * 0.25
            }
            vpHighlightColor={
              vpCounting.secondaryHighlightedTiles.has(hexKey) ? [0.4, 0.9, 0.4] : [0.95, 0.95, 1]
            }
          />
        );
      })}
    </TileHighlightBoardContext.Provider>
  );
}
