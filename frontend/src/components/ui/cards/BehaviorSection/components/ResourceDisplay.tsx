import PerConditionIcons from "./PerConditionIcons";
import React from "react";
import GameIcon from "../../../display/GameIcon.tsx";
import { getIconPath, getTagIconPath } from "@/utils/iconStore.ts";
import BehaviorIcon, { BehaviorSign } from "./BehaviorIcon.tsx";
import CardIcon from "./CardIcon.tsx";
import { useBehaviorLayout } from "./BehaviorContainer";
import Slash from "./Slash.tsx";
import { CalculatedOutputDto } from "@/types/generated/api-types.ts";
import {
  type ResourceCondition,
  isProduction as isProductionType,
  isTilePlacement as isTilePlacementType,
  getPer,
  getSelectors,
  getTileRestrictions,
  getTileType,
  isCardOperation,
} from "@/types/resourceConditions.ts";

interface IconDisplayInfo {
  resourceType: string;
  amount: number;
  displayMode: "individual" | "number";
  iconCount: number;
  variableAmount?: boolean;
}

interface TileScaleInfo {
  scale: 1 | 1.25 | 1.5 | 2;
  tileType: string | null;
}

interface ResourceDisplayProps {
  displayInfo: IconDisplayInfo;
  isInput?: boolean;
  resource?: ResourceCondition;
  isGroupedWithOtherNegatives?: boolean;
  context?: "standalone" | "action" | "production" | "default";
  isAffordable?: boolean;
  tileScaleInfo: TileScaleInfo;
  computedAmount?: number;
  computedOutputs?: CalculatedOutputDto[];
}

export function hasResourceVisual(resource: ResourceCondition): boolean {
  return (
    ("description" in resource && Boolean(resource.description)) ||
    !!getIconPath(resource.type) ||
    [
      "copy",
      "card-reveal",
      "discount",
      "vp",
      "global-parameter-lenience",
      "card-peek",
      "card-take",
      "card-buy",
      "card-discard",
    ].includes(resource.type)
  );
}

const isProductionResourceType = (type: string): boolean => type.includes("-production");

const ComputedValueDisplay: React.FC<{
  amount: number;
  resourceType: string;
}> = ({ amount, resourceType }) => {
  const isCredits = resourceType === "credit" || resourceType === "credit-production";
  const isProduction = isProductionResourceType(resourceType);
  const parenClasses = isProduction
    ? "text-[26px] font-normal text-white/70 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]"
    : "text-[22px] font-bold text-white/70 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]";

  return (
    <span className="flex items-center gap-[2px] opacity-80 ml-1">
      <span className={parenClasses}>(</span>
      {isCredits ? (
        <GameIcon iconType={resourceType} amount={amount} size="behavior" />
      ) : (
        <span className="flex items-center gap-[2px]">
          {amount > 1 && (
            <span className="text-[13px] font-black font-orbitron text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-none translate-y-[2px]">
              {amount}
            </span>
          )}
          <GameIcon iconType={resourceType} size="behavior" />
        </span>
      )}
      <span className={parenClasses}>)</span>
    </span>
  );
};

