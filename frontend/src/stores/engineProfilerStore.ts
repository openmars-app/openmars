import { create } from "zustand";
import { engineProfiler, type ProfilerSnapshot } from "../engine/profiler";

export const useEngineProfilerStore = create<
  ProfilerSnapshot & {
    start(): void;
    stop(): void;
    download(): void;
    toggleReload(): void;
    clear(): void;
  }
>(() => ({
  ...engineProfiler.snapshot(),
  start: () => engineProfiler.start(),
  stop: () => engineProfiler.stop(),
  download: () => engineProfiler.download(),
  toggleReload: () => engineProfiler.toggleReload(),
  clear: () => engineProfiler.clear(),
}));
engineProfiler.subscribe((snapshot) => useEngineProfilerStore.setState(snapshot));
