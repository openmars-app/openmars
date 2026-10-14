import { useState } from "react";
import type { PendingCardPlaySelectionDto } from "@/types/generated/api-types";
import { Z_INDEX } from "@/constants/zIndex";
import CardChoice from "../cards/CardChoice";
import GameButton from "../buttons/GameButton";
import CardSelectionRow from "./CardSelectionRow";
import OverlayDescription from "./OverlayDescription";
import { PlayabilityErrorList } from "./PlayabilityBadge";
import {
  OVERLAY_BACKDROP_BLUR_CLASS,
  OVERLAY_BACKDROP_TINT_CLASS,
  OVERLAY_CONTAINER_CLASS,
  OVERLAY_CONTAINER_STYLE,
  OVERLAY_HEADER_CLASS,
  OVERLAY_TITLE_CLASS,
  OVERLAY_FOOTER_CLASS,
  OVERLAY_ACTION_BUTTON_CLASS,
  OVERLAY_ROOT_SAFE_AREA_CLASS,
} from "./overlayStyles";

export default function CardPlaySelectionOverlay({
  selection,
  onPlay,
}: {
  selection: PendingCardPlaySelectionDto;
  onPlay: (cardId: string) => void;
}) {
  const [selected, setSelected] = useState<string | null>(null);
  const card = selection.availableCards.find((candidate) => candidate.id === selected);
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Play a card"
      className={`fixed inset-0 flex items-center justify-center ${OVERLAY_ROOT_SAFE_AREA_CLASS}`}
      style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}
    >
      <div className={OVERLAY_BACKDROP_BLUR_CLASS} />
      <div className={OVERLAY_BACKDROP_TINT_CLASS} />
      <div className={OVERLAY_CONTAINER_CLASS} style={OVERLAY_CONTAINER_STYLE}>
        <div className={OVERLAY_HEADER_CLASS}>
          <h2 className={OVERLAY_TITLE_CLASS}>Play a card</h2>
          <OverlayDescription>
            {selection.source}: reduce its cost by {selection.costReduction} M€.
          </OverlayDescription>
        </div>
        <CardSelectionRow>
          {selection.availableCards.map((candidate) => (
            <CardChoice
              key={candidate.id}
              card={candidate}
              isSelected={selected === candidate.id}
              onSelect={setSelected}
              showCheckbox
              showPlayability
              detailNotice={
                candidate.errors.length ? (
                  <PlayabilityErrorList errors={candidate.errors} />
                ) : undefined
              }
            />
          ))}
        </CardSelectionRow>
        <div className={OVERLAY_FOOTER_CLASS}>
          <GameButton
            size="lg"
            disabled={!card?.available}
            className={`${OVERLAY_ACTION_BUTTON_CLASS} ml-auto`}
            onClick={() => {
              if (card?.available) {
                onPlay(card.id);
              }
            }}
          >
            Play selected card
          </GameButton>
        </div>
      </div>
    </div>
  );
}
