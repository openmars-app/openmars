import GameIcon from "../../../display/GameIcon.tsx";
import { OperationRequirements } from "./BehaviorQualifiers.tsx";
import { BehaviorArrow } from "./BehaviorIcon";
import React from "react";
import ResourceDisplay from "./ResourceDisplay.tsx";
import CardIcon from "./CardIcon.tsx";
import OrChip from "./OrChip.tsx";
import { useBehaviorLayout } from "./BehaviorContainer";
import { orderIndependentResources, analyzeCardOutputs } from "../utils/displayAnalysis.ts";
import { CalculatedOutputDto, CardBehaviorDto } from "@/types/generated/api-types.ts";
import {
  type ResourceCondition,
  isCardOperation,
  isProduction as isProductionType,
} from "@/types/resourceConditions.ts";

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

interface ManualActionLayoutProps {
  behavior: CardBehaviorDto;
  layoutPlan: LayoutPlan;
  isResourceAffordable: (resource: ResourceCondition, isInput: boolean) => boolean;
  analyzeResourceDisplayWithConstraints: (
    resource: ResourceCondition,
    availableSpace: number,
    forceCompact: boolean,
  ) => IconDisplayInfo;
  tileScaleInfo: TileScaleInfo;
  hideActionChip?: boolean;
  computedOutputs?: CalculatedOutputDto[];
}

const isCardResourceType = (resource: ResourceCondition): boolean => isCardOperation(resource);

const getCardBadgeType = (type: string): "peek" | "take" | "buy" | "discard" | "none" =>
  type === "card-peek"
    ? "peek"
    : type === "card-take"
      ? "take"
      : type === "card-buy"
        ? "buy"
        : type === "card-discard"
          ? "discard"
          : "none";

const isAttackTarget = (target: string | undefined): boolean =>
  target === "any-player" || target === "all-opponents" || (target?.startsWith("steal-") ?? false);

const isAnyProductionChoicePattern = (behavior: CardBehaviorDto): boolean => {
  if (!behavior.choices || behavior.choices.length < 4) return false;
  return behavior.choices.every((choice) => {
    if (choice.stateRequirements?.length || choice.requirements?.items.length) {
      return false;
    }
    if (choice.inputs && choice.inputs.length > 0) return false;
    if (!choice.outputs || choice.outputs.length !== 1) return false;
    const output = choice.outputs[0];
    return isProductionType(output) && (output.amount === 1 || output.amount === -1);
  });
};

function standardResourceChoice(behavior: CardBehaviorDto) {
  const resources = ["credit", "steel", "titanium", "plant", "energy", "heat"];
  const choices = behavior.choices;
  if (choices?.length !== resources.length || behavior.outputs?.length) {
    return undefined;
  }
  const matched = new Set<string>();
  let pattern: "production" | "exchange" | undefined;
  let productionAmount: number | undefined;
  let resourceAmount: number | undefined;
  for (const choice of choices) {
    if (
      choice.inputs?.length ||
      choice.stateRequirements?.length ||
      choice.requirements?.items.length
    ) {
      return undefined;
    }
    const outputs = choice.outputs ?? [];
    const production = outputs.find((output) => output.type.endsWith("-production"));
    if (
      !production ||
      ![1, -1].includes(production.amount) ||
      production.target !== "self-player"
    ) {
      return undefined;
    }
    const resource = production.type.replace("-production", "");
    if (!resources.includes(resource) || matched.has(resource)) {
      return undefined;
    }
    const gain = outputs.find((output) => output.type === resource);
    const kind = gain ? "exchange" : "production";
    if (
      outputs.length !== (gain ? 2 : 1) ||
      (gain &&
        (production.amount !== -1 ||
          gain.amount <= 0 ||
          gain.target !== "self-player" ||
          behavior.inputs?.length)) ||
      (pattern && pattern !== kind) ||
      (productionAmount !== undefined && productionAmount !== production.amount) ||
      (resourceAmount !== undefined && resourceAmount !== gain?.amount) ||
      outputs.some((output) =>
        Object.keys(output).some((key) => !["type", "amount", "target"].includes(key)),
      )
    ) {
      return undefined;
    }
    pattern = kind;
    productionAmount = production.amount;
    resourceAmount = gain?.amount;
    matched.add(resource);
  }
  return { kind: pattern, productionAmount, resourceAmount };
}

