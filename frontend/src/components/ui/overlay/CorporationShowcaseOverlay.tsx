import { useOverlayPortalRoot, useIsolatedOverlay } from "@/contexts/OverlayEnvironment";
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import type { CardDto, GameDto, InitPhaseDto } from "@/types/generated/api-types.ts";
import { CardTypeCorporation, GamePhaseInitApplyPrelude } from "@/types/generated/api-types.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useReducedMotion } from "@/hooks/useReducedMotion.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { useElementSize } from "@/hooks/useElementSize.ts";
import { audioService } from "@/services/audioService.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { getCorporationBorderColor } from "@/utils/corporationColors.ts";
import { getCorporationLogo } from "@/utils/corporationLogos.tsx";
import { CARD_RELEASE_DURATION_MS, playCardRelease } from "@/utils/cardReleaseAnimation.ts";
import { SHOWCASE_FADE_MS } from "@/constants/gameConstants.ts";
import { useShowcaseTileHold } from "@/hooks/useShowcaseTileHold.ts";
import {
  corpShown,
  isShowcaseStepEnd,
  showcaseControllerId,
  showcasePlayers,
  shownPreludes,
  type ShowcasePlayer,
} from "@/utils/showcase.ts";
import CorporationCard from "../cards/CorporationCard.tsx";
import GameCard from "../cards/GameCard.tsx";
import GameButton from "../buttons/GameButton.tsx";
import FittedCard from "../cards/FittedCard.tsx";
import MobileCardDetail from "../../mobile/MobileCardDetail.tsx";
import { CurrentGameMenuButton } from "../buttons/MainMenuHamburger.tsx";

type Viewport = { width: number; height: number };

const CORP_CARD_WIDTH = 400;
/** Typical rendered heights, used only to pick a scale; boxes are sized from the real height. */
const CORP_CARD_HEIGHT = 400;
const CORP_MAX_SCALE = 1.2;
const PRELUDE_CARD_WIDTH = 360;
const PRELUDE_CARD_HEIGHT = 500;
const PRELUDE_GAP = 40;
const PRELUDE_MAX_SCALE = 0.8;
/** Summary preludes are drawn at this fraction of the corp card's scale, two fitting under it. */
const SUMMARY_PRELUDE_RATIO = 0.5;
const SUMMARY_GAP = 16;
const NAME_SPACE = 80;
const BOTTOM_BAR_SPACE = 96;
const ROSTER_COLUMN_GAP = 32;

const noop = () => {};

interface Props {
  game: GameDto;
  initPhase: InitPhaseDto;
}

export default function CorporationShowcaseOverlay(props: Props) {
  return (
    <CorporationShowcaseOverlayView
      {...props}
      onAdvance={() => {
        void globalWebSocketManager.confirmInitAdvance();
      }}
    />
  );
}

