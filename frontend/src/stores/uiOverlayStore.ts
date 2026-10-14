import { create } from "zustand";

interface UIOverlayState {
  showCardsPlayedModal: boolean;
  showDebugDropdown: boolean;
  showUIDebug: boolean;
  showPerformanceWindow: boolean;
  showFeedbackWindow: boolean;
  showCorporationOverlay: boolean;
  showLeaveGameConfirm: boolean;
  showCloseGameConfirm: boolean;
  showEndGameConfirm: boolean;
  showProductionPhaseModal: boolean;
  isProductionModalHidden: boolean;
  openProductionToCardSelection: boolean;
  showStartingSelection: boolean;
  isStartingSelectionHidden: boolean;
  showPendingCardSelection: boolean;
  showCardDrawSelection: boolean;
  showResourceRemovalSelection: boolean;
  showColonyResourceSelection: boolean;
  showColonyPlacementSelection: boolean;
  showFreeTradeSelection: boolean;
  showTabConflict: boolean;
  conflictingTabInfo: { gameId: string; playerName: string } | null;
  showCorporationModal: boolean;
  showCardBrowser: boolean;
  showServerDown: boolean;
  showServerSelector: boolean;

  setShowCardsPlayedModal: (show: boolean) => void;
  setShowDebugDropdown: (show: boolean) => void;
  setShowUIDebug: (show: boolean) => void;
  setShowPerformanceWindow: (show: boolean) => void;
  setShowFeedbackWindow: (show: boolean) => void;
  setShowCorporationOverlay: (show: boolean) => void;
  setShowLeaveGameConfirm: (show: boolean) => void;
  setShowCloseGameConfirm: (show: boolean) => void;
  setShowEndGameConfirm: (show: boolean) => void;
  requestLeaveGame: () => void;
  requestEndGame: () => void;
  setShowProductionPhaseModal: (show: boolean) => void;
  setIsProductionModalHidden: (hidden: boolean) => void;
  setOpenProductionToCardSelection: (open: boolean) => void;
  setShowStartingSelection: (show: boolean) => void;
  setIsStartingSelectionHidden: (hidden: boolean) => void;
  setShowPendingCardSelection: (show: boolean) => void;
  setShowCardDrawSelection: (show: boolean) => void;
  setShowResourceRemovalSelection: (show: boolean) => void;
  setShowColonyResourceSelection: (show: boolean) => void;
  setShowColonyPlacementSelection: (show: boolean) => void;
  setShowFreeTradeSelection: (show: boolean) => void;
  setShowTabConflict: (show: boolean) => void;
  setConflictingTabInfo: (info: { gameId: string; playerName: string } | null) => void;
  setShowCorporationModal: (show: boolean) => void;
  setShowCardBrowser: (show: boolean) => void;
  setShowServerDown: (show: boolean) => void;
  setShowServerSelector: (show: boolean) => void;
  toggleShowDebugDropdown: () => void;
  toggleShowPerformanceWindow: () => void;
  toggleShowFeedbackWindow: () => void;
}

export const useUIOverlayStore = create<UIOverlayState>((set) => ({
  showCardsPlayedModal: false,
  showDebugDropdown: false,
  showUIDebug: false,
  showPerformanceWindow: false,
  showFeedbackWindow: false,
  showCorporationOverlay: false,
  showLeaveGameConfirm: false,
  showCloseGameConfirm: false,
  showEndGameConfirm: false,
  showProductionPhaseModal: false,
  isProductionModalHidden: false,
  openProductionToCardSelection: false,
  showStartingSelection: false,
  isStartingSelectionHidden: false,
  showPendingCardSelection: false,
  showCardDrawSelection: false,
  showResourceRemovalSelection: false,
  showColonyResourceSelection: false,
  showColonyPlacementSelection: false,
  showFreeTradeSelection: false,
  showTabConflict: false,
  conflictingTabInfo: null,
  showCorporationModal: false,
  showCardBrowser: false,
  showServerDown: false,
  showServerSelector: false,

  setShowCardsPlayedModal: (show) => set({ showCardsPlayedModal: show }),
  setShowDebugDropdown: (show) => set({ showDebugDropdown: show }),
  setShowUIDebug: (show) => set({ showUIDebug: show }),
  setShowPerformanceWindow: (show) => set({ showPerformanceWindow: show }),
  setShowFeedbackWindow: (show) => set({ showFeedbackWindow: show }),
  setShowCorporationOverlay: (show) => set({ showCorporationOverlay: show }),
  setShowLeaveGameConfirm: (show) => set({ showLeaveGameConfirm: show }),
  setShowCloseGameConfirm: (show) => set({ showCloseGameConfirm: show }),
  setShowEndGameConfirm: (show) => set({ showEndGameConfirm: show }),
  requestLeaveGame: () => set({ showEndGameConfirm: false, showLeaveGameConfirm: true }),
  requestEndGame: () => set({ showLeaveGameConfirm: false, showEndGameConfirm: true }),
  setShowProductionPhaseModal: (show) => set({ showProductionPhaseModal: show }),
  setIsProductionModalHidden: (hidden) => set({ isProductionModalHidden: hidden }),
  setOpenProductionToCardSelection: (open) => set({ openProductionToCardSelection: open }),
  setShowStartingSelection: (show) => set({ showStartingSelection: show }),
  setIsStartingSelectionHidden: (hidden) => set({ isStartingSelectionHidden: hidden }),
  setShowPendingCardSelection: (show) => set({ showPendingCardSelection: show }),
  setShowCardDrawSelection: (show) => set({ showCardDrawSelection: show }),
  setShowResourceRemovalSelection: (show) => set({ showResourceRemovalSelection: show }),
  setShowColonyResourceSelection: (show) => set({ showColonyResourceSelection: show }),
  setShowColonyPlacementSelection: (show) => set({ showColonyPlacementSelection: show }),
  setShowFreeTradeSelection: (show) => set({ showFreeTradeSelection: show }),
  setShowTabConflict: (show) => set({ showTabConflict: show }),
  setConflictingTabInfo: (info) => set({ conflictingTabInfo: info }),
  setShowCorporationModal: (show) => set({ showCorporationModal: show }),
  setShowCardBrowser: (show) => set({ showCardBrowser: show }),
  setShowServerDown: (show) => set({ showServerDown: show }),
  setShowServerSelector: (show) => set({ showServerSelector: show }),
  toggleShowDebugDropdown: () => set((s) => ({ showDebugDropdown: !s.showDebugDropdown })),
  toggleShowPerformanceWindow: () =>
    set((s) => ({ showPerformanceWindow: !s.showPerformanceWindow })),
  toggleShowFeedbackWindow: () => set((s) => ({ showFeedbackWindow: !s.showFeedbackWindow })),
}));
