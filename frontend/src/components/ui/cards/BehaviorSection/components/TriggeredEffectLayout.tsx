import BadgedGameIcon from "../../../display/BadgedGameIcon";
import { StateSymbol, useCardResourceStorage } from "../../CardStateVisuals.tsx";
import { OperationRequirements } from "./BehaviorQualifiers.tsx";
import { BehaviorArrow } from "./BehaviorIcon";
import React from "react";
import { useBehaviorLayout } from "./BehaviorContainer";
import { orderIndependentResources } from "../utils/displayAnalysis";
import ResourceDisplay from "./ResourceDisplay.tsx";
import CardIcon from "./CardIcon.tsx";
import OrChip from "./OrChip.tsx";
import Slash from "./Slash.tsx";
import GameIcon from "../../../display/GameIcon.tsx";
import {
  CalculatedOutputDto,
  CardBehaviorDto,
  SelectorDto,
  TriggerDto,
  MinMaxValueDto,
} from "@/types/generated/api-types.ts";
import { type ResourceCondition, getTileRestrictions, getPer } from "@/types/resourceConditions.ts";
import { getZIndex } from "@/constants/zIndex.ts";

interface IconDisplayInfo {
  resourceType: string;
  amount: number;
  displayMode: "individual" | "number";
  iconCount: number;
}

interface LayoutPlan {
  rows: IconDisplayInfo[][];
  separators: Array<{ position: number; type: string }>;
  totalRows: number;
}

interface TileScaleInfo {
  scale: 1 | 1.25 | 1.5 | 2;
  tileType: string | null;
}

interface TriggeredEffectLayoutProps {
  behavior: CardBehaviorDto;
  mergedBehaviors?: CardBehaviorDto[];
  layoutPlan: LayoutPlan;
  isResourceAffordable: (resource: ResourceCondition, isInput: boolean) => boolean;
  analyzeResourceDisplayWithConstraints: (
    resource: ResourceCondition,
    availableSpace: number,
    forceCompact: boolean,
  ) => IconDisplayInfo;
  tileScaleInfo: TileScaleInfo;
  computedOutputs?: CalculatedOutputDto[];
}

function PrintedCost({ constraint }: { constraint: MinMaxValueDto }) {
  return (
    <span className="flex items-center gap-0.5 text-white font-orbitron">
      {constraint.min !== undefined && (
        <>
          <GameIcon iconType="credit" amount={constraint.min} size="behavior" />
          <span>+</span>
        </>
      )}
      {constraint.max !== undefined && (
        <>
          <span>≤</span>
          <GameIcon iconType="credit" amount={constraint.max} size="behavior" />
        </>
      )}
    </span>
  );
}

// Extract requiredOriginalCost from selectors (new location) or condition level (legacy)
const getRequiredOriginalCost = (
  selectors: SelectorDto[] | undefined,
  conditionLevelCost: MinMaxValueDto | undefined,
): MinMaxValueDto | undefined => {
  if (selectors) {
    for (const sel of selectors) {
      if (sel.requiredOriginalCost) {
        return sel.requiredOriginalCost;
      }
    }
  }
  return conditionLevelCost;
};

function DependentProjectIcon() {
  return (
    <span
      className={"relative inline-flex items-center shrink-0 pr-2"}
      aria-label="A project with an Earth-connected effect"
      data-dependent-project
    >
      <GameIcon iconType="card" size="behavior" />
      <span className={"absolute right-0 bottom-0 inline-flex [--behavior-icon-size:14px]"}>
        <GameIcon iconType="connection" size="behavior" />
      </span>
    </span>
  );
}

