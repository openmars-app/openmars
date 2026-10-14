import { GamePopover } from "../GamePopover";
import { Z_INDEX } from "@/constants/zIndex.ts";
import React, { useRef, useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { GameDto, OtherPlayerDto, PlayerDto } from "../../../types/generated/api-types.ts";
import { globalWebSocketManager } from "../../../services/globalWebSocketManager.ts";
import { CopyLinkButtonView } from "../buttons/CopyLinkButton.tsx";
import GameButton from "../buttons/GameButton.tsx";
import GameMenuModal from "./GameMenuModal.tsx";
import DemoSetupOverlay from "./DemoSetupOverlay.tsx";
import LobbySettingsOverlay from "./LobbySettingsOverlay.tsx";
import LobbyMapInfoPanel from "../lobby/LobbyMapInfoPanel.tsx";
import { BotPersonaChip, PlayerChip } from "../display/BotChips.tsx";
import InlineEmote from "../display/InlineEmote.tsx";
import MainMenuHamburger, { MainMenuDrawerButton } from "../buttons/MainMenuHamburger.tsx";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { serverLink } from "@/utils/gateway.ts";

interface WaitingRoomOverlayProps {
  game: GameDto;
  playerId: string;
  visible?: boolean;
  onExited?: () => void;
  chat?: React.ReactNode;
}

type LobbyTab = "players" | "chat" | "map";

const LOBBY_TABS: { id: LobbyTab; label: string }[] = [
  { id: "players", label: "Players" },
  { id: "chat", label: "Chat" },
  { id: "map", label: "Map" },
];

interface LeavingPlayer {
  id: string;
  name: string;
}

function getOrderedPlayers<T>(playerMap: Map<string, T>, game: GameDto): T[] {
  const order = [game.turnOrder, game.playerOrder].find((o) => o && o.length > 0);
  if (!order) {
    return Array.from(playerMap.values());
  }
  return order.map((pid) => playerMap.get(pid)).filter((p) => p !== undefined);
}

const COLOR_SWATCH_SIZE = 28;
const COMPACT_COLOR_SWATCH_SIZE = 40;
const COLOR_SWATCH_GAP = 6;
const COLOR_SWATCHES_PER_ROW = 5;
const COLOR_PICKER_PADDING = 24;
const COLOR_PICKER_GAP = 15;
const COLOR_PICKER_MIN_WIDTH = 200;

const colorPickerWidth = (swatchSize: number) =>
  COLOR_SWATCHES_PER_ROW * swatchSize + (COLOR_SWATCHES_PER_ROW - 1) * COLOR_SWATCH_GAP;

const ColorPicker: React.FC<{
  colors: string[];
  takenColors: Set<string>;
  currentColor: string;
  swatchSize: number;
  onSelect: (color: string) => void;
}> = ({ colors, takenColors, currentColor, swatchSize, onSelect }) => (
  <div
    className="flex flex-wrap gap-1.5"
    style={{
      width:
        colors.length > COLOR_SWATCHES_PER_ROW ? `${colorPickerWidth(swatchSize)}px` : undefined,
    }}
  >
    {colors.map((color) => {
      const isTaken = takenColors.has(color);
      const isSelected = color === currentColor;
      return (
        <button
          key={color}
          onClick={() => !isTaken && onSelect(color)}
          className={`rounded-full border-2 transition-all flex-shrink-0 ${
            isSelected
              ? "border-white scale-110"
              : isTaken
                ? "border-white/10 opacity-25 cursor-default"
                : "border-transparent cursor-pointer hover:scale-110"
          }`}
          style={{ backgroundColor: color, width: swatchSize, height: swatchSize }}
          disabled={isTaken}
        />
      );
    })}
  </div>
);

const WaitingRoomOverlay: React.FC<WaitingRoomOverlayProps> = (props) => {
  const navigate = useNavigate();
  return (
    <WaitingRoomOverlayView
      {...props}
      onStart={() => {
        void globalWebSocketManager.startGame();
      }}
      onColor={(color, playerId) => {
        void globalWebSocketManager.setPlayerColor(color, playerId);
      }}
      onKick={(id) => {
        void globalWebSocketManager.kickPlayer(id);
      }}
      onKickSpectator={(id) => {
        void globalWebSocketManager.kickSpectator(id);
      }}
      onAddBot={() => {
        void globalWebSocketManager.addBot();
      }}
      onLeave={() => {
        globalWebSocketManager.disconnect();
        void navigate("/");
      }}
      onCopyInvite={(text) => navigator.clipboard.writeText(text)}
      desktopMenu={<MainMenuHamburger gameId={props.game.id} />}
      compactMenu={<MainMenuDrawerButton gameId={props.game.id} />}
      mapPanel={(layout) => (
        <LobbyMapInfoPanel game={props.game} playerId={props.playerId} layout={layout} />
      )}
      renderSettings={(close) => (
        <LobbySettingsOverlay game={props.game} playerId={props.playerId} isOpen onClose={close} />
      )}
      renderDemo={(close) => (
        <DemoSetupOverlay game={props.game} playerId={props.playerId} isOpen onClose={close} />
      )}
    />
  );
};

export function WaitingRoomOverlayView({
  game,
  playerId,
  visible,
  onExited,
  chat,
  onStart,
  onColor,
  onKick,
  onKickSpectator,
  onAddBot,
  onLeave,
  onCopyInvite,
  desktopMenu,
  compactMenu,
  mapPanel,
  renderSettings,
  renderDemo,
}: WaitingRoomOverlayProps & {
  onStart: () => void;
  onColor: (color: string, playerId?: string) => void;
  onKick: (id: string) => void;
  onKickSpectator: (id: string) => void;
  onAddBot: () => void;
  onLeave: () => void;
  onCopyInvite: (text: string) => Promise<void>;
  desktopMenu: React.ReactNode;
  compactMenu: React.ReactNode;
  mapPanel: (layout?: "stacked" | "split") => React.ReactNode;
  renderSettings: (close: () => void) => React.ReactNode;
  renderDemo: (close: () => void) => React.ReactNode;
}) {
  const isHost = game.hostPlayerId === playerId;
  const joinUrl = serverLink(`/game/${game.id}?type=join`);
  const [showLeaveConfirm, setShowLeaveConfirm] = useState(false);
  const [leaveConfirmVisible, setLeaveConfirmVisible] = useState(true);
  const [pendingLeave, setPendingLeave] = useState(false);
  const [colorPickerForPlayer, setColorPickerForPlayer] = useState<string | null>(null);
  const [colorPickerPlacement, setColorPickerPlacement] = useState<"above" | "below">("below");
  const colorPickerRef = useRef<HTMLDivElement>(null);
  const { isCompact } = useLayoutMode();
  const swatchSize = isCompact ? COMPACT_COLOR_SWATCH_SIZE : COLOR_SWATCH_SIZE;
  const colorPickerPopoverWidth = Math.max(
    COLOR_PICKER_MIN_WIDTH,
    colorPickerWidth(swatchSize) + COLOR_PICKER_PADDING + 8,
  );

  const isDemoGame = game.settings.demoGame;
  const [showDemoSetup, setShowDemoSetup] = useState(false);
  const [showLobbySettings, setShowLobbySettings] = useState(false);
  const [lobbyTab, setLobbyTab] = useState<LobbyTab>("players");

  const handleStartGame = () => {
    if (!isHost) return;
    onStart();
  };

  const hasCurrentPlayer = !!game.currentPlayer?.id;
  const playerCount = (hasCurrentPlayer ? 1 : 0) + (game.otherPlayers?.length || 0);

  const allBotsReady = React.useMemo(() => {
    const bots: { botStatus?: string }[] = [];
    if (hasCurrentPlayer && game.currentPlayer?.playerType === "bot") bots.push(game.currentPlayer);
    game.otherPlayers?.forEach((p) => {
      if (p.playerType === "bot") bots.push(p);
    });
    return bots.every((b) => b.botStatus === "ready");
  }, [hasCurrentPlayer, game.currentPlayer, game.otherPlayers]);

  const anyBotFailed = React.useMemo(
    () => (game.otherPlayers ?? []).some((p) => p.playerType === "bot" && p.botStatus === "failed"),
    [game.otherPlayers],
  );

  const allDemoPlayersReady = React.useMemo(() => {
    if (!isDemoGame) {
      return true;
    }
    const humans: { demoReady?: boolean }[] = [];
    if (hasCurrentPlayer && game.currentPlayer?.playerType !== "bot") {
      humans.push(game.currentPlayer!);
    }
    game.otherPlayers?.forEach((p) => {
      if (p.playerType !== "bot") {
        humans.push(p);
      }
    });
    return humans.every((h) => h.demoReady);
  }, [isDemoGame, hasCurrentPlayer, game.currentPlayer, game.otherPlayers]);

  const allPlayers = React.useMemo(() => {
    const players: { id: string; name: string; playerType: string }[] = [];
    if (hasCurrentPlayer && game.currentPlayer)
      players.push({
        id: game.currentPlayer.id,
        name: game.currentPlayer.name,
        playerType: game.currentPlayer.playerType,
      });
    game.otherPlayers?.forEach((p) =>
      players.push({ id: p.id, name: p.name, playerType: p.playerType }),
    );
    return players;
  }, [hasCurrentPlayer, game.currentPlayer, game.otherPlayers]);

  const takenColorsFor = React.useCallback(
    (targetId: string) => {
      const colors = new Set<string>();
      const addPlayerColor = (p: PlayerDto | OtherPlayerDto) => {
        if (p.color && p.id !== targetId) colors.add(p.color);
      };
      if (hasCurrentPlayer && game.currentPlayer) addPlayerColor(game.currentPlayer);
      game.otherPlayers?.forEach(addPlayerColor);
      return colors;
    },
    [hasCurrentPlayer, game.currentPlayer, game.otherPlayers],
  );

  const getPlayerColor = React.useCallback(
    (pid: string) => {
      if (hasCurrentPlayer && game.currentPlayer?.id === pid)
        return game.currentPlayer?.color || "";
      return game.otherPlayers?.find((p) => p.id === pid)?.color || "";
    },
    [hasCurrentPlayer, game.currentPlayer, game.otherPlayers],
  );

  const toggleColorPicker = (targetId: string, anchor: HTMLElement) => {
    if (colorPickerForPlayer === targetId) {
      setColorPickerForPlayer(null);
      return;
    }
    const colorCount = game.settings.availablePlayerColors?.length ?? 0;
    const rows = Math.max(1, Math.ceil(colorCount / COLOR_SWATCHES_PER_ROW));
    const pickerHeight =
      rows * swatchSize + (rows - 1) * COLOR_SWATCH_GAP + COLOR_PICKER_PADDING + COLOR_PICKER_GAP;
    const rect = anchor.getBoundingClientRect();
    const fitsBelow = rect.bottom + pickerHeight <= window.innerHeight;
    const fitsAbove = rect.top - pickerHeight >= 0;
    setColorPickerPlacement(!fitsBelow && fitsAbove ? "above" : "below");
    setColorPickerForPlayer(targetId);
  };

  const handleColorSelect = (color: string) => {
    if (colorPickerForPlayer) {
      const targetId = colorPickerForPlayer === playerId ? undefined : colorPickerForPlayer;
      onColor(color, targetId);
    }
    setColorPickerForPlayer(null);
  };

  const currentPlayerIds = React.useMemo(() => new Set(allPlayers.map((p) => p.id)), [allPlayers]);

  const leavingTimers = useRef<ReturnType<typeof setTimeout>[]>([]);
  useEffect(
    () => () => {
      leavingTimers.current.forEach(clearTimeout);
    },
    [],
  );
  const prevPlayerIdsRef = useRef<Set<string>>(new Set());
  const [newPlayerIds, setNewPlayerIds] = useState<Set<string>>(new Set());
  const [leavingPlayers, setLeavingPlayers] = useState<LeavingPlayer[]>([]);
  const prevPlayersRef = useRef<Map<string, string>>(new Map());

  const handleAnimationEnd = useCallback((id: string) => {
    setNewPlayerIds((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }, []);

  useEffect(() => {
    const prevIds = prevPlayerIdsRef.current;

    const joined = new Set<string>();
    for (const id of currentPlayerIds) {
      if (!prevIds.has(id)) joined.add(id);
    }

    const left: LeavingPlayer[] = [];
    for (const id of prevIds) {
      if (!currentPlayerIds.has(id)) {
        left.push({ id, name: prevPlayersRef.current.get(id) ?? "Player" });
      }
    }

    if (joined.size > 0) setNewPlayerIds(joined);

    if (left.length > 0) {
      setLeavingPlayers((prev) => [...prev, ...left]);
      leavingTimers.current.push(
        setTimeout(() => {
          setLeavingPlayers((prev) => prev.filter((p) => !left.some((l) => l.id === p.id)));
        }, 300),
      );
    }

    prevPlayerIdsRef.current = new Set(currentPlayerIds);
    const nameMap = new Map<string, string>();
    allPlayers.forEach((p) => nameMap.set(p.id, p.name));
    prevPlayersRef.current = nameMap;
  }, [currentPlayerIds, allPlayers]);

  const handleCancelLeave = () => {
    setLeaveConfirmVisible(false);
    setPendingLeave(false);
  };

  const handleConfirmLeave = () => {
    setLeaveConfirmVisible(false);
    setPendingLeave(true);
  };

  const handleLeaveConfirmExited = () => {
    setShowLeaveConfirm(false);
    setLeaveConfirmVisible(true);
    if (pendingLeave) {
      setPendingLeave(false);
      onLeave();
    }
  };

  const openLeaveConfirm = () => {
    setShowLeaveConfirm(true);
    setLeaveConfirmVisible(true);
  };

  let botWaitMessage = "Waiting for demo setup";
  if (anyBotFailed) {
    botWaitMessage = "A bot failed to start";
  } else if (!allBotsReady) {
    botWaitMessage = "Waiting for bots";
  }

  const playerSection = (
    <>
      <h3 className="text-white text-sm font-semibold mb-2 uppercase tracking-wide compact:hidden">
        Players
      </h3>
      <div
        aria-label="Players"
        className="lobby-player-list flex flex-col gap-2 max-h-[200px] overflow-y-auto overscroll-contain compact:max-h-none compact:overflow-visible"
      >
        {(() => {
          const playerMap = new Map();
          if (hasCurrentPlayer && game.currentPlayer) {
            playerMap.set(game.currentPlayer.id, game.currentPlayer);
          }
          game.otherPlayers?.forEach((otherPlayer) => {
            playerMap.set(otherPlayer.id, otherPlayer);
          });

          const orderedPlayers = getOrderedPlayers(playerMap, game);

          const playerItems = orderedPlayers.map((player) => ({
            id: player.id,
            name: player.name,
            color: player.color || "",
            playerType: player.playerType as string,
            botStatus: (player.botStatus as string) || undefined,
            botPersona: (player.botPersona as string) || undefined,
            demoReady: (player as PlayerDto | OtherPlayerDto).demoReady || false,
            isLeaving: false,
          }));

          leavingPlayers.forEach((lp) => {
            if (!playerMap.has(lp.id)) {
              playerItems.push({
                id: lp.id,
                name: lp.name,
                color: "",
                playerType: "human",
                botStatus: undefined,
                botPersona: undefined,
                demoReady: false,
                isLeaving: true,
              });
            }
          });

          return playerItems.map((player) => {
            let animClass = "";
            if (player.isLeaving) {
              animClass = "animate-[playerSlideOut_0.3s_ease-out_forwards] overflow-hidden";
            } else if (newPlayerIds.has(player.id)) {
              animClass = "animate-[playerSlideIn_0.3s_ease-out]";
            }

            const isCurrentPlayer = player.id === playerId;
            const canEditColor =
              (isCurrentPlayer || (isHost && player.playerType === "bot")) && !player.isLeaving;
            const showingPicker = colorPickerForPlayer === player.id;

            return (
              <div
                key={player.id}
                className={`relative flex shrink-0 justify-between items-center py-2 px-3 bg-black/40 rounded-lg border border-space-blue-600/50 ${animClass}`}
                onAnimationEnd={
                  !player.isLeaving && newPlayerIds.has(player.id)
                    ? () => handleAnimationEnd(player.id)
                    : undefined
                }
              >
                <div className="flex gap-2 items-center">
                  <div
                    className="relative flex items-center"
                    ref={showingPicker ? colorPickerRef : undefined}
                  >
                    <button
                      className={`relative w-4 h-4 rounded-full transition-all flex-shrink-0 p-0 leading-none before:absolute before:-inset-[14px] before:content-[''] ${
                        canEditColor ? "cursor-pointer hover:scale-125" : "cursor-default"
                      }`}
                      style={{ backgroundColor: player.color || "#555" }}
                      onClick={(e) => {
                        if (canEditColor) {
                          toggleColorPicker(player.id, e.currentTarget);
                        }
                      }}
                      aria-label={`Change ${player.name}'s color`}
                      disabled={!canEditColor}
                    />
                    {showingPicker && game.settings.availablePlayerColors && (
                      <GamePopover
                        isVisible={showingPicker}
                        onClose={() => setColorPickerForPlayer(null)}
                        position={{
                          type: "anchor",
                          anchorRef: colorPickerRef,
                          placement: colorPickerPlacement,
                        }}
                        theme="menu"
                        width={colorPickerPopoverWidth}
                        zIndex={Z_INDEX.POPOVER}
                      >
                        <div className="p-3">
                          <ColorPicker
                            colors={game.settings.availablePlayerColors}
                            takenColors={takenColorsFor(player.id)}
                            currentColor={getPlayerColor(player.id)}
                            swatchSize={swatchSize}
                            onSelect={handleColorSelect}
                          />
                        </div>
                      </GamePopover>
                    )}
                  </div>
                  <span className="text-white text-sm font-medium">{player.name}</span>
                  <InlineEmote playerId={player.id} />
                </div>
                <div className="player-chip-group">
                  {player.id === playerId && (
                    <PlayerChip className="bg-space-blue-800 text-white">You</PlayerChip>
                  )}
                  {game.hostPlayerId === player.id && (
                    <PlayerChip className="bg-gradient-to-br from-[#ffa500] to-[#ff8c00] text-white">
                      Host
                    </PlayerChip>
                  )}
                  {player.playerType === "bot" && (
                    <BotPersonaChip persona={player.botPersona} botStatus={player.botStatus} />
                  )}
                  {isDemoGame &&
                    player.playerType !== "bot" &&
                    !player.isLeaving &&
                    (player.demoReady ? (
                      <PlayerChip className="bg-green-700/60 text-green-300">Ready</PlayerChip>
                    ) : (
                      <PlayerChip className="bg-red-900/40 text-red-300/70">Not Ready</PlayerChip>
                    ))}
                  {isHost && player.id !== playerId && !player.isLeaving && (
                    <GameButton
                      emphasis="quiet"
                      onClick={() => onKick(player.id)}
                      className="ml-1 text-red-400 hover:text-red-300 transition-colors cursor-pointer"
                      aria-label={`Kick ${player.name}`}
                    >
                      <svg
                        width="14"
                        height="14"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      >
                        <line x1="18" y1="6" x2="6" y2="18" />
                        <line x1="6" y1="6" x2="18" y2="18" />
                      </svg>
                    </GameButton>
                  )}
                </div>
              </div>
            );
          });
        })()}
      </div>

      {/* Spectators Section */}
      {game.spectators && game.spectators.length > 0 && (
        <div className="mt-4">
          <h3 className="text-white/60 text-xs font-semibold mb-2 uppercase tracking-wide">
            Spectators
          </h3>
          <div className="flex flex-col gap-1.5 max-h-[160px] overflow-y-auto overscroll-contain compact:max-h-none compact:overflow-visible">
            {game.spectators.map((spectator) => (
              <div
                key={spectator.id}
                className="flex shrink-0 justify-between items-center py-1.5 px-3 bg-black/25 rounded-lg border border-white/10"
              >
                <span className="text-white/70 text-sm">{spectator.name}</span>
                <div className="flex gap-1.5 items-center">
                  {isHost && (
                    <GameButton
                      emphasis="quiet"
                      onClick={() => onKickSpectator(spectator.id)}
                      className="ml-1 text-red-400/60 hover:text-red-300 transition-colors cursor-pointer"
                    >
                      <svg
                        width="12"
                        height="12"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      >
                        <line x1="18" y1="6" x2="6" y2="18" />
                        <line x1="6" y1="6" x2="18" y2="18" />
                      </svg>
                    </GameButton>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </>
  );

  const lobbyActions = (
    <>
      <CopyLinkButtonView
        onCopy={onCopyInvite}
        textToCopy={joinUrl}
        defaultText="Invite friends"
        copiedText="Link copied"
        icon={
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
          </svg>
        }
      />
      {isHost && game.settings.hasClaudeOAuthToken && (
        <GameButton emphasis="secondary" size="md" onClick={() => onAddBot()}>
          <span className="inline-flex items-center gap-2">
            Bot
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <line x1="12" y1="5" x2="12" y2="19" />
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          </span>
        </GameButton>
      )}
    </>
  );

  const leaveButton = (
    <GameButton
      emphasis="quiet"
      size="sm"
      onClick={openLeaveConfirm}
      aria-label="Leave game"
      className="!p-2.5 hover:text-red-400"
    >
      <svg
        width="22"
        height="22"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="-scale-x-100"
      >
        <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
        <polyline points="16 17 21 12 16 7" />
        <line x1="21" y1="12" x2="9" y2="12" />
      </svg>
    </GameButton>
  );

  const settingsButton = isHost ? (
    <GameButton
      emphasis="quiet"
      size="sm"
      onClick={() => setShowLobbySettings(true)}
      className="!p-2.5 hover:text-space-blue-300"
      aria-label="Lobby settings"
    >
      <svg
        width="22"
        height="22"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
    </GameButton>
  ) : null;

  const configureDemoButton = isDemoGame ? (
    <GameButton
      emphasis="secondary"
      size="md"
      onClick={() => setShowDemoSetup(true)}
      className="min-w-[180px] compact:min-w-0"
    >
      Configure demo
    </GameButton>
  ) : null;

  const startDisabled = playerCount < 1 || !allBotsReady || !allDemoPlayersReady;
  const showWaitMessage = isHost && (!allBotsReady || !allDemoPlayersReady);

  const desktopLobby = (
    <GameMenuModal
      layout="lobby"
      title="Game lobby"
      visible={visible}
      onExited={onExited}
      headerStart={leaveButton}
      headerEnd={settingsButton}
      footer={
        <div className="flex flex-wrap items-center justify-end gap-3 border-t border-white/10 pt-5 mt-6">
          {isDemoGame && <div className="text-center">{configureDemoButton}</div>}

          {isDemoGame && <div className="h-2" />}

          {showWaitMessage && <p className="text-sm text-white/60">{botWaitMessage}</p>}
          {isHost && (
            <div className="text-center">
              <GameButton
                size="lg"
                onClick={handleStartGame}
                disabled={startDisabled}
                className="min-w-[180px]"
              >
                Start game
              </GameButton>
            </div>
          )}

          {!isHost && (
            <p className="text-white/50 text-sm text-center">
              Waiting for host to start the game...
            </p>
          )}
        </div>
      }
    >
      <div className="menu-grid">
        <div className="min-w-0 flex flex-col justify-between">
          <div className="mb-6 min-w-0">
            {playerSection}
            <div className="mt-4 flex justify-center gap-2">{lobbyActions}</div>
          </div>

          {chat}
        </div>
        {mapPanel()}
      </div>
    </GameMenuModal>
  );

  let compactPane: React.ReactNode = (
    <div className="h-full min-h-0 flex flex-col gap-3">
      <div className="flex-1 min-h-0 overflow-y-auto overscroll-contain">{playerSection}</div>
      <div className="shrink-0 flex flex-wrap items-center justify-center gap-2">
        {lobbyActions}
        {configureDemoButton}
      </div>
      {showWaitMessage && (
        <p className="shrink-0 m-0 text-center text-xs text-white/60">{botWaitMessage}</p>
      )}
    </div>
  );
  if (lobbyTab === "chat") {
    compactPane = <div className="h-full min-h-0">{chat}</div>;
  } else if (lobbyTab === "map") {
    compactPane = mapPanel("split");
  }

  const compactLobby = (
    <div
      data-testid="mobile-lobby"
      className={`fixed inset-0 flex flex-col text-white bg-[rgba(3,3,4,0.98)] ${
        visible === false
          ? "animate-[compactLobbyExit_0.2s_ease-out_forwards]"
          : "animate-[modalFadeIn_0.3s_ease-out]"
      }`}
      style={{
        zIndex: Z_INDEX.MENU_DROPDOWN,
        paddingTop: "var(--safe-top)",
        paddingRight: "var(--safe-right)",
        paddingBottom: "var(--safe-bottom)",
        paddingLeft: "var(--safe-left)",
      }}
      onAnimationEnd={(e) => {
        if (e.target === e.currentTarget && visible === false) {
          onExited?.();
        }
      }}
    >
      <header
        data-testid="mobile-lobby-top-bar"
        className="shrink-0 h-11 flex items-center gap-1 pr-2 border-b border-white/15"
      >
        {compactMenu}
        <div
          role="tablist"
          aria-label="Lobby"
          className="flex-1 min-w-0 h-full flex items-center justify-center"
        >
          {LOBBY_TABS.map((tab) => {
            const selected = tab.id === lobbyTab;
            return (
              <GameButton
                key={tab.id}
                role="tab"
                aria-selected={selected}
                emphasis="quiet"
                size="xs"
                className={`!min-h-11 h-11 !px-3 portrait:!px-1.5 whitespace-nowrap font-orbitron !text-[11px] tracking-wider portrait:tracking-normal uppercase border-y-2 border-t-transparent ${selected ? "border-b-[#8fb8ca] !text-white" : "border-b-transparent"}`}
                onClick={() => setLobbyTab(tab.id)}
              >
                {tab.label}
              </GameButton>
            );
          })}
        </div>
        <div className="shrink-0 flex items-center">
          {settingsButton}
          {leaveButton}
          {isHost ? (
            <GameButton
              size="sm"
              onClick={handleStartGame}
              disabled={startDisabled}
              aria-label="Start game"
              className="ml-1 !min-h-9 whitespace-nowrap"
            >
              <span className="portrait:hidden">Start game</span>
              <span className="hidden portrait:inline">Start</span>
            </GameButton>
          ) : (
            <p className="m-0 ml-1 max-w-24 landscape:max-w-none text-right text-[11px] leading-tight text-white/50">
              Waiting for host…
            </p>
          )}
        </div>
      </header>
      <div className="flex-1 min-h-0 p-3">{compactPane}</div>
    </div>
  );

  return (
    <>
      <style>{`
        @keyframes playerSlideIn {
          from { opacity: 0; transform: translateY(-8px) scale(0.95); max-height: 0; }
          to { opacity: 1; transform: translateY(0) scale(1); max-height: 60px; }
        }
        @keyframes playerSlideOut {
          from { opacity: 1; transform: translateY(0) scale(1); max-height: 60px; }
          to { opacity: 0; transform: translateY(-8px) scale(0.95); max-height: 0; padding: 0; margin: 0; }
        }
        @keyframes compactLobbyExit {
          from { opacity: 1; }
          to { opacity: 0; }
        }
      `}</style>

      {!isCompact && desktopMenu}

      {showLeaveConfirm && (
        <GameMenuModal
          title="Leave game?"
          subtitle={isHost && playerCount > 1 ? "Another player will become the host" : undefined}
          visible={leaveConfirmVisible}
          onExited={handleLeaveConfirmExited}
          showBackdrop={true}
          zIndex={Z_INDEX.CONFIRMATION_MODAL}
          onClose={handleCancelLeave}
        >
          <div className="flex gap-3 justify-center">
            <GameButton emphasis="secondary" size="sm" onClick={handleCancelLeave}>
              Cancel
            </GameButton>
            <GameButton tone="error" size="sm" onClick={handleConfirmLeave}>
              Leave
            </GameButton>
          </div>
        </GameMenuModal>
      )}

      {isCompact ? compactLobby : desktopLobby}

      {isHost && showLobbySettings && renderSettings(() => setShowLobbySettings(false))}
      {isDemoGame && showDemoSetup && renderDemo(() => setShowDemoSetup(false))}
    </>
  );
}

export default WaitingRoomOverlay;
