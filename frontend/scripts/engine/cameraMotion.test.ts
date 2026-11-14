import { expect, test } from "bun:test";
import {
  CameraMotion,
  DEFAULT_CAMERA_SETTINGS,
  readCameraSettings,
  wheelZoom,
} from "../../src/engine/cameraMotion";

test("camera targets ease toward input without overshoot and settle exactly", () => {
  const motion = new CameraMotion();
  motion.add(1, -0.5, 0.2);
  let moved = [0, 0, 0];
  const first = motion.step(1 / 60, 0.18);
  expect(first[0]).toBeGreaterThan(0);
  expect(first[0]).toBeLessThan(0.3);
  moved = first;
  for (let frame = 0; frame < 60; frame++) {
    const change = motion.step(1 / 60, 0.18);
    moved = moved.map((value, index) => value + change[index]);
    expect(moved[0]).toBeLessThanOrEqual(1);
  }
  expect(moved[0]).toBeCloseTo(1, 8);
  expect(moved[1]).toBeCloseTo(-0.5, 8);
  expect(moved[2]).toBeCloseTo(0.2, 8);
  expect(motion.step(1 / 60, 0.18)).toEqual([0, 0, 0]);
});

function run(frames: number) {
  const motion = new CameraMotion();
  motion.add(0.8, 0, 0);
  let yaw = 0;
  for (let frame = 0; frame < frames; frame++) {
    yaw += motion.step(0.1 / frames, 0.18)[0];
  }
  return yaw;
}

test("camera smoothing is frame-rate independent and cancellation discards momentum", () => {
  expect(run(6)).toBeCloseTo(run(24), 10);
  const motion = new CameraMotion();
  motion.add(2, 1, 0.3);
  motion.step(0.016, 0.18);
  motion.clear();
  expect(motion.step(0.016, 0.18)).toEqual([0, 0, 0]);
  motion.add(0.5, 0, 0);
  expect(motion.step(0, 0)).toEqual([0.5, 0, 0]);
});

test("wheel units normalize consistently and extreme deltas remain bounded", () => {
  const settings = DEFAULT_CAMERA_SETTINGS;
  expect(wheelZoom(16, 0, 800, settings)).toBe(wheelZoom(1, 1, 800, settings));
  expect(wheelZoom(800, 0, 800, settings)).toBe(wheelZoom(1, 2, 800, settings));
  expect(wheelZoom(100000, 0, 800, settings)).toBe(0.35);
  expect(wheelZoom(16, 0, 800, { ...settings, reverseZoom: true })).toBe(-0.008);
  expect(wheelZoom(NaN, 0, 800, settings)).toBe(0);
});

test("stored camera preferences reject invalid values and clamp usable ranges", () => {
  expect(readCameraSettings(null)).toEqual(DEFAULT_CAMERA_SETTINGS);
  expect(
    readCameraSettings({
      orbitSensitivity: NaN,
      zoomSensitivity: -1,
      smoothing: 10,
      reverseZoom: true,
    }),
  ).toEqual({
    ...DEFAULT_CAMERA_SETTINGS,
    zoomSensitivity: 0.1,
    smoothing: 0.4,
    reverseZoom: true,
  });
});
