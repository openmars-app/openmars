import { ClassifiedBehavior } from "../types.ts";

function mergedMetadata(behaviors: ClassifiedBehavior[]) {
  const description = [
    ...new Set(behaviors.map((behavior) => behavior.description).filter(Boolean)),
  ].join(" ");
  const originalIndices = [
    ...new Set(
      behaviors.flatMap(
        (behavior) =>
          behavior.originalIndices ??
          (behavior.originalIndex === undefined ? [] : [behavior.originalIndex]),
      ),
    ),
  ];
  return { description: description || undefined, originalIndices };
}

/**
 * Gets a unique key for a behavior's trigger condition.
 * Combines condition type, target, and location to ensure effects with different
 * targets/locations are displayed separately (e.g., Tharsis Republic's two city effects).
 */
const getConditionKey = (behavior: any): string | null => {
  if (!behavior.triggers) return null;
  for (const trigger of behavior.triggers) {
    if (trigger.condition?.type) {
      const type = trigger.condition.type;
      const target = trigger.condition.target || "any";
      const location = trigger.condition.location || "any";
      return JSON.stringify({
        type,
        target,
        location,
        condition: trigger.condition,
        dependency: behavior.dependency,
        stateRequirements: behavior.stateRequirements,
        maxUsesPerGeneration: behavior.maxUsesPerGeneration,
        maxUsesPerGame: behavior.maxUsesPerGame,
        usageGroup: behavior.usageGroup,
      });
    }
  }
  return null;
};

/**
 * Merges triggered effects that share the exact same condition (type + target + location).
 * Effects with different targets or locations are kept separate to display as distinct boxes.
 *
 * @param classifiedBehaviors - Array of behaviors to potentially merge
 * @returns Array with merged triggered effects where applicable
 */
export const mergeTriggeredEffects = (
  classifiedBehaviors: ClassifiedBehavior[],
): ClassifiedBehavior[] => {
  const triggeredEffects: ClassifiedBehavior[] = [];
  const otherBehaviors: ClassifiedBehavior[] = [];

  // Separate triggered effects from other behaviors
  classifiedBehaviors.forEach((classifiedBehavior) => {
    if (classifiedBehavior.type === "triggered-effect") {
      triggeredEffects.push(classifiedBehavior);
    } else if (classifiedBehavior.behavior.group) {
      triggeredEffects.push(classifiedBehavior);
    } else {
      otherBehaviors.push(classifiedBehavior);
    }
  });

  // Group triggered effects by group field (if present) or condition key (type + target + location)
  const groupedByCondition: Map<string, ClassifiedBehavior[]> = new Map();
  const ungroupedEffects: ClassifiedBehavior[] = [];

  triggeredEffects.forEach((effect) => {
    const groupKey = effect.behavior.group
      ? `group:${effect.behavior.group}:${JSON.stringify([effect.behavior.dependency, effect.behavior.stateRequirements, effect.behavior.maxUsesPerGeneration, effect.behavior.maxUsesPerGame, effect.behavior.usageGroup])}`
      : getConditionKey(effect.behavior);
    if (groupKey) {
      const existing = groupedByCondition.get(groupKey) || [];
      existing.push(effect);
      groupedByCondition.set(groupKey, existing);
    } else {
      ungroupedEffects.push(effect);
    }
  });

  // Merge groups with multiple effects
  const mergedEffects: ClassifiedBehavior[] = [];
  groupedByCondition.forEach((effects) => {
    if (effects.length > 1) {
      // Create merged behavior using first behavior as primary, storing others in mergedBehaviors
      const mergedBehavior: ClassifiedBehavior = {
        behavior: effects[0].behavior,
        type: "triggered-effect",
        ...mergedMetadata(effects),
        mergedBehaviors: effects.slice(1).map((e) => e.behavior),
      };
      mergedEffects.push(mergedBehavior);
    } else {
      mergedEffects.push(effects[0]);
    }
  });

  // Combine results: merged triggered effects, ungrouped effects, then other behaviors
  return [...mergedEffects, ...ungroupedEffects, ...otherBehaviors];
};

/**
 * Merges multiple auto production behaviors into a single behavior.
 * This optimization reduces visual clutter by combining production resources
 * and immediate effects that occur automatically when the card is played.
 *
 * Behaviors with trigger conditions (e.g., "when greenery is placed") are NOT merged
 * to maintain clarity about when effects trigger.
 *
 * @param classifiedBehaviors - Array of behaviors to potentially merge
 * @returns Array with merged behaviors where applicable
 */
