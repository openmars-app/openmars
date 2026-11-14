import { useCardPlayFlowStore } from "@/stores/cardPlayFlowStore";
import React, {
  useState,
  useEffect,
  useLayoutEffect,
  useRef,
  useCallback,
  useMemo,
  useImperativeHandle,
  forwardRef,
} from "react";
import { flushSync } from "react-dom";
import { useStore } from "zustand";
import type { CardInspectionDrag, CardInspectionStore } from "@/hooks/useCardInspection.ts";
import {
  createCardInspectionFlight,
  type CardInspectionFlight,
} from "@/utils/cardInspectionFlight.ts";
import GameCard from "../cards/GameCard.tsx";
import CardStatusMessages from "../display/CardStatusMessages.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import BlurredOverlay from "./BlurredOverlay.tsx";
import { PlayerCardDto } from "@/types/generated/api-types.ts";
import { useSoundEffects } from "@/hooks/useSoundEffects.ts";
import { useCardDragStore } from "@/stores/cardDragStore.ts";

function clamp(n: number, min: number, max: number): number {
  return Math.min(Math.max(n, min), max);
}

// --- Fan layout constants ---
const SPACING = 120;
const MAX_PANEL_WIDTH = 640;
const FAN_PADDING = 40;
const BASE_Y_OFFSET = 160;
const VISIBLE_RADIUS = 3;
const CULL_RADIUS = VISIBLE_RADIUS + 1;

const SELECTED_LIFT = -180;
const SELECTED_SCALE = 1.12;

const PLAY_ZONE_BOTTOM_RATIO = 0.7;
const DRAG_THRESHOLD = 14;
const DRAG_RAISE_THRESHOLD = -60;
const WHEEL_SCALE = 0.005;

function isInCardPlayZone(y: number, viewportHeight: number): boolean {
  return y < viewportHeight * PLAY_ZONE_BOTTOM_RATIO;
}

interface CardTransform {
  x: number;
  y: number;
  rotation: number;
  scale: number;
  z: number;
}

function getCardTransform(i: number, scrollPos: number): CardTransform {
  return { x: (i - scrollPos) * SPACING, y: BASE_Y_OFFSET, rotation: 0, scale: 1, z: i };
}

export interface CardFanOverlayHandle {
  startInspectionDrag: (cardId: string, drag: CardInspectionDrag, detail: HTMLElement) => void;
  toggleExpand: () => void;
  collapse: () => void;
  readonly isExpanded: boolean;
}

interface CardFanOverlayProps {
  cards: PlayerCardDto[];
  hideWhenModalOpen?: boolean;
  inspectionStore: CardInspectionStore;
  onInspectCard: (cardId: string, source: HTMLElement) => void;
  onDismissInspection: () => void;
  onPlayCard?: (cardId: string) => Promise<void>;
}

