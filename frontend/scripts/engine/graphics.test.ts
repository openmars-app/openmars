import { expect, test } from "bun:test";
import {
  FrameLimiter,
  GRAPHICS_PRESETS,
  TEXTURE_LIMITS,
  graphicsPreset,
  readGraphicsSettings,
  renderDimensions,
  residentGraphicsKey,
} from "../../src/engine/graphics";

test("graphics preferences reject corrupt storage and preserve valid overrides", () => {
  expect(readGraphicsSettings(null)).toEqual(GRAPHICS_PRESETS.high);
  expect(readGraphicsSettings(null, GRAPHICS_PRESETS.medium)).toEqual(GRAPHICS_PRESETS.medium);
  expect(readGraphicsSettings({ frameLimit: -1 }, GRAPHICS_PRESETS.medium)).toEqual(
    GRAPHICS_PRESETS.medium,
  );
  expect(
    readGraphicsSettings({
      renderScale: Infinity,
      frameLimit: -1,
      antialiasing: "false",
      textureQuality: "ultra",
      effects: null,
      backgroundBrightness: NaN,
    }),
  ).toEqual(GRAPHICS_PRESETS.high);
  expect(readGraphicsSettings(GRAPHICS_PRESETS.low)).toEqual(GRAPHICS_PRESETS.low);
  expect(graphicsPreset({ ...GRAPHICS_PRESETS.high, backgroundBrightness: 0.4 })).toBe("high");
  expect(graphicsPreset({ ...GRAPHICS_PRESETS.high, renderScale: 0.9 })).toBe("custom");
  expect(
    residentGraphicsKey({
      ...GRAPHICS_PRESETS.high,
      antialiasing: false,
      renderScale: 0.5,
      frameLimit: 30,
    }),
  ).toBe(residentGraphicsKey(GRAPHICS_PRESETS.high));
  expect(residentGraphicsKey(GRAPHICS_PRESETS.low)).not.toBe(
    residentGraphicsKey(GRAPHICS_PRESETS.high),
  );
  expect([TEXTURE_LIMITS.low, TEXTURE_LIMITS.medium, TEXTURE_LIMITS.high]).toEqual([
    1024, 2048, 4096,
  ]);
});

test("quality scaling reduces real pixel cost and respects adapter limits without changing aspect", () => {
  const high = renderDimensions(1600, 900, 1, 8192, 1);
  const low = renderDimensions(1600, 900, 1, 8192, 0.65);
  expect([low.width, low.height]).toEqual([1040, 585]);
  expect((low.width * low.height) / (high.width * high.height)).toBeCloseTo(0.4225, 4);
  const limited = renderDimensions(1600, 900, 2, 2048, 1.5);
  expect([limited.width, limited.height]).toEqual([2048, 1152]);
});

test("FPS caps work across refresh rates without rounding down to a monitor divisor", () => {
  for (const refresh of [60, 144, 240]) {
    for (const limit of [0, 30, 60, 90, 120, 144, 165, 240]) {
      const limiter = new FrameLimiter();
      let frames = 0;
      for (let i = 0; i < refresh * 10; i++) if (limiter.due((i * 1000) / refresh, limit)) frames++;
      expect(Math.abs(frames - Math.min(refresh, limit || refresh) * 10)).toBeLessThanOrEqual(1);
      expect(limiter.due(12000, limit)).toBe(true);
    }
  }
});
