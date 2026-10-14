import {
  memo,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from "react";
import type { KeyboardEvent } from "react";
import { defaultRangeExtractor, useVirtualizer } from "@tanstack/react-virtual";
import type { Range } from "@tanstack/react-virtual";
import type { CardDto } from "@/types/generated/api-types.ts";
import { whenIdle } from "@/utils/scheduling.ts";
import { getCorporationBorderColor } from "@/utils/corporationColors.ts";
import GameCard from "../cards/GameCard.tsx";
import CorporationCard from "../cards/CorporationCard.tsx";
import type { CardDisplaySize, CardFamily } from "./cardCatalog.ts";

const noop = () => {};
const CORPORATION_CARD_WIDTH = 400;
const CORPORATION_ROW_HEIGHT = 450;
const PROJECT_ROW_HEIGHT = 348;
const INSPECTION_ROW_HEIGHT = 572;
const PROJECT_ROW_EXTRA_HEIGHT = 68;

const CatalogCard = memo(function CatalogCard({
  card,
  size,
  selected,
  tabIndex,
  onSelect,
}: {
  card: CardDto;
  size: CardDisplaySize;
  selected: boolean;
  tabIndex: number;
  onSelect: (id: string) => void;
}) {
  const corporation = card.type === "corporation";
  const projectWidth = size === "large" ? "var(--card-inspection-width)" : "var(--card-width)";
  return (
    <div
      role="checkbox"
      aria-checked={selected}
      aria-label={`${card.name} (${card.id})`}
      tabIndex={tabIndex}
      data-card-id={card.id}
      className={`mx-auto w-full pb-3 cursor-pointer rounded-sm outline-none motion-reduce:[&_*]:animate-none motion-reduce:[&_*]:transition-none focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-4 focus-visible:ring-offset-black ${corporation ? "" : "grid row-span-2 grid-rows-subgrid"}`}
      style={{ maxWidth: corporation ? `min(${CORPORATION_CARD_WIDTH}px, 100%)` : projectWidth }}
      onClick={() => onSelect(card.id)}
    >
      {corporation ? (
        <CorporationCard
          card={card}
          isSelected={selected}
          onSelect={noop}
          catalog
          showCheckbox
          borderColor={getCorporationBorderColor(card.name)}
        />
      ) : (
        <GameCard
          card={card}
          isSelected={selected}
          showCheckbox
          presentation={size === "large" ? "inspection" : "compact"}
        />
      )}
    </div>
  );
});

interface VirtualCardGridProps {
  cards: CardDto[];
  family: CardFamily;
  size: CardDisplaySize;
  selected: ReadonlySet<string>;
  onSelect: (id: string) => void;
}

const VirtualCardGrid = memo(function VirtualCardGrid({
  cards,
  family,
  size,
  selected,
  onSelect,
}: VirtualCardGridProps) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const [{ width, projectCardWidth, projectCardHeight }, setDimensions] = useState({
    width: 0,
    projectCardWidth: 0,
    projectCardHeight: 0,
  });
  const [focusedId, setFocusedId] = useState<string | null>(null);
  const pendingFocus = useRef<string | null>(null);
  const anchorId = useRef<string | null>(null);
  const cardWidth =
    family === "corporation" ? Math.min(CORPORATION_CARD_WIDTH, width) : projectCardWidth;
  const columns = Math.max(1, Math.floor((width - 8 + 24) / (cardWidth + 24)));
  let projectRowHeight = PROJECT_ROW_HEIGHT;
  if (size === "large") {
    projectRowHeight = INSPECTION_ROW_HEIGHT;
  } else if (projectCardHeight > 0) {
    projectRowHeight = Math.min(PROJECT_ROW_HEIGHT, projectCardHeight + PROJECT_ROW_EXTRA_HEIGHT);
  }
  const rows = useMemo(() => {
    const result: CardDto[][] = [];
    for (let i = 0; i < cards.length; i += columns) {
      result.push(cards.slice(i, i + columns));
    }
    return result;
  }, [cards, columns]);
  const focusedIndex = cards.findIndex((card) => card.id === focusedId);
  const focusedRow = focusedIndex < 0 ? -1 : Math.floor(focusedIndex / columns);
  const rangeExtractor = useCallback(
    (range: Range) => {
      const indexes = defaultRangeExtractor(range);
      if (focusedRow >= 0 && !indexes.includes(focusedRow)) {
        indexes.push(focusedRow);
        indexes.sort((a, b) => a - b);
      }
      return indexes;
    },
    [focusedRow],
  );
  const getItemKey = useCallback(
    (index: number) => rows[index].map((card) => card.id).join(","),
    [rows],
  );
  const virtualizer = useVirtualizer({
    count: width > 0 ? rows.length : 0,
    getScrollElement: () => viewportRef.current,
    estimateSize: () => (family === "corporation" ? CORPORATION_ROW_HEIGHT : projectRowHeight),
    getItemKey,
    overscan: 2,
    gap: 24,
    paddingStart: 24,
    paddingEnd: 24,
    rangeExtractor,
  });
  const virtualRows = virtualizer.getVirtualItems();
  const previous = useRef({ cards, columns, cardWidth });

  useLayoutEffect(() => {
    const content = contentRef.current;
    if (!content) {
      return;
    }
    const measure = () => {
      const style = getComputedStyle(content);
      const dimensions = {
        width: content.clientWidth,
        projectCardWidth: parseFloat(
          style.getPropertyValue(size === "large" ? "--card-inspection-width" : "--card-width"),
        ),
        projectCardHeight: parseFloat(style.getPropertyValue("--card-height")) || 0,
      };
      setDimensions((previous) =>
        previous.width === dimensions.width &&
        previous.projectCardWidth === dimensions.projectCardWidth &&
        previous.projectCardHeight === dimensions.projectCardHeight
          ? previous
          : dimensions,
      );
    };
    const observer = new ResizeObserver(measure);
    observer.observe(content);
    window.addEventListener("resize", measure);
    measure();
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [size]);

  useLayoutEffect(() => {
    const changedResults = previous.current.cards !== cards;
    const changedColumns = previous.current.columns !== columns;
    const changedSize = previous.current.cardWidth !== cardWidth;
    previous.current = { cards, columns, cardWidth };
    if (!changedResults && !changedColumns && !changedSize) {
      return;
    }
    virtualizer.measure();
    if (changedResults) {
      setFocusedId(null);
      pendingFocus.current = null;
      anchorId.current = cards[0]?.id ?? null;
      virtualizer.scrollToOffset(0);
    } else if (!anchorId.current || (virtualizer.scrollOffset ?? 0) < 25) {
      virtualizer.scrollToOffset(0);
    } else {
      const index = Math.max(
        0,
        cards.findIndex((card) => card.id === anchorId.current),
      );
      virtualizer.scrollToIndex(Math.floor(index / columns), { align: "start" });
    }
  }, [cards, columns, cardWidth, virtualizer]);

  useLayoutEffect(() => {
    if (pendingFocus.current) {
      const element = viewportRef.current?.querySelector<HTMLElement>(
        `[data-card-id="${CSS.escape(pendingFocus.current)}"]`,
      );
      if (element) {
        element.focus({ preventScroll: true });
        pendingFocus.current = null;
      }
    }
  });

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const element = (event.target as HTMLElement).closest<HTMLElement>("[data-card-id]");
    const index = cards.findIndex((card) => card.id === element?.dataset.cardId);
    if (index < 0) {
      return;
    }
    if (event.key === " " || event.key === "Enter") {
      event.preventDefault();
      onSelect(cards[index].id);
      return;
    }
    let next = index;
    switch (event.key) {
      case "ArrowRight":
        next++;
        break;
      case "ArrowLeft":
        next--;
        break;
      case "ArrowDown":
        next += columns;
        break;
      case "ArrowUp":
        next -= columns;
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = cards.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    next = Math.max(0, Math.min(cards.length - 1, next));
    pendingFocus.current = cards[next].id;
    setFocusedId(cards[next].id);
    virtualizer.scrollToIndex(Math.floor(next / columns), { align: "auto" });
  };

  return (
    <div
      ref={viewportRef}
      role="region"
      aria-label="Card results"
      className="h-full min-h-0 overflow-y-auto overscroll-contain [scrollbar-gutter:stable]"
      onKeyDown={handleKeyDown}
      onFocus={(event) => {
        const element = (event.target as HTMLElement).closest<HTMLElement>("[data-card-id]");
        if (element?.dataset.cardId) {
          setFocusedId(element.dataset.cardId);
        }
      }}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setFocusedId(null);
        }
      }}
      onScroll={() => {
        const offset = viewportRef.current?.scrollTop ?? 0;
        const row = virtualizer.getVirtualItems().find((item) => item.end > offset);
        if (row) {
          anchorId.current = rows[row.index]?.[0]?.id ?? null;
        }
      }}
    >
      <div
        ref={contentRef}
        className="relative mx-4 lg:mx-6"
        style={{ height: virtualizer.getTotalSize() }}
      >
        {virtualRows.map((row) => (
          <div
            key={row.key}
            ref={virtualizer.measureElement}
            data-index={row.index}
            data-card-row
            className="absolute left-0 top-0 grid w-full gap-x-6 px-1 will-change-transform"
            style={{
              transform: `translateY(${row.start}px)`,
              gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
              gridTemplateRows: family === "corporation" ? undefined : "auto auto",
            }}
          >
            {rows[row.index].map((card) => (
              <CatalogCard
                key={card.id}
                card={card}
                size={size}
                selected={selected.has(card.id)}
                tabIndex={card.id === (focusedId ?? cards[0]?.id) ? 0 : -1}
                onSelect={onSelect}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
});

function sameGridProps(a: VirtualCardGridProps, b: VirtualCardGridProps) {
  return (
    a.cards === b.cards &&
    a.family === b.family &&
    a.size === b.size &&
    a.selected === b.selected &&
    a.onSelect === b.onSelect
  );
}

export default memo(function CachedCardFamily({
  active,
  cards,
  family,
  size,
  selected,
  onSelect,
}: VirtualCardGridProps & { active: boolean }) {
  const latest = { cards, family, size, selected, onSelect };
  const shown = useRef(latest);
  const [, refresh] = useReducer((count: number) => count + 1, 0);
  if (active) {
    shown.current = latest;
  }
  const stale = !sameGridProps(shown.current, latest);
  useEffect(() => {
    if (!stale) {
      return;
    }
    return whenIdle(() => {
      shown.current = { cards, family, size, selected, onSelect };
      refresh();
    });
  }, [stale, cards, family, size, selected, onSelect]);
  return (
    <div
      className={`absolute inset-0 ${active ? "" : "[content-visibility:hidden] pointer-events-none"}`}
      inert={!active}
      data-card-family={family}
    >
      <VirtualCardGrid {...shown.current} />
    </div>
  );
});
