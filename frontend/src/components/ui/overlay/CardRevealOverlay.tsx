import { useContext, useState } from "react";
import type { CardDto, PendingCardRevealDto } from "@/types/generated/api-types.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import GameCard from "../cards/GameCard.tsx";
import FittedCard from "../cards/FittedCard.tsx";
import { CardFitHeightContext, GAME_CARD_NATURAL_WIDTH } from "../cards/CardFitHeightContext.ts";
import GameButton from "../buttons/GameButton.tsx";
import GameIcon from "../display/GameIcon.tsx";
import CardDetailPortal from "./CardDetailPortal.tsx";
import OverlayDescription from "./OverlayDescription.tsx";
import {
  OVERLAY_ACTION_BUTTON_CLASS,
  OVERLAY_CONTAINER_CLASS,
  OVERLAY_CONTAINER_STYLE,
  OVERLAY_FOOTER_CLASS,
  OVERLAY_HEADER_CLASS,
  OVERLAY_ROOT_SAFE_AREA_CLASS,
  OVERLAY_TITLE_CLASS,
} from "./overlayStyles.ts";
import CardSelectionRow from "./CardSelectionRow.tsx";

const MATCH_LABEL_HEIGHT = 24;

function RevealedCard({
  card,
  matched,
  onInspect,
}: {
  card: CardDto;
  matched: boolean;
  onInspect: () => void;
}) {
  const fitHeight = useContext(CardFitHeightContext);
  let cardHeight: number | undefined;
  if (fitHeight !== null) {
    cardHeight = Math.max(0, fitHeight - MATCH_LABEL_HEIGHT);
  }
  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={`Inspect ${card.name}`}
      className="flex flex-col items-center cursor-pointer shrink-0"
      onClick={onInspect}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onInspect();
        }
      }}
    >
      <FittedCard naturalWidth={GAME_CARD_NATURAL_WIDTH} height={cardHeight}>
        <GameCard card={card} />
      </FittedCard>
      <p className="h-6 m-0 flex items-end font-orbitron text-[12px] text-white/80">
        {matched ? "Match" : "No match"}
      </p>
    </div>
  );
}

interface Props {
  reveal: PendingCardRevealDto;
  onConfirm: () => Promise<unknown>;
}

export default function CardRevealOverlay({ reveal, onConfirm }: Props) {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [inspected, setInspected] = useState<CardDto | null>(null);
  const isMatch = (cardId: string) =>
    reveal.results.find((result) => result.cardId === cardId)?.matched ?? false;
  const handleContinue = () => {
    setSubmitting(true);
    setError(null);
    void onConfirm()
      .catch((reason: unknown) => {
        setError(reason instanceof Error ? reason.message : "Could not confirm reveal");
      })
      .finally(() => setSubmitting(false));
  };

  return (
    <div
      className={`fixed inset-0 flex items-center justify-center bg-black/60 backdrop-blur-sm ${OVERLAY_ROOT_SAFE_AREA_CLASS}`}
      style={{ zIndex: Z_INDEX.SELECTION_POPOVER }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label={`${reveal.source}: revealed cards`}
        className={OVERLAY_CONTAINER_CLASS}
        style={OVERLAY_CONTAINER_STYLE}
      >
        <div className={OVERLAY_HEADER_CLASS}>
          <h2 className={OVERLAY_TITLE_CLASS}>{reveal.source}</h2>
          <OverlayDescription>Revealed and discarded.</OverlayDescription>
        </div>
        <CardSelectionRow>
          {reveal.cards.map((card) => (
            <RevealedCard
              key={card.id}
              card={card}
              matched={isMatch(card.id)}
              onInspect={() => setInspected(card)}
            />
          ))}
        </CardSelectionRow>
        <div className={OVERLAY_FOOTER_CLASS}>
          <div className="flex-1 min-w-0 flex items-center gap-3 text-[13px] text-white/80">
            {reveal.rewards.length > 0 ? (
              <div className="flex items-center gap-2" aria-label="Reward">
                <span className="font-orbitron text-[11px] uppercase text-white/60">Reward</span>
                {reveal.rewards.map((reward, i) => (
                  <GameIcon
                    key={i}
                    iconType={reward.resourceType}
                    amount={reward.amount}
                    size="small"
                  />
                ))}
              </div>
            ) : (
              <span>No reward.</span>
            )}
            {error && (
              <span className="truncate text-red-300" role="alert">
                {error}
              </span>
            )}
          </div>
          <GameButton
            className={OVERLAY_ACTION_BUTTON_CLASS}
            disabled={submitting}
            onClick={handleContinue}
          >
            Continue
          </GameButton>
        </div>
      </div>
      {inspected && <CardDetailPortal card={inspected} onClose={() => setInspected(null)} />}
    </div>
  );
}