const renderChoiceOutputs = (
  outputs: ResourceCondition[] | undefined,
  choiceIndex: number,
  isResourceAffordable: (resource: ResourceCondition, isInput: boolean) => boolean,
  analyzeResourceDisplayWithConstraints: (
    resource: ResourceCondition,
    availableSpace: number,
    forceCompact: boolean,
  ) => IconDisplayInfo,
  tileScaleInfo: TileScaleInfo,
  computedOutputs?: CalculatedOutputDto[],
  numbered = false,
): React.ReactNode => {
  if (!outputs) return null;
  return outputs.map((output, outputIndex) => {
    const resourceType = output.type || "";
    if (isCardResourceType(output)) {
      return (
        <CardIcon
          key={`choice-${choiceIndex}-output-${outputIndex}`}
          amount={Math.abs(output.amount ?? 1)}
          badgeType={getCardBadgeType(resourceType)}
          isAffordable={isResourceAffordable(output, false)}
          isAttack={isAttackTarget(output.target)}
          totalCardTypes={1}
        />
      );
    }
    const displayInfo = analyzeResourceDisplayWithConstraints(output, 3, numbered);
    return (
      <ResourceDisplay
        key={`choice-${choiceIndex}-output-${outputIndex}`}
        displayInfo={displayInfo}
        isInput={false}
        resource={output}
        isGroupedWithOtherNegatives={false}
        context="action"
        isAffordable={isResourceAffordable(output, false)}
        tileScaleInfo={tileScaleInfo}
        computedOutputs={computedOutputs}
      />
    );
  });
};

