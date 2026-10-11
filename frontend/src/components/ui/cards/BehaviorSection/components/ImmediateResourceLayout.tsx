import { BehaviorArrow, BehaviorSign } from "./BehaviorIcon";
import React from "react";
import ResourceDisplay from "./ResourceDisplay";
import CardIcon, { TaggedCardIcon } from "./CardIcon";
import OrChip from "./OrChip";
import Slash from "./Slash";
import ChoiceRequirementBox from "./ChoiceRequirementBox";
import { useBehaviorLayout, BehaviorBorderLabel } from "./BehaviorContainer";
import {
  analyzeCardOutputs,
  coordinateDisplayModes,
  orderIndependentResources,
} from "../utils/displayAnalysis";
import type { IconDisplayInfo, TileScaleInfo } from "../types";
import type { CalculatedOutputDto, CardBehaviorDto, ChoiceDto } from "@/types/generated/api-types";
import {
  type ResourceCondition,
  isProduction,
  isCardOperation,
  getPer,
  getSelectors,
  isBasicResource,
  getVariableAmount,
} from "@/types/resourceConditions";

interface ImmediateResourceLayoutProps {
  behavior: CardBehaviorDto;
  isResourceAffordable: (resource: ResourceCondition, isInput: boolean) => boolean;
  tileScaleInfo: TileScaleInfo;
  computedOutputs?: CalculatedOutputDto[];
  productionLabel?: React.ReactNode;
}

const productionClass =
  "border border-[rgba(160,110,60,0.5)] bg-[linear-gradient(135deg,rgba(160,110,60,0.4),rgba(139,89,42,0.35))] px-1.5 py-[3px] max-w-full";

