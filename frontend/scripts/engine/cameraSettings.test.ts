import { expect, test } from "bun:test";
import { DEFAULT_CAMERA_SETTINGS } from "../../src/engine/cameraMotion";
import { loadCameraSettings } from "../../src/stores/cameraSettingsStore";

const previousKey = "openmars.camera";
const currentKey = "openmars.camera.settings";
function storageFor(settings?: Record<string, string>) {
  const values = new Map(Object.entries(settings ?? {}));
  return {
    values,
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value);
    },
    removeItem: (key: string) => {
      values.delete(key);
    },
  };
}

test("camera preference upgrade lowers the former default and preserves other custom values", () => {
  const previous = {
    orbitSensitivity: 0.35,
    zoomSensitivity: 0.8,
    smoothing: 0.3,
    reverseZoom: true,
  };
  const storage = storageFor({ [previousKey]: JSON.stringify(previous) });
  const expected = { ...previous, orbitSensitivity: 0.15 };
  expect(loadCameraSettings(storage)).toEqual(expected);
  expect(JSON.parse(storage.getItem(currentKey)!)).toEqual(expected);
  expect(storage.getItem(previousKey)).toBeNull();
});

test("custom orbit sensitivity survives the upgrade and a later explicit 35% remains unchanged", () => {
  const previous = { ...DEFAULT_CAMERA_SETTINGS, orbitSensitivity: 0.6 };
  const storage = storageFor({ [previousKey]: JSON.stringify(previous) });
  expect(loadCameraSettings(storage)).toEqual(previous);
  const customized = { ...previous, orbitSensitivity: 0.35 };
  storage.setItem(currentKey, JSON.stringify(customized));
  expect(loadCameraSettings(storage)).toEqual(customized);
});

test("current camera settings take precedence over any old leftover value", () => {
  const current = { ...DEFAULT_CAMERA_SETTINGS, orbitSensitivity: 0.2 };
  const storage = storageFor({
    [previousKey]: JSON.stringify({ ...current, orbitSensitivity: 0.35 }),
    [currentKey]: JSON.stringify(current),
  });
  expect(loadCameraSettings(storage)).toEqual(current);
});

test("an interrupted preference upgrade does not delete the saved custom preferences", () => {
  const previous = { ...DEFAULT_CAMERA_SETTINGS, orbitSensitivity: 0.5 };
  const storage = storageFor({ [previousKey]: JSON.stringify(previous) });
  storage.setItem = () => {
    throw new Error("Storage unavailable");
  };
  expect(loadCameraSettings(storage)).toEqual(previous);
  expect(storage.getItem(previousKey)).toBe(JSON.stringify(previous));
  expect(storage.getItem(currentKey)).toBeNull();
});

test("missing or malformed camera preferences use the lower current default", () => {
  expect(DEFAULT_CAMERA_SETTINGS.orbitSensitivity).toBe(0.15);
  expect(loadCameraSettings(storageFor())).toEqual(DEFAULT_CAMERA_SETTINGS);
  expect(loadCameraSettings(storageFor({ [currentKey]: "{" }))).toEqual(DEFAULT_CAMERA_SETTINGS);
});
