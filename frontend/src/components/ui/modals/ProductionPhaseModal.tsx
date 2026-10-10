import React, { useState, useEffect, useMemo, useCallback, useRef } from "react";
import {
  GameDto,
  ProductionPhaseDto,
  OtherPlayerDto,
  PlayerDto,
  ResourceType,
  ResourcesDto,
  ProductionDto,
  ResourceTypeCredit,
  ResourceTypeSteel,
  ResourceTypeTitanium,
  ResourceTypePlant,
  ResourceTypeEnergy,
  ResourceTypeHeat,
} from "@/types/generated/api-types.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import ProductionCardSelectionOverlay from "@/components/ui/overlay/ProductionCardSelectionOverlay.tsx";
import GameIcon from "@/components/ui/display/GameIcon.tsx";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import { Z_INDEX, getZIndex } from "@/constants/zIndex.ts";
import { audioService } from "@/services/audioService.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import {
  OVERLAY_BACKDROP_BLUR_CLASS,
  OVERLAY_BACKDROP_TINT_CLASS,
  OVERLAY_HEADER_CLASS,
  OVERLAY_TITLE_CLASS,
} from "@/components/ui/overlay/overlayStyles.ts";

interface ProductionPhaseModalProps {
  isOpen: boolean;
  gameState: GameDto | null;
  onClose: () => void;
  onHide?: () => void;
  openDirectlyToCardSelection?: boolean;
}

type AnimationPhase = "initial" | "energyTransfer" | "productionTransfer" | "final";

const RESOURCE_KEYS: { key: ResourceType; resField: keyof ResourcesDto }[] = [
  { key: "credit", resField: "credits" },
  { key: "steel", resField: "steel" },
  { key: "titanium", resField: "titanium" },
  { key: "plant", resField: "plants" },
  { key: "energy", resField: "energy" },
  { key: "heat", resField: "heat" },
];

const RESOURCE_ICON_TYPES: Record<string, string> = {
  credit: ResourceTypeCredit,
  steel: ResourceTypeSteel,
  titanium: ResourceTypeTitanium,
  plant: ResourceTypePlant,
  energy: ResourceTypeEnergy,
  heat: ResourceTypeHeat,
};

const ENERGY_INDEX = RESOURCE_KEYS.findIndex(({ key }) => key === "energy");

const PHASE_INITIAL_DELAY = 1000;
const COUNTER_DURATION = 1500;
const PHASE_FADE_BUFFER = 200;
const PHASE_PAUSE_BETWEEN = 500;

function useCounterAnimation(
  startValue: number,
  endValue: number,
  active: boolean,
  durationMs: number = COUNTER_DURATION,
): { value: number; done: boolean } {
  const [displayValue, setDisplayValue] = useState(startValue);
  const [done, setDone] = useState(false);

  useEffect(() => {
    if (!active) {
      setDisplayValue(startValue);
      setDone(false);
      return;
    }

    const delta = endValue - startValue;
    if (delta === 0) {
      setDisplayValue(endValue);
      setDone(true);
      return;
    }

    setDone(false);
    const steps = Math.abs(delta);
    const interval = durationMs / steps;
    const direction = delta > 0 ? 1 : -1;
    let current = startValue;

    const timer = setInterval(() => {
      current += direction;
      setDisplayValue(current);
      if (current === endValue) {
        clearInterval(timer);
        setDone(true);
      }
    }, interval);

    return () => clearInterval(timer);
  }, [startValue, endValue, active, durationMs]);

  return { value: displayValue, done };
}

