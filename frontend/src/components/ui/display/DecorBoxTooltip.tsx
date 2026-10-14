import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import React, { ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { FormattedDescription } from "./FormattedDescription.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";

interface DecorBoxTooltipProps {
  id?: string;
  description?: string | null;
  children?: ReactNode;
  position: { x: number; y: number } | null;
  placement?: "below" | "above";
  flipY?: number;
  cornerSize?: number;
  maxWidth?: number | string;
}

const DecorBoxTooltip: React.FC<DecorBoxTooltipProps> = ({
  id,
  description,
  children,
  position,
  placement = "below",
  flipY,
  cornerSize = 14,
  maxWidth,
}) => {
  const portalRoot = useOverlayPortalRoot();
  const tooltipRef = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const tooltip = tooltipRef.current;
    if (!tooltip || !position) {
      return;
    }
    const rect = tooltip.getBoundingClientRect();
    const left = Math.max(
      12,
      Math.min(position.x - rect.width / 2, window.innerWidth - rect.width - 12),
    );
    let top = position.y;
    if (placement === "above") {
      top -= rect.height;
    }
    if (flipY !== undefined && placement === "above" && top < 12) {
      top = flipY;
    }
    if (
      flipY !== undefined &&
      placement === "below" &&
      top + rect.height > window.innerHeight - 12
    ) {
      top = flipY - rect.height;
    }
    top = Math.max(12, Math.min(top, window.innerHeight - rect.height - 12));
    tooltip.style.left = `${left}px`;
    tooltip.style.top = `${top}px`;
    tooltip.style.transform = "none";
  }, [position, placement, flipY, children]);
  const [dismissedPosition, setDismissedPosition] = useState<typeof position>(null);
  useEffect(() => {
    if (!position) {
      return;
    }
    const dismiss = () => setDismissedPosition(position);
    window.addEventListener("scroll", dismiss, true);
    window.addEventListener("resize", dismiss);
    return () => {
      window.removeEventListener("scroll", dismiss, true);
      window.removeEventListener("resize", dismiss);
    };
  }, [position]);

  if (position === dismissedPosition) {
    return null;
  }
  if ((!description && !children) || !position) return null;

  const paddingClass = placement === "below" ? "pt-1" : "pb-2";
  const translateY = placement === "below" ? "0" : "-100%";

  return createPortal(
    <div
      ref={tooltipRef}
      id={id}
      role="tooltip"
      className={`fixed w-max ${maxWidth === undefined ? "max-w-40" : ""} ${paddingClass} pointer-events-none animate-[fadeIn_150ms_ease-in]`}
      style={{
        left: position.x,
        top: position.y,
        transform: `translate(-50%, ${translateY})`,
        zIndex: Z_INDEX.FLOATING_TOOLTIP,
        maxWidth: maxWidth ?? "min(160px, calc(100vw - 24px))",
      }}
    >
      <div
        className="game-panel text-white/90 text-left text-[11px] leading-tight px-3 py-2"
        style={{ "--panel-cut": `${cornerSize}px` } as React.CSSProperties}
      >
        {children || <FormattedDescription text={description!} />}
      </div>
    </div>,
    portalRoot,
  );
};

export default DecorBoxTooltip;
