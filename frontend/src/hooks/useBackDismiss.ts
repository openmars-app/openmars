import {
  useOverlayEventTarget,
  useIsolatedOverlay,
  isPreviewEvent,
} from "@/contexts/OverlayEnvironment";
import { useEffect, useRef } from "react";

const HISTORY_KEY = "mobileOverlayDepth";

interface DismissEntry {
  close: () => void;
}

const stack: DismissEntry[] = [];
const isolatedStacks = new WeakMap<object, DismissEntry[]>();
let pushedEntries = 0;
let ownPops = 0;
let syncScheduled = false;

function depthOf(state: unknown): number {
  if (state && typeof state === "object" && HISTORY_KEY in state) {
    return Number((state as Record<string, unknown>)[HISTORY_KEY]) || 0;
  }
  return 0;
}

function syncHistory() {
  syncScheduled = false;
  while (pushedEntries < stack.length) {
    pushedEntries += 1;
    window.history.pushState({ [HISTORY_KEY]: pushedEntries }, "");
  }
  if (pushedEntries > stack.length) {
    if (depthOf(window.history.state) !== pushedEntries) {
      pushedEntries = stack.length;
      return;
    }
    const delta = pushedEntries - stack.length;
    pushedEntries = stack.length;
    ownPops += 1;
    window.history.go(-delta);
  }
}

function scheduleSync() {
  if (syncScheduled) {
    return;
  }
  syncScheduled = true;
  queueMicrotask(syncHistory);
}

// Registered at module load in the capture phase so it runs before the in-game
// "close game?" back-button trap and the router, which must not see overlay pops.
function handlePopState(event: PopStateEvent) {
  if (ownPops > 0) {
    ownPops -= 1;
    event.stopImmediatePropagation();
    return;
  }
  const depth = depthOf(event.state);
  if (pushedEntries === 0 && depth === 0) {
    return;
  }
  event.stopImmediatePropagation();
  if (depth > pushedEntries) {
    ownPops += 1;
    window.history.go(pushedEntries - depth);
    return;
  }
  pushedEntries = depth;
  const dismissed = stack.slice(depth);
  for (let index = dismissed.length - 1; index >= 0; index -= 1) {
    dismissed[index].close();
  }
}

function handleKeyDown(event: KeyboardEvent) {
  if (event.key !== "Escape" || event.defaultPrevented || isPreviewEvent(event)) {
    return;
  }
  const top = stack[stack.length - 1];
  if (top) {
    top.close();
  }
}

if (typeof window !== "undefined") {
  window.addEventListener("popstate", handlePopState, { capture: true });
  window.addEventListener("keydown", handleKeyDown);
}

export function useBackDismiss(onDismiss: () => void) {
  const isolated = useIsolatedOverlay();
  const eventTarget = useOverlayEventTarget();
  const onDismissRef = useRef(onDismiss);
  onDismissRef.current = onDismiss;

  useEffect(() => {
    if (isolated) {
      let entries = isolatedStacks.get(eventTarget);
      if (!entries) {
        entries = [];
        isolatedStacks.set(eventTarget, entries);
      }
      const entry: DismissEntry = { close: () => onDismissRef.current() };
      entries.push(entry);
      const dismiss = (event: KeyboardEvent) => {
        if (event.key === "Escape" && !event.defaultPrevented && entries.at(-1) === entry) {
          event.preventDefault();
          onDismissRef.current();
        }
      };
      eventTarget.addEventListener("keydown", dismiss);
      return () => {
        eventTarget.removeEventListener("keydown", dismiss);
        const index = entries.indexOf(entry);
        if (index !== -1) entries.splice(index, 1);
      };
    }
    const entry: DismissEntry = { close: () => onDismissRef.current() };
    stack.push(entry);
    scheduleSync();
    return () => {
      const index = stack.indexOf(entry);
      if (index !== -1) {
        stack.splice(index, 1);
      }
      scheduleSync();
    };
  }, [isolated, eventTarget]);
}
