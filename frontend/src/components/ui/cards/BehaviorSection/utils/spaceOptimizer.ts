import { ClassifiedBehavior, CardLayoutPlan } from "../types.ts";
import { MAX_CARD_ROWS } from "../constants.ts";
import { createLayoutPlan } from "./layoutCalculator.ts";

/**
 * Analyzes all behaviors on a card and creates a coordinated layout plan.
 * Determines if the card needs overflow handling based on total row count.
 *
 * @param classifiedBehaviors - Array of behaviors to analyze
 * @returns Layout plan including row estimates and overflow requirements
 */
export const analyzeCardLayout = (
  classifiedBehaviors: ClassifiedBehavior[],
  maxRows = MAX_CARD_ROWS,
): CardLayoutPlan => {
  const behaviorPlans = classifiedBehaviors.map((classifiedBehavior, index) => {
    const { behavior, type } = classifiedBehavior;
    const layoutPlan = createLayoutPlan(behavior, type);

    // Estimate rows needed for this behavior
    let estimatedRows = layoutPlan.totalRows;
    if (classifiedBehavior.members) {
      const memberLayout = analyzeCardLayout(classifiedBehavior.members, Infinity);
      estimatedRows = memberLayout.totalEstimatedRows;
      if (classifiedBehavior.presentationGroup === "dependency") {
        estimatedRows += 0.5;
      }
    }

    // Add extra row for behavior container if it's not auto-no-background
    if (type !== "auto-no-background") {
      estimatedRows = Math.max(1, estimatedRows);
    }

    return {
      behaviorIndex: index,
      layoutPlan,
      estimatedRows,
    };
  });

  const totalEstimatedRows = behaviorPlans.reduce((sum, plan) => sum + plan.estimatedRows, 0);
  const needsOverflowHandling = totalEstimatedRows > maxRows;

  return {
    behaviors: behaviorPlans,
    totalEstimatedRows,
    needsOverflowHandling,
    maxRows,
  };
};

/**
 * Optimizes behaviors for space when multiple behaviors exceed available card space.
 * Converts resources to more compact "NxIcon" format to reduce total row count.
 *
 * Strategy: Force number format for amounts > 2 instead of the default > 3,
 * reducing the number of individual icons displayed.
 *
 * @param classifiedBehaviors - Behaviors to optimize
 * @param cardLayoutPlan - Current layout plan with space analysis
 * @returns Optimized behaviors with more aggressive number formatting
 */
export const optimizeBehaviorsForSpace = (
  classifiedBehaviors: ClassifiedBehavior[],
  cardLayoutPlan: CardLayoutPlan,
): ClassifiedBehavior[] => {
  if (!cardLayoutPlan.needsOverflowHandling) {
    return classifiedBehaviors; // No optimization needed
  }

  // Strategy: Convert more resources to NxIcon format to save space
  const optimizedBehaviors = classifiedBehaviors.map((classifiedBehavior) => {
    const { behavior } = classifiedBehavior;

    // Create optimized behavior with more aggressive number formatting
    const optimizedBehavior = {
      ...behavior,
      inputs: behavior.inputs?.map((input: any) => ({
        ...input,
        // Force number format for amounts > 2 instead of > 4
        forceNumberFormat: input.amount > 2,
      })),
      outputs: behavior.outputs?.map((output: any) => ({
        ...output,
        // Force number format for amounts > 2 instead of > 4
        forceNumberFormat: output.amount > 2,
      })),
    };

    return { ...classifiedBehavior, behavior: optimizedBehavior };
  });

  return optimizedBehaviors;
};
