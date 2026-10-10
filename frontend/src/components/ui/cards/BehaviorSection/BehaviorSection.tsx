import { FormattedDescription } from "../../display/FormattedDescription.tsx";
import React, { useMemo } from "react";
import { BehaviorSectionProps, ClassifiedBehavior, BehaviorPresentation } from "./types.ts";
import { CalculatedOutputDto } from "@/types/generated/api-types.ts";
import { classifyBehaviors } from "./utils/behaviorClassifier.ts";
import { detectTilePlacementScale } from "./utils/tileScaling.ts";
import { isResourceAffordable } from "./utils/resourceValidation.ts";
import { analyzeResourceDisplayWithConstraints } from "./utils/displayAnalysis.ts";
import { mergeAutoProductionBehaviors, mergeTriggeredEffects } from "./utils/behaviorMerger.ts";
import { analyzeCardLayout, optimizeBehaviorsForSpace } from "./utils/spaceOptimizer.ts";
import BehaviorContainer from "./components/BehaviorContainer.tsx";
import ManualActionLayout from "./components/ManualActionLayout.tsx";
import TriggeredEffectLayout from "./components/TriggeredEffectLayout.tsx";
import ImmediateResourceLayout from "./components/ImmediateResourceLayout.tsx";
import DiscountLayout from "./components/DiscountLayout.tsx";
import PaymentSubstituteLayout from "./components/PaymentSubstituteLayout.tsx";
import ValueModifierLayout from "./components/ValueModifierLayout.tsx";
import DefenseLayout from "./components/DefenseLayout.tsx";
import { hasResourceVisual } from "./components/ResourceDisplay.tsx";

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
    return resources.some(hasResourceVisual) || Boolean(behavior.description);
  });
  const merged = mergeTriggeredEffects(mergeAutoProductionBehaviors(classified));
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
  Map<BehaviorPresentation, ReturnType<typeof buildBehaviorLayout>>
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
  let layout = layouts.get(presentation);
  if (!layout) {
    layout = buildBehaviorLayout(behaviors, presentation);
    layouts.set(presentation, layout);
  }
  return layout;
}

export function hasBehaviorVisuals(
  behaviors: BehaviorSectionProps["behaviors"],
  presentation: BehaviorPresentation = "compact",
): boolean {
  return getBehaviorLayout(behaviors, presentation).optimizedBehaviors.length > 0;
}

const BehaviorSection: React.FC<BehaviorSectionProps> = ({
  behaviors,
  computedValues,
  playerResources,
  resourceStorage,
  cardId,
  greyOutAll = false,
  hideActionChip = false,
  noContainer = false,
  presentation = "compact",
  showTooltips = true,
}) => {
  const tooltipsEnabled = showTooltips && presentation === "compact";

  const computedValuesByIndex = useMemo(() => {
    const map = new Map<number, CalculatedOutputDto[]>();
    if (computedValues) {
      for (const cv of computedValues) {
        const match = cv.target.match(/^behaviors::(\d+)$/);
        if (match) {
          map.set(parseInt(match[1], 10), cv.outputs);
        }
      }
    }
    return map;
  }, [computedValues]);

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
  ): React.ReactNode => {
    const { behavior, type } = classifiedBehavior;
    const layoutPlan = cardLayoutPlan.behaviors[index]?.layoutPlan;
    const sourceIndices =
      classifiedBehavior.originalIndices ??
      (classifiedBehavior.originalIndex === undefined ? [] : [classifiedBehavior.originalIndex]);
    const behaviorComputedOutputs = sourceIndices.flatMap(
      (sourceIndex) => computedValuesByIndex.get(sourceIndex) ?? [],
    );

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
            hideActionChip={hideActionChip}
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

    const resources = [
      ...(behavior.inputs ?? []),
      ...(behavior.outputs ?? []),
      ...(behavior.choices ?? []).flatMap((choice) => [
        ...(choice.inputs ?? []),
        ...(choice.outputs ?? []),
      ]),
    ];
    const needsText =
      resources.some((resource) => !hasResourceVisual(resource)) ||
      resources.length === 0 ||
      Boolean(behavior.stateRequirements?.length) ||
      Boolean(behavior.maxUsesPerGeneration || behavior.maxUsesPerGame) ||
      Boolean(
        behavior.triggers?.some(
          (trigger) =>
            !["auto", "manual", "auto-corporation-start"].includes(trigger.type) ||
            Boolean(trigger.condition?.state) ||
            trigger.condition?.type === "production",
        ),
      );
    if (needsText && behavior.description) {
      content = (
        <div className="text-[11px] leading-snug font-sans text-left px-1 py-0.5">
          <FormattedDescription text={classifiedBehavior.description ?? behavior.description} />
        </div>
      );
    }
    const dependency = behavior.dependencyStatus;
    return (
      <BehaviorContainer
        key={`${behavior.id ?? "behavior"}-${index}`}
        classifiedBehavior={classifiedBehavior}
        index={index}
        description={classifiedBehavior.description}
        showTooltip={tooltipsEnabled}
        noContainer={noContainer}
        cardId={cardId}
      >
        <div
          className={`flex flex-col items-center gap-1 w-full ${dependency && !dependency.active ? "opacity-50" : ""}`}
        >
          {dependency && (
            <span
              className="font-sans text-[10px] leading-tight text-center text-sky-200"
              aria-label={
                dependency.imported
                  ? "Technology imported, independent"
                  : `${dependency.label ?? "Dependent operation"}: ${dependency.retired ? "retired" : dependency.active ? "online" : "offline"}`
              }
            >
              {dependency.imported
                ? "Technology imported · Independent"
                : `${dependency.label ?? "Dependent operation"} · ${dependency.retired ? "Retired" : dependency.active ? "Online" : "Offline"}`}
            </span>
          )}
          {content}
        </div>
      </BehaviorContainer>
    );
  };

  // Render behaviors with overflow handling if needed
  const containerClass = cardLayoutPlan.needsOverflowHandling
    ? "flex flex-col gap-[var(--behavior-section-gap,3px)] items-center w-full max-h-[120px] overflow-y-auto overflow-x-hidden scroll-smooth [scrollbar-width:thin] [&::-webkit-scrollbar]:w-0.5 [&::-webkit-scrollbar-track]:bg-white/10 [&::-webkit-scrollbar-track]:rounded-px [&::-webkit-scrollbar-thumb]:bg-white/30 [&::-webkit-scrollbar-thumb]:rounded-px"
    : "flex flex-col gap-[var(--behavior-section-gap,3px)] items-center w-full";

  return (
    <div className={`behavior-section ${containerClass}`}>
      {optimizedBehaviors.map((classifiedBehavior, index) =>
        renderBehavior(classifiedBehavior, index),
      )}
    </div>
  );
};

export default BehaviorSection;