const ProductionPhaseModal: React.FC<ProductionPhaseModalProps> = ({
  isOpen,
  gameState,
  onClose,
  onHide,
  openDirectlyToCardSelection = false,
}) => {
  const { isCompact } = useLayoutMode();
  const soundHandlesRef = useRef<HTMLAudioElement[]>([]);
  const [hasSubmittedCardSelection, setHasSubmittedCardSelection] = useState(false);
  const persistedSelectionRef = useRef<string[]>([]);
  const [currentPlayerIndex, setCurrentPlayerIndex] = useState(0);
  const [animationPhase, setAnimationPhase] = useState<AnimationPhase>("final");
  const [showCardSelection, setShowCardSelection] = useState(false);
  const prevSelectionCompleteRef = useRef<boolean | undefined>(undefined);
  const prevIsOpenRef = useRef(false);
  const animationPlayedRef = useRef(false);
  const animationTimersRef = useRef<NodeJS.Timeout[]>([]);

  const stopAllSounds = useCallback(() => {
    for (const handle of soundHandlesRef.current) {
      handle.pause();
      handle.currentTime = 0;
    }
    soundHandlesRef.current = [];
  }, []);

  const playScoreSound = useCallback(() => {
    const handle = audioService.playSoundWithHandle("production-score");
    if (handle) {
      soundHandlesRef.current.push(handle);
    }
  }, []);

  const clearAnimationTimers = useCallback(() => {
    for (const t of animationTimersRef.current) {
      clearTimeout(t);
    }
    animationTimersRef.current = [];
    stopAllSounds();
  }, [stopAllSounds]);

  // Clean up sounds on unmount
  useEffect(() => {
    return () => {
      for (const handle of soundHandlesRef.current) {
        handle.pause();
      }
    };
  }, []);

  const handleCardSelection = useCallback(
    async (selectedCardIds: string[], options?: { randomBuy?: boolean }) => {
      try {
        await globalWebSocketManager.confirmProductionCards(selectedCardIds, options);
        setHasSubmittedCardSelection(true);
        setShowCardSelection(false);
      } catch (error) {
        console.error("Failed to submit card selection:", error);
        onClose();
      }
    },
    [onClose],
  );

  const isLastRound = gameState?.isLastRound ?? false;

  const handleNextClick = useCallback(() => {
    clearAnimationTimers();
    animationPlayedRef.current = true;
    setAnimationPhase("final");

    if (isLastRound) {
      void handleCardSelection([]);
    } else {
      setShowCardSelection(true);
    }
  }, [isLastRound, handleCardSelection, clearAnimationTimers]);

  const handleReturnFromCardSelection = useCallback(() => {
    if (onHide) {
      onHide();
    }
  }, [onHide]);

  useEffect(() => {
    const selectionComplete = gameState?.currentPlayer?.productionPhase?.selectionComplete;
    const prev = prevSelectionCompleteRef.current;
    const wasOpen = prevIsOpenRef.current;

    const selectionTransitioned =
      selectionComplete === false && (prev === undefined || prev === true);
    const modalJustOpened = isOpen && !wasOpen && selectionComplete === false;

    if (isOpen && (selectionTransitioned || modalJustOpened)) {
      setHasSubmittedCardSelection(false);
      setShowCardSelection(openDirectlyToCardSelection);
      persistedSelectionRef.current = [];
      animationPlayedRef.current = false;
    }

    prevSelectionCompleteRef.current = selectionComplete;
    prevIsOpenRef.current = isOpen;
  }, [
    isOpen,
    gameState?.currentPlayer?.productionPhase?.selectionComplete,
    openDirectlyToCardSelection,
  ]);

  const modalProductionData = useMemo(() => {
    if (!gameState || !gameState.currentPlayer?.productionPhase) {
      return null;
    }

    const allPlayers: (PlayerDto | OtherPlayerDto)[] = [
      gameState.currentPlayer,
      ...gameState.otherPlayers,
    ];

    const playersWithProduction = allPlayers.filter((player) => player.productionPhase);

    if (playersWithProduction.length === 0) {
      return null;
    }

    const playersData = playersWithProduction.map((player) => {
      const productionPhase = player.productionPhase as ProductionPhaseDto;
      return {
        playerId: player.id,
        playerName: player.name,
        playerColor: player.color,
        production: productionPhase.productionUsed,
        terraformRating: player.terraformRating,
        ...productionPhase,
      };
    });

    return {
      playersData,
      generation: gameState.generation || 1,
    };
  }, [gameState]);

  useEffect(() => {
    if (!isOpen || !modalProductionData || animationPlayedRef.current) {
      return;
    }

    const currentPlayerData = modalProductionData.playersData[0];
    const hasEnergyToConvert = currentPlayerData && currentPlayerData.energyConverted > 0;

    setAnimationPhase("initial");

    const timers: NodeJS.Timeout[] = [];

    const energyPhaseDuration = COUNTER_DURATION + PHASE_FADE_BUFFER;
    const productionPhaseDuration = COUNTER_DURATION + PHASE_FADE_BUFFER;

    const t1 = setTimeout(() => {
      if (hasEnergyToConvert) {
        setAnimationPhase("energyTransfer");
        playScoreSound();
      } else {
        setAnimationPhase("productionTransfer");
        playScoreSound();
      }
    }, PHASE_INITIAL_DELAY);
    timers.push(t1);

    const stopAfterPhase1 = setTimeout(() => {
      stopAllSounds();
    }, PHASE_INITIAL_DELAY + COUNTER_DURATION);
    timers.push(stopAfterPhase1);

    const t2Delay = hasEnergyToConvert
      ? PHASE_INITIAL_DELAY + energyPhaseDuration + PHASE_PAUSE_BETWEEN
      : PHASE_INITIAL_DELAY;

    if (hasEnergyToConvert) {
      const t2 = setTimeout(() => {
        setAnimationPhase("productionTransfer");
        playScoreSound();
      }, t2Delay);
      timers.push(t2);

      const stopAfterPhase2 = setTimeout(() => {
        stopAllSounds();
      }, t2Delay + COUNTER_DURATION);
      timers.push(stopAfterPhase2);
    }

    const t3 = setTimeout(() => {
      setAnimationPhase("final");
      animationPlayedRef.current = true;
    }, t2Delay + productionPhaseDuration);
    timers.push(t3);

    animationTimersRef.current = timers;

    return () => {
      for (const t of timers) {
        clearTimeout(t);
      }
    };
  }, [isOpen, modalProductionData, playScoreSound, stopAllSounds]);

  const handlePlayerSelect = (playerIndex: number) => {
    if (playerIndex === currentPlayerIndex) {
      return;
    }

    if (!animationPlayedRef.current) {
      clearAnimationTimers();
      animationPlayedRef.current = true;
      setAnimationPhase("final");
    }

    setCurrentPlayerIndex(playerIndex);
  };

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    setCurrentPlayerIndex(0);
  }, [isOpen]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Enter" && !hasSubmittedCardSelection && !showCardSelection) {
        handleNextClick();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleKeyDown);
      return () => document.removeEventListener("keydown", handleKeyDown);
    }

    return () => {};
  }, [isOpen, hasSubmittedCardSelection, showCardSelection, handleNextClick]);

  if (!isOpen) {
    return null;
  }
  if (!modalProductionData) {
    return null;
  }

  const currentPlayerData = modalProductionData.playersData[currentPlayerIndex];
  if (!currentPlayerData) {
    return null;
  }

  const effectivePhase: AnimationPhase = currentPlayerIndex === 0 ? animationPhase : "final";

  const showNextButton = !hasSubmittedCardSelection && !showCardSelection;

  return (
    <>
      {!showCardSelection && isCompact && (
        <CompactProductionPanel
          generation={modalProductionData.generation}
          players={modalProductionData.playersData}
          currentPlayerIndex={currentPlayerIndex}
          onPlayerSelect={handlePlayerSelect}
          animationPhase={effectivePhase}
          showNextButton={showNextButton}
          nextLabel={isLastRound ? "Continue" : "Buy cards"}
          onNext={handleNextClick}
          onHide={isLastRound ? undefined : onHide}
        />
      )}

      {!showCardSelection && !isCompact && (
        <div
          className="fixed inset-0 flex items-center justify-center"
          style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}
        >
          <div className={OVERLAY_BACKDROP_BLUR_CLASS} />
          <div className={OVERLAY_BACKDROP_TINT_CLASS} />

          <div className="relative" style={{ zIndex: getZIndex("LOCAL", 1) }}>
            <div className="max-w-[1050px] min-w-[850px] flex flex-col game-panel game-panel-clipped game-window overflow-hidden">
              <div className={OVERLAY_HEADER_CLASS}>
                <h2 className={`${OVERLAY_TITLE_CLASS} text-center`}>Production</h2>
                <p className="mt-2 mb-0 text-base text-white/60 text-center">
                  Generation {modalProductionData.generation}
                </p>
              </div>

              {modalProductionData.playersData.length > 1 && (
                <div className="flex justify-center gap-3 px-8 py-4">
                  {modalProductionData.playersData.map((player, index) => (
                    <GameButton
                      key={player.playerId}
                      emphasis="secondary"
                      size="sm"
                      selected={index === currentPlayerIndex}
                      className={index === currentPlayerIndex ? "" : "opacity-60"}
                      onClick={() => handlePlayerSelect(index)}
                    >
                      <span
                        className="inline-block w-2.5 h-2.5 rounded-full mr-2 flex-shrink-0"
                        style={{ backgroundColor: player.playerColor }}
                      />
                      {player.playerName}
                    </GameButton>
                  ))}
                </div>
              )}

              <div className="px-8 pt-2 pb-14 max-[1100px]:pb-20">
                <ResourceGrid playerData={currentPlayerData} animationPhase={effectivePhase} />
              </div>
            </div>

            {showNextButton && (
              <div className="absolute left-full top-1/2 -translate-y-1/2 ml-5 max-[1100px]:left-auto max-[1100px]:right-6 max-[1100px]:top-auto max-[1100px]:bottom-4 max-[1100px]:translate-y-0 max-[1100px]:ml-0">
                <GameButton
                  emphasis="secondary"
                  size="lg"
                  onClick={handleNextClick}
                  className="whitespace-nowrap"
                >
                  Buy cards
                  <NextArrowIcon />
                </GameButton>
              </div>
            )}
          </div>
        </div>
      )}

      {showCardSelection && (
        <ProductionCardSelectionOverlay
          isOpen={showCardSelection}
          cards={gameState?.currentPlayer?.productionPhase?.availableCards || []}
          playerCredits={
            gameState?.currentPlayer?.actionCosts
              ?.find((a) => a.actionType === "card-buying")
              ?.costs.find((c) => c.resource === "credit")?.paymentCapacity ?? 0
          }
          costPerCard={
            gameState?.currentPlayer?.actionCosts?.find((a) => a.actionType === "card-buying")
              ?.costs[0]?.effectiveCost ?? 3
          }
          allowRandomBuy={gameState?.settings.allowRandomBuy ?? false}
          onSelectCards={handleCardSelection}
          onReturn={handleReturnFromCardSelection}
          initialSelectedCardIds={persistedSelectionRef.current}
          onSelectionChange={(ids) => {
            persistedSelectionRef.current = ids;
          }}
        />
      )}
    </>
  );
};