const ResourceDisplay: React.FC<ResourceDisplayProps> = ({
  displayInfo,
  isInput = false,
  resource,
  isGroupedWithOtherNegatives = false,
  context = "default",
  isAffordable = true,
  tileScaleInfo,
  computedAmount,
  computedOutputs,
}) => {
  const { compact, cardId } = useBehaviorLayout();
  const { resourceType, amount } = displayInfo;
  const displayMode = compact && Math.abs(amount) > 1 ? "number" : displayInfo.displayMode;
  const isVariableAmount = !!displayInfo.variableAmount;

  const isCredits = resourceType === "credit" || resourceType === "credit-production";
  const isDiscount = resourceType === "discount";
  const isVP = resourceType === "vp";
  const isProduction = resource ? isProductionType(resource) : false;
  const hasPer = resource ? getPer(resource) : undefined;
  const hasTriggerCap = resource && "maxTrigger" in resource && resource.maxTrigger != null;
  const isAttack =
    resource?.target === "any-player" ||
    resource?.target === "all-opponents" ||
    resource?.target?.startsWith("steal-");

  const resolvedComputedAmount =
    computedAmount ??
    (hasPer && computedOutputs
      ? computedOutputs.find((o) => o.resourceType === resourceType)?.amount
      : undefined);

  const allSelectors = resource ? getSelectors(resource) : undefined;
  const selectorTags: string[] = [];
  if (allSelectors) {
    for (const selector of allSelectors) {
      if (selector.tags) {
        for (const tag of selector.tags) {
          if (!selectorTags.includes(tag)) selectorTags.push(tag);
        }
      }
    }
  }

  const renderSelectorBadges = () => {
    if (selectorTags.length === 0) return null;
    return selectorTags.map((tag: string) => {
      const tagIcon = getTagIconPath(tag);
      if (!tagIcon) return null;
      return (
        <img
          key={`badge-${tag}`}
          src={tagIcon}
          alt={tag}
          className="w-[14px] h-[14px] shrink-0 object-contain [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.6))]"
        />
      );
    });
  };

  if (resource && isCardOperation(resource) && !hasPer) {
    let badgeType: "none" | "peek" | "take" | "buy" | "discard" = "none";
    if (resource.type === "card-peek") {
      badgeType = "peek";
    }
    if (resource.type === "card-take") {
      badgeType = "take";
    }
    if (resource.type === "card-buy") {
      badgeType = "buy";
    }
    if (resource.type === "card-discard") {
      badgeType = "discard";
    }
    return (
      <span className="inline-flex items-center gap-px shrink-0">
        <CardIcon
          amount={Math.abs(amount)}
          badgeType={badgeType}
          isAffordable={isAffordable}
          isAttack={isAttack}
          forceNumber={displayMode === "number"}
        />
        {renderSelectorBadges()}
      </span>
    );
  }

  if (resource?.type === "card-reveal") {
    return (
      <span className="inline-flex items-center gap-1 text-white">
        <CardIcon
          amount={resource.amount}
          badgeType="peek"
          label="Reveal and discard card"
          isAffordable={isAffordable}
        />
        {resource.onMatch && <span>*</span>}
      </span>
    );
  }
  if (resource?.type === "copy") {
    return (
      <span
        className="relative inline-flex w-12 h-10"
        role="img"
        aria-label="Copy a building production box"
      >
        <span className="absolute left-0 top-0 w-8 h-7 border border-[#a06e3c] bg-[#684726]" />
        <span className="absolute left-2 top-2 w-8 h-7 border border-[#a06e3c] bg-[#80582f]" />
        {resource.selectors
          ?.flatMap((selector) => selector.tags ?? [])
          .map((tag) => (
            <img
              key={tag}
              src={getTagIconPath(tag) ?? undefined}
              alt={tag}
              className="absolute right-0 bottom-0 w-5 h-5 object-contain"
            />
          ))}
      </span>
    );
  }
  if (resource?.type === "colony-track-step" || resource?.type === "trade-fleet") {
    return (
      <span className="inline-flex items-center gap-1 font-orbitron text-white">
        <span className="font-orbitron font-bold leading-none translate-y-[2px]">
          {amount > 0 ? "+" : "−"}
          {Math.abs(amount)}
        </span>
        <GameIcon iconType={resource.type === "trade-fleet" ? "trade" : "colony"} size="behavior" />
        {resource.selectionGroup && <span>*</span>}
      </span>
    );
  }
  if (
    resource &&
    "description" in resource &&
    resource.description &&
    !getIconPath(resource.type)
  ) {
    return (
      <span className="text-[11px] leading-tight font-sans text-white text-left max-w-[240px]">
        {resource.description}
      </span>
    );
  }
  if (resource && !hasResourceVisual(resource)) {
    return null;
  }

  // Handle production with per condition (e.g., 1 plant production per plant tag)
  if (isProduction && hasPer) {
    const baseResourceType = resourceType.replace("-production", "");

    let perIcon = null;
    if (hasPer.tags?.length) {
      perIcon = getTagIconPath(hasPer.tags[0]);
    } else if (hasPer.tag) {
      perIcon = getTagIconPath(hasPer.tag);
    } else if (hasPer.type) {
      perIcon = getIconPath(hasPer.type === "card-count" ? "card-draw" : hasPer.type);
    }

    if (perIcon) {
      const perIconEl = <PerConditionIcons per={hasPer} />;

      // Special handling for credits-production - use GameIcon with amount inside
      if (baseResourceType === "credit") {
        const itemClasses = !isAffordable
          ? "flex items-center gap-[3px] relative shrink-0 opacity-40 [filter:grayscale(0.7)_drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]"
          : "flex items-center gap-[3px] relative shrink-0";

        return (
          <div className="flex items-center gap-[3px]">
            <div className="flex flex-wrap gap-[3px] items-center justify-center bg-[linear-gradient(135deg,rgba(160,110,60,0.4)_0%,rgba(139,89,42,0.35)_100%)] border border-[rgba(160,110,60,0.5)] px-1.5 py-[3px] shadow-[0_1px_3px_rgba(0,0,0,0.2)]">
              <div className={itemClasses}>
                <GameIcon iconType="credit" amount={Math.abs(amount)} size="behavior" />
              </div>
              <Slash />
              {perIconEl}
              {(hasPer.selectors?.length ?? 0) > 0 && (
                <span className="text-white font-bold">*</span>
              )}
            </div>
            {resolvedComputedAmount !== undefined && (
              <ComputedValueDisplay amount={resolvedComputedAmount} resourceType={resourceType} />
            )}
          </div>
        );
      } else {
        // For other resources, use regular icon with amount overlay
        const productionIcon = (
          <BehaviorIcon
            resourceType={baseResourceType}
            isProduction={false}
            isAttack={isAttack}
            context="production"
            isAffordable={isAffordable}
            tileScaleInfo={tileScaleInfo}
          />
        );
        if (productionIcon) {
          return (
            <div className="flex items-center gap-[3px]">
              <div className="flex flex-wrap gap-[3px] items-center justify-center bg-[linear-gradient(135deg,rgba(160,110,60,0.4)_0%,rgba(139,89,42,0.35)_100%)] border border-[rgba(160,110,60,0.5)] px-1.5 py-[3px] shadow-[0_1px_3px_rgba(0,0,0,0.2)]">
                <div className="flex items-center gap-[3px] relative shrink-0">
                  {amount > 1 && (
                    <span className="text-[20px] font-black font-orbitron text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] flex items-center ml-0.5 leading-none translate-y-[2px]">
                      {amount}
                    </span>
                  )}
                  {productionIcon}
                </div>
                <Slash />
                {perIconEl}
                {(hasPer.selectors?.length ?? 0) > 0 && (
                  <span className="text-white font-bold">*</span>
                )}
              </div>
              {resolvedComputedAmount !== undefined && (
                <ComputedValueDisplay amount={resolvedComputedAmount} resourceType={resourceType} />
              )}
            </div>
          );
        }
      }
    }
  }

  // Handle production WITHOUT per condition - wrap in brown production box
  // "action" context: e.g., energy-production input in Equatorial Magnetizer
  // "default" context: e.g., production outputs in triggered effects (Tharsis Republic, Mining Area)
  if (isProduction && !hasPer && (context === "action" || context === "default")) {
    const baseResourceType = resourceType.replace("-production", "");
    const baseIsCredits = baseResourceType === "credit";

    const itemClasses = !isAffordable
      ? "flex items-center gap-[3px] relative shrink-0 opacity-40 [filter:grayscale(0.7)_drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]"
      : "flex items-center gap-[3px] relative shrink-0";

    return (
      <div className="flex flex-wrap gap-[3px] items-center justify-center bg-[linear-gradient(135deg,rgba(160,110,60,0.7)_0%,rgba(139,89,42,0.65)_100%)] border border-[rgba(160,110,60,0.7)] px-1.5 py-[3px] shadow-[0_1px_3px_rgba(0,0,0,0.2)]">
        <div className={itemClasses}>
          {baseIsCredits ? (
            <GameIcon iconType="credit" amount={Math.abs(amount)} size="behavior" />
          ) : (
            <>
              {amount > 1 && (
                <span className="text-[13px] font-black font-orbitron text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-none translate-y-[2px]">
                  {amount}
                </span>
              )}
              <BehaviorIcon
                resourceType={baseResourceType}
                isProduction={false}
                isAttack={isAttack}
                context="production"
                isAffordable={isAffordable}
                tileScaleInfo={tileScaleInfo}
              />
            </>
          )}
        </div>
      </div>
    );
  }

  // Handle regular resources with per condition (e.g., 1 floater per jovian tag)
  if (!isProduction && hasPer) {
    let perIcon = null;
    if (hasPer.tags?.length) {
      perIcon = getTagIconPath(hasPer.tags[0]);
    } else if (hasPer.tag) {
      perIcon = getTagIconPath(hasPer.tag);
    } else if (hasPer.type) {
      perIcon = getIconPath(hasPer.type === "card-count" ? "card-draw" : hasPer.type);
    }

    if (perIcon) {
      // Special handling for credits - use GameIcon with amount inside
      if (isCredits) {
        const itemClasses = !isAffordable
          ? "flex items-center gap-[3px] relative shrink-0 opacity-40 [filter:grayscale(0.7)_drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]"
          : "flex items-center gap-[3px] relative shrink-0";

        return (
          <div className="flex items-center gap-[3px]">
            <div className={itemClasses}>
              <GameIcon iconType="credit" amount={Math.abs(amount)} size="behavior" />
            </div>
            <Slash />
            <PerConditionIcons per={hasPer} />
            {hasTriggerCap && <span className="font-orbitron text-white">*</span>}
            {resolvedComputedAmount !== undefined && (
              <ComputedValueDisplay amount={resolvedComputedAmount} resourceType={resourceType} />
            )}
          </div>
        );
      } else {
        // For other resources, use BehaviorIcon with amount overlay
        const iconElement = (
          <BehaviorIcon
            resourceType={resourceType}
            isProduction={false}
            isAttack={isAttack}
            context={context}
            isAffordable={isAffordable}
            tileScaleInfo={tileScaleInfo}
          />
        );

        if (iconElement) {
          return (
            <div className="flex items-center gap-[3px]">
              <div className="flex items-center gap-[3px] relative shrink-0">
                {amount > 1 && (
                  <span className="text-[20px] font-black font-orbitron text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] flex items-center ml-0.5 leading-none translate-y-[2px]">
                    {amount}
                  </span>
                )}
                {iconElement}
                {renderSelectorBadges()}
              </div>
              <Slash />
              <PerConditionIcons per={hasPer} />
              {hasTriggerCap && <span className="font-orbitron text-white">*</span>}
              {resolvedComputedAmount !== undefined && (
                <ComputedValueDisplay amount={resolvedComputedAmount} resourceType={resourceType} />
              )}
            </div>
          );
        }
      }
    }
  }

  if (isCredits) {
    const creditsClasses = `flex items-center gap-[3px] relative shrink-0 text-[13px] font-black pl-[var(--behavior-credit-inset,0px)] ${isAttack ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_1px_rgba(244,67,54,0.9))_drop-shadow(0_0_2px_rgba(244,67,54,0.7))] animate-[attackPulseCredits_2s_ease-in-out_infinite]" : ""}`;

    const isNegative = amount < 0 && !isGroupedWithOtherNegatives;

    const finalCreditsClasses = !isAffordable
      ? `${creditsClasses} opacity-40 [filter:grayscale(0.7)_drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]`
      : creditsClasses;

    return (
      <div className={finalCreditsClasses}>
        {isNegative && <BehaviorSign />}
        <GameIcon
          iconType="credit"
          amount={isVariableAmount ? "X" : Math.abs(amount)}
          size="behavior"
        />
        {resource && "targetRestriction" in resource && resource.targetRestriction && (
          <span className="text-white font-bold">*</span>
        )}
      </div>
    );
  }

  if (isDiscount) {
    const discountClasses = !isAffordable
      ? "flex items-center gap-0.5 relative opacity-40 [filter:grayscale(0.7)_drop-shadow(0_1px_2px_rgba(0,0,0,0.5))]"
      : "flex items-center gap-0.5 relative";

    return (
      <div className={discountClasses}>
        <GameIcon iconType="credit" amount={-amount} size="behavior" />
      </div>
    );
  }

  if (isVP) {
    return (
      <span className="font-orbitron font-bold text-white text-sm [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)] leading-none translate-y-[2px]">
        {amount} VP
      </span>
    );
  }

  // Handle global-parameter-lenience with selector-based global params
  if (resourceType === "global-parameter-lenience") {
    const resourceSelectors = resource ? getSelectors(resource) : undefined;
    const globalParams: string[] = resourceSelectors?.[0]?.globalParameters ?? [
      "temperature",
      "oxygen",
      "ocean",
      "venus",
    ];
    const paramToIcon: Record<string, string> = {
      temperature: "temperature",
      oxygen: "oxygen",
      ocean: "ocean-tile",
      venus: "venus",
    };

    return (
      <div className="flex flex-wrap items-center justify-center gap-[2px]">
        <div className="flex items-center gap-[2px]">
          {globalParams.map((param: string) => (
            <BehaviorIcon
              key={param}
              resourceType={paramToIcon[param] ?? param}
              isProduction={false}
              isAttack={false}
              context={context}
              isAffordable={isAffordable}
              tileScaleInfo={tileScaleInfo}
            />
          ))}
        </div>
        <div className="flex items-center gap-[3px]">
          <span className="text-base font-bold text-white mx-1 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
            :
          </span>
          <div className="flex flex-col items-center leading-none -space-y-[11px] translate-y-[2px]">
            <span className="text-sm font-bold font-orbitron text-[#c8e6c9] [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
              +
            </span>
            <span className="text-sm font-bold font-orbitron text-[#ffcdd2] [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
              -
            </span>
          </div>
          <span className="text-base font-bold font-orbitron text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)] leading-none translate-y-[2px]">
            {amount}
          </span>
          {resource && "temporary" in resource && resource.temporary && (
            <span className="text-white font-bold">*</span>
          )}
        </div>
      </div>
    );
  }

  // Colony bonus: colony icon + "Bonus" label + computed values
  if (resourceType === "colony-bonus") {
    const hasContext = computedOutputs !== undefined;
    const bonusOutputs = computedOutputs ?? [];
    return (
      <div className="flex items-center gap-[3px]">
        <BehaviorIcon
          resourceType="colony"
          isProduction={false}
          isAttack={false}
          context="standalone"
          isAffordable={true}
          tileScaleInfo={tileScaleInfo}
        />
        <span className="text-[11px] font-bold font-orbitron text-white/80 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)] uppercase tracking-wide">
          Bonus
        </span>
        {hasContext && (
          <span className="flex items-center gap-[2px] opacity-80 ml-0.5">
            <span className="text-[22px] font-bold leading-none text-white/70 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
              (
            </span>
            {bonusOutputs.length > 0 ? (
              bonusOutputs.map((o, i) => (
                <span key={i} className="flex items-center gap-[2px]">
                  {o.resourceType === "credit" ? (
                    <GameIcon iconType="credit" amount={o.amount} size="behavior" />
                  ) : (
                    <span className="flex items-center gap-[2px]">
                      {o.amount > 1 && (
                        <span className="text-[13px] font-black font-orbitron text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-none translate-y-[2px]">
                          {o.amount}
                        </span>
                      )}
                      <GameIcon iconType={o.resourceType} size="behavior" />
                    </span>
                  )}
                </span>
              ))
            ) : (
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="white"
                strokeWidth="2.5"
                strokeLinecap="round"
                className="opacity-70 [filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.6))]"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="4.93" y1="4.93" x2="19.07" y2="19.07" />
              </svg>
            )}
            <span className="text-[22px] font-bold leading-none text-white/70 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
              )
            </span>
          </span>
        )}
      </div>
    );
  }

  // Use the passed context or determine based on production status
  let iconContext = context;
  if (iconContext === "default" && isProduction) {
    iconContext = "production";
  }

  const isTilePlacement = resource ? isTilePlacementType(resource) : false;

  const resourceTileRestrictions = resource ? getTileRestrictions(resource) : undefined;
  const hasTileRestrictions =
    isTilePlacement &&
    resourceTileRestrictions &&
    (resourceTileRestrictions.adjacency ||
      resourceTileRestrictions.area ||
      resourceTileRestrictions.adjacentToType ||
      resourceTileRestrictions.minAdjacentOfType !== undefined ||
      resourceTileRestrictions.adjacentToOwned ||
      (resourceTileRestrictions.onBonusType?.length ?? 0) > 0 ||
      (resourceTileRestrictions.boardTags?.length ?? 0) > 0);

  const isWorldTree = resource && getTileType(resource) === "world-tree";
  const specialRestriction = isWorldTree || (cardId === "149" && resourceType === "card-resource");
  const iconType = isWorldTree ? "greenery-placement" : resourceType;
  const iconTileScale = isWorldTree ? { ...tileScaleInfo, tileType: iconType } : tileScaleInfo;
  const baseIconElement = (
    <BehaviorIcon
      resourceType={iconType}
      isProduction={false}
      isAttack={isAttack}
      context={iconContext}
      isAffordable={isAffordable}
      tileScaleInfo={iconTileScale}
    />
  );

  if (!baseIconElement) {
    return (
      <span className="text-xs font-semibold text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
        {isInput && "-"}
        {amount} {resourceType}
      </span>
    );
  }

  if (displayMode === "individual") {
    const absoluteAmount = Math.abs(amount);
    return (
      <div className="flex items-center gap-px relative shrink-0">
        {(isInput || isAttack || amount < 0) &&
          !isGroupedWithOtherNegatives &&
          context !== "action" && <BehaviorSign className="mr-[2px]" />}
        {Array.from({ length: absoluteAmount }, (_, i) => (
          <React.Fragment key={i}>{baseIconElement}</React.Fragment>
        ))}
        {renderSelectorBadges()}
        {(specialRestriction ||
          resource?.type === "colony-tile-add" ||
          hasTileRestrictions ||
          (resource && "targetRestriction" in resource && !!resource.targetRestriction) ||
          resource?.target === "triggering-card") && (
          <span className="text-white font-bold text-sm ml-0.5 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
            *
          </span>
        )}
      </div>
    );
  } else {
    return (
      <div className="flex items-center gap-[3px] relative shrink-0">
        {(isInput || isAttack || amount < 0) &&
          !isGroupedWithOtherNegatives &&
          context !== "action" && <BehaviorSign />}
        <span className="text-[13px] font-black font-orbitron text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] flex items-center justify-end min-w-[var(--behavior-quantity-width,0px)] leading-none translate-y-[2px]">
          {Array.from(String(isVariableAmount ? "X" : Math.abs(amount)), (digit, index) => (
            <span key={index} className="inline-block w-[1ch] text-center">
              {digit}
            </span>
          ))}
        </span>
        {baseIconElement}
        {renderSelectorBadges()}
        {(specialRestriction ||
          resource?.type === "colony-tile-add" ||
          hasTileRestrictions ||
          (resource && "targetRestriction" in resource && !!resource.targetRestriction) ||
          resource?.target === "triggering-card") && (
          <span className="text-white font-bold text-sm ml-0.5 [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
            *
          </span>
        )}
      </div>
    );
  }
};

export default ResourceDisplay;
