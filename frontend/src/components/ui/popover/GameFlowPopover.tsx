import {
  useOverlayPortalRoot,
  useOverlayEventTarget,
  useIsolatedOverlay,
  isPreviewEvent,
} from "@/contexts/OverlayEnvironment";
import { useCardPlayFlowStore } from "@/stores/cardPlayFlowStore";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Z_INDEX } from "@/constants/zIndex";
import { useLayoutMode } from "@/hooks/useLayoutMode";

type GameFlowType = "immediate" | "interactive" | "interactive-mandatory";

const COMPACT_PROMPT_PADDING =
  "calc(var(--safe-top) + 8px) calc(var(--safe-right) + 16px) calc(var(--safe-bottom) + 8px) calc(var(--safe-left) + 16px)";

interface GameFlowPopoverProps {
  isVisible: boolean;
  onClose?: () => void;
  type?: GameFlowType;
  className?: string;
  outerClassName?: string;
  renderSiblings?: React.ReactNode;
  handleEscapeKey?: boolean;
  compactFill?: boolean;
  children: React.ReactNode;
}

export function GameFlowPopover({
  isVisible,
  onClose,
  type = "interactive",
  className = "",
  outerClassName = "",
  renderSiblings,
  handleEscapeKey = true,
  compactFill = false,
  children,
}: GameFlowPopoverProps) {
  const portalRoot = useOverlayPortalRoot();
  const eventTarget = useOverlayEventTarget();
  const isolated = useIsolatedOverlay();
  const { isCompact } = useLayoutMode();
  const host = useCardPlayFlowStore((state) => {
    if (!isolated && (isCompact || state.playSession?.phase === "choosing")) {
      return state.playPromptHost;
    }
    return null;
  });
  const popoverRef = useRef<HTMLDivElement>(null);
  const [isClosing, setIsClosing] = useState(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (closeTimer.current) clearTimeout(closeTimer.current);
    },
    [],
  );

  const isDismissible = type === "interactive" || (type === "immediate" && !!onClose);

  useEffect(() => {
    if (isVisible) {
      setIsClosing(false);
    }
  }, [eventTarget, isolated, isVisible]);

  const requestClose = useCallback(() => {
    if (!onClose) {
      return;
    }
    setIsClosing(true);
    if (closeTimer.current) clearTimeout(closeTimer.current);
    closeTimer.current = setTimeout(() => {
      setIsClosing(false);
      onClose();
    }, 200);
  }, [onClose]);

  useEffect(() => {
    const preventScroll = (event: WheelEvent | TouchEvent) => {
      if (eventTarget === document && isPreviewEvent(event)) return;
      if (
        (event.target instanceof Element &&
          event.target.closest("[data-card-play-stage], [data-overlay-layer]")) ||
        (popoverRef.current && popoverRef.current.contains(event.target as Node))
      ) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
    };

    const previousOverflow = document.body.style.overflow;
    if (isVisible) {
      if (!isolated) document.body.style.overflow = "hidden";
      eventTarget.addEventListener("wheel", preventScroll, { passive: false });
      eventTarget.addEventListener("touchmove", preventScroll, { passive: false });
    }

    return () => {
      if (!isolated) document.body.style.overflow = previousOverflow;
      eventTarget.removeEventListener("wheel", preventScroll);
      eventTarget.removeEventListener("touchmove", preventScroll);
    };
  }, [eventTarget, isolated, isVisible]);

  useEffect(() => {
    if (!isVisible || !isDismissible) {
      return;
    }

    const handleEscape = (event: KeyboardEvent) => {
      if (isolated && event.target instanceof Node && !popoverRef.current?.contains(event.target))
        return;
      if (event.defaultPrevented || (eventTarget === document && isPreviewEvent(event))) return;
      if (event.key === "Escape") {
        event.preventDefault();
        requestClose();
      }
    };

    const handlePointerOutside = (event: PointerEvent) => {
      if (eventTarget === document && isPreviewEvent(event)) return;
      if ((event.target as HTMLElement).closest?.("[data-overlay-layer]")) {
        return;
      }
      if (
        event.button === 0 &&
        popoverRef.current &&
        !popoverRef.current.contains(event.target as Node)
      ) {
        requestClose();
      }
    };

    if (type !== "immediate") {
      if (handleEscapeKey) {
        eventTarget.addEventListener("keydown", handleEscape);
      }
      eventTarget.addEventListener("pointerdown", handlePointerOutside);
    }

    return () => {
      eventTarget.removeEventListener("keydown", handleEscape);
      eventTarget.removeEventListener("pointerdown", handlePointerOutside);
    };
  }, [eventTarget, isVisible, isDismissible, type, requestClose, handleEscapeKey]);

  if (!isVisible) {
    return null;
  }

  const animationClass = isClosing ? "animate-fadeOut" : "animate-popIn";
  let compactContentClass = "";
  let compactPanelClass = "";
  if (isCompact && compactFill) {
    compactContentClass = "w-full h-full flex flex-col";
    compactPanelClass = "w-full! min-w-0! flex-1 min-h-0 max-h-none!";
  } else if (isCompact) {
    compactContentClass = host
      ? "w-full max-h-full flex flex-col"
      : "w-full max-w-[560px] max-h-full flex flex-col";
    compactPanelClass = "w-full! min-w-0! min-h-0 max-h-none!";
  }

  const content = (
    <div
      ref={popoverRef}
      className={`relative pointer-events-auto ${animationClass} ${compactContentClass}`}
      style={host ? { maxWidth: "100%" } : undefined}
    >
      <div
        className={`min-w-[240px] w-fit max-w-[90vw] max-h-[min(500px,100dvh_-_32px)] game-panel game-panel-clipped game-window flex flex-col overflow-hidden pointer-events-auto ${className} ${host ? "max-w-full! min-w-0!" : ""} ${compactPanelClass}`}
      >
        {children}
      </div>
      {renderSiblings}
    </div>
  );

  const rendered = (
    <>
      {type === "immediate" && (
        <div
          className="fixed inset-0 transition-opacity duration-[245ms] ease-in-out"
          style={{
            background: "rgba(0, 0, 0, 0.4)",
            backdropFilter: "blur(6px)",
            zIndex: Z_INDEX.IMMEDIATE_BACKDROP,
            opacity: isClosing ? 0 : 1,
            pointerEvents: isClosing ? "none" : "auto",
          }}
          onClick={onClose ? requestClose : undefined}
        />
      )}
      {host ? (
        createPortal(content, host)
      ) : (
        <div
          className={`
          fixed top-0 left-0 right-0 bottom-0
          flex items-center justify-center
          pointer-events-none
          ${outerClassName}
        `}
          style={{
            zIndex:
              type === "immediate" ? Z_INDEX.IMMEDIATE_POPOVER : Z_INDEX.SELECTION_POPOVER + 1,
            padding: isCompact ? COMPACT_PROMPT_PADDING : undefined,
          }}
        >
          {isCompact ? content : <div className="pointer-events-auto">{content}</div>}
        </div>
      )}

      <style>{`
        @keyframes popIn {
          from {
            opacity: 0;
          }
          to {
            opacity: 1;
          }
        }

        @keyframes fadeOut {
          from {
            opacity: 1;
          }
          to {
            opacity: 0;
          }
        }

        @keyframes choiceSlideIn {
          from {
            opacity: 0;
          }
          to {
            opacity: 1;
          }
        }

        .animate-popIn {
          animation: popIn 0.25s ease-out;
        }

        .animate-fadeOut {
          animation: fadeOut 0.2s ease-out forwards;
        }

        .animate-choiceSlideIn {
          animation: choiceSlideIn 0.3s ease-out both;
        }

        @media (max-width: 768px) {
          .min-w-\\[240px\\] {
            min-width: 180px;
          }
          .max-w-\\[90vw\\] {
            max-width: 95vw;
          }
        }
      `}</style>
    </>
  );

  if (type === "immediate") {
    return createPortal(rendered, portalRoot);
  }

  return rendered;
}

interface GameFlowTitleProps {
  children: React.ReactNode;
  className?: string;
}

export function GameFlowTitle({ children, className = "" }: GameFlowTitleProps) {
  return (
    <div
      className={`shrink-0 py-[15px] px-5 bg-black/40 border-b border-b-white/15 select-none ${className}`}
    >
      {children}
    </div>
  );
}

interface GameFlowBodyProps {
  children: React.ReactNode;
  className?: string;
}

export function GameFlowBody({ children, className = "" }: GameFlowBodyProps) {
  return (
    <div
      className={`min-h-0 flex-1 overflow-y-auto overscroll-contain p-2.5 scrollbar-thin scrollbar-thumb-white/20 scrollbar-track-white/5 ${className}`}
    >
      {children}
    </div>
  );
}

interface GameFlowFooterProps {
  children: React.ReactNode;
  className?: string;
}

export function GameFlowFooter({ children, className = "" }: GameFlowFooterProps) {
  return (
    <div
      className={`shrink-0 px-4 py-3 compact:py-2 bg-black/40 border-t border-white/15 flex justify-center compact:[&_.game-button]:min-h-11 ${className}`}
    >
      {children}
    </div>
  );
}
