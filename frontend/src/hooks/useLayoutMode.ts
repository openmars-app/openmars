import { useSyncExternalStore } from "react";
import { FORCE_COMPACT } from "@/utils/layoutParams.ts";

export type LayoutMode = "desktop" | "compact";

export interface LayoutModeState {
  mode: LayoutMode;
  isCompact: boolean;
  isPortrait: boolean;
  isCoarsePointer: boolean;
  isSmallViewport: boolean;
  isPhoneScreen: boolean;
}

export const COMPACT_MAX_SHORT_SIDE = 600;

const COARSE_POINTER_QUERY = "(pointer: coarse)";
const PORTRAIT_QUERY = "(orientation: portrait)";

/**
 * The device decision uses the physical screen, not the window: mobile browsers inflate
 * `innerWidth` when the page is zoomed out or content overflows, which would make a phone
 * look like a desktop.
 */
export function getLayoutMode(): LayoutModeState {
  const isCoarsePointer = window.matchMedia(COARSE_POINTER_QUERY).matches;
  const isPortrait = window.matchMedia(PORTRAIT_QUERY).matches;
  const isSmallViewport = Math.min(window.innerWidth, window.innerHeight) < COMPACT_MAX_SHORT_SIDE;
  const isPhoneScreen =
    Math.min(window.screen.width, window.screen.height) < COMPACT_MAX_SHORT_SIDE;
  const isCompact = FORCE_COMPACT || (isCoarsePointer && isPhoneScreen);
  return {
    mode: isCompact ? "compact" : "desktop",
    isCompact,
    isPortrait,
    isCoarsePointer,
    isSmallViewport,
    isPhoneScreen,
  };
}

let cachedSnapshot: LayoutModeState | null = null;

function getSnapshot(): LayoutModeState {
  const next = getLayoutMode();
  if (
    cachedSnapshot &&
    cachedSnapshot.mode === next.mode &&
    cachedSnapshot.isPortrait === next.isPortrait &&
    cachedSnapshot.isCoarsePointer === next.isCoarsePointer &&
    cachedSnapshot.isSmallViewport === next.isSmallViewport &&
    cachedSnapshot.isPhoneScreen === next.isPhoneScreen
  ) {
    return cachedSnapshot;
  }
  cachedSnapshot = next;
  return next;
}

function subscribe(onChange: () => void) {
  const coarse = window.matchMedia(COARSE_POINTER_QUERY);
  const portrait = window.matchMedia(PORTRAIT_QUERY);
  coarse.addEventListener("change", onChange);
  portrait.addEventListener("change", onChange);
  window.addEventListener("resize", onChange);
  return () => {
    coarse.removeEventListener("change", onChange);
    portrait.removeEventListener("change", onChange);
    window.removeEventListener("resize", onChange);
  };
}

export function useLayoutMode(): LayoutModeState {
  return useSyncExternalStore(subscribe, getSnapshot);
}
