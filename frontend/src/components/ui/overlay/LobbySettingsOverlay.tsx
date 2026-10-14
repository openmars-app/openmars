import React, { useState, useRef, useEffect } from "react";
import type { GameDto, UpdateGameSettingsRequest } from "@/types/generated/api-types.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import GameButton from "../buttons/GameButton.tsx";
import InfoTooltip from "../display/InfoTooltip.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useBotThoughtsPreferenceStore } from "@/stores/botPresenceStore.ts";
import {
  OVERLAY_CONTAINER_CLASS,
  OVERLAY_CONTAINER_STYLE,
  OVERLAY_FOOTER_CLASS,
  OVERLAY_HEADER_CLASS,
  OVERLAY_TITLE_CLASS,
} from "./overlayStyles.ts";

interface LobbySettingsOverlayProps {
  game: GameDto;
  playerId: string;
  isOpen: boolean;
  onClose: () => void;
}

type Tab = "general" | "ai";

const LobbySettingsOverlay: React.FC<LobbySettingsOverlayProps> = (props) => {
  const showBotThoughts = useBotThoughtsPreferenceStore((s) => s.showBotThoughts);
  const setShowBotThoughts = useBotThoughtsPreferenceStore((s) => s.setShowBotThoughts);
  return (
    <LobbySettingsOverlayView
      {...props}
      showBotThoughts={showBotThoughts}
      onBotThoughtsChange={setShowBotThoughts}
      onUpdateSettings={(patch) => {
        void globalWebSocketManager.updateGameSettings(patch);
      }}
    />
  );
};

