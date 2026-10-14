import React, { useEffect, useRef, useState } from "react";
import { globalWebSocketManager } from "@/services/globalWebSocketManager";
import { useResolutionFailure, type ResolutionFailure } from "@/hooks/useResolutionFailure";
import type { PendingResourceRemovalSelectionDto } from "@/types/generated/api-types";
import {
  ResourceType,
  ResourcesDto,
  ProductionDto,
  ResourceRemovalTargetDto,
} from "@/types/generated/api-types.ts";
import GameIcon from "../display/GameIcon.tsx";
import GameButton from "../buttons/GameButton.tsx";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "./GameFlowPopover.tsx";

interface TargetPlayer {
  id: string;
  name: string;
  resources: ResourcesDto;
  production: ProductionDto;
}

interface TargetPlayerSelectionPopoverProps {
  removalTargets: ResourceRemovalTargetDto[];
  resourceType: ResourceType;
  amount: number;
  isSteal?: boolean;
  players: TargetPlayer[];
  currentPlayerId?: string;
  onPlayerSelect?: (playerId: string) => void;
  onCancel?: () => void;
  pendingRemoval?: {
    selection: PendingResourceRemovalSelectionDto;
    failure?: ResolutionFailure | null;
    onConfirm: (id: string, target: string, amount: number) => Promise<unknown>;
  };
  isVisible: boolean;
  mandatory?: boolean;
}

function getPlayerResourceAmount(player: TargetPlayer, resourceType: ResourceType): number {
  switch (resourceType) {
    case "credit":
      return player.resources.credits;
    case "steel":
      return player.resources.steel;
    case "titanium":
      return player.resources.titanium;
    case "plant":
      return player.resources.plants;
    case "energy":
      return player.resources.energy;
    case "heat":
      return player.resources.heat;
    case "credit-production":
      return player.production.credits;
    case "steel-production":
      return player.production.steel;
    case "titanium-production":
      return player.production.titanium;
    case "plant-production":
      return player.production.plants;
    case "energy-production":
      return player.production.energy;
    case "heat-production":
      return player.production.heat;
    default:
      return 0;
  }
}

export function PendingResourceRemovalPopover({
  selection,
  players,
  currentPlayerId,
}: {
  selection: PendingResourceRemovalSelectionDto;
  players: TargetPlayer[];
  currentPlayerId: string;
}) {
  const failure = useResolutionFailure();
  return (
    <TargetPlayerSelectionPopover
      isVisible
      mandatory
      resourceType={selection.resourceType}
      amount={selection.amount}
      players={players}
      currentPlayerId={currentPlayerId}
      removalTargets={[]}
      pendingRemoval={{
        selection,
        failure,
        onConfirm: (id, target, amount) =>
          globalWebSocketManager.confirmResourceRemoval(id, target, amount),
      }}
    />
  );
}

