import { createContext, useContext } from "react";

/** A separate portal and input boundary for locally rendered UI previews. */
export const OverlayEnvironment = createContext<HTMLElement | null>(null);

export function useOverlayPortalRoot(): HTMLElement {
  return useContext(OverlayEnvironment) ?? document.body;
}

export function useOverlayEventTarget(): Pick<
  Document,
  "addEventListener" | "removeEventListener"
> {
  // Both surfaces implement the same DOM event map; expose its typed overloads.
  return (useContext(OverlayEnvironment) ?? document) as Pick<
    Document,
    "addEventListener" | "removeEventListener"
  >;
}

export function useIsolatedOverlay(): boolean {
  return useContext(OverlayEnvironment) !== null;
}

export function isPreviewEvent(event: Event): boolean {
  return event.target instanceof Element && !!event.target.closest("[data-ui-debug-preview]");
}
