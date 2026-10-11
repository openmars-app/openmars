import GameIcon from "../../../display/GameIcon.tsx";
import type { CardBehaviorDto, SelectorDto } from "@/types/generated/api-types";
import { isEffect, getSelectors } from "@/types/resourceConditions";

interface DefenseLayoutProps {
  behavior: CardBehaviorDto;
}

const getResourcesFromSelectors = (selectors: SelectorDto[]): string[] => {
  const resources: string[] = [];
  const seen = new Set<string>();
  selectors.forEach((selector: SelectorDto) => {
    if (selector.resources) {
      selector.resources.forEach((r: string) => {
        if (!seen.has(r)) {
          seen.add(r);
          resources.push(r);
        }
      });
    }
  });
  return resources;
};

const DefenseLayout: React.FC<DefenseLayoutProps> = ({ behavior }) => {
  if (!behavior.outputs || behavior.outputs.length === 0) {
    return null;
  }

  const defenseOutput = behavior.outputs.find(
    (output) => isEffect(output) && output.type === "defense",
  );
  if (!defenseOutput || !isEffect(defenseOutput)) {
    return null;
  }

  const selectors: SelectorDto[] = getSelectors(defenseOutput) || [];
  const affectedResources = getResourcesFromSelectors(selectors);

  if (affectedResources.length === 0) {
    return null;
  }

  return (
    <div className="flex gap-[6px] items-center">
      <GameIcon iconType="defense" size="behavior" />

      <div className="flex gap-[3px] items-center">
        {affectedResources.map((resourceType: string, index: number) => (
          <GameIcon key={`res-${index}`} iconType={resourceType} size="behavior" />
        ))}
      </div>
    </div>
  );
};

export default DefenseLayout;
