import {
  useOverlayPortalRoot,
  useOverlayEventTarget,
  isPreviewEvent,
} from "@/contexts/OverlayEnvironment";
import React, { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { CardDto } from "@/types/generated/api-types.ts";
import { cardImage } from "@/assets";
import { Z_INDEX } from "@/constants/zIndex.ts";
import GameCard from "../cards/GameCard.tsx";
import CorporationCard from "../cards/CorporationCard.tsx";
import GameButton from "../buttons/GameButton.tsx";
import { InfoGlyph } from "../display/InfoTooltip.tsx";

const noop = () => {};
const stopPropagation = (event: React.SyntheticEvent) => event.stopPropagation();

const CardPreview: React.FC<{ card: CardDto }> = ({ card }) => {
  if (card.type === "corporation") {
    return (
      <div style={{ transform: "scale(0.55)", transformOrigin: "top left", width: 400, height: 0 }}>
        <CorporationCard
          card={card}
          isSelected={false}
          onSelect={noop}
          showCheckbox={false}
          disableInteraction={true}
        />
      </div>
    );
  }
  return (
    <div className="card-size">
      <GameCard card={card} />
    </div>
  );
};

const CardPreviewPanel: React.FC<{ card: CardDto | null }> = ({ card }) => {
  const previewRef = useRef<HTMLDivElement>(null);
  const [fits, setFits] = useState(false);
  useLayoutEffect(() => {
    const element = previewRef.current;
    if (!element) return;
    const update = () => setFits(element.getBoundingClientRect().left >= 8);
    update();
    const observer = new ResizeObserver(update);
    if (element.parentElement) observer.observe(element.parentElement);
    window.addEventListener("resize", update);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", update);
    };
  }, [card]);

  return (
    <div
      ref={previewRef}
      style={{ visibility: fits ? "visible" : "hidden" }}
      className={`
        compact:hidden absolute right-full top-0 mr-4
        transition-all duration-200 ease-out
        ${card ? "opacity-100 translate-x-0" : "opacity-0 translate-x-2 pointer-events-none"}
      `}
    >
      {card && <CardPreview card={card} />}
    </div>
  );
};

export const CardPreviewThumbnail: React.FC<{ cardId: string }> = ({ cardId }) => {
  const artwork = cardImage(cardId);
  return (
    <div className="hidden pointer-coarse:block w-10 h-14 shrink-0 overflow-hidden border border-white/20 bg-black/40">
      {artwork && (
        <img
          {...artwork}
          sizes="40px"
          alt=""
          loading="lazy"
          decoding="async"
          className="w-full h-full object-cover"
        />
      )}
    </div>
  );
};

export const CardPreviewButton: React.FC<{ card: CardDto; onPreview: (card: CardDto) => void }> = ({
  card,
  onPreview,
}) => (
  <GameButton
    emphasis="quiet"
    clickSound={false}
    aria-label={`Show ${card.name}`}
    className="hidden pointer-coarse:inline-flex size-11 p-0 shrink-0 text-white/70"
    onClick={(event) => {
      event.stopPropagation();
      onPreview(card);
    }}
  >
    <InfoGlyph />
  </GameButton>
);

export const CardPreviewOverlay: React.FC<{ card: CardDto | null; onClose: () => void }> = ({
  card,
  onClose,
}) => {
  const portalRoot = useOverlayPortalRoot();
  const eventTarget = useOverlayEventTarget();
  useEffect(() => {
    if (!card) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (eventTarget === document && isPreviewEvent(event)) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopImmediatePropagation();
        onClose();
      }
    };
    eventTarget.addEventListener("keydown", handleKeyDown, true);
    return () => eventTarget.removeEventListener("keydown", handleKeyDown, true);
  }, [eventTarget, card, onClose]);

  if (!card) {
    return null;
  }

  return createPortal(
    <div
      data-overlay-layer
      role="dialog"
      aria-modal="true"
      aria-label={card.name}
      className="fixed inset-0 flex bg-black/70 p-4 overflow-y-auto overscroll-contain animate-[fadeIn_150ms_ease-in]"
      style={{ zIndex: Z_INDEX.CARD_PREVIEW_OVERLAY }}
      onClick={(event) => {
        event.stopPropagation();
        onClose();
      }}
      onTouchMove={stopPropagation}
      onWheel={stopPropagation}
    >
      <div className="m-auto" onClick={stopPropagation}>
        {card.type === "corporation" ? (
          <div className="w-[400px] max-w-full">
            <CorporationCard
              card={card}
              isSelected={false}
              onSelect={noop}
              showCheckbox={false}
              disableInteraction={true}
            />
          </div>
        ) : (
          <CardPreview card={card} />
        )}
      </div>
    </div>,
    portalRoot,
  );
};

export default CardPreviewPanel;