// Render a single selector (AND logic: tags together, then card type)
const renderSelector = (
  selector: SelectorDto,
  selectorIndex: number,
  triggerIndex: number,
  redGlowClass: string,
): React.ReactNode => {
  const elements: React.ReactNode[] = [];

  if (selector.dependency) {
    elements.push(<DependentProjectIcon key={`dependency-${selectorIndex}`} />);
  }
  if (selector.tags && selector.tags.length > 0) {
    selector.tags.forEach((tag: string, tagIndex: number) => {
      elements.push(
        <GameIcon
          key={`tag-${triggerIndex}-${selectorIndex}-${tagIndex}`}
          iconType={`${tag}-tag`}
          size="behavior"
        />,
      );
    });
  }

  if (selector.cardTypes && selector.cardTypes.length > 0) {
    selector.cardTypes.forEach((cardType: string, typeIndex: number) => {
      if (cardType === "event") {
        elements.push(
          <GameIcon
            key={`type-${triggerIndex}-${selectorIndex}-${typeIndex}`}
            iconType="event"
            size="behavior"
          />,
        );
      } else {
        elements.push(
          <span
            key={`type-${triggerIndex}-${selectorIndex}-${typeIndex}`}
            className="text-xs font-semibold text-[#e0e0e0] capitalize [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]"
          >
            {cardType}
          </span>,
        );
      }
    });
  }

  if (selector.vp) {
    const vpLabel =
      selector.vp.min === 0 && !selector.vp.max
        ? "+ VP"
        : selector.vp.max != null && !selector.vp.min
          ? `≤${selector.vp.max} VP`
          : `≥${selector.vp.min} VP`;
    elements.push(
      <span
        key={`vp-${triggerIndex}-${selectorIndex}`}
        className="font-orbitron text-[10px] font-semibold text-white bg-black/40 border border-white/15 px-1.5 [text-shadow:0_0_2px_rgba(0,0,0,0.6)] leading-[22px]"
      >
        {vpLabel}
      </span>,
    );
  }

  return (
    <div
      key={`selector-${triggerIndex}-${selectorIndex}`}
      className={`flex gap-[2px] items-center ${redGlowClass}`}
    >
      {elements}
    </div>
  );
};

function StateTriggerSymbol({ state }: { state: string; value?: number }) {
  return <StateSymbol state={state} />;
}

