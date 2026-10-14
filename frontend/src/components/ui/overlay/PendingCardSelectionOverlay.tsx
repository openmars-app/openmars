import React, { useState } from "react";
import CardChoice from "../cards/CardChoice.tsx";
import GameIcon from "../display/GameIcon.tsx";
import { PendingCardSelectionDto, ResourceTypeCredit } from "../../../types/generated/api-types.ts";
import { useCardSelection } from "../../../hooks/useCardSelection.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import {
  OVERLAY_BACKDROP_BLUR_CLASS,
  OVERLAY_BACKDROP_TINT_CLASS,
  OVERLAY_CONTAINER_CLASS,
  OVERLAY_CONTAINER_STYLE,
  OVERLAY_HEADER_CLASS,
  OVERLAY_TITLE_CLASS,
  OVERLAY_FOOTER_CLASS,
  RESOURCE_LABEL_CLASS,
  OVERLAY_ACTION_BUTTON_CLASS,
  OVERLAY_FOOTER_LEFT_CLASS,
  OVERLAY_FOOTER_RIGHT_CLASS,
  OVERLAY_ROOT_SAFE_AREA_CLASS,
} from "./overlayStyles.ts";
import CardSelectionRow from "./CardSelectionRow.tsx";
import OverlayDescription from "./OverlayDescription.tsx";
import GameButton from "../buttons/GameButton.tsx";
import PlayabilityBadge, { PlayabilityErrorList } from "./PlayabilityBadge.tsx";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";

interface PendingCardSelectionOverlayProps {
  isOpen: boolean;
  selection: PendingCardSelectionDto;
  playerCredits: number;
  onSelectCards: (selectedCardIds: string[]) => void;
  onCancel?: () => void;
}

interface TitleInfo {
  title: string;
  description: string;
}

