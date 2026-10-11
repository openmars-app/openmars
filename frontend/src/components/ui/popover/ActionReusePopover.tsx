import React from "react";
import { PlayerActionDto, GameDto } from "../../../types/generated/api-types.ts";
import BehaviorSection from "../cards/BehaviorSection";
import GameButton from "../buttons/GameButton.tsx";
import { GamePopoverItem, getThemeStyles } from "../GamePopover";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "./GameFlowPopover.tsx";

interface ActionReusePopoverProps {
  isVisible: boolean;
  onClose: () => void;
  actions: PlayerActionDto[];
  reuseSourceCardId: string;
  onActionSelect: (action: PlayerActionDto) => void;
  gameState?: GameDto;
}

const ActionReusePopover: React.FC<ActionReusePopoverProps> = ({
  isVisible,
  onClose,
  actions,
  reuseSourceCardId,
  onActionSelect,
  gameState,
}) => {
  const source = actions.find((action) => action.cardId === reuseSourceCardId);
  const options = source?.reuseOptions ?? [];
  const usedActions = actions.filter((action) =>
    options.some(
      (option) => option.cardId === action.cardId && option.behaviorIndex === action.behaviorIndex,
    ),
  );
  const getOption = (action: PlayerActionDto) =>
    options.find(
      (option) => option.cardId === action.cardId && option.behaviorIndex === action.behaviorIndex,
    );
  const availableCount = options.filter((option) => option.available).length;

  const handleActionClick = (action: PlayerActionDto) => {
    onActionSelect(action);
  };

  return (
    <GameFlowPopover isVisible={isVisible} onClose={onClose} className="w-[320px]!">
      <GameFlowTitle className="flex items-center justify-between gap-3">
        <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
          Reuse Action
        </h3>
        <span className="text-white/80 text-xs compact:text-[13px] bg-white/10 py-1 px-2 border border-white/20">
          {availableCount} available
        </span>
      </GameFlowTitle>
      <GameFlowBody>
        {usedActions.length === 0 ? (
          <div className="flex items-center justify-center py-10 px-5">
            <span className="font-orbitron text-sm text-white/50">No used actions to reuse</span>
          </div>
        ) : (
          <div className="flex flex-col gap-2" style={getThemeStyles("actions")}>
            {usedActions.map((action, index) => {
              const option = getOption(action);
              const reuseErrors = option?.errors ?? [];
              const isReuseAvailable = option?.available ?? false;

              return (
                <GamePopoverItem
                  clickSound={false}
                  key={`${action.cardId}-${action.behaviorIndex}`}
                  state={isReuseAvailable ? "available" : "disabled"}
                  onClick={isReuseAvailable ? () => handleActionClick(action) : undefined}
                  error={
                    !isReuseAvailable && reuseErrors.length > 0
                      ? { message: reuseErrors[0].message, count: reuseErrors.length }
                      : undefined
                  }
                  animationDelay={index * 0.05}
                  className="compact:min-h-11"
                >
                  <div className="flex flex-col gap-2 flex-1">
                    <div className="text-white/70 text-[11px] compact:text-[13px] font-medium uppercase tracking-[0.5px] [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-[1.2] opacity-80 flex items-center gap-2">
                      {action.cardName}
                    </div>

                    <div className="relative w-full min-h-[32px] [&>div]:!relative [&>div]:!bottom-auto [&>div]:!left-auto [&>div]:!right-auto [&>div]:w-full [&>div:hover]:!transform-none [&>div:hover]:!shadow-none [&>div:hover]:!filter-none">
                      <BehaviorSection
                        behaviors={[action.behavior]}
                        sourceBehaviorIndex={action.behaviorIndex}
                        computedValues={action.computedValues}
                        playerResources={gameState?.currentPlayer?.resources}
                        resourceStorage={gameState?.currentPlayer?.resourceStorage}
                        cardId={action.cardId}
                      />
                    </div>
                  </div>
                </GamePopoverItem>
              );
            })}
          </div>
        )}
      </GameFlowBody>
      <GameFlowFooter>
        <GameButton emphasis="secondary" tone="info" size="sm" onClick={onClose}>
          Cancel
        </GameButton>
      </GameFlowFooter>
    </GameFlowPopover>
  );
};

export default ActionReusePopover;
