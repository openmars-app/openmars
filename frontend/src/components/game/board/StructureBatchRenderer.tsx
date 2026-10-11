import { useScenePreparation } from "../../../contexts/ScenePreparationContext";
import { memo, useEffect, useLayoutEffect, useMemo, useRef } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import * as THREE from "three";
import { useTextures } from "../../../hooks/useTextures";
import { useClimate } from "../../../contexts/ClimateContext";
import { HexGrid2D } from "../../../utils/hex-grid-2d";
import type { StructurePlot } from "./structureLayout";
import { StructureBatchStore } from "./structureBatch";
import { STRUCTURE_EMERGENCE_DURATION } from "./boardConstants";
import { NUCLEAR_PLAYBACK_RATE } from "./nuclearGeometry";
import { getMaterials } from "./StructureRenderer";
import { splitSnippet } from "./shaders";
import type { NuclearTransitions } from "./nuclearTransitions";
import collapseNormal from "./shaders/structure-collapse-normal.vert.glsl?raw";
import emergence from "./shaders/structure-emergence.vert.glsl?raw";
import frostFragment from "./shaders/structure-frost.frag.glsl?raw";

function StructureBatchRenderer({
  plots,
  newlyPlaced,
  nuclearTransitions = EMPTY_TRANSITIONS,
  visible = true,
  onReady,
}: {
  plots: StructurePlot[];
  newlyPlaced: Set<string>;
  nuclearTransitions?: NuclearTransitions;
  visible?: boolean;
  onReady?: () => void;
}) {
  const { begin, finish } = useScenePreparation("Structure geometry");
  const clock = useThree((s) => s.clock);
  const textures = useTextures();
  const { runtime } = useClimate();
  const birthTimes = useRef(new Map<string, number>());
  const time = useMemo(() => ({ value: clock.elapsedTime }), []);
  const frost = useMemo(() => ({ value: runtime.current.current.frost }), [runtime]);
  const materials = useMemo(
    () => createStructureBatchMaterials(textures, time, frost),
    [
      textures.concrete,
      textures.grass,
      textures.sand,
      textures.structureFacades,
      textures.noiseMid,
      time,
      frost,
    ],
  );
  const store = useMemo(() => new StructureBatchStore(materials), [materials]);
  const latest = useRef({ plots, nuclearTransitions, onReady });
  useFrame(() => {
    time.value = clock.elapsedTime;
    frost.value = runtime.current.current.frost;
    store.group.visible = !store.ready || visible;
  });
  useLayoutEffect(() => {
    begin();
    store.beginPreparation();
    let frame = 0;
    const prepare = () => {
      if (!store.prepareNext()) {
        frame = requestAnimationFrame(prepare);
        return;
      }
      for (const [key, birth] of birthTimes.current) {
        if (birth >= 0) {
          birthTimes.current.set(key, clock.elapsedTime);
        }
      }
      store.update(latest.current.plots, birthTimes.current);
      store.setCollapses(latest.current.nuclearTransitions);
      latest.current.onReady?.();
      finish();
    };
    frame = requestAnimationFrame(prepare);
    return () => {
      cancelAnimationFrame(frame);
      finish();
      store.dispose();
    };
  }, [store, clock, begin, finish]);
  useLayoutEffect(() => {
    latest.current = { plots, nuclearTransitions, onReady };
    const keys = new Set(plots.map((p) => HexGrid2D.coordinateToKey(p.coordinate)));
    for (const key of birthTimes.current.keys()) {
      if (!keys.has(key)) {
        birthTimes.current.delete(key);
      }
    }
    for (const key of keys) {
      if (!birthTimes.current.has(key)) {
        birthTimes.current.set(key, newlyPlaced.has(key) ? clock.elapsedTime : -1000);
      }
    }
    if (store.ready) {
      store.update(plots, birthTimes.current);
      store.setCollapses(nuclearTransitions);
    }
  }, [plots, store, nuclearTransitions, newlyPlaced, onReady, clock]);
  useEffect(
    () => () => {
      for (const material of materials.values()) {
        material.dispose();
      }
    },
    [materials],
  );
  return <primitive object={store.group} dispose={null} />;
}

export function createStructureBatchMaterials(
  textures: Parameters<typeof getMaterials>[1] & Pick<ReturnType<typeof useTextures>, "noiseMid">,
  time: { value: number },
  frost: { value: number },
) {
  const result = new Map<string, THREE.MeshStandardMaterial>();
  const snippet = splitSnippet(emergence);
  const frostSnippet = splitSnippet(frostFragment);
  for (const mode of ["normal", "bright"] as const) {
    for (const [index, source] of getMaterials(mode, textures).entries()) {
      if (mode === "bright" && index >= 3) {
        continue;
      }
      const material = source.clone();
      const frostable = index !== 11 && index !== 14 && index !== 15;
      material.onBeforeCompile = (shader) => {
        shader.uniforms.uStructureTime = time;
        shader.uniforms.uStructureDuration = { value: STRUCTURE_EMERGENCE_DURATION };
        shader.uniforms.uNuclearPlaybackRate = { value: NUCLEAR_PLAYBACK_RATE };
        shader.vertexShader =
          snippet.header +
          "\n" +
          shader.vertexShader
            .replace("#include <begin_vertex>", snippet.body)
            .replace("#include <beginnormal_vertex>", collapseNormal);
        if (frostable) {
          shader.uniforms.uStructureFrost = frost;
          shader.uniforms.uStructureFrostNoise = { value: textures.noiseMid };
          shader.uniforms.uStructureFrostGlass = { value: index === 10 ? 1 : 0 };
          shader.fragmentShader =
            frostSnippet.header +
            "\n" +
            shader.fragmentShader.replace(
              "#include <emissivemap_fragment>",
              "#include <emissivemap_fragment>\n" + frostSnippet.body,
            );
        }
      };
      material.customProgramCacheKey = () =>
        emergence + collapseNormal + (frostable ? frostFragment : "");
      result.set(`${mode}:${index}`, material);
    }
  }
  return result;
}

const EMPTY_TRANSITIONS: NuclearTransitions = new Map();
export default memo(StructureBatchRenderer);
