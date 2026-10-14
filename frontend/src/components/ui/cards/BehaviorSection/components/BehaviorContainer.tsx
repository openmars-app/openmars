import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import React, { createContext, useContext, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { FormattedDescription } from "../../../display/FormattedDescription.tsx";
import { ClassifiedBehavior } from "../types.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useTapReveal } from "@/hooks/useTapReveal.ts";

const BehaviorLayoutContext = createContext({
  compact: false,
  adjacentScoring: [] as string[],
  cardId: undefined as string | undefined,
});
export const useBehaviorLayout = () => useContext(BehaviorLayoutContext);

interface BehaviorContainerProps {
  classifiedBehavior: ClassifiedBehavior;
  index: number;
  description?: string;
  showTooltip?: boolean;
  noContainer?: boolean;
  borderLabel?: React.ReactNode;
  cardId?: string;
  adjacentScoring?: string[];
  children: React.ReactNode;
}

const DescriptionPortal: React.FC<{
  description: string;
  anchorRef: React.RefObject<HTMLDivElement | null>;
}> = ({ description, anchorRef }) => {
  const portalRoot = useOverlayPortalRoot();
  const tooltipRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number; aboveY: number } | null>(null);

  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    if (!anchor) {
      return;
    }
    const updatePosition = () => {
      const rect = anchor.getBoundingClientRect();
      let bottom = rect.bottom;
      let top = rect.top;
      for (let parent = anchor.parentElement; parent; parent = parent.parentElement) {
        if (getComputedStyle(parent).overflowY !== "visible") {
          const bounds = parent.getBoundingClientRect();
          bottom = Math.min(bottom, bounds.bottom);
          top = Math.max(top, bounds.top);
        }
      }
      setPos({ x: rect.left + rect.width / 2, y: bottom, aboveY: top });
    };
    updatePosition();
    const observer = new ResizeObserver(updatePosition);
    observer.observe(anchor);
    const section = anchor.closest(".behavior-section");
    if (section) {
      observer.observe(section);
    }
    window.addEventListener("resize", updatePosition);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updatePosition);
    };
  }, [anchorRef]);

  useLayoutEffect(() => {
    const tooltip = tooltipRef.current;
    if (!tooltip || !pos) {
      return;
    }
    // Match DecorBoxTooltip's measured flip and viewport clamp.
    const { width, height } = tooltip.getBoundingClientRect();
    const left = Math.max(12, Math.min(pos.x - width / 2, window.innerWidth - width - 12));
    const preferredTop = pos.y + height > window.innerHeight - 12 ? pos.aboveY - height : pos.y;
    const top = Math.max(12, Math.min(preferredTop, window.innerHeight - height - 12));
    tooltip.style.left = `${left}px`;
    tooltip.style.top = `${top}px`;
  }, [pos, description]);

  if (!pos) return null;

  return createPortal(
    <div
      ref={tooltipRef}
      role="tooltip"
      className="font-sans fixed w-[184px] max-w-[calc(100vw-24px)] pt-1 pointer-events-none animate-[fadeIn_150ms_ease-in]"
      style={{ left: pos.x, top: pos.y, zIndex: Z_INDEX.FLOATING_TOOLTIP }}
    >
      <div
        className="relative bg-[rgba(10,10,15,0.98)] border border-[rgba(60,60,70,0.7)] text-white/90 text-[11px] leading-tight px-3 py-2 shadow-[0_2px_8px_rgba(0,0,0,0.5)]"
        style={{
          clipPath:
            "polygon(0 0, calc(100% - 14px) 0, 100% 14px, 100% 100%, 14px 100%, 0 calc(100% - 14px))",
        }}
      >
        <FormattedDescription text={description} />
        <svg
          className="absolute top-0 right-0 w-[14px] h-[14px] pointer-events-none"
          viewBox="0 0 14 14"
        >
          <line x1="0" y1="0" x2="14" y2="14" stroke="rgba(60,60,70,0.7)" strokeWidth="1.5" />
        </svg>
        <svg
          className="absolute bottom-0 left-0 w-[14px] h-[14px] pointer-events-none"
          viewBox="0 0 14 14"
        >
          <line x1="0" y1="0" x2="14" y2="14" stroke="rgba(60,60,70,0.7)" strokeWidth="1.5" />
        </svg>
      </div>
    </div>,
    portalRoot,
  );
};