export const mergeAutoProductionBehaviors = (
  classifiedBehaviors: ClassifiedBehavior[],
): ClassifiedBehavior[] => {
  const groups: ClassifiedBehavior[][] = [];
  for (const behavior of classifiedBehaviors) {
    const previous = groups[groups.length - 1];
    if (
      previous &&
      !previous[0].behavior.choices?.length &&
      !behavior.behavior.choices?.length &&
      previous[0].description === behavior.description
    ) {
      previous.push(behavior);
    } else {
      groups.push([behavior]);
    }
  }
  return groups.flatMap(mergeAutoProductionGroup);
};

const mergeAutoProductionGroup = (
  classifiedBehaviors: ClassifiedBehavior[],
): ClassifiedBehavior[] => {
  const autoProductionBehaviors: ClassifiedBehavior[] = [];
  const autoNoBackgroundBehaviors: ClassifiedBehavior[] = [];
  const otherBehaviors: ClassifiedBehavior[] = [];

  // Separate behaviors into categories
  classifiedBehaviors.forEach((classifiedBehavior) => {
    const { behavior, type } = classifiedBehavior;

    // Check if behavior has a trigger condition (e.g., greenery-placed for Herbivores)
    const hasCondition =
      Boolean(behavior.dependency) ||
      Boolean(behavior.stateRequirements?.length) ||
      Boolean(behavior.maxUsesPerGeneration) ||
      Boolean(behavior.maxUsesPerGame) ||
      (behavior.triggers && behavior.triggers.some((trigger: any) => trigger.condition));

    // Grouped behaviors skip auto merging — they'll be handled by mergeTriggeredEffects
    if (behavior.group) {
      otherBehaviors.push(classifiedBehavior);
      return;
    }

    const isAutoProduction =
      type === "auto-no-background" &&
      behavior.outputs &&
      behavior.outputs.every(
        (output: any) =>
          output.type?.includes("production") ||
          output.resourceType?.includes("production") ||
          output.isProduction,
      );

    // Don't merge behaviors that have trigger conditions
    if (hasCondition) {
      otherBehaviors.push(classifiedBehavior);
    } else if (isAutoProduction) {
      autoProductionBehaviors.push(classifiedBehavior);
    } else if (type === "auto-no-background") {
      autoNoBackgroundBehaviors.push(classifiedBehavior);
    } else {
      otherBehaviors.push(classifiedBehavior);
    }
  });

  // Merge multiple auto production behaviors
  let mergedAutoProduction: ClassifiedBehavior | null = null;
  if (autoProductionBehaviors.length > 1) {
    const mergedOutputs: any[] = [];
    autoProductionBehaviors.forEach((classifiedBehavior) => {
      if (classifiedBehavior.behavior.outputs) {
        mergedOutputs.push(...classifiedBehavior.behavior.outputs);
      }
    });

    mergedAutoProduction = {
      ...mergedMetadata(autoProductionBehaviors),
      behavior: {
        triggers: [{ type: "auto" }],
        outputs: mergedOutputs,
      },
      type: "auto-no-background",
    };
  } else if (autoProductionBehaviors.length === 1) {
    mergedAutoProduction = autoProductionBehaviors[0];
  }

  // Merge multiple auto-no-background behaviors (like Big Asteroid)
  let mergedAutoNoBackground: ClassifiedBehavior | null = null;
  if (autoNoBackgroundBehaviors.length > 1) {
    const mergedOutputs: any[] = [];
    autoNoBackgroundBehaviors.forEach((classifiedBehavior) => {
      if (classifiedBehavior.behavior.outputs) {
        mergedOutputs.push(...classifiedBehavior.behavior.outputs);
      }
    });

    mergedAutoNoBackground = {
      ...mergedMetadata(autoNoBackgroundBehaviors),
      behavior: {
        triggers: [{ type: "auto" }],
        outputs: mergedOutputs,
      },
      type: "auto-no-background",
    };
  } else if (autoNoBackgroundBehaviors.length === 1) {
    mergedAutoNoBackground = autoNoBackgroundBehaviors[0];
  }

  // Combine results
  const result: ClassifiedBehavior[] = [];

  // If both production and immediate resources exist, merge them into a single behavior
  if (mergedAutoProduction && mergedAutoNoBackground) {
    const combinedBehavior: ClassifiedBehavior = {
      ...mergedMetadata([mergedAutoProduction, mergedAutoNoBackground]),
      behavior: {
        triggers: [{ type: "auto" }],
        outputs: [
          ...(mergedAutoProduction.behavior.outputs || []),
          ...(mergedAutoNoBackground.behavior.outputs || []),
        ],
      },
      type: "auto-no-background",
    };
    result.push(combinedBehavior);
  } else {
    if (mergedAutoProduction) result.push(mergedAutoProduction);
    if (mergedAutoNoBackground) result.push(mergedAutoNoBackground);
  }

  result.push(...otherBehaviors);

  return result;
};
