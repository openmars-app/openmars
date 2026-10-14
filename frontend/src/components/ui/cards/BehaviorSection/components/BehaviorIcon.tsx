import React from "react";
import { getIconPath, isTagIcon } from "@/utils/iconStore.ts";

interface TileScaleInfo {
  scale: 1 | 1.25 | 1.5 | 2;
  tileType: string | null;
}

interface BehaviorIconProps {
  resourceType: string;
  sizingType?: string;
  isProduction?: boolean;
  isAttack?: boolean;
  context?: "standalone" | "action" | "production" | "default";
  isAffordable?: boolean;
  tileScaleInfo: TileScaleInfo;
}

const BehaviorIcon: React.FC<BehaviorIconProps> = ({
  resourceType,
  sizingType = resourceType,
  isProduction: _isProduction = false,
  isAttack = false,
  context = "default",
  isAffordable = true,
  tileScaleInfo,
}) => {
  const cleanType = sizingType?.toLowerCase().replace(/[_\s]/g, "-");

  if (cleanType === "vp") {
    return <span className="font-orbitron font-bold text-white text-xs leading-none">VP</span>;
  }

  const icon = getIconPath(resourceType);

  if (!icon) {
    return null;
  }

  const isScaledTile = tileScaleInfo.scale > 1 && cleanType === tileScaleInfo.tileType;

  let iconClass: string;
  if (isScaledTile) {
    if (tileScaleInfo.scale === 2) {
      iconClass =
        "w-[calc(var(--behavior-icon-size,26px)*2)] h-[calc(var(--behavior-icon-size,26px)*2)] object-contain shrink-0 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]";
    } else if (tileScaleInfo.scale === 1.5) {
      iconClass =
        "w-[calc(var(--behavior-icon-size,26px)*1.5)] h-[calc(var(--behavior-icon-size,26px)*1.5)] object-contain shrink-0 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]";
    } else if (tileScaleInfo.scale === 1.25) {
      iconClass =
        "w-[calc(var(--behavior-icon-size,26px)*1.25)] h-[calc(var(--behavior-icon-size,26px)*1.25)] object-contain shrink-0 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]";
    } else {
      iconClass =
        "w-[var(--behavior-icon-size,26px)] h-[var(--behavior-icon-size,26px)] object-contain shrink-0 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]";
    }
  } else {
    iconClass =
      "w-[var(--behavior-icon-size,26px)] h-[var(--behavior-icon-size,26px)] object-contain shrink-0 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]";
  }

  const isTag = isTagIcon(cleanType);
  const isPlacement =
    cleanType === "city-placement" ||
    cleanType === "greenery-placement" ||
    cleanType === "ocean-placement" ||
    cleanType === "volcano-placement" ||
    cleanType === "land-claim" ||
    cleanType === "tile-placement";
  const isTR = cleanType === "tr";
  const isCard =
    cleanType === "card-draw" ||
    cleanType === "card-take" ||
    cleanType === "card-peek" ||
    cleanType === "card";

  const isStandaloneTile =
    cleanType === "city-tile" ||
    cleanType === "greenery-tile" ||
    cleanType === "ocean-tile" ||
    cleanType === "volcano-tile";
  const isStandaloneCard = cleanType === "card-draw" || cleanType === "card";
  const shouldUseStandaloneSize =
    context === "standalone" &&
    (isPlacement || isStandaloneTile || isStandaloneCard || cleanType === "copy");

  if (!isScaledTile) {
    if (isAttack) {
      iconClass =
        "w-[var(--behavior-icon-size,26px)] h-[var(--behavior-icon-size,26px)] object-contain shrink-0 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_1px_rgba(244,67,54,0.9))_drop-shadow(0_0_2px_rgba(244,67,54,0.7))] animate-[attackPulse_2s_ease-in-out_infinite]";
    } else if (shouldUseStandaloneSize) {
      const cardGlow = isStandaloneCard
        ? "_drop-shadow(0_0_1px_rgba(255,248,220,0.6))_drop-shadow(0_0_2px_rgba(255,248,220,0.4))"
        : "";
      iconClass = `w-[calc(var(--behavior-icon-size,26px)*1.4)] h-[calc(var(--behavior-icon-size,26px)*1.4)] object-contain shrink-0 [filter:drop-shadow(0_1px_3px_rgba(0,0,0,0.7))${cardGlow}]`;
    } else if (isPlacement) {
      iconClass =
        "w-[calc(var(--behavior-icon-size,26px)*1.15)] h-[calc(var(--behavior-icon-size,26px)*1.15)] object-contain shrink-0 [filter:drop-shadow(0_1px_3px_rgba(0,0,0,0.6))]";
    } else if (isTR) {
      iconClass = "w-8 h-8 object-contain shrink-0 [filter:drop-shadow(0_1px_3px_rgba(0,0,0,0.6))]";
    } else if (isCard) {
      iconClass =
        "w-[30px] h-[30px] object-contain shrink-0 [filter:drop-shadow(0_1px_3px_rgba(0,0,0,0.6))_drop-shadow(0_0_1px_rgba(255,248,220,0.6))_drop-shadow(0_0_2px_rgba(255,248,220,0.4))]";
    } else if (isTag) {
      iconClass =
        "w-[var(--behavior-icon-size,26px)] h-[var(--behavior-icon-size,26px)] object-contain shrink-0 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]";
    }
  }

  const finalIconClass = !isAffordable
    ? `${iconClass} opacity-40 [filter:grayscale(0.7)_drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]`
    : iconClass;

  return (
    <img
      src={icon}
      alt={resourceType.toLowerCase().replace(/[_\s]/g, "-")}
      className={finalIconClass}
    />
  );
};

export const BehaviorSign: React.FC<{ positive?: boolean; className?: string }> = ({
  positive = false,
  className = "",
}) => (
  <svg
    aria-label={positive ? "+" : "−"}
    role="img"
    viewBox="0 0 12 12"
    className={`w-[10px] h-[10px] shrink-0 self-center ${positive ? "text-[#c8e6c9]" : "text-[#ffcdd2]"} ${className}`}
    fill="none"
    stroke="currentColor"
    strokeWidth="1.8"
    strokeLinecap="square"
  >
    <path d="M2 6h8" />
    {positive && <path d="M6 2v8" />}
  </svg>
);

export const BehaviorArrow: React.FC = () => (
  <svg
    aria-label="then"
    role="img"
    viewBox="0 0 14 14"
    className="w-3 h-3 shrink-0 self-center text-white"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.8"
    strokeLinecap="square"
    strokeLinejoin="miter"
  >
    <path d="M1.5 7h10M7.5 3l4 4-4 4" />
  </svg>
);

export default BehaviorIcon;
