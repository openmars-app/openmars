/**
 * Centralized z-index values for consistent layering across the application
 *
 * Organized in logical groups with clear separation between layers:
 * - Base layer (0-9): Game board, background elements
 * - UI layer (10-99): Basic UI components, overlays
 * - Navigation layer (100-199): Menu bars, sidebars
 * - Overlay layer (200-999): Tooltips, dropdowns, cards
 * - Modal layer (1000-8999): Standard modals, popups
 * - App layer (35000+): App-wide views, confirmations and critical overlays
 */

export const Z_INDEX = {
  CONTROL_DECORATION: -1,
  // Orders siblings inside one component's own stacking context: getZIndex("LOCAL", n)
  LOCAL: 0,
  // Base Layer (0-9)
  GAME_BOARD_BACKGROUND: 0,
  GAME_BOARD_BASE: 1,
  GAME_BOARD_TILES: 2,
  GAME_BOARD_EFFECTS: 5,
  GAME_BOARD_OVERLAY: 10,

  // UI Layer (10-99)
  UI_BASE: 10,
  // Player list in the left sidebar; thought bubbles and emotes sit one above it, so any modal backdrop covers them
  PLAYER_LIST: 10,
  PLAYER_PRESENCE: 11,
  // In-game global parameters sidebar, level with the player list on the left
  RIGHT_SIDEBAR: 10,
  COST_DISPLAY: 20,
  // Compact landing-page "play as an app" bar: above the menu footer, below the menu buttons
  INSTALL_APP_BAR: 30,
  TILE_PLACEMENT_PROMPT: 50,
  // Compact board overlays above the dock: tapped tile info and the placement confirm bar
  PLACEMENT_CONFIRM_BAR: 61,
  PLAYER_OVERLAY: 90,

  // Navigation Layer (100-199)
  // Pre-game corporation showcase: above the board, below selection popovers and the hamburger
  SHOWCASE: 105,
  TOP_MENU_BAR: 100,
  BOTTOM_RESOURCE_BAR: 100,
  LEFT_SIDEBAR: 110,
  // Compact layout HUD bands: top bar, side rails and bottom dock
  MOBILE_HUD: 100,
  // Compact toasts over the board (triggered effects, played cards, player reactions): above the HUD, below screens
  MOBILE_FEEDBACK: 120,
  PLAYER_EFFECT_TOAST: 120,

  // Overlay Layer (200-999)
  CARDS_HAND_OVERLAY: 200,
  CARDS_PREVIEW: 250,
  TOOLTIPS: 300,
  DROPDOWNS: 400,
  CARD_HOVER: 500,
  CARD_SELECTED: 600,

  // Decorative scroll hint above the per-card index band, below highlighted(2000)/dragged(3000)
  CARD_FAN_SCROLL_HINT: 1500,
  CARD_FAN_HIGHLIGHTED: 2000,
  CARD_FAN_DRAGGED: 3000,

  // Modal Layer (1000-8999)
  MENU_DROPDOWN: 1000,
  STANDARD_MODAL: 2000,
  CARD_DETAIL_MODAL: 3000,
  CORPORATION_SELECTION: 5000,
  PENDING_ACTION_BACKDROP: 4500,
  // Compact layout: full-screen views above the HUD bands, card detail above them, prompts (POPOVER+) above both
  MOBILE_SCREEN: 8000,
  MOBILE_CARD_DETAIL: 8500,

  // Popover Layer (10000+)
  POPOVER: 10001,
  SELECTION_BACKDROP: 10002,
  SELECTION_POPOVER: 10003,
  LOBBY_SETTINGS_MODAL: 10500,
  IMMEDIATE_BACKDROP: 30000,
  IMMEDIATE_POPOVER: 30001,
  // Touch card preview opened from a selection popover row; above every gameplay popover
  CARD_PREVIEW_OVERLAY: 30010,

  // Loading overlay - above everything
  LOADING_OVERLAY: 99999,
  FLOATING_TOOLTIP: 99999,
  DRAG_SHIELD: 99999,
  // Asks phone players in portrait to rotate; covers the game, including the loading overlay
  ROTATE_DEVICE_OVERLAY: 100000,

  // Debug Windows Layer (20000-20099)
  DEBUG_WINDOWS: 20000,

  // Always-on-top UI (above debug windows)
  TOP_MENU_ALWAYS_ON_TOP: 20100,
  CORPORATION_OVERLAY: 20200,
  EXPANDED_CARD_FAN: 20201,

  DEBUG_OVERLAY: 9999,

  // "New version available" pill: above the in-game HUD and its menus, below the compact menu drawer
  UPDATE_PILL: 33000,

  // Compact menu drawer: above every gameplay popover, below app overlays and confirmations
  MOBILE_MENU_DRAWER: 34000,

  // App-wide layers stay above all gameplay selections, including immediate flows.
  APP_OVERLAY: 35000,
  CONFIRMATION_MODAL: 40000,
  SYSTEM_NOTIFICATIONS: 41000,
  ERROR_OVERLAYS: 42000,
} as const;

// Type for z-index values
export type ZIndexValue = (typeof Z_INDEX)[keyof typeof Z_INDEX];

// Helper function to get z-index with optional offset
export const getZIndex = (level: keyof typeof Z_INDEX, offset: number = 0): number => {
  return Z_INDEX[level] + offset;
};
