import { useCallback, useLayoutEffect } from "react";
import type { Location, NavigateFunction } from "react-router-dom";
import { useGameStore } from "@/stores/gameStore.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import { useAppPhaseStore } from "@/stores/appPhaseStore.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { apiService } from "@/services/apiService.ts";
import { getTabManager } from "@/utils/tabManager.ts";
import { clearGameSession, getGameSession, saveGameSession } from "@/utils/sessionStorage.ts";
import type { GameDto } from "@/types/generated/api-types.ts";

function setInitPhase(
  kind: "checking" | "connecting" | "selecting" | "joining" | "spectating",
  gameId: string,
) {
  if (kind === "checking") {
    useAppPhaseStore.setState({
      phase: { kind, gameId },
      marsRevealedReady: false,
      lobbyMounted: false,
    });
    return;
  }
  useAppPhaseStore.getState().setPhase({ kind, gameId });
}

interface UseGameInitializationParams {
  navigate: NavigateFunction;
  location: Location;
  urlGameId: string | undefined;
}

export function useGameInitialization({
  navigate,
  location,
  urlGameId,
}: UseGameInitializationParams) {
  const handleTabTakeOver = useCallback(() => {
    const { conflictingTabInfo } = useUIOverlayStore.getState();
    if (!conflictingTabInfo) {
      return;
    }

    const tabManager = getTabManager();
    tabManager.forceTakeOver(conflictingTabInfo.gameId, conflictingTabInfo.playerName);
    useUIOverlayStore.getState().setShowTabConflict(false);
    useUIOverlayStore.getState().setConflictingTabInfo(null);

    const routeState = location.state as {
      game?: GameDto;
      playerId?: string;
      playerName?: string;
    } | null;

    if (routeState?.game && routeState?.playerId && routeState?.playerName) {
      const { setGame, setIsConnected, setCurrentPlayer, setPlayerId } = useGameStore.getState();

      setGame(routeState.game);
      setIsConnected(true);

      saveGameSession({
        gameId: routeState.game.id,
        playerId: routeState.playerId,
        playerName: routeState.playerName,
        timestamp: Date.now(),
      });

      const player = routeState.game.currentPlayer;
      setCurrentPlayer(player || null);

      setPlayerId(routeState.playerId);
    }
  }, [location.state]);

  const handleTabCancel = useCallback(() => {
    useUIOverlayStore.getState().setShowTabConflict(false);
    useUIOverlayStore.getState().setConflictingTabInfo(null);
    navigate("/", { replace: true });
  }, [navigate]);

  const handlePlayerSelected = useCallback(async (selectedPlayerId: string, playerName: string) => {
    const { gameForSelection } = useGameStore.getState();
    if (!gameForSelection) {
      return;
    }

    setInitPhase("connecting", gameForSelection.id);

    const tabManager = getTabManager();
    const canClaim = await tabManager.claimTab(gameForSelection.id, playerName);

    if (!canClaim) {
      const activeTabInfo = tabManager.getActiveTabInfo();
      if (activeTabInfo) {
        useUIOverlayStore.getState().setConflictingTabInfo(activeTabInfo);
        useUIOverlayStore.getState().setShowTabConflict(true);
        return;
      }
    }

    saveGameSession({
      gameId: gameForSelection.id,
      playerId: selectedPlayerId,
      playerName: playerName,
      timestamp: Date.now(),
    });

    useGameStore.getState().setPlayerId(selectedPlayerId);
    globalWebSocketManager.setCurrentPlayerId(selectedPlayerId);

    await globalWebSocketManager.playerTakeover(selectedPlayerId, gameForSelection.id);
  }, []);

  const handlePlayerSelectionCancel = useCallback(() => {
    navigate("/", { replace: true });
  }, [navigate]);

  const handleSpectatorConnected = useCallback(() => {
    const store = useGameStore.getState();
    store.setIsSpectator(true);
    store.setIsConnected(true);
  }, []);

  useLayoutEffect(() => {
    let aborted = false;

    const initializeGame = async () => {
      const {
        setIsSpectator,
        setIsConnected,
        setGame,
        setCurrentPlayer,
        setPlayerId,
        setGameForSelection,
      } = useGameStore.getState();

      const routeState = location.state as {
        game?: GameDto;
        playerId?: string;
        playerName?: string;
        isReconnection?: boolean;
        spectatorName?: string;
      } | null;

      const savedSession = getGameSession();
      const gameId = urlGameId || routeState?.game?.id || savedSession?.gameId;

      if (!gameId) {
        navigate("/", { replace: true });
        return;
      }

      setInitPhase("checking", gameId);

      let fetchedGame: GameDto | null = null;
      try {
        const playerIdForApi =
          savedSession?.gameId === gameId && !savedSession.isSpectator
            ? savedSession.playerId
            : undefined;
        fetchedGame = await apiService.getGame(gameId, playerIdForApi);
      } catch {
        navigate("/", {
          replace: true,
          state: { error: "Could not find game" },
        });
        return;
      }

      if (aborted) {
        return;
      }

      if (!fetchedGame) {
        clearGameSession();
        navigate("/", {
          replace: true,
          state: { error: "Could not find game" },
        });
        return;
      }

      if (fetchedGame.resumeLobby) {
        navigate(`/resume/${gameId}`, { replace: true });
        return;
      }

      const cachedForThisGame = savedSession && savedSession.gameId === gameId;

      if (cachedForThisGame && savedSession.isSpectator) {
        setIsSpectator(true);
        setIsConnected(true);
        await globalWebSocketManager.spectatorConnect(savedSession.playerName, gameId);
        return;
      }

      if (cachedForThisGame && savedSession.playerId) {
        const allPlayers = [fetchedGame.currentPlayer, ...(fetchedGame.otherPlayers || [])].filter(
          Boolean,
        );

        const cachedPlayer = allPlayers.find((p) => p?.id === savedSession.playerId);

        if (cachedPlayer) {
          if (!cachedPlayer.isConnected) {
            setInitPhase("connecting", fetchedGame.id);

            const tabManager = getTabManager();
            const canClaim = await tabManager.claimTab(fetchedGame.id, savedSession.playerName);
            if (aborted) {
              return;
            }

            if (!canClaim) {
              const activeTabInfo = tabManager.getActiveTabInfo();
              if (activeTabInfo) {
                useUIOverlayStore.getState().setConflictingTabInfo(activeTabInfo);
                useUIOverlayStore.getState().setShowTabConflict(true);
                return;
              }
            }

            setPlayerId(savedSession.playerId);
            globalWebSocketManager.setCurrentPlayerId(savedSession.playerId);

            await globalWebSocketManager.playerTakeover(savedSession.playerId, fetchedGame.id);
            if (aborted) {
              return;
            }
            return;
          }
        }
      }

      if (routeState?.game && routeState?.playerId && routeState?.playerName) {
        const tabManager = getTabManager();
        const canClaim = await tabManager.claimTab(routeState.game.id, routeState.playerName);
        if (aborted) {
          return;
        }

        if (!canClaim) {
          const activeTabInfo = tabManager.getActiveTabInfo();
          if (activeTabInfo) {
            useUIOverlayStore.getState().setConflictingTabInfo(activeTabInfo);
            useUIOverlayStore.getState().setShowTabConflict(true);
            return;
          }
        }

        setGame(routeState.game);
        setIsConnected(true);

        saveGameSession({
          gameId: routeState.game.id,
          playerId: routeState.playerId,
          playerName: routeState.playerName,
          timestamp: Date.now(),
        });

        const player = routeState.game.currentPlayer;
        setCurrentPlayer(player || null);

        setPlayerId(routeState.playerId);

        globalWebSocketManager.setCurrentPlayerId(routeState.playerId);

        void globalWebSocketManager.playerConnect(
          routeState.playerName,
          routeState.game.id,
          routeState.playerId,
        );
        return;
      }

      if (aborted) {
        return;
      }

      if (routeState?.spectatorName) {
        setIsSpectator(true);
        setIsConnected(true);
        saveGameSession({
          gameId,
          playerId: "",
          playerName: routeState.spectatorName,
          isSpectator: true,
        });
        await globalWebSocketManager.spectatorConnect(routeState.spectatorName, gameId);
        return;
      }

      const urlParams = new URLSearchParams(window.location.search);
      const linkType = urlParams.get("type");

      if (linkType === "spectate") {
        setGameForSelection(fetchedGame);
        setInitPhase("spectating", fetchedGame.id);
        return;
      }

      if (linkType === "join") {
        setGameForSelection(fetchedGame);
        setInitPhase("joining", fetchedGame.id);
        return;
      }

      setGameForSelection(fetchedGame);
      setInitPhase("selecting", fetchedGame.id);
    };

    void initializeGame();
    return () => {
      aborted = true;
    };
  }, [location.state, navigate, urlGameId]);

  return {
    handlePlayerSelected,
    handlePlayerSelectionCancel,
    handleSpectatorConnected,
    handleTabTakeOver,
    handleTabCancel,
  };
}
