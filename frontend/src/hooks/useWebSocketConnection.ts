import { engineProfiler } from "../engine/profiler";
import { useEffect, useRef } from "react";
import type { NavigateFunction } from "react-router-dom";
import type { MutableRefObject } from "react";
import { config } from "@/config.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { useGameStore } from "@/stores/gameStore.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import { useBotPresenceStore } from "@/stores/botPresenceStore.ts";
import { useSoundEffects } from "@/hooks/useSoundEffects.ts";
import { deepClone, findChangedPaths } from "@/utils/deepCompare.ts";
import { clearGameSession, getGameSession, saveGameSession } from "@/utils/sessionStorage.ts";
import { gatewayServer } from "@/utils/gateway.ts";
import type {
  BotThoughtPayload,
  ChatMessageDto,
  EmotePayload,
  GameDto,
  LogUpdatePayload,
  OtherPlayerDto,
} from "@/types/generated/api-types.ts";
import {
  GamePhaseInitApplyCorp,
  GamePhaseInitApplyPrelude,
  GameStatusLobby,
} from "@/types/generated/api-types.ts";
import type { PlayedCardNotification } from "@/hooks/usePlayedCardNotification.ts";

// Bots count as joined once their session is ready, not when they are added.
function isPresent(player: OtherPlayerDto): boolean {
  return player.playerType === "bot" ? player.botStatus === "ready" : player.isConnected;
}

type PlayedCardNotificationInput = Omit<PlayedCardNotification, "id">;

interface ShowNotificationOptions {
  message: string;
  type: "error" | "warning" | "info";
  duration?: number;
}

