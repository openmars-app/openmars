import { StatePredicates } from "../cards/CardStateVisuals.tsx";
import PerConditionIcons from "../cards/BehaviorSection/components/PerConditionIcons.tsx";
import GameIcon from "./GameIcon.tsx";
import React from "react";
import type { VPConditionDto } from "@/types/generated/api-types.ts";
import { getIconPath } from "@/utils/iconStore.ts";
import DecorBox from "./DecorBox.tsx";

interface VictoryPointIconProps {
  value?: number | string;
  vpConditions?: VPConditionDto[];
  corner?: "bottom-right" | "top-left";
  bare?: boolean;
}

const renderContent = (content: React.ReactNode) => (
  <div className="inline-flex items-center gap-1">
    {content}
    <span className="text-[9px] text-white/50 font-semibold tracking-wider">VP</span>
  </div>
);

const VictoryPointIcon: React.FC<VictoryPointIconProps> = ({
  value,
  vpConditions,
  corner = "bottom-right",
  bare = false,
}) => {
  const renderBox = (content: React.ReactNode) => {
    const conditional =
      vpConditions?.filter((condition) => condition.stateRequirements?.length) ?? [];
    const annotated = (
      <span className="inline-flex flex-wrap items-center gap-0.5 [--behavior-icon-size:14px]">
        {conditional.map((condition, index) => (
          <StatePredicates key={index} requirements={condition.stateRequirements} />
        ))}
        {content}
      </span>
    );
    if (bare) {
      return renderContent(annotated);
    }
    return (
      <DecorBox corner={corner}>
        {annotated}
        <span className="text-[9px] text-white/50 font-semibold tracking-wider">VP</span>
      </DecorBox>
    );
  };

  if (vpConditions && Array.isArray(vpConditions) && vpConditions.length > 0) {
    const totalConditions = vpConditions.length;

    if (totalConditions === 1) {
      const condition = vpConditions[0];

      if (condition.condition === "fixed" || condition.condition === "once") {
        if (condition.amount === 0) return null;
        return renderBox(<span className="text-[13px] font-bold">{condition.amount}</span>);
      } else if (condition.condition === "per" && condition.per) {
        const perCondition = condition.per;
        const resourceType = perCondition.tag || perCondition.type;
        const resourceIcon = resourceType ? getIconPath(resourceType) : null;
        const perAmount = perCondition.amount || 1;

        return renderBox(
          <div className="flex items-center gap-0.5">
            <span className="text-[11px] font-bold">{condition.amount}</span>
            <span className="text-[9px] text-white/40">/</span>
            {perAmount > 1 && <span className="text-[11px] font-bold">{perAmount}</span>}
            {!resourceIcon && (
              <span className="inline-flex [--behavior-icon-size:14px]">
                {perCondition.type === "permanent-production-types" ? (
                  <PerConditionIcons per={perCondition} />
                ) : (
                  <GameIcon iconType="effect" size="behavior" />
                )}
              </span>
            )}
            {resourceIcon && (
              <img src={resourceIcon} alt={resourceType} className="w-3.5 h-3.5 object-contain" />
            )}
            {perCondition.adjacentToSelfTile && (
              <span className="text-[9px] text-white/40 font-bold">*</span>
            )}
          </div>,
        );
      }
    } else {
      let totalFixed = 0;
      let firstPerCondition = null;

      for (const condition of vpConditions) {
        if (condition.condition === "fixed" || condition.condition === "once") {
          totalFixed += condition.amount;
        } else if (condition.condition === "per" && !firstPerCondition) {
          firstPerCondition = condition;
        }
      }

      if (firstPerCondition && firstPerCondition.per) {
        const perCondition = firstPerCondition.per;
        const resourceType = perCondition.tag || perCondition.type;
        const resourceIcon = resourceType ? getIconPath(resourceType) : null;
        const perAmount = perCondition.amount || 1;

        return renderBox(
          <div className="flex items-center gap-0.5">
            <span className="text-[11px] font-bold">{firstPerCondition.amount}</span>
            <span className="text-[9px] text-white/40">/</span>
            {perAmount > 1 && <span className="text-[11px] font-bold">{perAmount}</span>}
            {!resourceIcon && (
              <span className="inline-flex [--behavior-icon-size:14px]">
                {perCondition.type === "permanent-production-types" ? (
                  <PerConditionIcons per={perCondition} />
                ) : (
                  <GameIcon iconType="effect" size="behavior" />
                )}
              </span>
            )}
            {resourceIcon && (
              <img src={resourceIcon} alt={resourceType} className="w-3.5 h-3.5 object-contain" />
            )}
            {perCondition.adjacentToSelfTile && (
              <span className="text-[9px] text-white/40 font-bold">*</span>
            )}
          </div>,
        );
      } else if (totalFixed > 0) {
        return renderBox(<span className="text-[13px] font-bold">{totalFixed}</span>);
      }
    }

    return null;
  }

  if (value === 0 || !value) {
    return null;
  }

  return renderBox(<span className="text-[13px] font-bold">{value}</span>);
};

export default VictoryPointIcon;