const CardFanOverlay = forwardRef<CardFanOverlayHandle, CardFanOverlayProps>(
  (
    {
      cards: handCards,
      hideWhenModalOpen = false,
      inspectionStore,
      onInspectCard,
      onDismissInspection,
      onPlayCard,
    },
    ref,
  ) => {
    const playSession = useCardPlayFlowStore((state) => state.playSession);
    const cards = useMemo(() => {
      if (playSession && !handCards.some((card) => card.id === playSession.card.id)) {
        return [...handCards, playSession.card];
      }
      return handCards;
    }, [handCards, playSession?.card]);
    const inspections = useStore(inspectionStore, (state) => state.inspections);
    const [scrollPos, setScrollPos] = useState(0);
    const [cardOrder, setCardOrder] = useState<string[]>([]);
    const [highlightedCard, setHighlightedCard] = useState<string | null>(null);
    const [draggedCard, setDraggedCard] = useState<string | null>(null);
    const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
    const [dragScale, setDragScale] = useState(1);
    const [dragPosition, setDragPosition] = useState({ x: 0, y: 0 });
    const [dragStartPosition, setDragStartPosition] = useState({ x: 0, y: 0 });
    const [isInThrowZone, setIsInThrowZone] = useState(false);
    const [isInspectionDragFlying, setIsInspectionDragFlying] = useState(false);
    const [returningCard, setReturningCard] = useState<string | null>(null);
    const [isExpanded, setIsExpanded] = useState(false);
    const [isTransitioning, setIsTransitioning] = useState(false);

    const inspectionDragFlight = useRef<CardInspectionFlight | null>(null);
    const handRef = useRef<HTMLDivElement>(null);
    const sizingRef = useRef<HTMLDivElement>(null);
    const cardsRef = useRef(cards);
    const dragIntentRef = useRef(false);
    const capturedCardRef = useRef<HTMLDivElement | null>(null);
    const capturedPointerIdRef = useRef<number | null>(null);
    const lastPointerXRef = useRef(0);

    const { playCardHoverSound } = useSoundEffects();
    const startCardDrag = useCardDragStore((s) => s.startCardDrag);
    const endCardDrag = useCardDragStore((s) => s.endCardDrag);

    const [cardSize, setCardSize] = useState({ width: 0, height: 0, expandedWidth: 0 });
    const expandedSpacing = cardSize.expandedWidth + 24;
    const hasCards = cards.length > 0;

    useLayoutEffect(() => {
      const sizing = sizingRef.current;
      const expandedSizing = sizing?.firstElementChild as HTMLElement | null;
      if (!sizing || !expandedSizing) {
        return;
      }
      const measure = () =>
        setCardSize((previous) => {
          const next = {
            width: sizing.offsetWidth,
            height: sizing.offsetHeight,
            expandedWidth: expandedSizing.offsetWidth,
          };
          return previous.width === next.width &&
            previous.height === next.height &&
            previous.expandedWidth === next.expandedWidth
            ? previous
            : next;
        });
      const observer = new ResizeObserver(measure);
      observer.observe(sizing);
      observer.observe(expandedSizing);
      measure();
      return () => observer.disconnect();
    }, [hasCards, hideWhenModalOpen]);

    const [windowWidth, setWindowWidth] = useState(window.innerWidth);
    const [windowHeight, setWindowHeight] = useState(window.innerHeight);
    const expandedScale = Math.min(1.15, Math.max(0.7, windowHeight / 1320));
    const expandedWidth = `min(calc((var(--card-inspection-width) + 12px) * ${expandedScale}), calc(100vw - 48px))`;

    useEffect(() => {
      const handleResize = () => {
        setWindowWidth(window.innerWidth);
        setWindowHeight(window.innerHeight);
      };
      window.addEventListener("resize", handleResize);
      return () => window.removeEventListener("resize", handleResize);
    }, []);

    const panelWidth = Math.min(MAX_PANEL_WIDTH, Math.max(480, (windowWidth - 700) / 2));
    const fanScale = useMemo(() => {
      const availableWidth = windowWidth - 2 * panelWidth - FAN_PADDING;
      const maxVisibleWidth = 2 * VISIBLE_RADIUS * SPACING + cardSize.width;
      if (maxVisibleWidth <= availableWidth) return 1;
      return Math.min(1, Math.max(0.5, (availableWidth / maxVisibleWidth) * 1.2));
    }, [windowWidth, panelWidth, cardSize.width]);

    const expandedBaseY = -windowHeight / 2 + 48;

    const expandedCullRadius = useMemo(() => {
      return Math.ceil(windowWidth / expandedSpacing / 2) + 2;
    }, [windowWidth, expandedSpacing]);

    useLayoutEffect(() => {
      if (!isExpanded || hideWhenModalOpen || !handRef.current) {
        return;
      }
      const hand = handRef.current;
      const requirements = hand.querySelectorAll<HTMLElement>(".game-card-requirements");
      const heights = new Map<HTMLElement, number>();
      const align = (element: HTMLElement, height: number) => {
        heights.set(element, height);
        element
          .closest<HTMLElement>(".card-fan-card")
          ?.style.setProperty("--expanded-requirements-height", `${height}px`);
        hand.style.setProperty(
          "--expanded-max-requirements-height",
          `${Math.max(0, ...heights.values())}px`,
        );
      };
      const observer = new ResizeObserver((entries) => {
        for (const entry of entries) {
          align(
            entry.target as HTMLElement,
            entry.borderBoxSize[0]?.blockSize ?? entry.contentRect.height,
          );
        }
      });
      for (const element of requirements) {
        align(element, parseFloat(getComputedStyle(element).height));
        observer.observe(element);
      }
      return () => {
        observer.disconnect();
        hand.style.removeProperty("--expanded-max-requirements-height");
        for (const element of requirements) {
          element
            .closest<HTMLElement>(".card-fan-card")
            ?.style.removeProperty("--expanded-requirements-height");
        }
      };
    }, [
      isExpanded,
      hideWhenModalOpen,
      cards,
      cardOrder,
      expandedCullRadius,
      Math.floor(scrollPos),
    ]);

    // --- Expand / Collapse ---
    const handleExpand = useCallback(() => {
      onDismissInspection();
      setIsTransitioning(true);
      setIsExpanded(true);
      setHighlightedCard(null);
      setTimeout(() => setIsTransitioning(false), 350);
    }, [onDismissInspection]);

    const scrollTargetRef = useRef(0);
    const scrollAnimRef = useRef(0);
    const isAnimatingRef = useRef(false);

    const handleCollapse = useCallback(
      (cardId?: string) => {
        setIsTransitioning(true);
        setIsExpanded(false);
        if (cardId) {
          const cardIndex = cardOrder.indexOf(cardId);
          if (cardIndex >= 0) {
            scrollTargetRef.current = cardIndex;
            setScrollPos(cardIndex);
            setHighlightedCard(cardId);
          }
        }
        setTimeout(() => setIsTransitioning(false), 350);
      },
      [cardOrder],
    );

    const startInspectionDrag = useCallback(
      (cardId: string, drag: CardInspectionDrag, detail: HTMLElement | null) => {
        const source = handRef.current?.querySelector<HTMLDivElement>(
          `[data-card-id="${CSS.escape(cardId)}"]`,
        );
        const body = source?.querySelector<HTMLElement>(":scope > .game-card .game-card-body");
        const bounds = handRef.current?.getBoundingClientRect();
        if (!source || !body || !bounds) {
          return;
        }
        inspectionDragFlight.current?.dispose();
        source.setPointerCapture(drag.pointerId);
        capturedCardRef.current = source;
        capturedPointerIdRef.current = drag.pointerId;
        dragIntentRef.current = true;
        lastPointerXRef.current = drag.clientX;
        const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        const start = { x: drag.clientX, y: bounds.bottom };
        flushSync(() => {
          setIsInspectionDragFlying(!reducedMotion && detail !== null);
          setHighlightedCard(null);
          setDraggedCard(cardId);
          setDragScale(fanScale);
          setDragPosition({ x: drag.clientX, y: drag.clientY });
          setDragStartPosition(start);
          setDragOffset({
            x: (0.5 - drag.grabX) * body.offsetWidth * fanScale,
            y: (1 - drag.grabY) * body.offsetHeight * fanScale,
          });
          setIsInThrowZone(isInCardPlayZone(drag.clientY, windowHeight));
        });
        if (!reducedMotion && detail) {
          inspectionDragFlight.current = createCardInspectionFlight(source, detail, () => {
            inspectionDragFlight.current?.dispose();
            inspectionDragFlight.current = null;
            setIsInspectionDragFlying(false);
          });
          inspectionDragFlight.current?.play(true, 1.8);
          if (!inspectionDragFlight.current) {
            setIsInspectionDragFlying(false);
          }
        }
        startCardDrag();
        onDismissInspection();
      },
      [fanScale, onDismissInspection, startCardDrag, windowHeight],
    );

    useImperativeHandle(
      ref,
      () => ({
        startInspectionDrag,
        toggleExpand: () => {
          if (isExpanded) {
            handleCollapse();
          } else {
            handleExpand();
          }
        },
        collapse: () => {
          if (isExpanded) {
            handleCollapse();
          }
        },
        get isExpanded() {
          return isExpanded;
        },
      }),
      [isExpanded, handleCollapse, handleExpand, startInspectionDrag],
    );

    useEffect(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === "Escape" && isExpanded) {
          e.stopImmediatePropagation();
          handleCollapse();
        }
      };
      document.addEventListener("keydown", handleKeyDown);
      return () => document.removeEventListener("keydown", handleKeyDown);
    }, [isExpanded, handleCollapse]);

    useEffect(() => {
      cardsRef.current = cards;
    }, [cards]);

    // Sync cardOrder when cards prop changes (additions/removals)
    useEffect(() => {
      const propIds = new Set(cards.map((c) => c.id));
      const kept = cardOrder.filter((id) => propIds.has(id));
      const keptSet = new Set(kept);
      const added = cards.filter((c) => !keptSet.has(c.id)).map((c) => c.id);
      const next = [...kept, ...added];
      if (next.length === cardOrder.length && next.every((id, index) => id === cardOrder[index])) {
        return;
      }
      setCardOrder(next);
      // Center scroll on first load without scheduling state from a state updater.
      if (cardOrder.length === 0 && next.length > 0) {
        const center = (next.length - 1) / 2;
        setScrollPos(center);
        scrollTargetRef.current = center;
      }
    }, [cards, cardOrder]);

    // Clamp scrollPos when cards change
    useEffect(() => {
      const maxScroll = Math.max(cardOrder.length - 1, 0);
      setScrollPos((prev) => {
        const clamped = clamp(prev, 0, maxScroll);
        scrollTargetRef.current = clamped;
        return clamped;
      });
      // Clear selection if card disappeared
      if (highlightedCard && !cardOrder.includes(highlightedCard)) {
        setHighlightedCard(null);
      }
    }, [cardOrder, highlightedCard]);

    // --- Wheel scrolling ---
    const animateScroll = useCallback(() => {
      setScrollPos((prev) => {
        const diff = scrollTargetRef.current - prev;
        if (Math.abs(diff) < 0.01) {
          isAnimatingRef.current = false;
          return scrollTargetRef.current;
        }
        scrollAnimRef.current = requestAnimationFrame(animateScroll);
        return prev + diff * 0.25;
      });
    }, []);

    const startScrollAnimation = useCallback(() => {
      if (!isAnimatingRef.current) {
        isAnimatingRef.current = true;
        scrollAnimRef.current = requestAnimationFrame(animateScroll);
      }
    }, [animateScroll]);

    useEffect(() => {
      return () => cancelAnimationFrame(scrollAnimRef.current);
    }, []);

    const handleWheel = useCallback(
      (e: WheelEvent) => {
        if (draggedCard && !isExpanded) {
          if (e.ctrlKey) {
            return;
          }
          const behaviors = capturedCardRef.current?.querySelector<HTMLElement>(
            ".game-card-behaviors > .behavior-section",
          );
          if (behaviors && behaviors.scrollHeight > behaviors.clientHeight && e.deltaY !== 0) {
            e.preventDefault();
            e.stopPropagation();
            let delta = e.deltaY;
            if (e.deltaMode === WheelEvent.DOM_DELTA_LINE) {
              delta *= parseFloat(getComputedStyle(behaviors).lineHeight) || 16;
            } else if (e.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
              delta *= behaviors.clientHeight;
            }
            behaviors.scrollBy({ top: delta, behavior: "instant" });
            return;
          }
        }
        const scrollable = (e.target as Element).closest<HTMLElement>(".card-fan-expanded-content");
        if (
          isExpanded &&
          scrollable &&
          Math.abs(e.deltaY) > Math.abs(e.deltaX) &&
          scrollable.scrollHeight > scrollable.clientHeight
        ) {
          return;
        }
        // When dragging, only allow scroll if cursor is near the card fan
        if (draggedCard) {
          const containerRect = handRef.current?.getBoundingClientRect();
          if (!containerRect || e.clientY < containerRect.bottom - 200) return;
        }
        e.preventDefault();
        if (highlightedCard) {
          setHighlightedCard(null);
        }
        const delta = e.deltaY || e.deltaX;
        const maxScroll = Math.max(cardOrder.length - 1, 0);
        scrollTargetRef.current = clamp(
          scrollTargetRef.current + delta * WHEEL_SCALE,
          0,
          maxScroll,
        );
        startScrollAnimation();
      },
      [isExpanded, draggedCard, highlightedCard, cardOrder.length, startScrollAnimation],
    );

    useEffect(() => {
      if (isExpanded || draggedCard) {
        const capture = draggedCard !== null;
        document.addEventListener("wheel", handleWheel, { passive: false, capture });
        return () => document.removeEventListener("wheel", handleWheel, capture);
      } else {
        const el = handRef.current;
        if (!el) return;
        el.addEventListener("wheel", handleWheel, { passive: false });
        return () => el.removeEventListener("wheel", handleWheel);
      }
    }, [handleWheel, isExpanded, draggedCard]);

    useEffect(() => {
      if (!isExpanded) return;
      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
          e.preventDefault();
          if (highlightedCard) {
            setHighlightedCard(null);
          }
          const maxScroll = Math.max(cardOrder.length - 1, 0);
          const step = e.key === "ArrowLeft" ? -1 : 1;
          scrollTargetRef.current = clamp(scrollTargetRef.current + step, 0, maxScroll);
          startScrollAnimation();
        }
      };
      document.addEventListener("keydown", handleKeyDown);
      return () => document.removeEventListener("keydown", handleKeyDown);
    }, [isExpanded, highlightedCard, cardOrder.length, startScrollAnimation]);

    // --- Pointer events for drag ---
    const handlePointerDown = (cardId: string, e: React.PointerEvent<HTMLDivElement>) => {
      if (e.button !== 0 || useCardPlayFlowStore.getState().playSession) {
        return;
      }
      e.preventDefault();
      const cardEl = e.currentTarget;
      cardEl.setPointerCapture(e.pointerId);
      capturedCardRef.current = cardEl;
      capturedPointerIdRef.current = e.pointerId;

      dragIntentRef.current = false;

      const cardRect = cardEl.getBoundingClientRect();
      setDragOffset({
        x: cardRect.left + cardRect.width / 2 - e.clientX,
        y: cardRect.bottom - e.clientY,
      });
      setDragScale(cardRect.width / cardEl.offsetWidth);

      setDraggedCard(cardId);
      setDragPosition({ x: e.clientX, y: e.clientY });
      setDragStartPosition({ x: e.clientX, y: e.clientY });
      setIsInThrowZone(false);
    };

    const tryReorder = useCallback(
      (pointerX: number, pointerY: number) => {
        if (!draggedCard || !dragIntentRef.current) return;
        const containerRect = handRef.current?.getBoundingClientRect();
        if (!containerRect) return;
        const cursorNearFan = pointerY >= containerRect.bottom - 80;
        if (!cursorNearFan) return;
        const relativeX = pointerX - (containerRect.left + containerRect.width / 2);
        const targetSlot = clamp(
          Math.round(relativeX / (SPACING * fanScale) + scrollPos),
          0,
          cardOrder.length - 1,
        );
        setCardOrder((prev) => {
          const currentIdx = prev.indexOf(draggedCard);
          if (currentIdx === -1 || currentIdx === targetSlot) return prev;
          const next = [...prev];
          next.splice(currentIdx, 1);
          next.splice(targetSlot, 0, draggedCard);
          return next;
        });
      },
      [draggedCard, scrollPos, cardOrder.length, fanScale],
    );

    // Re-run reorder when scrollPos changes during drag
    useEffect(() => {
      if (draggedCard && dragIntentRef.current) {
        tryReorder(lastPointerXRef.current, dragPosition.y);
      }
    }, [scrollPos, draggedCard, tryReorder, dragPosition.y]);

    const handlePointerMove = useCallback(
      (e: React.PointerEvent<HTMLDivElement>) => {
        if (!draggedCard) return;

        const deltaX = e.clientX - dragStartPosition.x;
        const deltaY = e.clientY - dragStartPosition.y;
        const movedDist = Math.sqrt(deltaX * deltaX + deltaY * deltaY);

        if (!dragIntentRef.current && movedDist > DRAG_THRESHOLD) {
          if (isExpanded) {
            const body = e.currentTarget.querySelector<HTMLElement>(".game-card-body");
            if (!body) {
              return;
            }
            const rect = body.getBoundingClientRect();
            const drag = {
              pointerId: e.pointerId,
              clientX: e.clientX,
              clientY: e.clientY,
              grabX: clamp((dragStartPosition.x - rect.x) / rect.width, 0, 1),
              grabY: clamp((dragStartPosition.y - rect.y) / rect.height, 0, 1),
            };
            flushSync(() => handleCollapse(draggedCard));
            startInspectionDrag(draggedCard, drag, null);
            return;
          }
          dragIntentRef.current = true;
          onDismissInspection();
          startCardDrag();
        }

        if (!dragIntentRef.current) return;

        lastPointerXRef.current = e.clientX;
        setDragPosition({ x: e.clientX, y: e.clientY });

        tryReorder(e.clientX, e.clientY);

        setIsInThrowZone(isInCardPlayZone(e.clientY, windowHeight));
      },
      [
        draggedCard,
        dragStartPosition,
        isExpanded,
        handleCollapse,
        startInspectionDrag,
        tryReorder,
        startCardDrag,
        onDismissInspection,
        windowHeight,
      ],
    );

    const handlePointerUp = useCallback(
      async (e: React.PointerEvent<HTMLDivElement>) => {
        const cardId = draggedCard;
        if (!cardId) return;
        inspectionDragFlight.current?.dispose();
        inspectionDragFlight.current = null;
        setIsInspectionDragFlying(false);

        if (capturedCardRef.current) {
          const captured = capturedCardRef.current;
          capturedCardRef.current = null;
          capturedPointerIdRef.current = null;
          captured.releasePointerCapture(e.pointerId);
        }

        const wasDrag = dragIntentRef.current;
        dragIntentRef.current = false;

        endCardDrag();

        if (!wasDrag) {
          // This was a click, not a drag
          setDraggedCard(null);
          setIsInThrowZone(false);

          if (isExpanded) {
            handleCollapse(cardId);
          } else {
            onInspectCard(cardId, e.currentTarget);
            setHighlightedCard(null);
          }

          void playCardHoverSound();

          return;
        }

        // Drag ended — check throw
        const isThrow = isInCardPlayZone(e.clientY, windowHeight);

        if (isThrow && onPlayCard) {
          const cardData = cardsRef.current.find((c) => c.id === cardId);
          if (cardData?.available) {
            const flow = useCardPlayFlowStore.getState();
            if (!flow.beginPlay(cardData, e.currentTarget)) {
              setDraggedCard(null);
              setIsInThrowZone(false);
              return;
            }
            const sessionId = useCardPlayFlowStore.getState().playSession!.id;
            setDraggedCard(null);
            setIsInThrowZone(false);
            setHighlightedCard(null);
            try {
              await onPlayCard(cardId);
            } catch (error) {
              useCardPlayFlowStore.getState().returnPlay(sessionId, error);
            }
            return;
          }
        }

        // Return card to hand with animation
        setReturningCard(cardId);
        setDraggedCard(null);
        setIsInThrowZone(false);

        setTimeout(() => {
          setReturningCard(null);
        }, 400);
      },
      [
        draggedCard,
        dragOffset,
        dragScale,
        fanScale,
        highlightedCard,
        onPlayCard,
        onInspectCard,
        isExpanded,
        handleCollapse,
        playCardHoverSound,
        endCardDrag,
        windowHeight,
      ],
    );

    useEffect(() => {
      return () => {
        inspectionDragFlight.current?.dispose();
        endCardDrag();
      };
    }, [endCardDrag]);

    const cancelGesture = useCallback(() => {
      inspectionDragFlight.current?.dispose();
      inspectionDragFlight.current = null;
      dragIntentRef.current = false;
      const captured = capturedCardRef.current;
      const pointerId = capturedPointerIdRef.current;
      capturedCardRef.current = null;
      capturedPointerIdRef.current = null;
      if (pointerId !== null && captured?.hasPointerCapture(pointerId)) {
        captured.releasePointerCapture(pointerId);
      }
      endCardDrag();
      setDraggedCard(null);
      setIsInThrowZone(false);
      setIsInspectionDragFlying(false);
    }, [endCardDrag]);

    useEffect(() => {
      if (hideWhenModalOpen || (draggedCard && !cards.some((card) => card.id === draggedCard))) {
        cancelGesture();
      }
    }, [hideWhenModalOpen, draggedCard, cards, cancelGesture]);

    useEffect(() => {
      const cancelPointer = (event: PointerEvent) => {
        if (event.pointerId === capturedPointerIdRef.current) {
          cancelGesture();
        }
      };
      document.addEventListener("pointercancel", cancelPointer, true);
      return () => document.removeEventListener("pointercancel", cancelPointer, true);
    }, [cancelGesture]);

    const isDraggingCard = draggedCard !== null && dragIntentRef.current;
    const armedCardId =
      !isExpanded &&
      isDraggingCard &&
      !isInspectionDragFlying &&
      isInThrowZone &&
      onPlayCard &&
      cards.some((card) => card.id === draggedCard && card.available === true)
        ? draggedCard
        : null;
    useEffect(() => {
      if (!isDraggingCard || hideWhenModalOpen || isExpanded) {
        return;
      }
      document.documentElement.dataset.cardDragging = "true";
      return () => {
        delete document.documentElement.dataset.cardDragging;
      };
    }, [isDraggingCard, hideWhenModalOpen, isExpanded]);

    // --- Click outside to deselect ---
    useEffect(() => {
      const handleDocumentClick = (event: MouseEvent) => {
        if (handRef.current && !handRef.current.contains(event.target as Node)) {
          setHighlightedCard(null);
        }
      };
      document.addEventListener("click", handleDocumentClick);
      return () => document.removeEventListener("click", handleDocumentClick);
    }, []);

    if ((hideWhenModalOpen && !playSession) || cards.length === 0) {
      return null;
    }

    const activeCullRadius = isExpanded ? expandedCullRadius : CULL_RADIUS;
    const activeVisibleRadius = isExpanded ? expandedCullRadius : VISIBLE_RADIUS;

    const showRightScrollHint = !isExpanded && cardOrder.length - 1 - scrollPos > VISIBLE_RADIUS;
    const showLeftScrollHint = !isExpanded && scrollPos > VISIBLE_RADIUS;

    const scrollHintEdgeX = (VISIBLE_RADIUS * SPACING + cardSize.width / 2) * fanScale;
    const scrollHintOffsetY = (BASE_Y_OFFSET - cardSize.height / 2) * fanScale;

    return (
      <>
        <BlurredOverlay
          visible={isExpanded}
          onClose={() => handleCollapse()}
          zIndex={Z_INDEX.EXPANDED_CARD_FAN - 1}
        >
          <div />
        </BlurredOverlay>

        <div
          className="card-fan-overlay"
          ref={handRef}
          style={{ zIndex: isExpanded ? Z_INDEX.EXPANDED_CARD_FAN : Z_INDEX.CARDS_HAND_OVERLAY }}
        >
          <div
            ref={sizingRef}
            aria-hidden
            style={{
              position: "absolute",
              visibility: "hidden",
              pointerEvents: "none",
              width: "var(--card-width)",
              height: "var(--card-height)",
            }}
          >
            <div
              style={{
                width: expandedWidth,
              }}
            />
          </div>
          {cards.map((card) => {
            const index = cardOrder.indexOf(card.id);
            if (index === -1) return null;

            const isInspected = inspections.some((inspection) => inspection.cardId === card.id);
            const isDraggedCard = draggedCard === card.id;
            const isPlayCard = playSession?.card.id === card.id;
            const isRaisedPlayCard = isPlayCard && playSession.phase !== "returning";

            const absD = Math.abs(index - scrollPos);
            if (absD > activeCullRadius && !isDraggedCard && !isInspected && !isPlayCard)
              return null;

            const edgeOpacity = isExpanded || isDraggedCard || absD <= activeVisibleRadius ? 1 : 0;
            const isDragging = isDraggedCard && dragIntentRef.current;
            const isArmed = armedCardId === card.id;
            const isReturning = returningCard === card.id;
            const isHighlighted = highlightedCard === card.id;
            const enteringInspection =
              isPlayCard &&
              playSession.inspected &&
              !playSession.inspectionReady &&
              playSession.phase === "choosing";
            const isHidden =
              (isPlayCard && !enteringInspection) || (hideWhenModalOpen && !isPlayCard);
            const isInteractive = edgeOpacity !== 0 && !isHidden && !isInspected && !playSession;

            let finalX: number;
            let finalY: number;
            let finalRotation: number;
            let finalScale: number;
            let finalZ: number;
            let showErrors = false;

            if (isExpanded) {
              const d = index - scrollPos;
              finalX = d * expandedSpacing;
              finalY = expandedBaseY;
              finalRotation = 0;
              finalScale = 1;
              finalZ = index;
            } else {
              const base = getCardTransform(index, scrollPos);
              finalX = base.x * fanScale;
              finalY = base.y * fanScale;
              finalRotation = base.rotation;
              finalScale = fanScale;
              finalZ = base.z;

              if (isHighlighted && !isDragging && !isInspected) {
                finalY += SELECTED_LIFT * fanScale;
                finalScale = SELECTED_SCALE * fanScale;
                finalRotation = 0;
                finalZ = Z_INDEX.CARD_FAN_HIGHLIGHTED;
              }

              let isDragRaised = false;
              if (isDragging && !isReturning) {
                const containerRect = handRef.current?.getBoundingClientRect();
                if (containerRect) {
                  finalX =
                    dragPosition.x + dragOffset.x - (containerRect.left + containerRect.width / 2);
                  finalY = dragPosition.y + dragOffset.y - containerRect.bottom;
                  finalRotation = 0;
                  finalScale = dragScale;
                  finalZ = Z_INDEX.CARD_FAN_DRAGGED;

                  const dragDeltaY = dragPosition.y - dragStartPosition.y;
                  isDragRaised = dragIntentRef.current && dragDeltaY < DRAG_RAISE_THRESHOLD;
                }
              }

              const hasErrors = !card.available && card.errors.length > 0;
              const hasWarnings = (card.warnings?.length ?? 0) > 0;
              showErrors =
                (hasErrors || hasWarnings) && (isHighlighted || (isDraggedCard && isDragRaised));
            }

            const staggerDelay = isTransitioning
              ? `${Math.abs(index - Math.round(scrollPos)) * 25}ms`
              : undefined;

            const classNames = [
              "card-fan-card card-size",
              isTransitioning && !isDragging ? "is-mode-transition" : "",
              !isExpanded && isDragging && !isReturning ? "is-dragging" : "",
              !isExpanded && !isDraggedCard && dragIntentRef.current ? "is-reordering" : "",
              !isExpanded && isReturning ? "is-returning" : "",
              isExpanded ? "is-expanded" : "",
            ]
              .filter(Boolean)
              .join(" ");

            return (
              <div
                key={card.id}
                data-card-id={card.id}
                data-interactive={isInteractive}
                role="button"
                tabIndex={isInteractive ? 0 : -1}
                aria-label={`Inspect ${card.name}`}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && !playSession) {
                    event.preventDefault();
                    if (isExpanded) {
                      handleCollapse(card.id);
                    } else {
                      onInspectCard(card.id, event.currentTarget);
                    }
                  }
                }}
                className={classNames}
                style={{
                  width: isExpanded ? expandedWidth : undefined,
                  translate: isExpanded
                    ? `-50% calc(50% - var(--expanded-max-requirements-height, 0px) * ${expandedScale} / 2)`
                    : undefined,
                  transform:
                    isPlayCard && !playSession.inspectionReady && playSession.phase !== "returning"
                      ? playSession.sourceTransform
                      : `translate(${finalX}px, ${finalY}px) rotate(${finalRotation}deg) scale(${finalScale})`,
                  zIndex: isRaisedPlayCard ? Z_INDEX.CARD_FAN_DRAGGED : finalZ,
                  opacity: isRaisedPlayCard ? 1 : edgeOpacity,
                  visibility: isHidden ? "hidden" : undefined,
                  transitionDelay: staggerDelay,
                }}
                onPointerDown={(e) => handlePointerDown(card.id, e)}
                onPointerMove={handlePointerMove}
                onPointerUp={handlePointerUp}
                onPointerCancel={cancelGesture}
                onLostPointerCapture={() => {
                  if (capturedCardRef.current) {
                    cancelGesture();
                  }
                }}
              >
                {isExpanded ? (
                  <div
                    className="card-fan-expanded-content"
                    style={{
                      zoom: expandedScale,
                      maxHeight: `calc((100dvh - 96px) / ${expandedScale})`,
                    }}
                  >
                    <GameCard card={card} presentation="inspection" />
                  </div>
                ) : (
                  <GameCard
                    card={card}
                    moduleState={isArmed ? "armed" : "idle"}
                    isSelected={
                      !isExpanded &&
                      (isHighlighted || (isDraggedCard && isInThrowZone && card.available === true))
                    }
                  />
                )}
                {!isExpanded &&
                  (!card.available || (card.warnings && card.warnings.length > 0)) && (
                    <div className={`card-fan-error-panel ${showErrors ? "is-visible" : ""}`}>
                      <CardStatusMessages card={card} />
                    </div>
                  )}
              </div>
            );
          })}

          {showLeftScrollHint && (
            <div
              className="card-fan-scroll-hint card-fan-scroll-hint--left"
              style={{
                transform: `translate(calc(-50% + ${-scrollHintEdgeX}px), ${scrollHintOffsetY}px)`,
                zIndex: Z_INDEX.CARD_FAN_SCROLL_HINT,
              }}
            >
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polyline points="15 18 9 12 15 6" />
              </svg>
            </div>
          )}

          {showRightScrollHint && (
            <div
              className="card-fan-scroll-hint card-fan-scroll-hint--right"
              style={{
                transform: `translate(calc(-50% + ${scrollHintEdgeX}px), ${scrollHintOffsetY}px)`,
                zIndex: Z_INDEX.CARD_FAN_SCROLL_HINT,
              }}
            >
              <svg
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polyline points="9 18 15 12 9 6" />
              </svg>
            </div>
          )}

          <style>{`
        .card-fan-overlay {
          position: fixed;
          bottom: 48px;
          left: 50%;
          transform: translateX(-50%);
          width: 0;
          height: 300px;

          pointer-events: none;
        }

        .card-fan-card {
          position: absolute;
          max-width: none;
          bottom: 0;
          left: 50%;
          translate: -50% 0;
          cursor: pointer;
          transform-origin: bottom center;
          pointer-events: none;
          user-select: none;
          touch-action: none;
          transition: transform 180ms ease, filter 180ms ease, opacity 180ms ease;
        }

        .card-fan-card:focus-visible { outline: 2px solid #60a5fa; }

        .card-fan-card[data-interactive="true"] .game-card-body,
        .card-fan-card[data-interactive="true"] .game-card-requirements > div > div {
          pointer-events: auto;
        }

        .card-fan-card.is-mode-transition {
          transition: transform 280ms cubic-bezier(0.25, 0.46, 0.45, 0.94),
                      opacity 210ms ease !important;
        }

        .card-fan-card.is-expanded {
          cursor: pointer;
          touch-action: pan-y;
        }

        .card-fan-expanded-content > .game-card {
          padding-top: calc(
            var(--expanded-max-requirements-height, 0px) -
              var(--expanded-requirements-height, 0px)
          );
        }

        .card-fan-expanded-content {
          overflow-y: auto;
          overscroll-behavior: contain;
          scrollbar-width: thin;
          padding: 0 6px 4px;
        }

        .card-fan-card.is-dragging {
          transition: none;
          cursor: grabbing;
        }

        .card-fan-card.is-reordering {
          transition: transform 200ms ease, opacity 180ms ease;
        }

        .card-fan-card.is-returning {
          transition: transform 400ms cubic-bezier(0.25, 0.46, 0.45, 0.94),
                      filter 180ms ease,
                      opacity 180ms ease;
        }

        .card-fan-scroll-hint {
          position: absolute;
          bottom: 0;
          left: 50%;
          display: flex;
          align-items: center;
          justify-content: center;
          cursor: default;
          pointer-events: none;
        }

        .card-fan-scroll-hint svg {
          width: 40px;
          height: 40px;
          color: rgba(255, 255, 255, 0.85);
          filter: drop-shadow(0 2px 6px rgba(0, 0, 0, 0.6));
        }

        .card-fan-error-panel {
          position: absolute;
          left: 100%;
          /* Compact card bodies have a fixed height below the requirement area. */
          top: calc(100% - var(--card-height));
          margin-left: 10px;
          width: 180px;
          display: flex;
          flex-direction: column;
          gap: 4px;
          pointer-events: none;
          opacity: 0;
          transform: translateX(-6px);
          transition: opacity 200ms ease, transform 200ms ease;
        }

        .card-fan-error-panel.is-visible {
          opacity: 1;
          transform: translateX(0);
        }

        @media (max-width: 1200px) {
          .card-fan-overlay {
            height: 250px;
          }
        }

        @media (max-width: 768px) {
          .card-fan-overlay {
            height: 200px;
          }
        }
      `}</style>
        </div>
      </>
    );
  },
);

export default CardFanOverlay;
