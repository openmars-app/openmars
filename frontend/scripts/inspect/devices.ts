import { devices, chromium, firefox } from "@playwright/test";
export const chromiumLaunchOptions = { args: ["--enable-gpu"] };
export const inspectionDevices = {
  desktop: {
    browser: firefox,
    context: { viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 },
    tier: "high",
  },
  iphone: {
    browser: chromium,
    context: { ...devices["iPhone 14 landscape"], defaultBrowserType: undefined },
    tier: "low",
  },
  android: {
    browser: chromium,
    context: { ...devices["Pixel 7 landscape"], defaultBrowserType: undefined },
    tier: "low",
  },
} as const;
export type InspectionDevice = keyof typeof inspectionDevices;
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
