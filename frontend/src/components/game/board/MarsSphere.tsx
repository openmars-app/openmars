import {
  createMarsReliefGeometry,
  createMarsReliefRaycast,
  marsReliefFromTexture,
  type MarsRelief,
} from "./marsRelief";
import { useMemo, useRef, useEffect } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import * as THREE from "three";
import TileGrid from "./TileGrid.tsx";
import { isDragClick } from "../controls/PanControls.tsx";
import PlanetAtmosphere from "./PlanetAtmosphere.tsx";
import { ClimateProvider, useClimate } from "../../../contexts/ClimateContext.tsx";
import { BARREN_PARAMETERS } from "./climate.ts";
import { addMarsClimate, createMarsClimateUniforms } from "./shaders/index.ts";
import { useReducedMotion } from "../../../hooks/useReducedMotion.ts";

import { GameDto } from "../../../types/generated/api-types.ts";
import { useMarsRotation } from "../../../contexts/MarsRotationContext.tsx";
import { useTextures } from "../../../hooks/useTextures.ts";
import { usePlanetFocus } from "../../../contexts/PlanetFocusContext.tsx";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext.tsx";
import {
  SPHERE_RADIUS,
  MOHOLE_STENCIL_BIT,
  LAKE_STENCIL_BIT,
  NUCLEAR_STENCIL_BIT,
} from "./boardConstants.ts";
import {
  getMarsOrbitalPosition,
  setOrbitSpeedMultiplier,
  MARS_ATMOSPHERE,
  MARS_CLIMATE_ATMOSPHERES,
} from "./solarSystemConfig.ts";

interface MarsSphereProps {
  gameState?: GameDto;
  onHexClick?: (hex: string) => void;
  animateHexEntrance?: boolean;
  startHidden?: boolean;
  onStructureReady?: () => void;
}

export default function MarsSphere({
  gameState,
  onHexClick,
  animateHexEntrance = false,
  startHidden = false,
  onStructureReady,
}: MarsSphereProps) {
  const { marsGroupRef } = useMarsRotation();
  const { mars } = useTextures();
  const relief = useMemo(() => marsReliefFromTexture(mars), [mars]);
  const { settings: world3DSettings } = useWorld3DSettings();
  const worldCenterRef = useRef(new THREE.Vector3());
  const groupInverseMatrixRef = useRef(new THREE.Matrix4());

  const globalParameters = gameState?.globalParameters;
  const parameters =
    world3DSettings.climateOverride ??
    (globalParameters
      ? {
          temperature: globalParameters.temperature,
          oxygen: globalParameters.oxygen,
          oceans: globalParameters.oceans,
          maxOceans: globalParameters.maxOceans,
        }
      : BARREN_PARAMETERS);

  useFrame((state) => {
    setOrbitSpeedMultiplier(world3DSettings.orbitSpeedMultiplier);
    if (marsGroupRef.current) {
      const pos = getMarsOrbitalPosition(state.clock.elapsedTime);
      marsGroupRef.current.position.set(pos[0], pos[1], pos[2]);
      marsGroupRef.current.lookAt(0, 0, 0);
      marsGroupRef.current.updateMatrixWorld(true);
      worldCenterRef.current.set(pos[0], pos[1], pos[2]);
      groupInverseMatrixRef.current.copy(marsGroupRef.current.matrixWorld).invert();
    }
  });

  return (
    <group ref={marsGroupRef}>
      <ClimateProvider parameters={parameters}>
        <MarsSurface relief={relief} />
        <MarsAtmosphere />
        <group userData={{ perfGroup: "tiles & board" }}>
          <TileGrid
            relief={relief}
            highlightRoot={marsGroupRef}
            gameState={gameState}
            onHexClick={onHexClick}
            animateHexEntrance={animateHexEntrance}
            startHidden={startHidden}
            sphereCenter={worldCenterRef.current}
            groupInverseMatrix={groupInverseMatrixRef.current}
            onStructureReady={onStructureReady}
          />
        </group>
      </ClimateProvider>
    </group>
  );
}

