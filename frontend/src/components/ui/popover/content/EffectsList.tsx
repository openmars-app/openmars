import React from "react";
import { PlayerEffectDto } from "@/types/generated/api-types.ts";
import BehaviorSection from "../../cards/BehaviorSection";
import { GamePopoverItem, getThemeStyles } from "../../GamePopover";
import type { ContentDensity } from "./density.ts";

interface EffectsListProps {
  effects: PlayerEffectDto[];
  density: ContentDensity;
}

const EffectsList: React.FC<EffectsListProps> = ({ effects, density }) => {
  const isScreen = density === "screen";

  if (effects.length === 0) {
    return (
      <div className="flex items-center justify-center py-10 px-5">
        <span className="font-orbitron text-sm text-white/50">No effects</span>
      </div>
    );
  }

  const listClass = isScreen
    ? "popover-list p-3 grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-2"
    : "popover-list popover-list-headed p-2 flex flex-col gap-2";
  const nameClass = isScreen
    ? "text-white/80 text-[13px] font-medium uppercase tracking-[0.5px] [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-[1.2] flex items-center gap-2"
    : "text-white/70 text-[11px] font-medium uppercase tracking-[0.5px] [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-[1.2] opacity-80 flex items-center gap-2 max-[768px]:text-[11px]";

  return (
    <div className={listClass} style={isScreen ? getThemeStyles("effects") : undefined}>
      {effects.map((effect, index) => (
        <GamePopoverItem
          className={isScreen ? "popover-list-item min-h-11" : "popover-list-item"}
          key={`${effect.cardId}-${effect.behaviorIndex}`}
          state="available"
          animationDelay={index * 0.05}
        >
          <div className="flex flex-col gap-2 flex-1">
            <div className={nameClass}>{effect.cardName}</div>

            <div className="relative w-full min-h-[32px] [&>div]:!relative [&>div]:!bottom-auto [&>div]:!left-auto [&>div]:!right-auto [&>div]:w-full [&>div:hover]:!transform-none [&>div:hover]:!shadow-none [&>div:hover]:!filter-none">
              <BehaviorSection
                behaviors={[effect.behavior]}
                sourceBehaviorIndex={effect.behaviorIndex}
                cardId={effect.cardId}
                computedValues={effect.computedValues}
                greyOutAll={false}
              />
            </div>
          </div>
        </GamePopoverItem>
      ))}
    </div>
  );
};

export default EffectsList;