// Render a single trigger icon based on its condition type
const renderTriggerIcon = (trigger: TriggerDto, triggerIndex: number): React.ReactNode => {
  if (trigger.condition?.type === "state-changed") {
    return (
      <span
        key={triggerIndex}
        className="inline-flex items-center gap-1"
        data-behavior-atom="state-trigger"
      >
        <StateTriggerSymbol state={trigger.condition.state ?? ""} value={trigger.condition.to} />
        {trigger.condition.direction && (
          <span className="font-orbitron font-bold">
            {trigger.condition.direction === "increase" ? "+" : "−"}
          </span>
        )}
      </span>
    );
  }
  if (trigger.condition?.type === "production") {
    return <GameIcon key={triggerIndex} iconType="production" size="behavior" />;
  }
  // Check if trigger has selectors (new system)
  const hasSelectors = trigger.condition?.selectors && trigger.condition.selectors.length > 0;

  // Qualify a single project with its symbol; explain broader restrictions in the tooltip.
  const isStandardProjectPlayed = trigger.condition?.type === "standard-project-played";
  if (isStandardProjectPlayed) {
    const projects =
      trigger.condition?.selectors?.flatMap((selector) => selector.standardProjects ?? []) ?? [];
    const badgeByProject: Record<string, string> = {
      "power-plant": "power-tag",
      greenery: "greenery-tile",
      aquifer: "ocean-tile",
      city: "city-tile",
      asteroid: "temperature",
    };
    const badges = projects.map((project) => badgeByProject[project]).filter(Boolean);
    if (projects.length > 1 || badges.length !== projects.length) {
      return (
        <span
          key={triggerIndex}
          className="inline-flex items-center gap-1"
          data-standard-project-exception
        >
          <GameIcon iconType="standard-project" size="behavior" />
          <span className="font-bold text-white">*</span>
        </span>
      );
    }
    return <BadgedGameIcon key={triggerIndex} icon="standard-project" badges={badges} />;
  }

  // Check if trigger is city-placed condition (e.g., Tharsis Republic)
  const isCityPlaced = trigger.condition?.type === "city-placed";

  // Handle global-parameter-raised condition (e.g., Aphrodite for venus)
  const isGlobalParameterRaised = trigger.condition?.type === "global-parameter-raised";

  if (isGlobalParameterRaised) {
    const paramToIcon: Record<string, string> = {
      temperature: "temperature",
      oxygen: "oxygen",
      ocean: "ocean-tile",
      venus: "venus",
    };
    const globalParams: string[] = trigger.condition?.selectors?.[0]?.globalParameters ?? [];

    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <div className="flex items-center gap-[2px]">
          {globalParams.map((param: string) => (
            <GameIcon key={param} iconType={paramToIcon[param] ?? param} size="behavior" />
          ))}
        </div>
      </div>
    );
  }

  // Handle placement-bonus-gained (e.g., Mining Guild): tile icon + selectors
  const isPlacementBonusGained = trigger.condition?.type === "placement-bonus-gained";
  if (isPlacementBonusGained) {
    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <GameIcon iconType="tile-placement" size="behavior" />
        {hasSelectors && (
          <span className="text-white font-bold text-sm [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
            *
          </span>
        )}
      </div>
    );
  }

  // Handle tag-played without selectors (any tag): show wild-tag icon
  const isTagPlayed = trigger.condition?.type === "tag-played";
  if (isTagPlayed && !hasSelectors) {
    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <GameIcon iconType="wild-tag" size="behavior" />
        {trigger.condition?.unique && (
          <span className="text-white font-bold text-sm [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
            *
          </span>
        )}
      </div>
    );
  }

  // Handle selectors first (new system with AND within selector, OR between selectors)
  if (hasSelectors) {
    const target = trigger.condition?.target || "self-player";
    const isAnyPlayer = target === "any-player";

    const redGlowClass = isAnyPlayer
      ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_2px_rgba(244,67,54,0.9))_drop-shadow(0_0_4px_rgba(244,67,54,0.7))]"
      : "";

    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center">
        {trigger.condition!.selectors!.map((selector, selectorIndex: number) => (
          <React.Fragment key={`${triggerIndex}-${selectorIndex}`}>
            {selectorIndex > 0 && <Slash />}
            {renderSelector(selector, selectorIndex, triggerIndex, redGlowClass)}
          </React.Fragment>
        ))}
        {trigger.condition?.unique && (
          <span className="text-white font-bold text-sm [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
            *
          </span>
        )}
      </div>
    );
  }

  if (isCityPlaced) {
    const target = trigger.condition?.target || "self-player";
    const isAnyPlayer = target === "any-player";

    const redGlowClass = isAnyPlayer
      ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_2px_rgba(244,67,54,0.9))_drop-shadow(0_0_4px_rgba(244,67,54,0.7))]"
      : "";

    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <div className={`flex items-center justify-center ${redGlowClass}`}>
          <GameIcon iconType="city-tile" size="behavior" />
        </div>
      </div>
    );
  }

  // Check if trigger is greenery-placed condition (e.g., Herbivores)
  const isGreeneryPlaced = trigger.condition?.type === "greenery-placed";

  if (isGreeneryPlaced) {
    const target = trigger.condition?.target || "self-player";
    const isAnyPlayer = target === "any-player";

    const redGlowClass = isAnyPlayer
      ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_2px_rgba(244,67,54,0.9))_drop-shadow(0_0_4px_rgba(244,67,54,0.7))]"
      : "";

    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <div className={`flex items-center justify-center ${redGlowClass}`}>
          <GameIcon iconType="greenery-tile" size="behavior" />
        </div>
      </div>
    );
  }

  // Check if trigger is ocean-placed condition (e.g., Arctic Algae)
  const isOceanPlaced = trigger.condition?.type === "ocean-placed";

  if (isOceanPlaced) {
    const target = trigger.condition?.target || "self-player";
    const isAnyPlayer = target === "any-player";

    const redGlowClass = isAnyPlayer
      ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_2px_rgba(244,67,54,0.9))_drop-shadow(0_0_4px_rgba(244,67,54,0.7))]"
      : "";

    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <div className={`flex items-center justify-center ${redGlowClass}`}>
          <GameIcon iconType="ocean-tile" size="behavior" />
        </div>
      </div>
    );
  }

  const isColonyPlaced = trigger.condition?.type === "colony-placed";

  if (isColonyPlaced) {
    const target = trigger.condition?.target || "self-player";
    const isAnyPlayer = target === "any-player";

    const redGlowClass = isAnyPlayer
      ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_2px_rgba(244,67,54,0.9))_drop-shadow(0_0_4px_rgba(244,67,54,0.7))]"
      : "";

    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <div className={`flex items-center justify-center ${redGlowClass}`}>
          <GameIcon iconType="colony" size="behavior" />
        </div>
      </div>
    );
  }

  // Handle tile-placed condition with onBonusType (e.g., Mining Rights)
  const isTilePlaced = trigger.condition?.type === "tile-placed";
  if (isTilePlaced && trigger.condition?.onBonusType) {
    const bonusTypes: string[] = trigger.condition.onBonusType;
    const target = trigger.condition?.target || "self-player";
    const isAnyPlayer = target === "any-player";

    const redGlowClass = isAnyPlayer
      ? "[filter:drop-shadow(0_1px_2px_rgba(0,0,0,0.5))_drop-shadow(0_0_2px_rgba(244,67,54,0.9))_drop-shadow(0_0_4px_rgba(244,67,54,0.7))]"
      : "";

    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <div className={`flex items-center gap-[2px] ${redGlowClass}`}>
          {bonusTypes.map((bonusType: string, idx: number) => (
            <React.Fragment key={`bonus-${triggerIndex}-${idx}`}>
              {idx > 0 && <Slash />}
              <GameIcon iconType={bonusType} size="behavior" />
            </React.Fragment>
          ))}
        </div>
      </div>
    );
  }

  // Check if trigger has requiredOriginalCost (from selectors or legacy condition level)
  const requiredOriginalCost = getRequiredOriginalCost(
    trigger.condition?.selectors,
    trigger.condition?.requiredOriginalCost,
  );
  if (requiredOriginalCost) {
    return <PrintedCost key={triggerIndex} constraint={requiredOriginalCost} />;
  }

  // Handle trading trigger (e.g., Trade Envoys, Trading Colony)
  if (trigger.condition?.type === "before-colony-trade") {
    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center justify-center">
        <GameIcon iconType="trade" size="behavior" />
      </div>
    );
  }

  // Handle production-increased trigger (e.g., Manutech partial group)
  const isProductionIncreased = trigger.condition?.type === "production-increased";
  if (isProductionIncreased) {
    const resourceTypes: string[] = trigger.condition?.resourceTypes ?? [];
    return (
      <div key={triggerIndex} className="flex gap-[2px] items-center">
        <div className="bg-[linear-gradient(135deg,rgba(160,110,60,0.4)_0%,rgba(139,89,42,0.35)_100%)] border border-[rgba(160,110,60,0.5)] px-1 py-[2px] shadow-[0_1px_3px_rgba(0,0,0,0.2)] flex items-center gap-[2px]">
          {resourceTypes.map((rt: string, idx: number) => {
            const iconType = PRODUCTION_TYPE_TO_RESOURCE[rt] ?? rt;
            return (
              <React.Fragment key={`prod-${triggerIndex}-${idx}`}>
                {idx > 0 && <Slash />}
                <GameIcon iconType={iconType} size="behavior" />
              </React.Fragment>
            );
          })}
        </div>
      </div>
    );
  }

  const triggerIcons: Record<string, string> = {
    "card-played": "card",
    "card-hand-updated": "card",
    "tile-placed": "tile-placement",
  };
  return (
    <GameIcon
      key={triggerIndex}
      iconType={triggerIcons[trigger.condition?.type ?? ""] ?? "effect"}
      size="behavior"
    />
  );
};

