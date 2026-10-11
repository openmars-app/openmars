import { useCardResourceStorage } from "../../CardStateVisuals.tsx";
import GameIcon from "../../../display/GameIcon.tsx";
import React from "react";
import type { PerConditionDto } from "@/types/generated/api-types";
import { getIconPath, getTagIconPath } from "@/utils/iconStore";

export default function PerConditionIcons({ per }: { per: PerConditionDto }) {
  const storage = useCardResourceStorage();
  if (storage?.icon && per.target === "self-card" && per.type === storage.type) {
    return (
      <span className="inline-flex items-center gap-0.5">
        {per.amount > 1 && per.amount}
        <GameIcon iconType={storage.icon} size="behavior" />
      </span>
    );
  }
  if (per.type === "permanent-production-types") {
    return (
      <span
        className="relative inline-flex items-center"
        role="img"
        aria-label="Distinct positive permanent production types"
      >
        <GameIcon iconType="standard-resource-production" size="behavior" />
        <span className="absolute -bottom-1 -right-1.5 inline-flex [--behavior-icon-size:12px]">
          <GameIcon iconType="permanent" size="behavior" />
        </span>
      </span>
    );
  }
  const tags = [...new Set([...(per.tags ?? []), ...(per.tag ? [per.tag] : [])])];
  const icons = tags.length
    ? tags.map((tag) => ({ name: tag, src: getTagIconPath(tag) }))
    : [{ name: per.type, src: getIconPath(per.type === "card-count" ? "card-draw" : per.type) }];
  // City counts default to all owners, matching the backend tile counter.
  const others = per.target
    ? !["self-player", "self-card"].includes(per.target)
    : per.type === "city-tile";
  return (
    <span className="inline-flex items-center gap-px font-orbitron text-white">
      {per.amount > 1 && <span className="text-[13px] font-bold leading-none">{per.amount}</span>}
      {icons.map((icon, index) => (
        <React.Fragment key={icon.name}>
          {index > 0 && <span className="mx-0.5 text-xs font-bold">+</span>}
          {icon.src && (
            <img
              src={icon.src}
              alt={icon.name}
              className={`w-[var(--behavior-icon-size,26px)] h-[var(--behavior-icon-size,26px)] object-contain shrink-0 ${
                others
                  ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_1px_rgba(244,67,54,0.9))_drop-shadow(0_0_2px_rgba(244,67,54,0.7))] animate-[attackPulse_2s_ease-in-out_infinite]"
                  : "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]"
              }`}
            />
          )}
        </React.Fragment>
      ))}
    </span>
  );
}
