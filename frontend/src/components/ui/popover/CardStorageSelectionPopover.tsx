import React, { useCallback, useState } from "react";
import {
  CardDto,
  ResourceType,
  ColonyResourceReason,
  ColonyResourceReasonTrade,
  ColonyResourceReasonColonyTax,
  ColonyResourceReasonBuild,
  ColonyResourceReasonColonyBonus,
} from "@/types/generated/api-types.ts";
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

interface CardStorageSelectionPopoverProps {
  resourceType: ResourceType;
  amount: number;
  selectorTags?: string[];
  eligibleCardIds?: string[];
  playedCards: CardDto[];
  corporationCard?: CardDto;
  resourceStorage?: { [cardId: string]: number };
  reason?: ColonyResourceReason;
  mandatory?: boolean;
  onCardSelect: (cardId: string) => void;
  onCancel: () => void;
  isVisible: boolean;
}

interface CardStorageOption {
  card: CardDto;
  currentStorage: number;
}

const CardStorageSelectionPopover: React.FC<CardStorageSelectionPopoverProps> = ({
  resourceType,
  amount,
  selectorTags,
  eligibleCardIds,
  playedCards,
  corporationCard,
  resourceStorage,
  reason,
  mandatory,
  onCardSelect,
  onCancel,
  isVisible,
}) => {
  const [hoveredCard, setHoveredCard] = useState<CardDto | null>(null);
  const [previewCard, setPreviewCard] = useState<CardDto | null>(null);
  const closePreview = useCallback(() => setPreviewCard(null), []);

  const isDependency = ["remove-dependency", "retire-dependency"].includes(resourceType);
  const importing = resourceType === "remove-dependency";
  const allCandidateCards = [
    ...(corporationCard?.resourceStorage ? [corporationCard] : []),
    ...playedCards,
  ];

  const validCards: CardStorageOption[] = allCandidateCards
    .filter((card) => {
      if (isDependency) return eligibleCardIds?.includes(card.id) ?? false;
      if (!card.resourceStorage) {
        return false;
      }
      if (resourceType === "card-resource") {
        if (!selectorTags || selectorTags.length === 0) {
          return true;
        }
        return selectorTags.some((tag) => card.tags?.includes(tag as any));
      }
      if (card.resourceStorage.type !== resourceType) {
        return false;
      }
      if (selectorTags && selectorTags.length > 0) {
        return selectorTags.some((tag) => card.tags?.includes(tag as any));
      }
      return true;
    })
    .map((card) => ({
      card,
      currentStorage: resourceStorage?.[card.id] || 0,
    }));

  const hasNoStorage = validCards.length === 0;
  const canDismiss = !mandatory || hasNoStorage;

  const handleContinueWithoutStorage = () => {
    onCardSelect("");
  };

  const reasonTitleMap: Record<string, string> = {
    [ColonyResourceReasonTrade]: "Trading Complete",
    [ColonyResourceReasonColonyTax]: "Colony Taxes Received",
    [ColonyResourceReasonBuild]: "Colony Built",
    [ColonyResourceReasonColonyBonus]: "Colony Bonus",
  };
  const title = isDependency
    ? importing
      ? "Import technology"
      : "Retire an operation"
    : hasNoStorage
      ? "No Storage Available"
      : (reason && reasonTitleMap[reason]) || "Select Card Storage";

  return (
    <GameFlowPopover
      isVisible={isVisible}
      onClose={onCancel}
      type={canDismiss ? "interactive" : "interactive-mandatory"}
      renderSiblings={
        <>
          <CardPreviewPanel card={hoveredCard} />
          <CardPreviewOverlay card={previewCard} onClose={closePreview} />
        </>
      }
    >
      <GameFlowTitle>
        <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
          {title}
        </h3>
        {!hasNoStorage && !isDependency && (
          <div className="text-white/60 text-xs compact:text-[13px] text-shadow-glow mt-1 flex items-center justify-center gap-1.5">
            <span>
              Place {amount} resource{amount !== 1 ? "s" : ""}
            </span>
            {resourceType !== "card-resource" && <GameIcon iconType={resourceType} size="small" />}
          </div>
        )}
        {isDependency && (
          <p className="text-sm text-white/70 mt-2">
            {importing
              ? "Choose an eligible project. Its ongoing operation, including upkeep, will become independent. Immediate rewards are not repeated."
              : "Choose an eligible project. Its dependent operation will stop permanently."}
          </p>
        )}
      </GameFlowTitle>

      <GameFlowBody>
        {hasNoStorage && isDependency ? (
          <p className="p-4 text-sm text-white/70">No eligible project is available.</p>
        ) : hasNoStorage ? (
          <div className="flex flex-col items-center justify-center py-8 px-4 text-center">
            <div className="flex items-center justify-center gap-3 mb-4">
              <div className="text-yellow-400 text-4xl">⚠️</div>
              <div className="flex items-center gap-1">
                <GameIcon
                  iconType={resourceType === "card-resource" ? "card" : resourceType}
                  size="large"
                />
                {[...new Set(selectorTags ?? [])].map((tag) => (
                  <GameIcon key={tag} iconType={`${tag}-tag`} size="small" className="!w-4 !h-4" />
                ))}
              </div>
            </div>
            <div className="text-white text-sm mb-3 font-semibold">
              No {resourceType === "card-resource" ? "matching" : resourceType} storage available
            </div>
            <div className="text-white/70 text-xs compact:text-[13px] max-w-[280px]">
              If you continue, the {resourceType === "card-resource" ? "resource" : resourceType}{" "}
              will be lost.
            </div>
          </div>
        ) : (
          validCards.map(({ card, currentStorage }, index) => {
            const delay = index * 0.05;

            return (
              <div
                key={card.id}
                className="
                  game-panel game-panel-clipped game-choice
                    px-3.5 py-3
                  mb-2 compact:min-h-11 cursor-pointer
                  transition-all duration-[250ms] ease-out
                  hover:brightness-125


                  animate-choiceSlideIn
                  flex items-center justify-between gap-3
                "
                style={{ animationDelay: `${delay}s` }}
                onClick={() => onCardSelect(card.id)}
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
                  {!isDependency && (
                    <>
                      <span className="text-white/60 text-xs compact:text-[13px] font-medium">
                        {currentStorage}
                      </span>
                      <GameIcon
                        iconType={card.resourceStorage?.type || resourceType}
                        size="small"
                      />
                    </>
                  )}
                  <CardPreviewButton card={card} onPreview={setPreviewCard} />
                </div>
              </div>
            );
          })
        )}
      </GameFlowBody>

      {(hasNoStorage || canDismiss) && (
        <GameFlowFooter className="gap-3">
          {hasNoStorage ? (
            <>
              <GameButton emphasis="primary" tone="info" size="sm" onClick={onCancel}>
                Cancel
              </GameButton>
              {!isDependency && (
                <GameButton
                  emphasis="secondary"
                  tone="warn"
                  size="sm"
                  clickSound={false}
                  onClick={handleContinueWithoutStorage}
                >
                  Continue Anyway
                </GameButton>
              )}
            </>
          ) : (
            <GameButton emphasis="secondary" tone="info" size="sm" onClick={onCancel}>
              Cancel
            </GameButton>
          )}
        </GameFlowFooter>
      )}
    </GameFlowPopover>
  );
};

export default CardStorageSelectionPopover;