const BehaviorContainer: React.FC<BehaviorContainerProps> = ({
  classifiedBehavior,
  index,
  description,
  showTooltip = false,
  noContainer = false,
  borderLabel,
  cardId,
  adjacentScoring = [],
  children,
}) => {
  const { type: rawType } = classifiedBehavior;
  const type = noContainer ? ("auto-no-background" as const) : rawType;
  const {
    open: isRevealed,
    triggerRef: containerRef,
    triggerProps,
  } = useTapReveal<HTMLDivElement>({ enabled: showTooltip && !!description });
  const [compact, setCompact] = useState(false);

  useLayoutEffect(() => {
    const element = containerRef.current;
    const parent = element?.parentElement;
    if (!element || !parent) {
      return;
    }
    let previousWidth = parent.clientWidth;
    const measure = () => {
      const width = parent.clientWidth;
      if (width !== previousWidth) {
        previousWidth = width;
        if (compact) {
          setCompact(false);
          return;
        }
      }
      const bounds = element.getBoundingClientRect();
      const overflow = Array.from(element.querySelectorAll("img, [data-behavior-atom]")).some(
        (child) => {
          const rect = child.getBoundingClientRect();
          return rect.left < bounds.left - 1 || rect.right > bounds.right + 1;
        },
      );
      const overflowingRow = Array.from(element.querySelectorAll(".behavior-flow")).some(
        (row) => row.scrollWidth > row.clientWidth + 1,
      );
      if (element.scrollWidth > element.clientWidth + 1 || overflow || overflowingRow) {
        setCompact(true);
      }
    };
    const observer = new ResizeObserver(measure);
    observer.observe(parent);
    observer.observe(element);
    measure();
    return () => observer.disconnect();
  }, [children, compact]);

  const content = (
    <BehaviorLayoutContext.Provider value={{ compact, cardId, adjacentScoring }}>
      {children}
    </BehaviorLayoutContext.Provider>
  );

  if (type === "auto-no-background") {
    return (
      <div
        {...triggerProps}
        key={index}
        className={`behavior-fitting relative flex items-center justify-center p-0 max-w-full min-w-0 font-orbitron`}
        data-compact={compact}
      >
        {content}
        {isRevealed && description && (
          <DescriptionPortal description={description} anchorRef={containerRef} />
        )}
      </div>
    );
  } else {
    const typeStyles = {
      "manual-action":
        "bg-[linear-gradient(135deg,rgba(33,150,243,0.35)_0%,rgba(25,118,210,0.3)_100%)] border-[rgba(33,150,243,0.5)] shadow-[0_2px_4px_rgba(33,150,243,0.3)]",
      "triggered-effect": "bg-white/[0.08] border-white/20 shadow-[0_1px_3px_rgba(0,0,0,0.15)]",
      discount: "bg-white/[0.08] border-white/20 shadow-[0_1px_3px_rgba(0,0,0,0.15)]",
      "payment-substitute": "bg-white/[0.08] border-white/20 shadow-[0_1px_3px_rgba(0,0,0,0.15)]",
      "value-modifier": "bg-white/[0.08] border-white/20 shadow-[0_1px_3px_rgba(0,0,0,0.15)]",
      defense: "bg-white/[0.08] border-white/20 shadow-[0_1px_3px_rgba(0,0,0,0.15)]",
      "immediate-production":
        "bg-[linear-gradient(135deg,rgba(139,89,42,0.35)_0%,rgba(101,67,33,0.3)_100%)] border-[rgba(139,89,42,0.5)] shadow-[0_2px_4px_rgba(139,89,42,0.25)]",
      "immediate-effect": "bg-white/[0.08] border-white/20 shadow-[0_1px_3px_rgba(0,0,0,0.15)]",
    };

    const widthClass =
      type === "manual-action" ||
      type === "triggered-effect" ||
      type === "discount" ||
      type === "payment-substitute" ||
      type === "value-modifier" ||
      type === "defense"
        ? "w-fit"
        : "w-[calc(100%-20px)]";
    const { behavior } = classifiedBehavior;
    const compactOptionalExchange =
      type === "immediate-effect" &&
      behavior.optional &&
      behavior.inputs?.length === 1 &&
      behavior.outputs?.length === 1 &&
      behavior.outputs[0].type === "state-change";
    const spacingClass = compactOptionalExchange ? "py-0 min-h-0 my-0" : "py-1 min-h-8 my-px";

    return (
      <div
        {...triggerProps}
        key={index}
        className={`behavior-fitting relative px-2 ${spacingClass} border border-white/10 backdrop-blur-[2px] flex items-center max-w-full min-w-0 font-orbitron ${widthClass} ${typeStyles[type] || ""} ${borderLabel ? "mt-2.5 pt-3" : ""}`}
        data-compact={compact}
      >
        {borderLabel && <BehaviorBorderLabel>{borderLabel}</BehaviorBorderLabel>}
        <div className="flex items-center gap-1.5 min-w-0 w-full justify-center">{content}</div>
        {isRevealed && description && (
          <DescriptionPortal description={description} anchorRef={containerRef} />
        )}
      </div>
    );
  }
};

export function BehaviorBorderLabel({ children }: { children: React.ReactNode }) {
  return (
    <div
      data-behavior-border-label
      style={{ zIndex: Z_INDEX.UI_BASE }}
      className="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1/2 [&_.behavior-fitting]:min-h-0 [&_.behavior-fitting]:p-0 [&_.behavior-fitting]:m-0"
    >
      {children}
    </div>
  );
}

export default BehaviorContainer;
