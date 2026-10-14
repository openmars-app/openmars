import React, { useCallback, useEffect, useState } from "react";
import { useOverlayEventTarget, isPreviewEvent } from "@/contexts/OverlayEnvironment";
import { CardDto, ResourceType, ResourceRemovalTargetDto } from "@/types/generated/api-types.ts";
import { getResourceName } from "@/utils/resourceColors.ts";
import GameIcon from "../display/GameIcon.tsx";
import CardTagList from "../cards/CardTagList.tsx";
import CardPreviewPanel, {
  CardPreviewButton,
  CardPreviewOverlay,
  CardPreviewThumbnail,
} from "./CardPreviewPanel.tsx";
import GameButton from "../buttons/GameButton.tsx";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "./GameFlowPopover.tsx";

interface CardResourcePlayer {
  id: string;
  name: string;
  playedCards: CardDto[];
  resourceStorage: { [cardId: string]: number };
}

interface CardResourceSelectionPopoverProps {
  resourceType: ResourceType;
  amount: number;
  removalTargets: ResourceRemovalTargetDto[];
  optional: boolean;
  selection: { kind: "remove" } | { kind: "spend"; playerId: string; eligibleCardIds: string[] };
  players: CardResourcePlayer[];
  onCardSelect: (cardId: string) => void;
  onCancel: () => void;
  isVisible: boolean;
}

