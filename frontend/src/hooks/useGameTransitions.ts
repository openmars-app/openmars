import { useEffect, useRef } from "react";
import { useEngineStore } from "@/stores/engineStore";
import { isGameSceneReady } from "@/engine/status";
import { useGameStore } from "@/stores/gameStore.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import { useAppPhaseStore, gameIdOf, isInGameWorld } from "@/stores/appPhaseStore.ts";
import { audioService } from "@/services/audioService.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { SHOWCASE_PLAY_STEP_MS } from "@/constants/gameConstants.ts";
import { isShowcaseStepEnd, showcaseControllerId } from "@/utils/showcase.ts";
import { useShowcaseTileHold } from "@/hooks/useShowcaseTileHold.ts";
import {
  GamePhaseInitApplyCorp,
  GamePhaseInitApplyPrelude,
  GamePhaseStartingSelection,
  GameStatusActive,
  GameStatusCompleted,
  GameStatusLobby,
} from "@/types/generated/api-types.ts";

export function useGameTransitions(
  playProductionSound: () => Promise<void>,
  notificationQueueDoneAt: React.RefObject<number>,
): void {
  const gamePhase = useGameStore((s) => s.game?.currentPhase);
  const gameStatus = useGameStore((s) => s.game?.status);
  const gameId = useGameStore((s) => s.game?.id);
  const mapId = useGameStore((s) => s.game?.settings.mapId);
  const isConnected = useGameStore((s) => s.isConnected);
  const corpData = useGameStore((s) => s.currentPlayer?.corporation);
  const productionPhase = useGameStore((s) => s.currentPlayer?.productionPhase);
  const selectCorpPhase = useGameStore((s) => s.game?.currentPlayer?.selectCorporationPhase);
  const pendingCardSelection = useGameStore((s) => s.game?.currentPlayer?.pendingCardSelection);
  const hasCardReceipts = useGameStore(
    (s) => (s.game?.currentPlayer?.cardReceipts?.length ?? 0) > 0,
  );
  const pendingCardDrawSelection = useGameStore(
    (s) => s.game?.currentPlayer?.pendingCardDrawSelection,
  );
  const pendingResourceRemoval = useGameStore(
    (s) => s.game?.currentPlayer?.pendingResourceRemovalSelection,
  );
  const pendingColonyResource = useGameStore(
    (s) => s.game?.currentPlayer?.pendingColonyResourceSelection,
  );
  const pendingColonyPlacement = useGameStore((s) => s.game?.currentPlayer?.pendingColonySelection);
  const pendingFreeTrade = useGameStore((s) => s.game?.currentPlayer?.pendingFreeTradeSelection);
  const initPhase = useGameStore((s) => s.game?.initPhase);
  const hostPlayerId = useGameStore((s) => s.game?.hostPlayerId);
  const showcaseTileHold = useShowcaseTileHold(initPhase?.hasPendingSelection ?? false);
  const playersConnectedKey = useGameStore((s) =>
    [s.game?.currentPlayer, ...(s.game?.otherPlayers ?? [])]
      .map((p) => (p?.isConnected ? "1" : "0"))
      .join(""),
  );
  const currentPlayerId = useGameStore((s) => s.currentPlayer?.id);

  const phase = useAppPhaseStore((s) => s.phase);
  const isEngineReady = useEngineStore((s) => isGameSceneReady(s.status, gameId, mapId));
  const isCurrentGame = gameId !== undefined && gameIdOf(phase) === gameId;

  const isLobbyPhase = isCurrentGame && gameStatus === GameStatusLobby;

  const isStartingSelectionPhase =
    isCurrentGame && gameStatus === GameStatusActive && gamePhase === GamePhaseStartingSelection;

  const isInitApplyPhase =
    isCurrentGame &&
    gameStatus === GameStatusActive &&
    (gamePhase === GamePhaseInitApplyCorp || gamePhase === GamePhaseInitApplyPrelude);

  const lobbyGameId = useRef<string | null>(null);
  const isInitialMount = useRef(true);
  const loadingEnteredAt = useRef<number | null>(null);

  useEffect(() => {
    window.history.pushState(null, "", window.location.href);
    const handlePopState = () => {
      window.history.pushState(null, "", window.location.href);
      const { isConnected, game } = useGameStore.getState();
      if (isConnected && game) {
        useUIOverlayStore.getState().setShowCloseGameConfirm(true);
      }
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  useEffect(() => {
    const { isConnected, game } = useGameStore.getState();
    if (!isConnected || !game) {
      return;
    }
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => window.removeEventListener("beforeunload", handleBeforeUnload);
  }, [isConnected, gamePhase]);

  useEffect(() => {
    const { setCorporationData } = useGameStore.getState();
    if (corpData) {
      setCorporationData(corpData);
    } else {
      setCorporationData(null);
    }
  }, [corpData]);

  useEffect(() => {
    const { setDisplayedInitPlayerId } = useGameStore.getState();
    const initPlayerId = initPhase?.currentPlayerId ?? null;
    if (gamePhase !== GamePhaseInitApplyCorp && gamePhase !== GamePhaseInitApplyPrelude) {
      setDisplayedInitPlayerId(null);
      return;
    }
    const now = Date.now();
    const queueRemaining = Math.max(0, notificationQueueDoneAt.current - now);
    if (queueRemaining === 0) {
      setDisplayedInitPlayerId(initPlayerId);
      return;
    }
    const timer = setTimeout(() => {
      setDisplayedInitPlayerId(initPlayerId);
    }, queueRemaining);
    return () => clearTimeout(timer);
  }, [gamePhase, initPhase?.currentPlayerId, notificationQueueDoneAt]);

  useEffect(() => {
    const {
      showProductionPhaseModal,
      setShowProductionPhaseModal,
      setOpenProductionToCardSelection,
    } = useUIOverlayStore.getState();
    const currentPlayer = useGameStore.getState().currentPlayer;

    const hasProductionData =
      currentPlayer?.productionPhase &&
      !currentPlayer.productionPhase.selectionComplete &&
      currentPlayer.productionPhase.availableCards;

    if (hasProductionData && !showProductionPhaseModal) {
      if (!isInitialMount.current) {
        void playProductionSound();
      }
      setShowProductionPhaseModal(true);
      setOpenProductionToCardSelection(false);
    } else if (!hasProductionData && showProductionPhaseModal) {
      setShowProductionPhaseModal(false);
    }

    if (isInitialMount.current) {
      isInitialMount.current = false;
    }
  }, [productionPhase, playProductionSound]);

  useEffect(() => {
    const { showStartingSelection, setShowStartingSelection, setIsStartingSelectionHidden } =
      useUIOverlayStore.getState();

    const hasStartingData =
      gamePhase === GamePhaseStartingSelection &&
      gameStatus === GameStatusActive &&
      selectCorpPhase &&
      selectCorpPhase.availableCorporations.length > 0;

    if (hasStartingData && !showStartingSelection) {
      setShowStartingSelection(true);
    } else if (showStartingSelection && !selectCorpPhase) {
      setShowStartingSelection(false);
      setIsStartingSelectionHidden(false);
    }
  }, [gamePhase, gameStatus, selectCorpPhase]);

  useEffect(() => {
    const { showPendingCardSelection, setShowPendingCardSelection } = useUIOverlayStore.getState();

    if (pendingCardSelection && !showPendingCardSelection) {
      setShowPendingCardSelection(true);
    } else if (!pendingCardSelection && showPendingCardSelection) {
      setShowPendingCardSelection(false);
    }
  }, [pendingCardSelection]);

  useEffect(() => {
    const { showCardDrawSelection, setShowCardDrawSelection } = useUIOverlayStore.getState();

    if ((pendingCardDrawSelection || hasCardReceipts) && !showCardDrawSelection) {
      setShowCardDrawSelection(true);
    } else if (!pendingCardDrawSelection && !hasCardReceipts && showCardDrawSelection) {
      setShowCardDrawSelection(false);
    }
  }, [pendingCardDrawSelection, hasCardReceipts]);

  useEffect(() => {
    const { showResourceRemovalSelection, setShowResourceRemovalSelection } =
      useUIOverlayStore.getState();

    if (pendingResourceRemoval && !showResourceRemovalSelection) {
      setShowResourceRemovalSelection(true);
    } else if (!pendingResourceRemoval && showResourceRemovalSelection) {
      setShowResourceRemovalSelection(false);
    }
  }, [pendingResourceRemoval]);

  useEffect(() => {
    const { showColonyResourceSelection, setShowColonyResourceSelection } =
      useUIOverlayStore.getState();

    if (pendingColonyResource && !showColonyResourceSelection) {
      setShowColonyResourceSelection(true);
    } else if (!pendingColonyResource && showColonyResourceSelection) {
      setShowColonyResourceSelection(false);
    }
  }, [pendingColonyResource]);

  useEffect(() => {
    const { showColonyPlacementSelection, setShowColonyPlacementSelection } =
      useUIOverlayStore.getState();

    if (pendingColonyPlacement && !showColonyPlacementSelection) {
      setShowColonyPlacementSelection(true);
    } else if (!pendingColonyPlacement && showColonyPlacementSelection) {
      setShowColonyPlacementSelection(false);
    }
  }, [pendingColonyPlacement]);

  useEffect(() => {
    const { showFreeTradeSelection, setShowFreeTradeSelection } = useUIOverlayStore.getState();

    if (pendingFreeTrade && !showFreeTradeSelection) {
      setShowFreeTradeSelection(true);
    } else if (!pendingFreeTrade && showFreeTradeSelection) {
      setShowFreeTradeSelection(false);
    }
  }, [pendingFreeTrade]);

  useEffect(() => {
    if (isLobbyPhase) {
      useAppPhaseStore.getState().setLobbyMounted(true);
    }
  }, [isLobbyPhase]);

  useEffect(() => {
    const { setPhase } = useAppPhaseStore.getState();
    const activeGameId = gameId ?? gameIdOf(phase);
    if (!activeGameId || !isCurrentGame) {
      return;
    }

    if (isLobbyPhase) {
      if (phase.kind !== "lobby") {
        setPhase({ kind: "lobby", gameId: activeGameId });
      }
      lobbyGameId.current = activeGameId;
      return;
    }

    if (lobbyGameId.current !== activeGameId || phase.kind !== "lobby") {
      return;
    }

    setPhase({ kind: "loading", gameId: activeGameId });
  }, [isLobbyPhase, gameId, phase, isCurrentGame]);

  useEffect(() => {
    if (phase.kind !== "loading") {
      loadingEnteredAt.current = null;
      return;
    }
    if (loadingEnteredAt.current === null) {
      loadingEnteredAt.current = Date.now();
    }
    if (!isEngineReady) {
      return;
    }
    const activeGameId = phase.gameId;
    const elapsed = Date.now() - loadingEnteredAt.current;
    const remaining = Math.max(0, 1000 - elapsed) + 1000;
    const advance = () => {
      useAppPhaseStore.getState().setPhase({ kind: "fadeOutLobby", gameId: activeGameId });
    };
    if (remaining === 0) {
      advance();
      return;
    }
    const timer = window.setTimeout(advance, remaining);
    return () => window.clearTimeout(timer);
  }, [phase, isEngineReady]);

  useEffect(() => {
    const { setPhase } = useAppPhaseStore.getState();
    if (phase.kind === "fadeOutLobby") {
      const timer = setTimeout(
        () => setPhase({ kind: "marsRevealed", gameId: phase.gameId }),
        2500,
      );
      return () => clearTimeout(timer);
    }
    if (phase.kind === "animateUI") {
      const timer = setTimeout(() => setPhase({ kind: "playing", gameId: phase.gameId }), 2500);
      return () => clearTimeout(timer);
    }
    return undefined;
  }, [phase]);

  useEffect(() => {
    if (phase.kind === "marsRevealed" && !isStartingSelectionPhase) {
      useAppPhaseStore
        .getState()
        .setPhase({ kind: isInitApplyPhase ? "showcase" : "animateUI", gameId: phase.gameId });
    }
  }, [phase, isStartingSelectionPhase, isInitApplyPhase]);

  useEffect(() => {
    if (!isStartingSelectionPhase) {
      return;
    }
    if (
      phase.kind === "showcase" ||
      phase.kind === "animateUI" ||
      phase.kind === "playing" ||
      phase.kind === "completed"
    ) {
      useAppPhaseStore.getState().setPhase({ kind: "marsRevealed", gameId: phase.gameId });
    }
  }, [phase, isStartingSelectionPhase]);

  useEffect(() => {
    if (phase.kind === "showcase" && !isInitApplyPhase) {
      useAppPhaseStore.getState().setPhase({ kind: "animateUI", gameId: phase.gameId });
    }
  }, [phase, isInitApplyPhase]);

  useEffect(() => {
    const { setMarsRevealedReady } = useAppPhaseStore.getState();

    if (phase.kind === "marsRevealed") {
      const timer = setTimeout(() => setMarsRevealedReady(true), 2000);
      return () => clearTimeout(timer);
    }
    setMarsRevealedReady(false);
    return undefined;
  }, [phase]);

  const entranceActive = phase.kind === "fadeOutLobby" || isInGameWorld(phase);

  useEffect(() => {
    if (!entranceActive) {
      return;
    }
    if (useAppPhaseStore.getState().phase.kind !== "fadeOutLobby") {
      audioService.playAmbient();
      return;
    }

    audioService.stopAmbientWithDuration(1500);
    const entranceSound = audioService.playSoundWithHandle("game-start");
    let soundFinished = entranceSound === null;
    let animationFinished = false;
    let musicStarted = false;
    const resumeMusic = () => {
      if (soundFinished && animationFinished && !musicStarted) {
        musicStarted = true;
        audioService.playNextAmbient(3000);
      }
    };
    const finishSound = () => {
      soundFinished = true;
      resumeMusic();
    };
    entranceSound?.addEventListener("ended", finishSound);
    entranceSound?.addEventListener("error", finishSound);
    const timer = window.setTimeout(() => {
      animationFinished = true;
      resumeMusic();
    }, 4500);

    return () => {
      window.clearTimeout(timer);
      entranceSound?.removeEventListener("ended", finishSound);
      entranceSound?.removeEventListener("error", finishSound);
      entranceSound?.pause();
    };
  }, [entranceActive, gameId]);

  useEffect(() => {
    const activeGameId = gameId ?? gameIdOf(phase);
    if (!activeGameId) {
      return;
    }
    if (
      isStartingSelectionPhase &&
      (phase.kind === "checking" ||
        phase.kind === "connecting" ||
        phase.kind === "selecting" ||
        phase.kind === "joining" ||
        phase.kind === "spectating") &&
      lobbyGameId.current !== activeGameId &&
      isEngineReady
    ) {
      useAppPhaseStore.getState().setPhase({ kind: "marsRevealed", gameId: activeGameId });
    }
  }, [isStartingSelectionPhase, phase, isEngineReady, gameId]);

  useEffect(() => {
    if (!isCurrentGame || gameStatus !== GameStatusActive) {
      return;
    }
    if (isLobbyPhase || isStartingSelectionPhase) {
      return;
    }
    if (!isEngineReady) {
      return;
    }
    if (lobbyGameId.current === gameId) {
      return;
    }
    const activeGameId = gameId ?? gameIdOf(phase);
    if (!activeGameId) {
      return;
    }
    if (
      phase.kind === "checking" ||
      phase.kind === "connecting" ||
      phase.kind === "selecting" ||
      phase.kind === "joining" ||
      phase.kind === "spectating"
    ) {
      useAppPhaseStore
        .getState()
        .setPhase({ kind: isInitApplyPhase ? "showcase" : "playing", gameId: activeGameId });
    }
  }, [
    gameStatus,
    isLobbyPhase,
    isStartingSelectionPhase,
    isInitApplyPhase,
    isEngineReady,
    isCurrentGame,
    gameId,
    phase,
  ]);

  useEffect(() => {
    if (!isCurrentGame || gameStatus !== GameStatusCompleted) {
      return;
    }
    if (!isEngineReady) {
      return;
    }
    const activeGameId = gameId ?? gameIdOf(phase);
    if (!activeGameId) {
      return;
    }
    if (phase.kind !== "completed") {
      useAppPhaseStore.getState().setPhase({ kind: "completed", gameId: activeGameId });
    }
  }, [gameStatus, gameId, phase, isCurrentGame, isEngineReady]);

  useEffect(() => {
    if (!isInitApplyPhase || !initPhase?.waitingForConfirm) {
      return;
    }
    const game = useGameStore.getState().game;
    if (!game || showcaseControllerId(game) !== currentPlayerId) {
      return;
    }
    if (isShowcaseStepEnd(game)) {
      return;
    }

    const animationDone =
      phase.kind === "showcase" || phase.kind === "playing" || phase.kind === "completed";
    if (!animationDone) {
      return;
    }

    if (initPhase.hasPendingSelection || showcaseTileHold) {
      return;
    }

    const timer = setTimeout(() => {
      void globalWebSocketManager.confirmInitAdvance();
    }, SHOWCASE_PLAY_STEP_MS);

    return () => clearTimeout(timer);
  }, [
    isInitApplyPhase,
    initPhase?.waitingForConfirm,
    initPhase?.confirmVersion,
    initPhase?.currentPlayerIndex,
    initPhase?.hasPendingSelection,
    showcaseTileHold,
    initPhase?.stage,
    initPhase?.preludesPlayed,
    initPhase?.preludes.length,
    gamePhase,
    hostPlayerId,
    playersConnectedKey,
    currentPlayerId,
    phase,
  ]);
}
