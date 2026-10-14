import type { PendingAwardFundSelectionDto, GameDto } from "@/types/generated/api-types.ts";
import { webSocketService } from "@/services/webSocketService.ts";
import { useResolutionFailure, type ResolutionFailure } from "@/hooks/useResolutionFailure";
import { usePendingSubmission } from "@/hooks/usePendingSubmission";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "./GameFlowPopover.tsx";
import GameButton from "../buttons/GameButton";
import GameIcon from "../display/GameIcon";
import GamePopoverEmpty from "../GamePopover/GamePopoverEmpty";

interface Props {
  isOpen: boolean;
  selection: PendingAwardFundSelectionDto;
  gameState?: GameDto;
}

export default function AwardFundSelectionPopover(props: Props) {
  const failure = useResolutionFailure();
  return (
    <AwardFundSelectionPopoverView
      {...props}
      failure={failure}
      onSelect={(id) => webSocketService.confirmAwardFund(id)}
      onContinue={() => webSocketService.continueSelection("action.confirm-award-fund")}
    />
  );
}

export function AwardFundSelectionPopoverView({
  isOpen,
  selection,
  gameState,
  onSelect,
  onContinue,
  failure,
}: Props & {
  onSelect: (awardType: string) => unknown | Promise<unknown>;
  onContinue: () => unknown | Promise<unknown>;
  failure?: ResolutionFailure | null;
}) {
  const { submitting, error, submit } = usePendingSubmission(
    failure,
    "action.confirm-award-fund",
    JSON.stringify(selection),
  );
  const empty = selection.canContinueWithoutSelection;
  return (
    <GameFlowPopover isVisible={isOpen} type="interactive-mandatory">
      <GameFlowTitle>
        <h3 className="m-0 font-orbitron text-white text-base font-bold">Fund an Award</h3>
        {!empty && (
          <p className="text-white/60 text-xs compact:text-[13px] mt-1">
            Choose an award to fund for free
          </p>
        )}
      </GameFlowTitle>
      <GameFlowBody>
        {empty ? (
          <GamePopoverEmpty
            icon={<GameIcon iconType="award" size="large" />}
            title="No awards available"
            description="There are no awards you can fund. Continue without funding an award."
          />
        ) : (
          selection.availableAwards.map((id) => {
            const award = gameState?.awards.find((candidate) => candidate.type === id);
            return (
              <GameButton
                key={id}
                emphasis="secondary"
                disabled={submitting}
                className="w-full mb-2 px-3.5 py-3 text-left compact:min-h-11"
                onClick={() => void submit(() => onSelect(id))}
              >
                <span className="flex items-center gap-3 w-full">
                  <GameIcon iconType={award?.style?.icon ?? "award"} size="small" />
                  <span className="min-w-0">
                    <span className="block text-white text-sm font-bold font-orbitron">
                      {award?.name ?? id}
                    </span>
                    <span className="block text-white/60 text-xs compact:text-[13px] font-sans mt-1">
                      {award?.description}
                    </span>
                  </span>
                </span>
              </GameButton>
            );
          })
        )}
        {error && (
          <p role="alert" className="text-red-300 text-sm mt-2">
            {error}
          </p>
        )}
      </GameFlowBody>
      {empty && (
        <GameFlowFooter>
          <GameButton disabled={submitting} onClick={() => void submit(onContinue)}>
            Continue
          </GameButton>
        </GameFlowFooter>
      )}
    </GameFlowPopover>
  );
}
