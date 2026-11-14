import { devices } from "@playwright/test";
export const chromiumLaunchOptions = { args: ["--enable-gpu"] };
export function chromiumDevice(name: "iPhone 14 landscape" | "Pixel 7 landscape" | "iPhone 14") {
  const { viewport, deviceScaleFactor, isMobile, hasTouch } = devices[name];
  return {
    browserName: "chromium" as const,
    launchOptions: chromiumLaunchOptions,
    viewport,
    deviceScaleFactor,
    isMobile,
    hasTouch,
  };
}
