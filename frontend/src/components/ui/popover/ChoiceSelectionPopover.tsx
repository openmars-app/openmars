import React from "react";
import { ChoiceDto, CardBehaviorDto, ResourcesDto } from "../../../types/generated/api-types.ts";
import BehaviorSection from "../cards/BehaviorSection";
import { renderRequirementItems } from "../cards/BehaviorSection/components/ChoiceRequirementBox.tsx";
import GameButton from "../buttons/GameButton.tsx";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "./GameFlowPopover.tsx";

interface ChoiceItem {
  index: number;
  choice: ChoiceDto;
}

interface ChoiceSelectionPopoverProps {
  cardId: string;
  cardName: string;
  behaviors: CardBehaviorDto[];
  behaviorIndex: number;
  onChoiceSelect: (choiceIndex: number) => void;
  onCancel: () => void;
  isVisible: boolean;
  isAction?: boolean;
  playerResources?: ResourcesDto;
  resourceStorage?: { [key: string]: number };
}

const ChoiceSelectionPopover: React.FC<ChoiceSelectionPopoverProps> = ({
  cardId,
  cardName,
  behaviors,
  behaviorIndex,
  onChoiceSelect,
  onCancel,
  isVisible,
  isAction = false,
  playerResources,
  resourceStorage,
}) => {
  const behavior = behaviors?.[behaviorIndex];
  const choices: ChoiceItem[] =
    behavior?.choices?.map((choice, index) => ({
      index,
      choice,
    })) || [];

  const isChoiceAffordable = (choice: ChoiceDto): boolean => {
    if (!playerResources) {
      return true;
    }

    const storage = resourceStorage || {};
    const inputs = [...(behavior?.inputs || []), ...(choice.inputs || [])];

    for (const input of inputs) {
      switch (input.type) {
        case "credit":
          if (playerResources.credits < input.amount) {
            return false;
          }
          break;
        case "steel":
          if (playerResources.steel < input.amount) {
            return false;
          }
          break;
        case "titanium":
          if (playerResources.titanium < input.amount) {
            return false;
          }
          break;
        case "plant":
          if (playerResources.plants < input.amount) {
            return false;
          }
          break;
        case "energy":
          if (playerResources.energy < input.amount) {
            return false;
          }
          break;
        case "heat":
          if (playerResources.heat < input.amount) {
            return false;
          }
          break;
        case "animal":
        case "microbe":
        case "floater":
        case "science":
        case "asteroid":
          if (input.target === "self-card") {
            const cardStorage = storage[cardId] || 0;
            if (cardStorage < input.amount) {
              return false;
            }
          }
          break;
      }
    }

    return true;
  };

  const handleChoiceClick = (choiceIndex: number) => {
    const choice = choices[choiceIndex]?.choice;
    const indexToSend = choice?.originalIndex ?? choiceIndex;
    onChoiceSelect(indexToSend);
  };

  if (choices.length === 0) {
    return null;
  }

  return (
    <GameFlowPopover isVisible={isVisible} onClose={onCancel}>
      <GameFlowTitle>
        <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
          {isAction ? "Choose Action" : "Choose One Effect"}
        </h3>
        <div className="text-white/60 text-xs compact:text-[13px] text-shadow-glow mt-1">
          {cardName}
        </div>
      </GameFlowTitle>

      <GameFlowBody>
        {choices.map(({ index, choice }) => {
          const behaviorForChoice: CardBehaviorDto = {
            triggers: [{ type: "manual" }],
            inputs: choice.inputs,
            outputs: choice.outputs,
            choices: undefined,
          };

          const delay = index * 0.05;
          const isAffordable = isChoiceAffordable(choice);
          const hasBackendErrors = choice.errors && choice.errors.length > 0;
          const isSelectable = choice.available && isAffordable && !hasBackendErrors;

          return (
            <button
              type="button"
              key={index}
              disabled={!isSelectable}
              className={`
                relative block w-full text-left
                game-panel game-panel-clipped game-choice
                px-3.5 py-3
                mb-2 compact:min-h-11
                transition-all duration-[250ms] ease-out
                animate-choiceSlideIn
                ${
                  isSelectable ? "cursor-pointer hover:brightness-125" : "cursor-default opacity-60"
                }
              `}
              style={{ animationDelay: `${delay}s` }}
              onClick={() => handleChoiceClick(index)}
            >
              {hasBackendErrors && (
                <div className="popover-status popover-status-error absolute top-2 right-2">
                  <span className="max-w-[140px] truncate">{choice.errors[0].message}</span>
                </div>
              )}
              <div className="text-white/60 text-[11px] compact:text-[13px] font-semibold uppercase tracking-wider mb-1 text-shadow-glow">
                Choice {index + 1}
              </div>
              {choice.requirements &&
                choice.requirements.items &&
                choice.requirements.items.length > 0 && (
                  <div
                    className={`flex items-center gap-1 mb-2 text-[11px] compact:text-[13px] font-semibold ${isSelectable ? "text-white/70" : "text-red-400/80"}`}
                  >
                    <span>Requires:</span>
                    {renderRequirementItems(choice.requirements.items)}
                  </div>
                )}
              <div className="flex items-center justify-center w-full">
                <BehaviorSection
                  behaviors={[behaviorForChoice]}
                  playerResources={playerResources}
                  resourceStorage={resourceStorage}
                  cardId={cardId}
                  greyOutAll={!isSelectable}
                  hideActionChip
                  noContainer
                />
              </div>
            </button>
          );
        })}
      </GameFlowBody>

      <GameFlowFooter>
        <GameButton emphasis="secondary" tone="info" size="sm" onClick={onCancel}>
          Cancel
        </GameButton>
      </GameFlowFooter>
    </GameFlowPopover>
  );
};

export default ChoiceSelectionPopover;
