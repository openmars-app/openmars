import { useScenePreparation } from "../../../contexts/ScenePreparationContext";
import { memo, useCallback, useEffect, useLayoutEffect, useMemo } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import * as THREE from "three";
import { useTextures } from "../../../hooks/useTextures";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext";
import { useClimate } from "../../../contexts/ClimateContext";
import { LandscapeSurface } from "./landscapeSurface";
import { textureArray } from "./textureArray";
import type { PlantShadeMap } from "./plantShade";
import { performanceStore } from "../../../services/performanceStore";
import type { LandscapeState } from "./landscapeTypes";
const noop = () => {};
function LandscapeRenderer({
  state,
  capacity = 128,
  onReady = noop,
  groupInverseMatrix,
  plantShade,
}: {
  plantShade?: PlantShadeMap;
  state: LandscapeState;
  capacity?: number;
  onReady?: (state: LandscapeState, transitionStart: number) => void;
  groupInverseMatrix?: THREE.Matrix4;
}) {
  const { begin, finish } = useScenePreparation("Landscape upload");
  const textures = useTextures();
  const gl = useThree((s) => s.gl);
  const { settings } = useWorld3DSettings();
  const { runtime } = useClimate();
  const surface = useMemo(
    () =>
      new LandscapeSurface(
        textures,
        {
          groundAlbedo: textureArray(gl, textures.groundLayers.color, THREE.SRGBColorSpace),
          groundDetail: textureArray(gl, textures.groundLayers.detail, THREE.NoColorSpace),
          iceAlbedo: textureArray(gl, textures.iceLayers.color, THREE.SRGBColorSpace),
          iceDetail: textureArray(gl, textures.iceLayers.detail, THREE.NoColorSpace),
        },
        capacity,
      ),
    [
      gl,
      capacity,
      textures.mars,
      textures.grass,
      textures.sand,
      textures.rock,
      textures.concrete,
      textures.waterNormals,
      textures.noiseMid,
      textures.groundLayers,
      textures.iceLayers,
    ],
  );
  useLayoutEffect(() => {
    begin();
    surface.enqueue(state);
    return finish;
  }, [surface, state, begin, finish]);
  useLayoutEffect(() => {
    gl.initTexture(surface.terrain);
    gl.initTexture(surface.materials);
    gl.initTexture(surface.detail);
  }, [gl, surface]);
  useEffect(() => {
    if (plantShade) {
      surface.prepareBake(gl);
    }
  }, [surface, gl, plantShade]);
  // Only the board bakes; other instances (the warmup) draw the bake programs for it once their
  // patches are in, since a draw without instances leaves the programs unfinished.
  const handleReady = useCallback(
    (committed: LandscapeState, transitionStart: number) => {
      if (!plantShade) {
        surface.warmBake(gl);
      }
      onReady(committed, transitionStart);
      finish();
    },
    [surface, gl, plantShade, onReady, finish],
  );
  useEffect(() => {
    surface.retain();
    return () => surface.release();
  }, [surface]);
  useFrame(({ camera, clock }) => {
    surface.tick(gl, clock.elapsedTime * 1000, handleReady);
    if (plantShade && performanceStore.sectionDue("Landscape")) {
      performanceStore.setSection("Landscape", surface.stats());
    }
    const { current: climate, pulses } = runtime.current;
    const shared = surface.uniforms;
    shared.uFrost.value = climate.frost;
    shared.uChill.value = 1 - climate.sky;
    shared.uGreening.value = climate.greening;
    shared.uMeadow.value = climate.meadow;
    if (plantShade) {
      shared.uPlantShade.value = plantShade.target.texture;
      shared.uPlantShadeBounds.value.copy(plantShade.bounds);
    }
    shared.uIce.value = climate.ice;
    shared.uIceAge.value = climate.iceAge;
    shared.uClarity.value = climate.clarity;
    shared.uAlgae.value = climate.algae;
    shared.uWaves.value = climate.waves;
    shared.uFoam.value = climate.foam;
    shared.uShimmer.value = pulses.shimmer;
    const u = surface.water.uniforms;
    u.time.value = (clock.elapsedTime * 1000) / 1000;
    u.eye.value.copy(camera.position);
    if (groupInverseMatrix) {
      u.eye.value.applyMatrix4(groupInverseMatrix);
    }
    u.sunDirection.value
      .set(settings.sunDirectionX, settings.sunDirectionY, settings.sunDirectionZ)
      .normalize();
    u.sunColor.value.set(settings.sunColor.r, settings.sunColor.g, settings.sunColor.b);
    u.sunIntensity.value = settings.sunIntensity;
    u.rf0.value = settings.reflectance;
    if (plantShade) {
      surface.updateBake(gl, camera, clock.elapsedTime * 1000, plantShade.revision);
      if (performanceStore.sectionDue("Ground bake")) {
        performanceStore.setSection("Ground bake", surface.bakeStats());
      }
    }
  });
  return <primitive object={surface.group} dispose={null} />;
}
export default memo(LandscapeRenderer);
