import { usePendingSubmission } from "@/hooks/usePendingSubmission";
import type { ResolutionFailure } from "@/hooks/useResolutionFailure";
import GamePopoverEmpty from "../GamePopover/GamePopoverEmpty";
import GameIcon from "../display/GameIcon";
import React, { useState, useMemo } from "react";
import { PendingColonySelectionDto, ColonyDto } from "@/types/generated/api-types.ts";
import ColonySteps from "../popover/ColonySteps.tsx";
import GameButton from "../buttons/GameButton.tsx";
import ColonyOutputDisplay from "../display/ColonyOutputDisplay.tsx";
import { PlayerInfo } from "@/utils/colonyUtils.ts";
import { Z_INDEX, getZIndex } from "@/constants/zIndex.ts";
import {
  COLONY_TRACK_INNER_CLASS,
  COLONY_TRACK_SCROLL_CLASS,
  OVERLAY_ROOT_SAFE_AREA_CLASS,
} from "./overlayStyles.ts";

interface ColonySelectionOverlayProps {
  isOpen: boolean;
  pendingSelection: PendingColonySelectionDto;
  colonies: ColonyDto[];
  allPlayers: PlayerInfo[];
  onContinue: () => void | Promise<unknown>;
  failure?: ResolutionFailure | null;
  onConfirm: (colonyId: string) => void | Promise<unknown>;
}

const ColonySelectionOverlay: React.FC<ColonySelectionOverlayProps> = ({
  isOpen,
  pendingSelection,
  colonies,
  allPlayers,
  onConfirm,
  onContinue,
  failure,
}) => {
  const { submitting, error, submit } = usePendingSubmission(
    failure,
    "action.confirm-colony-placement",
    JSON.stringify(pendingSelection),
  );
  const empty = pendingSelection.canContinueWithoutSelection;
  const [selectedColonyId, setSelectedColonyId] = useState<string | null>(null);

  const selectableIds = useMemo(
    () => new Set(pendingSelection.availableColonyIds),
    [pendingSelection],
  );

  const getPlayerColor = (playerId: string): string => {
    return allPlayers.find((p) => p.id === playerId)?.color ?? "#666";
  };

  const getPlayerName = (playerId: string): string => {
    return allPlayers.find((p) => p.id === playerId)?.name ?? "Unknown";
  };

  if (!isOpen) {
    return null;
  }

  return (
    <div
      className={`fixed inset-0 flex items-center justify-center ${OVERLAY_ROOT_SAFE_AREA_CLASS}`}
      style={{ zIndex: Z_INDEX.SELECTION_POPOVER }}
    >
      <div className="absolute inset-0 backdrop-blur-sm" />
      <div className="absolute inset-0 bg-black/60 animate-[fadeIn_0.3s_ease]" />

      <div
        className="relative w-[640px] max-w-[calc(100vw-32px)] max-h-[80dvh] compact:max-h-full flex flex-col game-panel game-panel-clipped game-window overflow-hidden"
        style={{ zIndex: getZIndex("LOCAL", 1) }}
      >
        <div className="px-4 py-3 border-b border-white/10 flex items-center justify-between">
          <h2 className="font-orbitron text-base font-bold text-white tracking-wider m-0">
            {pendingSelection.addTile ? "Add colony tile" : "Place Colony"}
          </h2>
          <span className="text-xs text-white/40">{pendingSelection.source}</span>
        </div>

        <div className="flex-1 overflow-y-auto p-3 space-y-2">
          {empty ? (
            <GamePopoverEmpty
              icon={<GameIcon iconType="colony" size="large" />}
              title="No colony locations available"
              description="There are no legal choices for this effect. Continue without resolving it."
            />
          ) : (
            colonies.map((colony) => {
              const selectable = selectableIds.has(colony.id);
              const isSelected = selectedColonyId === colony.id;
              const nextSlotIndex = colony.playerColonies.length;
              const reward = colony.colonies[nextSlotIndex]?.reward ?? [];

              return (
                <GameButton
                  emphasis="quiet"
                  key={colony.id}
                  type="button"
                  className={`w-full text-left px-3 py-2.5 rounded-none border transition-all ${
                    !selectable
                      ? "border-white/5 bg-white/[0.01] opacity-40 cursor-default"
                      : isSelected
                        ? "border-space-blue-500 bg-space-blue-900/60 cursor-pointer"
                        : "border-white/10 bg-white/[0.02] hover:border-white/25 hover:bg-white/[0.04] cursor-pointer"
                  }`}
                  disabled={submitting || !selectable}
                  selected={isSelected}
                  aria-pressed={isSelected}
                  onClick={() => {
                    if (selectable) {
                      setSelectedColonyId(colony.id);
                    }
                  }}
                >
                  <div className="w-full min-w-0">
                    <div className="flex items-center justify-between gap-3 mb-2">
                      <h3 className="text-white text-xs font-bold font-orbitron m-0">
                        <GameIcon iconType={colony.style.icon} size="small" /> {colony.name}
                      </h3>
                      {!pendingSelection.addTile && reward.length > 0 && (
                        <div className="flex items-center gap-1">
                          <span className="text-[9px] font-orbitron text-white/40 uppercase compact:text-[11px]">
                            Reward
                          </span>
                          <ColonyOutputDisplay outputs={reward} />
                        </div>
                      )}
                    </div>

                    <div className={COLONY_TRACK_SCROLL_CLASS}>
                      <div className={COLONY_TRACK_INNER_CLASS}>
                        <ColonySteps
                          steps={colony.steps}
                          markerPosition={colony.markerPosition}
                          playerColonies={colony.playerColonies}
                          maxSlots={colony.colonies.length}
                          getPlayerColor={getPlayerColor}
                          getPlayerName={getPlayerName}
                        />
                      </div>
                    </div>

                    <div className="flex items-center gap-1.5 mt-1.5 text-[9px] text-white/40 compact:text-[11px]">
                      <span className="font-orbitron uppercase tracking-wider">Colony Bonus</span>
                      <ColonyOutputDisplay outputs={colony.colonyBonus} />
                    </div>
                  </div>
                </GameButton>
              );
            })
          )}
        </div>

        {error && (
          <p role="alert" className="px-4 py-2 text-red-300 text-sm">
            {error}
          </p>
        )}
        <div className="px-4 py-3 border-t border-white/10 flex items-center justify-end gap-3">
          {empty && (
            <GameButton size="sm" disabled={submitting} onClick={() => void submit(onContinue)}>
              Continue
            </GameButton>
          )}
          <GameButton
            size="sm"
            className="compact:min-h-11 compact:px-5"
            onClick={() => {
              if (selectedColonyId && selectableIds.has(selectedColonyId)) {
                void submit(() => onConfirm(selectedColonyId));
              }
            }}
            disabled={
              empty || submitting || !selectedColonyId || !selectableIds.has(selectedColonyId)
            }
          >
            Confirm
          </GameButton>
        </div>
      </div>
    </div>
  );
};

export default ColonySelectionOverlay;