export function useWebSocketConnection(
  navigate: NavigateFunction,
  enqueuePlayedCard: (input: PlayedCardNotificationInput) => void,
  notificationQueueDoneAt: MutableRefObject<number>,
  showNotification: (options: ShowNotificationOptions) => string,
): { attemptReconnection: () => Promise<void> } {
  const isWebSocketInitialized = useRef(false);
  const previousGameRef = useRef<GameDto | null>(null);
  const reconnectRequest = useRef<AbortController | null>(null);

  const {
    playTemperatureSound,
    playOxygenSound,
    playVenusSound,
    playCardPlayedSound,
    playPlayerJoinedSound,
  } = useSoundEffects();

  const attemptReconnection = async () => {
    if (reconnectRequest.current) {
      return;
    }
    const controller = new AbortController();
    reconnectRequest.current = controller;
    try {
      const savedGameData = getGameSession();
      if (!savedGameData) {
        clearGameSession();
        navigate("/", { replace: true });
        return;
      }

      const { gameId, playerId, playerName, isSpectator } = savedGameData;

      useGameStore.getState().setReconnectionStep("game");

      const query = isSpectator ? "" : `?playerId=${encodeURIComponent(playerId)}`;
      const response = await fetch(`${config.apiUrl}/games/${gameId}${query}`, {
        signal: controller.signal,
      });
      if (!response.ok) {
        clearGameSession();
        navigate("/", { replace: true });
        return;
      }

      const gameData = await response.json();
      controller.signal.throwIfAborted();

      const store = useGameStore.getState();
      store.setGame(gameData.game);
      store.setPlayerId(playerId);
      store.setIsSpectator(!!isSpectator);

      const player = gameData.game.currentPlayer;
      store.setCurrentPlayer(isSpectator ? null : player || null);

      if (isSpectator) {
        await globalWebSocketManager.spectatorConnect(playerName, gameId);
      } else {
        await globalWebSocketManager.playerConnect(playerName, gameId, playerId);
      }
    } catch (error) {
      if (controller.signal.aborted) {
        return;
      }
      console.error("Reconnection failed:", error);
      const store = useGameStore.getState();
      store.setIsReconnecting(false);
      store.setReconnectionStep(null);
    } finally {
      if (reconnectRequest.current === controller) {
        reconnectRequest.current = null;
      }
    }
  };

  useEffect(() => {
    if (isWebSocketInitialized.current) {
      return;
    }

    const initialPlayerId = useGameStore.getState().playerId;
    if (initialPlayerId) {
      globalWebSocketManager.setCurrentPlayerId(initialPlayerId);
    }

    const handleGameUpdated = (updatedGame: GameDto) => {
      const myPlayerId = useGameStore.getState().playerId;
      if (!myPlayerId && !updatedGame.isSpectator) {
        return;
      }

      engineProfiler.stateReceived();

      if (previousGameRef.current) {
        const changes = findChangedPaths(previousGameRef.current, updatedGame);
        useGameStore.getState().setChangedPaths(changes);

        const prevTemp = previousGameRef.current.globalParameters?.temperature;
        const newTemp = updatedGame.globalParameters?.temperature;
        if (prevTemp !== undefined && newTemp !== undefined && newTemp > prevTemp) {
          void playTemperatureSound();
        }

        const prevOxygen = previousGameRef.current.globalParameters?.oxygen;
        const newOxygen = updatedGame.globalParameters?.oxygen;
        const greeneryTypes = new Set([
          "greenery-tile",
          "ecological-zone-tile",
          "natural-preserve-tile",
        ]);
        const prevGreeneryCount =
          previousGameRef.current.board?.tiles?.filter(
            (t) => t.occupiedBy && greeneryTypes.has(t.occupiedBy.type),
          ).length ?? 0;
        const newGreeneryCount =
          updatedGame.board?.tiles?.filter(
            (t) => t.occupiedBy && greeneryTypes.has(t.occupiedBy.type),
          ).length ?? 0;
        const greeneryPlaced = newGreeneryCount > prevGreeneryCount;
        if (prevOxygen !== undefined && newOxygen !== undefined && newOxygen > prevOxygen) {
          void playOxygenSound();
        } else if (greeneryPlaced) {
          void playOxygenSound();
        }

        const prevVenus = previousGameRef.current.globalParameters?.venus;
        const newVenus = updatedGame.globalParameters?.venus;
        if (prevVenus !== undefined && newVenus !== undefined && newVenus > prevVenus) {
          void playVenusSound();
        }

        if (updatedGame.status === GameStatusLobby) {
          const prevPresent = new Set(
            (previousGameRef.current.otherPlayers ?? []).filter(isPresent).map((p) => p.id),
          );
          const someoneJoined = (updatedGame.otherPlayers ?? []).some(
            (p) => isPresent(p) && !prevPresent.has(p.id),
          );
          if (someoneJoined) {
            void playPlayerJoinedSound();
          }
        }

        if (changes.size > 0) {
          setTimeout(() => {
            useGameStore.getState().setChangedPaths(new Set());
          }, 1500);
        }
      }

      if (!previousGameRef.current || previousGameRef.current.id !== updatedGame.id) {
        useGameStore.getState().setChatMessages(updatedGame.chatMessages ?? []);
      }

      previousGameRef.current = deepClone(updatedGame);

      // The corporation showcase presents corp and prelude effects itself.
      const isInitApply =
        updatedGame.currentPhase === GamePhaseInitApplyCorp ||
        updatedGame.currentPhase === GamePhaseInitApplyPrelude;
      if (!isInitApply && updatedGame.triggeredEffects && updatedGame.triggeredEffects.length > 0) {
        const store = useGameStore.getState();
        store.setTriggeredEffects(updatedGame.triggeredEffects);
        const notificationCount = updatedGame.triggeredEffects.length;
        notificationQueueDoneAt.current = Date.now() + notificationCount * 2500;
        setTimeout(() => useGameStore.getState().setTriggeredEffects([]), 100);
      }

      const store = useGameStore.getState();
      store.setGameRef(updatedGame);
      store.setGame(updatedGame);
      store.setIsConnected(true);

      if (store.isReconnecting) {
        store.setIsReconnecting(false);
        store.setReconnectionStep(null);
      }

      const updatedPlayer = updatedGame.currentPlayer;
      store.setCurrentPlayer(updatedPlayer || null);

      if (updatedPlayer && !updatedPlayer.corporation) {
        useUIOverlayStore.getState().setShowCorporationModal(true);
      } else {
        useUIOverlayStore.getState().setShowCorporationModal(false);
      }
    };

    const handleLogUpdate = ({ logs, isHistory }: LogUpdatePayload) => {
      if (isHistory) {
        return;
      }
      const store = useGameStore.getState();
      const latestGame = store.gameRef;
      const myPlayerId = store.playerId;
      if (!latestGame || store.isReconnecting) {
        return;
      }
      if (!myPlayerId) {
        return;
      }
      // Corps and preludes played during setup are presented by the corporation showcase.
      const isInitApply =
        latestGame.currentPhase === GamePhaseInitApplyCorp ||
        latestGame.currentPhase === GamePhaseInitApplyPrelude;
      for (const log of logs) {
        if (
          log.playerId === myPlayerId &&
          (log.sourceType === "standard_project" || log.sourceType === "card_action")
        ) {
          void playCardPlayedSound();
        }

        if (log.sourceType !== "card_play" || isInitApply) {
          continue;
        }

        const playerChanges = log.changes?.playerChanges?.[log.playerId];
        const cardIds = playerChanges?.cardsPlayed;
        if (!cardIds || cardIds.length === 0) {
          continue;
        }

        const isMe = log.playerId === myPlayerId;
        if (isMe) {
          void playCardPlayedSound();
        }
        const player = isMe
          ? latestGame.currentPlayer
          : latestGame.otherPlayers?.find((p) => p.id === log.playerId);
        if (!player) {
          continue;
        }

        for (const cardId of cardIds) {
          const card = player.playedCards.find((c) => c.id === cardId);
          if (!card) {
            continue;
          }
          if (!isMe) {
            enqueuePlayedCard({
              card,
              playerName: player.name,
              playerColor: player.color,
            });
          }
        }
      }
    };

    const handleError = () => {};

    const handlePlayerKicked = () => {
      clearGameSession();
      globalWebSocketManager.disconnect();
      navigate("/", { replace: true });
      showNotification({
        message: "You were kicked from the game",
        type: "info",
      });
    };

    const handleGameEnded = () => {
      clearGameSession();
      globalWebSocketManager.disconnect();
      navigate("/", { replace: true });
      showNotification({
        message: "The host ended the game",
        type: "info",
      });
    };

    const handleDisconnect = () => {
      if (globalWebSocketManager.isGracefulDisconnect()) {
        return;
      }

      useGameStore.getState().setIsConnected(false);

      if (useGameStore.getState().playerId) {
        useGameStore.getState().setIsReconnecting(true);

        const savedGameData = getGameSession();
        if (savedGameData) {
          void attemptReconnection();
        } else {
          clearGameSession();
          navigate("/", { replace: true });
        }
      }
    };

    const handleConnectionAvailable = () => {
      const store = useGameStore.getState();
      const session = getGameSession();
      if (
        !store.isConnected &&
        store.playerId &&
        session?.gameId === store.game?.id &&
        session?.playerId === store.playerId &&
        !globalWebSocketManager.isGracefulDisconnect()
      ) {
        store.setIsReconnecting(true);
        void attemptReconnection();
      }
    };

    const handleMaxReconnectsReached = () => {
      clearGameSession();
      // Behind a gateway the menu would belong to the dead server; offer the other servers instead
      if (gatewayServer) {
        useUIOverlayStore.getState().setShowServerDown(true);
        return;
      }
      navigate("/", { state: { error: "Server is down", persistent: true } });
    };

    const handleChatUpdate = (chatMessage: ChatMessageDto) => {
      useGameStore.getState().addChatMessage(chatMessage);
    };

    const handleEmote = (payload: EmotePayload) => {
      useBotPresenceStore.getState().setEmote(payload.playerId, payload.emote);
    };

    const handleBotThought = (payload: BotThoughtPayload) => {
      const presence = useBotPresenceStore.getState();
      presence.setTyping(payload.playerId, payload.typing);
      if (payload.text) {
        presence.setThought(payload.playerId, payload.text);
      }
    };

    const handleSpectatorKicked = () => {
      globalWebSocketManager.disconnect();
      navigate("/", { replace: true });
    };

    const handleSpectatorIdReceived = (payload: { spectatorId: string }) => {
      const gameId = globalWebSocketManager.gameId;
      if (!gameId || !payload.spectatorId) {
        return;
      }
      useGameStore.getState().setPlayerId(payload.spectatorId);
      const savedSession = getGameSession();
      if (savedSession && savedSession.gameId === gameId) {
        saveGameSession({
          ...savedSession,
          playerId: payload.spectatorId,
          isSpectator: true,
        });
      }
    };

    globalWebSocketManager.on("game-updated", handleGameUpdated);
    globalWebSocketManager.on("log-update", handleLogUpdate);
    globalWebSocketManager.on("player-kicked", handlePlayerKicked);
    globalWebSocketManager.on("game-ended", handleGameEnded);
    globalWebSocketManager.on("error", handleError);
    globalWebSocketManager.on("disconnect", handleDisconnect);
    globalWebSocketManager.on("connect", handleConnectionAvailable);
    window.addEventListener("online", handleConnectionAvailable);
    globalWebSocketManager.on("max-reconnects-reached", handleMaxReconnectsReached);
    globalWebSocketManager.on("chat-update", handleChatUpdate);
    globalWebSocketManager.on("emote", handleEmote);
    globalWebSocketManager.on("bot-thought", handleBotThought);
    globalWebSocketManager.on("spectator-kicked", handleSpectatorKicked);
    globalWebSocketManager.on("spectator-connected", handleSpectatorIdReceived);

    isWebSocketInitialized.current = true;

    return () => {
      reconnectRequest.current?.abort();
      reconnectRequest.current = null;
      globalWebSocketManager.off("game-updated", handleGameUpdated);
      globalWebSocketManager.off("log-update", handleLogUpdate);
      globalWebSocketManager.off("player-kicked", handlePlayerKicked);
      globalWebSocketManager.off("game-ended", handleGameEnded);
      globalWebSocketManager.off("error", handleError);
      globalWebSocketManager.off("disconnect", handleDisconnect);
      globalWebSocketManager.off("connect", handleConnectionAvailable);
      window.removeEventListener("online", handleConnectionAvailable);
      globalWebSocketManager.off("max-reconnects-reached", handleMaxReconnectsReached);
      globalWebSocketManager.off("chat-update", handleChatUpdate);
      globalWebSocketManager.off("emote", handleEmote);
      globalWebSocketManager.off("bot-thought", handleBotThought);
      useBotPresenceStore.getState().reset();
      globalWebSocketManager.off("spectator-kicked", handleSpectatorKicked);
      globalWebSocketManager.off("spectator-connected", handleSpectatorIdReceived);
      isWebSocketInitialized.current = false;
    };
  }, []);

  return { attemptReconnection };
}
