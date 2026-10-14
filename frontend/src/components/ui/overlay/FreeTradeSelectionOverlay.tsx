import { usePendingSubmission } from "@/hooks/usePendingSubmission";
import type { ResolutionFailure } from "@/hooks/useResolutionFailure";
import GamePopoverEmpty from "../GamePopover/GamePopoverEmpty";
import GameIcon from "../display/GameIcon";
import React, { useState, useMemo } from "react";
import {
  PendingFreeTradeSelectionDto,
  ColonyDto,
  TradeFleetDto,
  CardDto,
} from "@/types/generated/api-types.ts";
import TradeTrackChoices, { selectedTradeOption } from "../display/TradeTrackChoices.tsx";
import ColonySteps from "../popover/ColonySteps.tsx";
import GameButton from "../buttons/GameButton.tsx";
import ColonyOutputDisplay from "../display/ColonyOutputDisplay.tsx";
import StorageWarningDialog from "../display/StorageWarningDialog.tsx";
import { PlayerInfo, getStorageWarning } from "@/utils/colonyUtils.ts";
import { Z_INDEX, getZIndex } from "@/constants/zIndex.ts";
import {
  COLONY_TRACK_INNER_CLASS,
  COLONY_TRACK_SCROLL_CLASS,
  OVERLAY_ROOT_SAFE_AREA_CLASS,
} from "./overlayStyles.ts";

interface FreeTradeSelectionOverlayProps {
  isOpen: boolean;
  pendingSelection: PendingFreeTradeSelectionDto;
  colonies: ColonyDto[];
  viewingPlayerId: string;
  tradeFleets: Record<string, TradeFleetDto>;
  allPlayers: PlayerInfo[];
  playedCards: CardDto[];
  corporation?: CardDto | null;
  onContinue: () => void | Promise<unknown>;
  failure?: ResolutionFailure | null;
  onConfirm: (colonyId: string, trackSteps: number) => void | Promise<unknown>;
}