const CardResourceSelectionPopover: React.FC<CardResourceSelectionPopoverProps> = ({
  resourceType,
  amount,
  removalTargets,
  optional,
  selection,
  players,
  onCardSelect,
  onCancel,
  isVisible,
}) => {
  const eventTarget = useOverlayEventTarget();
  const [selectedPlayerId, setSelectedPlayerId] = useState<string | null>(null);
  const [hoveredCard, setHoveredCard] = useState<CardDto | null>(null);
  const [previewCard, setPreviewCard] = useState<CardDto | null>(null);
  const closePreview = useCallback(() => setPreviewCard(null), []);

  const handleBackClick = () => {
    setSelectedPlayerId(null);
    setHoveredCard(null);
    setPreviewCard(null);
  };

  const handleCardClick = (cardId: string) => {
    setSelectedPlayerId(null);
    onCardSelect(cardId);
  };

  const handleContinueAnyway = () => {
    setSelectedPlayerId(null);
    onCardSelect("");
  };

  useEffect(() => {
    if (!isVisible) {
      setSelectedPlayerId(null);
      setHoveredCard(null);
      setPreviewCard(null);
    }
  }, [isVisible]);

  // Custom escape handling: back navigation when player is selected, close otherwise
  useEffect(() => {
    if (!isVisible) {
      return;
    }

    const handleEscape = (event: Event) => {
      if (
        !(event instanceof KeyboardEvent) ||
        (eventTarget === document && isPreviewEvent(event))
      ) {
        return;
      }
      if (event.key === "Escape") {
        if (selection.kind === "remove" && selectedPlayerId) {
          setSelectedPlayerId(null);
          setHoveredCard(null);
        } else {
          onCancel();
        }
      }
    };

    eventTarget.addEventListener("keydown", handleEscape);
    return () => {
      eventTarget.removeEventListener("keydown", handleEscape);
    };
  }, [isVisible, selectedPlayerId, selection.kind, onCancel, eventTarget]);

  const getPlayerCardsWithResource = (player: CardResourcePlayer) =>
    player.playedCards.flatMap((card) => {
      if (selection.kind === "spend") {
        if (player.id !== selection.playerId || !selection.eligibleCardIds.includes(card.id)) {
          return [];
        }
        return [{ card, count: player.resourceStorage[card.id] ?? 0 }];
      }
      const target = removalTargets.find(
        (t) =>
          t.playerId === player.id &&
          t.cardId === card.id &&
          t.resourceType === resourceType &&
          t.amount >= (optional ? 1 : amount),
      );
      return target ? [{ card, count: target.amount }] : [];
    });
  const getPlayerTotalResourceOnCards = (player: CardResourcePlayer) =>
    getPlayerCardsWithResource(player).reduce((total, item) => total + item.count, 0);

  const effectivePlayerId = selection.kind === "spend" ? selection.playerId : selectedPlayerId;
  const eligiblePlayers = players.filter((player) => getPlayerCardsWithResource(player).length > 0);
  const selectedPlayer = eligiblePlayers.find((player) => player.id === effectivePlayerId);
  const cardsWithResource = selectedPlayer ? getPlayerCardsWithResource(selectedPlayer) : [];
  const hasNoTargets = eligiblePlayers.length === 0;

  const removalTitle = (
    <>
      {selectedPlayer ? (
        <>
          <div className="flex items-center gap-2">
            <GameButton
              emphasis="quiet"
              size="sm"
              onClick={handleBackClick}
              className="!py-0 !px-0 compact:min-h-11 flex items-center gap-1"
            >
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polyline points="15 18 9 12 15 6" />
              </svg>
              Back
            </GameButton>
            <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
              {selectedPlayer.name}&apos;s Cards
            </h3>
          </div>
          <div className="text-white/60 text-xs compact:text-[13px] text-shadow-glow mt-1 flex items-center justify-center gap-1.5">
            <span>Select card to remove {amount}</span>
            <GameIcon iconType={resourceType} size="small" />
            <span>from</span>
          </div>
        </>
      ) : hasNoTargets ? (
        <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
          No Valid Targets
        </h3>
      ) : (
        <>
          <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
            Select Target
          </h3>
          <div className="text-white/60 text-xs compact:text-[13px] text-shadow-glow mt-1 flex items-center justify-center gap-1.5">
            <span>Remove {amount}</span>
            <GameIcon iconType={resourceType} size="small" />
            <span>from any card</span>
          </div>
        </>
      )}
    </>
  );

  const spendingTitle = (
    <>
      <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
        Select Card
      </h3>
      <div className="text-white/60 text-xs compact:text-[13px] text-shadow-glow mt-1 flex items-center justify-center gap-1.5">
        <span>
          Select a card to spend {amount} {getResourceName(resourceType, amount).toLowerCase()}
        </span>
        <GameIcon iconType={resourceType} size="small" />
      </div>
    </>
  );

  return (
    <GameFlowPopover
      isVisible={isVisible}
      onClose={onCancel}
      handleEscapeKey={false}
      className="min-w-[280px]"
      renderSiblings={
        <>
          <CardPreviewPanel card={hoveredCard} />
          <CardPreviewOverlay card={previewCard} onClose={closePreview} />
        </>
      }
    >
      <GameFlowTitle>{selection.kind === "spend" ? spendingTitle : removalTitle}</GameFlowTitle>

      <GameFlowBody>
        {selectedPlayer ? (
          cardsWithResource.map(({ card, count }, index) => {
            const delay = index * 0.05;
            return (
              <div
                key={card.id}
                className="
                  game-panel game-panel-clipped game-choice
                    px-3.5 py-3
                  mb-2 compact:min-h-11
                  transition-all duration-[250ms] ease-out
                  animate-choiceSlideIn
                  flex items-center justify-between gap-3
                  cursor-pointer hover:brightness-125
                "
                style={{ animationDelay: `${delay}s` }}
                onClick={() => handleCardClick(card.id)}
                onPointerEnter={(event) => {
                  if (event.pointerType === "mouse") {
                    setHoveredCard(card);
                  }
                }}
                onPointerLeave={() => setHoveredCard(null)}
              >
                <div className="flex items-center gap-3 min-w-0">
                  <CardPreviewThumbnail cardId={card.id} />
                  <div className="text-white font-semibold text-sm compact:min-w-0 compact:truncate">
                    {card.name}
                  </div>
                  <CardTagList
                    card={card}
                    size="sm"
                    className="hidden compact:flex items-center gap-1 shrink-0"
                  />
                </div>
                <div className="flex items-center gap-1.5 flex-shrink-0">
                  <span className="text-white/60 text-xs compact:text-[13px] font-medium">
                    {count}
                  </span>
                  <GameIcon iconType={resourceType} size="small" />
                  <CardPreviewButton card={card} onPreview={setPreviewCard} />
                </div>
              </div>
            );
          })
        ) : hasNoTargets ? (
          <div className="flex flex-col items-center justify-center py-8 px-4 text-center">
            <div className="flex items-center justify-center gap-3 mb-4">
              <GameIcon iconType={resourceType} size="large" />
            </div>
            <div className="text-white/70 text-xs compact:text-[13px] mb-4 max-w-[280px]">
              No cards have removable {resourceType} resources available.
              {optional ? " You may skip this removal." : " This action needs a valid source."}
            </div>
          </div>
        ) : (
          eligiblePlayers.map((player, index) => {
            const totalResources = getPlayerTotalResourceOnCards(player);
            const delay = index * 0.05;

            return (
              <div
                key={player.id}
                className="
                  game-panel game-panel-clipped game-choice
                    px-3.5 py-3
                  mb-2 compact:min-h-11
                  transition-all duration-[250ms] ease-out
                  animate-choiceSlideIn
                  flex items-center justify-between gap-3
                  cursor-pointer hover:brightness-125
                "
                style={{ animationDelay: `${delay}s` }}
                onClick={() => setSelectedPlayerId(player.id)}
              >
                <div className="text-white font-semibold text-sm">{player.name}</div>
                <div className="flex items-center gap-1.5 flex-shrink-0">
                  <span className="text-white/60 text-xs compact:text-[13px] font-medium">
                    {totalResources}
                  </span>
                  <GameIcon iconType={resourceType} size="small" />
                </div>
              </div>
            );
          })
        )}
      </GameFlowBody>

      <GameFlowFooter className="gap-3">
        {optional ? (
          <>
            <GameButton
              emphasis="primary"
              tone="warn"
              size="sm"
              clickSound={false}
              onClick={handleContinueAnyway}
            >
              Skip
            </GameButton>
            <GameButton emphasis="secondary" tone="info" size="sm" onClick={onCancel}>
              Cancel
            </GameButton>
          </>
        ) : (
          <GameButton emphasis="secondary" tone="info" size="sm" onClick={onCancel}>
            Cancel
          </GameButton>
        )}
      </GameFlowFooter>
    </GameFlowPopover>
  );
};

export default CardResourceSelectionPopover;
