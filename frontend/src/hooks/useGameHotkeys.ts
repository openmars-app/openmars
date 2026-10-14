import { usePaymentStore } from "@/stores/paymentStore";
import { type RefObject, useEffect } from "react";
import type { CardFanOverlayHandle } from "@/components/ui/overlay/CardFanOverlay.tsx";
import type { PlayerListHandle } from "@/components/ui/list/PlayerList.tsx";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import { useCardPlayFlowStore } from "@/stores/cardPlayFlowStore.ts";
import { useGameStore } from "@/stores/gameStore.ts";
import { useSpectateStore } from "@/stores/spectateStore.ts";
import { useWorld3DSettings } from "@/contexts/World3DSettingsContext.tsx";
import {
  GamePhaseAction,
  GamePhaseInitApplyCorp,
  GamePhaseInitApplyPrelude,
  GameStatusActive,
  GameStatusLobby,
} from "@/types/generated/api-types.ts";

export function useGameHotkeys(
  cardFanRef: RefObject<CardFanOverlayHandle | null>,
  playerListRef: RefObject<PlayerListHandle | null>,
): void {
  const { settings } = useWorld3DSettings();

  useEffect(() => {
    const handleToggleDebug = () => {
      useUIOverlayStore.getState().toggleShowDebugDropdown();
    };
    window.addEventListener("toggle-debug-dropdown", handleToggleDebug);
    return () => {
      window.removeEventListener("toggle-debug-dropdown", handleToggleDebug);
    };
  }, []);

  useEffect(() => {
    const handleTogglePerf = () => {
      useUIOverlayStore.getState().toggleShowPerformanceWindow();
    };
    window.addEventListener("toggle-performance-window", handleTogglePerf);
    return () => {
      window.removeEventListener("toggle-performance-window", handleTogglePerf);
    };
  }, []);

  useEffect(() => {
    const handleToggleFeedback = () => {
      useUIOverlayStore.getState().toggleShowFeedbackWindow();
    };
    window.addEventListener("toggle-feedback-window", handleToggleFeedback);
    return () => {
      window.removeEventListener("toggle-feedback-window", handleToggleFeedback);
    };
  }, []);

  useEffect(() => {
    const handleKeyPress = (event: KeyboardEvent) => {
      if (useUIOverlayStore.getState().showUIDebug) return;
      if (event.ctrlKey || event.metaKey) {
        switch (event.key) {
          case "1":
            event.preventDefault();
            useUIOverlayStore.getState().setShowCardsPlayedModal(true);
            break;
          case "4":
            event.preventDefault();
            break;
          case "5":
            event.preventDefault();
            break;
        }
      }
    };

    window.addEventListener("keydown", handleKeyPress);
    return () => window.removeEventListener("keydown", handleKeyPress);
  }, []);

  const spectatePlayerId = useSpectateStore((s) => s.spectatePlayerId);

  useEffect(() => {
    if (!spectatePlayerId) {
      return;
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (useUIOverlayStore.getState().showUIDebug) return;
      if (e.key === "Escape") {
        useSpectateStore.setState({ spectatePlayerId: null });
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [spectatePlayerId]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (useUIOverlayStore.getState().showUIDebug) return;
      const target = e.target as HTMLElement;
      if (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable) {
        return;
      }

      const ui = useUIOverlayStore.getState();
      const cpf = useCardPlayFlowStore.getState();
      const gameState = useGameStore.getState();
      const spectate = useSpectateStore.getState();

      const game = gameState.game;
      const currentPlayer = gameState.currentPlayer;
      const isSpectator = gameState.isSpectator;

      const isPreGamePhase = game?.status === GameStatusLobby;
      const isInitApplyPhase =
        game?.status === GameStatusActive &&
        (game?.currentPhase === GamePhaseInitApplyCorp ||
          game?.currentPhase === GamePhaseInitApplyPrelude);

      const anyModalOpen =
        ui.showStartingSelection ||
        ui.showPendingCardSelection ||
        ui.showCardDrawSelection ||
        (game?.currentPlayer?.pendingBehaviorResolutions?.length ?? 0) > 0 ||
        ui.showResourceRemovalSelection ||
        ui.showColonyResourceSelection ||
        ui.showProductionPhaseModal ||
        !!usePaymentStore.getState().pending ||
        cpf.showChoiceSelection ||
        cpf.showActionChoiceSelection ||
        cpf.showActionReuseSelection ||
        cpf.showCardStorageSelection ||
        cpf.showActionStorageSelection ||
        cpf.showTargetPlayerSelection ||
        cpf.showActionTargetPlayerSelection ||
        cpf.showCardResourceSelection ||
        cpf.showAmountSelection ||
        ui.showLeaveGameConfirm ||
        ui.showEndGameConfirm ||
        ui.showCardsPlayedModal ||
        isPreGamePhase ||
        isInitApplyPhase;

      if (e.key === " ") {
        e.preventDefault();

        if (settings.freeCameraEnabled) {
          return;
        }

        if (ui.showCorporationOverlay) {
          useUIOverlayStore.getState().setShowCorporationOverlay(false);
          return;
        }

        if (cardFanRef.current?.isExpanded) {
          cardFanRef.current.collapse();
          return;
        }

        if (anyModalOpen || spectate.spectatePlayerId || isSpectator) {
          return;
        }

        if (e.shiftKey) {
          useUIOverlayStore.getState().setShowCorporationOverlay(true);
        } else {
          cardFanRef.current?.toggleExpand();
        }
      }

      if (e.key === "Enter") {
        e.preventDefault();
        if (
          !anyModalOpen &&
          game?.currentPhase === GamePhaseAction &&
          currentPlayer?.id === game?.currentTurn &&
          !currentPlayer?.pendingTileSelection
        ) {
          playerListRef.current?.requestSkipAction();
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [cardFanRef, playerListRef, settings.freeCameraEnabled]);
}
