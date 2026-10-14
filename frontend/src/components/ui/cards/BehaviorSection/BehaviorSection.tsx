import { useStateVisualLabel, CardSourcePresentation } from "../CardStateVisuals.tsx";
import OrChip from "./components/OrChip.tsx";
import BehaviorQualifiers, { OperationRequirements } from "./components/BehaviorQualifiers.tsx";
import React, { useMemo } from "react";
import { BehaviorSectionProps, ClassifiedBehavior, BehaviorPresentation } from "./types.ts";
import { CalculatedOutputDto, PerConditionDto } from "@/types/generated/api-types.ts";
import { getPer } from "@/types/resourceConditions.ts";
import { classifyBehaviors } from "./utils/behaviorClassifier.ts";
import { detectTilePlacementScale } from "./utils/tileScaling.ts";
import { isResourceAffordable } from "./utils/resourceValidation.ts";
import { analyzeResourceDisplayWithConstraints } from "./utils/displayAnalysis.ts";
import {
  mergeAutoProductionBehaviors,
  mergeTriggeredEffects,
  groupBehaviorPresentation,
} from "./utils/behaviorMerger.ts";
import { createLayoutPlan } from "./utils/layoutCalculator.ts";
import { analyzeCardLayout, optimizeBehaviorsForSpace } from "./utils/spaceOptimizer.ts";
import BehaviorContainer from "./components/BehaviorContainer.tsx";
import ManualActionLayout from "./components/ManualActionLayout.tsx";
import TriggeredEffectLayout from "./components/TriggeredEffectLayout.tsx";
import ImmediateResourceLayout from "./components/ImmediateResourceLayout.tsx";
import DiscountLayout from "./components/DiscountLayout.tsx";
import PaymentSubstituteLayout from "./components/PaymentSubstituteLayout.tsx";
import ValueModifierLayout from "./components/ValueModifierLayout.tsx";
import DefenseLayout from "./components/DefenseLayout.tsx";
import { ComputedPerAmountsContext, hasResourceVisual } from "./components/ResourceDisplay.tsx";

function DependencyHeader({
  classifiedBehavior,
  index,
  showTooltip,
}: {
  classifiedBehavior: ClassifiedBehavior;
  index: number;
  showTooltip: boolean;
}) {
  const dependency = classifiedBehavior.behavior.dependencyStatus;
  const conditionLabel = useStateVisualLabel(dependency?.state ?? "", dependency?.equals);
  let label = conditionLabel;
  if (dependency?.retired) {
    label = `${dependency.label ?? "Dependent effects"}: ended`;
  } else if (dependency?.imported) {
    label = "Imported technology: these effects are independent";
  }
  return (
    <BehaviorContainer
      classifiedBehavior={{ ...classifiedBehavior, type: "auto-no-background" }}
      index={index}
      description={label}
      showTooltip={showTooltip}
      noContainer
    >
      <BehaviorQualifiers behavior={{ dependencyStatus: dependency }} />
    </BehaviorContainer>
  );
}

function buildBehaviorLayout(
  behaviors: NonNullable<BehaviorSectionProps["behaviors"]>,
  presentation: BehaviorPresentation,
) {
  const classified = classifyBehaviors(behaviors).filter(({ behavior, type }) => {
    if (["discount", "payment-substitute", "value-modifier", "defense"].includes(type)) {
      return (behavior.outputs?.length ?? 0) > 0;
    }
    const resources = [
      ...(behavior.inputs ?? []),
      ...(behavior.outputs ?? []),
      ...(behavior.choices ?? []).flatMap((choice) =>
        (choice.inputs ?? []).concat(choice.outputs ?? []),
      ),
    ];
    return resources.some(hasResourceVisual);
  });
  const mergedBehaviors = mergeTriggeredEffects(mergeAutoProductionBehaviors(classified));
  const merged = groupBehaviorPresentation(mergedBehaviors);
  const layout = analyzeCardLayout(merged, presentation === "inspection" ? Infinity : undefined);
  return {
    tileScaleInfo: detectTilePlacementScale(merged),
    cardLayoutPlan: layout,
    optimizedBehaviors: optimizeBehaviorsForSpace(merged, layout),
  };
}

