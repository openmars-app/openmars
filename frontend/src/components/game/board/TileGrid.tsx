import type { MarsRelief } from "./marsRelief";
import NuclearDebrisRenderer from "./NuclearDebrisRenderer";
import { createNuclearSites } from "./nuclearDebris";
import { useLandscape } from "./useLandscape";
import { PlantShadeMap } from "./plantShade";
import { sphereRaycast } from "../../../utils/sphereRaycast";
import { useCallback, useEffect, useMemo, useRef, useState, type RefObject } from "react";
import { TileHighlightBoardContext, useTileHighlightBoard } from "./TileHighlightContext";
import * as THREE from "three";
import { HexGrid2D } from "../../../utils/hex-grid-2d";
import { useCardDragStore } from "../../../stores/cardDragStore";
import { useDebugHexHighlightStore } from "../../../stores/debugHexHighlightStore";
import Tile from "./Tile";
import { useNuclearTransitions } from "./useNuclearTransitions";
import { NuclearCollapse } from "./NuclearCollapse";
import { GameDto, TileDto, TileBonusDto } from "../../../types/generated/api-types";
import { usePreviousTiles } from "../../../hooks/usePreviousTiles";
import { useSoundEffects } from "../../../hooks/useSoundEffects";
import { useHoverSound } from "../../../hooks/useHoverSound";
import GreeneryRenderer from "./GreeneryRenderer";
import StructureRenderer from "./StructureRenderer";
import StructureBatchRenderer from "./StructureBatchRenderer";
import { generateStructureLayout, type StructurePlot } from "./structureLayout";
import { EMPTY_LANDSCAPE, type LandscapeSource, type LandscapeState } from "./landscapeTypes";
import { hashSeed } from "./landscapeGeometry";
import LandscapeRenderer from "./LandscapeRenderer";
import PrimitiveRenderer from "./PrimitiveManager";
import BirdRenderer from "./BirdRenderer";
import { GRAPHICS } from "@/utils/graphicsQuality.ts";
import SpaceshipRenderer from "./SpaceshipRenderer";
import { SPHERE_RADIUS } from "./boardConstants";
import TileTooltip from "../../ui/display/TileTooltip";
import type { TileTooltipData } from "../../ui/display/TileInfoContent";
import { toggleInspectedHex, useTileInspectStore } from "../../../stores/tileInspectStore";
import { usePlacementSelectionStore } from "../../../stores/placementSelectionStore";
import { useLayoutMode } from "../../../hooks/useLayoutMode";
import { Html } from "@react-three/drei";
import { isDragClick, panState } from "../controls/PanControls";
import { useAutoFramePlacement } from "../controls/useAutoFramePlacement";
import { useVPCounting } from "../../../contexts/VPCountingContext";
import { usePlanetFocus } from "../../../contexts/PlanetFocusContext";
import { coldStartTrace } from "../../../services/performanceStore";

const noop = () => {};
const EMPTY_BONUSES = {};
const VP_COLOR_SECONDARY: [number, number, number] = [0.4, 0.9, 0.4];
const VP_COLOR_PRIMARY: [number, number, number] = [0.95, 0.95, 1.0];
const DEBUG_HIGHLIGHT_COLOR: [number, number, number] = [0.72, 0.42, 1.0];
const DEBUG_HIGHLIGHT_INTENSITY = 0.5;

interface TileGridProps {
  relief: MarsRelief;
  highlightRoot: RefObject<THREE.Group | null>;
  gameState?: GameDto;
  onHexClick?: (hexCoordinate: string) => void;
  animateHexEntrance?: boolean;
  startHidden?: boolean;
  sphereCenter?: THREE.Vector3;
  groupInverseMatrix?: THREE.Matrix4;
  onStructureReady?: () => void;
}

// Local type for tiles with projected positions
interface ProjectedTile {
  backendTile?: TileDto;
  coordinate: { q: number; r: number; s: number };
  position: { x: number; y: number };
  spherePosition: THREE.Vector3;
  normal: THREE.Vector3;
  isOceanSpace: boolean;
  bonuses: { [key: string]: number };
}

// Type for the tile data returned by getTileData
type TileType =
  | "city"
  | "empty"
  | "ocean"
  | "greenery"
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

// Labels for special tile types (keyed by occupant type from backend)
const SPECIAL_TILE_LABELS: Record<string, string> = {
  "commercial-district-tile": "Commercial District",
  "industrial-center-tile": "Industrial Center",
};

