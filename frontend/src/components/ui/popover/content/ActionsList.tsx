import { GameDto, PlayerActionDto } from "@/types/generated/api-types.ts";
import BehaviorSection from "../../cards/BehaviorSection";
import { canPerformActions } from "@/utils/actionUtils.ts";
import { GamePopoverItem, getThemeStyles } from "../../GamePopover";
import type { PopoverItemError } from "../../GamePopover";
import type { ContentDensity } from "./density.ts";

interface ActionsListProps {
  actions: PlayerActionDto[];
  gameState?: GameDto;
  onActionSelect?: (action: PlayerActionDto) => void;
  density: ContentDensity;
}

function actionError(action: PlayerActionDto): PopoverItemError | undefined {
  if (action.available || !action.errors || action.errors.length === 0) {
    return undefined;
  }
  if (action.errors.some((e) => e.code === "action-already-played")) {
    return undefined;
  }
  return { message: action.errors[0].message, count: action.errors.length };
}

export default function ActionsList({
  actions,
  gameState,
  onActionSelect,
  density,
}: ActionsListProps) {
  const isScreen = density === "screen";
  const hasPendingTileSelection = gameState?.currentPlayer?.pendingTileSelection;
  const canPlayActions = canPerformActions(gameState) && !hasPendingTileSelection;

  if (actions.length === 0) {
    return (
      <div className="flex items-center justify-center py-10 px-5">
        <span className={`font-orbitron text-white/50 ${isScreen ? "text-[13px]" : "text-sm"}`}>
          {isScreen ? "No actions available" : "No actions"}
        </span>
      </div>
    );
  }

  const nameClass = isScreen
    ? "text-[13px]"
    : "text-[11px] opacity-80 max-[768px]:text-[10px] leading-[1.2]";
  const playedClass = isScreen ? "text-[11px] py-0.5 px-2" : "text-[8px] py-0.5 px-1.5";

  return (
    <div
      className={`popover-list popover-list-headed flex flex-col gap-2 ${isScreen ? "w-full" : "p-2"}`}
      style={isScreen ? getThemeStyles("actions") : undefined}
    >
      {actions.map((action, index) => {
        const isAvailable = action.available;
        const isActionPlayable = canPlayActions && isAvailable;
        const isPlayed = action.timesUsedThisGeneration > 0;
        const error = actionError(action);

        return (
          <GamePopoverItem
            key={`${action.cardId}-${action.behaviorIndex}`}
            state={isAvailable ? "available" : "disabled"}
            onClick={isActionPlayable ? () => onActionSelect?.(action) : undefined}
            clickSound={false}
            error={error}
            animationDelay={index * 0.05}
            className={`popover-list-item ${isScreen ? "min-h-14" : ""} ${!isActionPlayable && isAvailable ? "cursor-default" : ""} ${isPlayed ? "grayscale saturate-0" : ""}`}
          >
            <div className={`flex flex-col gap-2 flex-1 ${isScreen ? "min-w-0" : ""}`}>
              <div
                className={`text-white/70 font-medium uppercase tracking-[0.5px] [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] flex items-center gap-2 ${nameClass}`}
              >
                {action.cardName}
                {isPlayed && (
                  <span
                    className={`bg-[linear-gradient(135deg,rgba(120,120,120,0.8)_0%,rgba(80,80,80,0.9)_100%)] text-white/90 font-semibold uppercase tracking-[0.3px] rounded-lg border border-[rgba(120,120,120,0.6)] [text-shadow:none] opacity-100 ${playedClass}`}
                  >
                    played
                  </span>
                )}
              </div>

              <div className="relative w-full min-h-[32px] [&>div]:!relative [&>div]:!bottom-auto [&>div]:!left-auto [&>div]:!right-auto [&>div]:w-full [&>div:hover]:!transform-none [&>div:hover]:!shadow-none [&>div:hover]:!filter-none">
                <BehaviorSection
                  behaviors={[action.behavior]}
                  sourceBehaviorIndex={action.behaviorIndex}
                  computedValues={action.computedValues}
                  playerResources={gameState?.currentPlayer?.resources}
                  resourceStorage={gameState?.currentPlayer?.resourceStorage}
                  cardId={action.cardId}
                  greyOutAll={isPlayed}
                />
              </div>
            </div>
          </GamePopoverItem>
        );
      })}
    </div>
  );
}