const FreeTradeSelectionOverlay: React.FC<FreeTradeSelectionOverlayProps> = ({
  isOpen,
  pendingSelection,
  colonies,
  viewingPlayerId,
  tradeFleets,
  allPlayers,
  playedCards,
  corporation,
  onConfirm,
  onContinue,
  failure,
}) => {
  const tradeFleetAvailable = (tradeFleets[viewingPlayerId]?.available ?? 0) > 0;
  const [selectedSteps, setSelectedSteps] = useState<Record<string, number>>({});
  const { submitting, error, submit } = usePendingSubmission(
    failure,
    "action.confirm-free-trade",
    JSON.stringify(pendingSelection),
  );
  const empty = pendingSelection.canContinueWithoutSelection;
  const [selectedColonyId, setSelectedColonyId] = useState<string | null>(null);
  const [storageWarning, setStorageWarning] = useState<{
    message: string;
    colonyId: string;
    trackSteps: number;
  } | null>(null);

  const tradeableIds = useMemo(
    () => new Set(pendingSelection.availableColonyIds),
    [pendingSelection],
  );

  const isColonyTradeable = (colony: ColonyDto): boolean => {
    return tradeableIds.has(colony.id) && !colony.tradedThisGen && tradeFleetAvailable;
  };

  const getPlayerColor = (playerId: string): string => {
    return allPlayers.find((p) => p.id === playerId)?.color ?? "#666";
  };

  const getPlayerName = (playerId: string): string => {
    return allPlayers.find((p) => p.id === playerId)?.name ?? "Unknown";
  };

  const selectedColony = colonies.find((colony) => colony.id === selectedColonyId);
  const selectedOption = selectedColony
    ? selectedTradeOption(selectedColony.tradeOptions, selectedSteps[selectedColony.id])
    : undefined;

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
        className="relative w-[min(480px,calc(100%-32px))] max-h-[80dvh] compact:h-full compact:max-h-full flex flex-col game-panel game-panel-clipped game-window overflow-hidden"
        style={{ zIndex: getZIndex("LOCAL", 1) }}
      >
        <div className="px-4 py-3 border-b border-white/10 flex items-center justify-between">
          <h2 className="font-orbitron text-base font-bold text-white tracking-wider m-0">
            Free Trade
          </h2>
          <span className="text-xs text-white/40">{pendingSelection.source}</span>
        </div>

        {!tradeFleetAvailable && (
          <div className="px-4 py-2 bg-red-900/20 border-b border-red-500/30">
            <span className="text-xs text-red-400 font-orbitron">No trade fleet available</span>
          </div>
        )}

        <div className="px-4 py-2 border-b border-white/10 flex flex-wrap items-center gap-2">
          <span className="text-[10px] font-orbitron text-white/50 uppercase tracking-wider compact:text-[11px]">
            Ships:
          </span>
          {allPlayers.map((player) => (
            <div key={player.id} className="flex items-center gap-1">
              <div className="w-3 h-3 rounded-sm" style={{ backgroundColor: player.color }} />
              <span className="text-[10px] font-orbitron text-white/60 compact:text-[11px]">
                {player.name} {tradeFleets[player.id]?.available ?? 0}/
                {tradeFleets[player.id]?.total ?? 0}
              </span>
            </div>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto p-3 space-y-2">
          {empty ? (
            <GamePopoverEmpty
              icon={<GameIcon iconType="colony" size="large" />}
              title="No trade available"
              description="There are no legal choices for this effect. Continue without resolving it."
            />
          ) : (
            colonies.map((colony) => {
              const tradeable = isColonyTradeable(colony);
              const isSelected = selectedColonyId === colony.id;
              const tradeOption = selectedTradeOption(
                colony.tradeOptions,
                selectedSteps[colony.id],
              );
              const tradeGainOutputs = tradeOption?.outputs ?? [];

              return (
                <div key={colony.id}>
                  <GameButton
                    emphasis="quiet"
                    type="button"
                    className={`w-full text-left px-3 py-2.5 rounded-none border transition-all ${
                      !tradeable
                        ? "border-white/5 bg-white/[0.01] opacity-40 cursor-default"
                        : isSelected
                          ? "border-space-blue-500 bg-space-blue-900/60 cursor-pointer"
                          : "border-white/10 bg-white/[0.02] hover:border-white/25 hover:bg-white/[0.04] cursor-pointer"
                    }`}
                    disabled={submitting || !tradeable}
                    selected={isSelected}
                    aria-pressed={isSelected}
                    onClick={() => {
                      if (tradeable) {
                        setSelectedColonyId(colony.id);
                      }
                    }}
                  >
                    <div className="w-full min-w-0">
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <h3 className="text-white text-xs font-bold font-orbitron m-0">
                            {colony.name}
                          </h3>
                          {colony.tradedThisGen && (
                            <span className="text-[9px] font-orbitron text-white/30 uppercase compact:text-[11px]">
                              Traded
                            </span>
                          )}
                        </div>
                        {tradeable && (
                          <div className="flex items-center gap-1">
                            <span className="text-white/30 text-sm">→</span>
                            <ColonyOutputDisplay outputs={tradeGainOutputs} />
                          </div>
                        )}
                      </div>

                      <div className={COLONY_TRACK_SCROLL_CLASS}>
                        <div className={COLONY_TRACK_INNER_CLASS}>
                          <ColonySteps
                            steps={colony.steps}
                            markerPosition={colony.markerPosition}
                            previewPosition={tradeOption?.markerPosition}
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
                  <TradeTrackChoices
                    options={colony.tradeOptions}
                    selected={tradeOption?.trackSteps ?? 0}
                    disabled={submitting || !tradeable}
                    onSelect={(steps) => {
                      setSelectedSteps((previous) => ({ ...previous, [colony.id]: steps }));
                      setSelectedColonyId(colony.id);
                    }}
                  />
                </div>
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
              if (!selectedColony || !selectedOption || !isColonyTradeable(selectedColony)) {
                return;
              }
              const warning = getStorageWarning(selectedOption.outputs, playedCards, corporation);
              if (warning) {
                setStorageWarning({
                  message: warning,
                  colonyId: selectedColony.id,
                  trackSteps: selectedOption.trackSteps,
                });
                return;
              }
              void submit(() => onConfirm(selectedColony.id, selectedOption.trackSteps));
            }}
            disabled={
              empty ||
              submitting ||
              !selectedColony ||
              !selectedOption ||
              !isColonyTradeable(selectedColony)
            }
          >
            Confirm
          </GameButton>
        </div>
      </div>

      {storageWarning && (
        <StorageWarningDialog
          message={storageWarning.message}
          onCancel={() => setStorageWarning(null)}
          onContinue={() => {
            void submit(() => onConfirm(storageWarning.colonyId, storageWarning.trackSteps));
            setStorageWarning(null);
          }}
        />
      )}
    </div>
  );
};

export default FreeTradeSelectionOverlay;
