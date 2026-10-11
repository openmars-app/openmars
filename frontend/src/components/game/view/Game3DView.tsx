import { Suspense, useEffect, useMemo, useState, useRef, useCallback, type RefObject } from "react";
import CanvasClock from "../../3d/CanvasClock.tsx";
import {
  isRenderPaused,
  usePauseWhileDocumentHidden,
  useRenderPause,
  useRenderPauseStore,
} from "@/stores/renderPauseStore.ts";
import WebGLContextLossHandler from "../../3d/WebGLContextLossHandler.tsx";
import { invalidateGroundBakesAfterContextRestore } from "../board/groundBake.ts";
import { invalidatePlantShadesAfterContextRestore } from "../board/plantShade.ts";
import { invalidateGpuTimersAfterContextRestore } from "../board/gpuTimer.ts";
import { restoreTextureArraysAfterContextRestore } from "../board/textureArray.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { Canvas, useFrame, useThree, type RootState } from "@react-three/fiber";
import * as THREE from "three";
import { PanControls } from "../controls/PanControls.tsx";
import { FreeCamera, CameraFrustumHelper } from "../controls/FreeCamera.tsx";
import { CameraFraming, DESKTOP_FOV, useCameraFraming } from "../controls/CameraFraming.tsx";
import AtmosphereRenderer from "../board/AtmosphereRenderer.tsx";
import MarsSphere from "../board/MarsSphere.tsx";
import CelestialBody from "../board/CelestialBody.tsx";
import PhobosBody from "../board/PhobosBody.tsx";
import OrbitalStation from "../board/OrbitalStation.tsx";
import AsteroidImpact from "../board/AsteroidImpact.tsx";

import SkyboxLoader from "./SkyboxLoader.tsx";
import GameIcon from "../../ui/display/GameIcon.tsx";
import { GameDto } from "@/types/generated/api-types.ts";
import { MarsRotationProvider } from "../../../contexts/MarsRotationContext.tsx";
import { usePlanetFocus } from "../../../contexts/PlanetFocusContext.tsx";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { getTileIconType } from "@/utils/boardOverlay.ts";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext.tsx";
import { usePlanetSurface } from "../../../hooks/useTextures.ts";
import {
  createSunSurfaceMaterial,
  createSunCoronaMaterial,
  createSunProminenceMaterial,
} from "../board/shaders";
import GpuWarmup from "../board/GpuWarmup.tsx";
import PerformanceProbe from "../board/PerformanceProbe.tsx";
import { QUICK_MODE } from "@/utils/quickMode.ts";
import { GRAPHICS } from "@/utils/graphicsQuality.ts";
import SolarSystemOverview from "../board/SolarSystemOverview.tsx";
import {
  PLANET_CONFIGS,
  PLANET_FILL_LIGHT,
  LOCATION_TO_PLANET,
  getMarsOrbitalPosition,
  getPlanetOrbit,
} from "../board/solarSystemConfig.ts";

const HIGH_TIER_SHADOWS = { type: THREE.PCFShadowMap };

function ContextLossRecovery() {
  const [contextLost, setContextLost] = useState(false);
  useRenderPause("context-lost", contextLost);
  const handleLost = useCallback(() => setContextLost(true), []);
  const handleRestored = useCallback((gl: THREE.WebGLRenderer) => {
    invalidateGroundBakesAfterContextRestore();
    invalidatePlantShadesAfterContextRestore();
    invalidateGpuTimersAfterContextRestore();
    restoreTextureArraysAfterContextRestore(gl);
    setContextLost(false);
  }, []);
  return <WebGLContextLossHandler onLost={handleLost} onRestored={handleRestored} />;
}

function FreeCameraFrustum() {
  const { size } = useThree();
  const { fov } = useCameraFraming();
  const { storedCameraState } = useWorld3DSettings();

  if (!storedCameraState) return null;

  return (
    <CameraFrustumHelper
      storedState={storedCameraState}
      fov={fov}
      aspect={size.width / size.height}
    />
  );
}

