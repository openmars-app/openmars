import { webSocketService } from "@/services/webSocketService";
import { useCallback, useEffect, useRef } from "react";
import { useGameStore } from "@/stores/gameStore.ts";
import { useCardPlayFlowStore } from "@/stores/cardPlayFlowStore.ts";
import { useSpectateStore } from "@/stores/spectateStore.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { StandardProject } from "@/types/cards.tsx";

import {
  getAllAnyCardStorageSelections,
  needsTargetPlayerSelection,
  needsCardResourceInput,
  getVariableAmountInfo,
} from "@/utils/cardPlayUtils.ts";
import {
  MessageTypeActionPlayCard,
  type CardDto,
  type ErrorPayload,
  type PaymentDto,
  type PlayerActionDto,
  type ResourceType,
} from "@/types/generated/api-types.ts";

export function useCardPlayFlow() {
  const activeReuseSourceCardId = useRef<string | undefined>(undefined);

  useEffect(() => {
    const unsubscribe = useGameStore.subscribe((state, previous) => {
      const flow = useCardPlayFlowStore.getState();
      const session = flow.playSession;
      if (!session || state.currentPlayer === previous.currentPlayer) {
        return;
      }
      if (
        state.game?.id !== previous.game?.id ||
        state.currentPlayer?.id !== previous.currentPlayer?.id
      ) {
        flow.clearPlayPresentation();
        return;
      }
      if (session.phase !== "submitting" && session.phase !== "reconnecting") {
        return;
      }
      const inHand = state.currentPlayer?.cards.some((card) => card.id === session.card.id);
      if (!inHand) {
        flow.confirmPlay(session.id);
      } else if (session.phase === "reconnecting") {
        flow.returnPlay(
          session.id,
          "Connection restored. The card was not played. Please try again.",
        );
      }
    });
    const failed = (payload: unknown) => {
      if (!payload || typeof payload !== "object") {
        return;
      }
      const error = payload as Partial<ErrorPayload>;
      const flow = useCardPlayFlowStore.getState();
      const session = flow.playSession;
      if (
        session &&
        error.requestType === MessageTypeActionPlayCard &&
        error.cardId === session?.card.id &&
        (session.phase === "submitting" || session.phase === "reconnecting")
      ) {
        flow.returnPlay(session.id, error.message ?? "Could not play card.");
      }
    };
    const disconnected = () => useCardPlayFlowStore.getState().suspendPlay();
    webSocketService.on("error", failed);
    webSocketService.on("disconnect", disconnected);
    return () => {
      unsubscribe();
      webSocketService.off("error", failed);
      webSocketService.off("disconnect", disconnected);
      useCardPlayFlowStore.getState().clearPlayPresentation();
    };
  }, []);

  const finalizePlayCard = useCallback(
    async (
      cardId: string,
      payment: PaymentDto | undefined,
      choiceIndex?: number,
      cardStorageTargets?: string[],
      cardForBehaviors?: CardDto,
      selectedAmount?: number,
      cardStorageSources: string[] = [],
    ) => {
      const cp = useGameStore.getState().currentPlayer;
      const card = cp?.cards.find((c) => c.id === cardId) || cardForBehaviors;
      const store = useCardPlayFlowStore.getState();

      if (card && selectedAmount === undefined) {
        const autoTriggerBehaviors = card.behaviors?.filter((b) =>
          b.triggers?.some((t) => t.type === "auto"),
        );
        for (const behavior of autoTriggerBehaviors || []) {
          const matchedChoice =
            choiceIndex !== undefined
              ? behavior.choices?.find((c) => c.originalIndex === choiceIndex)
              : undefined;
          const outputs = matchedChoice ? matchedChoice.outputs : behavior.outputs;
          const inputs = matchedChoice ? matchedChoice.inputs : behavior.inputs;
          const variableInfo = getVariableAmountInfo(
            inputs,
            outputs,
            cp,
            matchedChoice?.inputOptions ?? behavior.inputOptions,
          );
          if (variableInfo) {
            store.setPendingVariableAmount({
              type: "play-card",
              cardId,
              payment,
              choiceIndex,
              cardStorageTargets,
              cardStorageSources,
              resourceType: variableInfo.resourceType,
              maxAmount: variableInfo.maxAmount,
            });
            store.setShowAmountSelection(true);
            return;
          }
        }
      }

      if (card) {
        const requiredSources =
          card.behaviors
            ?.filter((behavior) =>
              behavior.triggers?.some((trigger) => trigger.type === "auto" && !trigger.condition),
            )
            .flatMap((behavior) => {
              const choice = behavior.choices?.find((c) => c.originalIndex === choiceIndex);
              const inputs = [...(behavior.inputs ?? []), ...(choice?.inputs ?? [])].filter(
                (input) => input.target === "any-card",
              );
              const options = choice?.inputOptions ?? behavior.inputOptions;
              return inputs.map((input, index) => ({
                input,
                ids: options?.storageSources?.[index] ?? [],
              }));
            }) ?? [];
        const next = requiredSources[cardStorageSources.length];
        if (next) {
          store.setPendingCardResourceInput({
            type: "play-card",
            cardId,
            payment,
            choiceIndex,
            cardStorageTargets,
            cardStorageSources,
            selectedAmount,
            resourceType: next.input.type as ResourceType,
            amount: next.input.amount,
            eligibleInputCardIds: next.ids,
          });
          store.setShowCardResourceSelection(true);
          return;
        }
        const autoTriggerBehaviors = card.behaviors?.filter((b) =>
          b.triggers?.some((t) => t.type === "auto"),
        );
        for (const behavior of autoTriggerBehaviors || []) {
          const matchedChoice2 =
            choiceIndex !== undefined
              ? behavior.choices?.find((c) => c.originalIndex === choiceIndex)
              : undefined;
          const outputs = matchedChoice2 ? matchedChoice2.outputs : behavior.outputs;
          const removal = outputs?.find((o) => o.target === "any-card" && o.amount < 0);
          if (removal) {
            store.setPendingCardResourceInput({
              type: "play-card",
              cardId,
              payment,
              choiceIndex,
              cardStorageTargets,
              cardStorageSources,
              selectedAmount,
              resourceType: removal.type as ResourceType,
              amount: -removal.amount,
            });
            store.setShowCardResourceSelection(true);
            return;
          }
          const g = useGameStore.getState().game;
          const targetInfo = needsTargetPlayerSelection(outputs, g?.otherPlayers);
          if (targetInfo) {
            store.setPendingTargetPlayer({
              cardId,
              payment,
              choiceIndex,
              cardStorageTargets,
              cardStorageSources,
              selectedAmount,
              resourceType: targetInfo.resourceType,
              amount: targetInfo.amount,
              isSteal: targetInfo.isSteal,
            });
            store.setShowTargetPlayerSelection(true);
            return;
          }
        }
      }

      await globalWebSocketManager.playCard(
        cardId,
        payment,
        choiceIndex,
        cardStorageTargets,
        undefined,
        selectedAmount,
        cardStorageSources,
      );
    },
    [],
  );

  const finalizeCardActionInputs = useCallback(
    async (
      action: PlayerActionDto,
      choiceIndex?: number,
      cardStorageTargets?: string[],
      selectedAmount?: number,
      cardStorageSources: string[] = [],
      reuseSourceCardId?: string,
    ) => {
      const store = useCardPlayFlowStore.getState();
      const choice = action.behavior.choices?.find((c) => c.originalIndex === choiceIndex);
      const options = choice?.inputOptions ?? action.behavior.inputOptions;
      const inputs = [...(action.behavior.inputs ?? []), ...(choice?.inputs ?? [])];
      const outputs = [...(action.behavior.outputs ?? []), ...(choice?.outputs ?? [])];
      const variable = options?.variableAmount;
      if (variable && selectedAmount === undefined) {
        store.setPendingVariableAmount({
          type: "card-action",
          cardId: action.cardId,
          behaviorIndex: action.behaviorIndex,
          choiceIndex,
          cardStorageTargets,
          cardStorageSources,
          reuseSourceCardId,
          resourceType: variable.resourceType,
          maxAmount: variable.max,
        });
        store.setShowAmountSelection(true);
        return;
      }
      const sourceInputs = inputs.filter((input) => input.target === "any-card");
      const next = sourceInputs[cardStorageSources.length];
      if (next) {
        store.setPendingCardResourceInput({
          type: "card-action",
          cardId: action.cardId,
          behaviorIndex: action.behaviorIndex,
          choiceIndex,
          cardStorageTargets,
          cardStorageSources,
          selectedAmount,
          reuseSourceCardId,
          resourceType: next.type as ResourceType,
          amount: next.amount,
          eligibleInputCardIds: options?.storageSources?.[cardStorageSources.length] ?? [],
        });
        store.setShowCardResourceSelection(true);
        return;
      }
      const removal = needsCardResourceInput(inputs, outputs);
      if (removal) {
        store.setPendingCardResourceInput({
          type: "card-action",
          cardId: action.cardId,
          behaviorIndex: action.behaviorIndex,
          choiceIndex,
          cardStorageTargets,
          cardStorageSources,
          selectedAmount,
          reuseSourceCardId,
          resourceType: removal.resourceType,
          amount: removal.amount,
        });
        store.setShowCardResourceSelection(true);
        return;
      }
      const allStorageNeeds = getAllAnyCardStorageSelections(
        outputs,
        choice?.storageTargets?.[0] ?? action.dependencyTargets,
      );
      if (allStorageNeeds.length > 0 && !cardStorageTargets) {
        const first = allStorageNeeds[0];
        store.setPendingActionStorage({
          cardId: action.cardId,
          behaviorIndex: action.behaviorIndex,
          choiceIndex,
          cardStorageSources,
          selectedAmount,
          reuseSourceCardId,
          allStorageNeeds,
          collectedTargets: [],
          currentIndex: 0,
          resourceType: first.resourceType,
          amount: first.amount,
          selectorTags: first.selectorTags,
          eligibleCardIds: first.eligibleCardIds,
        });
        store.setShowActionStorageSelection(true);
        return;
      }
      const game = useGameStore.getState().game;
      if (outputs.some((output) => output.type === "trade")) {
        let warning: string | undefined;
        if ((game?.tradeFleets?.[game.viewingPlayerId ?? ""]?.available ?? 0) === 0) {
          warning = "No trade fleet available";
        } else if (
          !(game?.colonies ?? []).some((colony) => colony.active && !colony.tradedThisGen)
        ) {
          warning = "No colonies available for trading";
        }
        if (warning) {
          store.setPendingFreeTradeWarning(warning);
          store.setShowFreeTradeWarning(true);
          activeReuseSourceCardId.current = undefined;
          return;
        }
      }
      const target = needsTargetPlayerSelection(outputs, game?.otherPlayers);
      if (target) {
        store.setPendingActionTargetPlayer({
          cardId: action.cardId,
          behaviorIndex: action.behaviorIndex,
          choiceIndex,
          cardStorageTargets,
          cardStorageSources,
          selectedAmount,
          reuseSourceCardId,
          ...target,
        });
        store.setShowActionTargetPlayerSelection(true);
        return;
      }
      await globalWebSocketManager.playCardAction(
        action.cardId,
        action.behaviorIndex,
        choiceIndex,
        cardStorageTargets,
        undefined,
        undefined,
        selectedAmount,
        undefined,
        reuseSourceCardId,
        cardStorageSources,
      );
      activeReuseSourceCardId.current = undefined;
    },
    [],
  );

  const handlePlayCard = useCallback(
    async (cardId: string) => {
      try {
        const g = useGameStore.getState().game;
        const cp = useGameStore.getState().currentPlayer;
        const store = useCardPlayFlowStore.getState();

        if (useSpectateStore.getState().spectatePlayerId) {
          throw new Error("Cannot play cards while spectating");
        }

        if (g?.currentTurn !== g?.viewingPlayerId) {
          throw new Error("Not your turn");
        }

        if (cp?.pendingTileSelection) {
          throw new Error("Finish selecting a tile before playing a card");
        }

        const card = cp?.cards.find((c) => c.id === cardId);
        if (!card) {
          throw new Error("This card is no longer in your hand");
        }

        const behaviorWithChoices = card.behaviors?.findIndex(
          (b) =>
            b.choices &&
            b.choices.length > 0 &&
            b.triggers?.some((t) => t.type === "auto") &&
            b.choicePolicy?.type !== "auto",
        );

        if (
          behaviorWithChoices !== undefined &&
          behaviorWithChoices >= 0 &&
          card.behaviors?.[behaviorWithChoices]?.choices
        ) {
          store.setCardPendingChoice(card);
          store.setPendingCardBehaviorIndex(behaviorWithChoices);
          store.setShowChoiceSelection(true);
        } else {
          const autoTriggerBehaviors = card.behaviors?.filter((b) =>
            b.triggers?.some((t) => t.type === "auto"),
          );

          const allStorageNeeds: Array<{
            resourceType: ResourceType;
            amount: number;
            selectorTags?: string[];
            eligibleCardIds?: string[];
          }> = [];
          for (const behavior of autoTriggerBehaviors || []) {
            const selections = getAllAnyCardStorageSelections(
              behavior.outputs,
              card.dependencyTargets,
            );
            for (const sel of selections) {
              allStorageNeeds.push({
                resourceType: sel.resourceType,
                amount: sel.amount,
                selectorTags: sel.selectorTags,
                eligibleCardIds: sel.eligibleCardIds,
              });
            }
          }

          if (allStorageNeeds.length > 0) {
            const first = allStorageNeeds[0];
            store.setPendingCardStorage({
              cardId: card.id,
              choiceIndex: undefined,
              allStorageNeeds,
              collectedTargets: [],
              currentIndex: 0,
              resourceType: first.resourceType,
              amount: first.amount,
              selectorTags: first.selectorTags,
              eligibleCardIds: first.eligibleCardIds,
            });
            store.setShowCardStorageSelection(true);
          } else {
            const payment = undefined;
            await finalizePlayCard(cardId, payment, undefined, undefined, card);
          }
        }
      } catch (error) {
        console.error(`Failed to play card ${cardId}:`, error);
        throw error;
      }
    },
    [finalizePlayCard, finalizeCardActionInputs],
  );

  const handleChoiceSelect = useCallback(
    async (choiceIndex: number) => {
      const store = useCardPlayFlowStore.getState();
      const { cardPendingChoice, pendingCardBehaviorIndex } = store;
      const currentPlayer = useGameStore.getState().currentPlayer;

      if (!cardPendingChoice || !currentPlayer) {
        return;
      }

      try {
        store.setShowChoiceSelection(false);

        const behavior = cardPendingChoice.behaviors?.[pendingCardBehaviorIndex];
        const selectedChoice = behavior?.choices?.find((c) => c.originalIndex === choiceIndex);

        const allStorageNeeds: Array<{
          resourceType: ResourceType;
          amount: number;
          selectorTags?: string[];
          eligibleCardIds?: string[];
        }> = [];
        const choiceSelections = getAllAnyCardStorageSelections(
          selectedChoice?.outputs,
          selectedChoice?.storageTargets?.[0] ?? cardPendingChoice.dependencyTargets,
        );
        for (const sel of choiceSelections) {
          allStorageNeeds.push({
            resourceType: sel.resourceType,
            amount: sel.amount,
            selectorTags: sel.selectorTags,
            eligibleCardIds: sel.eligibleCardIds,
          });
        }
        if (allStorageNeeds.length === 0) {
          const behaviorSelections = getAllAnyCardStorageSelections(
            behavior?.outputs,
            cardPendingChoice.dependencyTargets,
          );
          for (const sel of behaviorSelections) {
            allStorageNeeds.push({
              resourceType: sel.resourceType,
              amount: sel.amount,
              selectorTags: sel.selectorTags,
              eligibleCardIds: sel.eligibleCardIds,
            });
          }
        }

        if (allStorageNeeds.length > 0) {
          const first = allStorageNeeds[0];
          store.setPendingCardStorage({
            cardId: cardPendingChoice.id,
            choiceIndex: choiceIndex,
            allStorageNeeds,
            collectedTargets: [],
            currentIndex: 0,
            resourceType: first.resourceType,
            amount: first.amount,
            selectorTags: first.selectorTags,
            eligibleCardIds: first.eligibleCardIds,
          });
          store.setShowCardStorageSelection(true);
          store.setCardPendingChoice(null);
          store.setPendingCardBehaviorIndex(0);
        } else {
          const payment = undefined;
          await finalizePlayCard(
            cardPendingChoice.id,
            payment,
            choiceIndex,
            undefined,
            cardPendingChoice,
          );
          store.setCardPendingChoice(null);
          store.setPendingCardBehaviorIndex(0);
        }
      } catch (error) {
        const active = store.playSession;
        if (active) {
          useCardPlayFlowStore.getState().returnPlay(active.id, error);
        }
        console.error(
          `Failed to play card ${cardPendingChoice.id} with choice ${choiceIndex}:`,
          error,
        );
        store.setCardPendingChoice(null);
        store.setPendingCardBehaviorIndex(0);
      }
    },
    [finalizePlayCard, finalizeCardActionInputs],
  );

  const handleChoiceCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    if (store.playSession) {
      store.returnPlay(store.playSession.id);
    }
    store.setShowChoiceSelection(false);
    store.setCardPendingChoice(null);
    store.setPendingCardBehaviorIndex(0);
  }, []);

  const handleActionChoiceSelect = useCallback(
    async (choiceIndex: number) => {
      const store = useCardPlayFlowStore.getState();
      const action = store.actionPendingChoice;
      if (!action) {
        return;
      }
      store.setShowActionChoiceSelection(false);
      store.setActionPendingChoice(null);
      try {
        await finalizeCardActionInputs(
          action,
          choiceIndex,
          undefined,
          undefined,
          [],
          activeReuseSourceCardId.current,
        );
      } catch (error) {
        activeReuseSourceCardId.current = undefined;
        console.error("Failed to use card action:", error);
      }
    },
    [finalizeCardActionInputs],
  );

  const handleActionChoiceCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    store.setShowActionChoiceSelection(false);
    store.setActionPendingChoice(null);
    activeReuseSourceCardId.current = undefined;
  }, []);

  const handleActionReuseCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    store.setShowActionReuseSelection(false);
    store.setPendingActionReuse(null);
    activeReuseSourceCardId.current = undefined;
  }, []);

  const handleColonyResourceSelect = useCallback(async (cardId: string) => {
    void globalWebSocketManager.confirmColonyResource(cardId);
  }, []);

  const handleColonyResourceSkip = useCallback(async () => {
    void globalWebSocketManager.confirmColonyResource("");
  }, []);

  const handleCardStorageSelect = useCallback(
    async (targetCardId: string) => {
      const store = useCardPlayFlowStore.getState();
      const { pendingCardStorage } = store;

      if (!pendingCardStorage) {
        return;
      }
      const cp = useGameStore.getState().currentPlayer;
      if (!cp) {
        return;
      }

      try {
        const newCollected = [...pendingCardStorage.collectedTargets, targetCardId];
        const nextIndex = pendingCardStorage.currentIndex + 1;

        if (nextIndex < pendingCardStorage.allStorageNeeds.length) {
          const next = pendingCardStorage.allStorageNeeds[nextIndex];
          store.setPendingCardStorage({
            ...pendingCardStorage,
            collectedTargets: newCollected,
            currentIndex: nextIndex,
            resourceType: next.resourceType,
            amount: next.amount,
            selectorTags: next.selectorTags,
            eligibleCardIds: next.eligibleCardIds,
          });
          return;
        }

        store.setShowCardStorageSelection(false);
        const card = cp.cards.find((c) => c.id === pendingCardStorage.cardId);

        const payment = undefined;
        await finalizePlayCard(
          pendingCardStorage.cardId,
          payment,
          pendingCardStorage.choiceIndex,
          newCollected,
          card,
        );
        store.setPendingCardStorage(null);
      } catch (error) {
        const active = store.playSession;
        if (active) {
          useCardPlayFlowStore.getState().returnPlay(active.id, error);
        }
        console.error(
          `Failed to play card ${pendingCardStorage.cardId} with card storage target ${targetCardId}:`,
          error,
        );
        store.setPendingCardStorage(null);
      }
    },
    [finalizePlayCard, finalizeCardActionInputs],
  );

  const handleCardStorageCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    if (store.playSession) {
      store.returnPlay(store.playSession.id);
    }
    store.setShowCardStorageSelection(false);
    store.setPendingCardStorage(null);
  }, []);

  const handleActionStorageSelect = useCallback(
    async (targetCardId: string) => {
      const store = useCardPlayFlowStore.getState();
      const pending = store.pendingActionStorage;
      if (!pending) {
        return;
      }
      const collected = [...pending.collectedTargets, targetCardId];
      const nextIndex = pending.currentIndex + 1;
      if (nextIndex < pending.allStorageNeeds.length) {
        const next = pending.allStorageNeeds[nextIndex];
        store.setPendingActionStorage({
          ...pending,
          collectedTargets: collected,
          currentIndex: nextIndex,
          resourceType: next.resourceType,
          amount: next.amount,
          selectorTags: next.selectorTags,
          eligibleCardIds: next.eligibleCardIds,
        });
        return;
      }
      store.setShowActionStorageSelection(false);
      store.setPendingActionStorage(null);
      const action = useGameStore
        .getState()
        .currentPlayer?.actions?.find(
          (item) => item.cardId === pending.cardId && item.behaviorIndex === pending.behaviorIndex,
        );
      if (!action) {
        activeReuseSourceCardId.current = undefined;
        return;
      }
      try {
        await finalizeCardActionInputs(
          action,
          pending.choiceIndex,
          collected,
          pending.selectedAmount,
          pending.cardStorageSources ?? [],
          pending.reuseSourceCardId,
        );
      } catch (error) {
        activeReuseSourceCardId.current = undefined;
        console.error("Failed to use card action:", error);
      }
    },
    [finalizeCardActionInputs],
  );

  const handleActionStorageCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    store.setShowActionStorageSelection(false);
    store.setPendingActionStorage(null);
    activeReuseSourceCardId.current = undefined;
  }, []);

  const handleTargetPlayerSelect = useCallback(async (targetPlayerId: string) => {
    const store = useCardPlayFlowStore.getState();
    const { pendingTargetPlayer } = store;

    if (!pendingTargetPlayer) {
      return;
    }

    try {
      store.setShowTargetPlayerSelection(false);
      await globalWebSocketManager.playCard(
        pendingTargetPlayer.cardId,
        pendingTargetPlayer.payment,
        pendingTargetPlayer.choiceIndex,
        pendingTargetPlayer.cardStorageTargets,
        targetPlayerId,
        pendingTargetPlayer.selectedAmount,
        pendingTargetPlayer.cardStorageSources,
      );
      store.setPendingTargetPlayer(null);
    } catch (error) {
      const active = store.playSession;
      if (active) {
        useCardPlayFlowStore.getState().returnPlay(active.id, error);
      }
      console.error(
        `Failed to play card ${pendingTargetPlayer.cardId} with target player ${targetPlayerId}:`,
        error,
      );
      store.setPendingTargetPlayer(null);
    }
  }, []);

  const handleTargetPlayerCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    if (store.playSession) {
      store.returnPlay(store.playSession.id);
    }
    store.setShowTargetPlayerSelection(false);
    store.setPendingTargetPlayer(null);
  }, []);

  const handleAmountSelect = useCallback(
    async (amount: number) => {
      const store = useCardPlayFlowStore.getState();
      const { pendingVariableAmount } = store;

      if (!pendingVariableAmount) {
        return;
      }

      try {
        store.setShowAmountSelection(false);
        if (pendingVariableAmount.type === "play-card") {
          await finalizePlayCard(
            pendingVariableAmount.cardId,
            pendingVariableAmount.payment!,
            pendingVariableAmount.choiceIndex,
            pendingVariableAmount.cardStorageTargets,
            undefined,
            amount,
            pendingVariableAmount.cardStorageSources,
          );
        } else if (pendingVariableAmount.type === "card-action") {
          const action = useGameStore
            .getState()
            .currentPlayer?.actions.find(
              (a) =>
                a.cardId === pendingVariableAmount.cardId &&
                a.behaviorIndex === pendingVariableAmount.behaviorIndex,
            );
          if (!action) {
            throw new Error("Card action is no longer available");
          }
          await finalizeCardActionInputs(
            action,
            pendingVariableAmount.choiceIndex,
            pendingVariableAmount.cardStorageTargets,
            amount,
            pendingVariableAmount.cardStorageSources,
            pendingVariableAmount.reuseSourceCardId,
          );
        }
        store.setPendingVariableAmount(null);
      } catch (error) {
        const active = store.playSession;
        if (active) {
          useCardPlayFlowStore.getState().returnPlay(active.id, error);
        }
        console.error(`Failed to execute with amount ${amount}:`, error);
        store.setPendingVariableAmount(null);
      }
    },
    [finalizePlayCard, finalizeCardActionInputs],
  );

  const handleAmountCancel = useCallback(() => {
    activeReuseSourceCardId.current = undefined;
    const store = useCardPlayFlowStore.getState();
    if (store.playSession) {
      store.returnPlay(store.playSession.id);
    }
    store.setShowAmountSelection(false);
    store.setPendingVariableAmount(null);
  }, []);

  const handleActionTargetPlayerSelect = useCallback(async (targetPlayerId: string) => {
    const store = useCardPlayFlowStore.getState();
    const { pendingActionTargetPlayer } = store;

    if (!pendingActionTargetPlayer) {
      return;
    }

    try {
      store.setShowActionTargetPlayerSelection(false);
      await globalWebSocketManager.playCardAction(
        pendingActionTargetPlayer.cardId,
        pendingActionTargetPlayer.behaviorIndex,
        pendingActionTargetPlayer.choiceIndex,
        pendingActionTargetPlayer.cardStorageTargets,
        targetPlayerId,
        undefined,
        pendingActionTargetPlayer.selectedAmount,
        undefined,
        pendingActionTargetPlayer.reuseSourceCardId,
        pendingActionTargetPlayer.cardStorageSources,
      );
      store.setPendingActionTargetPlayer(null);
    } catch (error) {
      console.error(
        `Failed to play action ${pendingActionTargetPlayer.cardId} with target player ${targetPlayerId}:`,
        error,
      );
      store.setPendingActionTargetPlayer(null);
    }
  }, []);

  const handleActionTargetPlayerCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    store.setShowActionTargetPlayerSelection(false);
    store.setPendingActionTargetPlayer(null);
    activeReuseSourceCardId.current = undefined;
  }, []);

  const handleCardResourceSelect = useCallback(
    async (sourceCardId: string) => {
      const store = useCardPlayFlowStore.getState();
      const { pendingCardResourceInput } = store;

      if (!pendingCardResourceInput) {
        return;
      }

      try {
        store.setShowCardResourceSelection(false);
        if (pendingCardResourceInput.eligibleInputCardIds !== undefined) {
          const sources = [...(pendingCardResourceInput.cardStorageSources ?? []), sourceCardId];
          store.setPendingCardResourceInput(null);
          if (pendingCardResourceInput.type === "play-card") {
            await finalizePlayCard(
              pendingCardResourceInput.cardId,
              pendingCardResourceInput.payment,
              pendingCardResourceInput.choiceIndex,
              pendingCardResourceInput.cardStorageTargets,
              undefined,
              pendingCardResourceInput.selectedAmount,
              sources,
            );
          } else {
            const action = useGameStore
              .getState()
              .currentPlayer?.actions.find(
                (a) =>
                  a.cardId === pendingCardResourceInput.cardId &&
                  a.behaviorIndex === pendingCardResourceInput.behaviorIndex,
              );
            if (!action) {
              throw new Error("Card action is no longer available");
            }
            await finalizeCardActionInputs(
              action,
              pendingCardResourceInput.choiceIndex,
              pendingCardResourceInput.cardStorageTargets,
              pendingCardResourceInput.selectedAmount,
              sources,
              pendingCardResourceInput.reuseSourceCardId,
            );
          }
          return;
        }
        if (pendingCardResourceInput.type === "play-card") {
          await globalWebSocketManager.playCard(
            pendingCardResourceInput.cardId,
            pendingCardResourceInput.payment,
            pendingCardResourceInput.choiceIndex,
            [...(pendingCardResourceInput.cardStorageTargets || []), sourceCardId],
            undefined,
            pendingCardResourceInput.selectedAmount,
          );
        } else {
          await globalWebSocketManager.playCardAction(
            pendingCardResourceInput.cardId,
            pendingCardResourceInput.behaviorIndex,
            pendingCardResourceInput.choiceIndex,
            pendingCardResourceInput.cardStorageTargets,
            undefined,
            sourceCardId,
            undefined,
            undefined,
            pendingCardResourceInput.reuseSourceCardId,
          );
        }
        store.setPendingCardResourceInput(null);
        activeReuseSourceCardId.current = undefined;
      } catch (error) {
        const active = store.playSession;
        if (active) {
          useCardPlayFlowStore.getState().returnPlay(active.id, error);
        }
        console.error(
          `Failed to play action ${pendingCardResourceInput.cardId} with source card ${sourceCardId}:`,
          error,
        );
        store.setPendingCardResourceInput(null);
        activeReuseSourceCardId.current = undefined;
      }
    },
    [finalizePlayCard, finalizeCardActionInputs],
  );

  const handleCardResourceCancel = useCallback(() => {
    const store = useCardPlayFlowStore.getState();
    if (store.playSession) {
      store.returnPlay(store.playSession.id);
    }
    store.setShowCardResourceSelection(false);
    store.setPendingCardResourceInput(null);
    activeReuseSourceCardId.current = undefined;
  }, []);

  const beginCardAction = useCallback(
    (action: PlayerActionDto, reuseSourceCardId?: string) => {
      const store = useCardPlayFlowStore.getState();
      activeReuseSourceCardId.current = reuseSourceCardId;
      if (action.behavior.choices && action.behavior.choices.length > 0) {
        store.setActionPendingChoice(action);
        store.setShowActionChoiceSelection(true);
        return;
      }
      void finalizeCardActionInputs(
        action,
        undefined,
        undefined,
        undefined,
        [],
        reuseSourceCardId,
      ).catch((error) => {
        activeReuseSourceCardId.current = undefined;
        console.error("Failed to use card action:", error);
      });
    },
    [finalizeCardActionInputs],
  );

  const handleActionSelect = useCallback(
    (action: PlayerActionDto) => {
      if (useGameStore.getState().currentPlayer?.pendingTileSelection) {
        return;
      }
      const store = useCardPlayFlowStore.getState();
      activeReuseSourceCardId.current = undefined;
      if (action.behavior.outputs?.some((output) => output.type === "action-reuse")) {
        store.setPendingActionReuse({ cardId: action.cardId, behaviorIndex: action.behaviorIndex });
        store.setShowActionReuseSelection(true);
        return;
      }
      beginCardAction(action);
    },
    [beginCardAction],
  );

  const handleActionReuseSelect = useCallback(
    (target: PlayerActionDto) => {
      const store = useCardPlayFlowStore.getState();
      const source = store.pendingActionReuse;
      if (!source) {
        return;
      }
      store.setShowActionReuseSelection(false);
      store.setPendingActionReuse(null);
      beginCardAction(target, source.cardId);
    },
    [beginCardAction],
  );

  const handleStandardProjectSelect = useCallback((project: StandardProject) => {
    const cp = useGameStore.getState().currentPlayer;
    if (cp?.pendingTileSelection) {
      return;
    }

    void globalWebSocketManager.standardProject(project);
  }, []);

  const handleConvertPlantsToGreenery = useCallback(() => {
    const cp = useGameStore.getState().currentPlayer;
    if (cp?.pendingTileSelection) {
      return;
    }

    void globalWebSocketManager.convertPlantsToGreenery();
  }, []);

  const handleConvertHeatToTemperature = useCallback(() => {
    void globalWebSocketManager.convertHeatToTemperature();
  }, []);

  return {
    handlePlayCard,
    handleChoiceSelect,
    handleChoiceCancel,
    handleCardStorageSelect,
    handleCardStorageCancel,
    handleTargetPlayerSelect,
    handleTargetPlayerCancel,
    handleAmountSelect,
    handleAmountCancel,
    handleActionChoiceSelect,
    handleActionChoiceCancel,
    handleActionStorageSelect,
    handleActionStorageCancel,
    handleActionTargetPlayerSelect,
    handleActionTargetPlayerCancel,
    handleCardResourceSelect,
    handleCardResourceCancel,
    handleActionReuseSelect,
    handleActionReuseCancel,
    handleColonyResourceSelect,
    handleColonyResourceSkip,
    handleStandardProjectSelect,
    handleConvertPlantsToGreenery,
    handleConvertHeatToTemperature,
    handleActionSelect,
    finalizePlayCard,
  };
}
