import { useMemo, useState } from "react";
import TradeTrackChoices, { selectedTradeOption } from "../../display/TradeTrackChoices.tsx";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import {
  ColonyDto,
  ColonyOutputDto,
  GameDto,
  ResourceTypeCredit,
  ResourceTypeEnergy,
  ResourceTypeTitanium,
} from "@/types/generated/api-types.ts";
import GameIcon from "../../display/GameIcon.tsx";
import { webSocketService } from "@/services/webSocketService.ts";
import { getStorageWarning } from "@/utils/colonyUtils.ts";
import ColonyOutputDisplay from "../../display/ColonyOutputDisplay.tsx";
import StorageWarningDialog from "../../display/StorageWarningDialog.tsx";
import { GamePopoverItem, getThemeStyles } from "../../GamePopover";
import ColonySteps, { getTradeExpression, mapOutputTypeToIcon } from "../ColonySteps.tsx";
import type { ContentDensity } from "./density.ts";
import { canActOnProjects, listGamePlayers } from "./gamePlayers.ts";

export type ColonyMode = "trade" | "build";
export type TradePaymentType = "credits" | "energy" | "titanium";

const SCREEN_TRACK_CHOICES_CLASS =
  "[&>div>span]:!text-[11px] [&_.game-button]:!min-h-11 [&_.game-button]:min-w-11";

const PAYMENT_TYPES: TradePaymentType[] = ["credits", "energy", "titanium"];
const PAYMENT_RESOURCE: Record<TradePaymentType, string> = {
  credits: "credit",
  energy: "energy",
  titanium: "titanium",
};
const PAYMENT_ICON: Record<TradePaymentType, string> = {
  credits: ResourceTypeCredit,
  energy: ResourceTypeEnergy,
  titanium: ResourceTypeTitanium,
};
const DEFAULT_TRADE_COST: Record<TradePaymentType, number> = {
  credits: 9,
  energy: 3,
  titanium: 3,
};

interface StorageWarningState {
  message: string;
  action: () => void;
}

export interface ColonyControls {
  canAct: boolean;
  mode: ColonyMode;
  setMode: (mode: ColonyMode) => void;
  tradePayment: TradePaymentType;
  setTradePayment: (payment: TradePaymentType) => void;
  tradeCosts: Record<TradePaymentType, { icon: string; amount: number }>;
  affordable: Record<TradePaymentType, boolean>;
  storageWarning: StorageWarningState | null;
  dismissStorageWarning: () => void;
  trade: (colonyId: string, trackSteps: number) => void;
  build: (colonyId: string) => void;
}

export function useColonyControls(
  gameState: GameDto | undefined,
  onFlowStart?: () => void,
): ColonyControls {
  return useColonyControlState(
    gameState,
    {
      trade: (id, payment, steps) => {
        void webSocketService.tradeWithColony(id, payment, steps);
      },
      build: (id) => {
        void webSocketService.buildColony(id);
      },
    },
    onFlowStart,
  );
}

