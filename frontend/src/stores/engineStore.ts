import type { FocusTarget, OverlayFrame, SceneCue } from "../engine/scene.generated";
import type { SceneDiagnostics } from "../engine/diagnostics";
import { create } from "zustand";
import { INITIAL_ENGINE_STATUS, type EngineStatus } from "@/engine/status";
import type { PerformanceSource } from "@/engine/performance";

export const useEngineStore = create<{
  status: EngineStatus;
  focus: FocusTarget;
  setFocus: (focus: FocusTarget) => void;
  overlay: OverlayFrame | null;
  setOverlay: (overlay: OverlayFrame | null) => void;
  present: ((cue: SceneCue) => boolean) | null;
  setPresenter: (present: ((cue: SceneCue) => boolean) | null) => void;
  inspectScene: (() => SceneDiagnostics | null) | null;
  setInspector: (inspectScene: (() => SceneDiagnostics | null) | null) => void;
  performance: PerformanceSource | null;
  setPerformance: (source: PerformanceSource | null) => void;
  camera: ((zoom: number, reset?: boolean) => void) | null;
  setCamera: (camera: ((zoom: number, reset?: boolean) => void) | null) => void;
  setStatus: (status: EngineStatus) => void;
}>((set) => ({
  status: INITIAL_ENGINE_STATUS,
  focus: { type: "body", body: "mars" },
  setFocus: (focus) => set({ focus }),
  overlay: null,
  setOverlay: (overlay) => set({ overlay }),
  camera: null,
  present: null,
  setPresenter: (present) => set({ present }),
  inspectScene: null,
  setInspector: (inspectScene) => set({ inspectScene }),
  performance: null,
  setPerformance: (performance) => set({ performance }),
  setCamera: (camera) => set({ camera }),
  setStatus: (status) => set({ status }),
}));