function CentralSunLight({
  startDark = false,
  lightRef,
}: {
  startDark?: boolean;
  lightRef: RefObject<THREE.PointLight | null>;
}) {
  const { settings } = useWorld3DSettings();
  const fillLightRef = useRef<THREE.HemisphereLight>(null);
  const keyLightRef = useRef<THREE.DirectionalLight>(null);
  const sunriseStartTime = useRef<number | null>(null);

  useFrame((state) => {
    if (!lightRef.current) {
      return;
    }

    let intensityMultiplier = 1;
    if (startDark) {
      intensityMultiplier = 0;
      sunriseStartTime.current = null;
    } else if (sunriseStartTime.current === null) {
      sunriseStartTime.current = state.clock.elapsedTime;
      intensityMultiplier = 0;
    } else {
      const elapsed = state.clock.elapsedTime - sunriseStartTime.current;
      const t = Math.min(elapsed / 2.5, 1);
      intensityMultiplier = 1 - (1 - t) * (1 - t);
    }

    lightRef.current.intensity = settings.sunIntensity * intensityMultiplier;
    lightRef.current.color.setRGB(settings.sunColor.r, settings.sunColor.g, settings.sunColor.b);
    if (fillLightRef.current) {
      fillLightRef.current.intensity =
        lightRef.current.intensity * PLANET_FILL_LIGHT.intensityRatio;
    }
    if (keyLightRef.current) {
      keyLightRef.current.intensity =
        lightRef.current.intensity * PLANET_FILL_LIGHT.keyIntensityRatio;
    }
  });

  return (
    <>
      <pointLight ref={lightRef} position={[0, 0, 0]} intensity={0} distance={0} decay={0} />
      <directionalLight
        ref={keyLightRef}
        color={PLANET_FILL_LIGHT.keyColor}
        intensity={0}
        position={[0, 1, 0]}
      />
      <hemisphereLight
        ref={fillLightRef}
        args={[PLANET_FILL_LIGHT.skyColor, PLANET_FILL_LIGHT.groundColor, 0]}
        position={[0, 1, 0]}
      />
    </>
  );
}

function SunMesh() {
  const sunTexture = usePlanetSurface("sun", true);
  const geometry = useMemo(() => new THREE.SphereGeometry(22, ...GRAPHICS.sphereSegments), []);
  const material = useMemo(() => createSunSurfaceMaterial(sunTexture), [sunTexture]);
  const corona = useMemo(() => createSunCoronaMaterial(), []);
  const prominence = useMemo(() => createSunProminenceMaterial(), []);
  const arcGeometry = useMemo(() => {
    const points = Array.from({ length: 13 }, (_, i) => {
      const t = i / 12;
      return new THREE.Vector3(
        (t - 0.5) * 5.6,
        Math.sin(t * Math.PI) * (2.8 + 0.6 * Math.sin(t * 5)),
        Math.sin(t * Math.PI * 2) * 0.65,
      );
    });
    return new THREE.TubeGeometry(new THREE.CatmullRomCurve3(points), 64, 0.12, 8, false);
  }, []);
  const arcs = useMemo(
    () => [
      { angle: 0.45, tilt: 0.15, scale: 1 },
      { angle: 2.1, tilt: -0.35, scale: 0.75 },
      { angle: 3.6, tilt: 0.4, scale: 1.15 },
      { angle: 5.2, tilt: -0.15, scale: 0.6 },
    ],
    [],
  );

  useEffect(
    () => () => {
      geometry.dispose();
      arcGeometry.dispose();
      corona.dispose();
      prominence.dispose();
    },
    [geometry, arcGeometry, corona, prominence],
  );
  useEffect(() => () => material.dispose(), [material]);
  useFrame(({ clock }) => {
    material.uniforms.uTime.value = clock.elapsedTime;
    corona.uniforms.uTime.value = clock.elapsedTime;
    prominence.uniforms.uTime.value = clock.elapsedTime;
  });

  return (
    <group>
      <mesh geometry={geometry} material={material} />
      <mesh geometry={geometry} material={corona} scale={1.5} />
      {arcs.map(({ angle, tilt, scale }) => (
        <group key={angle} rotation={[0, tilt, angle]}>
          <mesh
            position={[0, 21.82, 0]}
            scale={scale}
            geometry={arcGeometry}
            material={prominence}
          />
        </group>
      ))}
    </group>
  );
}

function DynamicFog() {
  const { camera } = useThree();
  const { activePlanet } = usePlanetFocus();
  const overview = activePlanet === "solar-system";

  // Removing fog changes shader cache keys, including materials with fog disabled.
  return (
    <fog
      attach="fog"
      args={["#0a0a0a"]}
      near={overview ? camera.far : 8}
      far={overview ? camera.far * 2 : 25}
    />
  );
}