export function CorporationShowcaseOverlayView({
  game,
  initPhase,
  onAdvance,
}: Props & { onAdvance: () => void }) {
  const { isCompact } = useLayoutMode();
  const reducedMotion = useReducedMotion();
  const [viewport, setViewport] = useState<Viewport>({
    width: window.innerWidth,
    height: window.innerHeight,
  });
  const [viewingPlayerId, setViewingPlayerId] = useState<string | null>(null);
  const [sentVersion, setSentVersion] = useState<number | null>(null);

  useEffect(() => {
    const resize = () => setViewport({ width: window.innerWidth, height: window.innerHeight });
    window.addEventListener("resize", resize);
    return () => window.removeEventListener("resize", resize);
  }, []);

  const players = showcasePlayers(game);
  const isPreludePhase = game.currentPhase === GamePhaseInitApplyPrelude;
  const isRoster = initPhase.stage === "roster";
  const livePlayer = players.find((p) => p.id === initPhase.currentPlayerId) ?? null;
  const liveIndex = livePlayer ? players.indexOf(livePlayer) : -1;
  const pending = initPhase.hasPendingSelection;
  const holdingAfterChoice = useShowcaseTileHold(pending);
  const faded = pending || holdingAfterChoice;
  const animate = !reducedMotion;

  const viewerId = game.isSpectator ? null : (game.currentPlayer?.id ?? null);
  const isController = viewerId !== null && showcaseControllerId(game) === viewerId;
  const stepEnd = isShowcaseStepEnd(game) && !faded;
  const nextSent = sentVersion === initPhase.confirmVersion;
  const viewingPlayer = players.find((p) => p.id === viewingPlayerId) ?? null;

  const handleNext = () => {
    setViewingPlayerId(null);
    setSentVersion(initPhase.confirmVersion);
    onAdvance();
  };

  if (isCompact) {
    return (
      <CompactShowcase
        game={game}
        initPhase={initPhase}
        players={players}
        livePlayer={livePlayer}
        viewingPlayer={viewingPlayer}
        onViewPlayer={setViewingPlayerId}
        isPreludePhase={isPreludePhase}
        isRoster={isRoster}
        faded={faded}
        animate={animate}
        isController={isController}
        stepEnd={stepEnd}
        nextSent={nextSent}
        onNext={handleNext}
      />
    );
  }

  let content: ReactNode = null;
  if (viewingPlayer) {
    const index = players.indexOf(viewingPlayer);
    content = (
      <PlayerSummary
        key={`history:${viewingPlayer.id}`}
        player={viewingPlayer}
        corporation={corpShown(game, index) ? viewingPlayer.corporation : undefined}
        preludes={shownPreludes(viewingPlayer)}
        scale={summaryScale(viewport)}
      />
    );
  } else if (isRoster) {
    content = <RosterSlide players={players} viewport={viewport} />;
  } else if (livePlayer && isPreludePhase) {
    content = (
      <PreludeSlide
        key={`prelude:${livePlayer.id}`}
        player={livePlayer}
        preludes={initPhase.preludes}
        played={initPhase.preludesPlayed}
        viewport={viewport}
        animate={animate}
      />
    );
  } else if (livePlayer?.corporation) {
    content = (
      <CorpSlide
        key={`corp:${livePlayer.id}`}
        player={livePlayer}
        corporation={livePlayer.corporation}
        played={initPhase.stage === "applied"}
        viewport={viewport}
        animate={animate}
      />
    );
  }

  return (
    <div
      className={`fixed inset-0 flex flex-col transition-opacity ease-out ${faded ? "pointer-events-none opacity-0" : "pointer-events-auto opacity-100"}`}
      style={{
        zIndex: Z_INDEX.SHOWCASE,
        transitionDuration: `${SHOWCASE_FADE_MS}ms`,
        // Let the card that caused the choice finish playing before the board takes over.
        transitionDelay: faded ? `${CARD_RELEASE_DURATION_MS}ms` : "0ms",
      }}
      role="dialog"
      aria-label="Corporation showcase"
      aria-hidden={faded}
    >
      <style>{SHOWCASE_KEYFRAMES}</style>
      <div className="absolute inset-0 bg-black/65 backdrop-blur-sm animate-[showcaseFadeIn_400ms_ease-out_both]" />
      <main className="relative flex min-h-0 flex-1 items-center justify-center px-4">
        {content}
      </main>
      <footer className="relative flex items-center gap-3 px-8 pb-6 max-md:px-4">
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
          {players.map((p, i) => {
            const shown = corpShown(game, i);
            const isLive = !isRoster && i === liveIndex;
            const isViewed = viewingPlayerId === p.id;
            return (
              <button
                key={p.id}
                type="button"
                disabled={!shown}
                onClick={() => setViewingPlayerId(p.id)}
                className={`flex items-center border px-3 py-1.5 font-orbitron text-[11px] uppercase tracking-[0.1em] transition-colors duration-300 ${shown ? "cursor-pointer hover:border-white/50" : "cursor-default opacity-50"} ${isViewed ? "border-white bg-white/15" : ""} ${!isViewed && isLive ? "border-white/60 bg-white/10" : ""} ${!isViewed && !isLive ? "border-white/15 bg-black/40" : ""}`}
              >
                <span className="h-2 w-2 shrink-0" style={{ background: p.color }} />
                <span className="ml-2 text-white/90">{p.name}</span>
                <span
                  className={`grid transition-[grid-template-columns,opacity] duration-500 ease-out ${shown && p.corporation ? "grid-cols-[1fr] opacity-100" : "grid-cols-[0fr] opacity-0"}`}
                >
                  <span className="overflow-hidden whitespace-nowrap text-white/60">
                    <span className="px-2 text-white/40">·</span>
                    {p.corporation?.name}
                  </span>
                </span>
              </button>
            );
          })}
          {viewingPlayer && (
            <GameButton size="sm" emphasis="quiet" onClick={() => setViewingPlayerId(null)}>
              Follow host
            </GameButton>
          )}
        </div>
        {isController ? (
          <GameButton size="sm" disabled={!stepEnd || nextSent} onClick={handleNext}>
            {isRoster ? "Start game" : "Next"}
          </GameButton>
        ) : (
          <span
            className={`font-orbitron text-xs uppercase tracking-[0.15em] text-white/60 transition-opacity duration-300 ${stepEnd ? "opacity-100" : "opacity-0"}`}
          >
            Waiting for host…
          </span>
        )}
      </footer>
    </div>
  );
}

