import {
  MAX_ASSET_FILE_BYTES,
  SCENE_ASSET_CACHE_BYTES,
  type GraphicsSettings,
  type Quality,
} from "./scene.generated";

// The default cache fits Medium. High must hold its complete celestial working
// set; a smaller LRU otherwise misses on every asset during sequential re-entry.
export const ASSET_CACHE_BUDGETS: Record<Quality, number> = {
  low: MAX_ASSET_FILE_BYTES,
  medium: SCENE_ASSET_CACHE_BYTES,
  high: SCENE_ASSET_CACHE_BYTES * 2,
};

export const GRAPHICS_PRESETS: Record<Quality, Readonly<GraphicsSettings>> = {
  low: {
    textureQuality: "low",
    renderScale: 0.65,
    antialiasing: false,
    effects: "low",
    frameLimit: 60,
    backgroundBrightness: 0.65,
  },
  medium: {
    textureQuality: "medium",
    renderScale: 0.85,
    antialiasing: true,
    effects: "medium",
    frameLimit: 0,
    backgroundBrightness: 0.65,
  },
  high: {
    textureQuality: "high",
    renderScale: 1,
    antialiasing: true,
    effects: "high",
    frameLimit: 0,
    backgroundBrightness: 0.65,
  },
};
export const FRAME_LIMITS = [0, 30, 60, 90, 120, 144, 165, 240] as const;
export const TEXTURE_LIMITS: Record<Quality, number> = { low: 1024, medium: 2048, high: 4096 };
const quality = (value: unknown): value is Quality =>
  ["low", "medium", "high"].includes(String(value));

export function readGraphicsSettings(
  input: unknown,
  defaults: Readonly<GraphicsSettings> = GRAPHICS_PRESETS.high,
): GraphicsSettings {
  const value = input && typeof input === "object" ? (input as Partial<GraphicsSettings>) : {};
  const bounded = (number: unknown, min: number, max: number, fallback: number) =>
    typeof number === "number" && Number.isFinite(number) && number >= min && number <= max
      ? number
      : fallback;
  return {
    textureQuality: quality(value.textureQuality) ? value.textureQuality : defaults.textureQuality,
    effects: quality(value.effects) ? value.effects : defaults.effects,
    renderScale: bounded(value.renderScale, 0.5, 1.5, defaults.renderScale),
    backgroundBrightness: bounded(
      value.backgroundBrightness,
      0.1,
      1,
      defaults.backgroundBrightness,
    ),
    antialiasing:
      typeof value.antialiasing === "boolean" ? value.antialiasing : defaults.antialiasing,
    frameLimit: FRAME_LIMITS.some((limit) => limit === value.frameLimit)
      ? value.frameLimit!
      : defaults.frameLimit,
  };
}

export function residentGraphicsKey(settings: GraphicsSettings): string {
  return `${settings.textureQuality}:${settings.effects}`;
}
export function graphicsPreset(settings: GraphicsSettings): Quality | "custom" {
  for (const key of ["low", "medium", "high"] as const) {
    const preset = GRAPHICS_PRESETS[key];
    if (
      Object.keys(preset)
        .filter((field) => field !== "backgroundBrightness")
        .every(
          (field) =>
            preset[field as keyof GraphicsSettings] === settings[field as keyof GraphicsSettings],
        )
    ) {
      return key;
    }
  }
  return "custom";
}

/** Uniform scaling preserves projection and pointer coordinates at every quality. */
export function renderDimensions(
  width: number,
  height: number,
  dpr: number,
  limit: number,
  scale: number,
) {
  const ratio = Math.min(Math.min(dpr, 1.5) * scale, limit / width, limit / height);
  return {
    width: Math.max(1, Math.round(width * ratio)),
    height: Math.max(1, Math.round(height * ratio)),
    ratio,
  };
}

/** Accumulate elapsed time rather than rounding a requested rate to monitor divisors. */
export class FrameLimiter {
  private next = 0;
  private limit = 0;
  due(timestamp: number, limit: number): boolean {
    if (limit !== this.limit) {
      this.limit = limit;
      this.next = 0;
    }
    if (!limit) {
      return true;
    }
    if (timestamp + 0.1 < this.next) {
      return false;
    }
    const interval = 1000 / limit;
    this.next = this.next
      ? this.next + Math.max(1, Math.floor((timestamp - this.next) / interval) + 1) * interval
      : timestamp + interval;
    return true;
  }
  reset() {
    this.next = 0;
  }
}