const emptyBehaviors: NonNullable<BehaviorSectionProps["behaviors"]> = [];
const layoutCache = new WeakMap<
  NonNullable<BehaviorSectionProps["behaviors"]>,
  Map<string, ReturnType<typeof buildBehaviorLayout>>
>();

function getBehaviorLayout(
  behaviors = emptyBehaviors,
  presentation: BehaviorPresentation = "compact",
) {
  let layouts = layoutCache.get(behaviors);
  if (!layouts) {
    layouts = new Map();
    layoutCache.set(behaviors, layouts);
  }
  const key = presentation;
  let layout = layouts.get(key);
  if (!layout) {
    layout = buildBehaviorLayout(behaviors, presentation);
    layouts.set(key, layout);
  }
  return layout;
}

export function hasBehaviorVisuals(
  behaviors: BehaviorSectionProps["behaviors"],
  presentation: BehaviorPresentation = "compact",
): boolean {
  return getBehaviorLayout(behaviors, presentation).optimizedBehaviors.length > 0;
}

const BehaviorContent: React.FC<BehaviorSectionProps> = ({
  behaviors,
  computedValues,
  sourceBehaviorIndex,
  playerResources,
  resourceStorage,
  cardId,
  vpConditions,
  greyOutAll = false,
  hideActionChip = false,
  noContainer = false,
  presentation = "compact",
  showTooltips = true,
  layout = "project",
}) => {
  const wide = layout === "corporation";
  const tooltipsEnabled = showTooltips && presentation === "compact";

  const computedValuesByIndex = useMemo(() => {
    const map = new Map<number, CalculatedOutputDto[]>();
    if (computedValues) {
      for (const cv of computedValues) {
        const match = cv.target.match(/^behaviors::(\d+)$/);
        if (match) {
          const index = parseInt(match[1], 10);
          if (sourceBehaviorIndex === undefined || sourceBehaviorIndex === index) {
            map.set(sourceBehaviorIndex === undefined ? index : 0, cv.outputs);
          }
        }
      }
    }
    return map;
  }, [computedValues, sourceBehaviorIndex]);

  const computedPerAmounts = useMemo(() => {
    const amounts = new Map<PerConditionDto, number>();
    behaviors?.forEach((behavior, behaviorIndex) => {
      // The backend sends scaled outputs in authored order, including choice outputs.
      // Associate them before layout merging/reordering; per objects survive layout clones.
      const calculated = (computedValuesByIndex.get(behaviorIndex) ?? []).filter(
        (output) => output.isScaled,
      );
      let calculatedIndex = 0;
      const outputs = [
        ...(behavior.outputs ?? []),
        ...(behavior.choices ?? []).flatMap((choice) => choice.outputs ?? []),
      ];
      for (const output of outputs) {
        const per = getPer(output);
        if (!per || per.amount <= 0) {
          continue;
        }
        const value = calculated[calculatedIndex++];
        if (value?.resourceType === output.type) {
          amounts.set(per, value.amount);
        }
      }
    });
    return amounts;
  }, [behaviors, computedValuesByIndex]);

  const { tileScaleInfo, cardLayoutPlan, optimizedBehaviors } = getBehaviorLayout(
    behaviors,
    presentation,
  );

  if (optimizedBehaviors.length === 0) {
    return null;
  }

  // Helper function to check if a resource is affordable (bound to current context)
  const checkResourceAffordable = (resource: any, isInput: boolean = true): boolean => {
    return isResourceAffordable(
      resource,
      isInput,
      playerResources,
      resourceStorage,
      cardId,
      greyOutAll,
    );
  };

  // Render individual behavior based on its type
  const renderBehavior = (
    classifiedBehavior: ClassifiedBehavior,
    index: number,
    groupedDependency = false,
    groupedAction = false,
    groupedEffect = false,
    groupedState = false,
    borderLabel?: React.ReactNode,
  ): React.ReactNode => {
    const { behavior, type } = classifiedBehavior;
    if (classifiedBehavior.presentationGroup === "dependency") {
      const dependency = behavior.dependencyStatus;
      const member =
        classifiedBehavior.members?.length === 1 ? classifiedBehavior.members[0] : undefined;
      const productionBox =
        member?.type === "auto-no-background" &&
        !member.behavior.choices?.length &&
        !member.behavior.inputs?.length &&
        !!member.behavior.outputs?.length &&
        member.behavior.outputs.every(
          (output) =>
            output.type.endsWith("-production") &&
            (member.behavior.outputs!.length === 1 || !("per" in output && output.per)),
        );
      const singleBox =
        member &&
        !member.presentationGroup &&
        (productionBox ||
          [
            "manual-action",
            "triggered-effect",
            "discount",
            "payment-substitute",
            "value-modifier",
            "defense",
          ].includes(member.type));
      if (singleBox) {
        return (
          <div
            key={`dependency-${behavior.dependency}`}
            data-dependency-group={behavior.dependency}
            className={`min-w-0 max-w-full ${dependency && !dependency.active ? "opacity-50" : ""}`}
          >
            {renderBehavior(
              member,
              index,
              true,
              false,
              false,
              false,
              <DependencyHeader
                classifiedBehavior={classifiedBehavior}
                index={index}
                showTooltip={tooltipsEnabled}
              />,
            )}
          </div>
        );
      }
      return (
        <fieldset
          key={`dependency-${behavior.dependency}`}
          className={`border border-sky-200/30 px-1.5 pb-1 pt-0 min-w-0 max-w-full flex flex-col items-center gap-0.5 ${dependency && !dependency.active ? "opacity-50" : ""}`}
          data-dependency-group={behavior.dependency}
          aria-label={dependency?.label ?? "Dependent effects"}
        >
          <legend className="px-1 mx-auto [&_.behavior-fitting]:min-h-0 [&_.behavior-fitting]:p-0 [&_.behavior-fitting]:my-0">
            <DependencyHeader
              classifiedBehavior={classifiedBehavior}
              index={index}
              showTooltip={tooltipsEnabled}
            />
          </legend>
          <div className="flex flex-col items-center justify-center gap-1 min-w-0 max-w-full [&>.behavior-fitting]:shrink-0">
            {classifiedBehavior.members?.map((member, memberIndex) =>
              renderBehavior(member, memberIndex, true),
            )}
          </div>
        </fieldset>
      );
    }
    if (["shared-action", "shared-effect"].includes(classifiedBehavior.presentationGroup ?? "")) {
      const sharedEffect = classifiedBehavior.presentationGroup === "shared-effect";
      return (
        <BehaviorContainer
          key={`shared-action-${behavior.usageGroup}`}
          classifiedBehavior={classifiedBehavior}
          index={index}
          cardId={cardId}
          adjacentScoring={vpConditions
            ?.filter((condition) => condition.per?.adjacentToSelfTile)
            .map((condition) => condition.per!.type)}
          noContainer={noContainer || sharedEffect}
          description={classifiedBehavior.description}
          showTooltip={tooltipsEnabled}
        >
          <div
            className={`flex ${wide ? "flex-row flex-wrap justify-center gap-3" : "flex-col gap-1"} items-center min-w-0 max-w-full`}
            data-shared-action={sharedEffect ? undefined : behavior.usageGroup}
            data-shared-effect={sharedEffect ? behavior.usageGroup : undefined}
          >
            {classifiedBehavior.members?.map((member, memberIndex) => {
              const stateFrame = sharedEffect && !!member.behavior.stateRequirements?.length;
              const memberContent = renderBehavior(
                member,
                memberIndex,
                groupedDependency,
                !sharedEffect,
                sharedEffect,
                stateFrame,
              );
              return (
                <React.Fragment key={member.behavior.id ?? memberIndex}>
                  {!sharedEffect && memberIndex > 0 && <OrChip />}
                  {stateFrame ? (
                    <fieldset
                      className="border border-sky-200/30 bg-white/[0.08] px-1.5 pb-1 pt-0 min-w-0 max-w-full"
                      data-state-effect
                    >
                      <legend className="px-1 mx-auto flex items-center [--behavior-icon-size:18px]">
                        <OperationRequirements
                          stateRequirements={member.behavior.stateRequirements}
                        />
                      </legend>
                      {memberContent}
                    </fieldset>
                  ) : (
                    memberContent
                  )}
                </React.Fragment>
              );
            })}
            {sharedEffect && behavior.maxUsesPerGeneration && (
              <span data-rule-exception className="font-bold self-center">
                *
              </span>
            )}
          </div>
        </BehaviorContainer>
      );
    }
    const layoutPlan = createLayoutPlan(behavior, type);
    const sourceIndices =
      classifiedBehavior.originalIndices ??
      (classifiedBehavior.originalIndex === undefined ? [] : [classifiedBehavior.originalIndex]);
    let behaviorComputedOutputs: CalculatedOutputDto[] | undefined;
    if (computedValues !== undefined) {
      behaviorComputedOutputs = classifiedBehavior.alternativeOriginalIndices
        ? (classifiedBehavior.alternativeOriginalIndices
            .map((sourceIndex) => computedValuesByIndex.get(sourceIndex))
            .find((outputs) => outputs?.length) ?? [])
        : sourceIndices.flatMap((sourceIndex) => computedValuesByIndex.get(sourceIndex) ?? []);
    }

    let content: React.ReactNode = null;

    switch (type) {
      case "manual-action":
        content = (
          <ManualActionLayout
            behavior={behavior}
            layoutPlan={layoutPlan}
            isResourceAffordable={checkResourceAffordable}
            analyzeResourceDisplayWithConstraints={analyzeResourceDisplayWithConstraints}
            tileScaleInfo={tileScaleInfo}
            hideActionChip={hideActionChip || groupedAction}
            computedOutputs={behaviorComputedOutputs}
          />
        );
        break;

      case "triggered-effect":
        content = (
          <TriggeredEffectLayout
            behavior={behavior}
            mergedBehaviors={classifiedBehavior.mergedBehaviors}
            layoutPlan={layoutPlan}
            isResourceAffordable={checkResourceAffordable}
            analyzeResourceDisplayWithConstraints={analyzeResourceDisplayWithConstraints}
            tileScaleInfo={tileScaleInfo}
            computedOutputs={behaviorComputedOutputs}
          />
        );
        break;

      case "immediate-production":
      case "immediate-effect":
      case "auto-no-background":
        content = (
          <ImmediateResourceLayout
            productionLabel={type === "auto-no-background" ? borderLabel : undefined}
            behavior={behavior}
            isResourceAffordable={checkResourceAffordable}
            tileScaleInfo={tileScaleInfo}
            computedOutputs={behaviorComputedOutputs}
          />
        );
        break;

      case "discount":
        content = <DiscountLayout behavior={behavior} />;
        break;

      case "payment-substitute":
        content = <PaymentSubstituteLayout behavior={behavior} />;
        break;

      case "value-modifier":
        content = <ValueModifierLayout behavior={behavior} />;
        break;

      case "defense":
        content = <DefenseLayout behavior={behavior} />;
        break;
    }

    const dependency = groupedDependency ? undefined : behavior.dependencyStatus;
    const inlineDependency =
      dependency &&
      !behavior.stateRequirements?.length &&
      !behavior.resourceRequirements?.length &&
      !behavior.maxUsesPerGeneration &&
      !behavior.maxUsesPerGame;

    const inlineStateAction =
      type === "manual-action" &&
      !behavior.choices?.length &&
      (behavior.stateRequirements?.length ?? 0) === 1 &&
      !behavior.maxUsesPerGame &&
      (behavior.resourceRequirements?.length ?? 0) <= 1 &&
      (behavior.inputs?.length ?? 0) <= 1 &&
      (behavior.outputs?.length ?? 0) === 1;

    const ruleException =
      Boolean(behavior.maxUsesPerGame) ||
      (!groupedEffect &&
        Boolean(behavior.maxUsesPerGeneration) &&
        (type !== "manual-action" || behavior.maxUsesPerGeneration! > 1));
    const immediateExchange =
      (type === "triggered-effect" || type === "immediate-effect") &&
      !behavior.optional &&
      !!behavior.inputs?.length &&
      behavior.triggers?.every((trigger) => trigger.type === "auto" && !trigger.condition);

    return (
      <BehaviorContainer
        key={`${behavior.id ?? "behavior"}-${index}`}
        classifiedBehavior={classifiedBehavior}
        index={index}
        description={classifiedBehavior.description}
        showTooltip={tooltipsEnabled}
        borderLabel={type === "auto-no-background" ? undefined : borderLabel}
        noContainer={noContainer || groupedAction || groupedEffect || immediateExchange}
        cardId={cardId}
        adjacentScoring={vpConditions
          ?.filter((condition) => condition.per?.adjacentToSelfTile)
          .map((condition) => condition.per!.type)}
      >
        <div
          className={`flex ${inlineDependency || inlineStateAction || (wide && (groupedAction || groupedEffect)) ? "flex-row justify-center" : "flex-col"} items-center gap-1 w-full ${dependency && !dependency.active ? "opacity-50" : ""}`}
        >
          <BehaviorQualifiers
            behavior={behavior}
            hideDependency={groupedDependency}
            hideStateRequirements={groupedState || behavior.optional}
          />
          {
            <div className="flex items-center justify-center gap-1 min-w-0 max-w-full">
              {content}
              {ruleException && (
                <span data-rule-exception className="font-bold shrink-0">
                  *
                </span>
              )}
            </div>
          }
        </div>
      </BehaviorContainer>
    );
  };

  const immediateRow =
    optimizedBehaviors.length > 1 &&
    optimizedBehaviors.every(
      ({ behavior, type }) =>
        ["auto-no-background", "immediate-effect", "immediate-production"].includes(type) &&
        !behavior.choices?.length &&
        !behavior.inputs?.length &&
        !behavior.stateRequirements?.length &&
        !behavior.resourceRequirements?.length &&
        !behavior.maxUsesPerGame &&
        !behavior.maxUsesPerGeneration &&
        !behavior.triggers?.some((trigger) => trigger.condition),
    ) &&
    optimizedBehaviors.reduce(
      (count, { behavior }) => count + (behavior.outputs?.length ?? 0),
      0,
    ) <= 3;

  // Render behaviors with overflow handling if needed
  const containerClass =
    cardLayoutPlan.needsOverflowHandling && !wide && !immediateRow
      ? "flex flex-col gap-[var(--behavior-section-gap,3px)] items-center w-full max-h-[120px] overflow-y-auto overflow-x-hidden scroll-smooth [scrollbar-width:thin] [&::-webkit-scrollbar]:w-0.5 [&::-webkit-scrollbar-track]:bg-white/10 [&::-webkit-scrollbar-track]:rounded-px [&::-webkit-scrollbar-thumb]:bg-white/30 [&::-webkit-scrollbar-thumb]:rounded-px"
      : `flex ${immediateRow || wide ? "flex-row flex-wrap justify-center" : "flex-col"} ${immediateRow ? "gap-x-2 gap-y-[var(--behavior-section-gap,3px)]" : "gap-[var(--behavior-section-gap,3px)]"} items-center w-full`;

  return (
    <ComputedPerAmountsContext.Provider value={computedPerAmounts}>
      <div
        className={`behavior-section ${containerClass}`}
        data-behavior-layout={wide ? "corporation" : undefined}
      >
        {optimizedBehaviors.map((classifiedBehavior, index) =>
          renderBehavior(classifiedBehavior, index),
        )}
      </div>
    </ComputedPerAmountsContext.Provider>
  );
};

export default function BehaviorSection(props: BehaviorSectionProps) {
  return (
    <CardSourcePresentation cardId={props.cardId} cardName={props.cardName}>
      <BehaviorContent {...props} />
    </CardSourcePresentation>
  );
}