// Render a single behavior row (trigger : outputs)
const renderBehaviorRow = (
  behavior: CardBehaviorDto,
  rowIndex: number,
  isResourceAffordable: (resource: ResourceCondition, isInput: boolean) => boolean,
  analyzeResourceDisplayWithConstraints: (
    resource: ResourceCondition,
    availableSpace: number,
    forceCompact: boolean,
  ) => IconDisplayInfo,
  tileScaleInfo: TileScaleInfo,
  computedOutputs?: CalculatedOutputDto[],
  compact = false,
): React.ReactNode => {
  // Check if this is a global-parameter-lenience effect (special case)
  const isGlobalParameterLenience =
    behavior.outputs?.some((output) => output.type === "global-parameter-lenience") ?? false;

  const hasTriggers =
    !isGlobalParameterLenience &&
    behavior.triggers?.some((trigger) => trigger.condition !== undefined);

  const hasChoices = behavior.choices && behavior.choices.length > 0;
  const hasOutputs = behavior.outputs && behavior.outputs.length > 0;

  // When we have triggered choices, each choice row renders its own trigger,
  // so skip the main row if there are no behavior-level outputs
  const skipMainRow = hasTriggers && hasChoices && !hasOutputs;

  return (
    <React.Fragment key={`behavior-${rowIndex}`}>
      {!skipMainRow && (
        <div className="behavior-flow flex gap-[3px] items-center justify-center max-w-full">
          {/* Trigger conditions - hide for global-parameter-lenience */}
          {hasTriggers && behavior.triggers && (
            <>
              <div className="flex gap-[3px] items-center">
                {(() => {
                  // Check if any trigger has requiredOriginalCost (from selectors or condition level)
                  const triggersWithCost = behavior.triggers.filter(
                    (trigger) =>
                      getRequiredOriginalCost(
                        trigger.condition?.selectors,
                        trigger.condition?.requiredOriginalCost,
                      ) !== undefined,
                  );

                  // If we have cost-based triggers, deduplicate and show once
                  if (triggersWithCost.length > 0) {
                    const uniqueCosts = new Map<string, MinMaxValueDto>();
                    for (const trigger of triggersWithCost) {
                      const cost = getRequiredOriginalCost(
                        trigger.condition?.selectors,
                        trigger.condition?.requiredOriginalCost,
                      );
                      if (cost) {
                        uniqueCosts.set(JSON.stringify(cost), cost);
                      }
                    }
                    return [...uniqueCosts].map(([key, constraint]) => (
                      <PrintedCost key={key} constraint={constraint} />
                    ));
                  }

                  // Otherwise, render other trigger types normally
                  return behavior.triggers.map((trigger, triggerIndex: number) =>
                    renderTriggerIcon(trigger, triggerIndex),
                  );
                })()}
              </div>
              <span
                className="flex items-center justify-center text-white text-base font-bold [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] min-w-[8px] shrink-0"
                style={{ zIndex: getZIndex("LOCAL", 1) }}
              >
                :
              </span>
            </>
          )}

          {behavior.inputs?.map((input, index) => {
            if (input.type === "card-discard") {
              return (
                <CardIcon
                  key={`triggered-input-${rowIndex}-${index}`}
                  amount={input.amount}
                  badgeType="discard"
                  isAffordable={isResourceAffordable(input, true)}
                  totalCardTypes={1}
                />
              );
            }
            return (
              <ResourceDisplay
                key={`triggered-input-${rowIndex}-${index}`}
                displayInfo={analyzeResourceDisplayWithConstraints(input, 6, false)}
                isInput
                resource={input}
                isGroupedWithOtherNegatives={false}
                context="action"
                isAffordable={isResourceAffordable(input, true)}
                tileScaleInfo={tileScaleInfo}
              />
            );
          })}
          {(behavior.inputs?.length ?? 0) > 0 && (behavior.outputs?.length ?? 0) > 0 && (
            <BehaviorArrow />
          )}

          {/* Outputs in same row if they fit */}
          {behavior.outputs &&
            orderIndependentResources(behavior.outputs, compact).map((output, index: number) => {
              const resourceType = output.type;
              const cardOutputTypes = [
                "card-draw",
                "card-peek",
                "card-take",
                "card-buy",
                "card-discard",
              ];
              if (cardOutputTypes.includes(resourceType)) {
                const badgeType =
                  resourceType === "card-peek"
                    ? "peek"
                    : resourceType === "card-take"
                      ? "take"
                      : resourceType === "card-buy"
                        ? "buy"
                        : resourceType === "card-discard"
                          ? "discard"
                          : "none";
                const isAttack =
                  output.target === "any-player" ||
                  output.target === "all-opponents" ||
                  output.target?.startsWith("steal-");
                return (
                  <CardIcon
                    key={`triggered-output-${rowIndex}-${index}`}
                    amount={Math.abs(output.amount || 1)}
                    badgeType={badgeType}
                    isAffordable={isResourceAffordable(output, false)}
                    isAttack={isAttack}
                    totalCardTypes={1}
                  />
                );
              }
              const displayInfo = analyzeResourceDisplayWithConstraints(output, 6, false);
              return (
                <React.Fragment key={`triggered-output-${rowIndex}-${index}`}>
                  <ResourceDisplay
                    displayInfo={displayInfo}
                    isInput={false}
                    resource={output}
                    isGroupedWithOtherNegatives={false}
                    context="default"
                    isAffordable={isResourceAffordable(output, false)}
                    tileScaleInfo={tileScaleInfo}
                    computedOutputs={computedOutputs}
                  />
                </React.Fragment>
              );
            })}

          {(behavior.generationalEventRequirements?.length ?? 0) > 0 && (
            <span className="text-white font-bold text-sm ml-1">*</span>
          )}
        </div>
      )}

      {/* Triggered effect with choices */}
      {behavior.choices &&
        behavior.choices.length > 0 &&
        hasTriggers &&
        (() => {
          const cardOutputTypes = ["card-draw", "card-peek", "card-take", "card-buy"];
          const isSimpleChoices = behavior.choices.every(
            (c) =>
              (!c.inputs || c.inputs.length === 0) &&
              c.outputs?.every((o) => !cardOutputTypes.includes(o.type)),
          );

          if (isSimpleChoices) {
            // Simple output-only choices: two rows — triggers on top, outputs below
            return (
              <>
                <div className="behavior-flow flex gap-[3px] items-center justify-center max-w-full">
                  <div className="flex gap-[3px] items-center">
                    {behavior.triggers!.map((trigger, triggerIndex: number) =>
                      renderTriggerIcon(trigger, triggerIndex),
                    )}
                  </div>
                  <span
                    className="flex items-center justify-center text-white text-base font-bold [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] min-w-[8px] shrink-0"
                    style={{ zIndex: getZIndex("LOCAL", 1) }}
                  >
                    :
                  </span>
                </div>
                <div className="behavior-flow flex gap-[3px] items-center justify-center max-w-full">
                  {behavior.choices.map((choice, idx: number) => (
                    <React.Fragment key={`choice-${rowIndex}-${idx}`}>
                      {idx > 0 && <Slash />}
                      <OperationRequirements stateRequirements={choice.stateRequirements} />
                      {choice.outputs?.map((output, outputIndex: number) => {
                        const displayInfo = analyzeResourceDisplayWithConstraints(output, 6, false);
                        return (
                          <ResourceDisplay
                            key={`choice-${rowIndex}-${idx}-output-${outputIndex}`}
                            displayInfo={displayInfo}
                            isInput={false}
                            resource={output}
                            isGroupedWithOtherNegatives={false}
                            context="default"
                            isAffordable={isResourceAffordable(output, false)}
                            tileScaleInfo={tileScaleInfo}
                          />
                        );
                      })}
                    </React.Fragment>
                  ))}
                </div>
              </>
            );
          }

          // Complex choices (has inputs or card outputs): each choice on its own row
          return behavior.choices.map((choice, idx: number) => (
            <div
              key={`choice-row-${rowIndex}-${idx}`}
              className="behavior-flow flex gap-[3px] items-center justify-center max-w-full"
            >
              <div className="flex gap-[3px] items-center">
                {behavior.triggers!.map((trigger, triggerIndex: number) =>
                  renderTriggerIcon(trigger, triggerIndex),
                )}
              </div>
              <span
                className="flex items-center justify-center text-white text-base font-bold [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] min-w-[8px] shrink-0"
                style={{ zIndex: getZIndex("LOCAL", 1) }}
              >
                :
              </span>
              <OperationRequirements stateRequirements={choice.stateRequirements} />
              {choice.inputs?.map((input, inputIndex: number) => {
                const displayInfo = analyzeResourceDisplayWithConstraints(input, 6, false);
                return (
                  <ResourceDisplay
                    key={`choice-${rowIndex}-${idx}-input-${inputIndex}`}
                    displayInfo={displayInfo}
                    isInput={true}
                    resource={input}
                    isGroupedWithOtherNegatives={false}
                    context="action"
                    isAffordable={isResourceAffordable(input, true)}
                    tileScaleInfo={tileScaleInfo}
                  />
                );
              })}
              {(choice.inputs?.length ?? 0) > 0 && (choice.outputs?.length ?? 0) > 0 && (
                <BehaviorArrow />
              )}
              {choice.outputs?.map((output, outputIndex: number) => {
                const resourceType = output.type;
                const isCardResource = cardOutputTypes.includes(resourceType);

                if (isCardResource) {
                  const badgeType =
                    resourceType === "card-peek"
                      ? "peek"
                      : resourceType === "card-take"
                        ? "take"
                        : resourceType === "card-buy"
                          ? "buy"
                          : resourceType === "card-discard"
                            ? "discard"
                            : "none";
                  const isAttack =
                    output.target === "any-player" ||
                    output.target === "all-opponents" ||
                    output.target?.startsWith("steal-");
                  return (
                    <CardIcon
                      key={`choice-${rowIndex}-${idx}-output-${outputIndex}`}
                      amount={Math.abs(output.amount || 1)}
                      badgeType={badgeType}
                      isAffordable={isResourceAffordable(output, false)}
                      isAttack={isAttack}
                      totalCardTypes={1}
                    />
                  );
                }

                const displayInfo = analyzeResourceDisplayWithConstraints(output, 6, false);
                return (
                  <ResourceDisplay
                    key={`choice-${rowIndex}-${idx}-output-${outputIndex}`}
                    displayInfo={displayInfo}
                    isInput={false}
                    resource={output}
                    isGroupedWithOtherNegatives={false}
                    context="default"
                    isAffordable={isResourceAffordable(output, false)}
                    tileScaleInfo={tileScaleInfo}
                  />
                );
              })}
              {idx < behavior.choices!.length - 1 && <OrChip />}
            </div>
          ));
        })()}

      {/* Non-triggered choices: keep existing single-row layout */}
      {behavior.choices && behavior.choices.length > 0 && !hasTriggers && (
        <div className="flex gap-[6px] items-center justify-center">
          {behavior.choices.map((choice, idx: number) => (
            <React.Fragment key={`choice-${rowIndex}-${idx}`}>
              {idx > 0 && <Slash />}
              <OperationRequirements stateRequirements={choice.stateRequirements} />
              {choice.inputs?.map((input, inputIndex: number) => {
                const displayInfo = analyzeResourceDisplayWithConstraints(input, 6, false);
                return (
                  <ResourceDisplay
                    key={`choice-${rowIndex}-${idx}-input-${inputIndex}`}
                    displayInfo={displayInfo}
                    isInput={true}
                    resource={input}
                    isGroupedWithOtherNegatives={false}
                    context="action"
                    isAffordable={isResourceAffordable(input, true)}
                    tileScaleInfo={tileScaleInfo}
                  />
                );
              })}
              {(choice.inputs?.length ?? 0) > 0 && (choice.outputs?.length ?? 0) > 0 && (
                <BehaviorArrow />
              )}
              {choice.outputs?.map((output, outputIndex: number) => {
                const displayInfo = analyzeResourceDisplayWithConstraints(output, 6, false);
                return (
                  <ResourceDisplay
                    key={`choice-${rowIndex}-${idx}-output-${outputIndex}`}
                    displayInfo={displayInfo}
                    isInput={false}
                    resource={output}
                    isGroupedWithOtherNegatives={false}
                    context="default"
                    isAffordable={isResourceAffordable(output, false)}
                    tileScaleInfo={tileScaleInfo}
                  />
                );
              })}
            </React.Fragment>
          ))}
        </div>
      )}
    </React.Fragment>
  );
};