function availableHeight(viewport: Viewport): number {
  return viewport.height - NAME_SPACE - BOTTOM_BAR_SPACE - 48;
}

function summaryScale(viewport: Viewport): number {
  const columnHeight = CORP_CARD_HEIGHT + PRELUDE_CARD_HEIGHT * SUMMARY_PRELUDE_RATIO + SUMMARY_GAP;
  return Math.min(
    CORP_MAX_SCALE,
    availableHeight(viewport) / columnHeight,
    (viewport.width - 64) / CORP_CARD_WIDTH,
  );
}

type CardPlayState = "waiting" | "releasing" | "gone";

/**
 * Plays a showcase card once `played` turns true: the normal card-play release animation
 * and sound, after which the card stays gone. A card already played on mount is gone.
 */
function useCardPlay(played: boolean, animate: boolean) {
  const isolated = useIsolatedOverlay();
  const ref = useRef<HTMLDivElement | null>(null);
  const handled = useRef(played);
  const [state, setState] = useState<CardPlayState>(played ? "gone" : "waiting");

  useLayoutEffect(() => {
    if (!played || handled.current) {
      return;
    }
    handled.current = true;
    if (!isolated) {
      void audioService.playSound("card-played");
    }
    const element = ref.current;
    if (!element || !animate) {
      setState("gone");
      return;
    }
    setState("releasing");
    const animation = playCardRelease(element);
    animation.addEventListener("finish", () => setState("gone"));
  }, [played, animate, isolated]);

  return { ref, state };
}

const FLIP_DELAY_MS = 150;
const FLIP_STAGGER_MS = 200;

const FLIP_CLASS = "animate-[showcaseFlipIn_700ms_cubic-bezier(0.2,0.8,0.2,1)_both]";

function CorpSlide({
  player,
  corporation,
  played,
  viewport,
  animate,
}: {
  player: ShowcasePlayer;
  corporation: CardDto;
  played: boolean;
  viewport: Viewport;
  animate: boolean;
}) {
  const { ref, state } = useCardPlay(played, animate);
  const scale = Math.min(
    CORP_MAX_SCALE,
    (viewport.width - 64) / CORP_CARD_WIDTH,
    availableHeight(viewport) / CORP_CARD_HEIGHT,
  );

  return (
    <div className="flex flex-col items-center gap-6">
      <PlayerName player={player} />
      <div
        className={animate ? FLIP_CLASS : ""}
        style={{ animationDelay: animate ? `${FLIP_DELAY_MS}ms` : undefined }}
      >
        <div ref={ref} style={{ visibility: state === "gone" ? "hidden" : "visible" }}>
          <FittedCard naturalWidth={CORP_CARD_WIDTH} maxScale={scale}>
            <CorpCard card={corporation} />
          </FittedCard>
        </div>
      </div>
    </div>
  );
}

