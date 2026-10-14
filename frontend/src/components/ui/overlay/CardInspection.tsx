import { isPreviewEvent } from "@/contexts/OverlayEnvironment";
import { useCardPlayFlowStore, type CardPlaySession } from "@/stores/cardPlayFlowStore";
import { Profiler, useEffect, useLayoutEffect, useRef, useState } from "react";
import { coldStartTrace } from "@/services/performanceStore.ts";
import { createPortal, flushSync } from "react-dom";
import type { PlayerCardDto } from "@/types/generated/api-types.ts";
import { useReducedMotion } from "@/hooks/useReducedMotion.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import type { CardInspectionSession, CardInspectionDrag } from "@/hooks/useCardInspection.ts";
import {
  createCardInspectionFlight,
  type CardInspectionFlight,
} from "@/utils/cardInspectionFlight.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import GameCard from "../cards/GameCard.tsx";
import CardDescription from "../cards/CardDescription.tsx";
import CardStatusMessages from "../display/CardStatusMessages.tsx";

interface CardInspectionProps {
  card: PlayerCardDto;
  playSession?: CardPlaySession;
  sheet?: boolean;
  inspection: CardInspectionSession;
  onReturned: (inspection: CardInspectionSession) => void;
  chatBounds: DOMRectReadOnly | null;
  onClose: (restoreFocus?: boolean) => void;
  onDragStart: (cardId: string, drag: CardInspectionDrag, detail: HTMLElement) => void;
}

export default function CardInspection(props: CardInspectionProps) {
  if (coldStartTrace.enabled) {
    return (
      <Profiler id="card-inspection" onRender={coldStartTrace.reactRender}>
        <CardInspectionContent {...props} />
      </Profiler>
    );
  }
  return <CardInspectionContent {...props} />;
}

