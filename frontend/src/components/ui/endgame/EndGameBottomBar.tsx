import { useOverlayEventTarget } from "@/contexts/OverlayEnvironment";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import { FC, useState, useMemo, useEffect, useRef } from "react";
import type { GameDto, GameHistoryEntryDto } from "../../../types/generated/api-types";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { getPhaseDisplayName } from "../../../constants/gameConstants";
import { useVPCounting } from "../../../contexts/VPCountingContext";
import GameGraphs from "./GameGraphs.tsx";
import ReplayControls from "./ReplayControls.tsx";
import VPPhaseTabsOverlay from "./VPPhaseTabsOverlay.tsx";

const ANGLE_INDENT = 14;
const BUTTON_HEIGHT = 32;
const BUTTON_SPACING = 4;
const COMPACT_BUTTON_HEIGHT = 36;
const COMPACT_HIT_AREA_CLASS = "after:absolute after:inset-x-0 after:-inset-y-1 after:content-['']";
const COMPACT_TOP_OFFSET_CLASS = "top-[calc(var(--hud-top-h)+var(--safe-top))]";
const DESKTOP_TOP_OFFSET_CLASS = "top-[60px] max-lg:top-[50px]";

type EndgamePanel = "score" | "graphs" | "replay";

const COMPACT_PANEL_BUTTONS: { id: EndgamePanel; label: string; width: number }[] = [
  { id: "score", label: "SCORE", width: 84 },
  { id: "graphs", label: "GRAPHS", width: 92 },
  { id: "replay", label: "REPLAY", width: 92 },
];

interface ParaButtonConfig {
  id: string;
  label: string;
  accentColor: string;
  width: number;
  height?: number;
  className?: string;
  isActive?: boolean;
  visible?: boolean;
  onClick: () => void;
}

function GameButtonGroup({
  buttons,
  direction = "right",
}: {
  buttons: ParaButtonConfig[];
  direction?: "right" | "left" | "open";
}) {
  const visibleButtons = buttons.filter((b) => b.visible !== false);

  return (
    <div className="flex items-center">
      {visibleButtons.map((btn, idx) => (
        <ParaButton
          key={btn.id}
          config={btn}
          index={idx}
          total={visibleButtons.length}
          direction={direction}
        />
      ))}
    </div>
  );
}

function ParaButton({
  config,
  index,
  total,
  direction,
}: {
  config: ParaButtonConfig;
  index: number;
  total: number;
  direction: "right" | "left" | "open";
}) {
  const isFlat =
    direction !== "open" && (direction === "right" ? index === 0 : index === total - 1);
  const defaultLeftEdge = isFlat ? "flat" : "slope-right";
  const leftEdge = direction === "left" ? "slope-left" : defaultLeftEdge;
  const defaultRightEdge = direction === "left" ? "slope-right" : "slope-left";
  const rightEdge = direction === "left" && isFlat ? "flat" : defaultRightEdge;
  return (
    <GameButton
      shape="toolbar"
      emphasis="secondary"
      size="xs"
      width={config.width}
      height={config.height ?? BUTTON_HEIGHT}
      className={config.className}
      accent={config.accentColor}
      selected={config.isActive}
      onClick={config.onClick}
      leftEdge={leftEdge}
      rightEdge={rightEdge}
      style={
        {
          "--button-angle": `${ANGLE_INDENT}px`,
          marginLeft: index === 0 ? 0 : -ANGLE_INDENT + BUTTON_SPACING,
          zIndex: Z_INDEX.UI_BASE - index,
        } as React.CSSProperties
      }
    >
      {config.label}
    </GameButton>
  );
}

interface EndGameBottomBarProps {
  game: GameDto;
  playerId: string;
  historyEntries?: GameHistoryEntryDto[];
  activePanel: EndgamePanel;
  onPanelChange?: (panel: EndgamePanel) => void;
  isReplayActive?: boolean;
  replayIndex?: number;
  replayTotal?: number;
  replayPlaying?: boolean;
  replaySpeed?: number;
  onReplayPlay?: () => void;
  onReplayPause?: () => void;
  onReplaySeek?: (index: number) => void;
  onReplayStepForward?: () => void;
  onReplayStepBackward?: () => void;
  onReplaySpeedChange?: (speed: number) => void;
  replaySpectatePlayerId?: string | null;
  onReplaySpectatePlayerChange?: (playerId: string | null) => void;
}

