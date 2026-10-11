import type {
  BehaviorResourceRequirementDto,
  CardBehaviorDto,
  StateRequirementDto,
} from "@/types/generated/api-types.ts";
import GameIcon from "../../../display/GameIcon.tsx";
import { StatePredicates, StateSymbol, useCardResourceStorage } from "../../CardStateVisuals.tsx";

export function OperationRequirements({
  stateRequirements,
  resourceRequirements,
}: {
  stateRequirements?: StateRequirementDto[];
  resourceRequirements?: BehaviorResourceRequirementDto[];
}) {
  return (
    <>
      <StatePredicates requirements={stateRequirements} />
      {Boolean(resourceRequirements?.length) && (
        <span
          data-rule-exception
          className="font-bold"
          aria-label="Additional resource requirement"
        >
          *
        </span>
      )}
    </>
  );
}

export default function BehaviorQualifiers({
  behavior,
  hideDependency = false,
  hideStateRequirements = false,
}: {
  behavior: CardBehaviorDto;
  hideDependency?: boolean;
  hideStateRequirements?: boolean;
}) {
  const storage = useCardResourceStorage();
  const dependency = hideDependency ? undefined : behavior.dependencyStatus;
  const manual = behavior.triggers?.some((trigger) => trigger.type === "manual");
  const stateRequirements =
    hideStateRequirements || (manual && behavior.maxUsesPerGame)
      ? undefined
      : behavior.stateRequirements;
  const resourceRequirements = behavior.resourceRequirements?.filter((requirement) => {
    if (
      !storage?.capacity ||
      requirement.target !== "self-card" ||
      requirement.type !== storage.type ||
      requirement.min !== undefined
    ) {
      return true;
    }
    const storedGain = (behavior.outputs ?? []).reduce((total, output) => {
      if (
        output.target === "self-card" &&
        output.type === storage.type &&
        (output.amount ?? 0) > 0
      ) {
        return total + output.amount!;
      }
      return total;
    }, 0);
    return !storedGain || requirement.max !== storage.capacity - storedGain;
  });
  const hasQualifiers = dependency || stateRequirements?.length || resourceRequirements?.length;
  if (!hasQualifiers) {
    return null;
  }
  let dependencyStatus = "active";
  let dependencyIcon: string | undefined;
  let dependencyLabel = dependency?.label ?? "Dependent operation";
  if (dependency?.retired) {
    dependencyStatus = "retired";
    dependencyIcon = "operation-retired";
    dependencyLabel = `${dependency.label ?? "Dependent effects"}: ended`;
  } else if (dependency?.imported) {
    dependencyStatus = "imported";
    dependencyIcon = "permanent";
    dependencyLabel = "Permanent effects, independent of Earth contact";
  } else if (dependency && !dependency.active) {
    dependencyStatus = "inactive";
    dependencyLabel += ": inactive";
  }
  return (
    <div className="flex flex-wrap items-center justify-center gap-1" data-behavior-qualifiers>
      {dependency && (
        <span
          className="inline-flex items-center justify-center leading-none [--behavior-icon-size:18px]"
          role="img"
          aria-label={dependencyLabel}
          data-dependency-status={dependencyStatus}
        >
          {dependencyIcon ? (
            <GameIcon iconType={dependencyIcon} size="behavior" />
          ) : (
            <span className="inline-flex items-center gap-0.5">
              <StateSymbol state={dependency.state} value={dependency.equals} />
            </span>
          )}
        </span>
      )}
      <OperationRequirements
        stateRequirements={stateRequirements}
        resourceRequirements={resourceRequirements}
      />
    </div>
  );
}
