import CardPlayPresentation from "../../ui/overlay/CardPlayPresentation";
import { useState, useCallback, forwardRef } from "react";
import { useStore } from "zustand";
import LeftSidebar from "../panels/LeftSidebar.tsx";
import type { PlayerListHandle } from "../../ui/list/PlayerList.tsx";
import TopMenuBar from "../panels/TopMenuBar.tsx";
import { usePlanetFocus } from "../../../contexts/PlanetFocusContext.tsx";
import RightSidebar from "../panels/RightSidebar.tsx";
import MainContentDisplay from "../../ui/display/MainContentDisplay.tsx";

import BottomResourceBar, {
  BottomResourceBarCallbacks,
} from "../../ui/overlay/BottomResourceBar.tsx";
import PlayedCardNotificationOverlay from "../../ui/overlay/PlayedCardNotification.tsx";
import type { PlayedCardNotification } from "@/hooks/usePlayedCardNotification.ts";
import { StandardProject } from "../../../types/cards.tsx";
import {
  ChatMessageDto,
  GameDto,
  GamePhaseComplete,
  GamePhaseInitApplyCorp,
  GamePhaseInitApplyPrelude,
  PlayerDto,
  OtherPlayerDto,
  CardDto,
  TriggeredEffectDto,
} from "../../../types/generated/api-types.ts";
import { useConfirmDialogStore } from "@/stores/confirmDialogStore.ts";
import { useAppPhaseStore } from "@/stores/appPhaseStore.ts";
import GameConfirmDialogs from "./GameConfirmDialogs.tsx";
import type { CardInspectionStore, CardInspectionDrag } from "@/hooks/useCardInspection.ts";
import CardInspection from "../../ui/overlay/CardInspection.tsx";
import type { PlayerCardDto } from "@/types/generated/api-types.ts";
import ChatOverlay from "../../ui/overlay/ChatOverlay.tsx";

export function SolarSystemFade({ children }: { children: React.ReactNode }) {
  const { activePlanet } = usePlanetFocus();
  if (activePlanet === "solar-system") {
    return null;
  }
  return <>{children}</>;
}

export interface GameLayoutProps {
  gameState: GameDto;
  inspectionStore: CardInspectionStore;
  inspectionHand: PlayerCardDto[];
  inspectionBlocked: boolean;
  onInspectionDrag: (cardId: string, drag: CardInspectionDrag, detail: HTMLElement) => void;
  currentPlayer: PlayerDto | null;
  playedCards?: CardDto[];
  corporationCard?: CardDto | null;
  initTurnPlayerId?: string | null;
  showStartingSelection?: boolean;
  animateHexEntrance?: boolean;
  startDark?: boolean;
  tilesHidden?: boolean;
  changedPaths?: Set<string>;
  triggeredEffects?: TriggeredEffectDto[];
  bottomBarCallbacks?: BottomResourceBarCallbacks;
  onStandardProjectSelect?: (project: StandardProject) => void;
  onLeaveGame?: () => void;
  onEndGame?: () => void;
  onSkyboxReady?: () => void;
  onGpuReady?: () => void;
  onPlayerClick?: (player: PlayerDto | OtherPlayerDto) => void;
  spectatingPlayer?: PlayerDto | OtherPlayerDto | null;
  spectatingCorporation?: CardDto | null;
  spectatePlayerColor?: string;
  onStopSpectating?: () => void;
  isGameSpectator?: boolean;
  chatMessages?: ChatMessageDto[];
  onSendChatMessage?: (message: string) => void;
  playerColorMap?: Map<string, string>;
  endgameFadeUI?: boolean;
  isEndgame?: boolean;
  activeEndgamePanel?: "score" | "graphs" | "replay";
  onEndgamePanelChange?: (panel: "score" | "graphs" | "replay") => void;
  hasHistory?: boolean;
  playedCardNotification?: PlayedCardNotification | null;
  isPlayedCardPinned?: boolean;
  onPlayedCardTogglePin?: () => void;
  onPlayedCardAdvance?: () => void;
}

export function CardInspections({
  store,
  hand,
  blocked,
  chatBounds,
  onDrag,
}: {
  store: CardInspectionStore;
  hand: PlayerCardDto[];
  blocked: boolean;
  chatBounds: DOMRectReadOnly | null;
  onDrag: GameLayoutProps["onInspectionDrag"];
}) {
  const inspections = useStore(store, (state) => state.inspections);
  const { finishInspection, closeInspection } = store.getState();
  if (blocked) {
    return null;
  }
  return inspections.map((inspection) => {
    const card = hand.find((candidate) => candidate.id === inspection.cardId);
    return card ? (
      <CardInspection
        key={card.id}
        card={card}
        inspection={inspection}
        onReturned={finishInspection}
        chatBounds={chatBounds}
        onClose={closeInspection}
        onDragStart={onDrag}
      />
    ) : null;
  });
}