const EndGameBottomBar: FC<EndGameBottomBarProps> = ({
  game,
  playerId,
  historyEntries,
  activePanel,
  onPanelChange,
  isReplayActive,
  replayIndex,
  replayTotal,
  replayPlaying,
  replaySpeed,
  onReplayPlay,
  onReplayPause,
  onReplaySeek,
  onReplayStepForward,
  onReplayStepBackward,
  onReplaySpeedChange,
  replaySpectatePlayerId,
  onReplaySpectatePlayerChange,
}) => {
  const eventTarget = useOverlayEventTarget();
  const { state: vpState, controls: vpControls } = useVPCounting();
  const { isCompact } = useLayoutMode();
  const topOffsetClass = isCompact ? COMPACT_TOP_OFFSET_CLASS : DESKTOP_TOP_OFFSET_CLASS;

  useEffect(() => {
    if (activePanel !== "graphs" || !onPanelChange) {
      return;
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onPanelChange("score");
      }
    };
    eventTarget.addEventListener("keydown", handleKeyDown);
    return () => eventTarget.removeEventListener("keydown", handleKeyDown);
  }, [activePanel, onPanelChange, eventTarget]);

  const hasHistory = (historyEntries?.length ?? 0) > 0;
  const allScores = game.finalScores ?? [];
  const sortedScores = [...allScores].sort((a, b) => b.vpBreakdown.totalVP - a.vpBreakdown.totalVP);

  const maxVP = Math.max(...sortedScores.map((s) => s.vpBreakdown.totalVP), 1);
  const isCounting = vpState.isActive && !vpState.isComplete;

  const playerColors = useMemo(() => {
    const colors: Record<string, string> = {};
    if (game.currentPlayer) {
      colors[game.currentPlayer.id] = game.currentPlayer.color || "#ffffff";
    }
    for (const p of game.otherPlayers ?? []) {
      colors[p.id] = p.color || "#ffffff";
    }
    return colors;
  }, [game]);

  const playerNames = useMemo(() => {
    const names: Record<string, string> = {};
    if (game.currentPlayer) {
      names[game.currentPlayer.id] = game.currentPlayer.name;
    }
    for (const p of game.otherPlayers ?? []) {
      names[p.id] = p.name;
    }
    return names;
  }, [game]);

  const allPlayers = useMemo(() => {
    const players: { id: string; name: string }[] = [];
    if (game.currentPlayer) {
      players.push({ id: game.currentPlayer.id, name: game.currentPlayer.name });
    }
    for (const p of game.otherPlayers ?? []) {
      players.push({ id: p.id, name: p.name });
    }
    return players;
  }, [game]);

  const replayLabel = useMemo(() => {
    if (!isReplayActive || !historyEntries?.length) {
      return "";
    }
    const entry = historyEntries[replayIndex ?? 0];
    if (!entry) {
      return "";
    }
    return `Generation ${entry.generation} - ${getPhaseDisplayName(entry.phase)}`;
  }, [isReplayActive, historyEntries, replayIndex]);

  const generationMarkers = useMemo(() => {
    if (!historyEntries?.length) return [];
    const markers: { index: number; generation: number }[] = [];
    let lastGen = -1;
    for (let i = 0; i < historyEntries.length; i++) {
      if (historyEntries[i].generation !== lastGen) {
        lastGen = historyEntries[i].generation;
        markers.push({ index: i, generation: lastGen });
      }
    }
    return markers;
  }, [historyEntries]);

  const actionButtonHeight = isCompact ? 44 : undefined;
  const actionButtons: ParaButtonConfig[] = [
    {
      id: "replay-vp",
      label: "\u27F3",
      accentColor: "#ffffff",
      width: 50,
      height: actionButtonHeight,
      visible: activePanel === "score" && vpState.isComplete && !isCounting,
      onClick: vpControls.start,
    },
    {
      id: "skip",
      label: "Skip",
      accentColor: "#ffffff",
      width: 80,
      height: actionButtonHeight,
      visible: isCounting,
      onClick: vpControls.skip,
    },
  ];

  if (allScores.length === 0) {
    return (
      <div
        className="fixed bottom-0 left-0 right-0 p-4 flex items-center justify-center"
        style={{ zIndex: Z_INDEX.STANDARD_MODAL }}
      >
        <div className="text-center">
          <p className="text-red-400 font-orbitron mb-3">No scores available</p>
        </div>
      </div>
    );
  }

  return (
    <>
      {/* Graphs overlay — sits below the top menu bar so SCORE/GRAPHS/REPLAY stay visible */}
      <div
        className={`fixed ${topOffsetClass} left-0 right-0 bottom-0 bg-black/70 backdrop-blur-lg flex items-center justify-center transition-opacity duration-500 ${
          activePanel === "graphs" ? "opacity-100" : "opacity-0 pointer-events-none"
        }`}
        style={{ zIndex: Z_INDEX.MENU_DROPDOWN }}
      >
        {historyEntries && (
          <div
            className={isCompact ? "w-full h-full pt-1" : "w-[90%] h-full py-4"}
            style={
              isCompact
                ? {
                    paddingLeft: "calc(var(--safe-left) + 8px)",
                    paddingRight: "calc(var(--safe-right) + 8px)",
                    paddingBottom: "calc(var(--safe-bottom) + 6px)",
                  }
                : undefined
            }
          >
            <GameGraphs
              entries={historyEntries}
              playerColors={playerColors}
              playerNames={playerNames}
            />
          </div>
        )}
      </div>

      {/* Replay controls overlay — blurred strip below the top menu bar */}
      <div
        className={`fixed ${topOffsetClass} left-0 right-0 bg-black/50 flex justify-center transition-opacity duration-500 ${
          activePanel === "replay" ? "opacity-100" : "opacity-0 pointer-events-none"
        }`}
        style={{
          paddingTop: 10,
          paddingBottom: 10,
          paddingLeft: isCompact ? "calc(var(--safe-left) + 12px)" : undefined,
          paddingRight: isCompact ? "calc(var(--safe-right) + 12px)" : undefined,
          zIndex: Z_INDEX.MENU_DROPDOWN,
        }}
      >
        <div className={isCompact ? "w-full" : "w-[60%]"}>
          <ReplayControls
            currentIndex={replayIndex ?? 0}
            totalStates={replayTotal ?? 0}
            isPlaying={replayPlaying ?? false}
            playbackSpeed={replaySpeed ?? 1}
            currentLabel={replayLabel}
            onPlay={onReplayPlay ?? (() => {})}
            onPause={onReplayPause ?? (() => {})}
            onStepForward={onReplayStepForward ?? (() => {})}
            onStepBackward={onReplayStepBackward ?? (() => {})}
            onSeek={onReplaySeek ?? (() => {})}
            onSpeedChange={onReplaySpeedChange ?? (() => {})}
            generationMarkers={generationMarkers}
            rightSlot={
              <div className={`flex items-center ${isCompact ? "gap-0" : "gap-2"}`}>
                <span
                  className={`text-white/50 font-orbitron ${isCompact ? "text-[11px] mr-1" : "text-xs"}`}
                >
                  VIEW AS
                </span>
                {allPlayers.map((p) => {
                  const isViewed = replaySpectatePlayerId === p.id;
                  const color = playerColors[p.id] || "#ffffff";
                  const dotStyle: React.CSSProperties = {
                    width: isViewed ? 18 : 14,
                    height: isViewed ? 18 : 14,
                    borderRadius: "50%",
                    backgroundColor: color,
                    opacity: isViewed ? 1 : 0.5,
                    border: "none",
                    boxShadow: isViewed ? `0 0 8px ${color}` : "none",
                  };
                  const toggle = () => onReplaySpectatePlayerChange?.(isViewed ? null : p.id);
                  if (isCompact) {
                    return (
                      <button
                        key={p.id}
                        onClick={toggle}
                        className="flex h-11 w-11 cursor-pointer items-center justify-center"
                        aria-label={p.name}
                      >
                        <span className="transition-all duration-200" style={dotStyle} />
                      </button>
                    );
                  }
                  return (
                    <button
                      key={p.id}
                      onClick={toggle}
                      className="cursor-pointer transition-all duration-200"
                      style={dotStyle}
                      aria-label={p.name}
                    />
                  );
                })}
              </div>
            }
          />
        </div>
      </div>

      {/* Score bottom bar with faded background */}
      <div
        className={`fixed bottom-0 left-0 right-0 transition-opacity duration-500 ${
          activePanel === "score" ? "opacity-100" : "opacity-0 pointer-events-none"
        }`}
        style={{ zIndex: Z_INDEX.STANDARD_MODAL }}
      >
        <div
          className="bg-black/50 backdrop-blur-sm"
          style={{
            paddingTop: isCompact ? 8 : 16,
            paddingBottom: isCompact ? "calc(var(--safe-bottom) + 8px)" : 12,
          }}
        >
          {/* Phase tabs */}
          <VPPhaseTabsOverlay
            phases={vpControls.phases}
            currentPhaseIndex={vpState.currentPhaseIndex}
            isActive={vpState.isActive}
          />

          {/* Score bars */}
          <div
            className="space-y-1.5"
            style={
              isCompact
                ? {
                    marginLeft: "calc(var(--safe-left) + 12px)",
                    marginRight: "calc(var(--safe-right) + 12px)",
                  }
                : { marginLeft: "20%", marginRight: "20%" }
            }
          >
            {sortedScores.map((score) => (
              <PlayerBar
                key={score.playerId}
                playerId={score.playerId}
                playerName={score.playerName}
                totalVP={score.vpBreakdown.totalVP}
                displayVP={
                  vpState.isComplete
                    ? score.vpBreakdown.totalVP
                    : (vpState.playerAccumulatedVP[score.playerId] ?? 0)
                }
                maxVP={maxVP}
                playerColor={playerColors[score.playerId] || "#ffffff"}
                isYou={score.playerId === playerId}
                isActivePlayer={isCounting && vpState.activePlayerId === score.playerId}
                isDimmed={isCounting && vpState.activePlayerId !== score.playerId}
                isCompact={isCompact}
              />
            ))}
          </div>

          {/* Controls row — only VP replay/skip buttons */}
          {actionButtons.some((b) => b.visible !== false) && (
            <div
              className="relative flex items-center px-4 pb-1 pt-1"
              style={isCompact ? { paddingRight: "calc(var(--safe-right) + 12px)" } : undefined}
            >
              <div className="ml-auto">
                <GameButtonGroup buttons={actionButtons} direction="left" />
              </div>
            </div>
          )}
        </div>
      </div>

      {isCompact && onPanelChange && (
        <div
          className="fixed flex items-center"
          style={{
            top: "var(--safe-top)",
            right: "calc(var(--safe-right) + 6px)",
            height: "var(--hud-top-h)",
            zIndex: Z_INDEX.MENU_DROPDOWN,
          }}
        >
          <GameButtonGroup
            direction="left"
            buttons={COMPACT_PANEL_BUTTONS.map((btn) => ({
              id: btn.id,
              label: btn.label,
              accentColor: "#ffffff",
              width: btn.width,
              height: COMPACT_BUTTON_HEIGHT,
              className: COMPACT_HIT_AREA_CLASS,
              isActive: activePanel === btn.id,
              visible: btn.id === "score" || hasHistory,
              onClick: () => onPanelChange(btn.id),
            }))}
          />
        </div>
      )}
    </>
  );
};

