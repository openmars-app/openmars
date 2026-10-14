import { useMemo } from "react";
import {
  AwardRewardDto,
  GameDto,
  ResourceTypeCredit,
  StateErrorDto,
} from "@/types/generated/api-types.ts";
import GameIcon from "../../display/GameIcon.tsx";
import { webSocketService } from "@/services/webSocketService.ts";
import { GamePopoverItem, getThemeStyles } from "../../GamePopover";
import { FormattedDescription } from "../../display/FormattedDescription";
import BehaviorSection from "../../cards/BehaviorSection/BehaviorSection.tsx";
import AwardScoreboard from "../../display/AwardScoreboard.tsx";
import type { ContentDensity } from "./density.ts";
import { canActOnProjects, listGamePlayers } from "./gamePlayers.ts";

interface AwardListProps {
  gameState?: GameDto;
  density: ContentDensity;
  onFlowStart?: () => void;
}

export default function AwardList({ gameState, density, onFlowStart }: AwardListProps) {
  return (
    <AwardListView
      gameState={gameState}
      density={density}
      onFundAward={(id) => {
        onFlowStart?.();
        void webSocketService.fundAward(id);
      }}
    />
  );
}

export function AwardListView({
  gameState,
  density,
  onFundAward,
}: AwardListProps & { onFundAward: (id: string) => void }) {
  const isScreen = density === "screen";
  const canFundAwards = canActOnProjects(gameState);

  const playerAwards = gameState?.currentPlayer?.awards ?? [];
  const globalAwards = gameState?.awards ?? [];

  // For spectators, use global awards as the item source
  const awards =
    playerAwards.length > 0
      ? playerAwards
      : globalAwards.map((a) => ({
          type: a.type,
          name: a.name,
          description: a.description,
          fundingCost: a.fundingCost,
          isFunded: a.isFunded,
          fundedBy: a.fundedBy,
          available: false,
          errors: [] as StateErrorDto[],
        }));

  const allPlayers = useMemo(() => listGamePlayers(gameState), [gameState]);

  const getPlayerName = (playerId: string | undefined): string => {
    if (!playerId) {
      return "Unknown";
    }
    return allPlayers.find((p) => p.id === playerId)?.name ?? "Unknown";
  };

  const handleFundAward = (awardId: string) => {
    if (!canFundAwards) {
      return;
    }
    onFundAward(awardId);
  };

  const smallText = isScreen ? "text-[13px]" : "text-xs";

  return (
    <div
      className={`popover-list flex flex-col gap-2 ${isScreen ? "w-full" : "p-2"}`}
      style={isScreen ? getThemeStyles("colonies") : undefined}
    >
      {awards.map((award) => {
        const isFunded = award.isFunded;
        const isAvailable = award.available && !isFunded;
        const isExecutable = canFundAwards && isAvailable;

        const globalData = globalAwards.find((a) => a.type === award.type);
        const styleColor = globalData?.style?.color ?? "#f39c12";
        const playerProgress = globalData?.playerProgress ?? {};

        const getState = () => {
          if (isFunded) {
            return "claimed" as const;
          }
          if (isAvailable) {
            return "available" as const;
          }
          return "disabled" as const;
        };

        const error =
          !isAvailable && !isFunded && award.errors && award.errors.length > 0
            ? { message: award.errors[0].message, count: award.errors.length }
            : undefined;

        return (
          <GamePopoverItem
            key={award.type}
            className={`popover-list-item ${isScreen ? "min-h-14" : ""}`}
            state={getState()}
            onClick={isExecutable ? () => handleFundAward(award.type) : undefined}
            clickSound={false}
            hoverSound={false}
            error={error}
            statusBadge={isFunded ? "Funded" : undefined}
            borderColor={styleColor}
            style={
              isFunded
                ? {
                    borderColor: styleColor + "BB",
                    background: "#141415",
                  }
                : undefined
            }
          >
            <div className={`flex-1 ${isScreen ? "min-w-0" : ""}`}>
              <div className={`flex items-center gap-2 mb-2 ${isScreen ? "flex-wrap" : ""}`}>
                {globalData?.style?.icon && (
                  <div className="opacity-70 flex items-center">
                    <GameIcon iconType={globalData.style.icon} size="small" />
                  </div>
                )}
                <h3 className="text-white text-sm font-bold font-orbitron m-0">{award.name}</h3>
                {isFunded && award.fundedBy && (
                  <span className={`text-white/50 ${smallText}`}>
                    Funded by{" "}
                    <span style={{ color: allPlayers.find((p) => p.id === award.fundedBy)?.color }}>
                      {getPlayerName(award.fundedBy)}
                    </span>
                  </span>
                )}
              </div>

              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className={`flex items-center gap-2 mb-2 ${isScreen ? "flex-wrap" : ""}`}>
                    <div
                      className="flex items-center gap-2 overflow-hidden transition-all duration-700 ease-in-out"
                      style={
                        isFunded
                          ? { maxWidth: 0, opacity: 0, gap: 0 }
                          : { maxWidth: "100px", opacity: 1 }
                      }
                    >
                      <GameIcon
                        iconType={ResourceTypeCredit}
                        amount={award.fundingCost}
                        size="small"
                      />
                      <span className={`text-white/60 ${smallText}`}>→</span>
                    </div>
                    {(globalData?.rewards ?? []).map((reward: AwardRewardDto) => (
                      <div key={reward.place} className="flex items-center gap-1">
                        <span
                          className={`text-white/50 font-orbitron ${isScreen ? "text-[11px]" : "text-[10px]"}`}
                        >
                          {reward.place === 1 ? "1st" : `${reward.place}nd`}:
                        </span>
                        <div className="[&>div]:items-center [&_div]:justify-start">
                          <BehaviorSection
                            behaviors={[
                              {
                                triggers: [],
                                inputs: [],
                                outputs: reward.outputs,
                              },
                            ]}
                            noContainer
                          />
                        </div>
                      </div>
                    ))}
                  </div>

                  <p className={`text-white/70 leading-relaxed m-0 text-left ${smallText}`}>
                    <FormattedDescription text={award.description} />
                  </p>
                </div>

                <AwardScoreboard players={allPlayers} playerProgress={playerProgress} />
              </div>
            </div>
          </GamePopoverItem>
        );
      })}
    </div>
  );
}