function WithoutInspection({
  store,
  children,
}: {
  store: CardInspectionStore;
  children: React.ReactNode;
}) {
  const inspecting = useStore(store, (state) => state.inspections.length > 0);
  return inspecting ? null : children;
}

const GameLayout = forwardRef<PlayerListHandle, GameLayoutProps>(function GameLayout(
  {
    gameState,
    inspectionStore,
    inspectionHand,
    inspectionBlocked,
    onInspectionDrag,
    currentPlayer,
    playedCards = [],
    corporationCard = null,
    initTurnPlayerId = null,
    showStartingSelection = false,
    animateHexEntrance = false,
    startDark = false,
    tilesHidden = false,
    changedPaths = new Set(),
    triggeredEffects = [],
    bottomBarCallbacks,
    onStandardProjectSelect,
    onLeaveGame,
    onEndGame,
    onSkyboxReady,
    onGpuReady,
    onPlayerClick,
    spectatingPlayer,
    spectatingCorporation,
    spectatePlayerColor,
    onStopSpectating,
    isGameSpectator = false,
    chatMessages,
    onSendChatMessage,
    playerColorMap,
    endgameFadeUI = false,
    isEndgame = false,
    activeEndgamePanel,
    onEndgamePanelChange,
    hasHistory = false,
    playedCardNotification,
    isPlayedCardPinned,
    onPlayedCardTogglePin,
    onPlayedCardAdvance,
  },
  ref,
) {
  const [chatBounds, setChatBounds] = useState<DOMRectReadOnly | null>(null);

  // Create a map of all players (current + others) for easy lookup
  const playerMap = new Map<string, PlayerDto | OtherPlayerDto>();
  if (gameState?.currentPlayer) {
    playerMap.set(gameState.currentPlayer.id, gameState.currentPlayer);
  }
  gameState?.otherPlayers?.forEach((otherPlayer) => {
    playerMap.set(otherPlayer.id, otherPlayer);
  });

  // Construct allPlayers using the turn order from the backend
  const allPlayers: (PlayerDto | OtherPlayerDto)[] =
    (gameState?.turnOrder
      ?.map((playerId) => playerMap.get(playerId))
      .filter((player) => player !== undefined) as (PlayerDto | OtherPlayerDto)[]) || [];

  // Find the current turn player for the right sidebar
  const currentTurnPlayer =
    allPlayers.find((player) => player.id === gameState?.currentTurn) || null;

  const handleKickPlayer = useCallback(
    (playerId: string) => {
      const player = allPlayers.find((p) => p.id === playerId);
      useConfirmDialogStore
        .getState()
        .request({ kind: "kick", playerId, playerName: player?.name || "Unknown" });
    },
    [allPlayers],
  );

  const handleConvertToBot = useCallback(
    (playerId: string) => {
      const player = allPlayers.find((p) => p.id === playerId);
      useConfirmDialogStore
        .getState()
        .request({ kind: "convertToBot", playerId, playerName: player?.name || "Unknown" });
    },
    [allPlayers],
  );

  const phase = useAppPhaseStore((s) => s.phase);
  const isShowcase = phase.kind === "showcase";
  const showUI =
    isShowcase ||
    phase.kind === "animateUI" ||
    phase.kind === "playing" ||
    phase.kind === "completed";
  const showHud = showUI && !isShowcase;
  const isAnimatingIn = phase.kind === "animateUI";
  const uiAnimationClass = isAnimatingIn ? "animate-[uiFadeIn_1200ms_ease-out_both]" : "";
  // Elements already shown during the showcase fade in once, when the showcase starts.
  const earlyUiAnimationClass =
    isShowcase || isAnimatingIn ? "animate-[uiFadeIn_1200ms_ease-out_both]" : "";
  const endgameFadeClass = endgameFadeUI ? "opacity-0 pointer-events-none" : "opacity-100";

  return (
    <div className="relative w-full h-dvh text-white overflow-hidden">
      {/* CSS animations for transition */}
      <style>{`
        @keyframes uiFadeIn {
          from { opacity: 0; }
          to { opacity: 1; }
        }
      `}</style>

      {/* Game world canvas: mounted while it should be visible; covers SpaceBackground.
          Skipped during "lobby" so the persistent SpaceBackground at App level shows through. */}
      {phase.kind !== "lobby" && (
        <div className="absolute inset-0 bg-[#000000]">
          <MainContentDisplay
            gameState={gameState}
            animateHexEntrance={animateHexEntrance}
            startDark={startDark}
            tilesHidden={tilesHidden}
            onSkyboxReady={onSkyboxReady}
            onGpuReady={onGpuReady}
            showUI={showUI}
            uiAnimationClass={uiAnimationClass}
          />
        </div>
      )}

      {/* TopMenuBar overlays on top — always visible in endgame */}
      {showUI && !showStartingSelection && (
        <div
          className={`${earlyUiAnimationClass} ${isEndgame ? "opacity-100" : endgameFadeClass} transition-opacity duration-700 ease-in-out`}
        >
          <TopMenuBar
            minimal={isShowcase}
            contentClassName={uiAnimationClass}
            gameState={gameState}
            currentPlayer={currentPlayer}
            onStandardProjectSelect={onStandardProjectSelect}
            onLeaveGame={onLeaveGame}
            onEndGame={onEndGame}
            gameId={gameState?.id}
            isEndgame={isEndgame}
            activeEndgamePanel={activeEndgamePanel}
            onEndgamePanelChange={onEndgamePanelChange}
            hasHistory={hasHistory}
          />
        </div>
      )}

      {/* Chat overlay - rendered before sidebars so it's behind them in z-order */}
      {showHud && !showStartingSelection && chatMessages && onSendChatMessage && (
        <SolarSystemFade>
          <div className={uiAnimationClass}>
            <ChatOverlay
              onBoundsChange={setChatBounds}
              messages={chatMessages}
              onSendMessage={onSendChatMessage}
              isEndgame={endgameFadeUI}
              playerColorMap={playerColorMap}
            />
          </div>
        </SolarSystemFade>
      )}

      {showUI && (
        <SolarSystemFade>
          <CardPlayPresentation chatBounds={chatBounds} />
          <CardInspections
            store={inspectionStore}
            hand={inspectionHand}
            blocked={inspectionBlocked}
            chatBounds={chatBounds}
            onDrag={onInspectionDrag}
          />
        </SolarSystemFade>
      )}

      {/* Player list — stays visible in solar system view */}
      {showUI && (
        <div
          className={`${earlyUiAnimationClass} ${endgameFadeClass} transition-opacity duration-700 ease-in-out`}
        >
          <LeftSidebar
            ref={ref}
            players={allPlayers}
            currentPlayer={currentPlayer}
            turnPlayerId={
              gameState?.currentPhase === GamePhaseInitApplyCorp ||
              gameState?.currentPhase === GamePhaseInitApplyPrelude
                ? initTurnPlayerId || ""
                : gameState?.currentTurn || ""
            }
            currentPhase={gameState?.currentPhase}
            hostPlayerId={gameState?.hostPlayerId}
            triggeredEffects={triggeredEffects}
            onPlayerClick={onPlayerClick}
            onKickPlayer={handleKickPlayer}
            onConvertToBot={
              gameState?.settings?.hasClaudeOAuthToken ? handleConvertToBot : undefined
            }
          />
        </div>
      )}

      {/* Overlay Components — fade out in solar system view */}
      {showHud && (
        <SolarSystemFade>
          <div
            className={`${uiAnimationClass} ${endgameFadeClass} transition-opacity duration-700 ease-in-out`}
          >
            <RightSidebar
              game={gameState ?? undefined}
              globalParameters={gameState?.globalParameters}
              generation={gameState?.generation}
              currentPlayer={currentTurnPlayer}
              showVenus={gameState?.settings?.systems.includes("venus")}
            />

            {playedCardNotification && onPlayedCardTogglePin && onPlayedCardAdvance && (
              <WithoutInspection store={inspectionStore}>
                <PlayedCardNotificationOverlay
                  notification={playedCardNotification}
                  isPinned={isPlayedCardPinned ?? false}
                  onTogglePin={onPlayedCardTogglePin}
                  onAdvance={onPlayedCardAdvance}
                />
              </WithoutInspection>
            )}
          </div>
        </SolarSystemFade>
      )}

      {showHud &&
        !showStartingSelection &&
        (gameState?.currentPhase !== GamePhaseComplete ||
          (isEndgame && activeEndgamePanel === "replay")) && (
          <SolarSystemFade>
            <div className={uiAnimationClass}>
              <BottomResourceBar
                currentPlayer={currentPlayer}
                gameState={gameState}
                playedCards={playedCards}
                changedPaths={changedPaths}
                callbacks={bottomBarCallbacks}
                gameId={gameState?.id}
                corporation={corporationCard}
                spectatingPlayer={spectatingPlayer}
                spectatingCorporation={spectatingCorporation}
                spectatePlayerColor={spectatePlayerColor}
                onStopSpectating={onStopSpectating}
                isGameSpectator={isGameSpectator}
              />
            </div>
          </SolarSystemFade>
        )}

      <GameConfirmDialogs />
    </div>
  );
});

export default GameLayout;