const ManualActionLayout: React.FC<ManualActionLayoutProps> = ({
  behavior,
  layoutPlan: _layoutPlan,
  isResourceAffordable,
  analyzeResourceDisplayWithConstraints,
  tileScaleInfo,
  hideActionChip = false,
  computedOutputs,
}) => {
  const { compact } = useBehaviorLayout();
  const standardChoice = standardResourceChoice(behavior);
  if (standardChoice) {
    return (
      <div
        className="behavior-flow flex items-center justify-center gap-2"
        data-standard-choice={standardChoice.kind}
        data-choice-indices={behavior.choices?.map((_, index) => index).join(",")}
      >
        {behavior.inputs?.map((input, index) => (
          <ResourceDisplay
            key={index}
            resource={input}
            displayInfo={analyzeResourceDisplayWithConstraints(input, 3, false)}
            isInput
            isGroupedWithOtherNegatives={false}
            context="action"
            isAffordable={isResourceAffordable(input, true)}
            tileScaleInfo={tileScaleInfo}
          />
        ))}
        {standardChoice.kind === "production" && Boolean(behavior.inputs?.length) && (
          <BehaviorArrow />
        )}
        {standardChoice.productionAmount === -1 && <span>−</span>}
        <GameIcon iconType="standard-resource-production" size="behavior" />
        {standardChoice.kind === "exchange" && (
          <>
            <BehaviorArrow />
            <span>{standardChoice.resourceAmount}</span>
            <GameIcon iconType="standard-resource" size="behavior" />
          </>
        )}
        {(standardChoice.kind === "exchange" || behavior.choicePolicy) && (
          <span data-rule-exception className="font-bold">
            *
          </span>
        )}
      </div>
    );
  }
  const choices = (behavior.choices ?? []).filter(
    (choice) => (choice.inputs?.length ?? 0) + (choice.outputs?.length ?? 0) > 0,
  );
  // Handle choice-based behaviors
  if (choices.length > 0) {
    // Compact rendering for "any standard production +1" patterns (e.g. Robinson Industries)
    if (isAnyProductionChoicePattern(behavior)) {
      const hasInputs = behavior.inputs && behavior.inputs.length > 0;
      return (
        <div className="behavior-flow flex items-center justify-center gap-2 w-full">
          {hasInputs ? (
            <div className="flex items-center gap-[3px]">
              {behavior.inputs!.map((input, inputIndex: number) => {
                const displayInfo = analyzeResourceDisplayWithConstraints(input, 3, false);
                return (
                  <ResourceDisplay
                    key={`input-${inputIndex}`}
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
            </div>
          ) : !hideActionChip ? (
            <GameIcon iconType="action" size="behavior" />
          ) : null}
          <BehaviorArrow />
          <div className="bg-[linear-gradient(135deg,rgba(160,110,60,0.4)_0%,rgba(139,89,42,0.35)_100%)] border border-[rgba(160,110,60,0.5)] px-1.5 py-[3px] shadow-[0_1px_3px_rgba(0,0,0,0.2)] flex items-center">
            <GameIcon iconType="resources" size="behavior" />
          </div>
          {behavior.choicePolicy && (
            <span className="text-white font-bold text-sm [text-shadow:1px_1px_2px_rgba(0,0,0,0.6)]">
              *
            </span>
          )}
        </div>
      );
    }

    // Check if choices only have inputs (no outputs) and behavior has outputs
    const choicesOnlyHaveInputs = choices.every(
      (choice) => !choice.outputs || choice.outputs.length === 0,
    );
    const behaviorHasOutputs = behavior.outputs && behavior.outputs.length > 0;

    // Special case: choices with only inputs + behavior-level outputs
    // Format: <input1> / <input2> -> <outputs>
    if (choicesOnlyHaveInputs && behaviorHasOutputs) {
      return (
        <div className="behavior-flow flex items-center justify-center gap-2 w-full">
          {/* All choice inputs with "/" separator */}
          <div className="flex items-center gap-[6px]">
            {choices.map((choice, choiceIndex) => (
              <React.Fragment key={`choice-input-${choiceIndex}`}>
                {choiceIndex > 0 && <OrChip />}
                <div className="flex gap-[3px] items-center">
                  <OperationRequirements stateRequirements={choice.stateRequirements} />
                  {choice.inputs &&
                    choice.inputs.map((input, inputIndex: number) => {
                      const displayInfo = analyzeResourceDisplayWithConstraints(input, 3, false);
                      return (
                        <React.Fragment key={`choice-${choiceIndex}-input-${inputIndex}`}>
                          <ResourceDisplay
                            displayInfo={displayInfo}
                            isInput={true}
                            resource={input}
                            isGroupedWithOtherNegatives={false}
                            context="action"
                            isAffordable={isResourceAffordable(input, true)}
                            tileScaleInfo={tileScaleInfo}
                          />
                        </React.Fragment>
                      );
                    })}
                </div>
              </React.Fragment>
            ))}
          </div>

          {/* Arrow separator */}
          <BehaviorArrow />

          {/* Behavior-level outputs */}
          <div className="flex flex-col gap-0.5 items-center min-w-0">
            {behavior.outputs &&
              behavior.outputs.map((output, outputIndex: number) => {
                const displayInfo = analyzeResourceDisplayWithConstraints(output, 3, false);
                return (
                  <React.Fragment key={`output-${outputIndex}`}>
                    <ResourceDisplay
                      displayInfo={displayInfo}
                      isInput={false}
                      resource={output}
                      isGroupedWithOtherNegatives={false}
                      context="action"
                      isAffordable={isResourceAffordable(output, false)}
                      tileScaleInfo={tileScaleInfo}
                    />
                  </React.Fragment>
                );
              })}
          </div>

          {(behavior.generationalEventRequirements?.length ?? 0) > 0 && (
            <span className="text-white font-bold text-sm ml-1">*</span>
          )}
        </div>
      );
    }

    return (
      <div
        data-action-choices
        className="flex flex-wrap items-center justify-center gap-x-1 gap-y-1.5 max-w-full min-w-0"
      >
        {Boolean(behavior.inputs?.length) && (
          <div data-shared-choice-cost className="inline-flex items-center gap-1 shrink-0">
            {behavior.inputs?.map((input, inputIndex) => (
              <ResourceDisplay
                key={`shared-input-${inputIndex}`}
                displayInfo={analyzeResourceDisplayWithConstraints(input, 3, false)}
                isInput
                resource={input}
                isGroupedWithOtherNegatives={false}
                context="action"
                isAffordable={isResourceAffordable(input, true)}
                tileScaleInfo={tileScaleInfo}
              />
            ))}
            <BehaviorArrow />
          </div>
        )}
        {choices.map((choice, choiceIndex) => (
          <div
            key={`choice-${choiceIndex}`}
            data-action-choice={choiceIndex}
            className="inline-flex items-center justify-center gap-1 max-w-full"
          >
            {choiceIndex > 0 && <OrChip />}
            <OperationRequirements stateRequirements={choice.stateRequirements} />
            <div className="behavior-flow flex items-center gap-1 max-w-full">
              {choice.inputs?.map((input, inputIndex) => (
                <ResourceDisplay
                  key={`choice-${choiceIndex}-input-${inputIndex}`}
                  displayInfo={analyzeResourceDisplayWithConstraints(input, 3, false)}
                  isInput
                  resource={input}
                  isGroupedWithOtherNegatives={false}
                  context="action"
                  isAffordable={isResourceAffordable(input, true)}
                  tileScaleInfo={tileScaleInfo}
                />
              ))}
              {(choice.inputs?.length ?? 0) > 0 && (choice.outputs?.length ?? 0) > 0 && (
                <BehaviorArrow />
              )}
              {renderChoiceOutputs(
                choice.outputs,
                choiceIndex,
                isResourceAffordable,
                analyzeResourceDisplayWithConstraints,
                tileScaleInfo,
                computedOutputs,
                choices.length >= 3,
              )}
            </div>
          </div>
        ))}
        {(behavior.generationalEventRequirements?.length ?? 0) > 0 && (
          <span className="font-bold text-sm ml-1">*</span>
        )}
      </div>
    );
  }

  // Check for action-reuse output (e.g., Viron)
  const hasActionReuse =
    behavior.outputs && behavior.outputs.some((o) => o.type === "action-reuse");
  if (hasActionReuse) {
    return (
      <div className="behavior-flow flex items-center justify-center gap-1 w-full">
        {!hideActionChip && <GameIcon iconType="action" size="behavior" />}
        <div className="bg-white/[0.08] border border-white/20 shadow-[0_1px_3px_rgba(0,0,0,0.15)] px-2 py-1 flex items-center justify-center">
          <GameIcon iconType="action-reuse" size="behavior" />
        </div>
      </div>
    );
  }

  // Regular behavior handling
  // Analyze and consolidate card outputs (card-draw, card-peek, card-take, card-buy)
  const consolidatedCards = behavior.outputs ? analyzeCardOutputs(behavior.outputs) : [];

  // Helper to check if an output is a card resource
  const isCardResource = (output: ResourceCondition): boolean => isCardResourceType(output);

  // Filter out card resources from regular outputs (they'll be rendered via consolidatedCards)
  const nonCardOutputs = behavior.outputs
    ? behavior.outputs.filter((output) => !isCardResource(output))
    : [];

  const hasInputs = behavior.inputs && behavior.inputs.length > 0;

  return (
    <div className="behavior-flow flex items-center justify-center gap-1 w-full">
      {/* Input side */}
      <div className="flex flex-col gap-0.5 items-center min-w-0">
        {hasInputs ? (
          behavior.inputs!.map((input, inputIndex: number) => {
            const displayInfo = analyzeResourceDisplayWithConstraints(input, 3, false);
            return (
              <React.Fragment key={`input-${inputIndex}`}>
                <ResourceDisplay
                  displayInfo={displayInfo}
                  isInput={true}
                  resource={input}
                  isGroupedWithOtherNegatives={false}
                  context="action"
                  isAffordable={isResourceAffordable(input, true)}
                  tileScaleInfo={tileScaleInfo}
                />
              </React.Fragment>
            );
          })
        ) : !hideActionChip ? (
          <GameIcon iconType="action" size="behavior" />
        ) : null}
      </div>

      {/* Arrow separator - only show when there are inputs */}
      {hasInputs && <BehaviorArrow />}

      {/* Output side */}
      <div className="flex flex-row flex-wrap gap-0.5 items-center min-w-0">
        {/* Regular non-card outputs */}
        {orderIndependentResources(nonCardOutputs, compact).map((output, outputIndex: number) => {
          const displayInfo = analyzeResourceDisplayWithConstraints(output, 3, false);
          return (
            <React.Fragment key={`output-${outputIndex}`}>
              <ResourceDisplay
                displayInfo={displayInfo}
                isInput={false}
                resource={output}
                isGroupedWithOtherNegatives={false}
                context="action"
                isAffordable={isResourceAffordable(output, false)}
                tileScaleInfo={tileScaleInfo}
                computedOutputs={computedOutputs}
              />
            </React.Fragment>
          );
        })}

        {/* Consolidated card icons (card-draw, card-peek, card-take, card-buy) */}
        {consolidatedCards.map((cardItem, index) => (
          <React.Fragment key={`card-${index}`}>
            <CardIcon
              amount={cardItem.amount}
              keepAmount={cardItem.keepAmount}
              badgeType={cardItem.badgeType}
              isAffordable={true}
              isAttack={cardItem.isAttack}
              totalCardTypes={consolidatedCards.length}
            />
          </React.Fragment>
        ))}
      </div>

      {(behavior.generationalEventRequirements?.length ?? 0) > 0 && (
        <span className="text-white font-bold text-sm ml-1">*</span>
      )}
    </div>
  );
};

export default ManualActionLayout;
