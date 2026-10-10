import { create } from "zustand";
import type {
  PaymentDto,
  PlayerActionDto,
  PlayerCardDto,
  ResourceType,
} from "@/types/generated/api-types.ts";

export interface CardStoragePending {
  cardId: string;
  choiceIndex?: number;
  allStorageNeeds: Array<{
    resourceType: ResourceType;
    amount: number;
    selectorTags?: string[];
    eligibleCardIds?: string[];
  }>;
  collectedTargets: string[];
  currentIndex: number;
  resourceType: ResourceType;
  amount: number;
  selectorTags?: string[];
  eligibleCardIds?: string[];
}

export interface TargetPlayerPending {
  cardId: string;
  payment: PaymentDto | undefined;
  choiceIndex?: number;
  cardStorageTargets?: string[];
  cardStorageSources?: string[];
  selectedAmount?: number;
  resourceType: ResourceType;
  amount: number;
  isSteal: boolean;
}

export interface ActionStoragePending {
  reuseSourceCardId?: string;
  selectedAmount?: number;
  cardStorageSources?: string[];
  cardId: string;
  behaviorIndex: number;
  choiceIndex?: number;
  allStorageNeeds: Array<{
    resourceType: ResourceType;
    amount: number;
    selectorTags?: string[];
    eligibleCardIds?: string[];
  }>;
  collectedTargets: string[];
  currentIndex: number;
  resourceType: ResourceType;
  amount: number;
  selectorTags?: string[];
  eligibleCardIds?: string[];
}

export interface ActionTargetPlayerPending {
  selectedAmount?: number;
  reuseSourceCardId?: string;
  cardId: string;
  behaviorIndex: number;
  choiceIndex?: number;
  cardStorageTargets?: string[];
  cardStorageSources?: string[];
  resourceType: ResourceType;
  amount: number;
  isSteal: boolean;
}

export type CardResourceInputPending = {
  eligibleInputCardIds?: string[];
  selectedAmount?: number;
  cardId: string;
  choiceIndex?: number;
  cardStorageTargets?: string[];
  cardStorageSources?: string[];
  resourceType: ResourceType;
  amount: number;
} & (
  | {
      type: "play-card";
      payment: PaymentDto | undefined;
    }
  | {
      type: "card-action";
      behaviorIndex: number;
      reuseSourceCardId?: string;
    }
);

export interface VariableAmountPending {
  reuseSourceCardId?: string;
  type: "play-card" | "card-action";
  cardId: string;
  payment?: PaymentDto;
  choiceIndex?: number;
  cardStorageTargets?: string[];
  cardStorageSources?: string[];
  behaviorIndex?: number;
  resourceType: ResourceType;
  maxAmount: number;
}

export interface CardPlaySession {
  id: string;
  card: PlayerCardDto;
  source: HTMLElement;
  sourceTransform: string;
  origin: { x: number; y: number; scale: number };
  phase: "preparing" | "choosing" | "submitting" | "returning" | "reconnecting";
  inspected: boolean;
  inspectionReady: boolean;
  animationDone: boolean;
  confirmed: boolean;
}

interface CardPlayFlowState {
  playSession: CardPlaySession | null;
  playError: string | null;
  playPromptHost: HTMLElement | null;
  beginPlay: (card: PlayerCardDto, source: HTMLElement) => boolean;
  showPlayPrompt: (cardId: string) => void;
  submitPlay: (sessionId: string) => boolean;
  returnPlay: (sessionId: string, error?: unknown) => void;
  inspectionArrived: (sessionId: string) => void;
  finishPlayAnimation: (sessionId: string) => void;
  confirmPlay: (sessionId: string) => void;
  suspendPlay: () => void;
  clearPlayPresentation: () => void;
  dismissPlayError: () => void;
  setPlayPromptHost: (host: HTMLElement | null) => void;

  // Card play flow
  showChoiceSelection: boolean;
  cardPendingChoice: PlayerCardDto | null;
  pendingCardBehaviorIndex: number;

  showCardStorageSelection: boolean;
  pendingCardStorage: CardStoragePending | null;

  showTargetPlayerSelection: boolean;
  pendingTargetPlayer: TargetPlayerPending | null;

  showAmountSelection: boolean;
  pendingVariableAmount: VariableAmountPending | null;

  // Action flow
  showActionChoiceSelection: boolean;
  actionPendingChoice: PlayerActionDto | null;

  showActionStorageSelection: boolean;
  pendingActionStorage: ActionStoragePending | null;

  showActionTargetPlayerSelection: boolean;
  pendingActionTargetPlayer: ActionTargetPlayerPending | null;