function AutoNavigateForTileSelection({ gameState }: { gameState: GameDto }) {
  const { setActivePlanet } = usePlanetFocus();
  const pendingTileSelection = gameState.currentPlayer?.pendingTileSelection;

  useEffect(() => {
    if (!pendingTileSelection || !gameState.board?.tiles) {
      return;
    }

    const tileKeyToLocation = new Map<string, string>();
    for (const tile of gameState.board.tiles) {
      const key = `${tile.coordinates.q},${tile.coordinates.r},${tile.coordinates.s}`;
      tileKeyToLocation.set(key, tile.location);
    }

    const targetPlanets = new Set<string>();
    for (const hex of pendingTileSelection.availableHexes) {
      const location = tileKeyToLocation.get(hex);
      if (location) {
        const planet = LOCATION_TO_PLANET[location] || "mars";
        targetPlanets.add(planet);
      }
    }

    if (targetPlanets.size === 1) {
      const target = [...targetPlanets][0];
      setActivePlanet(target as Parameters<typeof setActivePlanet>[0]);
    }
    // Only navigate once when pendingTileSelection changes — don't re-trigger on manual navigation
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingTileSelection, gameState.board?.tiles]);

  return null;
}

function TravelFade() {
  const { activePlanet } = usePlanetFocus();
  const divRef = useRef<HTMLDivElement>(null);
  const prevPlanetRef = useRef(activePlanet);

  useEffect(() => {
    if (activePlanet === prevPlanetRef.current) {
      return;
    }
    prevPlanetRef.current = activePlanet;
    const el = divRef.current;
    if (!el) {
      return;
    }

    // Instantly go black (no transition) to hide the teleport
    el.style.transition = "none";
    el.style.opacity = "1";
    // Force reflow so the browser applies opacity:1 immediately
    void el.offsetHeight;
    // Then fade back to transparent
    el.style.transition = "opacity 400ms ease-in-out";
    el.style.opacity = "0";
  }, [activePlanet]);

  return (
    <div
      ref={divRef}
      style={{
        position: "absolute",
        inset: 0,
        backgroundColor: "black",
        opacity: 0,
        pointerEvents: "none",
        zIndex: Z_INDEX.UI_BASE,
      }}
    />
  );
}

interface Game3DViewProps {
  gameState: GameDto;
  onHexClick: (coordinate: string) => void;
  manualFrames?: boolean;
  onCreated?: (state: RootState) => void;
  animateHexEntrance?: boolean;
  startDark?: boolean;
  tilesHidden?: boolean;
  onSkyboxReady?: () => void;
  onGpuReady?: () => void;
  showUI?: boolean;
  uiAnimationClass?: string;
}