function PreludeSlide({
  player,
  preludes,
  played,
  viewport,
  animate,
}: {
  player: ShowcasePlayer;
  preludes: CardDto[];
  played: number;
  viewport: Viewport;
  animate: boolean;
}) {
  const count = Math.max(1, preludes.length);
  const rowWidth = count * PRELUDE_CARD_WIDTH + (count - 1) * PRELUDE_GAP;
  const scale = Math.min(
    PRELUDE_MAX_SCALE,
    (viewport.width - 64) / rowWidth,
    availableHeight(viewport) / PRELUDE_CARD_HEIGHT,
  );

  return (
    <div className="flex flex-col items-center gap-6">
      <PlayerName player={player} />
      <div className="flex items-start" style={{ gap: PRELUDE_GAP * scale }}>
        {preludes.map((card, i) => (
          <PreludeCard
            key={card.id}
            card={card}
            played={i < played}
            index={i}
            scale={scale}
            animate={animate}
          />
        ))}
      </div>
    </div>
  );
}

function PreludeCard({
  card,
  played,
  index,
  scale,
  animate,
}: {
  card: CardDto;
  played: boolean;
  index: number;
  scale: number;
  animate: boolean;
}) {
  const { ref, state } = useCardPlay(played, animate);
  const flipDelay = FLIP_DELAY_MS + index * FLIP_STAGGER_MS;
  return (
    <div
      className={animate ? FLIP_CLASS : ""}
      style={{ animationDelay: animate ? `${flipDelay}ms` : undefined }}
    >
      <div ref={ref} style={{ visibility: state === "gone" ? "hidden" : "visible" }}>
        <FittedCard naturalWidth={PRELUDE_CARD_WIDTH} maxScale={scale}>
          <GameCard
            card={card}
            presentation="inspection"
            moduleState={state === "releasing" ? "releasing" : "idle"}
          />
        </FittedCard>
      </div>
    </div>
  );
}