const PendingCardSelectionOverlay: React.FC<PendingCardSelectionOverlayProps> = ({
  isOpen,
  selection,
  playerCredits,
  onSelectCards,
  onCancel,
}) => {
  const {
    selectedCardIds,
    totalCost,
    totalReward,
    showConfirmation,
    isValidSelection,
    handleCardSelect,
    handleConfirm: handleCardConfirm,
    canAffordCard,
  } = useCardSelection({
    cards: selection.availableCards,
    isOpen,
    playerCredits,
    getCardCost: (cardId) => selection.cardCosts[cardId] || 0,
    getCardReward: (cardId) => selection.cardRewards[cardId] || 0,
    minCards: selection.minCards,
    maxCards: selection.maxCards,
  });

  const [showPlayability, setShowPlayability] = useState(false);
  const { isCompact } = useLayoutMode();

  if (!isOpen || selection.availableCards.length === 0) return null;

  const isSellPatents = selection.source === "sell-patents";

  const getTitleAndDescription = (source: string): TitleInfo => {
    switch (source) {
      case "sell-patents":
        return {
          title: "Sell Cards for Credits",
          description: "Select cards to sell. Each card gives you 1 MC.",
        };
      case "research-phase":
        return {
          title: "Buy Research Cards",
          description: "Select cards to purchase. Each card costs 3 MC.",
        };
      default:
        return {
          title: "Select Cards",
          description: "Choose cards from the available options.",
        };
    }
  };

  const getCardBadge = (
    cardId: string,
  ): { type: "reward" | "cost" | "free"; value?: number } | null => {
    const cost = selection.cardCosts[cardId] || 0;
    const reward = selection.cardRewards[cardId] || 0;

    if (reward > 0) {
      return { type: "reward", value: reward };
    } else if (cost > 0) {
      return { type: "cost", value: cost };
    } else if (cost === 0 && reward === 0) {
      return { type: "free" };
    }
    return null;
  };

  const handleConfirm = () => {
    handleCardConfirm(onSelectCards);
  };

  const handleCancel = () => {
    if (onCancel) {
      onCancel();
    } else {
      // If no cancel handler, treat as selecting 0 cards (if allowed)
      if (selection.minCards === 0) {
        onSelectCards([]);
      }
    }
  };

  const titleInfo = getTitleAndDescription(selection.source);
  const iconSize = isCompact ? "medium" : "large";

  return (
    <div
      className={`fixed inset-0 flex items-center justify-center ${OVERLAY_ROOT_SAFE_AREA_CLASS}`}
      style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}
    >
      <div className={OVERLAY_BACKDROP_BLUR_CLASS} />
      <div className={OVERLAY_BACKDROP_TINT_CLASS} />

      {/* Content container */}
      <div className={OVERLAY_CONTAINER_CLASS} style={OVERLAY_CONTAINER_STYLE}>
        {/* Header */}
        <div className={OVERLAY_HEADER_CLASS}>
          <div className="flex items-center justify-between w-full">
            <div>
              <h2 className={OVERLAY_TITLE_CLASS}>{titleInfo.title}</h2>
              <OverlayDescription>{titleInfo.description}</OverlayDescription>
            </div>
            {!isSellPatents && !isCompact && (
              <GameButton
                emphasis="secondary"
                size="sm"
                onClick={() => setShowPlayability((prev) => !prev)}
              >
                {showPlayability ? "Hide Playability" : "Show Playability"}
              </GameButton>
            )}
          </div>
        </div>

        {/* Cards display */}
        <CardSelectionRow>
          {selection.availableCards.map((card, index) => {
            const isSelected = selectedCardIds.includes(card.id);
            const badge = getCardBadge(card.id);
            const canAfford = canAffordCard(card.id);
            const hasErrors = !isSellPatents && !card.available && card.errors.length > 0;

            return (
              <div key={card.id} className="relative">
                <CardChoice
                  card={card}
                  showPlayability={isCompact || showPlayability}
                  isSelected={isSelected}
                  onSelect={handleCardSelect}
                  animationDelay={index * 100}
                  showCheckbox={true}
                  detailNotice={
                    hasErrors ? <PlayabilityErrorList errors={card.errors} /> : undefined
                  }
                />
                {/* Cost/Reward Badge (hidden for sell-patents since reward is obvious) */}
                {badge && selection.source !== "sell-patents" && (
                  <div
                    className={`absolute top-2 right-2 px-2 py-1 rounded-md font-bold text-sm shadow-lg compact:top-auto compact:right-auto compact:bottom-2 compact:left-2 compact:px-1.5 compact:py-0.5 compact:text-xs ${
                      badge.type === "cost" ? "bg-[#f44336] text-white" : "bg-[#4caf50] text-white"
                    }`}
                  >
                    {badge.type === "cost"
                      ? `${badge.value} MC`
                      : badge.type === "reward"
                        ? `+${badge.value} MC`
                        : "FREE"}
                  </div>
                )}
                {/* Unaffordable overlay (buy cost) */}
                {!canAfford && !isSelected && (
                  <div className="absolute inset-0 bg-black/60 rounded-lg flex items-center justify-center compact:pointer-events-none">
                    <span className="text-white/80 font-bold compact:text-sm">Can't Afford</span>
                  </div>
                )}
                {/* Playability errors */}
                {(isCompact || showPlayability) && hasErrors && (
                  <PlayabilityBadge errors={card.errors} />
                )}
                {(isCompact || showPlayability) && !!card.warnings?.length && !hasErrors && (
                  <PlayabilityBadge errors={card.warnings} warning />
                )}
              </div>
            );
          })}
        </CardSelectionRow>

        {/* Footer with cost and confirm button */}
        <div className={OVERLAY_FOOTER_CLASS}>
          <div className={OVERLAY_FOOTER_LEFT_CLASS}>
            <div className="flex items-center gap-3">
              <span className={RESOURCE_LABEL_CLASS}>
                {isCompact ? "Credits" : "Your Credits:"}
              </span>
              <GameIcon iconType={ResourceTypeCredit} amount={playerCredits} size={iconSize} />
            </div>
            {totalCost > 0 && (
              <div className="flex items-center gap-3">
                <span className={RESOURCE_LABEL_CLASS}>{isCompact ? "Cost" : "Total Cost:"}</span>
                <GameIcon iconType={ResourceTypeCredit} amount={totalCost} size={iconSize} />
              </div>
            )}
            {totalReward > 0 && (
              <div className="flex items-center gap-3">
                <span className={RESOURCE_LABEL_CLASS}>Gain:</span>
                <GameIcon iconType={ResourceTypeCredit} amount={totalReward} size={iconSize} />
              </div>
            )}
          </div>

          <div className={OVERLAY_FOOTER_RIGHT_CLASS}>
            <div className="text-sm compact:text-xs compact:leading-tight compact:max-w-36">
              {selectedCardIds.length === 0 ? (
                showConfirmation ? (
                  <span className="text-[#ff9800]">
                    Are you sure you don't want to select any cards?
                  </span>
                ) : (
                  <span className="text-white/70">No cards selected</span>
                )
              ) : (
                <span className="text-white/70">
                  {selectedCardIds.length} card
                  {selectedCardIds.length !== 1 ? "s" : ""} selected
                </span>
              )}
            </div>
            <div className="flex gap-3 items-center">
              {(onCancel || selection.minCards === 0) && (
                <GameButton
                  emphasis="quiet"
                  size="md"
                  className={OVERLAY_ACTION_BUTTON_CLASS}
                  onClick={handleCancel}
                >
                  Cancel
                </GameButton>
              )}
              <GameButton
                size="lg"
                clickSound={!isSellPatents}
                onClick={handleConfirm}
                disabled={!isValidSelection || totalCost > playerCredits}
                className={OVERLAY_ACTION_BUTTON_CLASS}
              >
                {showConfirmation
                  ? "Confirm Skip"
                  : selection.source === "sell-patents"
                    ? "Sell Cards"
                    : "Confirm Selection"}
              </GameButton>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PendingCardSelectionOverlay;