const isTilePlacementWithBonusProductionPattern = (allBehaviors: CardBehaviorDto[]): boolean => {
  const hasTilePlacement = allBehaviors.some((b) =>
    b.outputs?.some((o) => o.type === "tile-placement"),
  );
  const hasTilePlacedTrigger = allBehaviors.some(
    (b) => b.triggers?.[0]?.condition?.type === "tile-placed",
  );
  return hasTilePlacement && hasTilePlacedTrigger;
};

const ALL_PRODUCTION_TYPES = new Set([
  "credit-production",
  "steel-production",
  "titanium-production",
  "plant-production",
  "energy-production",
  "heat-production",
]);

const isAllStandardResourceProductionPattern = (allBehaviors: CardBehaviorDto[]): boolean => {
  if (allBehaviors.length < 6) {
    return false;
  }
  const coveredTypes = new Set<string>();
  for (const b of allBehaviors) {
    const trigger = b.triggers?.[0];
    if (trigger?.condition?.type !== "production-increased") {
      return false;
    }
    const resourceTypes: string[] = trigger.condition.resourceTypes ?? [];
    if (resourceTypes.length !== 1) {
      return false;
    }
    if (!ALL_PRODUCTION_TYPES.has(resourceTypes[0])) {
      return false;
    }
    const outputs = b.outputs ?? [];
    if (
      outputs.length !== 1 ||
      outputs[0].type !== resourceTypes[0].replace("-production", "") ||
      outputs[0].amount !== 1 ||
      outputs[0].target !== "self-player" ||
      b.inputs?.length ||
      b.choices?.length ||
      b.optional ||
      b.stateRequirements?.length
    ) {
      return false;
    }
    coveredTypes.add(resourceTypes[0]);
  }
  return coveredTypes.size === 6;
};