function MarsSurface({ relief }: { relief: MarsRelief }) {
  const { activePlanet, setActivePlanet } = usePlanetFocus();
  const { gl } = useThree();
  const { runtime } = useClimate();
  const reducedMotion = useReducedMotion();
  const { mars: diffuseMap, noiseMid } = useTextures();

  const sphereGeometry = useMemo(() => createMarsReliefGeometry(relief), [relief]);
  const reliefRaycast = useMemo(() => createMarsReliefRaycast(relief), [relief]);
  useEffect(() => () => sphereGeometry.dispose(), [sphereGeometry]);

  const climateUniforms = useMemo(() => createMarsClimateUniforms(noiseMid), [noiseMid]);

  const marsMaterial = useMemo(() => {
    const mat = new THREE.MeshStandardMaterial({
      map: diffuseMap,
      roughness: 0.8,
      metalness: 0,
      fog: false,
    });

    mat.stencilWrite = true;
    mat.stencilFunc = THREE.EqualStencilFunc;
    mat.stencilRef = 0;
    mat.stencilFuncMask = MOHOLE_STENCIL_BIT | LAKE_STENCIL_BIT | NUCLEAR_STENCIL_BIT;
    mat.stencilWriteMask = 0;
    mat.stencilFail = THREE.KeepStencilOp;
    mat.stencilZFail = THREE.KeepStencilOp;
    mat.stencilZPass = THREE.KeepStencilOp;
    addMarsClimate(mat, climateUniforms);

    return mat;
  }, [diffuseMap, climateUniforms]);

  useFrame(() => {
    const { current: climate, pulses } = runtime.current;
    climateUniforms.uChill.value = 1 - climate.sky;
    climateUniforms.uSweep.value = pulses.sweep;
    climateUniforms.uSweepFade.value = reducedMotion ? 1 : 0;
  });

  return (
    <mesh
      geometry={sphereGeometry}
      raycast={reliefRaycast}
      material={marsMaterial}
      userData={{ perfGroup: "mars sphere" }}
      // Solid ground and plants write depth first; only actual holes mask out Mars.
      renderOrder={16}
      onPointerEnter={(e) => {
        if (activePlanet !== "mars") {
          if (e.intersections[0]?.object !== e.object) {
            return;
          }
          gl.domElement.style.cursor = "pointer";
        }
      }}
      onPointerLeave={() => {
        if (activePlanet !== "mars") {
          gl.domElement.style.cursor = "default";
        }
      }}
      onClick={(e) => {
        if (activePlanet !== "mars") {
          e.stopPropagation();
          if (isDragClick()) {
            return;
          }
          gl.domElement.style.cursor = "default";
          setActivePlanet("mars");
        }
      }}
    />
  );
}

const SKY_PRESET = MARS_CLIMATE_ATMOSPHERES.map((preset) => ({
  thickness: preset.thickness,
  intensity: preset.intensity,
  halo: preset.halo,
  color: new THREE.Color(preset.color),
  shadowColor: new THREE.Color(preset.shadowColor),
}));

function MarsAtmosphere() {
  const { runtime } = useClimate();
  const profile = useMemo(() => ({ ...MARS_ATMOSPHERE }), []);
  const scratch = useMemo(
    () => ({ color: new THREE.Color(), shadow: new THREE.Color(), key: [-1, -1] }),
    [],
  );
  useFrame(() => {
    const { current: climate, pulses } = runtime.current;
    const key = scratch.key;
    if (key[0] === climate.sky && key[1] === pulses.warm) {
      return;
    }
    key[0] = climate.sky;
    key[1] = pulses.warm;
    const position = climate.sky * (SKY_PRESET.length - 1);
    const index = Math.min(SKY_PRESET.length - 2, Math.floor(position));
    const t = position - index;
    const from = SKY_PRESET[index];
    const to = SKY_PRESET[index + 1];
    scratch.color.lerpColors(from.color, to.color, t);
    scratch.shadow.lerpColors(from.shadowColor, to.shadowColor, t);
    profile.color = "#" + scratch.color.getHexString();
    profile.shadowColor = "#" + scratch.shadow.getHexString();
    profile.thickness = THREE.MathUtils.lerp(from.thickness, to.thickness, t);
    profile.intensity =
      THREE.MathUtils.lerp(from.intensity, to.intensity, t) * (1 + pulses.warm * 0.6);
    profile.halo = THREE.MathUtils.lerp(from.halo, to.halo, t);
  });
  return <PlanetAtmosphere radius={SPHERE_RADIUS} profile={profile} />;
}
