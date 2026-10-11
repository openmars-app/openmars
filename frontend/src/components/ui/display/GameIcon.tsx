import React from "react";
import { getCardStateIcon } from "./CardStateIcon.tsx";
import SemanticIcon, { hasSemanticIcon } from "./SemanticIcon.tsx";
import { ResourceType, ResourceTypeCredit, CardTag } from "@/types/generated/api-types.ts";
import { getIconPath } from "@/utils/iconStore.ts";

/**
 * Unified icon type supporting backend types and frontend-specific icons.
 * - ResourceType: All game resources (credits, steel, titanium, plants, energy, heat, etc.)
 * - CardTag: Card category tags (space, earth, science, power, building, etc.)
 * - Frontend-specific: "milestone", "award", "card"
 *
 * Production resources are automatically detected via "-production" suffix (e.g., "energy-production")
 */
export type GameIconType = ResourceType | CardTag | "milestone" | "award" | "card";

interface GameIconProps {
  /** The type of icon to display. Supports ResourceType, CardTag, or frontend-specific icons */
  iconType?: GameIconType;
  /**
   * Amount to display on the icon.
   * - For credits: Shows value inside the icon (number or string like "X")
   * - For other resources: Shows number in bottom-right corner (if > 1)
   */
  amount?: number | string;
  amountPlacement?: "badge" | "center";
  /**
   * Whether to display attack/threat indicator with red glow animation.
   * Only applicable for resource icons when showing enemy attacks or resource removal.
   */
  isAttack?: boolean;
  /** Size of the icon: small (24px), medium (32px), large (40px) */
  size?: "small" | "medium" | "large" | "behavior";
  /** Additional CSS classes to apply to the icon container */
  className?: string;
}

/**
 * Unified icon component for displaying all game icons (resources, tags, and frontend-specific icons).
 *
 * Features:
 * - Automatic production background for resources ending in "-production"
 * - Special number overlay for megacredits (inside icon)
 * - Attack indicator with red glow animation
 * - Consistent sizing across all icon types
 * - Maps backend ResourceType and CardTag enums to asset paths
 *
 * @example
 * // Basic resource icon
 * <GameIcon iconType={ResourceTypeCredit} />
 *
 * @example
 * // Credits with amount (number inside icon)
 * <GameIcon iconType={ResourceTypeCredit} amount={25} size="large" />
 *
 * @example
 * // Production resource (automatic brown background)
 * <GameIcon iconType="energy-production" amount={3} />
 *
 * @example
 * // Card tag icon
 * <GameIcon iconType={TagSpace} size="small" />
 *
 * @example
 * // Attack indicator (red glow)
 * <GameIcon iconType={ResourceTypePlant} amount={2} isAttack={true} />
 *
 * @example
 * // Frontend-specific icon
 * <GameIcon iconType="milestone" size="medium" />
 */
