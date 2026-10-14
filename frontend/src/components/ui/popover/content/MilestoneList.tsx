import React, { useMemo } from "react";
import {
  AwardRewardDto,
  GameDto,
  ResourceTypeCredit,
  StateErrorDto,
} from "@/types/generated/api-types.ts";
import GameIcon from "../../display/GameIcon.tsx";
import { webSocketService } from "@/services/webSocketService.ts";
import { GamePopoverItem, getThemeStyles } from "../../GamePopover";
import type { PopoverItemError, PopoverItemInfo } from "../../GamePopover/types.ts";
import { FormattedDescription } from "../../display/FormattedDescription";
import BehaviorSection from "../../cards/BehaviorSection/BehaviorSection.tsx";
import type { ContentDensity } from "./density.ts";
import { canActOnProjects, listGamePlayers } from "./gamePlayers.ts";

interface MilestoneListProps {
  gameState?: GameDto;
  density: ContentDensity;
  onFlowStart?: () => void;
}

function milestoneError(errors: StateErrorDto[] | undefined): PopoverItemError | undefined {
  if (!errors?.length || errors.some((e) => e.category === "requirement")) {
    return undefined;
  }
  return { message: errors[0].message, count: errors.length };
}

function milestoneInfo(errors: StateErrorDto[] | undefined): PopoverItemInfo | undefined {
  const reqError = errors?.find((e) => e.category === "requirement");
  if (!reqError) {
    return undefined;
  }
  return { message: reqError.message };
}

export default function MilestoneList({ gameState, density, onFlowStart }: MilestoneListProps) {
  return (
    <MilestoneListView
      gameState={gameState}
      density={density}
      onClaimMilestone={(id) => {
        onFlowStart?.();
        void webSocketService.claimMilestone(id);
      }}
    />
  );
}

