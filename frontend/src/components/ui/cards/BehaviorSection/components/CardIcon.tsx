import { BehaviorArrow } from "./BehaviorIcon.tsx";
import React from "react";
import { getIconPath } from "@/utils/iconStore.ts";
import { useBehaviorLayout } from "./BehaviorContainer";
import GameIcon from "../../../display/GameIcon";

export function TaggedCardIcon({
  tags,
  iconType = "card-draw",
}: {
  tags: string[];
  iconType?: string;
}) {
  return (
    <span
      data-behavior-atom
      className="relative inline-flex items-center shrink-0 pr-2"
      role="img"
      aria-label={`${tags.join(" and ")} card`}
    >
      <GameIcon iconType={iconType} size="behavior" />
      <span className="absolute right-0 bottom-0 flex items-center gap-px [--behavior-icon-size:14px]">
        {tags.map((tag) => (
          <GameIcon key={tag} iconType={`${tag}-tag`} size="behavior" />
        ))}
      </span>
    </span>
  );
}

interface CardIconProps {
  amount: number;
  label?: string;
  badgeType: "peek" | "take" | "buy" | "discard" | "none";
  isAffordable?: boolean;
  isAttack?: boolean;
  totalCardTypes?: number;
  keepAmount?: number;
  forceNumber?: boolean;
}

const cardActionBadges = {
  peek: {
    label: "Peek at card",
    glyph: (
      <>
        <path d="M1.5 8s2.5-4 6.5-4 6.5 4 6.5 4-2.5 4-6.5 4-6.5-4-6.5-4Z" />
        <circle cx="8" cy="8" r="1.5" />
      </>
    ),
  },
  take: {
    label: "Take card",
    glyph: <path d="M8 2v11m-4-4 4 4 4-4" />,
  },
  buy: {
    label: "Buy card",
    glyph: (
      <>
        <path d="M11.5 4.5C10 2.5 4 2.5 4 5.5s8 2 8 5-6 3-8 1" />
        <path d="M8 1.5v13" />
      </>
    ),
  },
  discard: {
    label: "Discard card",
    glyph: <path d="m4 4 8 8m0-8-8 8" />,
  },
};

const CardIcon: React.FC<CardIconProps> = ({
  amount,
  label,
  badgeType,
  isAffordable = true,
  isAttack = false,
  totalCardTypes = 1,
  keepAmount,
  forceNumber = false,
}) => {
  const { compact } = useBehaviorLayout();
  const cardIcon = getIconPath("card-draw");

  if (!cardIcon) {
    return null;
  }

  const badge = badgeType === "none" ? null : cardActionBadges[badgeType];
  const badgeOpacity = isAffordable ? "" : "opacity-40";

  const glowFilter = isAttack
    ? "drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_1px_rgba(244,67,54,0.9))_drop-shadow(0_0_2px_rgba(244,67,54,0.7))"
    : "drop-shadow(0_1px_3px_rgba(0,0,0,0.6))_drop-shadow(0_0_1px_rgba(255,248,220,0.6))_drop-shadow(0_0_2px_rgba(255,248,220,0.4))";

  const attackAnimation = isAttack ? " animate-[attackPulse_2s_ease-in-out_infinite]" : "";

  const iconClass = isAffordable
    ? `w-[var(--behavior-icon-size,26px)] h-[var(--behavior-icon-size,26px)] object-contain [filter:${glowFilter}]${attackAnimation}`
    : `w-[var(--behavior-icon-size,26px)] h-[var(--behavior-icon-size,26px)] object-contain opacity-40 [filter:grayscale(0.7)_drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]`;

  const renderSingleIcon = (picked = false, hideBadge = false) => (
    <div
      className={`relative inline-flex shrink-0 ${(badge && !hideBadge) || picked ? "pr-[3px] pb-0.5" : ""}`}
      role="img"
      aria-label={picked ? "Keep this card" : (label ?? badge?.label ?? "Card")}
    >
      <img src={cardIcon} alt="" className={iconClass} />
      {((badge && !hideBadge) || picked) && (
        <svg
          className={`absolute bottom-0 right-0 w-4 h-4 text-[#fff8e7] pointer-events-none ${badgeOpacity}`}
          viewBox="0 0 16 16"
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
          focusable="false"
        >
          <g stroke="#090b10" strokeWidth="4.5">
            {picked ? cardActionBadges.take.glyph : badge?.glyph}
          </g>
          <g stroke="currentColor" strokeWidth="1.75">
            {picked ? cardActionBadges.take.glyph : badge?.glyph}
          </g>
        </svg>
      )}
    </div>
  );

  if (keepAmount !== undefined) {
    const showSet = amount <= 4;
    return (
      <div
        data-behavior-atom
        className="flex flex-col items-center gap-1 shrink-0 font-orbitron text-white"
        role="img"
        aria-label={`Look at ${amount} cards and keep ${keepAmount} of them`}
      >
        <svg
          className="w-4 h-4"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          aria-hidden="true"
        >
          {cardActionBadges.peek.glyph}
        </svg>
        <div className="flex items-center gap-2 pb-1">
          {showSet ? (
            Array.from({ length: amount }, (_, index) => (
              <React.Fragment key={index}>
                {renderSingleIcon(index < keepAmount, true)}
              </React.Fragment>
            ))
          ) : (
            <>
              <span className="text-[13px] font-bold leading-none translate-y-[2px]">{amount}</span>
              {renderSingleIcon(false, true)}
              <BehaviorArrow />
              <span className="text-[11px] font-bold">{keepAmount}</span>
              {renderSingleIcon(true)}
            </>
          )}
        </div>
      </div>
    );
  }

  // Amount 1: Show single icon, no number
  if (amount === 1) {
    return <div className="flex items-center gap-0.5 relative">{renderSingleIcon()}</div>;
  }

  // Amount 2-3 with single card type: Show individual icons side by side, no number
  if (amount <= 3 && totalCardTypes === 1 && !compact && !forceNumber) {
    return (
      <div className="flex items-center gap-0.5 relative">
        {Array.from({ length: amount }, (_, i) => (
          <React.Fragment key={i}>{renderSingleIcon()}</React.Fragment>
        ))}
      </div>
    );
  }

  // Multiple card types or amount > 2: Show number + single icon
  return (
    <div className="flex items-center gap-0.5 relative">
      <span className="text-[13px] font-orbitron font-bold leading-none translate-y-[2px] text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] mr-px">
        {amount}
      </span>
      {renderSingleIcon()}
    </div>
  );
};

export default CardIcon;