const PRODUCTION_TYPE_TO_RESOURCE: Record<string, string> = {
  "credit-production": "credit",
  "steel-production": "steel",
  "titanium-production": "titanium",
  "plant-production": "plant",
  "energy-production": "energy",
  "heat-production": "heat",
};

const TriggeredEffectLayout: React.FC<TriggeredEffectLayoutProps> = ({
  behavior,
  mergedBehaviors,
  layoutPlan: _layoutPlan,
  isResourceAffordable,
  analyzeResourceDisplayWithConstraints,
  tileScaleInfo,
  computedOutputs,
}) => {
  const { compact } = useBehaviorLayout();
  const storage = useCardResourceStorage();
  const removal = behavior.outputs?.find(
    (output) =>
      output.target === "self-card" && output.type === storage?.type && output.amount === -1,
  );
  const gain = behavior.outputs?.find(
    (output) => output.target === "self-player" && output.amount === 1,
  );
  const removalPer = removal && getPer(removal);
  const gainPer = gain && getPer(gain);
  if (
    storage &&
    removal &&
    gain &&
    behavior.outputs?.length === 2 &&
    !mergedBehaviors?.length &&
    !behavior.inputs?.length &&
    !behavior.choices?.length &&
    removalPer?.target === "self-card" &&
    removalPer.type === storage.type &&
    removalPer.amount === 1 &&
    JSON.stringify(removalPer) === JSON.stringify(gainPer)
  ) {
    return (
      <div
        className="behavior-flow inline-flex items-center justify-center gap-1 max-w-full"
        data-storage-exchange
      >
        {behavior.triggers?.map((trigger, index) => renderTriggerIcon(trigger, index))}
        <span>:</span>
        <span className="font-orbitron text-[11px]">X</span>
        <GameIcon iconType={storage.icon ?? storage.type} size="behavior" />
        <BehaviorArrow />
        <span className="font-orbitron text-[11px]">X</span>
        <GameIcon iconType={gain.type} size="behavior" />
      </div>
    );
  }
  // Collect all behaviors to render (primary + merged)
  const allBehaviors = [behavior, ...(mergedBehaviors || [])];

  // Compact rendering for all-6-standard-resource production-increased pattern (Manutech)
  if (isAllStandardResourceProductionPattern(allBehaviors)) {
    return (
      <div className="behavior-flow flex gap-[3px] items-center justify-center max-w-full">
        <div className="bg-[linear-gradient(135deg,rgba(160,110,60,0.4)_0%,rgba(139,89,42,0.35)_100%)] border border-[rgba(160,110,60,0.5)] px-1.5 py-[3px] shadow-[0_1px_3px_rgba(0,0,0,0.2)] flex items-center">
          <GameIcon iconType="resources" size="behavior" />
        </div>
        <span
          className="flex items-center justify-center text-white text-base font-bold [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] min-w-[8px] shrink-0"
          style={{ zIndex: getZIndex("LOCAL", 1) }}
        >
          :
        </span>
        <GameIcon iconType="resources" size="behavior" />
      </div>
    );
  }

  // Compact rendering for tile-placement + conditional production pattern (Mining Area/Rights)
  if (isTilePlacementWithBonusProductionPattern(allBehaviors)) {
    const tileBehavior = allBehaviors.find((b) =>
      b.outputs?.some((o) => o.type === "tile-placement"),
    );
    const tileOutput = tileBehavior?.outputs?.find((o) => o.type === "tile-placement");
    const tileRestrictions = tileOutput ? getTileRestrictions(tileOutput) : undefined;
    const hasBonusRestriction = (tileRestrictions?.onBonusType?.length ?? 0) > 0;
    const triggeredBehaviors = allBehaviors.filter(
      (b) => b.triggers?.[0]?.condition?.type === "tile-placed",
    );

    return (
      <div className="flex flex-col gap-1 items-center justify-center">
        <div className="flex gap-[2px] items-center justify-center">
          <GameIcon iconType="mine" size="behavior" />
          {hasBonusRestriction && (
            <span className="text-white font-bold text-sm [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
              *
            </span>
          )}
        </div>
        {triggeredBehaviors.map((b, index) =>
          renderBehaviorRow(
            b,
            index,
            isResourceAffordable,
            analyzeResourceDisplayWithConstraints,
            tileScaleInfo,
            computedOutputs,
            compact,
          ),
        )}
      </div>
    );
  }

  // For production-increased triggers, render resource icons inside production box
  return (
    <div className="flex flex-col gap-2 items-center justify-center">
      {allBehaviors.map((b, index) =>
        renderBehaviorRow(
          b,
          index,
          isResourceAffordable,
          analyzeResourceDisplayWithConstraints,
          tileScaleInfo,
          computedOutputs,
          compact,
        ),
      )}
    </div>
  );
};

export default TriggeredEffectLayout;