export function useColonyControlState(
  gameState: GameDto | undefined,
  actions: {
    trade: (id: string, payment: TradePaymentType, steps: number) => void;
    build: (id: string) => void;
  },
  onFlowStart?: () => void,
  initialMode: ColonyMode = "trade",
): ColonyControls {
  const [mode, setMode] = useState<ColonyMode>(initialMode);
  const [storageWarning, setStorageWarning] = useState<StorageWarningState | null>(null);

  const tradeActionCosts = gameState?.currentPlayer?.actionCosts?.find(
    (a) => a.actionType === "colony-trade",
  );
  const costFor = (payment: TradePaymentType) =>
    tradeActionCosts?.costs.find((c) => c.resource === PAYMENT_RESOURCE[payment]);

  const tradeCosts = {} as Record<TradePaymentType, { icon: string; amount: number }>;
  const affordable = {} as Record<TradePaymentType, boolean>;
  for (const payment of PAYMENT_TYPES) {
    const cost = costFor(payment);
    const amount = cost?.effectiveCost ?? DEFAULT_TRADE_COST[payment];
    tradeCosts[payment] = { icon: PAYMENT_ICON[payment], amount };
    affordable[payment] = (cost?.paymentCapacity ?? 0) >= amount;
  }

  const defaultPayment = (): TradePaymentType => {
    return PAYMENT_TYPES.find((payment) => affordable[payment]) ?? "credits";
  };

  const [tradePayment, setTradePayment] = useState<TradePaymentType>(defaultPayment);

  const canAct = canActOnProjects(gameState);
  const colonies = gameState?.colonies ?? [];

  const runWithStorageCheck = (outputs: ColonyOutputDto[], send: () => void) => {
    const warning = getStorageWarning(
      outputs,
      gameState?.currentPlayer?.playedCards ?? [],
      gameState?.currentPlayer?.corporation,
    );
    const start = () => {
      onFlowStart?.();
      send();
    };
    if (warning) {
      setStorageWarning({
        message: warning,
        action: () => {
          setStorageWarning(null);
          start();
        },
      });
      return;
    }
    start();
  };

  const trade = (colonyId: string, trackSteps: number) => {
    if (!canAct || !affordable[tradePayment]) {
      return;
    }
    const send = () => actions.trade(colonyId, tradePayment, trackSteps);
    const colony = colonies.find((c) => c.id === colonyId);
    const tradeOutputs =
      colony?.tradeOptions.find((option) => option.trackSteps === trackSteps)?.outputs ?? [];
    runWithStorageCheck(tradeOutputs, send);
  };

  const build = (colonyId: string) => {
    if (!canAct) {
      return;
    }
    const send = () => actions.build(colonyId);
    const colony = colonies.find((c) => c.id === colonyId);
    const reward = colony?.colonies[colony.playerColonies.length]?.reward ?? [];
    runWithStorageCheck(reward, send);
  };

  return {
    canAct,
    mode,
    setMode,
    tradePayment,
    setTradePayment,
    tradeCosts,
    affordable,
    storageWarning,
    dismissStorageWarning: () => setStorageWarning(null),
    trade,
    build,
  };
}

export function ColonyStorageWarning({ controls }: { controls: ColonyControls }) {
  if (!controls.storageWarning) {
    return null;
  }
  return (
    <StorageWarningDialog
      message={controls.storageWarning.message}
      onCancel={controls.dismissStorageWarning}
      onContinue={controls.storageWarning.action}
    />
  );
}

interface ColonyControlProps {
  controls: ColonyControls;
  density: ContentDensity;
}

export function ColonyModeToggle({ controls, density }: ColonyControlProps) {
  const isScreen = density === "screen";
  const buttonClass = (mode: ColonyMode) => {
    const sizeClass = isScreen ? "!min-h-11 px-4 text-[13px]" : "px-3 py-1 text-[11px]";
    const stateClass =
      controls.mode === mode
        ? "bg-white/20 text-white"
        : "bg-transparent text-white/40 hover:text-white/60";
    return `${sizeClass} font-orbitron font-bold transition-colors cursor-pointer ${stateClass}`;
  };
  return (
    <div className="flex rounded overflow-hidden border border-white/20">
      <GameButton
        emphasis="quiet"
        className={buttonClass("trade")}
        aria-pressed={isScreen ? controls.mode === "trade" : undefined}
        onClick={() => controls.setMode("trade")}
      >
        Trade
      </GameButton>
      <GameButton
        emphasis="quiet"
        className={buttonClass("build")}
        aria-pressed={isScreen ? controls.mode === "build" : undefined}
        onClick={() => controls.setMode("build")}
      >
        Build
      </GameButton>
    </div>
  );
}

