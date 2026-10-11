import { useEffect, useRef } from "react";
import type { PlayerCardDto } from "@/types/generated/api-types.ts";
import HandCardTile from "./HandCardTile.tsx";
import { useElementSize } from "@/hooks/useElementSize.ts";
import { useScrollAnchor } from "./useScrollAnchor.ts";

const ROW_PADDING = 16;

interface HandRowProps {
  cards: PlayerCardDto[];
  emptyMessage: string;
  highlightIds: ReadonlySet<string>;
  focusCardId: string | null;
  onSelect: (card: PlayerCardDto) => void;
}

export default function HandRow({
  cards,
  emptyMessage,
  highlightIds,
  focusCardId,
  onSelect,
}: HandRowProps) {
  const rowRef = useRef<HTMLDivElement>(null);
  const { height: rowHeight } = useElementSize(rowRef);
  const cardHeight = rowHeight > 0 ? Math.max(0, rowHeight - ROW_PADDING) : undefined;
  useScrollAnchor(rowRef, cards.map((card) => card.id).join("|"));

  useEffect(() => {
    if (!focusCardId) {
      return;
    }
    const tile = rowRef.current?.querySelector<HTMLElement>(
      `[data-hand-card-id="${CSS.escape(focusCardId)}"]`,
    );
    tile?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [focusCardId]);

  const occurrences = new Map<string, number>();
  const tiles = cards.map((card) => {
    const occurrence = occurrences.get(card.id) ?? 0;
    occurrences.set(card.id, occurrence + 1);
    return { card, key: occurrence === 0 ? card.id : `${card.id}-${occurrence}` };
  });

  return (
    <div
      ref={rowRef}
      className="relative h-full flex items-start justify-center-safe gap-5 px-6 py-2 scroll-px-6 overflow-x-auto overflow-y-hidden snap-x snap-proximity overscroll-x-contain"
    >
      {cards.length === 0 && (
        <div className="self-center text-[13px] text-white/50">{emptyMessage}</div>
      )}
      {tiles.map(({ card, key }) => (
        <HandCardTile
          key={key}
          card={card}
          height={cardHeight}
          highlighted={highlightIds.has(card.id)}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}
