import RotateDeviceOverlay from "../../overlay/RotateDeviceOverlay";
import {
  Component,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { apiService } from "@/services/apiService";
import type { CardDto } from "@/types/generated/api-types";
import { OverlayEnvironment } from "@/contexts/OverlayEnvironment";
import { Z_INDEX } from "@/constants/zIndex";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { useRenderPause } from "@/stores/renderPauseStore";
import FloatingWindow from "../FloatingWindow";
import { useWindowDrag, useWindowManager } from "../WindowManager";
import GameButton from "../../buttons/GameButton";
import CloseButton from "../../buttons/CloseButton";
import { previews } from "./registry";
import type { PreviewDefinition } from "./types";

class PreviewBoundary extends Component<{ children: ReactNode }, { error: string | null }> {
  state = { error: null as string | null };
  static getDerivedStateFromError(error: Error) {
    return { error: error.message };
  }
  render() {
    if (this.state.error) {
      return (
        <div role="alert" className="game-panel p-6 m-auto text-white">
          This preview could not render: {this.state.error}. Use Reset or Back to UI Debug.
        </div>
      );
    }
    return this.props.children;
  }
}

function PreviewControls({ onBack, onReset }: { onBack: () => void; onReset: () => void }) {
  const [collapsed, setCollapsed] = useState(false);
  const [position, setPosition] = useState<{ x: number; y: number } | null>(null);
  const nav = useRef<HTMLElement>(null);
  const drag = useRef<{ x: number; y: number; left: number; top: number } | null>(null);
  useEffect(() => {
    const clamp = () =>
      setPosition((current) => {
        if (!current || !nav.current) return current;
        const bounds = nav.current.getBoundingClientRect();
        return {
          x: Math.max(0, Math.min(window.innerWidth - bounds.width, current.x)),
          y: Math.max(
            0,
            Math.min(
              (window.visualViewport?.height ?? window.innerHeight) - bounds.height,
              current.y,
            ),
          ),
        };
      });
    window.addEventListener("resize", clamp);
    window.visualViewport?.addEventListener("resize", clamp);
    return () => {
      window.removeEventListener("resize", clamp);
      window.visualViewport?.removeEventListener("resize", clamp);
    };
  }, []);
  return (
    <nav
      ref={nav}
      aria-label="UI Debug preview controls"
      data-overlay-layer
      className="fixed left-3 top-3 compact:top-auto compact:bottom-2 flex items-center gap-1 bg-black/95 border border-white/30 p-1 shadow-lg"
      style={{
        zIndex: Z_INDEX.UI_DEBUG_CONTROLS,
        ...(position ? { left: position.x, top: position.y, bottom: "auto" } : {}),
      }}
    >
      {collapsed ? (
        <GameButton className="min-h-11" onClick={() => setCollapsed(false)}>
          UI Debug
        </GameButton>
      ) : (
        <>
          <GameButton aria-label="Back to UI Debug" className="min-h-11" onClick={onBack}>
            <span className="compact:hidden">Back to UI Debug</span>
            <span className="hidden compact:inline">Back</span>
          </GameButton>
          <GameButton className="min-h-11" onClick={onReset}>
            Reset
          </GameButton>
          <GameButton
            aria-label="Move preview controls"
            className="min-h-11 touch-none px-2"
            onPointerDown={(event) => {
              const bounds = nav.current!.getBoundingClientRect();
              drag.current = {
                x: event.clientX,
                y: event.clientY,
                left: bounds.left,
                top: bounds.top,
              };
              event.currentTarget.setPointerCapture(event.pointerId);
            }}
            onPointerMove={(event) => {
              if (!drag.current || !nav.current) return;
              const bounds = nav.current.getBoundingClientRect();
              setPosition({
                x: Math.max(
                  0,
                  Math.min(
                    window.innerWidth - bounds.width,
                    drag.current.left + event.clientX - drag.current.x,
                  ),
                ),
                y: Math.max(
                  0,
                  Math.min(
                    window.innerHeight - bounds.height,
                    drag.current.top + event.clientY - drag.current.y,
                  ),
                ),
              });
            }}
            onPointerUp={() => {
              drag.current = null;
            }}
            onPointerCancel={() => {
              drag.current = null;
            }}
          >
            Move
          </GameButton>
          <GameButton
            aria-label="Collapse preview controls"
            className="min-h-11 px-2"
            onClick={() => {
              setCollapsed(true);
              setPosition(null);
            }}
          >
            −
          </GameButton>
        </>
      )}
    </nav>
  );
}

function PreviewHost({
  definition,
  presetId,
  cards,
  onBack,
  onComplete,
}: {
  definition: PreviewDefinition;
  presetId: string;
  cards: CardDto[];
  onBack: () => void;
  onComplete: (message?: string) => void;
}) {
  const [root, setRoot] = useState<HTMLDivElement | null>(null);
  const [revision, setRevision] = useState(0);
  const onBackRef = useRef(onBack);
  onBackRef.current = onBack;
  useRenderPause("ui-debug-preview", true);
  useEffect(() => {
    const previousFocus = document.activeElement;
    const host = document.createElement("div");
    host.dataset.uiDebugPreview = definition.id;
    host.tabIndex = -1;
    host.className = "fixed inset-0 isolate flex text-white";
    host.style.zIndex = String(Z_INDEX.UI_DEBUG_PREVIEW);
    document.body.append(host);
    const inertBefore = new Map<HTMLElement, boolean>();
    const isolate = () => {
      for (const element of document.body.children) {
        if (element instanceof HTMLElement && element !== host && !inertBefore.has(element)) {
          inertBefore.set(element, element.inert);
          element.inert = true;
        }
      }
    };
    isolate();
    const observer = new MutationObserver(isolate);
    observer.observe(document.body, { childList: true });
    const stop = (event: Event) => event.stopPropagation();
    let dismissTimer: ReturnType<typeof setTimeout> | undefined;
    const keydown = (event: KeyboardEvent) => {
      event.stopPropagation();
      if (event.key === "Tab") {
        const focusable = Array.from(
          host.querySelectorAll<HTMLElement>(
            'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]',
          ),
        ).filter((element) => element.getClientRects().length > 0 && !element.closest("[inert]"));
        const first = focusable[0];
        const last = focusable.at(-1);
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
      // Nested dialogs get first refusal, even if mounted after this host listener.
      if (dismissTimer) clearTimeout(dismissTimer);
      dismissTimer = setTimeout(() => {
        if (event.key === "Escape" && !event.defaultPrevented) onBackRef.current();
      });
    };
    host.addEventListener("keydown", keydown);
    host.addEventListener("keyup", stop);
    host.addEventListener("wheel", stop);
    host.addEventListener("touchmove", stop);
    host.addEventListener("pointerdown", stop);
    host.addEventListener("pointerup", stop);
    setRoot(host);
    host.focus();
    return () => {
      if (dismissTimer) clearTimeout(dismissTimer);
      observer.disconnect();
      host.remove();
      for (const [element, inert] of inertBefore) element.inert = inert;
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) previousFocus.focus();
    };
  }, [definition.id]);
  if (!root) return null;
  const View = definition.Component;
  return createPortal(
    <OverlayEnvironment.Provider value={root}>
      <PreviewBoundary key={`${presetId}:${revision}`}>
        <View presetId={presetId} cards={cards} onComplete={onComplete} />
      </PreviewBoundary>
      <PreviewControls onBack={onBack} onReset={() => setRevision((value) => value + 1)} />
      <RotateDeviceOverlay />
    </OverlayEnvironment.Provider>,
    root,
  );
}

export default function UIDebugWindow({ onClose }: { onClose: () => void }) {
  const { isCompact } = useLayoutMode();
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  useEffect(() => {
    const focused = document.activeElement;
    return () => {
      if (focused instanceof HTMLElement && focused.isConnected) focused.focus();
    };
  }, []);
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("All");
  const [selectedId, setSelectedId] = useState(previews[0].id);
  const [presetId, setPresetId] = useState(previews[0].presets[0].id);
  const [active, setActive] = useState(false);
  const [cards, setCards] = useState<CardDto[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<string | null>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const catalogRef = useRef<HTMLDivElement>(null);
  const catalogScroll = useRef(0);
  useLayoutEffect(() => {
    if (!active && catalogRef.current) catalogRef.current.scrollTop = catalogScroll.current;
  }, [active]);
  const openRef = useRef<HTMLButtonElement>(null);
  const { position, handleMouseDown } = useWindowDrag({
    windowId: "ui-debug",
    width: 780,
    height: () => window.innerHeight * 0.8,
    isVisible: !active,
  });
  const { getZIndex } = useWindowManager();
  const selected = previews.find((item) => item.id === selectedId)!;
  useEffect(() => {
    let cancelled = false;
    setError(null);
    async function load() {
      const all: CardDto[] = [];
      let total = Infinity;
      while (all.length < total) {
        const response = await apiService.listCards(all.length, 200);
        if (cancelled) return;
        total = response.totalCount;
        if (!response.cards.length && all.length < total)
          throw new Error("Card catalog returned an incomplete page");
        all.push(...response.cards);
      }
      if (!cancelled) setCards(all);
    }
    void load().catch((reason: unknown) => {
      if (!cancelled)
        setError(reason instanceof Error ? reason.message : "Could not load sample cards");
    });
    return () => {
      cancelled = true;
    };
  }, [attempt]);
  useEffect(() => {
    if (active) return;
    searchRef.current?.focus({ preventScroll: true });
    const close = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !event.defaultPrevented) {
        event.preventDefault();
        onCloseRef.current();
      }
    };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, [active]);
  const back = useCallback(() => {
    setActive(false);
    requestAnimationFrame(() => openRef.current?.focus({ preventScroll: true }));
  }, []);
  const complete = useCallback(
    (message?: string) => {
      setResult(message ?? "Preview completed");
      back();
    },
    [back],
  );
  if (active && cards)
    return (
      <PreviewHost
        definition={selected}
        presetId={presetId}
        cards={cards}
        onBack={back}
        onComplete={complete}
      />
    );
  const filtered = previews.filter(
    (item) =>
      (category === "All" || item.category === category) &&
      `${item.name} ${item.category} ${item.keywords?.join(" ") ?? ""}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const content = (
    <>
      <div className="flex gap-3 mb-3 shrink-0">
        <input
          ref={searchRef}
          aria-label="Search UI previews"
          placeholder="Search dialogs and prompts…"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          className="min-w-0 flex-1 bg-black/50 border border-white/25 p-3 text-base"
        />
        <select
          aria-label="Preview category"
          value={category}
          onChange={(event) => setCategory(event.target.value)}
          className="bg-black border border-white/25 px-2 text-base max-w-[40%]"
        >
          {["All", ...new Set(previews.map((item) => item.category))].map((name) => (
            <option key={name}>{name}</option>
          ))}
        </select>
      </div>
      {result && (
        <p role="status" className="text-sm border-l-2 border-white/40 pl-3 mb-3">
          Simulated: {result}
        </p>
      )}
      <div className="flex gap-4 min-h-0 flex-1">
        <div
          ref={catalogRef}
          onScroll={(event) => {
            catalogScroll.current = event.currentTarget.scrollTop;
          }}
          aria-label="UI preview catalog"
          className="w-[46%] min-w-0 overflow-y-auto overscroll-contain border border-white/15"
        >
          {filtered.map((item) => (
            <button
              key={item.id}
              data-preview-id={item.id}
              aria-pressed={selectedId === item.id}
              onClick={() => {
                setSelectedId(item.id);
                setPresetId(item.presets[0].id);
                setResult(null);
              }}
              className={`block w-full text-left p-3 min-h-14 border-b border-white/10 cursor-pointer ${selectedId === item.id ? "bg-[#163854]" : "hover:bg-white/10"}`}
            >
              <span className="block font-orbitron text-xs">{item.name}</span>
              <span className="block text-xs text-white/50 mt-1">{item.category}</span>
            </button>
          ))}
          {!filtered.length && <p className="p-4 text-white/60">No matching previews.</p>}
        </div>
        <div className="flex-1 min-w-0 flex flex-col gap-3 overflow-y-auto">
          <h4 className="font-orbitron text-sm compact:text-xs">{selected.name}</h4>
          <label className="flex flex-col gap-2 text-sm">
            Sample scenario
            <select
              aria-label="Sample scenario"
              value={presetId}
              onChange={(event) => setPresetId(event.target.value)}
              className="w-full bg-black border border-white/25 p-3 text-base"
            >
              {selected.presets.map((preset) => (
                <option key={preset.id} value={preset.id}>
                  {preset.name}
                </option>
              ))}
            </select>
          </label>
          <p className="text-sm compact:text-xs text-white/60">
            Local sample data. Your game is unchanged.
          </p>
          {error ? (
            <div role="alert" className="text-sm">
              {error}
              <GameButton
                onClick={() => setAttempt((value) => value + 1)}
                className="mt-3 min-h-11"
              >
                Retry loading samples
              </GameButton>
            </div>
          ) : (
            <GameButton
              ref={openRef}
              emphasis="primary"
              disabled={!cards}
              className="self-start min-h-11"
              onClick={() => setActive(true)}
            >
              {cards ? "Open preview" : "Loading sample cards…"}
            </GameButton>
          )}
          <p className="text-xs text-white/40 compact:hidden">
            {previews.length} previews ·{" "}
            {previews.reduce((sum, item) => sum + item.presets.length, 0)} scenarios
          </p>
        </div>
      </div>
    </>
  );
  if (isCompact)
    return (
      <section
        role="dialog"
        aria-label="UI Debug"
        data-overlay-layer
        className="fixed inset-0 game-panel text-white flex flex-col"
        style={{
          zIndex: getZIndex("ui-debug"),
          padding: "var(--safe-top) var(--safe-right) var(--safe-bottom) var(--safe-left)",
        }}
      >
        <header className="shrink-0 flex justify-between items-center border-b border-white/20 pl-4">
          <h3 className="font-orbitron text-sm">UI Debug</h3>
          <CloseButton label="Close UI Debug" onClick={onClose} />
        </header>
        <div className="flex-1 min-h-0 flex flex-col p-3">{content}</div>
      </section>
    );
  return (
    <FloatingWindow
      title="UI Debug"
      subtitle="Dialogs and prompts · sample data"
      onClose={onClose}
      onMouseDown={handleMouseDown}
      className="[&>div]:flex-1"
      style={{
        top: position.y,
        left: position.x,
        width: 780,
        height: "80dvh",
        maxHeight: "calc(100dvh - 80px)",
        zIndex: getZIndex("ui-debug"),
      }}
    >
      {content}
    </FloatingWindow>
  );
}