const TargetPlayerSelectionPopover: React.FC<TargetPlayerSelectionPopoverProps> = ({
  resourceType,
  removalTargets,
  amount,
  isSteal,
  players,
  currentPlayerId,
  onPlayerSelect,
  onCancel,
  isVisible,
  mandatory = false,
  pendingRemoval,
}) => {
  const [target, setTarget] = useState("");
  const [selectedAmount, setSelectedAmount] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pending = useRef(false);
  const selection = pendingRemoval?.selection;
  const failure = pendingRemoval?.failure;
  const isProduction = resourceType.endsWith("-production");
  const eligiblePlayers = players.filter((player) =>
    selection
      ? selection.eligiblePlayerIds.includes(player.id) && selection.maxAmounts[player.id] > 0
      : isProduction
        ? getPlayerResourceAmount(player, resourceType) > 0
        : removalTargets.some(
            (t) =>
              t.playerId === player.id &&
              !t.cardId &&
              t.resourceType === resourceType &&
              t.amount > 0,
          ),
  );
  const hasNoTargets = eligiblePlayers.length === 0;
  const maximum = selection?.maxAmounts[target] ?? 0;
  const validAmount =
    !!target &&
    eligiblePlayers.some((player) => player.id === target) &&
    Number.isInteger(selectedAmount) &&
    selectedAmount >= 1 &&
    selectedAmount <= maximum;

  useEffect(() => {
    if (
      failure &&
      (failure.disconnected || (pending.current && failure.selectionId === selection?.id))
    ) {
      pending.current = false;
      setSubmitting(false);
      setError(failure.message);
    }
  }, [failure, selection?.id]);

  const submit = async (skip: boolean) => {
    if (!pendingRemoval) {
      onPlayerSelect?.(skip ? "" : target);
      return;
    }
    if (pending.current || (!skip && !validAmount)) return;
    pending.current = true;
    setSubmitting(true);
    setError(null);
    try {
      await pendingRemoval.onConfirm(
        pendingRemoval.selection.id,
        skip ? "" : target,
        skip ? 0 : selectedAmount,
      );
    } catch (reason) {
      pending.current = false;
      setSubmitting(false);
      setError(reason instanceof Error ? reason.message : "Could not remove resources");
    }
  };

  return (
    <GameFlowPopover
      isVisible={isVisible}
      onClose={onCancel}
      type={onCancel && (!mandatory || hasNoTargets) ? "interactive" : "interactive-mandatory"}
      className="min-w-[280px] max-w-[min(400px,95vw)]"
    >
      <GameFlowTitle>
        <h3 className="m-0 font-orbitron text-white text-base font-bold">
          {hasNoTargets ? "No Valid Targets" : "Select Target Player"}
        </h3>
        {selection && <p className="text-white/60 text-xs mt-1">{selection.source}</p>}
        {!hasNoTargets && (
          <div className="text-white/60 text-xs compact:text-[13px] mt-1 flex items-center justify-center gap-1.5">
            <span>{isSteal ? "Steal" : "Remove"} up to</span>
            <GameIcon iconType={resourceType} amount={amount} size="small" />
          </div>
        )}
      </GameFlowTitle>
      <GameFlowBody>
        {hasNoTargets ? (
          <div className="flex flex-col items-center gap-4 py-8 px-4 text-center">
            <GameIcon iconType={resourceType} size="large" />
            <p className="text-white/70 text-sm max-w-[280px]">
              No eligible players have resources available. You can continue without targeting
              anyone.
            </p>
          </div>
        ) : (
          eligiblePlayers.map((player) => (
            <GameButton
              key={player.id}
              emphasis="secondary"
              selected={!!selection && target === player.id}
              aria-pressed={selection ? target === player.id : undefined}
              className="w-full mb-2 px-3.5 py-3 compact:min-h-11 text-left"
              disabled={submitting}
              onClick={() => {
                if (!selection) {
                  onPlayerSelect?.(player.id);
                  return;
                }
                setTarget(player.id);
                setSelectedAmount(selection.maxAmounts[player.id]);
                setError(null);
              }}
            >
              <span className="w-full flex items-center justify-between gap-3">
                <span className="text-white font-sans font-semibold text-sm break-words">
                  {player.id === currentPlayerId ? "You" : player.name}
                </span>
                <GameIcon
                  iconType={resourceType}
                  amount={getPlayerResourceAmount(player, resourceType)}
                  size="small"
                />
              </span>
            </GameButton>
          ))
        )}
        {selection && target && !hasNoTargets && (
          <label className="flex items-center justify-between gap-3 text-white text-sm compact:min-h-11">
            Amount
            <input
              aria-label="Amount to remove"
              className="w-24 min-h-11 border border-white/30 bg-black/40 p-2 text-base font-orbitron"
              type="number"
              min={1}
              max={maximum}
              step={1}
              value={selectedAmount}
              disabled={submitting}
              onChange={(event) => setSelectedAmount(Number(event.target.value))}
            />
          </label>
        )}
        {error && (
          <p role="alert" className="text-red-300 text-sm mt-2">
            {error}
          </p>
        )}
      </GameFlowBody>
      {(onCancel || selection || hasNoTargets || !mandatory) && (
        <GameFlowFooter className="gap-3">
          {onCancel && (
            <GameButton emphasis="secondary" size="sm" disabled={submitting} onClick={onCancel}>
              Cancel
            </GameButton>
          )}
          {(hasNoTargets || !isProduction) && (
            <GameButton
              emphasis={selection && !hasNoTargets ? "secondary" : "primary"}
              tone={selection ? "info" : "warn"}
              size="sm"
              disabled={submitting}
              onClick={() => void submit(true)}
            >
              {hasNoTargets ? "Continue" : "Skip"}
            </GameButton>
          )}
          {selection && !hasNoTargets && (
            <GameButton
              size="sm"
              disabled={submitting || !validAmount}
              onClick={() => void submit(false)}
            >
              Remove
            </GameButton>
          )}
        </GameFlowFooter>
      )}
    </GameFlowPopover>
  );
};

export default TargetPlayerSelectionPopover;