function paymentOptionClass(isSelected: boolean, canAfford: boolean): string {
  if (isSelected) {
    return "bg-white/20 text-white";
  }
  if (canAfford) {
    return "bg-transparent text-white/40 hover:text-white/60";
  }
  return "bg-transparent text-white/20 opacity-40";
}

export function TradePaymentSelector({ controls, density }: ColonyControlProps) {
  const isScreen = density === "screen";
  return (
    <div className="flex rounded overflow-hidden border border-white/20">
      {PAYMENT_TYPES.map((type) => {
        const isSelected = controls.tradePayment === type;
        return (
          <GameButton
            emphasis="quiet"
            key={type}
            aria-label={isScreen ? `Pay with ${type}` : undefined}
            aria-pressed={isScreen ? isSelected : undefined}
            className={`flex items-center transition-colors cursor-pointer ${isScreen ? "!min-h-11 min-w-11 px-2.5 justify-center" : "px-1.5 py-0.5"} ${paymentOptionClass(isSelected, controls.affordable[type])}`}
            onClick={() => controls.setTradePayment(type)}
          >
            <GameIcon
              iconType={PAYMENT_ICON[type]}
              amount={type === "credits" ? "X" : undefined}
              size="small"
            />
          </GameButton>
        );
      })}
    </div>
  );
}

interface ColonyListProps {
  gameState?: GameDto;
  controls: ColonyControls;
  density: ContentDensity;
}

export default function ColonyList({ gameState, controls, density }: ColonyListProps) {
  const isScreen = density === "screen";
  const colonies = gameState?.colonies ?? [];
  const allPlayers = useMemo(() => listGamePlayers(gameState), [gameState]);

  const getPlayerColor = (playerId: string): string => {
    return allPlayers.find((p) => p.id === playerId)?.color ?? "#666";
  };

  const getPlayerName = (playerId: string): string => {
    return allPlayers.find((p) => p.id === playerId)?.name ?? "Unknown";
  };

  const labelText = isScreen ? "text-[11px]" : "text-[10px]";

  const fleets = gameState?.tradeFleets && (
    <div
      className={`py-2 border-b border-white/10 flex flex-wrap items-center gap-2 ${isScreen ? "" : "px-3"}`}
    >
      <span className={`${labelText} font-orbitron text-white/50 uppercase tracking-wider`}>
        Ships:
      </span>
      {allPlayers.map((player) => {
        const fleet = gameState.tradeFleets?.[player.id];
        const hasFleet = (fleet?.available ?? 0) > 0;
        return (
          <div key={player.id} className="flex items-center gap-1">
            <div
              className={`w-3 h-3 rounded-sm ${!hasFleet ? "opacity-30" : ""}`}
              style={{ backgroundColor: player.color }}
            />
            <GameIcon iconType="trade" size="small" />
            <span
              className={`${isScreen ? "text-[12px]" : "text-[10px]"} font-orbitron ${hasFleet ? "text-white/60" : "text-white/25"}`}
            >
              {fleet?.available ?? 0}/{fleet?.total ?? 0}
            </span>
          </div>
        );
      })}
    </div>
  );

  const list = (
    <div className={`popover-list popover-list-headed space-y-2 ${isScreen ? "pt-2" : "p-2"}`}>
      {colonies.map((colony) => (
        <ColonyCard
          key={`${colony.id}:${colony.markerPosition}:${colony.tradedThisGen}`}
          colony={colony}
          controls={controls}
          density={density}
          getPlayerColor={getPlayerColor}
          getPlayerName={getPlayerName}
        />
      ))}
    </div>
  );

  if (!isScreen) {
    return (
      <>
        {fleets}
        {list}
      </>
    );
  }

  return (
    <div className="w-full flex flex-col" style={getThemeStyles("colonies")}>
      <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
        {controls.mode === "trade" ? (
          <TradePaymentSelector controls={controls} density={density} />
        ) : (
          <span />
        )}
        <ColonyModeToggle controls={controls} density={density} />
      </div>
      {fleets}
      {list}
      <ColonyStorageWarning controls={controls} />
    </div>
  );
}