const GameIcon: React.FC<GameIconProps> = ({
  iconType,
  amount,
  amountPlacement = "badge",
  isAttack = false,
  size = "medium",
  className = "",
}) => {
  const isProduction = iconType?.endsWith("-production") || false;
  const baseType = isProduction && iconType ? iconType.replace("-production", "") : iconType;
  const isCredits = baseType === ResourceTypeCredit;
  const isCardType =
    baseType === "card-draw" ||
    baseType === "card-take" ||
    baseType === "card-peek" ||
    baseType === "card";

  const iconUrl = baseType ? getIconPath(baseType) : null;
  const displayType = baseType || "";
  const customIcon = getCardStateIcon(displayType);

  if (!iconUrl && !customIcon) {
    const dimension =
      size === "behavior"
        ? "var(--behavior-icon-size,26px)"
        : `${{ small: 24, medium: 32, large: 40 }[size]}px`;
    return (
      <span
        className={`inline-flex items-center gap-0.5 ${className}`}
        data-unsupported-icon={hasSemanticIcon(displayType) ? undefined : displayType}
      >
        {amount !== undefined && amount !== 1 && (
          <span className="font-orbitron text-[12px] font-bold">{amount}</span>
        )}
        <span style={{ width: dimension, height: dimension }} className="inline-flex shrink-0">
          <SemanticIcon icon={displayType} className="h-full w-full" />
        </span>
      </span>
    );
  }

  const sizeMap = {
    small: { icon: 24, baseFontSize: 12 },
    medium: { icon: 32, baseFontSize: 16 },
    large: { icon: 40, baseFontSize: 20 },
  };

  const dimensions = sizeMap[size === "behavior" ? "small" : size];
  const behaviorClass =
    size === "behavior" ? "[--game-icon-size:var(--behavior-icon-size,26px)] shrink-0" : "";
  const iconSize = size === "behavior" ? "var(--game-icon-size)" : `${dimensions.icon}px`;
  const creditFontSize = () => {
    if (size !== "behavior") {
      return `${getCreditFontSize()}px`;
    }
    const digits = amount === undefined ? 1 : String(amount).length;
    return `calc(var(--game-icon-size) * ${digits > 2 ? 0.425 : 0.5})`;
  };

  const getCreditFontSize = () => {
    if (amount === undefined) return dimensions.baseFontSize;
    const digits = typeof amount === "string" ? amount.length : String(Math.abs(amount)).length;
    if (digits <= 2) return dimensions.baseFontSize;
    return Math.round(dimensions.baseFontSize * 0.85);
  };

  const attackGlow = isAttack
    ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_1px_rgba(244,67,54,0.9))_drop-shadow(0_0_2px_rgba(244,67,54,0.7))] animate-[attackPulse_2s_ease-in-out_infinite]"
    : "";

  const cardOutline = isCardType
    ? "[filter:drop-shadow(0_0_1px_rgba(255,248,220,0.6))_drop-shadow(0_0_2px_rgba(255,248,220,0.4))]"
    : "";

  const renderCoreIcon = () => {
    const glowEffect = attackGlow || cardOutline;

    if ((isCredits || amountPlacement === "center") && amount !== undefined) {
      return (
        <div
          className={`relative inline-flex items-center justify-center ${glowEffect} ${behaviorClass} ${isProduction ? "" : className}`}
          style={{
            width: iconSize,
            height: iconSize,
          }}
        >
          {customIcon ? (
            <span
              className="inline-flex h-full w-full"
              role="img"
              aria-label={displayType}
              data-semantic-icon={displayType}
            >
              {customIcon}
            </span>
          ) : (
            <img
              src={iconUrl ?? undefined}
              alt={displayType}
              className="w-full h-full object-contain [filter:drop-shadow(0_2px_4px_rgba(0,0,0,0.6))]"
            />
          )}
          <span
            className={`absolute top-0 left-0 right-0 bottom-0 ${isCredits ? "text-black" : "text-white"} font-black font-[Prototype,Arial_Black,Arial,sans-serif] flex items-center justify-center text-center leading-none [text-shadow:0_0_2px_rgba(255,255,255,0.3)] tracking-[0.5px] [-webkit-font-smoothing:antialiased] [-moz-osx-font-smoothing:grayscale] [text-rendering:optimizeLegibility]`}
            style={{ fontSize: creditFontSize() }}
          >
            {amount}
          </span>
        </div>
      );
    }

    return (
      <div
        className={`relative inline-flex items-center justify-center ${glowEffect} ${behaviorClass} ${isProduction ? "" : className}`}
        style={{
          width: iconSize,
          height: iconSize,
        }}
      >
        {customIcon ? (
          <span
            className="inline-flex h-full w-full"
            role="img"
            aria-label={displayType}
            data-semantic-icon={displayType}
          >
            {customIcon}
          </span>
        ) : (
          <img
            src={iconUrl ?? undefined}
            alt={displayType}
            className="w-full h-full object-contain [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]"
          />
        )}
        {amount !== undefined && typeof amount === "number" && amount > 1 && !isCredits && (
          <span
            className="absolute bottom-0 right-0 text-white font-bold [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] bg-black/50 rounded-full px-1 min-w-[16px] text-center leading-none"
            style={{ fontSize: `${dimensions.baseFontSize}px` }}
          >
            {amount}
          </span>
        )}
      </div>
    );
  };

  if (isProduction) {
    return (
      <div
        className={`inline-flex items-center justify-center bg-[linear-gradient(135deg,rgba(160,110,60,0.4)_0%,rgba(139,89,42,0.35)_100%)] border border-[rgba(160,110,60,0.5)] px-1.5 py-[3px] shadow-[0_1px_3px_rgba(0,0,0,0.2)] ${className}`}
      >
        {renderCoreIcon()}
      </div>
    );
  }

  return renderCoreIcon();
};

export default GameIcon;