interface TileData {
  type: TileType;
  ownerId: string | null;
  specialLabel: string | null;
}

const ORIGIN = new THREE.Vector3(0, 0, 0);

export default function TileGrid({
  highlightRoot,
  relief,
  gameState,
  onHexClick,
  animateHexEntrance = false,
  startHidden = false,
  sphereCenter = ORIGIN,
  groupInverseMatrix,
  onStructureReady,
}: TileGridProps) {
  const highlightBoard = useTileHighlightBoard(highlightRoot, SPHERE_RADIUS, -1, gameState?.id);
  const { newlyPlaced: newlyPlacedTiles, replacedTiles } = usePreviousTiles(
    gameState?.board?.tiles,
    gameState?.id,
  );
  const nuclearTransitions = useNuclearTransitions(
    gameState?.board?.tiles,
    gameState?.id,
    newlyPlacedTiles,
    replacedTiles,
  );
  const { playWaterPlacementSound, playOxygenSound, playConstructionSound } = useSoundEffects();
  const { state: vpCountingState } = useVPCounting();
  const debugHighlightedHexes = useDebugHexHighlightStore((state) => state.hexes);
  const { activePlanet } = usePlanetFocus();

  // Play placement sounds for newly placed tiles
  useEffect(() => {
    if (newlyPlacedTiles.size === 0 || !gameState?.board?.tiles) return;

    for (const hexKey of newlyPlacedTiles) {
      const tile = gameState.board.tiles.find(
        (t) => HexGrid2D.coordinateToKey(t.coordinates) === hexKey,
      );
      if (!tile?.occupiedBy) continue;

      switch (tile.occupiedBy.type) {
        case "ocean-tile":
          coldStartTrace.begin("ocean-placement", { key: hexKey });
          void playWaterPlacementSound();
          break;
        case "greenery-tile":
        case "ecological-zone-tile":
        case "natural-preserve-tile":
          break;
        case "city-tile":
        case "volcano-tile":
        case "special-tile":
        case "commercial-district-tile":
        case "industrial-center-tile":
        case "mining-tile":
        case "mohole-tile":
        case "restricted-tile":
          void playConstructionSound();
          break;
      }
    }
  }, [
    newlyPlacedTiles,
    gameState?.board?.tiles,
    playWaterPlacementSound,
    playOxygenSound,
    playConstructionSound,
  ]);

  const playerColorMap = useMemo(() => {
    const map = new Map<string, string>();
    if (gameState) {
      if (gameState.currentPlayer?.color) {
        map.set(gameState.currentPlayer.id, gameState.currentPlayer.color);
      }
      gameState.otherPlayers?.forEach((p) => {
        if (p.color) map.set(p.id, p.color);
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

  const handleTileHoverMove = useCallback((position: { x: number; y: number }) => {
    tooltipPositionRef.current = position;
  }, []);

  const handleTileHoverLeave = useCallback(() => {
    setTooltipData(null);
  }, []);

  // Convert hex coordinates to 2D pixel position (same as backend logic).
  // y is negated so JSON row 0 (r = -radius, "north") maps to sphere +Y.
  const hexToPixel = (coord: { q: number; r: number; s: number }) => {
    const size = 0.3;
    const x = size * Math.sqrt(3) * (coord.q + coord.r / 2);
    const y = -((size * 3) / 2) * coord.r;
    return { x, y };
  };

  // Convert backend tile bonuses to legacy format
  const convertBackendBonuses = (bonuses: TileBonusDto[] | undefined) => {
    const converted: { [key: string]: number } = {};
    if (bonuses) {
      bonuses.forEach((bonus) => {
        converted[bonus.type] = bonus.amount;
      });
    }
    return converted;
  };

  const tileSignature = JSON.stringify(gameState?.board?.tiles);
  const boardTiles = useMemo(() => gameState?.board?.tiles, [tileSignature]);
  const stablePlots = useRef(new Map<string, StructurePlot>());
  const stableStructureList = useRef<StructurePlot[]>([]);
  // Use backend board tiles or fallback to hardcoded generation
  const plantShade = useMemo(() => new PlantShadeMap(), []);
  useEffect(() => () => plantShade.dispose(), [plantShade]);
  const projectedHexGrid = useMemo((): ProjectedTile[] => {
    // Use backend tiles if available (filter to Mars-only)
    if (boardTiles) {
      return boardTiles
        .filter((tile: TileDto) => tile.location === "mars" && tile.type !== "empty")
        .map((tile: TileDto): ProjectedTile => {
          // Convert hex coordinate to 2D position for projection
          const position2D = hexToPixel(tile.coordinates);
          const spherePosition = projectToSphere(position2D, SPHERE_RADIUS);

          return {
            backendTile: tile,
            coordinate: tile.coordinates,
            position: position2D,
            spherePosition,
            normal: spherePosition.clone().normalize(),
            // Convert backend tile data to legacy interface for compatibility
            isOceanSpace: tile.type === "ocean-space",
            bonuses: convertBackendBonuses(tile.bonuses),
          };
        });
    }

    // Fallback to hardcoded generation if backend data not available
    const hexGrid = HexGrid2D.generateGrid();
    return hexGrid.map((tile) => {
      const spherePosition = projectToSphere(tile.position, SPHERE_RADIUS);
      return {
        ...tile,
        spherePosition,
        normal: spherePosition.clone().normalize(),
      };
    });
  }, [boardTiles]);

  const nuclearTiles = useMemo(
    () =>
      new Set(
        projectedHexGrid
          .filter((tile) => tile.backendTile?.occupiedBy?.type === "nuclear-zone-tile")
          .map((tile) => HexGrid2D.coordinateToKey(tile.coordinate)),
      ),
    [projectedHexGrid],
  );
  const visualHexGrid = useMemo(
    () =>
      projectedHexGrid.map((tile) => {
        const transition = nuclearTransitions.get(HexGrid2D.coordinateToKey(tile.coordinate));
        return transition?.outgoing && !transition.collapsed
          ? { ...tile, backendTile: transition.outgoing }
          : tile;
      }),
    [projectedHexGrid, nuclearTransitions],
  );
  const landscapeHexGrid = useMemo(
    () =>
      projectedHexGrid.map((tile) => {
        const transition = nuclearTransitions.get(HexGrid2D.coordinateToKey(tile.coordinate));
        return transition?.outgoing && !transition.impacted
          ? { ...tile, backendTile: transition.outgoing }
          : tile;
      }),
    [projectedHexGrid, nuclearTransitions],
  );

  const structurePlots = useMemo(() => {
    let tracedPlacement = false;
    const plots = visualHexGrid
      .filter(
        (tile) =>
          tile.backendTile?.occupiedBy?.type === "city-tile" ||
          tile.backendTile?.occupiedBy?.visual?.structure != null,
      )
      .map((tile) => {
        const key = HexGrid2D.coordinateToKey(tile.coordinate);
        if (!tracedPlacement && newlyPlacedTiles.has(key) && !stablePlots.current.has(key)) {
          coldStartTrace.begin("structure-placement", { key });
          tracedPlacement = true;
        }
        const endLayout = coldStartTrace.span("structure:layout", { key });
        const layout = generateStructureLayout(
          tile.backendTile!.occupiedBy!.visual?.seed ?? hashSeed(`${gameState?.id}:${key}`),
          tile.backendTile!.occupiedBy!.visual?.structure,
          tile.backendTile?.occupiedBy?.displayName ?? tile.backendTile?.displayName ?? "Structure",
        );
        endLayout();
        const cached = stablePlots.current.get(key);
        if (cached?.layout === layout) {
          return cached;
        }
        const plot = {
          coordinate: tile.coordinate,
          worldPosition: tile.spherePosition,
          normal: tile.normal,
          layout,
        };
        stablePlots.current.set(key, plot);
        return plot;
      });
    const active = new Set(plots.map((p) => HexGrid2D.coordinateToKey(p.coordinate)));
    for (const key of stablePlots.current.keys()) {
      if (!active.has(key)) {
        stablePlots.current.delete(key);
      }
    }
    if (
      plots.length === stableStructureList.current.length &&
      plots.every((p, i) => p === stableStructureList.current[i])
    ) {
      return stableStructureList.current;
    }
    stableStructureList.current = plots;
    return plots;
  }, [visualHexGrid, gameState?.id]);
  const structureKeys = useMemo(
    () => new Set(structurePlots.map((plot) => HexGrid2D.coordinateToKey(plot.coordinate))),
    [structurePlots],
  );

  // Get tile type and occupancy data
  const getTileData = (tile: ProjectedTile): TileData => {
    if (tile.backendTile) {
      const backendTile: TileDto = tile.backendTile;

      // Determine tile type based on occupancy
      if (backendTile.occupiedBy) {
        switch (backendTile.occupiedBy.type) {
          case "ocean-tile":
            return {
              type: "ocean",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "city-tile":
            return {
              type: "city",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "greenery-tile":
            return {
              type: "greenery",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "volcano-tile":
            return {
              type: "volcano",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "nuclear-zone-tile":
            return {
              type: "nuclear-zone",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "ecological-zone-tile":
            return {
              type: "ecological-zone",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "natural-preserve-tile":
            return {
              type: "natural-preserve",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "industrial-center-tile":
            return {
              type: "industrial-center",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "mining-tile":
            return {
              type: "mining",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "restricted-tile":
            return {
              type: "restricted",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "world-tree-tile":
            return {
              type: "world-tree",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          case "mohole-tile":
            return {
              type: "mohole",
              ownerId: backendTile.ownerId || null,
              specialLabel: null,
            };
          default:
            return {
              type: "special",
              ownerId: backendTile.ownerId || null,
              specialLabel:
                SPECIAL_TILE_LABELS[backendTile.occupiedBy.type] || backendTile.occupiedBy.type,
            };
        }
      }

      // Reserved (land claim) but not occupied - show as fence tile
      if (backendTile.reservedBy) {
        return {
          type: "restricted",
          ownerId: backendTile.reservedBy,
          specialLabel: null,
        };
      }

      // Empty tile
      return {
        type: "empty",
        ownerId: backendTile.ownerId || null,
        specialLabel: null,
      };
    }

    // Fallback for hardcoded tiles
    return { type: "empty", ownerId: null, specialLabel: null };
  };

  // Get available hexes from current player's pending tile selection
  const availableHexes = gameState?.currentPlayer?.pendingTileSelection?.availableHexes || [];

  // Collect all greenery tiles for the GreeneryRenderer (includes living greenery types)
  const greeneryTiles = useMemo(() => {
    return landscapeHexGrid
      .filter((tile) => {
        const tileData = getTileData(tile);
        return (
          tileData.type === "greenery" ||
          tileData.type === "ecological-zone" ||
          tileData.type === "natural-preserve"
        );
      })
      .map((tile) => ({
        coordinate: tile.coordinate,
        worldPosition: tile.spherePosition,
        normal: tile.normal,
      }));
  }, [landscapeHexGrid]);

  const renderedGreenery = greeneryTiles;
  const landscapeInputs = useMemo(() => {
    const tiles: LandscapeSource[] = structurePlots
      .filter(
        (plot) => !nuclearTransitions.get(HexGrid2D.coordinateToKey(plot.coordinate))?.impacted,
      )
      .map((plot) => ({
        coordinate: plot.coordinate,
        kind: "structure",
        seed: plot.layout.seed,
        layout: plot.layout,
      }));
    for (const t of renderedGreenery) {
      const original = landscapeHexGrid.find(
        (p) => HexGrid2D.coordinateToKey(p.coordinate) === HexGrid2D.coordinateToKey(t.coordinate),
      );
      const special = original?.backendTile?.occupiedBy?.type !== "greenery-tile";
      tiles.push({
        coordinate: t.coordinate,
        kind: special ? "special-greenery" : "greenery",
        seed:
          original?.backendTile?.occupiedBy?.visual?.seed ??
          hashSeed(`${gameState?.id}:${HexGrid2D.coordinateToKey(t.coordinate)}`),
      });
    }
    for (const tile of landscapeHexGrid) {
      const type = getTileData(tile).type;
      if (type === "ocean" || type === "volcano") {
        tiles.push({
          coordinate: tile.coordinate,
          kind: type,
          seed: hashSeed(`${gameState?.id}:${HexGrid2D.coordinateToKey(tile.coordinate)}`),
        });
      } else if (
        tile.backendTile?.occupiedBy &&
        !tiles.some(
          (s) =>
            HexGrid2D.coordinateToKey(s.coordinate) === HexGrid2D.coordinateToKey(tile.coordinate),
        )
      ) {
        tiles.push({ coordinate: tile.coordinate, kind: "excluded", seed: 0 });
      }
    }
    return { sources: tiles, seed: hashSeed(gameState?.id ?? "showcase"), relief };
  }, [
    relief,
    structurePlots,
    renderedGreenery,
    landscapeHexGrid,
    gameState?.id,
    nuclearTransitions,
  ]);
  const landscapeSignature = useMemo(
    () => JSON.stringify([landscapeInputs.seed, landscapeInputs.sources, relief.revision]),
    [landscapeInputs, relief],
  );
  const pendingLandscape = useLandscape(landscapeInputs, landscapeSignature);
  const [landscapeView, setLandscapeView] = useState({
    current: EMPTY_LANDSCAPE,
    previous: EMPTY_LANDSCAPE,
    transitionStart: 0,
  });
  const handleLandscapeReady = useCallback((state: LandscapeState, transitionStart: number) => {
    setLandscapeView((previous) => ({
      current: state,
      previous: previous.current,
      transitionStart,
    }));
  }, []);
  const nuclearBoard = projectedHexGrid.map((tile) => ({
    coordinate: tile.coordinate,
    kind: getTileData(tile).type,
  }));
  const nuclearTerrainSignature = JSON.stringify([
    gameState?.id,
    nuclearBoard,
    [...landscapeView.current.patches.values()].map((patch) => [patch.key, patch.signature]),
  ]);
  const nuclearSites = useMemo(
    () => createNuclearSites(nuclearBoard, gameState?.id ?? "showcase", landscapeView.current),
    [nuclearTerrainSignature],
  );
  // Collect living greenery tiles (ecological-zone, natural-preserve) for boosted vegetation
  const livingGreeneryTiles = useMemo(() => {
    return projectedHexGrid
      .filter((tile) => {
        const tileData = getTileData(tile);
        return tileData.type === "ecological-zone" || tileData.type === "natural-preserve";
      })
      .map((tile) => ({
        coordinate: tile.coordinate,
        worldPosition: tile.spherePosition,
        normal: tile.normal,
      }));
  }, [projectedHexGrid]);

  const describeHex = useCallback(
    (key: string): TileTooltipData | null => {
      const tile = projectedHexGrid.find((t) => HexGrid2D.coordinateToKey(t.coordinate) === key);
      if (!tile) {
        return null;
      }
      const tileData = getTileData(tile);
      const structurePlot = structurePlots.find(
        (plot) => HexGrid2D.coordinateToKey(plot.coordinate) === key,
      );
      const ownerId = tileData.ownerId;
      const reservedById = tile.backendTile?.reservedBy || null;
      return {
        tileType: tileData.type,
        displayName: tileData.specialLabel || tile.backendTile?.displayName,
        placedName: tile.backendTile?.occupiedBy?.displayName,
        ownerName: ownerId ? playerNameMap.get(ownerId) : undefined,
        ownerColor: ownerId ? playerColorMap.get(ownerId) : undefined,
        reservedByName: reservedById ? playerNameMap.get(reservedById) : undefined,
        isOceanSpace: tile.isOceanSpace,
        isVolcanic: tile.backendTile?.tags?.includes("volcanic") ?? false,
        bonuses: structurePlot ? {} : tile.bonuses,
      };
    },
    [projectedHexGrid, structurePlots, playerNameMap, playerColorMap],
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

  // --- Centralized interaction sphere (single raycast target) ---
  const [hoveredHexKey, setHoveredHexKey] = useState<string | null>(null);
  const hoveredHexKeyRef = useRef<string | null>(null);
  useEffect(() => {
    hoveredHexKeyRef.current = null;
    setHoveredHexKey(null);
    handleTileHoverLeave();
  }, [activePlanet, gameState?.id]);
  const hoverSound = useHoverSound();
  const interactionSphereGeometry = useMemo(
    () => new THREE.SphereGeometry(SPHERE_RADIUS + 0.02, 64, 64),
    [],
  );

  const findNearestHex = useCallback(
    (hitPoint: THREE.Vector3): string | null => {
      let bestKey: string | null = null;
      let bestDist = Infinity;
      const HEX_HIT_RADIUS = 0.17;
      for (const tile of projectedHexGrid) {
        const dist = hitPoint.distanceTo(tile.spherePosition);
        if (dist < bestDist && dist < HEX_HIT_RADIUS) {
          bestDist = dist;
          bestKey = HexGrid2D.coordinateToKey(tile.coordinate);
        }
      }
      return bestKey;
    },
    [projectedHexGrid],
  );

  const isDraggingCard = useCardDragStore((s) => s.isDraggingCard);

  const availableSignature = availableHexes.join("|");
  const availablePoints = useMemo(() => {
    const available = new Set(availableSignature ? availableSignature.split("|") : []);
    return projectedHexGrid
      .filter((tile) => available.has(HexGrid2D.coordinateToKey(tile.coordinate)))
      .map((tile) => tile.spherePosition);
  }, [projectedHexGrid, availableSignature]);
  useAutoFramePlacement(availablePoints, highlightRoot, "mars");

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
      if (panState.isPanning || useCardDragStore.getState().isDraggingCard) {
        if (hoveredHexKeyRef.current) {
          hoveredHexKeyRef.current = null;
          setHoveredHexKey(null);
          handleTileHoverLeave();
        }
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
            if (availableHexes.includes(key)) {
              hoverSound.onMouseEnter?.();
            }
            tooltipPositionRef.current = {
              x: event.nativeEvent.clientX,
              y: event.nativeEvent.clientY,
            };
            setTooltipData(info);
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
      availableHexes,
      findNearestHex,
      describeHex,
      handleTileHoverMove,
      handleTileHoverLeave,
      hoverSound,
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
        nativeEvent: MouseEvent;
      },
    ) => {
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
        const isAvailable = availableHexes.includes(key);
        if (isAvailable) {
          hoverSound.onClick?.();
        }
        onHexClick?.(key);
      }
    },
    [
      findNearestHex,
      availableHexes,
      onHexClick,
      hoverSound,
      isCompact,
      hasPendingPlacement,
      describeHex,
    ],
  );

  const handleSpherePointerMissed = useCallback(() => {
    if (isCompact && !isDragClick()) {
      useTileInspectStore.getState().clear();
    }
  }, [isCompact]);

  return (
    <TileHighlightBoardContext.Provider value={highlightBoard}>
      {/* Single invisible sphere for all pointer interaction */}
      {activePlanet === "mars" && (
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

      {/* Tile hover tooltip — Html bridges R3F→DOM, TileTooltip portals to body */}
      {tooltipData && (
        <Html>
          <TileTooltip data={tooltipData} positionRef={tooltipPositionRef} />
        </Html>
      )}
      <GreeneryRenderer
        shade={plantShade}
        landscape={landscapeView.current}
        previousLandscape={landscapeView.previous}
        nuclearTransitions={nuclearTransitions}
        nuclearTiles={nuclearTiles}
        transitionStart={landscapeView.transitionStart}
      />
      <StructureBatchRenderer
        key={gameState?.id}
        plots={structurePlots}
        newlyPlaced={newlyPlacedTiles}
        nuclearTransitions={nuclearTransitions}
        visible={!startHidden || animateHexEntrance}
        onReady={onStructureReady}
      />
      <group visible={!startHidden || animateHexEntrance}>
        <LandscapeRenderer
          state={pendingLandscape}
          onReady={handleLandscapeReady}
          groupInverseMatrix={groupInverseMatrix}
          plantShade={plantShade}
        />
        {structurePlots.map((plot) => (
          <NuclearCollapse
            key={HexGrid2D.coordinateToKey(plot.coordinate)}
            transition={nuclearTransitions.get(HexGrid2D.coordinateToKey(plot.coordinate))}
            seed={plot.layout.seed}
            origin={plot.worldPosition}
            normal={plot.normal}
          >
            <StructureRenderer
              renderBuildings={false}
              showLabel={false}
              isNewlyPlaced={
                !nuclearTransitions.has(HexGrid2D.coordinateToKey(plot.coordinate)) &&
                newlyPlacedTiles.has(HexGrid2D.coordinateToKey(plot.coordinate))
              }
              connectedGround
              plot={plot}
              sphereCenter={sphereCenter}
              groupInverseMatrix={groupInverseMatrix}
            />
          </NuclearCollapse>
        ))}
      </group>
      <NuclearDebrisRenderer sites={nuclearSites} transitions={nuclearTransitions} />
      <PrimitiveRenderer />
      {GRAPHICS.birds && <BirdRenderer livingGreeneryTiles={livingGreeneryTiles} />}
      {gameState?.settings?.systems.includes("colonies") && (
        <SpaceshipRenderer
          gameState={gameState}
          animateEntrance={animateHexEntrance}
          startHidden={startHidden}
          sphereCenter={sphereCenter}
          groupInverseMatrix={groupInverseMatrix}
        />
      )}
      {projectedHexGrid.map((tile, index) => {
        const hexKey = HexGrid2D.coordinateToKey(tile.coordinate);
        const transition = nuclearTransitions.get(hexKey);
        const isStructureRenderer = structureKeys.has(hexKey) && !transition;
        const tileData = getTileData(tile);

        const ownerId = tileData.ownerId;
        const isAvailable = availableHexes.includes(hexKey);

        const isHovered = !isDraggingCard && hoveredHexKey === hexKey;

        const isPrimaryHighlight = vpCountingState.highlightedTiles.has(hexKey);
        const isSecondaryHighlight = vpCountingState.secondaryHighlightedTiles.has(hexKey);
        const vpHighlightIntensity = isPrimaryHighlight ? 0.5 : isSecondaryHighlight ? 0.25 : 0;
        const vpHighlightColor = isSecondaryHighlight ? VP_COLOR_SECONDARY : VP_COLOR_PRIMARY;
        const isDebugHighlight = vpHighlightIntensity === 0 && debugHighlightedHexes.has(hexKey);

        return (
          <Tile
            key={hexKey}
            tileData={tile}
            tileType={tileData.type}
            hasStructure={isStructureRenderer}
            nuclearSite={nuclearSites.get(hexKey)}
            nuclearTransition={transition}
            outgoingType={
              transition?.outgoing
                ? getTileData({ ...tile, backendTile: transition.outgoing }).type
                : undefined
            }
            ownerId={ownerId}
            ownerColor={ownerId ? playerColorMap.get(ownerId) : undefined}
            outgoingOwnerColor={
              transition?.outgoing?.ownerId
                ? playerColorMap.get(transition.outgoing.ownerId)
                : undefined
            }
            reservedById={tile.backendTile?.reservedBy || null}
            reservedByColor={
              tile.backendTile?.reservedBy
                ? playerColorMap.get(tile.backendTile.reservedBy)
                : undefined
            }
            displayName={
              !transition?.impacted && transition?.outgoing
                ? transition.outgoing.displayName
                : tileData.specialLabel || tile.backendTile?.displayName
            }
            isVolcanic={tile.backendTile?.tags?.includes("volcanic") ?? false}
            isOceanSpace={tile.isOceanSpace}
            bonuses={isStructureRenderer ? EMPTY_BONUSES : tile.bonuses}
            onClick={noop}
            isHovered={isHovered}
            isInspected={isCompact && inspectedHex === hexKey}
            isAvailableForPlacement={isAvailable}
            hideLabels={isCompact}
            isSelectedForPlacement={isAvailable && selectedPlacementHex === hexKey}
            animateEntrance={animateHexEntrance}
            startHidden={startHidden}
            entranceDelay={index * 15}
            isNewlyPlaced={newlyPlacedTiles.has(hexKey)}
            visualSeed={hashSeed(`${gameState?.id}:${hexKey}`)}
            vpHighlightIntensity={
              isDebugHighlight ? DEBUG_HIGHLIGHT_INTENSITY : vpHighlightIntensity
            }
            vpHighlightColor={isDebugHighlight ? DEBUG_HIGHLIGHT_COLOR : vpHighlightColor}
            sphereCenter={sphereCenter}
            groupInverseMatrix={groupInverseMatrix}
          />
        );
      })}
    </TileHighlightBoardContext.Provider>
  );
}

/**
 * Project a 2D point onto the surface of a sphere
 * This simulates "wrapping" the flat hex grid around the sphere
 */
function projectToSphere(position2D: { x: number; y: number }, radius: number): THREE.Vector3 {
  // Scale the 2D coordinates to fit nicely on the sphere
  const scale = 0.4; // Reduced scale to bring hexagons closer together
  const x = position2D.x * scale;
  const y = position2D.y * scale;

  // Project onto sphere using azimuthal projection
  // This simulates "wrapping" the flat grid around the front hemisphere
  const r = Math.sqrt(x * x + y * y);

  if (r === 0) {
    // Center point
    return new THREE.Vector3(0, 0, radius);
  }

  // Convert to spherical coordinates
  const theta = Math.atan2(y, x); // Azimuth angle
  const phi = (r / radius) * (Math.PI / 2); // Polar angle (scaled to hemisphere)

  // Convert back to Cartesian coordinates on sphere
  const sphereX = radius * Math.sin(phi) * Math.cos(theta);
  const sphereY = radius * Math.sin(phi) * Math.sin(theta);
  const sphereZ = radius * Math.cos(phi);

  return new THREE.Vector3(sphereX, sphereY, sphereZ);
}