export function MilestoneListView({
  gameState,
  density,
  onClaimMilestone,
}: MilestoneListProps & { onClaimMilestone: (id: string) => void }) {
  const isScreen = density === "screen";
  const canClaimMilestones = canActOnProjects(gameState);

  const playerMilestones = gameState?.currentPlayer?.milestones ?? [];
  const globalMilestones = gameState?.milestones ?? [];

  const milestones =
    playerMilestones.length > 0
      ? playerMilestones
      : globalMilestones.map((m) => ({
          type: m.type,
          name: m.name,
          description: m.description,
          claimCost: m.claimCost,
          isClaimed: m.isClaimed,
          claimedBy: m.claimedBy,
          available: false,
          progress: 0,
          required: m.required,
          errors: [] as StateErrorDto[],
        }));

  const allPlayers = useMemo(() => listGamePlayers(gameState), [gameState]);

  const longestNameLength = useMemo(() => {
    return allPlayers.reduce((max, p) => Math.max(max, p.name.length), 0);
  }, [allPlayers]);

  const getPlayerName = (playerId: string | undefined): string => {
    if (!playerId) {
      return "Unknown";
    }
    return allPlayers.find((p) => p.id === playerId)?.name ?? "Unknown";
  };

  const handleClaimMilestone = (milestoneId: string) => {
    if (!canClaimMilestones) {
      return;
    }
    onClaimMilestone(milestoneId);
  };

  const smallText = isScreen ? "text-[13px]" : "text-xs";

  return (
    <div
      className={`popover-list flex flex-col gap-2 ${isScreen ? "w-full" : "p-2"}`}
      style={isScreen ? getThemeStyles("colonies") : undefined}
    >
      {milestones.map((milestone) => {
        const isClaimed = milestone.isClaimed;
        const isAvailable = milestone.available && !isClaimed;
        const isExecutable = canClaimMilestones && isAvailable;
        const meetsRequirement =
          canClaimMilestones &&
          !isClaimed &&
          !(milestone.errors ?? []).some((e) => e.category === "requirement");

        const globalData = globalMilestones.find((m) => m.type === milestone.type);
        const styleColor = globalData?.style?.color ?? "#ff6b35";
        const playerProgress = globalData?.playerProgress ?? {};
        const required = globalData?.required ?? 0;

        const sortedPlayers = [...allPlayers].sort(
          (a, b) => (playerProgress[b.id] ?? 0) - (playerProgress[a.id] ?? 0),
        );

        const getState = () => {
          if (isClaimed) {
            return "claimed" as const;
          }
          if (isAvailable) {
            return "available" as const;
          }
          return "disabled" as const;
        };

        const showsReasons = !isAvailable && !isClaimed;
        const error = showsReasons ? milestoneError(milestone.errors) : undefined;
        const info = showsReasons ? milestoneInfo(milestone.errors) : undefined;

        return (
          <GamePopoverItem
            key={milestone.type}
            state={getState()}
            className={`popover-list-item ${meetsRequirement ? "milestone-eligible-glow" : ""} ${isScreen ? "min-h-14" : ""}`}
            onClick={isExecutable ? () => handleClaimMilestone(milestone.type) : undefined}
            clickSound={false}
            hoverSound={false}
            error={error}
            info={info}
            statusBadge={isClaimed ? "Claimed" : undefined}
            borderColor={styleColor}
            style={
              {
                ...(isClaimed
                  ? {
                      borderColor: styleColor + "BB",
                      background: "#141415",
                    }
                  : {}),
                "--milestone-glow-rgb": `${parseInt(styleColor.slice(1, 3), 16)}, ${parseInt(styleColor.slice(3, 5), 16)}, ${parseInt(styleColor.slice(5, 7), 16)}`,
              } as React.CSSProperties
            }
          >
            <div className={`flex-1 ${isScreen ? "min-w-0" : ""}`}>
              <div className={`flex items-center gap-2 mb-2 ${isScreen ? "flex-wrap" : ""}`}>
                {globalData?.style?.icon && (
                  <div className="opacity-70 flex items-center">
                    <GameIcon iconType={globalData.style.icon} size="small" />
                  </div>
                )}
                <h3 className="text-white text-sm font-bold font-orbitron m-0">{milestone.name}</h3>
                {isClaimed && milestone.claimedBy && (
                  <span className={`text-white/50 ${smallText}`}>
                    Claimed by{" "}
                    <span
                      style={{
                        color: allPlayers.find((p) => p.id === milestone.claimedBy)?.color,
                      }}
                    >
                      {getPlayerName(milestone.claimedBy)}
                    </span>
                  </span>
                )}
              </div>

              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-2">
                    <div
                      className="flex items-center gap-2 overflow-hidden transition-all duration-700 ease-in-out"
                      style={
                        isClaimed
                          ? { maxWidth: 0, opacity: 0, gap: 0 }
                          : { maxWidth: "100px", opacity: 1 }
                      }
                    >
                      <GameIcon
                        iconType={ResourceTypeCredit}
                        amount={milestone.claimCost}
                        size="small"
                      />
                      <span className={`text-white/60 ${smallText}`}>→</span>
                    </div>
                    {(globalData?.rewards ?? []).map((reward: AwardRewardDto, idx: number) => (
                      <div key={idx} className="[&>div]:items-center [&_div]:justify-start">
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
                    ))}
                  </div>

                  <p className={`text-white/70 leading-relaxed m-0 text-left ${smallText}`}>
                    <FormattedDescription text={milestone.description} />
                  </p>
                </div>

                <div
                  className="flex-shrink-0 grid grid-cols-[auto_auto] gap-x-3 gap-y-1"
                  style={{ minWidth: `${longestNameLength + 5}ch` }}
                >
                  {sortedPlayers.map((player) => {
                    const progress = playerProgress[player.id] ?? 0;
                    const met = required > 0 && progress >= required;
                    return (
                      <React.Fragment key={player.id}>
                        <div className="flex items-center gap-2 text-sm">
                          <span
                            className="w-2 h-2 rounded-full flex-shrink-0"
                            style={{ backgroundColor: player.color }}
                          />
                          <span className="text-white/80">{player.name}</span>
                        </div>
                        <span
                          className={`text-sm font-orbitron font-semibold ${met ? "text-green-400" : "text-white/50"}`}
                        >
                          {progress}/{required}
                        </span>
                      </React.Fragment>
                    );
                  })}
                </div>
              </div>
            </div>
          </GamePopoverItem>
        );
      })}
    </div>
  );
}
