import { create } from "zustand";
import { getLayoutMode } from "../hooks/useLayoutMode";
import { GRAPHICS_PRESETS, readGraphicsSettings } from "../engine/graphics";
import type { GraphicsSettings } from "../engine/scene.generated";

const STORAGE_KEY = "openmars.graphics.settings";
export function defaultGraphicsSettings(): GraphicsSettings {
  return { ...GRAPHICS_PRESETS[getLayoutMode().isCompact ? "medium" : "high"] };
}
function load() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    return saved === null
      ? defaultGraphicsSettings()
      : readGraphicsSettings(JSON.parse(saved), defaultGraphicsSettings());
  } catch {
    return defaultGraphicsSettings();
  }
}
export const useGraphicsSettingsStore = create<{
  settings: GraphicsSettings;
  apply: (settings: GraphicsSettings) => void;
}>((set) => ({
  settings: load(),
  apply: (settings) => set({ settings: readGraphicsSettings(settings, defaultGraphicsSettings()) }),
}));
useGraphicsSettingsStore.subscribe(({ settings }) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  } catch {
    /* Settings remain usable when storage is unavailable. */
  }
});
