export interface CameraSettings {
  orbitSensitivity: number;
  zoomSensitivity: number;
  smoothing: number;
  reverseZoom: boolean;
}

export const DEFAULT_CAMERA_SETTINGS: CameraSettings = {
  orbitSensitivity: 0.15,
  zoomSensitivity: 0.5,
  smoothing: 0.18,
  reverseZoom: false,
};

export function readCameraSettings(value: unknown): CameraSettings {
  const stored = value && typeof value === "object" ? (value as Partial<CameraSettings>) : {};
  const number = (
    key: "orbitSensitivity" | "zoomSensitivity" | "smoothing",
    min: number,
    max: number,
  ) => {
    const candidate = stored[key];
    return typeof candidate === "number" && Number.isFinite(candidate)
      ? Math.max(min, Math.min(max, candidate))
      : DEFAULT_CAMERA_SETTINGS[key];
  };
  return {
    orbitSensitivity: number("orbitSensitivity", 0.1, 2),
    zoomSensitivity: number("zoomSensitivity", 0.1, 2),
    smoothing: number("smoothing", 0, 0.4),
    reverseZoom: typeof stored.reverseZoom === "boolean" ? stored.reverseZoom : false,
  };
}

/** Wheel pixels, lines and pages share one bounded logarithmic zoom scale. */
export function wheelZoom(
  delta: number,
  mode: number,
  height: number,
  settings: CameraSettings,
): number {
  if (!Number.isFinite(delta)) {
    return 0;
  }
  const pixels = delta * (mode === 1 ? 16 : mode === 2 ? height : 1);
  const zoom = Math.max(-0.35, Math.min(0.35, pixels * 0.001 * settings.zoomSensitivity));
  return settings.reverseZoom ? -zoom : zoom;
}

/** Critically damped targets keep event frequency separate from camera movement. */
export class CameraMotion {
  private axes = Array.from({ length: 3 }, () => ({ current: 0, target: 0, velocity: 0 }));

  add(yaw: number, pitch: number, zoom: number) {
    for (const [index, delta] of [yaw, pitch, zoom].entries()) {
      if (Number.isFinite(delta)) {
        this.axes[index].target += delta;
      }
    }
  }

  clear() {
    for (const axis of this.axes) {
      axis.current = axis.target = axis.velocity = 0;
    }
  }

  step(deltaSeconds: number, smoothing: number): [number, number, number] {
    const dt = Math.max(0, Math.min(0.1, deltaSeconds));
    const result = this.axes.map((axis) => {
      const before = axis.current;
      if (smoothing <= 0) {
        axis.current = axis.target;
        axis.velocity = 0;
      } else if (dt > 0) {
        const frequency = 8 / smoothing;
        const difference = axis.current - axis.target;
        const impulse = (axis.velocity + frequency * difference) * dt;
        const decay = Math.exp(-frequency * dt);
        axis.current = axis.target + (difference + impulse) * decay;
        axis.velocity = (axis.velocity - frequency * impulse) * decay;
        if (Math.abs(axis.target - axis.current) < 0.000001 && Math.abs(axis.velocity) < 0.00001) {
          axis.current = axis.target;
          axis.velocity = 0;
        }
      }
      return axis.current - before;
    }) as [number, number, number];
    if (this.axes.every((axis) => axis.current === axis.target && axis.velocity === 0)) {
      this.clear();
    }
    return result;
  }
}