export default function Game3DView({
  gameState,
  onHexClick,
  manualFrames = false,
  onCreated,
  animateHexEntrance = false,
  startDark = false,
  tilesHidden = false,
  onSkyboxReady,
  onGpuReady,
  showUI = true,
  uiAnimationClass = "",
}: Game3DViewProps) {
  const { activePlanet } = usePlanetFocus();
  const [warmup, setWarmup] = useState({ gameId: gameState.id, general: false, cities: false });
  const notifiedGame = useRef<string | null>(null);
  const handleGeneralReady = useCallback(() => {
    setWarmup((previous) => ({
      gameId: gameState.id,
      general: true,
      cities: previous.gameId === gameState.id && previous.cities,
    }));
  }, [gameState.id]);
  const handleStructureReady = useCallback(() => {
    setWarmup((previous) => ({
      gameId: gameState.id,
      cities: true,
      general: previous.gameId === gameState.id && previous.general,
    }));
  }, [gameState.id]);
  useEffect(() => {
    // Quick mode skips the warmup and accepts first-use shader hitches.
    const warmedUp =
      QUICK_MODE || (warmup.gameId === gameState.id && warmup.general && warmup.cities);
    if (warmedUp && notifiedGame.current !== gameState.id) {
      notifiedGame.current = gameState.id;
      onGpuReady?.();
    }
  }, [warmup, gameState.id, onGpuReady]);
  const orbitalProject = gameState.projectFunding?.find((p) => p.id === "pf_orbital_station");
  const orbitalStationSeats = orbitalProject ? orbitalProject.seatOwners.length : 0;
  const containerRef = useRef<HTMLDivElement>(null);
  const sunLightRef = useRef<THREE.PointLight>(null);
  const initialCameraPos = useMemo((): [number, number, number] => {
    const mp = getMarsOrbitalPosition(0);
    const center = new THREE.Vector3(mp[0], mp[1], mp[2]);
    const radius = getPlanetOrbit("mars").defaultRadius;
    const offset = new THREE.Vector3().setFromSpherical(
      new THREE.Spherical(radius, Math.PI / 2, 0),
    );
    const toSun = center.clone().negate().normalize();
    const quat = new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 0, 1), toSun);
    offset.applyQuaternion(quat);
    return [center.x + offset.x, center.y + offset.y, center.z + offset.z];
  }, []);

  const renderPaused = useRenderPauseStore(isRenderPaused);
  usePauseWhileDocumentHidden("game-hidden");

  const pendingTileSelection = gameState.currentPlayer?.pendingTileSelection;
  const { isCompact } = useLayoutMode();

  return (
    <div
      ref={containerRef}
      style={{
        flex: 1,
        height: "100%",
        width: "100%",
        minHeight: 0,
        position: "relative",
      }}
    >
      {showUI && pendingTileSelection && !isCompact && (
        <div
          className={`absolute top-[66px] left-1/2 -translate-x-1/2 game-panel game-panel-clipped game-window px-6 py-3 ${uiAnimationClass}`}
          style={{ zIndex: Z_INDEX.TILE_PLACEMENT_PROMPT }}
        >
          <div className="flex items-center gap-2">
            <span className="font-orbitron text-lg text-white tracking-wider-2xl">Place</span>
            <GameIcon iconType={getTileIconType(pendingTileSelection.tileType)} size="medium" />
          </div>
        </div>
      )}

      <AutoNavigateForTileSelection gameState={gameState} />
      <TravelFade />

      <Canvas
        frameloop={manualFrames || renderPaused ? "never" : "always"}
        onCreated={onCreated}
        camera={{
          position: initialCameraPos,
          fov: DESKTOP_FOV,
          near: 0.1,
          far: 5000,
        }}
        style={{
          background: "#000000",
          width: "100%",
          height: "100%",
          position: "relative",
          zIndex: 0,
        }}
        resize={{ scroll: false, debounce: { scroll: 50, resize: 0 } }}
        gl={{
          stencil: true,
          antialias: GRAPHICS.antialias,
          powerPreference: "high-performance",
        }}
        dpr={typeof window !== "undefined" ? Math.min(window.devicePixelRatio, GRAPHICS.maxDpr) : 1}
        shadows={GRAPHICS.tier === "high" ? HIGH_TIER_SHADOWS : false}
      >
        <CanvasClock />
        <ContextLossRecovery />
        <MarsRotationProvider>
          <Suspense fallback={null}>
            <AtmosphereRenderer
              sunLight={sunLightRef}
              enabled={activePlanet !== "solar-system"}
              shakeEnabled={activePlanet === "mars"}
              reportGpuStats
            >
              <SkyboxLoader onReady={onSkyboxReady} />

              <ambientLight intensity={0.4} color="#2a2a2a" />
              <CentralSunLight startDark={startDark} lightRef={sunLightRef} />
              <group userData={{ perfGroup: "sun" }}>
                <SunMesh />
              </group>
              <DynamicFog />

              <MarsSphere
                gameState={gameState}
                onHexClick={onHexClick}
                animateHexEntrance={animateHexEntrance}
                startHidden={tilesHidden}
                onStructureReady={handleStructureReady}
              />

              <group userData={{ perfGroup: "other planets & moons" }}>
                <PhobosBody gameState={gameState} onHexClick={onHexClick} />
                {PLANET_CONFIGS.map((config) => (
                  <CelestialBody
                    key={config.id}
                    config={config}
                    gameState={gameState}
                    onHexClick={onHexClick}
                  />
                ))}
              </group>

              <group userData={{ perfGroup: "solar system overview" }}>
                <SolarSystemOverview />
              </group>

              {orbitalProject && (
                <OrbitalStation
                  filledSeats={orbitalStationSeats}
                  totalSeats={orbitalProject.seats.length}
                  isCompleted={orbitalProject.isCompleted}
                  name={orbitalProject.name}
                />
              )}

              <AsteroidImpact />

              {!QUICK_MODE && (
                <group userData={{ perfGroup: "gpu warmup" }}>
                  <GpuWarmup key={gameState.id} onReady={handleGeneralReady} />
                </group>
              )}
              <PerformanceProbe />

              <CameraFraming />
              <PanControls />
              <FreeCamera />
              <FreeCameraFrustum />
            </AtmosphereRenderer>
          </Suspense>
        </MarsRotationProvider>
      </Canvas>
    </div>
  );
}