function CardInspectionContent({
  card,
  playSession,
  sheet: forcedSheet,
  inspection,
  onReturned,
  chatBounds,
  onClose,
  onDragStart,
}: CardInspectionProps) {
  const gestureRef = useRef<{ x: number; y: number; pointerId: number } | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const motionRef = useRef<HTMLDivElement>(null);
  const motionStarted = useRef(false);
  const flightRef = useRef<CardInspectionFlight | null>(null);
  const latest = useRef({ inspection, onReturned, playSession });
  latest.current = { inspection, onReturned, playSession };
  const releaseRef = useRef<Animation | null>(null);
  const releasingRef = useRef(false);
  const reducedMotion = useReducedMotion();
  const { isCompact } = useLayoutMode();
  const [motionReady, setMotionReady] = useState(false);
  const [viewport, setViewport] = useState(() => ({
    width: window.innerWidth,
    height: window.innerHeight,
  }));
  const [panelHeight, setPanelHeight] = useState(0);
  const [contentLayout, setContentLayout] = useState({
    descriptionHeight: 200,
    scrollBody: false,
    requirementsHeight: 0,
  });
  const [contentScrollTop, setContentScrollTop] = useState(0);
  const showStatus =
    !playSession && ((!card.available && card.errors.length > 0) || !!card.warnings?.length);
  const desktopScale = Math.min(1.15, Math.max(0.7, viewport.height / 1320));
  const statusWidth = 240;
  const right = Math.max(
    96,
    viewport.width * 0.12,
    showStatus ? (statusWidth + 10) * desktopScale + 12 : 0,
  );
  const topInset = 96;
  const panelWidth = 372 * desktopScale;
  const panelLeft = viewport.width - right - panelWidth;
  const chatOverlaps =
    chatBounds && chatBounds.right > panelLeft && chatBounds.left < viewport.width - right;
  const desktopBottom = chatOverlaps
    ? Math.min(chatBounds.top - 16, viewport.height - 106)
    : viewport.height - 106;
  const desktopHeight = desktopBottom - topInset;
  const sheet = forcedSheet ?? (viewport.width < 1024 || desktopHeight < 540 * desktopScale);
  const scale = sheet ? 1 : desktopScale;
  const top = Math.max(
    topInset,
    Math.min((viewport.height - panelHeight) / 2, desktopBottom - panelHeight),
  );
  const sheetRatio = playSession ? 0.4 : 0.7;
  const maxHeight = sheet
    ? Math.max(100, Math.min(viewport.height * sheetRatio, viewport.height - 122))
    : desktopHeight;
  const contentMaxHeight = maxHeight / scale;
  const sheetBottom = isCompact
    ? "calc(var(--hud-dock-h) + var(--safe-bottom) + 12px)"
    : "calc(90px + var(--safe-bottom) + 12px)";

  useLayoutEffect(() => {
    const panel = panelRef.current;
    if (!coldStartTrace.active || !panel) {
      return;
    }
    const describe = (image: HTMLImageElement) => ({
      url: image.currentSrc || image.src,
      complete: image.complete,
      naturalWidth: image.naturalWidth,
      naturalHeight: image.naturalHeight,
    });
    coldStartTrace.mark("card:images-at-commit", {
      source: Array.from(inspection.source.querySelectorAll("img"), describe),
      inspection: Array.from(panel.querySelectorAll("img"), describe),
    });
    const loaded = (event: Event) => {
      if (event.target instanceof HTMLImageElement) {
        coldStartTrace.mark("card:image-loaded", describe(event.target));
      }
    };
    panel.addEventListener("load", loaded, true);
    return () => panel.removeEventListener("load", loaded, true);
  }, [inspection.source]);

  useEffect(() => {
    const resize = () => setViewport({ width: window.innerWidth, height: window.innerHeight });
    window.addEventListener("resize", resize);
    return () => window.removeEventListener("resize", resize);
  }, []);

  useLayoutEffect(() => {
    const panel = panelRef.current;
    if (!panel) {
      return;
    }
    const measure = () => {
      const end = coldStartTrace.span("card:measure-panel");
      setPanelHeight(panel.offsetHeight);
      end();
    };
    const observer = new ResizeObserver(measure);
    observer.observe(panel);
    measure();
    return () => observer.disconnect();
  }, []);

  useLayoutEffect(() => {
    const panel = panelRef.current;
    if (!panel) {
      return;
    }
    const sections = [
      ".game-card-requirements",
      ".game-card-heading",
      ".game-card-labels",
      ".game-card-behaviors",
    ]
      .map((selector) => panel.querySelector<HTMLElement>(selector))
      .filter((element): element is HTMLElement => element !== null);
    const measure = () => {
      const end = coldStartTrace.span("card:measure-sections");
      // Reserve body padding, description padding, and its optional panning control.
      const fixedHeight = sections.reduce((sum, element) => sum + element.offsetHeight, 0) + 64;
      const available = contentMaxHeight - fixedHeight;
      const body = panel.querySelector<HTMLElement>(".game-card-body");
      const minimumHeight = body ? parseFloat(getComputedStyle(body).minHeight) : 0;
      const requirementsHeight = sections[0]?.offsetHeight ?? 0;
      const next = {
        descriptionHeight: Math.min(240, Math.max(0, available)),
        scrollBody: available < 84 || minimumHeight + requirementsHeight + 4 > contentMaxHeight,
        requirementsHeight,
      };
      setContentLayout((previous) =>
        previous.descriptionHeight === next.descriptionHeight &&
        previous.scrollBody === next.scrollBody &&
        previous.requirementsHeight === next.requirementsHeight
          ? previous
          : next,
      );
      end();
    };
    const observer = new ResizeObserver(measure);
    sections.forEach((element) => observer.observe(element));
    measure();
    return () => observer.disconnect();
  }, [contentMaxHeight, card.id]);

  useLayoutEffect(() => {
    let frame = 0;
    const run = () => {
      coldStartTrace.mark("card:flight-callback", { closing: inspection.closing, reducedMotion });
      const detail = motionRef.current?.querySelector<HTMLElement>(".game-card");
      if ((releasingRef.current && !inspection.closing) || !detail) {
        return;
      }
      if (reducedMotion || (inspection.closing && !motionStarted.current)) {
        flightRef.current?.dispose();
        flightRef.current = null;
        setMotionReady(true);
        if (inspection.closing) {
          latest.current.onReturned(latest.current.inspection);
        }
        return;
      }
      if (
        (latest.current.playSession?.phase === "submitting" ||
          latest.current.playSession?.phase === "reconnecting") &&
        !flightRef.current
      ) {
        setMotionReady(true);
        return;
      }
      motionStarted.current = true;
      if (!inspection.closing && !latest.current.playSession) {
        panelRef.current?.focus({ preventScroll: true });
      }
      if (!flightRef.current) {
        flightRef.current = createCardInspectionFlight(inspection.source, detail, (closing) => {
          coldStartTrace.mark("card:flight-finished", { closing });
          const finished = flightRef.current;
          flightRef.current = null;
          flushSync(() => {
            if (closing) {
              latest.current.onReturned(latest.current.inspection);
            } else {
              setMotionReady(true);
            }
          });
          finished?.dispose();
        });
      }
      if (flightRef.current) {
        setMotionReady(false);
        if (inspection.closing) {
          flightRef.current.fadeBack();
        } else {
          flightRef.current.play(false);
        }
      } else if (inspection.closing) {
        latest.current.onReturned(latest.current.inspection);
      } else {
        setMotionReady(true);
      }
    };
    if (inspection.closing || flightRef.current) {
      run();
    } else {
      coldStartTrace.mark("card:layout-committed-awaiting-two-frames", { cardId: card.id });
      frame = requestAnimationFrame(() => {
        coldStartTrace.mark("card:first-animation-frame");
        frame = requestAnimationFrame(run);
      });
    }
    return () => cancelAnimationFrame(frame);
  }, [inspection.closing, inspection.source, reducedMotion]);

  useLayoutEffect(
    () => () => {
      flightRef.current?.dispose();
      if (releaseRef.current) {
        releaseRef.current.onfinish = null;
        releaseRef.current.cancel();
      }
    },
    [],
  );

  useLayoutEffect(() => {
    if (
      !playSession ||
      (playSession.phase !== "submitting" && playSession.phase !== "reconnecting") ||
      playSession.animationDone ||
      releasingRef.current
    ) {
      return;
    }
    const finish = () => useCardPlayFlowStore.getState().finishPlayAnimation(playSession.id);
    if (reducedMotion) {
      finish();
      return;
    }
    if (flightRef.current) {
      releasingRef.current = true;
      flightRef.current.release(finish);
    } else if (motionReady && motionRef.current) {
      releasingRef.current = true;
      const animation = motionRef.current.animate(
        [
          { transform: "scale(1)", opacity: 1 },
          { transform: "scale(0.85)", opacity: 0 },
        ],
        { duration: 350, easing: "ease-out", fill: "forwards" },
      );
      animation.onfinish = finish;
      releaseRef.current = animation;
    }
  }, [playSession?.id, playSession?.phase, playSession?.animationDone, motionReady, reducedMotion]);

  useLayoutEffect(() => {
    if (playSession?.phase !== "returning") {
      return;
    }
    releasingRef.current = false;
    if (releaseRef.current) {
      releaseRef.current.onfinish = null;
      releaseRef.current.cancel();
      releaseRef.current = null;
    }
  }, [playSession?.phase]);

  useLayoutEffect(() => {
    if (!motionReady) {
      return;
    }
    const session = latest.current.playSession;
    if (session) {
      useCardPlayFlowStore.getState().inspectionArrived(session.id);
    }
    const source = inspection.source;
    source.dataset.cardInspectionReady = "true";
    return () => {
      delete source.dataset.cardInspectionReady;
    };
  }, [motionReady, inspection.source]);

  useEffect(() => {
    if (inspection.closing || playSession) {
      return;
    }
    const dismissOutside = (event: PointerEvent) => {
      const target = event.target;
      if (
        !(target instanceof Element) ||
        panelRef.current?.contains(target) ||
        target.closest(".card-fan-overlay")
      ) {
        return;
      }
      onClose();
    };
    const dismissEscape = (event: KeyboardEvent) => {
      if (isPreviewEvent(event)) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopImmediatePropagation();
        onClose(true);
      }
    };
    document.addEventListener("pointerdown", dismissOutside);
    document.addEventListener("keydown", dismissEscape, true);
    return () => {
      document.removeEventListener("pointerdown", dismissOutside);
      document.removeEventListener("keydown", dismissEscape, true);
    };
  }, [onClose, inspection.closing, !!playSession]);

  const content = (
    <div
      ref={panelRef}
      role="region"
      aria-label={`Inspect ${card.name}`}
      className="outline-none cursor-default select-none shrink-0"
      tabIndex={-1}
      onDragStart={(event) => event.preventDefault()}
      onPointerDown={(event) => {
        if (
          !!playSession ||
          isCompact ||
          event.button !== 0 ||
          inspection.closing ||
          !motionReady ||
          (event.target as Element).closest("button, a, input, select, textarea")
        ) {
          return;
        }
        event.preventDefault();
        gestureRef.current = { x: event.clientX, y: event.clientY, pointerId: event.pointerId };
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={(event) => {
        const gesture = gestureRef.current;
        if (
          !gesture ||
          gesture.pointerId !== event.pointerId ||
          Math.hypot(event.clientX - gesture.x, event.clientY - gesture.y) < 14
        ) {
          return;
        }
        const detail = motionRef.current?.querySelector<HTMLElement>(".game-card");
        const body = detail?.querySelector<HTMLElement>(".game-card-body");
        if (!detail || !body) {
          return;
        }
        const rect = body.getBoundingClientRect();
        gestureRef.current = null;
        onDragStart(
          card.id,
          {
            pointerId: event.pointerId,
            clientX: event.clientX,
            clientY: event.clientY,
            grabX: Math.max(0, Math.min(1, (gesture.x - rect.x) / rect.width)),
            grabY: Math.max(0, Math.min(1, (gesture.y - rect.y) / rect.height)),
          },
          detail,
        );
      }}
      onPointerUp={() => {
        gestureRef.current = null;
      }}
      onPointerCancel={() => {
        gestureRef.current = null;
      }}
      onLostPointerCapture={() => {
        gestureRef.current = null;
      }}
      data-card-inspection
      data-overlay-layer={playSession ? true : undefined}
      data-motion={playSession?.phase ?? (inspection.closing ? "returning" : "inspecting")}
      data-placement={sheet ? "sheet" : "side"}
      style={{
        position: playSession && sheet ? "relative" : "fixed",
        zIndex: playSession ? Z_INDEX.SELECTION_POPOVER : Z_INDEX.CARDS_PREVIEW,
        pointerEvents: inspection.closing || playSession?.phase === "submitting" ? "none" : "auto",
        width: sheet ? "min(420px, calc(100vw - 24px))" : panelWidth,
        maxHeight,
        right: sheet ? undefined : right,
        top: sheet ? undefined : top,
        left: sheet && !playSession ? "50%" : undefined,
        bottom: sheet && !playSession ? sheetBottom : undefined,
        transform: sheet && !playSession ? "translateX(-50%)" : undefined,
      }}
    >
      <div
        ref={motionRef}
        className="relative"
        style={{
          visibility: motionReady ? "visible" : "hidden",
          zoom: scale,
          width: sheet ? undefined : 372,
        }}
      >
        <div
          className={
            contentLayout.scrollBody
              ? "overflow-y-auto overscroll-contain [scrollbar-width:thin] px-1.5 pb-1"
              : "px-1.5 pb-1"
          }
          style={{ maxHeight: contentMaxHeight }}
          onScroll={(event) => setContentScrollTop(event.currentTarget.scrollTop)}
        >
          <GameCard
            card={card}
            presentation="inspection"
            dimUnavailable={!playSession}
            description={
              <CardDescription
                key={card.id}
                sections={card.description}
                maxHeight={contentLayout.descriptionHeight}
                autoPan={!contentLayout.scrollBody}
              />
            }
          />
          {showStatus && sheet && (
            <div className="mt-2">
              <CardStatusMessages card={card} size="large" />
            </div>
          )}
        </div>
        {showStatus && !sheet && (
          <div
            className="absolute left-full ml-1 pointer-events-none"
            style={{
              top: Math.max(0, contentLayout.requirementsHeight - contentScrollTop),
              width: statusWidth,
            }}
          >
            <CardStatusMessages card={card} size="large" />
          </div>
        )}
      </div>
    </div>
  );
  return playSession ? content : createPortal(content, document.body);
}