  showCardResourceSelection: boolean;
  pendingCardResourceInput: CardResourceInputPending | null;

  showActionReuseSelection: boolean;
  pendingActionReuse: { cardId: string; behaviorIndex: number } | null;

  showFreeTradeWarning: boolean;
  pendingFreeTradeWarning: string | null;

  // Resource conversion payment flow

  // Behavior choice flow (passive triggered)

  // Setters
  setShowChoiceSelection: (show: boolean) => void;
  setCardPendingChoice: (card: PlayerCardDto | null) => void;
  setPendingCardBehaviorIndex: (index: number) => void;
  setShowCardStorageSelection: (show: boolean) => void;
  setPendingCardStorage: (pending: CardStoragePending | null) => void;
  setShowTargetPlayerSelection: (show: boolean) => void;
  setPendingTargetPlayer: (pending: TargetPlayerPending | null) => void;
  setShowAmountSelection: (show: boolean) => void;
  setPendingVariableAmount: (pending: VariableAmountPending | null) => void;
  setShowActionChoiceSelection: (show: boolean) => void;
  setActionPendingChoice: (action: PlayerActionDto | null) => void;
  setShowActionStorageSelection: (show: boolean) => void;
  setPendingActionStorage: (pending: ActionStoragePending | null) => void;
  setShowActionTargetPlayerSelection: (show: boolean) => void;
  setPendingActionTargetPlayer: (pending: ActionTargetPlayerPending | null) => void;
  setShowCardResourceSelection: (show: boolean) => void;
  setPendingCardResourceInput: (pending: CardResourceInputPending | null) => void;
  setShowActionReuseSelection: (show: boolean) => void;
  setPendingActionReuse: (pending: { cardId: string; behaviorIndex: number } | null) => void;
  setShowFreeTradeWarning: (show: boolean) => void;
  setPendingFreeTradeWarning: (warning: string | null) => void;

  resetCardPlayFlow: () => void;
  resetActionFlow: () => void;
  resetAll: () => void;
}

const presentationInitial = {
  playSession: null,
  playError: null,
  playPromptHost: null,
};

const cardPlayFlowInitial = {
  showChoiceSelection: false,
  cardPendingChoice: null,
  pendingCardBehaviorIndex: 0,
  showCardStorageSelection: false,
  pendingCardStorage: null,
  showTargetPlayerSelection: false,
  pendingTargetPlayer: null,
  showAmountSelection: false,
  pendingVariableAmount: null,
};

const actionFlowInitial = {
  showActionChoiceSelection: false,
  actionPendingChoice: null,
  showActionStorageSelection: false,
  pendingActionStorage: null,
  showActionTargetPlayerSelection: false,
  pendingActionTargetPlayer: null,
  showCardResourceSelection: false,
  pendingCardResourceInput: null,
  showActionReuseSelection: false,
  pendingActionReuse: null,
  showFreeTradeWarning: false,
  pendingFreeTradeWarning: null,
};

const conversionInitial = {};

const behaviorChoiceInitial = {};

const allInitial = {
  ...presentationInitial,
  ...cardPlayFlowInitial,
  ...actionFlowInitial,
  ...conversionInitial,
  ...behaviorChoiceInitial,
};

