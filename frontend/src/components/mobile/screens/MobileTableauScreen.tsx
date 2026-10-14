import { useMemo, useRef, useState, type ReactNode } from "react";
import { ResourceTypeCredit, type CardDto } from "@/types/generated/api-types.ts";
import { useMobileUiStore } from "@/stores/mobileUiStore.ts";
import MobileScreen from "../MobileScreen.tsx";
import { TableauIcon } from "../screenIcons.tsx";
import MobileCardDetail from "../MobileCardDetail.tsx";
import { useMobileGame } from "../MobileGameContext.tsx";
import GameIcon from "../../ui/display/GameIcon.tsx";
import { useElementSize } from "@/hooks/useElementSize.ts";
import GameCard from "../../ui/cards/GameCard.tsx";
import FittedCard from "../../ui/cards/FittedCard.tsx";
import { filterPlayedCards, NoPlayedCards } from "../../ui/cards/PlayedCardsGrid.tsx";
import EffectsList from "../../ui/popover/content/EffectsList.tsx";
import TagsSummary, {
  countPlayerTags,
  summarizeTags,
} from "../../ui/popover/content/TagsSummary.tsx";
import StoragesList, { useStorageItems } from "../../ui/popover/content/StoragesList.tsx";
import VictoryPointsBreakdown, {
  totalVictoryPoints,
} from "../../ui/popover/content/VictoryPointsBreakdown.tsx";
import TableauOverview from "../tableau/TableauOverview.tsx";
import TableauPlayerRail from "../tableau/TableauPlayerRail.tsx";
import { playerColor, playersInTurnOrder, type TableauPlayer } from "../tableau/tableauPlayers.ts";

type TableauTab = "overview" | "played" | "effects" | "tags" | "storages" | "vp";

const TAB_IDS: TableauTab[] = ["overview", "played", "effects", "tags", "storages", "vp"];

function isTableauTab(id: string): id is TableauTab {
  return (TAB_IDS as string[]).includes(id);
}

const PLAYED_CARD_WIDTH = 200;
const PLAYED_ROW_PADDING = 8;
const PLAYED_CARD_MAX_SCALE = 1.5;

function PlayedCardsRow({
  cards,
  onCardTap,
}: {
  cards: CardDto[];
  onCardTap: (card: CardDto) => void;
}) {
  const rowRef = useRef<HTMLDivElement>(null);
  const { height } = useElementSize(rowRef);
  const cardHeight = Math.max(0, height - 2 * PLAYED_ROW_PADDING);

  return (
    <div
      ref={rowRef}
      className="flex-1 min-h-0 flex items-center justify-center-safe gap-2 overflow-x-auto overflow-y-hidden overscroll-contain"
      style={{ padding: PLAYED_ROW_PADDING }}
    >
      {cards.length === 0 && <NoPlayedCards />}
      {cardHeight > 0 &&
        cards.map((card) => (
          <FittedCard
            key={card.id}
            naturalWidth={PLAYED_CARD_WIDTH}
            height={cardHeight}
            maxScale={PLAYED_CARD_MAX_SCALE}
          >
            <div
              role="button"
              tabIndex={0}
              aria-label={card.name}
              className="cursor-pointer focus-visible:outline focus-visible:outline-white/60"
              onClick={() => onCardTap(card)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  onCardTap(card);
                }
              }}
            >
              <div className="pointer-events-none">
                <GameCard card={card} isSelected={false} />
              </div>
            </div>
          </FittedCard>
        ))}
    </div>
  );
}

export function PlayedCardsTab({
  cards,
  onCardTap,
}: {
  cards: CardDto[];
  onCardTap: (card: CardDto) => void;
}) {
  const [searchQuery, setSearchQuery] = useState("");
  const totalCost = cards.reduce((sum, card) => sum + card.cost, 0);

  return (
    <div className="h-full flex flex-col">
      <div className="shrink-0 flex items-center gap-3 h-11 px-3 border-b border-white/10">
        <span className="font-orbitron text-[13px] text-white/80 tabular-nums">
          {cards.length} played
        </span>
        <GameIcon iconType={ResourceTypeCredit} amount={totalCost} size="small" />
        <input
          type="text"
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
          placeholder="Search cards..."
          aria-label="Search played cards"
          spellCheck={false}
          autoComplete="off"
          className="ml-auto w-[220px] min-w-0 h-9 bg-black/50 border border-white/25 rounded-none text-white px-3 text-[16px] placeholder:text-white/40 outline-none focus:border-white/50 cursor-text"
        />
      </div>
      <PlayedCardsRow cards={filterPlayedCards(cards, searchQuery)} onCardTap={onCardTap} />
    </div>
  );
}

