import { useEffect, useRef } from "react";
import { useEngineStore } from "../stores/engineStore";
import { effectAudio } from "./effectAudio";
import { placementSounds } from "./presentation";
import { useSoundEffects } from "../hooks/useSoundEffects";
import { useGraphicsSettingsStore } from "../stores/graphicsSettingsStore";
import { startEngine } from "./runtime";
import { engineProfiler } from "./profiler";
import { INITIAL_ENGINE_STATUS, type EngineStatus } from "./status";
import type { MenuPlacement, SceneEvent, SceneSnapshot, WorldEntry } from "./scene.generated";

interface EngineCanvasProps {
  onStatus: (status: EngineStatus) => void;
  paused?: boolean;
  scene: SceneSnapshot | null;
  connected: boolean;
  menuPlacement: MenuPlacement | null;
  entry: WorldEntry;
  onEvent: (event: SceneEvent) => void;
}

export default function EngineCanvas({
  onStatus,
  paused = false,
  scene,
  menuPlacement,
  entry,
  connected,
  onEvent,
}: EngineCanvasProps) {
  const { playConstructionSound, playWaterPlacementSound } = useSoundEffects();
  const graphics = useGraphicsSettingsStore((s) => s.settings);
  const previousScene = useRef<SceneSnapshot | null>(null);
  const focus = useEngineStore((state) => state.focus);
  const currentFocus = useRef(focus);
  currentFocus.current = focus;
  const canvas = useRef<HTMLDivElement>(null);
  const runtime = useRef<ReturnType<typeof startEngine> | null>(null);
  const sounds = useRef<ReturnType<typeof effectAudio> | null>(null);
  const report = useRef(onStatus);
  report.current = onStatus;
  const eventHandler = useRef(onEvent);
  eventHandler.current = onEvent;
  const previousConnected = useRef(false);
  const currentMenu = useRef(menuPlacement);
  currentMenu.current = menuPlacement;
  const currentEntry = useRef(entry);
  currentEntry.current = entry;
  const currentScene = useRef(scene);
  currentScene.current = scene;
  useEffect(() => {
    report.current(INITIAL_ENGINE_STATUS);
    const audio = effectAudio();
    sounds.current = audio;
    let previousPhase: EngineStatus["phase"] | undefined;
    const instance = startEngine(
      canvas.current!,
      (status) => {
        if (status.phase !== previousPhase) {
          engineProfiler.mark("renderer.status", {
            phase: status.phase,
            backend: status.backend ?? "auto",
          });
          previousPhase = status.phase;
        }
        if (status.phase === "error") {
          audio.dispose();
        }
        report.current(status);
      },
      undefined,
      (event) => eventHandler.current(event),
      (event) => {
        if (event.action === "stop" || event.gameId === currentScene.current?.gameId) {
          audio.dispatch(event);
        }
      },
      (overlay) => useEngineStore.getState().setOverlay(overlay),
    );
    instance.setMenu(currentMenu.current);
    instance.setScene(currentScene.current, "snapshot");
    instance.setWorldEntry(currentEntry.current);
    instance.setFocus(currentFocus.current);
    runtime.current = instance;
    useEngineStore.getState().setPresenter((cue) => instance.present(cue));
    useEngineStore.getState().setInspector(() => instance.sceneStats());
    useEngineStore.getState().setPerformance({
      setActive: (active) => instance.setPerformanceActive(active),
      sample: () => instance.performance(),
    });
    useEngineStore.getState().setCamera((zoom, reset) => instance.camera(zoom, reset));
    return () => {
      audio.dispose();
      sounds.current = null;
      useEngineStore.getState().setPresenter(null);
      instance.dispose();
      runtime.current = null;
      useEngineStore.getState().setInspector(null);
      useEngineStore.getState().setPerformance(null);
      useEngineStore.getState().setCamera(null);
    };
  }, []);
  useEffect(() => {
    runtime.current?.setPaused(paused);
    sounds.current?.pause(paused);
  }, [paused]);
  useEffect(() => {
    sounds.current?.dispose();
  }, [scene?.gameId]);
  useEffect(() => {
    runtime.current?.setMenu(menuPlacement);
  }, [menuPlacement]);
  useEffect(() => {
    runtime.current?.setFocus(focus);
  }, [focus]);
  useEffect(() => {
    runtime.current?.setGraphics(graphics);
  }, [graphics]);
  useEffect(() => {
    runtime.current?.setWorldEntry(entry);
  }, [entry]);
  useEffect(() => {
    const delivery = !connected || !previousConnected.current ? "snapshot" : "live";
    const placement = placementSounds(previousScene.current, scene, delivery);
    previousScene.current = scene;
    runtime.current?.setScene(scene, delivery);
    for (const sound of placement) {
      if (sound === "construction") void playConstructionSound();
      else void playWaterPlacementSound();
    }
    previousConnected.current = connected;
  }, [scene, connected, playConstructionSound, playWaterPlacementSound]);
  return (
    <div
      ref={canvas}
      className="block h-full w-full"
      style={{ touchAction: "none", pointerEvents: scene ? "auto" : "none" }}
    />
  );
}