const BAR_BASE_MS = 800;
const BAR_MS_PER_PERCENT = 30;

function PlayerBar({
  playerName,
  displayVP,
  maxVP,
  playerColor,
  isYou,
  isActivePlayer,
  isDimmed,
  isCompact,
}: {
  playerId: string;
  playerName: string;
  totalVP: number;
  displayVP: number;
  maxVP: number;
  playerColor: string;
  isYou: boolean;
  isActivePlayer: boolean;
  isDimmed: boolean;
  isCompact: boolean;
}) {
  const barWidth = maxVP > 0 ? (displayVP / maxVP) * 100 : 0;
  const prevWidthRef = useRef(barWidth);

  const widthDelta = Math.abs(barWidth - prevWidthRef.current);
  const isResetting = barWidth < prevWidthRef.current;
  const transitionMs = isResetting ? 400 : BAR_BASE_MS + widthDelta * BAR_MS_PER_PERCENT;

  useEffect(() => {
    prevWidthRef.current = barWidth;
  }, [barWidth]);

  const barRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [visualVP, setVisualVP] = useState(displayVP);
  const [visualLeft, setVisualLeft] = useState(0);
  const rafRef = useRef(0);

  useEffect(() => {
    const tick = () => {
      if (barRef.current && containerRef.current) {
        const barPx = barRef.current.getBoundingClientRect().width;
        const containerPx = containerRef.current.getBoundingClientRect().width;
        const visualPct = containerPx > 0 ? barPx / containerPx : 0;
        setVisualVP(Math.round(visualPct * maxVP));
        setVisualLeft(barPx);
      }
      rafRef.current = requestAnimationFrame(tick);
    };
    rafRef.current = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(rafRef.current);
  }, [maxVP]);

  return (
    <div className={`flex items-center ${isCompact ? "gap-2" : "gap-3"}`}>
      <div
        className={`font-orbitron truncate shrink-0 transition-opacity duration-300 ${isCompact ? "text-[13px]" : "text-base"}`}
        style={{ width: isCompact ? 80 : 120, color: "#ffffff", opacity: isDimmed ? 0.4 : 1 }}
      >
        {playerName}
        {isYou && <span className="text-white/30 text-xs ml-1">(you)</span>}
      </div>
      <div className={`flex-1 h-2 relative ${isCompact ? "mr-14" : ""}`} ref={containerRef}>
        <div
          ref={barRef}
          className="h-full ease-out"
          style={{
            width: `${Math.min(barWidth, 100)}%`,
            backgroundColor: playerColor,
            transform: "skewX(-10deg)",
            opacity: isDimmed ? 0.3 : 0.85,
            boxShadow: isActivePlayer ? `0 0 10px 2px ${playerColor}` : "none",
            transition: `width ${transitionMs}ms ease-out, opacity 300ms ease-out, box-shadow 300ms ease-out`,
          }}
        />
        <span
          className="absolute top-1/2 -translate-y-1/2 font-orbitron text-xs tabular-nums text-white/80 whitespace-nowrap transition-opacity duration-300"
          style={{
            left: visualLeft,
            paddingLeft: "4px",
            opacity: isDimmed ? 0.4 : 1,
          }}
        >
          {visualVP} VP
        </span>
      </div>
    </div>
  );
}

export default EndGameBottomBar;
