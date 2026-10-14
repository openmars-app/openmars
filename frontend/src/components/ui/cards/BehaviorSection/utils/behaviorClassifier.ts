import { CardBehaviorDto } from "@/types/generated/api-types.ts";
import { type ResourceCondition, isProduction } from "@/types/resourceConditions.ts";
import { ClassifiedBehavior } from "../types.ts";

const outputHasType = (outputs: ResourceCondition[] | undefined, type: string): boolean =>
  outputs?.some((o) => o.type === type) ?? false;

export const classifyBehaviors = (behaviors: CardBehaviorDto[]): ClassifiedBehavior[] => {
  return behaviors.flatMap((behavior, originalIndex): ClassifiedBehavior | ClassifiedBehavior[] => {
    const hasTrigger = behavior.triggers && behavior.triggers.length > 0;
    const triggerType = hasTrigger ? behavior.triggers?.[0]?.type : null;
    const hasCondition = behavior.triggers?.[0]?.condition !== undefined;
    const hasInputs = behavior.inputs && behavior.inputs.length > 0;
    const hasProduction = behavior.outputs?.some((o) => isProduction(o)) ?? false;

    const hasDiscount = outputHasType(behavior.outputs, "discount");
    const hasPaymentSubstitute = outputHasType(behavior.outputs, "payment-substitute");
    const hasValueModifier = outputHasType(behavior.outputs, "value-modifier");
    const hasDefense = outputHasType(behavior.outputs, "defense");

    const { description } = behavior;

    if (hasDiscount) {
      const discountOutputs = behavior.outputs?.filter((o) => o.type === "discount") ?? [];
      const otherOutputs = behavior.outputs?.filter((o) => o.type !== "discount") ?? [];

      if (otherOutputs.length > 0) {
        const discountBehavior = { ...behavior, outputs: discountOutputs };
        const remainderBehavior = { ...behavior, outputs: otherOutputs };
        return [
          { behavior: discountBehavior, type: "discount" as const, description, originalIndex },
          {
            behavior: remainderBehavior,
            type: "auto-no-background" as const,
            description,
            originalIndex,
          },
        ];
      }

      return [{ behavior, type: "discount" as const, description, originalIndex }];
    }

    if (hasPaymentSubstitute) {
      return { behavior, type: "payment-substitute", description, originalIndex };
    }

    if (hasValueModifier) {
      return { behavior, type: "value-modifier", description, originalIndex };
    }

    if (hasDefense) {
      return { behavior, type: "defense", description, originalIndex };
    }

    if (behavior.optional) {
      return { behavior, type: "immediate-effect", description, originalIndex };
    }

    if (triggerType === "manual") {
      return { behavior, type: "manual-action", description, originalIndex };
    }

    if ((triggerType === "auto" || triggerType === "auto-corporation-start") && hasCondition) {
      return { behavior, type: "triggered-effect", description, originalIndex };
    }

    if (triggerType === "auto" && !hasInputs) {
      return { behavior, type: "auto-no-background", description, originalIndex };
    }

    if (triggerType === "auto" && !hasCondition && hasInputs) {
      return { behavior, type: "immediate-effect", description, originalIndex };
    }

    if (hasTrigger && hasInputs) {
      return { behavior, type: "triggered-effect", description, originalIndex };
    }

    if (hasProduction && (!hasTrigger || triggerType === "auto")) {
      return { behavior, type: "immediate-production", description, originalIndex };
    }

    return { behavior, type: "immediate-effect", description, originalIndex };
  });
};