export const useCardPlayFlowStore = create<CardPlayFlowState>((set, get) => ({
  ...allInitial,

  beginPlay: (card, source) => {
    if (get().playSession) {
      return false;
    }
    const bounds = source.getBoundingClientRect();
    set({
      playError: null,
      playSession: {
        id: crypto.randomUUID(),
        card,
        source,
        sourceTransform: source.style.transform,
        origin: { x: bounds.x, y: bounds.y, scale: bounds.width / source.offsetWidth },
        phase: "preparing",
        inspected: false,
        inspectionReady: false,
        animationDone: false,
        confirmed: false,
      },
    });
    return true;
  },
  showPlayPrompt: (cardId) =>
    set(({ playSession }) => {
      if (
        !playSession ||
        playSession.card.id !== cardId ||
        (playSession.phase !== "preparing" && playSession.phase !== "choosing")
      ) {
        return {};
      }
      return { playSession: { ...playSession, phase: "choosing", inspected: true } };
    }),
  submitPlay: (sessionId) => {
    const session = get().playSession;
    if (
      !session ||
      session.id !== sessionId ||
      (session.phase !== "preparing" && session.phase !== "choosing")
    ) {
      return false;
    }
    set({ playSession: { ...session, phase: "submitting" } });
    return true;
  },
  returnPlay: (sessionId, error) => {
    const session = get().playSession;
    if (
      !session ||
      session.id !== sessionId ||
      session.confirmed ||
      session.phase === "returning"
    ) {
      return;
    }
    let message: string | null = null;
    if (error instanceof Error) {
      message = error.message;
    } else if (typeof error === "string") {
      message = error;
    }
    set({
      ...cardPlayFlowInitial,
      showCardResourceSelection: false,
      pendingCardResourceInput: null,
      playError: message === "Payment cancelled" ? null : message,
      playSession: { ...session, phase: "returning", animationDone: false },
    });
  },
  inspectionArrived: (sessionId) =>
    set(({ playSession }) =>
      playSession?.id === sessionId && !playSession.inspectionReady
        ? { playSession: { ...playSession, inspectionReady: true } }
        : {},
    ),
  finishPlayAnimation: (sessionId) =>
    set(({ playSession }) => {
      if (playSession?.id !== sessionId) {
        return {};
      }
      return {
        playSession:
          playSession.confirmed || playSession.phase === "returning"
            ? null
            : { ...playSession, animationDone: true },
      };
    }),
  confirmPlay: (sessionId) =>
    set(({ playSession }) => {
      if (playSession?.id !== sessionId) {
        return {};
      }
      return {
        playSession: playSession.animationDone
          ? null
          : { ...playSession, phase: "submitting", confirmed: true },
      };
    }),
  suspendPlay: () =>
    set(({ playSession }) =>
      playSession?.phase === "submitting" && !playSession.confirmed
        ? { playSession: { ...playSession, phase: "reconnecting" } }
        : {},
    ),
  clearPlayPresentation: () =>
    set({
      ...presentationInitial,
      ...cardPlayFlowInitial,
      showCardResourceSelection: false,
      pendingCardResourceInput: null,
    }),
  dismissPlayError: () => set({ playError: null }),
  setPlayPromptHost: (playPromptHost) => set({ playPromptHost }),

  setShowChoiceSelection: (show) => {
    const state = get();
    if (show && state.playSession) {
      state.showPlayPrompt(state.playSession.card.id);
    }
    set({ showChoiceSelection: show });
  },
  setCardPendingChoice: (card) => set({ cardPendingChoice: card }),
  setPendingCardBehaviorIndex: (index) => set({ pendingCardBehaviorIndex: index }),
  setShowCardStorageSelection: (show) => {
    const state = get();
    if (show && state.playSession) {
      state.showPlayPrompt(state.playSession.card.id);
    }
    set({ showCardStorageSelection: show });
  },
  setPendingCardStorage: (pending) => set({ pendingCardStorage: pending }),
  setShowTargetPlayerSelection: (show) => {
    const state = get();
    if (show && state.playSession) {
      state.showPlayPrompt(state.playSession.card.id);
    }
    set({ showTargetPlayerSelection: show });
  },
  setPendingTargetPlayer: (pending) => set({ pendingTargetPlayer: pending }),
  setShowAmountSelection: (show) => {
    const state = get();
    if (show && state.playSession && state.pendingVariableAmount?.type === "play-card") {
      state.showPlayPrompt(state.playSession.card.id);
    }
    set({ showAmountSelection: show });
  },
  setPendingVariableAmount: (pending) => set({ pendingVariableAmount: pending }),
  setShowActionChoiceSelection: (show) => set({ showActionChoiceSelection: show }),
  setActionPendingChoice: (action) => set({ actionPendingChoice: action }),
  setShowActionStorageSelection: (show) => set({ showActionStorageSelection: show }),
  setPendingActionStorage: (pending) => set({ pendingActionStorage: pending }),
  setShowActionTargetPlayerSelection: (show) => set({ showActionTargetPlayerSelection: show }),
  setPendingActionTargetPlayer: (pending) => set({ pendingActionTargetPlayer: pending }),
  setShowCardResourceSelection: (show) => {
    const state = get();
    if (show && state.playSession && state.pendingCardResourceInput?.type === "play-card") {
      state.showPlayPrompt(state.playSession.card.id);
    }
    set({ showCardResourceSelection: show });
  },
  setPendingCardResourceInput: (pending) => set({ pendingCardResourceInput: pending }),
  setShowActionReuseSelection: (show) => set({ showActionReuseSelection: show }),
  setPendingActionReuse: (pending) => set({ pendingActionReuse: pending }),
  setShowFreeTradeWarning: (show) => set({ showFreeTradeWarning: show }),
  setPendingFreeTradeWarning: (warning) => set({ pendingFreeTradeWarning: warning }),

  resetCardPlayFlow: () => set(cardPlayFlowInitial),
  resetActionFlow: () => set(actionFlowInitial),
  resetAll: () => set(allInitial),
}));