export default function ImmediateResourceLayout({
  behavior,
  isResourceAffordable,
  tileScaleInfo,
  computedOutputs,
  productionLabel,
}: ImmediateResourceLayoutProps) {
  const { compact } = useBehaviorLayout();
  const choices = (
    behavior.optional
      ? [
          {
            originalIndex: 0,
            available: true,
            errors: [],
            inputs: behavior.inputs,
            outputs: behavior.outputs,
            stateRequirements: behavior.stateRequirements,
          } as ChoiceDto,
        ]
      : (behavior.choices ?? [])
  ).filter((choice) => (choice.inputs?.length ?? 0) + (choice.outputs?.length ?? 0) > 0);
  const optional = behavior.optional === true;
  const compactExchange =
    choices.length === 1 &&
    choices[0].inputs?.length === 1 &&
    choices[0].outputs?.length === 1 &&
    choices[0].outputs[0].type === "state-change";
  const numberedChoices =
    choices.length >= 3 ||
    choices.some((choice) =>
      choice.outputs?.some(
        (resource) => !resource.type.startsWith("credit") && Math.abs(resource.amount ?? 1) > 3,
      ),
    );

  const renderResource = (
    resource: ResourceCondition,
    displayInfo: IconDisplayInfo,
    grouped = false,
    input = false,
  ) => (
    <ResourceDisplay
      resource={resource}
      displayInfo={displayInfo}
      isInput={input}
      isGroupedWithOtherNegatives={grouped}
      context={input ? "action" : "standalone"}
      isAffordable={isResourceAffordable(resource, input)}
      tileScaleInfo={tileScaleInfo}
      computedOutputs={computedOutputs}
      productionLabel={getPer(resource) ? productionLabel : undefined}
    />
  );

  const renderSignedGroup = (resources: ResourceCondition[], production: boolean) => {
    const negative = resources.filter((resource) => (resource.amount ?? 1) < 0);
    const positive = resources.filter((resource) => (resource.amount ?? 1) >= 0);
    const paired = negative.length > 0 && positive.length > 0;
    const modes = coordinateDisplayModes(resources, compact, paired);
    const numberedResources = resources.filter(
      (resource) =>
        !resource.type.startsWith("credit") && modes.get(resource)?.displayMode === "number",
    );
    const quantityWidth =
      paired && numberedResources.length
        ? Math.max(
            ...numberedResources.map(
              (resource) =>
                String(getVariableAmount(resource) ? "X" : Math.abs(resource.amount ?? 1)).length,
            ),
          )
        : 0;
    const quantityColumns = quantityWidth
      ? ({
          "--behavior-quantity-width": `${quantityWidth}ch`,
          "--behavior-credit-inset": `calc(${quantityWidth}ch + 3px)`,
        } as React.CSSProperties)
      : undefined;
    const rows = negative.length ? [negative, positive].filter((row) => row.length) : [positive];
    return (
      <div
        className={
          production
            ? `${productionClass} relative ${productionLabel ? "mt-2.5 pt-3" : ""}`
            : "max-w-full"
        }
        style={quantityColumns}
      >
        {production && productionLabel && (
          <BehaviorBorderLabel>{productionLabel}</BehaviorBorderLabel>
        )}
        <div className="flex flex-col items-stretch gap-[3px]">
          {rows.map((row, index) => (
            <div key={index} className="flex items-center gap-[3px] max-w-full">
              {negative.length > 0 && <BehaviorSign positive={index > 0} />}
              <div className="flex flex-wrap items-center gap-[3px]">
                {orderIndependentResources(row, compact).map((resource, resourceIndex) => (
                  <React.Fragment key={resourceIndex}>
                    {renderResource(resource, modes.get(resource)!, negative.length > 0)}
                  </React.Fragment>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  };

  const renderOutputs = (outputs: ResourceCondition[], choiceOutput = false) => {
    const production = outputs.filter((resource) => isProduction(resource) && !getPer(resource));
    const perProduction = outputs.filter((resource) => isProduction(resource) && getPer(resource));
    const cards = outputs.filter(
      (resource) =>
        isCardOperation(resource) && !getPer(resource) && !getSelectors(resource)?.length,
    );
    const others = outputs.filter(
      (resource) => !isProduction(resource) && !cards.includes(resource),
    );
    const basic = others.filter(isBasicResource);
    const hasResourcePair =
      basic.some((resource) => resource.amount < 0) &&
      basic.some((resource) => resource.amount > 0);
    const independent = hasResourcePair
      ? others.filter((resource) => !basic.includes(resource as (typeof basic)[number]))
      : others;
    const modes = coordinateDisplayModes(independent, compact);
    if (numberedChoices && choiceOutput) {
      for (const [resource, info] of modes) {
        if (Math.abs(resource.amount ?? 1) > 1) {
          modes.set(resource, { ...info, displayMode: "number", iconCount: 1 });
        }
      }
    }
    const cardGroups = analyzeCardOutputs(cards);
    return (
      <div className="behavior-flow flex flex-wrap items-center justify-center gap-x-2 gap-y-1 max-w-full">
        {production.length > 0 && renderSignedGroup(production, true)}
        {perProduction.map((resource, index) => (
          <React.Fragment key={`per-${index}`}>
            {renderResource(resource, coordinateDisplayModes([resource], compact).get(resource)!)}
          </React.Fragment>
        ))}
        {hasResourcePair && renderSignedGroup(basic, false)}
        {orderIndependentResources(independent, compact).map((resource, index) => (
          <React.Fragment key={`resource-${index}`}>
            {renderResource(resource, modes.get(resource)!)}
          </React.Fragment>
        ))}
        {cardGroups.map((card, index) => (
          <React.Fragment key={`cards-${index}`}>
            {index > 0 &&
              cardGroups[index - 1].badgeType === "discard" &&
              card.badgeType !== "discard" && <BehaviorArrow />}
            <CardIcon
              {...card}
              isAffordable={cards.every((resource) => isResourceAffordable(resource, false))}
            />
          </React.Fragment>
        ))}
      </div>
    );
  };

  const sharedSelectors = choices.map((choice) => {
    const resources = choice.outputs ?? [];
    if (!resources.length || resources.some((resource) => resource.target !== "any-card")) {
      return undefined;
    }
    const tags = getSelectors(resources[0])?.flatMap((selector) => selector.tags ?? []) ?? [];
    if (
      !tags.length ||
      resources.some(
        (resource) =>
          JSON.stringify(getSelectors(resource)) !== JSON.stringify(getSelectors(resources[0])),
      )
    ) {
      return undefined;
    }
    return { tags, selectors: getSelectors(resources[0]) };
  });
  const commonTarget = sharedSelectors[0];
  const shareTarget =
    !!commonTarget &&
    sharedSelectors.every((target) => JSON.stringify(target) === JSON.stringify(commonTarget));

  const choiceContent = (
    <div
      className={`behavior-flow flex ${choices.some((choice) => (choice.outputs?.length ?? 0) > 2) ? "flex-col" : "flex-wrap"} items-center justify-center gap-[3px] max-w-full`}
    >
      {choices.map((choice, index) => (
        <div
          key={index}
          data-choice-index={behavior.choices?.indexOf(choice)}
          className="flex items-center gap-[3px] max-w-full"
        >
          {index > 0 && (choices.length >= 3 ? <Slash /> : <OrChip />)}
          <ChoiceRequirementBox
            requirements={choice.requirements}
            stateRequirements={compactExchange ? undefined : choice.stateRequirements}
          >
            <div
              className={`flex ${compactExchange ? "flex-nowrap" : "flex-wrap"} items-center justify-center gap-1 max-w-full`}
              data-compact-exchange={compactExchange || undefined}
            >
              {choice.inputs?.map((resource, inputIndex) => (
                <React.Fragment key={inputIndex}>
                  {renderResource(
                    resource,
                    coordinateDisplayModes([resource], compact || compactExchange).get(resource)!,
                    false,
                    true,
                  )}
                </React.Fragment>
              ))}
              {!!choice.inputs?.length && !!choice.outputs?.length && <BehaviorArrow />}
              {renderOutputs(
                shareTarget
                  ? (choice.outputs ?? []).map((resource) => {
                      if ("selectors" in resource) {
                        return Object.assign({}, resource, { selectors: undefined });
                      }
                      return resource;
                    })
                  : (choice.outputs ?? []),
                true,
              )}
              {compactExchange && (optional || Boolean(choice.stateRequirements?.length)) && (
                <span data-rule-exception className="font-bold">
                  *
                </span>
              )}
            </div>
          </ChoiceRequirementBox>
        </div>
      ))}
    </div>
  );

  return (
    <div
      className="flex flex-col items-center gap-1 max-w-full min-w-0"
      data-optional-choice={optional || undefined}
    >
      {optional && <span className="font-orbitron text-[9px] text-white/70">MAY</span>}
      {choices.length > 0 &&
        (shareTarget ? (
          <div
            className="border border-dashed border-white/30 px-1.5 py-1 flex flex-col items-center gap-1 max-w-full"
            aria-label={`Target a ${commonTarget.tags.join(" and ")} card`}
          >
            <TaggedCardIcon tags={commonTarget.tags} />
            {choiceContent}
          </div>
        ) : (
          choiceContent
        ))}
      {!optional && !!behavior.outputs?.length && renderOutputs(behavior.outputs)}
    </div>
  );
}