/** A player's shown cards for looking back: the corporation on top, played preludes below. */
function PlayerSummary({
  player,
  corporation,
  preludes,
  scale,
}: {
  player: ShowcasePlayer;
  corporation: CardDto | undefined;
  preludes: CardDto[];
  scale: number;
}) {
  const preludeScale = scale * SUMMARY_PRELUDE_RATIO;
  const preludeGap = CORP_CARD_WIDTH * scale - 2 * PRELUDE_CARD_WIDTH * preludeScale;
  return (
    <div className="flex flex-col items-center gap-4 animate-[showcaseFadeIn_400ms_ease-out_both]">
      <PlayerName player={player} />
      <div className="flex flex-col items-center" style={{ gap: SUMMARY_GAP }}>
        {corporation && (
          <FittedCard naturalWidth={CORP_CARD_WIDTH} maxScale={scale}>
            <CorpCard card={corporation} />
          </FittedCard>
        )}
        {preludes.length > 0 && (
          <div className="flex items-start justify-center" style={{ gap: Math.max(8, preludeGap) }}>
            {preludes.map((card) => (
              <FittedCard key={card.id} naturalWidth={PRELUDE_CARD_WIDTH} maxScale={preludeScale}>
                <GameCard card={card} presentation="inspection" />
              </FittedCard>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function CorpCard({ card }: { card: CardDto }) {
  return (
    <CorporationCard
      card={card}
      isSelected={false}
      onSelect={noop}
      disableInteraction={true}
      borderColor={getCorporationBorderColor(card.name)}
    />
  );
}

function PlayerName({
  player,
  size = "large",
}: {
  player: ShowcasePlayer;
  size?: "large" | "small";
}) {
  const sizeClass = size === "large" ? "text-3xl max-md:text-xl" : "text-lg";
  return (
    <span
      className={`font-orbitron font-bold uppercase tracking-[0.15em] [text-shadow:0_2px_8px_rgba(0,0,0,0.8)] animate-[showcaseFadeIn_400ms_ease-out_both] ${sizeClass}`}
      style={{ color: player.color }}
    >
      {player.name}
    </span>
  );
}

/** The final step: every player's corporation side by side. */
function RosterSlide({ players, viewport }: { players: ShowcasePlayer[]; viewport: Viewport }) {
  const withCorp = players.filter(
    (p): p is ShowcasePlayer & { corporation: CardDto } => p.corporation !== undefined,
  );
  const count = Math.max(1, withCorp.length);
  const scale = Math.min(
    0.9,
    availableHeight(viewport) / CORP_CARD_HEIGHT,
    (viewport.width - 64 - (count - 1) * ROSTER_COLUMN_GAP) / (count * CORP_CARD_WIDTH),
  );
  return (
    <div className="flex items-start justify-center" style={{ gap: ROSTER_COLUMN_GAP }}>
      {withCorp.map((p, i) => (
        <div
          key={p.id}
          className="flex flex-col items-center gap-3 animate-[showcaseFadeIn_500ms_ease-out_both]"
          style={{ animationDelay: `${i * 120}ms` }}
        >
          <PlayerName player={p} size="small" />
          <FittedCard naturalWidth={CORP_CARD_WIDTH} maxScale={scale}>
            <CorpCard card={p.corporation} />
          </FittedCard>
        </div>
      ))}
    </div>
  );
}

const ROSTER_SLOT = "roster";
const SWIPE_THRESHOLD_PX = 48;
const COMPACT_MAX_SCALE = 1;
const COMPACT_CARD_WIDTH = 200;
const COMPACT_CORP_GAP = 32;
const COMPACT_PRELUDE_GAP = 20;

interface CompactShowcaseProps {
  game: GameDto;
  initPhase: InitPhaseDto;
  players: ShowcasePlayer[];
  livePlayer: ShowcasePlayer | null;
  viewingPlayer: ShowcasePlayer | null;
  onViewPlayer: (playerId: string | null) => void;
  isPreludePhase: boolean;
  isRoster: boolean;
  faded: boolean;
  animate: boolean;
  isController: boolean;
  stepEnd: boolean;
  nextSent: boolean;
  onNext: () => void;
}

/**
 * Phone presentation: one player at a time between the top bar, the player rail and a footer
 * in the dock's place. Chevrons and swipes look back at players the server has already shown;
 * stepping onto the live slot follows the host again.
 */
function CompactShowcase({
  game,
  initPhase,
  players,
  livePlayer,
  viewingPlayer,
  onViewPlayer,
  isPreludePhase,
  isRoster,
  faded,
  animate,
  isController,
  stepEnd,
  nextSent,
  onNext,
}: CompactShowcaseProps) {
  const portalRoot = useOverlayPortalRoot();
  const [detailCard, setDetailCard] = useState<CardDto | null>(null);
  const swipeRef = useRef<{ x: number; y: number } | null>(null);
  const swipedRef = useRef(false);

  const liveSlot = isRoster ? ROSTER_SLOT : (livePlayer?.id ?? null);
  const slots = players.filter((_, i) => corpShown(game, i)).map((p) => p.id);
  if (isRoster) {
    slots.push(ROSTER_SLOT);
  }
  const currentSlot = viewingPlayer?.id ?? liveSlot;
  const position = currentSlot ? slots.indexOf(currentSlot) : -1;
  const prevSlot = position > 0 ? slots[position - 1] : null;
  const nextSlot = position >= 0 && position < slots.length - 1 ? slots[position + 1] : null;

  const goTo = (slot: string | null) => {
    if (!slot) {
      return;
    }
    onViewPlayer(slot === liveSlot ? null : slot);
  };

  const handlePointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    swipeRef.current = { x: event.clientX, y: event.clientY };
    swipedRef.current = false;
  };
  const handlePointerUp = (event: ReactPointerEvent<HTMLElement>) => {
    const start = swipeRef.current;
    swipeRef.current = null;
    if (!start) {
      return;
    }
    const dx = event.clientX - start.x;
    const dy = event.clientY - start.y;
    if (Math.abs(dx) < SWIPE_THRESHOLD_PX || Math.abs(dx) < Math.abs(dy)) {
      return;
    }
    swipedRef.current = true;
    goTo(dx > 0 ? prevSlot : nextSlot);
  };
  const inspect = (card: CardDto) => {
    if (swipedRef.current) {
      swipedRef.current = false;
      return;
    }
    setDetailCard(card);
  };

  let content: ReactNode = null;
  if (viewingPlayer) {
    const index = players.indexOf(viewingPlayer);
    content = (
      <CompactPlayerStage
        key={`history:${viewingPlayer.id}`}
        player={viewingPlayer}
        corporationName={corpShown(game, index) ? viewingPlayer.corporation?.name : undefined}
        corporation={corpShown(game, index) ? viewingPlayer.corporation : undefined}
        corporationPlayed={false}
        preludes={shownPreludes(viewingPlayer)}
        preludesPlayed={0}
        animate={false}
        onInspect={inspect}
      />
    );
  } else if (isRoster) {
    content = <CompactRoster players={players} onSelect={(id) => onViewPlayer(id)} />;
  } else if (livePlayer && isPreludePhase) {
    content = (
      <CompactPlayerStage
        key={`prelude:${livePlayer.id}`}
        player={livePlayer}
        corporationName={livePlayer.corporation?.name}
        corporation={undefined}
        corporationPlayed={false}
        preludes={initPhase.preludes}
        preludesPlayed={initPhase.preludesPlayed}
        animate={animate}
        onInspect={inspect}
      />
    );
  } else if (livePlayer?.corporation) {
    content = (
      <CompactPlayerStage
        key={`corp:${livePlayer.id}`}
        player={livePlayer}
        corporationName={livePlayer.corporation.name}
        corporation={livePlayer.corporation}
        corporationPlayed={initPhase.stage === "applied"}
        preludes={[]}
        preludesPlayed={0}
        animate={animate}
        onInspect={inspect}
      />
    );
  }

  const interactive = faded ? "pointer-events-none" : "pointer-events-auto";

  return (
    <div
      className={`fixed inset-0 pointer-events-none transition-opacity ease-out ${faded ? "opacity-0" : "opacity-100"}`}
      style={{
        zIndex: Z_INDEX.SHOWCASE,
        transitionDuration: `${SHOWCASE_FADE_MS}ms`,
        transitionDelay: faded ? `${CARD_RELEASE_DURATION_MS}ms` : "0ms",
      }}
      role="dialog"
      aria-label="Corporation showcase"
      aria-hidden={faded}
    >
      <style>{SHOWCASE_KEYFRAMES}</style>
      <div className={`absolute inset-0 bg-black/65 backdrop-blur-sm ${interactive}`} />
      <div
        className={`absolute h-11 flex items-center ${interactive}`}
        style={{ top: "var(--safe-top)", left: "var(--safe-left)" }}
      >
        <CurrentGameMenuButton />
      </div>
      <main
        className={`absolute left-0 flex touch-pan-y ${interactive}`}
        style={{
          top: "calc(var(--hud-top-h) + var(--safe-top))",
          right: "var(--safe-right)",
          bottom: "calc(var(--hud-dock-h) + var(--safe-bottom))",
          paddingLeft: "var(--safe-left)",
        }}
        onPointerDown={handlePointerDown}
        onPointerUp={handlePointerUp}
        onPointerCancel={() => {
          swipeRef.current = null;
        }}
      >
        <ChevronButton direction="previous" disabled={!prevSlot} onClick={() => goTo(prevSlot)} />
        <div className="flex min-w-0 flex-1 flex-col py-2">{content}</div>
        <ChevronButton direction="next" disabled={!nextSlot} onClick={() => goTo(nextSlot)} />
      </main>
      <footer
        className={`absolute inset-x-0 bottom-0 flex items-center gap-3 border-t border-white/10 bg-[rgba(3,3,4,0.88)] ${interactive}`}
        style={{
          height: "calc(var(--hud-dock-h) + var(--safe-bottom))",
          paddingBottom: "var(--safe-bottom)",
          paddingLeft: "calc(var(--safe-left) + 12px)",
          paddingRight: "calc(var(--safe-right) + 12px)",
        }}
      >
        <div className="flex shrink-0 items-center gap-1.5" aria-hidden="true">
          {slots.map((slot) => (
            <span
              key={slot}
              className={`h-2 w-2 transition-opacity duration-300 ${slot === currentSlot ? "opacity-100" : "opacity-35"}`}
              style={{ background: players.find((p) => p.id === slot)?.color ?? "#ffffff" }}
            />
          ))}
        </div>
        <div className="flex min-w-0 flex-1 items-center">
          {viewingPlayer && (
            <GameButton size="sm" emphasis="quiet" height={44} onClick={() => onViewPlayer(null)}>
              Follow host
            </GameButton>
          )}
        </div>
        {isController ? (
          <GameButton size="sm" height={44} disabled={!stepEnd || nextSent} onClick={onNext}>
            {isRoster ? "Start game" : "Next"}
          </GameButton>
        ) : (
          <span
            className={`font-orbitron text-[11px] uppercase tracking-[0.15em] text-white/60 transition-opacity duration-300 ${stepEnd ? "opacity-100" : "opacity-0"}`}
          >
            Waiting for host…
          </span>
        )}
      </footer>
      {detailCard &&
        !faded &&
        createPortal(
          <MobileCardDetail card={detailCard} onClose={() => setDetailCard(null)} />,
          portalRoot,
        )}
    </div>
  );
}

function ChevronButton({
  direction,
  disabled,
  onClick,
}: {
  direction: "previous" | "next";
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <div className="flex w-11 shrink-0 items-center justify-center">
      <GameButton
        emphasis="quiet"
        width={44}
        height={44}
        disabled={disabled}
        aria-label={direction === "previous" ? "Previous player" : "Next player"}
        className={`!px-0 ${disabled ? "opacity-0" : ""}`}
        onClick={onClick}
      >
        <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path
            d={
              direction === "previous"
                ? "M15.41 7.41 14 6l-6 6 6 6 1.41-1.41L10.83 12z"
                : "M8.59 16.59 10 18l6-6-6-6-1.41 1.41L13.17 12z"
            }
          />
        </svg>
      </GameButton>
    </div>
  );
}

function CompactPlayerStage({
  player,
  corporationName,
  corporation,
  corporationPlayed,
  preludes,
  preludesPlayed,
  animate,
  onInspect,
}: {
  player: ShowcasePlayer;
  corporationName: string | undefined;
  corporation: CardDto | undefined;
  corporationPlayed: boolean;
  preludes: CardDto[];
  preludesPlayed: number;
  animate: boolean;
  onInspect: (card: CardDto) => void;
}) {
  const stageRef = useRef<HTMLDivElement | null>(null);
  const cardsRef = useRef<HTMLDivElement | null>(null);
  const stage = useElementSize(stageRef);
  const isRow = stage.width > stage.height;

  const preludesWidth =
    preludes.length * COMPACT_CARD_WIDTH + Math.max(0, preludes.length - 1) * COMPACT_PRELUDE_GAP;
  let cardsWidth = preludesWidth;
  if (corporation) {
    cardsWidth = CORP_CARD_WIDTH + (preludes.length > 0 ? COMPACT_CORP_GAP + preludesWidth : 0);
  }

  const identityClass = isRow
    ? "w-[30%] max-w-[220px] flex-col items-start justify-center gap-1 pl-2"
    : "max-w-full items-baseline justify-center gap-2";

  return (
    <div
      ref={stageRef}
      className={`flex min-h-0 flex-1 items-center gap-2 ${isRow ? "flex-row" : "flex-col"}`}
    >
      <div className={`flex min-w-0 shrink-0 font-orbitron ${identityClass}`}>
        <PlayerName player={player} size="small" />
        {corporationName && (
          <span className="max-w-full truncate text-[12px] uppercase tracking-[0.1em] text-white/60">
            {corporationName}
          </span>
        )}
      </div>
      <div
        ref={cardsRef}
        className="flex min-h-0 min-w-0 flex-1 items-center justify-center self-stretch"
      >
        {cardsWidth > 0 && (
          <FittedCard naturalWidth={cardsWidth} boundsRef={cardsRef} maxScale={COMPACT_MAX_SCALE}>
            <div className="flex items-center" style={{ gap: COMPACT_CORP_GAP }}>
              {corporation && (
                <CompactShowcaseCard
                  card={corporation}
                  played={corporationPlayed}
                  index={0}
                  animate={animate}
                  onInspect={onInspect}
                />
              )}
              {preludes.length > 0 && (
                <div className="flex items-center" style={{ gap: COMPACT_PRELUDE_GAP }}>
                  {preludes.map((card, i) => (
                    <CompactShowcaseCard
                      key={card.id}
                      card={card}
                      played={i < preludesPlayed}
                      index={i + 1}
                      animate={animate}
                      onInspect={onInspect}
                    />
                  ))}
                </div>
              )}
            </div>
          </FittedCard>
        )}
      </div>
    </div>
  );
}

function CompactShowcaseCard({
  card,
  played,
  index,
  animate,
  onInspect,
}: {
  card: CardDto;
  played: boolean;
  index: number;
  animate: boolean;
  onInspect: (card: CardDto) => void;
}) {
  const { ref, state } = useCardPlay(played, animate);
  const isCorporation = card.type === CardTypeCorporation;
  const flipDelay = FLIP_DELAY_MS + index * FLIP_STAGGER_MS;

  return (
    <div
      className={animate ? FLIP_CLASS : ""}
      style={{ animationDelay: animate ? `${flipDelay}ms` : undefined }}
    >
      <div ref={ref} style={{ visibility: state === "gone" ? "hidden" : "visible" }}>
        <button
          type="button"
          className="block cursor-pointer text-left"
          aria-label={`Show ${card.name}`}
          onClick={() => onInspect(card)}
        >
          {isCorporation ? (
            <CorpCard card={card} />
          ) : (
            <div style={{ width: COMPACT_CARD_WIDTH }}>
              <GameCard
                card={card}
                moduleState={state === "releasing" ? "releasing" : "idle"}
                dimUnavailable={false}
              />
            </div>
          )}
        </button>
      </div>
    </div>
  );
}

function CompactRoster({
  players,
  onSelect,
}: {
  players: ShowcasePlayer[];
  onSelect: (playerId: string) => void;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col items-center justify-center">
      <ul className="m-0 flex max-h-full w-full max-w-[560px] list-none flex-col gap-1.5 overflow-y-auto overscroll-contain p-0">
        {players.map((p, i) => (
          <li
            key={p.id}
            className="animate-[showcaseFadeIn_500ms_ease-out_both]"
            style={{ animationDelay: `${i * 120}ms` }}
          >
            <button
              type="button"
              className="flex min-h-[52px] w-full cursor-pointer items-center gap-3 border border-white/15 bg-black/40 px-3 py-1.5 text-left"
              onClick={() => onSelect(p.id)}
            >
              <span className="h-3 w-3 shrink-0" style={{ background: p.color }} />
              <span
                className="w-[30%] min-w-0 shrink-0 truncate font-orbitron text-[13px] font-bold uppercase tracking-[0.1em]"
                style={{ color: p.color }}
              >
                {p.name}
              </span>
              {p.corporation && (
                <span className="flex min-w-0 flex-1 items-center gap-3">
                  {getCorporationLogo(p.corporation.name, "h-[36px] w-[72px] shrink-0", "72px")}
                  <span className="truncate font-orbitron text-[12px] uppercase tracking-[0.08em] text-white/80">
                    {p.corporation.name}
                  </span>
                </span>
              )}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

const SHOWCASE_KEYFRAMES = `
  @keyframes showcaseFadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  @keyframes showcaseFlipIn {
    from { opacity: 0; transform: perspective(1200px) rotateY(80deg) scale(0.9); }
    to { opacity: 1; transform: perspective(1200px) rotateY(0deg) scale(1); }
  }
`;
