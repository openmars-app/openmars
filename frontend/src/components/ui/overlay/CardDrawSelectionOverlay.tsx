import React, { useContext, useEffect, useState } from "react";
import GameCard from "../cards/GameCard.tsx";
import FittedCard from "../cards/FittedCard.tsx";
import { CardFitHeightContext, GAME_CARD_NATURAL_WIDTH } from "../cards/CardFitHeightContext.ts";
import CardChoice from "../cards/CardChoice.tsx";
import GameIcon from "../display/GameIcon.tsx";
import {
  PendingCardDrawSelectionDto,
  CardReceiptDto,
  ResourceTypeCredit,
} from "../../../types/generated/api-types.ts";
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
import CardDetailPortal from "./CardDetailPortal.tsx";

interface CardSelectionProps {
  mode: "selection";
  isOpen: boolean;
  selection: PendingCardDrawSelectionDto;
  playerCredits: number;
  onConfirm: (cardsToTake: string[], cardsToBuy: string[]) => void;
}

const CardSelection: React.FC<CardSelectionProps> = ({
  isOpen,
  selection,
  playerCredits,
  onConfirm,
}) => {
  const [cardsToTake, setCardsToTake] = useState<string[]>([]);
  const [cardsToBuy, setCardsToBuy] = useState<string[]>([]);
  const [showConfirmation, setShowConfirmation] = useState(false);
  const [showPlayability, setShowPlayability] = useState(false);
  const { isCompact } = useLayoutMode();

  // Initialize selection when overlay opens
  useEffect(() => {
    if (isOpen && selection.availableCards.length > 0) {
      setCardsToTake([]);
      setCardsToBuy([]);
      setShowConfirmation(false);
    }
  }, [isOpen, selection.availableCards]);

  if (!isOpen || selection.availableCards.length === 0) return null;

  // Pure card-draw: All shown cards must be taken (no choice)
  // Peek+Draw/Take: Some cards must/can be taken (player has choice)
  const isCardDraw =
    selection.maxBuyCount === 0 && selection.minFreeTakeCount === selection.availableCards.length;

  const getTitleAndDescription = (): {
    title: string;
    description: string;
  } => {
    if (isCardDraw) {
      return {
        title: "New cards",
        description: "",
      };
    }

    if (selection.playAsPrelude) {
      return {
        title: "Select prelude",
        description: "Select 1 prelude card to play",
      };
    }

    // For all peek/take/buy scenarios, use consistent "Select cards" title
    const maxCards = selection.freeTakeCount + selection.maxBuyCount;
    let description = `Select up to ${maxCards} cards`;
    if (selection.minFreeTakeCount > 0) {
      const count = selection.minFreeTakeCount;
      description = `Select ${count} card${count === 1 ? "" : "s"}`;
      if (maxCards > count) {
        description += `, up to ${maxCards} total`;
      }
    }
    return { title: "Select cards", description };
  };

  const canAffordBuy = (): boolean => {
    const currentBuyCost = cardsToBuy.length * selection.cardBuyCost;
    return currentBuyCost + selection.cardBuyCost <= playerCredits;
  };

  const handleCardSelect = (cardId: string) => {
    // For card-draw scenarios, auto-select all cards
    if (isCardDraw) {
      return;
    }

    // Reset confirmation when user selects cards
    if (showConfirmation) {
      setShowConfirmation(false);
    }

    // If card is already in take list, remove it
    if (cardsToTake.includes(cardId)) {
      setCardsToTake((prev) => prev.filter((id) => id !== cardId));
      return;
    }

    // If card is already in buy list, remove it
    if (cardsToBuy.includes(cardId)) {
      setCardsToBuy((prev) => prev.filter((id) => id !== cardId));
      return;
    }

    // Try to add to free take list first
    if (cardsToTake.length < selection.freeTakeCount) {
      setCardsToTake((prev) => [...prev, cardId]);
    } else if (cardsToBuy.length < selection.maxBuyCount && canAffordBuy()) {
      // Otherwise add to buy list if we can afford it
      setCardsToBuy((prev) => [...prev, cardId]);
    }
  };

  const handleConfirm = () => {
    // For card-draw, automatically select all cards
    if (isCardDraw) {
      const allCardIds = selection.availableCards.map((c) => c.id);
      onConfirm(allCardIds, []);
      return;
    }

    const totalSelected = cardsToTake.length + cardsToBuy.length;
    const maxAllowed = selection.freeTakeCount + selection.maxBuyCount;

    if (totalSelected > maxAllowed || cardsToTake.length < selection.minFreeTakeCount) {
      return; // Invalid selection
    }

    // Require confirmation in two scenarios:
    // 1. Discarding all cards (totalSelected === 0)
    // 2. Not taking all available free cards (cardsToTake.length < freeTakeCount)
    const needsConfirmation = totalSelected === 0 || cardsToTake.length < selection.freeTakeCount;

    if (needsConfirmation && !showConfirmation) {
      // First click - show confirmation
      setShowConfirmation(true);
      return;
    }

    // Second click or no confirmation needed - proceed with selection
    onConfirm(cardsToTake, cardsToBuy);
  };

  const getButtonText = (): string => {
    if (isCardDraw) {
      return "Return";
    }

    // For peek/take/buy scenarios
    const totalSelected = cardsToTake.length + cardsToBuy.length;

    // Check if confirmation is needed
    const needsConfirmation = totalSelected === 0 || cardsToTake.length < selection.freeTakeCount;

    if (needsConfirmation && showConfirmation) {
      return "Confirm Selection";
    }

    if (totalSelected === 0 && selection.minFreeTakeCount === 0) {
      return "Discard all";
    }

    // Show buy count if any cards are being bought
    if (cardsToBuy.length > 0) {
      return cardsToBuy.length === 1 ? "Buy 1 card" : `Buy ${cardsToBuy.length} cards`;
    }

    // Otherwise just confirm the free selection
    return "Confirm Selection";
  };

  const titleInfo = getTitleAndDescription();
  const iconSize = isCompact ? "medium" : "large";
  const totalBuyCost = cardsToBuy.length * selection.cardBuyCost;
  const totalSelected = cardsToTake.length + cardsToBuy.length;
  // For peek scenarios, allow any selection from 0 to max (including discarding all)
  const isValidSelection =
    isCardDraw ||
    (cardsToTake.length >= selection.minFreeTakeCount &&
      cardsToTake.length <= selection.freeTakeCount &&
      cardsToBuy.length <= selection.maxBuyCount &&
      totalSelected <= selection.freeTakeCount + selection.maxBuyCount &&
      totalBuyCost <= playerCredits);

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
              {titleInfo.description && (
                <OverlayDescription>{titleInfo.description}</OverlayDescription>
              )}
            </div>
            {!isCardDraw && !isCompact && (
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
            const isSelected = cardsToTake.includes(card.id) || cardsToBuy.includes(card.id);
            const hasErrors = !isCardDraw && !card.available && card.errors.length > 0;

            return (
              <div key={card.id} className="relative">
                <CardChoice
                  card={card}
                  showPlayability={isCompact || showPlayability}
                  isSelected={isSelected}
                  onSelect={handleCardSelect}
                  animationDelay={index * 100}
                  showCheckbox={!isCardDraw}
                  detailNotice={
                    hasErrors ? <PlayabilityErrorList errors={card.errors} /> : undefined
                  }
                />
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
          {!selection.playAsPrelude && (
            <div className={OVERLAY_FOOTER_LEFT_CLASS}>
              <div className="flex items-center gap-3">
                <span className={RESOURCE_LABEL_CLASS}>
                  {isCompact ? "Credits" : "Available payment:"}
                </span>
                <GameIcon iconType={ResourceTypeCredit} amount={playerCredits} size={iconSize} />
              </div>
              {totalBuyCost > 0 && (
                <div className="flex items-center gap-3">
                  <span className={RESOURCE_LABEL_CLASS}>Buy Cost:</span>
                  <GameIcon iconType={ResourceTypeCredit} amount={totalBuyCost} size={iconSize} />
                </div>
              )}
            </div>
          )}

          <div className={`${OVERLAY_FOOTER_RIGHT_CLASS} ml-auto`}>
            <div className="text-sm compact:text-xs compact:leading-tight compact:max-w-56">
              {isCardDraw ? (
                <span className="text-white/70">
                  Drawing {selection.freeTakeCount} card
                  {selection.freeTakeCount !== 1 ? "s" : ""}
                </span>
              ) : showConfirmation ? (
                <span className="text-[#ff9800]">
                  {totalSelected === 0
                    ? "Are you sure you want to discard all?"
                    : (() => {
                        const remainingFreeTakes = selection.freeTakeCount - cardsToTake.length;
                        return `You can take ${remainingFreeTakes} more card${remainingFreeTakes !== 1 ? "s" : ""} for free. Confirm?`;
                      })()}
                </span>
              ) : (
                (() => {
                  const discardCount = selection.availableCards.length - totalSelected;
                  return discardCount > 0 ? (
                    <span className="text-white/70">
                      Discard {discardCount} card
                      {discardCount !== 1 ? "s" : ""}
                    </span>
                  ) : null;
                })()
              )}
            </div>
            <div className="flex gap-3 items-center">
              <GameButton
                size="lg"
                onClick={handleConfirm}
                disabled={!isValidSelection || totalBuyCost > playerCredits}
                className={OVERLAY_ACTION_BUTTON_CLASS}
              >
                {getButtonText()}
              </GameButton>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

function ReceivedCard({ card }: { card: CardReceiptDto["cards"][number] }) {
  const { isCompact } = useLayoutMode();
  const fitHeight = useContext(CardFitHeightContext);
  const [inspecting, setInspecting] = useState(false);
  if (!isCompact) {
    return (
      <div className="card-size">
        <GameCard card={card} />
      </div>
    );
  }
  return (
    <div className="relative cursor-pointer" onClick={() => setInspecting(true)}>
      <FittedCard naturalWidth={GAME_CARD_NATURAL_WIDTH} height={fitHeight ?? undefined}>
        <GameCard card={card} />
      </FittedCard>
      {inspecting && <CardDetailPortal card={card} onClose={() => setInspecting(false)} />}
    </div>
  );
}

type CardDrawSelectionOverlayProps =
  | CardSelectionProps
  | {
      mode: "received";
      isOpen: boolean;
      receipt: CardReceiptDto;
      onClose: () => void;
    };

function CardDrawSelectionOverlay(props: CardDrawSelectionOverlayProps) {
  if (props.mode === "selection") return <CardSelection {...props} />;
  if (!props.isOpen) return null;
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Cards received"
      className={`fixed inset-0 flex items-center justify-center ${OVERLAY_ROOT_SAFE_AREA_CLASS}`}
      style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}
    >
      <div className={OVERLAY_BACKDROP_BLUR_CLASS} />
      <div className={OVERLAY_BACKDROP_TINT_CLASS} />
      <div className={OVERLAY_CONTAINER_CLASS} style={OVERLAY_CONTAINER_STYLE}>
        <div className={OVERLAY_HEADER_CLASS}>
          <h2 className={OVERLAY_TITLE_CLASS}>Cards received</h2>
          <OverlayDescription>{props.receipt.source}</OverlayDescription>
        </div>
        <CardSelectionRow>
          {props.receipt.cards.map((card) => (
            <ReceivedCard key={card.id} card={card} />
          ))}
        </CardSelectionRow>
        <div className={OVERLAY_FOOTER_CLASS}>
          <GameButton
            size="lg"
            className={`${OVERLAY_ACTION_BUTTON_CLASS} ml-auto`}
            onClick={props.onClose}
          >
            Close
          </GameButton>
        </div>
      </div>
    </div>
  );
}

export default CardDrawSelectionOverlay;
