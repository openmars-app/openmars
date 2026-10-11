import CloseButton from "@/components/ui/buttons/CloseButton.tsx";
import React, { useRef, useState, useLayoutEffect } from "react";
import { createPortal } from "react-dom";
import { GamePopoverProps, PopoverPosition } from "./types";
import { getThemeStyles } from "./themes";
import { usePopover } from "./usePopover";
import { Z_INDEX } from "@/constants/zIndex";

const VIEWPORT_MARGIN = 8;
const ANCHOR_GAP = 15;
const ANCHOR_EDGE_PADDING = 30;

interface PopoverLayout {
  top?: number;
  bottom?: number;
  left?: number;
  width: number;
  maxHeight?: number;
}

function resolveMaxHeight(maxHeight: number | string): number {
  if (typeof maxHeight === "number") {
    return maxHeight;
  }
  if (maxHeight.endsWith("vh")) {
    return (parseFloat(maxHeight) / 100) * window.innerHeight;
  }
  return Infinity;
}

function clampLeft(left: number, width: number): number {
  const maxLeft = window.innerWidth - width - VIEWPORT_MARGIN;
  return Math.min(Math.max(left, VIEWPORT_MARGIN), maxLeft);
}

function fitHeight(requested: number, availableSpace: number): number {
  return Math.max(0, Math.min(requested, availableSpace - VIEWPORT_MARGIN));
}

function computeLayout(
  position: PopoverPosition,
  requestedWidth: number,
  requestedMaxHeight: number | string,
): PopoverLayout {
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;
  const width = Math.min(requestedWidth, viewportWidth - 2 * VIEWPORT_MARGIN);
  const maxHeight = resolveMaxHeight(requestedMaxHeight);

  if (position.type === "anchor") {
    const anchor = position.anchorRef.current;
    if (!anchor) {
      return { width, maxHeight: fitHeight(maxHeight, viewportHeight) };
    }
    const rect = anchor.getBoundingClientRect();
    const rightAlignedLeft =
      viewportWidth - Math.max(ANCHOR_EDGE_PADDING, viewportWidth - rect.right) - width;

    if (position.placement === "above") {
      const bottom = viewportHeight - rect.top + ANCHOR_GAP;
      return {
        bottom,
        left: clampLeft(
          position.align === "center" ? rect.left + (rect.width - width) / 2 : rightAlignedLeft,
          width,
        ),
        width,
        maxHeight: fitHeight(maxHeight, viewportHeight - bottom),
      };
    }

    const top = rect.bottom + ANCHOR_GAP;
    const wouldOverflowRight = rect.left + width > viewportWidth - ANCHOR_EDGE_PADDING;
    let left = wouldOverflowRight ? rightAlignedLeft : Math.max(ANCHOR_EDGE_PADDING, rect.left);
    if (position.align === "center") {
      left = rect.left + (rect.width - width) / 2;
    }
    return {
      top,
      left: clampLeft(left, width),
      width,
      maxHeight: fitHeight(maxHeight, viewportHeight - top),
    };
  }

  let left: number | undefined;
  if (position.left !== undefined) {
    left = clampLeft(position.left, width);
  } else if (position.right !== undefined) {
    left = clampLeft(viewportWidth - position.right - width, width);
  }

  let availableHeight = viewportHeight;
  if (position.top !== undefined) {
    availableHeight = viewportHeight - position.top;
  } else if (position.bottom !== undefined) {
    availableHeight = viewportHeight - position.bottom;
  }

  return {
    top: position.top,
    bottom: position.bottom,
    left,
    width,
    maxHeight: fitHeight(maxHeight, availableHeight),
  };
}

const GamePopover: React.FC<GamePopoverProps> = ({
  isVisible,
  onClose,
  position,
  theme,
  header,
  arrow,
  width = 320,
  maxHeight = 400,
  zIndex = Z_INDEX.POPOVER,
  animation = "slideUp",
  children,
  className = "",
  excludeRef,
  contentRef,
  overlayLayer,
}) => {
  const popoverRef = useRef<HTMLDivElement>(null);
  const [layout, setLayout] = useState<PopoverLayout | null>(null);

  const anchorRef = position.type === "anchor" ? position.anchorRef : excludeRef;

  usePopover({
    isVisible,
    onClose,
    popoverRef,
    anchorRef,
  });

  useLayoutEffect(() => {
    if (!isVisible) {
      return;
    }

    const update = () => {
      setLayout(computeLayout(position, width, maxHeight));
    };

    update();
    window.addEventListener("resize", update);
    window.addEventListener("orientationchange", update);
    return () => {
      window.removeEventListener("resize", update);
      window.removeEventListener("orientationchange", update);
    };
  }, [isVisible, position, width, maxHeight]);

  if (!isVisible) {
    return null;
  }

  const resolvedLayout = layout ?? { width };

  const themeStyles = getThemeStyles(theme);
  const animationClass =
    animation === "slideUp"
      ? "animate-[popoverSlideUp_0.3s_ease-out]"
      : "animate-[popoverSlideDown_0.3s_ease-out]";

  const getArrowPosition = () => {
    if (!arrow?.enabled) return "";
    const offset = arrow.offset ?? 30;
    switch (arrow.position) {
      case "left":
        return `left-[${offset}px]`;
      case "center":
        return "left-1/2 -translate-x-1/2";
      case "right":
      default:
        return `right-[${offset}px]`;
    }
  };

  return createPortal(
    <div
      ref={popoverRef}
      className={`fixed game-panel game-panel-clipped ${animationClass} flex flex-col overflow-hidden isolate pointer-events-auto ${className}`}
      style={{
        ...themeStyles,
        top: resolvedLayout.top,
        bottom: resolvedLayout.bottom,
        left: resolvedLayout.left,
        width: resolvedLayout.width,
        maxHeight: resolvedLayout.maxHeight,
        zIndex,
      }}
      {...(overlayLayer ? { "data-overlay-layer": true } : {})}
    >
      {arrow?.enabled && (
        <div
          className={`absolute -bottom-2 ${getArrowPosition()} w-0 h-0 border-l-[8px] border-l-transparent border-r-[8px] border-r-transparent border-t-[8px] border-t-[var(--popover-accent)]`}
          style={arrow.offset !== undefined ? { right: `${arrow.offset}px` } : undefined}
        />
      )}

      {header && (
        <div className="flex items-center justify-between py-[15px] px-5 bg-black/40 border-b border-b-[var(--popover-accent)]/60">
          <div className="flex items-center gap-2.5">
            <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
              {header.title}
            </h3>
            {header.badge && (
              <div className="text-white/80 text-xs bg-[rgba(var(--popover-accent-rgb),0.2)] py-1 px-2 rounded-md border border-[rgba(var(--popover-accent-rgb),0.3)]">
                {header.badge}
              </div>
            )}
          </div>
          {header.centerContent}
          <div className="flex items-center gap-2">
            {header.rightContent}
            {header.showCloseButton && <CloseButton onClick={onClose} />}
          </div>
        </div>
      )}

      <div
        ref={contentRef}
        className="flex-1 overflow-y-auto [scrollbar-width:thin] [scrollbar-color:var(--popover-accent)_rgba(30,60,150,0.3)] [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:bg-[rgba(30,60,150,0.3)] [&::-webkit-scrollbar-track]:rounded [&::-webkit-scrollbar-thumb]:bg-[var(--popover-accent)]/70 [&::-webkit-scrollbar-thumb]:rounded [&::-webkit-scrollbar-thumb:hover]:bg-[var(--popover-accent)]"
      >
        {children}
      </div>
    </div>,
    document.body,
  );
};

export default GamePopover;
