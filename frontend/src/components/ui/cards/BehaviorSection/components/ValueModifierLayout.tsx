import React from "react";
import GameIcon from "../../../display/GameIcon.tsx";
import Slash from "./Slash.tsx";
import type { CardBehaviorDto, SelectorDto } from "@/types/generated/api-types";
import { isEffect, getSelectors } from "@/types/resourceConditions";

interface ValueModifierLayoutProps {
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

const ValueModifierLayout: React.FC<ValueModifierLayoutProps> = ({ behavior }) => {
  const modifiers = (behavior.outputs ?? [])
    .filter((output) => isEffect(output) && output.type === "value-modifier")
    .map((output) => ({
      amount: output.amount ?? 1,
      resources: getResourcesFromSelectors(getSelectors(output) ?? []),
    }))
    .filter((modifier) => modifier.resources.length > 0);

  return (
    <div className="flex flex-col items-center gap-1">
      {modifiers.map((modifier, index) => (
        <div key={index} className="flex gap-[3px] items-center justify-center" data-value-modifier>
          <div className="flex gap-[3px] items-center">
            {modifier.resources.map((resourceType, resourceIndex) => (
              <React.Fragment key={resourceType}>
                {resourceIndex > 0 && <Slash />}
                <GameIcon iconType={resourceType} size="behavior" />
              </React.Fragment>
            ))}
          </div>
          <span className="text-base font-bold text-white mx-[3px]">:</span>
          <span className="text-base font-bold text-white">{modifier.amount < 0 ? "−" : "+"}</span>
          <GameIcon iconType="credit" amount={Math.abs(modifier.amount)} size="behavior" />
        </div>
      ))}
    </div>
  );
};

export default ValueModifierLayout;