interface PlayerProductionData {
  beforeResources: ResourcesDto;
  afterResources: ResourcesDto;
  production: ProductionDto;
  energyConverted: number;
  creditsIncome: number;
  terraformRating: number;
}

interface ResourceFlow {
  startForEnergy: number;
  endForEnergy: number;
  startForProduction: number;
  endForProduction: number;
}

function resourceFlow(
  playerData: PlayerProductionData,
  key: ResourceType,
  resField: keyof ResourcesDto,
): ResourceFlow {
  const { beforeResources, afterResources, energyConverted } = playerData;
  const beforeVal = beforeResources[resField];

  let afterEnergyVal = beforeVal;
  let startForProduction = beforeVal;
  if (key === "energy") {
    afterEnergyVal = 0;
    startForProduction = 0;
  } else if (key === "heat") {
    afterEnergyVal = beforeResources.heat + energyConverted;
    startForProduction = afterEnergyVal;
  }

  return {
    startForEnergy: beforeVal,
    endForEnergy: afterEnergyVal,
    startForProduction,
    endForProduction: afterResources[resField],
  };
}

interface ResourceGridProps {
  playerData: PlayerProductionData;
  animationPhase: AnimationPhase;
}

const ResourceGrid: React.FC<ResourceGridProps> = ({ playerData, animationPhase }) => {
  const { beforeResources, production, energyConverted } = playerData;

  const isEnergyPhaseActive = animationPhase === "energyTransfer";
  const hasEnergy = beforeResources.energy > 0;
  const shouldAnimateEnergyTransfer = isEnergyPhaseActive && hasEnergy && energyConverted > 0;

  const energyDrainCounter = useCounterAnimation(
    beforeResources.energy,
    0,
    shouldAnimateEnergyTransfer,
  );
  const energyTransferDone = energyDrainCounter.done;

  return (
    <div className="flex flex-col items-center gap-2 py-4 scale-125 my-4">
      <div className="flex items-start justify-center gap-3 relative">
        {RESOURCE_KEYS.map(({ key, resField }, index) => {
          const flow = resourceFlow(playerData, key, resField);

          return (
            <React.Fragment key={key}>
              <div className="relative">
                <ResourceColumn
                  resourceKey={key}
                  productionValue={production[resField]}
                  startForEnergy={flow.startForEnergy}
                  endForEnergy={flow.endForEnergy}
                  startForProduction={flow.startForProduction}
                  endForProduction={flow.endForProduction}
                  animationPhase={animationPhase}
                  energyConverted={energyConverted}
                  terraformRating={key === "credit" ? playerData.terraformRating : undefined}
                />
                {key === "energy" && (
                  <EnergyToHeatArrows active={shouldAnimateEnergyTransfer && !energyTransferDone} />
                )}
              </div>
              {index < RESOURCE_KEYS.length - 1 && (
                <div
                  className={`w-[1px] h-[90px] self-center mt-2 transition-opacity duration-300 ${index === ENERGY_INDEX && shouldAnimateEnergyTransfer && !energyTransferDone ? "opacity-0" : ""}`}
                  style={{
                    background: "linear-gradient(transparent, rgba(60,60,70,0.8), transparent)",
                  }}
                />
              )}
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
};

function useResourceDisplay(
  resourceKey: ResourceType,
  flow: ResourceFlow,
  animationPhase: AnimationPhase,
  energyConverted: number,
) {
  const { startForEnergy, endForEnergy, startForProduction, endForProduction } = flow;
  const isEnergyPhaseActive = animationPhase === "energyTransfer";
  const isProductionPhaseActive = animationPhase === "productionTransfer";

  const isEnergyResource = resourceKey === "energy" || resourceKey === "heat";
  const shouldAnimateEnergy = isEnergyPhaseActive && isEnergyResource && energyConverted > 0;
  const shouldAnimateProduction =
    isProductionPhaseActive && endForProduction - startForProduction !== 0;

  const energyCounter = useCounterAnimation(startForEnergy, endForEnergy, shouldAnimateEnergy);

  const productionCounter = useCounterAnimation(
    startForProduction,
    endForProduction,
    shouldAnimateProduction,
  );

  const getDisplayedResourceValue = (): number => {
    if (animationPhase === "initial") {
      return startForEnergy;
    }
    if (animationPhase === "energyTransfer") {
      if (shouldAnimateEnergy) {
        return energyCounter.value;
      }
      return startForEnergy;
    }
    if (animationPhase === "productionTransfer") {
      if (shouldAnimateProduction) {
        return productionCounter.value;
      }
      return startForProduction;
    }
    return endForProduction;
  };

  return {
    displayedValue: getDisplayedResourceValue(),
    isEnergyPhaseActive,
    isProductionPhaseActive,
    isEnergyResource,
    productionCounter,
  };
}

interface ResourceColumnProps {
  resourceKey: ResourceType;
  productionValue: number;
  startForEnergy: number;
  endForEnergy: number;
  startForProduction: number;
  endForProduction: number;
  animationPhase: AnimationPhase;
  energyConverted: number;
  terraformRating?: number;
}

const ResourceColumn: React.FC<ResourceColumnProps> = ({
  resourceKey,
  productionValue,
  startForEnergy,
  endForEnergy,
  startForProduction,
  endForProduction,
  animationPhase,
  energyConverted,
  terraformRating,
}) => {
  const {
    displayedValue,
    isEnergyPhaseActive,
    isProductionPhaseActive,
    isEnergyResource,
    productionCounter,
  } = useResourceDisplay(
    resourceKey,
    { startForEnergy, endForEnergy, startForProduction, endForProduction },
    animationPhase,
    energyConverted,
  );

  const isDimmedDuringEnergy = isEnergyPhaseActive && !isEnergyResource;

  const hasProdDelta = endForProduction - startForProduction !== 0;
  const showDownArrows = isProductionPhaseActive && hasProdDelta && !productionCounter.done;

  const prodBadgeLift =
    animationPhase === "productionTransfer" && hasProdDelta && !productionCounter.done
      ? "-translate-y-2"
      : "";

  return (
    <div
      className={`flex flex-col items-center gap-0 px-3 py-2 min-w-[70px] transition-opacity duration-300 ${
        isDimmedDuringEnergy ? "opacity-30" : "opacity-100"
      }`}
    >
      <div className="relative">
        {resourceKey === "credit" && terraformRating !== undefined && (
          <div
            className={`absolute bottom-full left-1/2 -translate-x-1/2 mb-0.5 inline-flex items-center justify-center bg-[linear-gradient(135deg,rgba(80,80,120,0.5)_0%,rgba(60,60,100,0.45)_100%)] border border-[rgba(100,100,160,0.6)] py-0.5 w-[36px] transition-transform duration-500 ease-in-out ${prodBadgeLift}`}
          >
            <span className="text-[10px] font-bold font-orbitron text-white/90 [text-shadow:0_1px_2px_rgba(0,0,0,0.8)] leading-none tabular-nums">
              {terraformRating}
            </span>
          </div>
        )}
        <div
          className={`inline-flex items-center justify-center bg-[linear-gradient(135deg,rgba(160,110,60,0.5)_0%,rgba(139,89,42,0.45)_100%)] border border-[rgba(160,110,60,0.6)] px-3 py-0.5 w-[36px] transition-transform duration-500 ease-in-out ${prodBadgeLift}`}
        >
          <span className="text-[11px] font-bold font-orbitron text-white [text-shadow:0_1px_2px_rgba(0,0,0,0.8)] leading-none tabular-nums">
            {productionValue}
          </span>
        </div>
      </div>

      <div
        className={`h-[20px] flex items-center justify-center overflow-hidden transition-transform duration-500 ease-in-out ${showDownArrows ? "-translate-y-1" : ""}`}
      >
        {showDownArrows && <VerticalArrows />}
      </div>

      <div className="flex flex-col items-center gap-0.5">
        {resourceKey === "credit" ? (
          <div className="flex items-center gap-1 w-[60px] justify-center scale-110">
            <GameIcon iconType={ResourceTypeCredit} amount={displayedValue} size="small" />
          </div>
        ) : (
          <div className="flex items-center gap-1 w-[60px] justify-center scale-[0.85]">
            <GameIcon iconType={RESOURCE_ICON_TYPES[resourceKey]} size="small" />
            <span className="text-sm font-bold font-orbitron text-white [text-shadow:0_1px_3px_rgba(0,0,0,0.8)] tabular-nums min-w-[32px] text-left">
              {displayedValue}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};

const VerticalArrows: React.FC = () => {
  return (
    <div className="flex flex-col items-center leading-none overflow-hidden h-[20px] w-[16px] relative">
      <span className="text-[10px] text-white/80 font-bold absolute left-1/2 -translate-x-1/2 top-0 animate-[slideDown_0.6s_linear_infinite]">
        ▼
      </span>
      <span className="text-[10px] text-white/50 font-bold absolute left-1/2 -translate-x-1/2 top-0 animate-[slideDown_0.6s_linear_0.2s_infinite]">
        ▼
      </span>
      <span className="text-[10px] text-white/30 font-bold absolute left-1/2 -translate-x-1/2 top-0 animate-[slideDown_0.6s_linear_0.4s_infinite]">
        ▼
      </span>
    </div>
  );
};

interface EnergyToHeatArrowsProps {
  active: boolean;
}

const EnergyToHeatArrows: React.FC<EnergyToHeatArrowsProps> = ({ active }) => {
  return (
    <div
      className={`absolute right-0 translate-x-[100%] bottom-[13px] w-[30px] overflow-hidden transition-opacity duration-500 pointer-events-none ${
        active ? "opacity-100" : "opacity-0"
      }`}
    >
      <div className="relative h-[16px] w-[30px] overflow-hidden">
        <span className="text-[10px] text-white/80 font-bold absolute top-1/2 -translate-y-1/2 left-0 animate-[slideRight_0.6s_linear_infinite]">
          ▶
        </span>
        <span className="text-[10px] text-white/50 font-bold absolute top-1/2 -translate-y-1/2 left-0 animate-[slideRight_0.6s_linear_0.2s_infinite]">
          ▶
        </span>
        <span className="text-[10px] text-white/30 font-bold absolute top-1/2 -translate-y-1/2 left-0 animate-[slideRight_0.6s_linear_0.4s_infinite]">
          ▶
        </span>
      </div>
    </div>
  );
};

const NextArrowIcon: React.FC = () => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width="24"
    height="24"
    viewBox="0 0 24 24"
    className="inline-block ml-2"
  >
    <path
      fill="currentColor"
      d="m15.06 5.283l5.657 5.657a1.5 1.5 0 0 1 0 2.12l-5.656 5.658a1.5 1.5 0 0 1-2.122-2.122l3.096-3.096H4.5a1.5 1.5 0 0 1 0-3h11.535L12.94 7.404a1.5 1.5 0 0 1 2.122-2.121Z"
    />
  </svg>
);

interface CompactPlayerProduction extends PlayerProductionData {
  playerId: string;
  playerName: string;
  playerColor: string;
}

interface CompactProductionPanelProps {
  generation: number;
  players: CompactPlayerProduction[];
  currentPlayerIndex: number;
  onPlayerSelect: (index: number) => void;
  animationPhase: AnimationPhase;
  showNextButton: boolean;
  nextLabel: string;
  onNext: () => void;
  onHide?: () => void;
}

const CompactProductionPanel: React.FC<CompactProductionPanelProps> = ({
  generation,
  players,
  currentPlayerIndex,
  onPlayerSelect,
  animationPhase,
  showNextButton,
  nextLabel,
  onNext,
  onHide,
}) => {
  const playerData = players[currentPlayerIndex];
  if (!playerData) {
    return null;
  }

  return (
    <div className="fixed inset-0 h-dvh" style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}>
      <div className={OVERLAY_BACKDROP_BLUR_CLASS} />
      <div className={OVERLAY_BACKDROP_TINT_CLASS} />

      <div
        className="relative flex h-full w-full flex-col game-panel game-window overflow-hidden"
        style={{
          zIndex: getZIndex("LOCAL", 1),
          paddingTop: "var(--safe-top)",
          paddingBottom: "var(--safe-bottom)",
          paddingLeft: "var(--safe-left)",
          paddingRight: "var(--safe-right)",
        }}
      >
        <header className="flex shrink-0 items-center gap-4 border-b border-white/15 bg-black/40 px-3 py-1">
          <div className="shrink-0">
            <h2 className="m-0 font-orbitron text-base font-bold tracking-wider text-white text-shadow-glow">
              Production
            </h2>
            <p className="m-0 font-orbitron text-[11px] text-white/60">Generation {generation}</p>
          </div>
          {players.length > 1 && (
            <div className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto overscroll-contain py-0.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
              {players.map((player, index) => (
                <GameButton
                  key={player.playerId}
                  emphasis="secondary"
                  size="sm"
                  height={44}
                  selected={index === currentPlayerIndex}
                  className={`shrink-0 whitespace-nowrap ${index === currentPlayerIndex ? "" : "opacity-60"}`}
                  onClick={() => onPlayerSelect(index)}
                >
                  <span
                    className="inline-block w-2.5 h-2.5 rounded-full mr-2 flex-shrink-0"
                    style={{ backgroundColor: player.playerColor }}
                  />
                  {player.playerName}
                </GameButton>
              ))}
            </div>
          )}
        </header>

        <main className="flex min-h-0 flex-1 flex-col justify-center gap-3 overflow-y-auto overscroll-contain px-4 py-2">
          <div className="grid grid-cols-3 gap-2">
            {RESOURCE_KEYS.map(({ key, resField }) => (
              <CompactResourceCell
                key={key}
                resourceKey={key}
                before={playerData.beforeResources[resField]}
                productionValue={playerData.production[resField]}
                flow={resourceFlow(playerData, key, resField)}
                animationPhase={animationPhase}
                energyConverted={playerData.energyConverted}
              />
            ))}
          </div>
          <div className="flex flex-wrap items-center justify-center gap-x-6 gap-y-1 font-orbitron text-[13px] text-white/80">
            <span className="flex items-center gap-1.5">
              <GameIcon iconType={ResourceTypeEnergy} size="small" />
              <span className="text-white/50">to</span>
              <GameIcon iconType={ResourceTypeHeat} size="small" />
              <span className="font-bold tabular-nums text-white">
                {playerData.energyConverted}
              </span>
            </span>
            <span className="flex items-center gap-1.5">
              <span className="text-white/50">TR income</span>
              <span className="font-bold tabular-nums text-white">
                +{playerData.terraformRating}
              </span>
            </span>
          </div>
        </main>

        <footer className="flex shrink-0 items-center justify-between gap-3 border-t border-white/15 bg-black/40 px-3 py-1.5">
          {onHide ? (
            <GameButton emphasis="quiet" size="sm" height={44} onClick={onHide}>
              Hide
            </GameButton>
          ) : (
            <span />
          )}
          {showNextButton ? (
            <GameButton
              emphasis="secondary"
              size="sm"
              height={44}
              onClick={onNext}
              className="whitespace-nowrap"
            >
              {nextLabel}
              <NextArrowIcon />
            </GameButton>
          ) : (
            <span className="font-orbitron text-[12px] uppercase tracking-wider text-white/60">
              Waiting for other players
            </span>
          )}
        </footer>
      </div>
    </div>
  );
};

interface CompactResourceCellProps {
  resourceKey: ResourceType;
  before: number;
  productionValue: number;
  flow: ResourceFlow;
  animationPhase: AnimationPhase;
  energyConverted: number;
}

const CompactResourceCell: React.FC<CompactResourceCellProps> = ({
  resourceKey,
  before,
  productionValue,
  flow,
  animationPhase,
  energyConverted,
}) => {
  const { displayedValue, isEnergyPhaseActive, isEnergyResource } = useResourceDisplay(
    resourceKey,
    flow,
    animationPhase,
    energyConverted,
  );
  const dimmed = isEnergyPhaseActive && !isEnergyResource;
  const deltaPrefix = productionValue > 0 ? "+" : "";

  return (
    <div
      className={`flex items-center gap-2.5 border border-white/10 bg-black/30 px-3 py-2 transition-opacity duration-300 ${dimmed ? "opacity-30" : "opacity-100"}`}
    >
      <GameIcon iconType={RESOURCE_ICON_TYPES[resourceKey]} size="medium" />
      <div className="flex min-w-0 flex-col gap-1">
        <div className="flex items-baseline gap-1.5 font-orbitron tabular-nums">
          <span className="text-[13px] text-white/60">{before}</span>
          <span className="text-[12px] text-white/40" aria-hidden="true">
            &rarr;
          </span>
          <span className="text-[16px] font-bold text-white">{displayedValue}</span>
        </div>
        <span className="self-start border border-[rgba(160,110,60,0.6)] bg-[linear-gradient(135deg,rgba(160,110,60,0.5)_0%,rgba(139,89,42,0.45)_100%)] px-1.5 font-orbitron text-[13px] font-bold leading-snug tabular-nums text-white">
          {deltaPrefix}
          {productionValue}
        </span>
      </div>
    </div>
  );
};

export default ProductionPhaseModal;