function resolveShownPlayer(
  players: TableauPlayer[],
  tableauPlayerId: string | null,
  selfId: string | null,
): TableauPlayer | null {
  const requested = tableauPlayerId ? players.find((p) => p.id === tableauPlayerId) : undefined;
  if (requested) {
    return requested;
  }
  const self = selfId ? players.find((p) => p.id === selfId) : undefined;
  return self ?? players[0] ?? null;
}

export default function MobileTableauScreen() {
  const close = useMobileUiStore((s) => s.close);
  const tableauPlayerId = useMobileUiStore((s) => s.tableauPlayerId);
  const { gameState, currentPlayer, isSpectator, playerColorMap } = useMobileGame();
  const [activeTab, setActiveTab] = useState<TableauTab>("overview");
  const [detailCard, setDetailCard] = useState<CardDto | null>(null);

  const players = useMemo(() => playersInTurnOrder(gameState), [gameState]);
  const selfId = isSpectator ? null : (currentPlayer?.id ?? gameState.viewingPlayerId ?? null);
  const shown = resolveShownPlayer(players, tableauPlayerId, selfId);

  const playedCards = shown?.playedCards ?? [];
  const tagCounts = useMemo(
    () => countPlayerTags(shown?.corporation, playedCards),
    [shown?.corporation, playedCards],
  );
  const storageItems = useStorageItems(shown?.resourceStorage, activeTab === "storages");

  if (!shown) {
    return (
      <MobileScreen icon={TableauIcon} title="Tableau" onClose={close}>
        <div className="h-full flex items-center justify-center p-6 text-[13px] text-white/50">
          No players
        </div>
      </MobileScreen>
    );
  }

  const isMe = shown.id === selfId;
  const color = playerColor(shown, playerColorMap);
  const vpGranters = shown.vpGranters ?? [];
  const totalVP = totalVictoryPoints(vpGranters);
  const shownIndex = players.findIndex((p) => p.id === shown.id);

  const switchBy = (offset: number) => {
    if (players.length === 0) {
      return;
    }
    const nextIndex = (shownIndex + offset + players.length) % players.length;
    useMobileUiStore.getState().open("tableau", { tableauPlayerId: players[nextIndex].id });
  };

  const tabs = [
    { id: "overview", label: "Overview" },
    { id: "played", label: "Played", badge: playedCards.length },
    { id: "effects", label: "Effects", badge: shown.effects?.length ?? 0 },
    { id: "tags", label: "Tags", badge: summarizeTags(tagCounts).totalTags },
    { id: "storages", label: "Storages", badge: Object.keys(shown.resourceStorage ?? {}).length },
    { id: "vp", label: "VP", badge: totalVP },
  ];

  let body: ReactNode;
  switch (activeTab) {
    case "overview":
      body = <TableauOverview player={shown} totalVP={totalVP} onCardTap={setDetailCard} />;
      break;
    case "played":
      body = <PlayedCardsTab cards={playedCards} onCardTap={setDetailCard} />;
      break;
    case "effects":
      body = <EffectsList effects={shown.effects ?? []} density="screen" />;
      break;
    case "tags":
      body = <TagsSummary tagCounts={tagCounts} density="screen" />;
      break;
    case "storages":
      body = <StoragesList items={storageItems} density="screen" />;
      break;
    case "vp":
      body = <VictoryPointsBreakdown vpGranters={vpGranters} density="screen" />;
      break;
    default: {
      const unreachable: never = activeTab;
      body = unreachable;
    }
  }

  return (
    <>
      <MobileScreen
        icon={TableauIcon}
        title={isMe ? "Your tableau" : `${shown.name}'s tableau`}
        tabs={tabs}
        activeTab={activeTab}
        onTabChange={(id) => {
          if (isTableauTab(id)) {
            setActiveTab(id);
          }
        }}
        onClose={close}
      >
        <div className="h-full flex">
          <TableauPlayerRail
            color={color}
            playerName={isMe ? "You" : shown.name}
            canSwitch={players.length > 1}
            onPrevious={() => switchBy(-1)}
            onNext={() => switchBy(1)}
          />
          <div
            className={`flex-1 min-w-0 h-full ${activeTab === "overview" || activeTab === "played" ? "overflow-hidden" : "overflow-y-auto overscroll-contain"}`}
          >
            {body}
          </div>
        </div>
      </MobileScreen>
      {detailCard && <MobileCardDetail card={detailCard} onClose={() => setDetailCard(null)} />}
    </>
  );
}
