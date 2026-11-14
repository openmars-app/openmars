import { create } from "zustand";
import {
  DEFAULT_CAMERA_SETTINGS,
  readCameraSettings,
  type CameraSettings,
} from "../engine/cameraMotion";

const STORAGE_KEY = "openmars.camera.settings";
const PREVIOUS_STORAGE_KEY = "openmars.camera";
type CameraStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export function loadCameraSettings(storage: CameraStorage): CameraSettings {
  try {
    const current = storage.getItem(STORAGE_KEY);
    if (current !== null) {
      return readCameraSettings(JSON.parse(current));
    }
    const previous = storage.getItem(PREVIOUS_STORAGE_KEY);
    if (previous === null) {
      return { ...DEFAULT_CAMERA_SETTINGS };
    }
    const settings = readCameraSettings(JSON.parse(previous));
    // Move saved preferences once; the old default did not carry a customized marker.
    // A subsequently chosen 35% value in the new key is an explicit preference.
    if (settings.orbitSensitivity === 0.35) {
      settings.orbitSensitivity = DEFAULT_CAMERA_SETTINGS.orbitSensitivity;
    }
    try {
      storage.setItem(STORAGE_KEY, JSON.stringify(settings));
      storage.removeItem(PREVIOUS_STORAGE_KEY);
    } catch {
      // Keep the existing preferences if storage cannot complete the move.
    }
    return settings;
  } catch {
    return { ...DEFAULT_CAMERA_SETTINGS };
  }
}

function loadSettings(): CameraSettings {
  try {
    return loadCameraSettings(localStorage);
  } catch {
    return { ...DEFAULT_CAMERA_SETTINGS };
  }
}

export const useCameraSettingsStore = create<
  CameraSettings & {
    update: (patch: Partial<CameraSettings>) => void;
    reset: () => void;
  }
>((set) => ({
  ...loadSettings(),
  update: (patch) => set((settings) => readCameraSettings({ ...settings, ...patch })),
  reset: () => set({ ...DEFAULT_CAMERA_SETTINGS }),
}));

useCameraSettingsStore.subscribe((settings) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(readCameraSettings(settings)));
  } catch {
    // Controls still work when browser storage is unavailable.
  }
});