export function LobbySettingsOverlayView({
  game,
  playerId,
  isOpen,
  onClose,
  showBotThoughts,
  onBotThoughtsChange: setShowBotThoughts,
  onUpdateSettings: dispatch,
}: LobbySettingsOverlayProps & {
  showBotThoughts: boolean;
  onBotThoughtsChange: (value: boolean) => void;
  onUpdateSettings: (patch: UpdateGameSettingsRequest) => void;
}) {
  const tokenResetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (tokenResetTimer.current) {
        clearTimeout(tokenResetTimer.current);
      }
    },
    [],
  );
  const isHost = game.hostPlayerId === playerId;
  const [activeTab, setActiveTab] = useState<Tab>("general");
  const [claudeToken, setClaudeToken] = useState("");
  const [tokenStatus, setTokenStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [spendCapDraft, setSpendCapDraft] = useState<string | null>(null);

  if (!isOpen || !isHost) {
    return null;
  }

  const settings = game.settings;
  const mode = settings.availableGameModes.find((item) => item.id === settings.gameModeId);
  const playerCount = (game.currentPlayer?.id ? 1 : 0) + (game.otherPlayers?.length || 0);

  const handleSubmitToken = () => {
    if (!claudeToken.trim()) {
      return;
    }
    setTokenStatus("saving");
    dispatch({ claudeOAuthToken: claudeToken.trim() });
    setClaudeToken("");
    setTokenStatus("saved");
    if (tokenResetTimer.current) {
      clearTimeout(tokenResetTimer.current);
    }
    tokenResetTimer.current = setTimeout(() => setTokenStatus("idle"), 1500);
  };

  const handleClearToken = () => {
    dispatch({ claudeOAuthToken: "" });
  };

  const commitSpendCap = () => {
    if (spendCapDraft === null) {
      return;
    }
    const value = Number(spendCapDraft);
    setSpendCapDraft(null);
    if (!Number.isFinite(value) || value <= 0 || value === settings.botSpendCapUsd) {
      return;
    }
    dispatch({ botSpendCapUsd: value });
  };

  const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

  const handleMaxPlayersChange = (raw: number) => {
    const value = clamp(Math.round(raw), 1, mode?.maxPlayers ?? 5);
    if (value === settings.maxPlayers) {
      return;
    }
    dispatch({ maxPlayers: value });
  };

  const tabClass = (tab: Tab) =>
    `text-left px-4 py-3 text-sm font-semibold uppercase tracking-wide transition-colors cursor-pointer max-[640px]:shrink-0 ${
      activeTab === tab
        ? "text-white bg-space-blue-600/30 border-r-2 border-space-blue-400 max-[640px]:border-r-0 max-[640px]:border-b-2"
        : "text-white/50 hover:text-white/80 hover:bg-white/5"
    }`;

  const minPlayersAllowed = Math.max(1, playerCount);

  return (
    <div
      className="fixed inset-0 flex items-center justify-center bg-black/70 backdrop-blur-sm animate-[fadeIn_0.3s_ease] pt-[var(--safe-top)] pr-[var(--safe-right)] pb-[var(--safe-bottom)] pl-[var(--safe-left)]"
      style={{ zIndex: Z_INDEX.LOBBY_SETTINGS_MODAL }}
    >
      <div
        className={`${OVERLAY_CONTAINER_CLASS} max-w-[1100px] h-[80dvh] max-[640px]:h-full max-[640px]:max-h-full [@media(max-height:500px)]:w-full [@media(max-height:500px)]:h-full [@media(max-height:500px)]:max-h-full`}
        style={OVERLAY_CONTAINER_STYLE}
      >
        <div className={`${OVERLAY_HEADER_CLASS} [@media(max-height:500px)]:py-3`}>
          <h2 className={`${OVERLAY_TITLE_CLASS} [@media(max-height:500px)]:text-xl`}>
            Lobby Settings
          </h2>
        </div>

        <div className="flex-1 flex min-h-0 max-[640px]:flex-col">
          <div className="w-44 shrink-0 bg-black/30 border-r border-space-blue-600/50 flex flex-col py-2 max-[640px]:w-full max-[640px]:flex-row max-[640px]:overflow-x-auto max-[640px]:py-0 max-[640px]:border-r-0 max-[640px]:border-b [@media(max-height:500px)_and_(min-width:641px)]:w-32">
            <GameButton
              emphasis="quiet"
              onClick={() => setActiveTab("general")}
              className={tabClass("general")}
            >
              General
            </GameButton>
            <GameButton
              emphasis="quiet"
              onClick={() => setActiveTab("ai")}
              className={tabClass("ai")}
            >
              AI
            </GameButton>
          </div>

          <div className="flex-1 overflow-y-auto overscroll-contain p-6 min-h-0 max-[640px]:p-4 [@media(max-height:500px)]:p-4">
            {activeTab === "general" && (
              <div className="max-w-xl mx-auto space-y-4">
                <div className="bg-black/40 border border-space-blue-600/50 rounded-xl p-4">
                  <h3 className="text-white font-semibold mb-3 uppercase tracking-wide text-xs">
                    Lobby
                  </h3>
                  <label className="flex items-center justify-between gap-3 py-2 px-1 compact:min-h-11">
                    <span className="text-white text-sm font-medium flex items-center gap-2">
                      Max Players
                      <InfoTooltip size="small">
                        Cannot be reduced below the number of players who have already joined.
                      </InfoTooltip>
                    </span>
                    <input
                      type="number"
                      min={minPlayersAllowed}
                      max={mode?.maxPlayers ?? 5}
                      value={settings.maxPlayers}
                      onChange={(e) => handleMaxPlayersChange(parseInt(e.target.value, 10) || 0)}
                      className="game-input w-16 !py-1 !px-2 text-sm text-center pointer-coarse:text-base"
                    />
                  </label>
                </div>

                <div className="bg-black/40 border border-space-blue-600/50 rounded-xl p-4">
                  <h3 className="text-white font-semibold mb-3 uppercase tracking-wide text-xs">
                    Toggles
                  </h3>
                  <ToggleRow
                    label="Development Mode"
                    checked={settings.developmentMode}
                    onChange={(v) => dispatch({ developmentMode: v })}
                    tooltip="Enable admin commands for debugging and testing."
                  />
                  <ToggleRow
                    label="Demo Game"
                    checked={settings.demoGame}
                    onChange={(v) => dispatch({ demoGame: v })}
                    tooltip="Players configure their corporation, starting cards, resources and production in the lobby before starting."
                  />
                  <ToggleRow
                    label="Allow Random Buy"
                    checked={settings.allowRandomBuy}
                    onChange={(v) => dispatch({ allowRandomBuy: v })}
                    tooltip="When selecting 0 cards to buy at the end of a generation, players can choose to buy a single random card instead."
                  />
                </div>
              </div>
            )}

            {activeTab === "ai" && (
              <div className="max-w-xl mx-auto space-y-4">
                <div className="bg-black/40 border border-space-blue-600/50 rounded-xl p-4">
                  <h3 className="text-white font-semibold mb-3 uppercase tracking-wide text-xs">
                    Claude OAuth token
                  </h3>
                  <p className="text-white/70 text-sm mb-1">
                    {settings.hasClaudeOAuthToken
                      ? "A Claude OAuth token is configured. Paste a new one to replace it, or clear to disable bots."
                      : "Paste a Claude OAuth token to enable bot players. Bots use this token for every action they take."}
                  </p>
                  <p className="text-white/50 text-xs mb-3">
                    Generate one by running{" "}
                    <code className="text-white/80">claude setup-token</code> in a terminal.
                  </p>
                  <div className="flex gap-2">
                    <input
                      type="password"
                      value={claudeToken}
                      onChange={(e) => setClaudeToken(e.target.value)}
                      placeholder="sk-ant-..."
                      spellCheck={false}
                      autoComplete="off"
                      className="flex-1 min-w-0 bg-black/50 border border-white/20 rounded-none py-2 px-3 text-white text-sm outline-none focus:border-white/60 placeholder:text-white/30 pointer-coarse:text-base"
                    />
                    <GameButton
                      emphasis="primary"
                      size="sm"
                      onClick={handleSubmitToken}
                      disabled={!claudeToken.trim() || tokenStatus === "saving"}
                    >
                      {tokenStatus === "saved" ? "Saved" : "Save"}
                    </GameButton>
                    {settings.hasClaudeOAuthToken && (
                      <GameButton
                        emphasis="secondary"
                        tone="error"
                        size="sm"
                        onClick={handleClearToken}
                      >
                        Clear
                      </GameButton>
                    )}
                  </div>
                </div>
                <div className="bg-black/40 border border-space-blue-600/50 rounded-xl p-4">
                  <h3 className="text-white font-semibold mb-3 uppercase tracking-wide text-xs">
                    Bot spend cap (USD)
                  </h3>
                  <div className="flex items-center gap-3">
                    <input
                      type="number"
                      min={0.01}
                      step={0.5}
                      value={spendCapDraft ?? String(settings.botSpendCapUsd)}
                      onChange={(e) => setSpendCapDraft(e.target.value)}
                      onBlur={commitSpendCap}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          commitSpendCap();
                        }
                      }}
                      aria-label="Bot spend cap in USD"
                      className="w-28 shrink-0 bg-black/50 border border-white/20 rounded-none py-2 px-3 text-white text-sm font-orbitron outline-none focus:border-white/60 cursor-text pointer-coarse:text-base"
                    />
                    <span className="text-white/70 text-sm font-orbitron">
                      Spent so far: ${settings.botSpendUsd.toFixed(2)}
                    </span>
                  </div>
                </div>
                <div className="bg-black/40 border border-space-blue-600/50 rounded-xl p-4">
                  <ToggleRow
                    label="Show bot thoughts"
                    checked={showBotThoughts}
                    onChange={setShowBotThoughts}
                    tooltip="Show short thought bubbles above bot player cards. This only affects your own view."
                  />
                </div>
                <div className="bg-yellow-900/30 border border-yellow-600/40 rounded-lg p-3 text-yellow-200/90 text-xs leading-relaxed">
                  Bots consume Claude usage that bills against your account. Bots switch to a simple
                  autopilot if the spend cap or your usage limit is hit. Open Mars is not
                  responsible for charges incurred while running bots.
                </div>
              </div>
            )}
          </div>
        </div>

        <div className={`${OVERLAY_FOOTER_CLASS} [@media(max-height:500px)]:py-3`}>
          <div className="text-white/60 text-sm">All changes save automatically.</div>
          <GameButton emphasis="primary" size="md" onClick={onClose}>
            Close
          </GameButton>
        </div>
      </div>
    </div>
  );
}

interface ToggleRowProps {
  label: string;
  checked: boolean;
  onChange: (next: boolean) => void;
  tooltip?: string;
}

const ToggleRow: React.FC<ToggleRowProps> = ({ label, checked, onChange, tooltip }) => (
  <label className="flex items-center gap-3 cursor-pointer py-2 px-1 rounded hover:bg-white/5 transition-all duration-200 compact:min-h-11">
    <input
      type="checkbox"
      checked={checked}
      onChange={(e) => onChange(e.target.checked)}
      className="game-checkbox m-0"
    />
    <span className="text-white text-sm font-medium leading-none flex items-center gap-2">
      {label}
      {tooltip ? <InfoTooltip size="small">{tooltip}</InfoTooltip> : null}
    </span>
  </label>
);

export default LobbySettingsOverlay;
