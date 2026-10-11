import { usePaymentStore } from "@/stores/paymentStore";
import ResourceRemovalOverlay from "../../ui/overlay/ResourceRemovalOverlay";
import CardRevealOverlay from "@/components/ui/overlay/CardRevealOverlay.tsx";
import EffectSelectionOverlay from "@/components/ui/overlay/EffectSelectionOverlay.tsx";
import { useEffect, useCallback, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import GameLayout, { SolarSystemFade, type GameLayoutProps } from "./GameLayout.tsx";
import MobileGameLayout from "../../mobile/MobileGameLayout.tsx";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import CardsPlayedModal from "../../ui/modals/CardsPlayedModal.tsx";
import ProductionPhaseModal from "../../ui/modals/ProductionPhaseModal.tsx";
import PaymentSelectionPopover from "../../ui/popover/PaymentSelectionPopover.tsx";
import DebugDropdown from "../../ui/debug/DebugDropdown.tsx";
import PerformanceWindow from "../../ui/debug/PerformanceWindow.tsx";
import FeedbackWindow from "../../ui/debug/FeedbackWindow.tsx";

import { WindowManagerProvider } from "../../ui/debug/WindowManager.tsx";
import WaitingRoomOverlay from "../../ui/overlay/WaitingRoomOverlay.tsx";
import PlayerSelectionOverlay from "../../ui/overlay/PlayerSelectionOverlay.tsx";
import JoinGameOverlay from "../../ui/overlay/JoinGameOverlay.tsx";
import SpectateGameOverlay from "../../ui/overlay/SpectateGameOverlay.tsx";
import TabConflictOverlay from "../../ui/overlay/TabConflictOverlay.tsx";
import StartingCardSelectionOverlay from "../../ui/overlay/StartingCardSelectionOverlay.tsx";
import PendingCardSelectionOverlay from "../../ui/overlay/PendingCardSelectionOverlay.tsx";
import CardDrawSelectionOverlay from "../../ui/overlay/CardDrawSelectionOverlay.tsx";
import BehaviorResolutionOverlay from "../../ui/overlay/BehaviorResolutionOverlay.tsx";
import AwardFundSelectionPopover from "../../ui/popover/AwardFundSelectionPopover.tsx";
import CardFanOverlay, { CardFanOverlayHandle } from "../../ui/overlay/CardFanOverlay.tsx";
import ColonySelectionOverlay from "../../ui/overlay/ColonySelectionOverlay.tsx";
import FreeTradeSelectionOverlay from "../../ui/overlay/FreeTradeSelectionOverlay.tsx";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "../../ui/popover/GameFlowPopover.tsx";
import CorporationOverlay from "../../ui/overlay/CorporationOverlay.tsx";
import CorporationShowcaseOverlay from "../../ui/overlay/CorporationShowcaseOverlay.tsx";
import LoadingOverlay from "../../game/view/LoadingOverlay.tsx";
import GameEventBanner from "../../ui/overlay/GameEventBanner.tsx";
import { useCardInspection, type CardInspectionDrag } from "@/hooks/useCardInspection.ts";
import { useGameEvent } from "@/hooks/useGameEvent.ts";
import { usePlayedCardNotification } from "@/hooks/usePlayedCardNotification.ts";
import ChatOverlay from "../../ui/overlay/ChatOverlay.tsx";
import GameButton from "../../ui/buttons/GameButton.tsx";
import { BotPersonaChip, PlayerChip } from "../../ui/display/BotChips.tsx";
import GameMenuModal from "../../ui/overlay/GameMenuModal.tsx";
import CardBrowserOverlay from "../../ui/overlay/CardBrowserOverlay.tsx";
import MainMenuHamburger, { CurrentGameMenuButton } from "../../ui/buttons/MainMenuHamburger.tsx";
import EndGameBottomBar from "../../ui/endgame/EndGameBottomBar.tsx";
import { VPCountingProvider } from "../../../contexts/VPCountingContext.tsx";
import { useVPCountingAnimation } from "@/hooks/useVPCountingAnimation.ts";

import { Z_INDEX } from "@/constants/zIndex.ts";
import { useGameHistory } from "@/hooks/useGameHistory.ts";
import { useGameReplay } from "@/hooks/useGameReplay.ts";
import {
  historyEntryToGameDto,
  buildCardLookup,
  historyPlayerToPlayerDto,
} from "@/utils/historyToGameDto.ts";
import ChoiceSelectionPopover from "../../ui/popover/ChoiceSelectionPopover.tsx";
import CardStorageSelectionPopover from "../../ui/popover/CardStorageSelectionPopover.tsx";
import TargetPlayerSelectionPopover from "../../ui/popover/TargetPlayerSelectionPopover.tsx";
import CardResourceSelectionPopover from "../../ui/popover/CardResourceSelectionPopover.tsx";
import AmountSelectionPopover from "../../ui/popover/AmountSelectionPopover.tsx";
import ActionReusePopover from "../../ui/popover/ActionReusePopover.tsx";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { clearGameSession } from "@/utils/sessionStorage.ts";
import { useSoundEffects } from "@/hooks/useSoundEffects.ts";
import { useSpaceBackground } from "@/contexts/SpaceBackgroundContext.tsx";
import { useNotifications } from "@/contexts/NotificationContext.tsx";
import {
  GamePhaseComplete,
  GamePhaseInitApplyCorp,
  GamePhaseInitApplyPrelude,
  GamePhaseStartingSelection,
  GameStatusActive,
  GameStatusCompleted,
  GameStatusLobby,
  ResourceType,
} from "@/types/generated/api-types.ts";
import type { PlayerDto, OtherPlayerDto, CardDto, GameDto } from "@/types/generated/api-types.ts";
import { PlayerListHandle } from "../../ui/list/PlayerList.tsx";

import { useGameStore } from "@/stores/gameStore.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import { useCardPlayFlowStore } from "@/stores/cardPlayFlowStore.ts";
import { isInGameWorld, useAppPhaseStore } from "@/stores/appPhaseStore.ts";
import { useSpectateStore } from "@/stores/spectateStore.ts";
import { useCardPlayFlow } from "@/hooks/useCardPlayFlow.ts";
import { useWebSocketConnection } from "@/hooks/useWebSocketConnection.ts";
import { useGameInitialization } from "@/hooks/useGameInitialization.ts";
import { useGameTransitions } from "@/hooks/useGameTransitions.ts";
import { useGameHotkeys } from "@/hooks/useGameHotkeys.ts";
import { PlanetFocusProvider } from "../../../contexts/PlanetFocusContext.tsx";

const ARRIVAL_LOADING_DELAY_MS = 600;

function WaitingPlayerList({ game }: { game: GameDto }) {
  const allPlayers: {
    id: string;
    name: string;
    isReady: boolean;
    isSelf: boolean;
    playerType: string;
    botPersona?: string;
    botStatus?: string;
  }[] = [];

  if (game.currentPlayer) {
    allPlayers.push({
      id: game.currentPlayer.id,
      name: game.currentPlayer.name,
      isReady:
        !game.currentPlayer.selectCorporationPhase &&
        !game.currentPlayer.selectPreludeCardsPhase &&
        !game.currentPlayer.selectStartingCardsPhase &&
        !!game.currentPlayer.corporation,
      isSelf: true,
      playerType: game.currentPlayer.playerType,
      botPersona: game.currentPlayer.botPersona || undefined,
      botStatus: game.currentPlayer.botStatus || undefined,
    });
  }

  game.otherPlayers?.forEach((other) => {
    allPlayers.push({
      id: other.id,
      name: other.name,
      isReady:
        !other.selectStartingCardsPhase &&
        !other.selectCorporationPhase &&
        !other.selectPreludeCardsPhase &&
        !!other.corporation,
      isSelf: false,
      playerType: other.playerType,
      botPersona: other.botPersona || undefined,
      botStatus: other.botStatus || undefined,
    });
  });

  const ordered = game.turnOrder?.length
    ? game.turnOrder
        .map((pid) => allPlayers.find((p) => p.id === pid))
        .filter((p) => p !== undefined)
    : allPlayers;

  return (
    <div className="flex flex-col gap-2">
      {ordered.map((player) => (
        <div
          key={player.id}
          className="flex flex-wrap justify-between items-center gap-2 py-2 px-3 bg-black/40 border border-space-blue-600/50"
        >
          <span className="text-white text-sm font-medium">{player.name}</span>
          <div className="player-chip-group">
            {player.isSelf && <PlayerChip className="bg-space-blue-800 text-white">You</PlayerChip>}
            {player.playerType === "bot" && (
              <BotPersonaChip persona={player.botPersona} botStatus={player.botStatus} />
            )}
            {player.isReady ? (
              <PlayerChip className="bg-emerald-700/80 text-white">
                <svg
                  width="10"
                  height="10"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="3"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
                Ready
              </PlayerChip>
            ) : (
              <PlayerChip className="bg-white/10 text-white/70">
                <div className="w-2.5 h-2.5 border border-white/50 border-t-transparent rounded-full animate-spin" />
                Selecting...
              </PlayerChip>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

export default function GameInterface() {
  const location = useLocation();
  const navigate = useNavigate();
  const { gameId: urlGameId } = useParams<{ gameId?: string }>();
  const { playProductionSound } = useSoundEffects();
  const { showNotification } = useNotifications();
  const { isLoaded: isSpaceBgLoaded } = useSpaceBackground();

  // Refs that stay in the component
  const cardFanRef = useRef<CardFanOverlayHandle>(null);
  const playerListRef = useRef<PlayerListHandle>(null);
  const notificationQueueDoneAt = useRef<number>(0);
  const winnerBannerShownRef = useRef<string | null>(null);

  // Game event and played card notification hooks
  const playedCardNotification = usePlayedCardNotification();
  const { enqueue: enqueuePlayedCard } = playedCardNotification;

  // --- Stores ---
  const game = useGameStore((s) => s.game);
  const currentPlayer = useGameStore((s) => s.currentPlayer);
  const playerId = useGameStore((s) => s.playerId);
  const isConnected = useGameStore((s) => s.isConnected);
  const isSpectator = useGameStore((s) => s.isSpectator);
  const isReconnecting = useGameStore((s) => s.isReconnecting);
  const {
    currentEvent,
    dismissCurrent: dismissGameEvent,
    enqueue: enqueueGameEvent,
  } = useGameEvent(game, isConnected && !isReconnecting);
  const reconnectionStep = useGameStore((s) => s.reconnectionStep);
  const gameForSelection = useGameStore((s) => s.gameForSelection);
  const changedPaths = useGameStore((s) => s.changedPaths);
  const triggeredEffects = useGameStore((s) => s.triggeredEffects);
  const chatMessages = useGameStore((s) => s.chatMessages);
  const corporationData = useGameStore((s) => s.corporationData);
  const displayedInitPlayerId = useGameStore((s) => s.displayedInitPlayerId);

  const showCardsPlayedModal = useUIOverlayStore((s) => s.showCardsPlayedModal);
  const showProductionPhaseModal = useUIOverlayStore((s) => s.showProductionPhaseModal);
  const isProductionModalHidden = useUIOverlayStore((s) => s.isProductionModalHidden);
  const openProductionToCardSelection = useUIOverlayStore((s) => s.openProductionToCardSelection);
  const showDebugDropdown = useUIOverlayStore((s) => s.showDebugDropdown);
  const showPerformanceWindow = useUIOverlayStore((s) => s.showPerformanceWindow);
  const showFeedbackWindow = useUIOverlayStore((s) => s.showFeedbackWindow);
  const showTabConflict = useUIOverlayStore((s) => s.showTabConflict);
  const conflictingTabInfo = useUIOverlayStore((s) => s.conflictingTabInfo);
  const showLeaveGameConfirm = useUIOverlayStore((s) => s.showLeaveGameConfirm);
  const showCloseGameConfirm = useUIOverlayStore((s) => s.showCloseGameConfirm);
  const showEndGameConfirm = useUIOverlayStore((s) => s.showEndGameConfirm);
  const showStartingSelection = useUIOverlayStore((s) => s.showStartingSelection);
  const isStartingSelectionHidden = useUIOverlayStore((s) => s.isStartingSelectionHidden);
  const showPendingCardSelection = useUIOverlayStore((s) => s.showPendingCardSelection);
  const hasBlockingSelection = Boolean(
    currentPlayer?.pendingCardDrawSelection ||
    currentPlayer?.pendingCardSelection ||
    currentPlayer?.pendingTileSelection ||
    currentPlayer?.pendingColonySelection ||
    currentPlayer?.pendingColonyResourceSelection ||
    currentPlayer?.pendingAwardFundSelection ||
    currentPlayer?.pendingFreeTradeSelection ||
    currentPlayer?.pendingEffectSelection ||
    currentPlayer?.pendingResourceRemovalSelection ||
    currentPlayer?.pendingCardReveal ||
    currentPlayer?.pendingBehaviorResolutions?.some((resolution) => resolution.kind !== "optional"),
  );
  const showCardDrawSelection = useUIOverlayStore((s) => s.showCardDrawSelection);
  const showCorporationOverlay = useUIOverlayStore((s) => s.showCorporationOverlay);
  const showResourceRemovalSelection = useUIOverlayStore((s) => s.showResourceRemovalSelection);
  const showColonyResourceSelection = useUIOverlayStore((s) => s.showColonyResourceSelection);
  const showColonyPlacementSelection = useUIOverlayStore((s) => s.showColonyPlacementSelection);
  const showFreeTradeSelection = useUIOverlayStore((s) => s.showFreeTradeSelection);
  const showCardBrowser = useUIOverlayStore((s) => s.showCardBrowser);

  const cardPendingChoice = useCardPlayFlowStore((s) => s.cardPendingChoice);
  const pendingCardBehaviorIndex = useCardPlayFlowStore((s) => s.pendingCardBehaviorIndex);
  const showChoiceSelection = useCardPlayFlowStore((s) => s.showChoiceSelection);
  const actionPendingChoice = useCardPlayFlowStore((s) => s.actionPendingChoice);
  const showActionChoiceSelection = useCardPlayFlowStore((s) => s.showActionChoiceSelection);
  const pendingActionReuse = useCardPlayFlowStore((s) => s.pendingActionReuse);
  const showActionReuseSelection = useCardPlayFlowStore((s) => s.showActionReuseSelection);
  const pendingFreeTradeWarning = useCardPlayFlowStore((s) => s.pendingFreeTradeWarning);
  const showFreeTradeWarning = useCardPlayFlowStore((s) => s.showFreeTradeWarning);
  const pendingCardStorage = useCardPlayFlowStore((s) => s.pendingCardStorage);
  const showCardStorageSelection = useCardPlayFlowStore((s) => s.showCardStorageSelection);
  const pendingActionStorage = useCardPlayFlowStore((s) => s.pendingActionStorage);
  const showActionStorageSelection = useCardPlayFlowStore((s) => s.showActionStorageSelection);
  const pendingTargetPlayer = useCardPlayFlowStore((s) => s.pendingTargetPlayer);
  const showTargetPlayerSelection = useCardPlayFlowStore((s) => s.showTargetPlayerSelection);
  const pendingActionTargetPlayer = useCardPlayFlowStore((s) => s.pendingActionTargetPlayer);
  const showActionTargetPlayerSelection = useCardPlayFlowStore(
    (s) => s.showActionTargetPlayerSelection,
  );
  const pendingCardResourceInput = useCardPlayFlowStore((s) => s.pendingCardResourceInput);
  const showCardResourceSelection = useCardPlayFlowStore((s) => s.showCardResourceSelection);
  const pendingVariableAmount = useCardPlayFlowStore((s) => s.pendingVariableAmount);
  const showAmountSelection = useCardPlayFlowStore((s) => s.showAmountSelection);

  const phase = useAppPhaseStore((s) => s.phase);
  const isSkyboxReady = useAppPhaseStore((s) => s.isSkyboxReady);
  const isGpuReady = useAppPhaseStore((s) => s.isGpuReady);
  const marsRevealedReady = useAppPhaseStore((s) => s.marsRevealedReady);
  const lobbyMounted = useAppPhaseStore((s) => s.lobbyMounted);

  const spectatePlayerId = useSpectateStore((s) => s.spectatePlayerId);
  const replaySpectatePlayerId = useSpectateStore((s) => s.replaySpectatePlayerId);

  // --- Hooks ---
  const { isCompact } = useLayoutMode();
  const flow = useCardPlayFlow();
  const { attemptReconnection: _attemptReconnection } = useWebSocketConnection(
    navigate,
    enqueuePlayedCard,
    notificationQueueDoneAt,
    showNotification,
  );
  const init = useGameInitialization({ navigate, location, urlGameId });
  useGameTransitions(playProductionSound, notificationQueueDoneAt);
  useGameHotkeys(cardFanRef, playerListRef);

  // --- VP counting animation ---
  const isGameComplete =
    game?.currentPhase === GamePhaseComplete && game?.status === GameStatusCompleted;
  const scoringGameId =
    isGameComplete && phase.kind === "completed" && !showProductionPhaseModal ? game.id : null;
  const vpCounting = useVPCountingAnimation(scoringGameId, game?.finalScores ?? []);

  useEffect(() => {
    if (!scoringGameId) {
      winnerBannerShownRef.current = null;
      return;
    }
    if (
      !vpCounting.state.isComplete ||
      winnerBannerShownRef.current === scoringGameId ||
      !game?.finalScores?.length
    ) {
      return;
    }
    winnerBannerShownRef.current = scoringGameId;
    const sorted = [...game.finalScores].sort(
      (a, b) => b.vpBreakdown.totalVP - a.vpBreakdown.totalVP,
    );
    const w = sorted[0];
    const allPlayers = [game.currentPlayer, ...(game.otherPlayers ?? [])];
    const wp = allPlayers.find((p) => p?.id === w.playerId);
    enqueueGameEvent({
      title: `${w.playerName} WINS!`,
      duration: 12000,
      playerName: w.playerName,
      playerColor: wp?.color ?? "#f59e0b",
      size: "large",
    });
  }, [scoringGameId, vpCounting.state.isComplete, game, enqueueGameEvent]);

  // --- Game history and replay ---
  const { entries: historyEntries } = useGameHistory(isGameComplete ? (game?.id ?? null) : null);
  const replay = useGameReplay(historyEntries);
  const hasHistory = (historyEntries?.length ?? 0) > 0;

  // Endgame panel tracking
  const [endgamePanel, setEndgamePanel] = useState<"score" | "graphs" | "replay">("score");
  const endgameFadeUI = isGameComplete && endgamePanel !== "replay";

  const handleEndgamePanelChange = useCallback(
    (panel: "score" | "graphs" | "replay") => {
      setEndgamePanel(panel);
      const isCounting = vpCounting.state.isActive && !vpCounting.state.isComplete;
      if (panel !== "score" && isCounting) {
        vpCounting.skip();
      }
      if (panel !== "score") {
        dismissGameEvent();
      }
      if (panel === "replay" && !replay.isActive) {
        replay.start();
      }
      if (panel !== "replay" && replay.isActive) {
        replay.exit();
        useSpectateStore.getState().setReplaySpectatePlayerId(null);
      }
    },
    [vpCounting, replay, dismissGameEvent],
  );

  const cardLookup = useMemo(() => {
    if (!game) {
      return new Map();
    }
    return buildCardLookup(game);
  }, [game]);

  const replayGameState = useMemo(() => {
    if (!replay.isActive || !replay.currentEntry || !game) {
      return null;
    }
    return historyEntryToGameDto(replay.currentEntry, game, cardLookup);
  }, [replay.isActive, replay.currentEntry, game, cardLookup]);

  const replayViewAsPlayer = useMemo(() => {
    if (!replay.isActive || !replay.currentEntry || !replaySpectatePlayerId) {
      return null;
    }
    const historyPlayer = replay.currentEntry.players[replaySpectatePlayerId];
    if (!historyPlayer) {
      return null;
    }
    return historyPlayerToPlayerDto(historyPlayer, cardLookup);
  }, [replay.isActive, replay.currentEntry, replaySpectatePlayerId, cardLookup]);

  // --- Spectate player derived values ---
  const spectatePlayer = useMemo(() => {
    if (!spectatePlayerId || !game) {
      return null;
    }
    if (game.currentPlayer?.id === spectatePlayerId) {
      return game.currentPlayer;
    }
    return game.otherPlayers?.find((p) => p.id === spectatePlayerId) ?? null;
  }, [spectatePlayerId, game]);

  const spectatePlayerColor = useMemo(() => {
    if (!spectatePlayerId || !game) {
      return "#6496ff";
    }
    if (game.currentPlayer?.id === spectatePlayerId) {
      return game.currentPlayer.color || "#6496ff";
    }
    const other = game.otherPlayers?.find((p) => p.id === spectatePlayerId);
    return other?.color || "#6496ff";
  }, [spectatePlayerId, game]);

  const playerColorMap = useMemo(() => {
    if (!game) {
      return new Map<string, string>();
    }
    const map = new Map<string, string>();
    if (game.currentPlayer?.id && game.currentPlayer.color) {
      map.set(game.currentPlayer.id, game.currentPlayer.color);
    }
    game.otherPlayers?.forEach((p) => {
      if (p.id && p.color) {
        map.set(p.id, p.color);
      }
    });
    game.spectators?.forEach((s) => {
      if (s.id && s.color) {
        map.set(s.id, s.color);
      }
    });
    return map;
  }, [game]);

  const handlePlayerClick = useCallback(
    (player: PlayerDto | OtherPlayerDto) => {
      const phase = game?.currentPhase;
      if (phase === GamePhaseInitApplyCorp || phase === GamePhaseInitApplyPrelude) {
        return;
      }
      useSpectateStore.getState().toggleSpectatePlayer(player.id, game?.currentPlayer?.id);
    },
    [game?.currentPlayer?.id, game?.currentPhase],
  );

  const handleStopSpectating = useCallback(() => {
    useSpectateStore.getState().clearSpectating();
  }, []);

  // --- Computed phase booleans ---
  const isLobbyPhase = game?.status === GameStatusLobby;
  const isPreGamePhase = isLobbyPhase;

  const showWaitingForPlayers =
    game?.status === GameStatusActive &&
    game?.currentPhase === GamePhaseStartingSelection &&
    !game?.currentPlayer?.selectCorporationPhase &&
    !game?.currentPlayer?.selectPreludeCardsPhase &&
    !game?.currentPlayer?.selectStartingCardsPhase &&
    !game?.currentPlayer?.pendingTileSelection;

  // --- Simple WebSocket handlers that stay here ---
  const handleStartingChoicesConfirm = useCallback(
    async (corporationId: string, preludeIds: string[], cardIds: string[]) => {
      try {
        await globalWebSocketManager.selectStartingChoices(corporationId, preludeIds, cardIds);
      } catch (error) {
        console.error("Failed to select starting choices:", error);
      }
    },
    [],
  );

  const handlePendingCardSelection = useCallback(async (selectedCardIds: string[]) => {
    try {
      await globalWebSocketManager.selectCards(selectedCardIds);
    } catch (error) {
      console.error("Failed to select cards:", error);
    }
  }, []);

  const handleCardDrawConfirm = useCallback(async (cardsToTake: string[], cardsToBuy: string[]) => {
    try {
      await globalWebSocketManager.confirmCardDraw(cardsToTake, cardsToBuy);
    } catch (error) {
      console.error("Failed to confirm card draw:", error);
    }
  }, []);

  // --- Leave/end game handlers ---
  const handleLeaveGame = useUIOverlayStore((s) => s.requestLeaveGame);

  const handleConfirmLeaveGame = useCallback(() => {
    useUIOverlayStore.getState().setShowLeaveGameConfirm(false);
    useUIOverlayStore.getState().setShowCloseGameConfirm(false);
    clearGameSession();
    globalWebSocketManager.disconnect();
    setTimeout(() => {
      navigate("/", { replace: true });
    }, 100);
  }, [navigate]);

  const handleEndGame = useUIOverlayStore((s) => s.requestEndGame);

  const handleConfirmEndGame = useCallback(() => {
    useUIOverlayStore.getState().setShowEndGameConfirm(false);
    void globalWebSocketManager.endGame();
  }, []);

  // --- Bottom bar callbacks ---
  const bottomBarCallbacks = useMemo(
    () => ({
      onOpenCardsPlayedModal: () => useUIOverlayStore.getState().setShowCardsPlayedModal(true),
      onActionSelect: flow.handleActionSelect,
      onConvertPlantsToGreenery: flow.handleConvertPlantsToGreenery,
      onConvertHeatToTemperature: flow.handleConvertHeatToTemperature,
    }),
    [
      flow.handleActionSelect,
      flow.handleConvertPlantsToGreenery,
      flow.handleConvertHeatToTemperature,
    ],
  );

  // --- Loading message ---
  const loadingMessage = (() => {
    if (phase.kind === "selecting" || phase.kind === "joining" || phase.kind === "spectating") {
      if (!isSpaceBgLoaded) {
        return "Loading";
      }
      return "Loading game";
    }
    if (phase.kind === "checking") {
      return "Loading game";
    }
    if (phase.kind === "connecting") {
      return "Connecting";
    }
    if (isReconnecting && reconnectionStep) {
      if (reconnectionStep === "game") {
        return "Reconnecting to game";
      }
      if (reconnectionStep === "environment") {
        return "Loading";
      }
    }
    if (!isSkyboxReady) {
      return "Loading";
    }
    return "Connecting to game";
  })();

  const loadingSubtitle = (() => {
    if (!isSkyboxReady || !isGpuReady) {
      if (isSkyboxReady && !isGpuReady) {
        return "Preparing renderer";
      }
      if (!isSkyboxReady) {
        return "Preparing environment";
      }
      return "Loading assets";
    }
    return "";
  })();

  // --- Loading state ---
  const isFullyLoaded = (() => {
    if (phase.kind === "menu") {
      return false;
    }
    if (phase.kind === "checking" || phase.kind === "connecting" || phase.kind === "loading") {
      return false;
    }
    if (phase.kind === "selecting" || phase.kind === "joining" || phase.kind === "spectating") {
      return isSpaceBgLoaded;
    }
    if (phase.kind === "lobby") {
      return isConnected && !!game && !isReconnecting;
    }
    return isConnected && !!game && !isReconnecting && isSkyboxReady && isGpuReady;
  })();

  const handleSkyboxReady = useCallback(() => {
    useAppPhaseStore.getState().setSkyboxReady(true);
  }, []);

  const handleGpuReady = useCallback(() => {
    useAppPhaseStore.getState().setGpuReady(true);
  }, []);

  const handleLoadingTransitionEnd = useCallback(() => {}, []);

  // Create/join already confirmed the game, so the quick re-check on arrival gets a grace
  // period: if it finishes in time the lobby fades in with no loading screen at all.
  const arrivedWithGame = !!(location.state as { game?: GameDto } | null)?.game;
  const beforeLobby =
    phase.kind === "menu" || phase.kind === "checking" || phase.kind === "connecting";
  const loadingShowDelayMs = arrivedWithGame && beforeLobby ? ARRIVAL_LOADING_DELAY_MS : 0;

  // --- Backdrop ---
  const hasPendingActionSelection =
    showResourceRemovalSelection ||
    showColonyResourceSelection ||
    showColonyPlacementSelection ||
    showFreeTradeSelection;

  const shouldShowStartingBackdrop =
    (showStartingSelection &&
      !isStartingSelectionHidden &&
      (marsRevealedReady || phase.kind === "playing")) ||
    showWaitingForPlayers;

  // --- Card fan visibility ---
  const colonyAllPlayers = useMemo(() => {
    if (!game) {
      return [];
    }
    return [
      ...(game.currentPlayer
        ? [
            {
              id: game.currentPlayer.id,
              name: game.currentPlayer.name,
              color: game.currentPlayer.color,
            },
          ]
        : []),
      ...(game.otherPlayers?.map((p) => ({ id: p.id, name: p.name, color: p.color })) ?? []),
    ];
  }, [
    game?.currentPlayer?.id,
    game?.currentPlayer?.name,
    game?.currentPlayer?.color,
    game?.otherPlayers,
  ]);

  const hideCardFanForModals =
    showStartingSelection ||
    showPendingCardSelection ||
    showCardDrawSelection ||
    (game?.currentPlayer?.pendingBehaviorResolutions?.length ?? 0) > 0 ||
    showColonyPlacementSelection ||
    showFreeTradeSelection ||
    isPreGamePhase ||
    !!currentPlayer?.pendingTileSelection;

  const inspectionStore = useCardInspection();
  const { inspectCard, finishInspection, clearInspection } = inspectionStore.getState();
  const dragInspectionCard = useCallback(
    (cardId: string, drag: CardInspectionDrag, detail: HTMLElement) => {
      cardFanRef.current?.startInspectionDrag(cardId, drag, detail);
    },
    [],
  );
  const playSession = useCardPlayFlowStore((s) => s.playSession);
  const playError = useCardPlayFlowStore((s) => s.playError);
  const showPaymentSelection = usePaymentStore((s) => !!s.pending);
  const inspectionBlocked =
    !!playSession ||
    hideCardFanForModals ||
    hasPendingActionSelection ||
    (showProductionPhaseModal && !isProductionModalHidden) ||
    showChoiceSelection ||
    showActionChoiceSelection ||
    showActionReuseSelection ||
    showCardStorageSelection ||
    showPaymentSelection ||
    showActionStorageSelection ||
    showTargetPlayerSelection ||
    showActionTargetPlayerSelection ||
    showCardResourceSelection ||
    showAmountSelection ||
    showFreeTradeWarning ||
    showTabConflict ||
    showCardsPlayedModal ||
    showCardBrowser ||
    showLeaveGameConfirm ||
    showCloseGameConfirm ||
    showEndGameConfirm ||
    spectatePlayerId !== null;
  const inspectionHand = replayViewAsPlayer?.cards ?? currentPlayer?.cards ?? [];
  useEffect(() => {
    clearInspection();
    const clearPlay = () => {
      const pending = usePaymentStore.getState().pending;
      if (pending?.intent.action === "play-card") {
        pending.reject(new Error("Payment cancelled"));
      }
      useCardPlayFlowStore.getState().clearPlayPresentation();
    };
    clearPlay();
    return clearPlay;
  }, [
    game?.id,
    currentPlayer?.id,
    replay.isActive,
    replayViewAsPlayer?.id,
    spectatePlayerId,
    clearInspection,
  ]);
  useEffect(() => {
    if (inspectionBlocked) {
      clearInspection();
      return;
    }
    for (const inspection of inspectionStore.getState().inspections) {
      if (!inspectionHand.some((card) => card.id === inspection.cardId)) {
        finishInspection(inspection);
      }
    }
  }, [inspectionBlocked, inspectionHand, inspectionStore, clearInspection, finishInspection]);

  const handleMobilePlayCard = useCallback(
    (cardId: string) => {
      void flow.handlePlayCard(cardId);
    },
    [flow.handlePlayCard],
  );

  const handleSendChatMessage = useCallback((message: string) => {
    void globalWebSocketManager.sendChatMessage(message);
  }, []);

  const layoutProps: GameLayoutProps | null = game
    ? {
        inspectionStore,
        inspectionHand,
        inspectionBlocked,
        onInspectionDrag: dragInspectionCard,
        gameState: replayGameState ?? game,
        currentPlayer: replayViewAsPlayer ?? (replay.isActive ? null : currentPlayer),
        playedCards: replayViewAsPlayer?.playedCards ?? currentPlayer?.playedCards ?? [],
        corporationCard: replayViewAsPlayer?.corporation ?? corporationData,
        initTurnPlayerId: displayedInitPlayerId,
        showStartingSelection,
        animateHexEntrance:
          phase.kind === "marsRevealed" ||
          phase.kind === "showcase" ||
          phase.kind === "animateUI" ||
          phase.kind === "playing" ||
          phase.kind === "completed",
        startDark: phase.kind === "loading" || phase.kind === "lobby",
        tilesHidden:
          phase.kind === "loading" || phase.kind === "lobby" || phase.kind === "fadeOutLobby",
        changedPaths,
        triggeredEffects,
        bottomBarCallbacks,
        onStandardProjectSelect: flow.handleStandardProjectSelect,
        onLeaveGame: handleLeaveGame,
        onEndGame: handleEndGame,
        onSkyboxReady: handleSkyboxReady,
        onGpuReady: handleGpuReady,
        onPlayerClick: handlePlayerClick,
        spectatingPlayer: spectatePlayer,
        spectatingCorporation: spectatePlayer?.corporation ?? null,
        spectatePlayerColor,
        onStopSpectating: handleStopSpectating,
        isGameSpectator: isSpectator,
        chatMessages,
        onSendChatMessage: handleSendChatMessage,
        playerColorMap,
        endgameFadeUI,
        isEndgame: isGameComplete,
        activeEndgamePanel: endgamePanel,
        onEndgamePanelChange: handleEndgamePanelChange,
        hasHistory,
        playedCardNotification: playedCardNotification.currentNotification,
        isPlayedCardPinned: playedCardNotification.isPinned,
        onPlayedCardTogglePin: playedCardNotification.togglePin,
        onPlayedCardAdvance: playedCardNotification.advance,
      }
    : null;

  const cardFanTransitionClass = (() => {
    if (spectatePlayerId) {
      return "opacity-0 pointer-events-none";
    }
    if (phase.kind === "animateUI") {
      return "animate-[uiFadeIn_1200ms_ease-out_both]";
    }
    if (
      phase.kind === "loading" ||
      phase.kind === "fadeOutLobby" ||
      phase.kind === "marsRevealed"
    ) {
      return "opacity-0";
    }
    if (phase.kind === "showcase") {
      return "opacity-0 pointer-events-none";
    }
    return "";
  })();

  return (
    <PlanetFocusProvider>
      <VPCountingProvider
        state={vpCounting.state}
        controls={{
          start: vpCounting.start,
          skip: vpCounting.skip,
          goToPhase: vpCounting.goToPhase,
          phases: vpCounting.phases,
        }}
      >
        {shouldShowStartingBackdrop && (
          <div
            className="fixed inset-0 bg-black/60 backdrop-blur-sm animate-[backdropFadeIn_0.3s_ease-out]"
            style={{ zIndex: Z_INDEX.PENDING_ACTION_BACKDROP }}
          />
        )}
        {hasPendingActionSelection && (
          <div
            className="fixed inset-0 bg-black/60 backdrop-blur-sm animate-[backdropFadeIn_0.3s_ease-out]"
            style={{ zIndex: Z_INDEX.SELECTION_BACKDROP }}
          />
        )}

        <style>{`
        @keyframes backdropFadeIn {
          from { opacity: 0; }
          to { opacity: 1; }
        }
        @keyframes fadeOut {
          from { opacity: 1; }
          to { opacity: 0; }
        }
        @keyframes modalFadeIn {
          from { opacity: 0; transform: scale(0.95); }
          to { opacity: 1; transform: scale(1); }
        }
        @keyframes uiFadeIn {
          from { opacity: 0; transform: translateY(20px); }
          to { opacity: 1; transform: translateY(0); }
        }
      `}</style>

        {layoutProps &&
          phase.kind !== "selecting" &&
          phase.kind !== "joining" &&
          phase.kind !== "spectating" &&
          (isCompact ? (
            <MobileGameLayout
              {...layoutProps}
              onPlayCard={handleMobilePlayCard}
              onInspectCard={inspectCard}
              isReplay={replay.isActive}
            />
          ) : (
            <GameLayout ref={playerListRef} {...layoutProps} />
          ))}

        <CardsPlayedModal
          isVisible={showCardsPlayedModal}
          onClose={() => useUIOverlayStore.getState().setShowCardsPlayedModal(false)}
          cards={(spectatePlayer?.playedCards ?? currentPlayer?.playedCards) || []}
        />

        <CardBrowserOverlay
          isVisible={showCardBrowser}
          onClose={() => useUIOverlayStore.getState().setShowCardBrowser(false)}
        />

        <ProductionPhaseModal
          isOpen={showProductionPhaseModal && !isProductionModalHidden}
          gameState={game}
          onClose={() => {
            useUIOverlayStore.getState().setShowProductionPhaseModal(false);
            useUIOverlayStore.getState().setIsProductionModalHidden(false);
            useUIOverlayStore.getState().setOpenProductionToCardSelection(false);
          }}
          onHide={() => {
            useUIOverlayStore.getState().setIsProductionModalHidden(true);
            useUIOverlayStore.getState().setOpenProductionToCardSelection(false);
          }}
          openDirectlyToCardSelection={openProductionToCardSelection}
        />

        <WindowManagerProvider>
          <DebugDropdown
            isVisible={showDebugDropdown}
            onClose={() => useUIOverlayStore.getState().setShowDebugDropdown(false)}
            gameState={game}
            changedPaths={changedPaths}
          />

          <PerformanceWindow
            isVisible={showPerformanceWindow}
            onClose={() => useUIOverlayStore.getState().setShowPerformanceWindow(false)}
          />

          <FeedbackWindow
            isVisible={showFeedbackWindow}
            onClose={() => useUIOverlayStore.getState().setShowFeedbackWindow(false)}
            gameState={game}
          />
        </WindowManagerProvider>

        {lobbyMounted && game && (playerId || isSpectator) && (
          <>
            <WaitingRoomOverlay
              game={game}
              playerId={playerId ?? "spectator"}
              visible={isLobbyPhase}
              onExited={() => useAppPhaseStore.getState().setLobbyMounted(false)}
              chat={
                isLobbyPhase ? (
                  <ChatOverlay
                    messages={chatMessages}
                    onSendMessage={(msg) => void globalWebSocketManager.sendChatMessage(msg)}
                    embedded
                    fill={isCompact}
                    playerColorMap={playerColorMap}
                  />
                ) : null
              }
            />
          </>
        )}

        {showTabConflict && conflictingTabInfo && (
          <TabConflictOverlay
            activeGameInfo={conflictingTabInfo}
            onTakeOver={init.handleTabTakeOver}
            onCancel={init.handleTabCancel}
          />
        )}

        {phase.kind === "selecting" && gameForSelection && (
          <PlayerSelectionOverlay
            game={gameForSelection}
            onSelectPlayer={(pid, playerName) => void init.handlePlayerSelected(pid, playerName)}
            onSpectate={init.handleSpectatorConnected}
            onCancel={init.handlePlayerSelectionCancel}
          />
        )}

        {phase.kind === "joining" && gameForSelection && (
          <JoinGameOverlay game={gameForSelection} onCancel={init.handlePlayerSelectionCancel} />
        )}

        {phase.kind === "spectating" && gameForSelection && (
          <SpectateGameOverlay
            game={gameForSelection}
            onCancel={init.handlePlayerSelectionCancel}
            onConnected={init.handleSpectatorConnected}
          />
        )}

        {showLeaveGameConfirm && (
          <GameMenuModal
            title="Leave game?"
            showBackdrop={true}
            onClose={() => useUIOverlayStore.getState().setShowLeaveGameConfirm(false)}
            zIndex={Z_INDEX.CONFIRMATION_MODAL}
          >
            <p className="text-white/80 text-center mb-6">
              You can reconnect to the game again without losing any progress.
            </p>
            <div className="flex gap-4 justify-center">
              <GameButton
                emphasis="secondary"
                onClick={() => useUIOverlayStore.getState().setShowLeaveGameConfirm(false)}
              >
                Cancel
              </GameButton>
              <GameButton tone="error" onClick={handleConfirmLeaveGame}>
                Leave
              </GameButton>
            </div>
          </GameMenuModal>
        )}

        {showCloseGameConfirm && (
          <GameMenuModal
            title="Close game?"
            showBackdrop={true}
            onClose={() => useUIOverlayStore.getState().setShowCloseGameConfirm(false)}
            zIndex={Z_INDEX.CONFIRMATION_MODAL}
          >
            <p className="text-white/80 text-center mb-6">
              You can reconnect to the game again without losing any progress.
            </p>
            <div className="flex gap-4 justify-center">
              <GameButton
                emphasis="secondary"
                onClick={() => useUIOverlayStore.getState().setShowCloseGameConfirm(false)}
              >
                Cancel
              </GameButton>
              <GameButton tone="error" onClick={handleConfirmLeaveGame}>
                Close
              </GameButton>
            </div>
          </GameMenuModal>
        )}

        {showEndGameConfirm && (
          <GameMenuModal
            title="End game?"
            showBackdrop={true}
            onClose={() => useUIOverlayStore.getState().setShowEndGameConfirm(false)}
            zIndex={Z_INDEX.CONFIRMATION_MODAL}
          >
            <p className="text-white/80 text-center mb-6">
              This will end the game for all players. This action cannot be undone.
            </p>
            <div className="flex gap-4 justify-center">
              <GameButton
                emphasis="secondary"
                onClick={() => useUIOverlayStore.getState().setShowEndGameConfirm(false)}
              >
                Cancel
              </GameButton>
              <GameButton tone="error" onClick={handleConfirmEndGame}>
                End game
              </GameButton>
            </div>
          </GameMenuModal>
        )}

        {showStartingSelection && !isStartingSelectionHidden && game && !isCompact && (
          <MainMenuHamburger
            gameId={game.id}
            onLeaveGame={handleLeaveGame}
            onEndGame={playerId === game.hostPlayerId ? handleEndGame : undefined}
          />
        )}
        {showStartingSelection && (
          <StartingCardSelectionOverlay
            key={`${game?.id}:${currentPlayer?.id}`}
            isOpen={!isStartingSelectionHidden && (marsRevealedReady || phase.kind === "playing")}
            availableCorporations={
              game?.currentPlayer?.selectCorporationPhase?.availableCorporations || []
            }
            availablePreludes={
              game?.currentPlayer?.selectPreludeCardsPhase?.availablePreludes || []
            }
            maxSelectablePreludes={game?.currentPlayer?.selectPreludeCardsPhase?.maxSelectable || 2}
            cards={game?.currentPlayer?.selectStartingCardsPhase?.availableCards || []}
            playerCredits={currentPlayer?.resources?.credits || 40}
            onConfirm={handleStartingChoicesConfirm}
            onHide={() => useUIOverlayStore.getState().setIsStartingSelectionHidden(true)}
          />
        )}

        {showStartingSelection && isStartingSelectionHidden && marsRevealedReady && (
          <GameButton
            className="fixed top-[80px] left-[70%] compact:top-[calc(var(--safe-top)+8px)] compact:left-auto compact:right-[calc(var(--safe-right)+8px)] !py-3.5 !px-7 !text-base !border-space-blue-400 text-shadow-glow shadow-[0_4px_15px_rgba(0,0,0,0.5),0_0_20px_rgba(30,60,150,0.4)] whitespace-nowrap hover:!border-space-blue-500 hover:shadow-[0_6px_20px_rgba(0,0,0,0.6),0_0_35px_rgba(30,60,150,0.6)] active:shadow-[0_2px_10px_rgba(0,0,0,0.4),0_0_20px_rgba(30,60,150,0.4)]"
            style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}
            onClick={() => useUIOverlayStore.getState().setIsStartingSelectionHidden(false)}
          >
            Return to Selection
          </GameButton>
        )}

        {showWaitingForPlayers && game && (
          <>
            {!isCompact && (
              <MainMenuHamburger
                gameId={game.id}
                onLeaveGame={handleLeaveGame}
                onEndGame={playerId === game.hostPlayerId ? handleEndGame : undefined}
              />
            )}
            {isCompact ? (
              <div
                data-testid="mobile-waiting-for-players"
                className="fixed inset-0 flex flex-col text-white bg-[rgba(3,3,4,0.98)] animate-[modalFadeIn_0.3s_ease-out]"
                style={{
                  zIndex: Z_INDEX.CORPORATION_SELECTION,
                  paddingTop: "var(--safe-top)",
                  paddingRight: "var(--safe-right)",
                  paddingBottom: "var(--safe-bottom)",
                  paddingLeft: "var(--safe-left)",
                }}
              >
                <header className="shrink-0 h-11 flex items-center gap-2 pr-3 border-b border-white/15">
                  <CurrentGameMenuButton />
                  <h2 className="m-0 font-orbitron text-[13px] font-bold uppercase tracking-wider text-white">
                    Waiting for players
                  </h2>
                </header>
                <div className="flex-1 min-h-0 overflow-y-auto overscroll-contain p-3">
                  <WaitingPlayerList game={game} />
                </div>
              </div>
            ) : (
              <div
                className="fixed inset-0 flex items-center justify-center"
                style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}
              >
                <div className="w-[450px] max-w-[90vw] game-panel game-panel-clipped game-window p-8 animate-[modalFadeIn_0.3s_ease-out]">
                  <div className="text-center mb-6">
                    <h2 className="font-orbitron text-white text-[24px] m-0 mb-2 text-shadow-glow font-bold tracking-wider">
                      Waiting for players...
                    </h2>
                  </div>
                  <div className="mb-6">
                    <h3 className="font-orbitron text-white text-sm font-semibold mb-2 uppercase tracking-wide">
                      Players
                    </h3>
                    <WaitingPlayerList game={game} />
                  </div>
                </div>
              </div>
            )}
          </>
        )}

        {game?.currentPlayer?.pendingCardSelection && (
          <PendingCardSelectionOverlay
            isOpen={showPendingCardSelection}
            selection={game.currentPlayer.pendingCardSelection}
            playerCredits={
              currentPlayer?.actionCosts
                ?.find((a) => a.actionType === "card-buying")
                ?.costs.find((c) => c.resource === "credit")?.paymentCapacity ?? 0
            }
            onSelectCards={handlePendingCardSelection}
          />
        )}

        {game?.currentPlayer?.cardReceipts?.[0] && !hasBlockingSelection && (
          <CardDrawSelectionOverlay
            mode="received"
            isOpen={showCardDrawSelection}
            receipt={game.currentPlayer.cardReceipts[0]}
            onClose={() => {
              void globalWebSocketManager.acknowledgeCardReceipt(
                game.currentPlayer.cardReceipts[0].id,
              );
            }}
          />
        )}
        {game?.currentPlayer?.pendingCardDrawSelection && (
          <CardDrawSelectionOverlay
            mode="selection"
            isOpen={showCardDrawSelection}
            selection={game.currentPlayer.pendingCardDrawSelection}
            playerCredits={
              currentPlayer?.actionCosts
                ?.find((a) => a.actionType === "card-buying")
                ?.costs.find((c) => c.resource === "credit")?.paymentCapacity ?? 0
            }
            onConfirm={handleCardDrawConfirm}
          />
        )}

        {currentPlayer &&
          (!game?.currentPlayer?.cardReceipts?.length || hasBlockingSelection) &&
          (game?.currentPlayer?.pendingBehaviorResolutions?.length ?? 0) > 0 && (
            <BehaviorResolutionOverlay
              resolutions={game!.currentPlayer!.pendingBehaviorResolutions.filter(
                (resolution) =>
                  resolution.kind !== "optional" || !game?.currentPlayer?.cardReceipts?.length,
              )}
              handCards={currentPlayer.cards || []}
              playedCards={currentPlayer.playedCards || []}
              corporation={currentPlayer.corporation ?? undefined}
            />
          )}

        {game &&
          !isCompact &&
          isInGameWorld(phase) &&
          (currentPlayer || replayViewAsPlayer) &&
          (game.currentPhase !== GamePhaseComplete || !!replayViewAsPlayer) && (
            <SolarSystemFade>
              <div className={`transition-all duration-300 ease-in-out ${cardFanTransitionClass}`}>
                <CardFanOverlay
                  ref={cardFanRef}
                  cards={replayViewAsPlayer?.cards ?? currentPlayer?.cards ?? []}
                  hideWhenModalOpen={hideCardFanForModals}
                  onInspectCard={inspectCard}
                  inspectionStore={inspectionStore}
                  onDismissInspection={clearInspection}
                  onPlayCard={spectatePlayerId ? undefined : flow.handlePlayCard}
                />
              </div>
            </SolarSystemFade>
          )}

        {!isCompact && (
          <CorporationOverlay
            visible={showCorporationOverlay}
            onClose={() => useUIOverlayStore.getState().setShowCorporationOverlay(false)}
            currentPlayer={replayViewAsPlayer ?? currentPlayer}
            otherPlayers={game?.otherPlayers ?? []}
          />
        )}

        {game &&
          playerId &&
          game.currentPhase === GamePhaseComplete &&
          game.status === GameStatusCompleted &&
          game.finalScores &&
          game.finalScores.length > 0 && (
            <>
              <EndGameBottomBar
                game={game}
                playerId={playerId}
                historyEntries={historyEntries}
                activePanel={endgamePanel}
                onPanelChange={handleEndgamePanelChange}
                isReplayActive={replay.isActive}
                replayIndex={replay.currentIndex}
                replayTotal={replay.totalStates}
                replayPlaying={replay.isPlaying}
                replaySpeed={replay.playbackSpeed}
                onReplayPlay={replay.play}
                onReplayPause={replay.pause}
                onReplaySeek={replay.seekTo}
                onReplayStepForward={replay.stepForward}
                onReplayStepBackward={replay.stepBackward}
                onReplaySpeedChange={replay.setPlaybackSpeed}
                replaySpectatePlayerId={replaySpectatePlayerId}
                onReplaySpectatePlayerChange={(id) =>
                  useSpectateStore.getState().setReplaySpectatePlayerId(id)
                }
              />
            </>
          )}

        {cardPendingChoice && (
          <ChoiceSelectionPopover
            cardId={cardPendingChoice.id}
            cardName={cardPendingChoice.name}
            behaviors={cardPendingChoice.behaviors || []}
            behaviorIndex={pendingCardBehaviorIndex}
            onChoiceSelect={flow.handleChoiceSelect}
            onCancel={flow.handleChoiceCancel}
            isVisible={showChoiceSelection}
            playerResources={currentPlayer?.resources}
            resourceStorage={currentPlayer?.resourceStorage}
          />
        )}

        {actionPendingChoice && (
          <ChoiceSelectionPopover
            cardId={actionPendingChoice.cardId}
            cardName={actionPendingChoice.cardName}
            behaviors={[actionPendingChoice.behavior]}
            behaviorIndex={0}
            onChoiceSelect={flow.handleActionChoiceSelect}
            onCancel={flow.handleActionChoiceCancel}
            isVisible={showActionChoiceSelection}
            isAction={true}
            playerResources={currentPlayer?.resources}
            resourceStorage={currentPlayer?.resourceStorage}
          />
        )}

        {pendingActionReuse && currentPlayer?.actions && (
          <ActionReusePopover
            isVisible={showActionReuseSelection}
            onClose={flow.handleActionReuseCancel}
            actions={currentPlayer.actions}
            reuseSourceCardId={pendingActionReuse.cardId}
            onActionSelect={flow.handleActionReuseSelect}
            gameState={game ?? undefined}
          />
        )}

        {pendingCardStorage && (
          <CardStorageSelectionPopover
            resourceType={pendingCardStorage.resourceType}
            amount={pendingCardStorage.amount}
            selectorTags={pendingCardStorage.selectorTags}
            eligibleCardIds={pendingCardStorage.eligibleCardIds}
            playedCards={(() => {
              const played = currentPlayer?.playedCards || [];
              const cardBeingPlayed = currentPlayer?.cards?.find(
                (c) => c.id === pendingCardStorage.cardId,
              );
              if (cardBeingPlayed && !played.some((p) => p.id === cardBeingPlayed.id)) {
                return [...played, cardBeingPlayed as unknown as CardDto];
              }
              return played;
            })()}
            corporationCard={currentPlayer?.corporation}
            resourceStorage={currentPlayer?.resourceStorage}
            onCardSelect={flow.handleCardStorageSelect}
            onCancel={flow.handleCardStorageCancel}
            isVisible={showCardStorageSelection}
          />
        )}

        <PaymentSelectionPopover />
        {playError && (
          <GameFlowPopover
            isVisible
            onClose={() => useCardPlayFlowStore.getState().dismissPlayError()}
            className="text-white max-w-[420px]"
          >
            <GameFlowTitle>
              <h3 className="font-orbitron text-base font-bold">Card not played</h3>
            </GameFlowTitle>
            <GameFlowBody>
              <p className="p-3 text-sm">{playError}</p>
            </GameFlowBody>
            <GameFlowFooter>
              <GameButton onClick={() => useCardPlayFlowStore.getState().dismissPlayError()}>
                OK
              </GameButton>
            </GameFlowFooter>
          </GameFlowPopover>
        )}

        {pendingActionStorage && (
          <CardStorageSelectionPopover
            resourceType={pendingActionStorage.resourceType}
            amount={pendingActionStorage.amount}
            selectorTags={pendingActionStorage.selectorTags}
            eligibleCardIds={pendingActionStorage.eligibleCardIds}
            playedCards={currentPlayer?.playedCards || []}
            corporationCard={currentPlayer?.corporation}
            resourceStorage={currentPlayer?.resourceStorage}
            onCardSelect={flow.handleActionStorageSelect}
            onCancel={flow.handleActionStorageCancel}
            isVisible={showActionStorageSelection}
          />
        )}

        {pendingTargetPlayer && game && (
          <TargetPlayerSelectionPopover
            removalTargets={currentPlayer?.resourceRemovalTargets || []}
            resourceType={pendingTargetPlayer.resourceType}
            amount={pendingTargetPlayer.amount}
            isSteal={pendingTargetPlayer.isSteal}
            players={[
              ...(currentPlayer
                ? [
                    {
                      id: currentPlayer.id,
                      name: currentPlayer.name,
                      resources: currentPlayer.resources,
                      production: currentPlayer.production,
                    },
                  ]
                : []),
              ...(game.otherPlayers || []).map((p) => ({
                id: p.id,
                name: p.name,
                resources: p.resources,
                production: p.production,
              })),
            ]}
            currentPlayerId={currentPlayer?.id}
            onPlayerSelect={flow.handleTargetPlayerSelect}
            onCancel={flow.handleTargetPlayerCancel}
            isVisible={showTargetPlayerSelection}
          />
        )}

        {pendingActionTargetPlayer && game && (
          <TargetPlayerSelectionPopover
            removalTargets={currentPlayer?.resourceRemovalTargets || []}
            resourceType={pendingActionTargetPlayer.resourceType}
            amount={pendingActionTargetPlayer.amount}
            isSteal={pendingActionTargetPlayer.isSteal}
            players={[
              ...(currentPlayer
                ? [
                    {
                      id: currentPlayer.id,
                      name: currentPlayer.name,
                      resources: currentPlayer.resources,
                      production: currentPlayer.production,
                    },
                  ]
                : []),
              ...(game.otherPlayers || []).map((p) => ({
                id: p.id,
                name: p.name,
                resources: p.resources,
                production: p.production,
              })),
            ]}
            currentPlayerId={currentPlayer?.id}
            onPlayerSelect={flow.handleActionTargetPlayerSelect}
            onCancel={flow.handleActionTargetPlayerCancel}
            isVisible={showActionTargetPlayerSelection}
          />
        )}

        {game?.currentPlayer?.pendingResourceRemovalSelection && (
          <ResourceRemovalOverlay
            key={game.currentPlayer.pendingResourceRemovalSelection.id}
            selection={game.currentPlayer.pendingResourceRemovalSelection}
            currentPlayerId={game.currentPlayer.id}
            players={[game.currentPlayer, ...(game.otherPlayers ?? [])]}
          />
        )}

        {game?.currentPlayer?.pendingColonyResourceSelection && currentPlayer && (
          <CardStorageSelectionPopover
            resourceType={
              game.currentPlayer.pendingColonyResourceSelection.resourceType as ResourceType
            }
            amount={game.currentPlayer.pendingColonyResourceSelection.amount}
            reason={game.currentPlayer.pendingColonyResourceSelection.reason}
            mandatory
            playedCards={currentPlayer.playedCards || []}
            corporationCard={currentPlayer.corporation}
            resourceStorage={currentPlayer.resourceStorage}
            onCardSelect={flow.handleColonyResourceSelect}
            onCancel={flow.handleColonyResourceSkip}
            isVisible={showColonyResourceSelection}
          />
        )}

        {game?.currentPlayer?.pendingColonySelection && game && (
          <ColonySelectionOverlay
            isOpen={showColonyPlacementSelection}
            pendingSelection={game.currentPlayer.pendingColonySelection}
            colonies={
              game.currentPlayer.pendingColonySelection.addTile
                ? (game.currentPlayer.pendingColonySelection.tileOptions ?? [])
                : (game.colonies ?? [])
            }
            allPlayers={colonyAllPlayers}
            onConfirm={(colonyId) => void globalWebSocketManager.confirmColonyPlacement(colonyId)}
          />
        )}

        {game?.currentPlayer?.pendingCardReveal && (
          <CardRevealOverlay
            reveal={game.currentPlayer.pendingCardReveal}
            onConfirm={() => globalWebSocketManager.confirmCardReveal()}
          />
        )}
        {game?.currentPlayer?.pendingEffectSelection && (
          <EffectSelectionOverlay
            key={game.currentPlayer.pendingEffectSelection.sourceCardId}
            selection={game.currentPlayer.pendingEffectSelection}
            cards={[
              ...game.currentPlayer.playedCards,
              ...(game.currentPlayer.corporation ? [game.currentPlayer.corporation] : []),
            ]}
            colonies={game.colonies ?? []}
            players={[game.currentPlayer, ...game.otherPlayers]}
            onConfirm={(optionIndex) => globalWebSocketManager.confirmEffectSelection(optionIndex)}
          />
        )}
        {game?.currentPlayer?.pendingFreeTradeSelection && game && (
          <FreeTradeSelectionOverlay
            isOpen={showFreeTradeSelection}
            pendingSelection={game.currentPlayer.pendingFreeTradeSelection}
            colonies={game.colonies ?? []}
            viewingPlayerId={game.viewingPlayerId ?? ""}
            tradeFleets={game.tradeFleets ?? {}}
            allPlayers={colonyAllPlayers}
            playedCards={currentPlayer?.playedCards ?? []}
            corporation={currentPlayer?.corporation}
            onConfirm={(colonyId, trackSteps) =>
              void globalWebSocketManager.confirmFreeTrade(colonyId, trackSteps)
            }
          />
        )}

        {showFreeTradeWarning && pendingFreeTradeWarning && (
          <GameFlowPopover
            isVisible={true}
            onClose={() => {
              useCardPlayFlowStore.getState().setShowFreeTradeWarning(false);
              useCardPlayFlowStore.getState().setPendingFreeTradeWarning(null);
            }}
            type="interactive"
          >
            <GameFlowTitle>
              <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
                Cannot Trade
              </h3>
            </GameFlowTitle>
            <GameFlowBody>
              <div className="flex flex-col items-center justify-center py-6 px-4 text-center">
                <p className="text-white/70 text-sm">{pendingFreeTradeWarning}</p>
              </div>
            </GameFlowBody>
            <GameFlowFooter>
              <GameButton
                size="sm"
                onClick={() => {
                  useCardPlayFlowStore.getState().setShowFreeTradeWarning(false);
                  useCardPlayFlowStore.getState().setPendingFreeTradeWarning(null);
                }}
              >
                OK
              </GameButton>
            </GameFlowFooter>
          </GameFlowPopover>
        )}

        {phase.kind === "showcase" && game?.initPhase && (
          <CorporationShowcaseOverlay game={game} initPhase={game.initPhase} />
        )}

        {game?.currentPlayer?.pendingAwardFundSelection && (
          <AwardFundSelectionPopover
            isOpen={true}
            selection={game.currentPlayer.pendingAwardFundSelection}
            gameState={game}
          />
        )}

        {pendingCardResourceInput && (
          <CardResourceSelectionPopover
            resourceType={pendingCardResourceInput.resourceType}
            amount={pendingCardResourceInput.amount}
            selection={
              pendingCardResourceInput.eligibleInputCardIds === undefined
                ? { kind: "remove" }
                : {
                    kind: "spend",
                    playerId: currentPlayer?.id ?? "",
                    eligibleCardIds: pendingCardResourceInput.eligibleInputCardIds,
                  }
            }
            removalTargets={currentPlayer?.resourceRemovalTargets ?? []}
            optional={
              pendingCardResourceInput.type === "play-card" &&
              pendingCardResourceInput.eligibleInputCardIds === undefined
            }
            players={[
              ...(currentPlayer
                ? [
                    {
                      id: currentPlayer.id,
                      name: currentPlayer.name,
                      playedCards: [
                        ...currentPlayer.playedCards,
                        ...(currentPlayer.corporation ? [currentPlayer.corporation] : []),
                      ],
                      resourceStorage: currentPlayer.resourceStorage,
                    },
                  ]
                : []),
              ...(game?.otherPlayers || []).map((p) => ({
                id: p.id,
                name: p.name,
                playedCards: [...p.playedCards, ...(p.corporation ? [p.corporation] : [])],
                resourceStorage: p.resourceStorage,
              })),
            ]}
            onCardSelect={flow.handleCardResourceSelect}
            onCancel={flow.handleCardResourceCancel}
            isVisible={showCardResourceSelection}
          />
        )}

        {pendingVariableAmount && (
          <AmountSelectionPopover
            resourceType={pendingVariableAmount.resourceType}
            maxAmount={pendingVariableAmount.maxAmount}
            onAmountSelect={flow.handleAmountSelect}
            onCancel={flow.handleAmountCancel}
            isVisible={showAmountSelection}
          />
        )}

        {showProductionPhaseModal && isProductionModalHidden && (
          <GameButton
            className="fixed top-[80px] left-[70%] compact:top-[calc(var(--hud-top-h)+var(--safe-top)+8px)] compact:left-auto compact:right-[calc(var(--hud-rail-right-w)+var(--safe-right)+8px)] !py-3.5 !px-7 !text-base !border-space-blue-400 text-shadow-glow shadow-[0_4px_15px_rgba(0,0,0,0.5),0_0_20px_rgba(30,60,150,0.4)] whitespace-nowrap hover:!border-space-blue-500 hover:shadow-[0_6px_20px_rgba(0,0,0,0.6),0_0_35px_rgba(30,60,150,0.6)] active:shadow-[0_2px_10px_rgba(0,0,0,0.4),0_0_20px_rgba(30,60,150,0.4)]"
            style={{ zIndex: Z_INDEX.CORPORATION_SELECTION }}
            onClick={() => {
              useUIOverlayStore.getState().setIsProductionModalHidden(false);
              useUIOverlayStore.getState().setOpenProductionToCardSelection(true);
            }}
          >
            Return to Production
          </GameButton>
        )}

        <GameEventBanner
          event={endgamePanel === "score" ? currentEvent : null}
          onDismiss={dismissGameEvent}
        />

        <LoadingOverlay
          isLoaded={isFullyLoaded}
          fadeDurationMs={1500}
          message={loadingMessage}
          subtitle={loadingSubtitle}
          onTransitionEnd={handleLoadingTransitionEnd}
          showDelayMs={loadingShowDelayMs}
          minDurationMs={200}
          showProgress
        />
      </VPCountingProvider>
    </PlanetFocusProvider>
  );
}