interface ColonyCardProps {
  colony: ColonyDto;
  controls: ColonyControls;
  density: ContentDensity;
  getPlayerColor: (playerId: string) => string;
  getPlayerName: (playerId: string) => string;
}

function ColonyCard({ colony, controls, density, getPlayerColor, getPlayerName }: ColonyCardProps) {
  const isScreen = density === "screen";
  const { canAct, mode, tradePayment, tradeCosts } = controls;
  const canTrade = canAct && colony.tradeAvailable && controls.affordable[tradePayment];
  const canBuild = canAct && colony.buildAvailable;

  const isDisabled = mode === "trade" ? !canTrade : !canBuild;
  const dimmed = canAct && isDisabled;

  const [selectedSteps, setSelectedSteps] = useState<number>();
  const tradeOption = selectedTradeOption(colony.tradeOptions, selectedSteps);
  const tradeGainOutputs = tradeOption?.outputs ?? [];
  const buildReward = colony.colonies[0]?.reward ?? [];
  const tradeExpression = getTradeExpression(colony.steps);

  const labelText = isScreen ? "text-[11px]" : "text-[10px]";
  const actionButtonClass = isScreen ? "min-h-11 min-w-[88px] px-4" : "";

  const trackChoices = (
    <TradeTrackChoices
      options={colony.tradeOptions}
      selected={tradeOption?.trackSteps ?? 0}
      onSelect={setSelectedSteps}
      disabled={!canTrade}
    />
  );

  const steps = (
    <ColonySteps
      steps={colony.steps}
      markerPosition={colony.markerPosition}
      previewPosition={tradeOption?.markerPosition}
      playerColonies={colony.playerColonies}
      maxSlots={colony.colonies.length}
      getPlayerColor={getPlayerColor}
      getPlayerName={getPlayerName}
    />
  );

  return (
    <GamePopoverItem
      state={dimmed ? "disabled" : "available"}
      borderColor="#142c58"
      className="popover-list-item"
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <h3 className="text-white text-sm font-bold font-orbitron m-0">
            <GameIcon iconType={colony.style.icon} size="small" /> {colony.name}
          </h3>
          {!colony.active && (
            <span className="border border-white/20 bg-white/10 px-2 py-0.5 font-orbitron text-[11px] text-white/60">
              Inactive
            </span>
          )}
          {colony.tradedThisGen && colony.traderId && (
            <span
              className={`px-2 py-0.5 rounded-full ${labelText} font-orbitron text-white`}
              style={{ backgroundColor: getPlayerColor(colony.traderId) }}
            >
              {getPlayerName(colony.traderId)}
            </span>
          )}
        </div>

        {canAct && (
          <div>
            {mode === "trade" ? (
              <GameButton
                emphasis="primary"
                shape="cut"
                size="sm"
                className={actionButtonClass}
                onClick={(e) => {
                  e.stopPropagation();
                  if (canTrade && tradeOption) {
                    controls.trade(colony.id, tradeOption.trackSteps);
                  }
                }}
                disabled={!canTrade}
              >
                Trade
              </GameButton>
            ) : (
              <GameButton
                emphasis="primary"
                shape="cut"
                size="sm"
                className={actionButtonClass}
                onClick={(e) => {
                  e.stopPropagation();
                  if (canBuild) {
                    controls.build(colony.id);
                  }
                }}
                disabled={!canBuild}
              >
                Build
              </GameButton>
            )}
          </div>
        )}
      </div>

      {mode === "trade" && isScreen && (
        <div className={SCREEN_TRACK_CHOICES_CLASS}>{trackChoices}</div>
      )}
      {mode === "trade" && !isScreen && trackChoices}
      <div className="relative h-7" hidden={!colony.active}>
        {PAYMENT_TYPES.map((pt) => {
          const config = tradeCosts[pt];
          return (
            <div
              key={pt}
              className="absolute inset-0 flex items-center gap-1.5"
              style={{
                opacity: mode === "trade" && !colony.tradedThisGen && tradePayment === pt ? 1 : 0,
                transition: mode === "trade" ? "opacity 300ms" : "none",
              }}
            >
              {pt !== "credits" && (
                <span
                  className={`${isScreen ? "text-[13px]" : "text-xs"} text-white/70 font-orbitron font-bold`}
                >
                  {config.amount}
                </span>
              )}
              <GameIcon
                iconType={config.icon}
                amount={pt === "credits" ? config.amount : undefined}
                size="small"
              />
              <span className="text-white/30 text-sm">→</span>
              <CostDisplay outputs={tradeGainOutputs} isScreen={isScreen} />
            </div>
          );
        })}
        <div
          className="absolute inset-0 flex items-center"
          style={{
            opacity: mode === "trade" && colony.tradedThisGen ? 1 : 0,
            transition: mode === "trade" ? "opacity 300ms" : "none",
          }}
        >
          <span
            className={`${labelText} font-orbitron font-bold text-white/30 uppercase tracking-wider`}
          >
            Already Traded
          </span>
        </div>
        <div
          className="absolute inset-0 flex items-center gap-1.5"
          style={{
            opacity: mode === "build" ? 1 : 0,
            transition: mode === "build" ? "opacity 300ms" : "none",
          }}
        >
          <GameIcon iconType={ResourceTypeCredit} amount={17} size="small" />
          <span className="text-white/30 text-sm">→</span>
          <GameIcon iconType="colony" size="small" />
          <CostDisplay outputs={buildReward} isScreen={isScreen} />
        </div>
      </div>

      <div style={{ height: "1px", background: "rgba(255,255,255,0.15)", margin: "16px 0" }} />

      {isScreen ? (
        <div className="mb-3 overflow-x-auto overscroll-x-contain [scrollbar-width:thin]">
          <div className="min-w-[480px]">{steps}</div>
        </div>
      ) : (
        <div className="mb-3">{steps}</div>
      )}

      <div
        className={`flex items-center justify-between text-white/40 ${labelText} ${isScreen ? "flex-wrap gap-2" : ""}`}
      >
        <div className="flex items-center gap-2">
          <span className="font-orbitron uppercase tracking-wider">Colony Bonus</span>
          <ColonyOutputDisplay outputs={colony.colonyBonus} />
        </div>
        {tradeExpression && (
          <div className="flex items-center gap-2">
            <span className="font-orbitron uppercase tracking-wider">Trade Gain</span>
            <span className="inline-flex items-center gap-0.5">
              {tradeExpression.type === "x-icon" && !tradeExpression.isCreditType && (
                <span className={`${labelText} text-white/70 font-orbitron font-bold`}>X</span>
              )}
              <GameIcon
                iconType={tradeExpression.icon}
                amount={
                  tradeExpression.type === "x-icon" && tradeExpression.isCreditType
                    ? "X"
                    : undefined
                }
                size="small"
              />
            </span>
          </div>
        )}
      </div>
    </GamePopoverItem>
  );
}

function CostDisplay({ outputs, isScreen }: { outputs: ColonyOutputDto[]; isScreen: boolean }) {
  return (
    <span className="inline-flex items-center gap-0.5">
      {outputs.map((output, i) => {
        const icon = mapOutputTypeToIcon(output.type);
        const useAmountProp = output.type === "credit" || output.type === "credit-production";
        return (
          <span key={i} className="inline-flex items-center gap-0.5">
            {!useAmountProp && output.amount > 1 && (
              <span
                className={`${isScreen ? "text-[13px]" : "text-xs"} text-white/70 font-orbitron font-bold`}
              >
                {output.amount}
              </span>
            )}
            <GameIcon
              iconType={icon}
              amount={useAmountProp ? output.amount : undefined}
              size="small"
            />
          </span>
        );
      })}
    </span>
  );
}
