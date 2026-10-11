import React, { useState, useRef, useEffect, useMemo } from "react";
import { usePlanetFocus } from "../../../contexts/PlanetFocusContext.tsx";
import {
  GameDto,
  GamePhaseAction,
  GamePhaseInitApplyCorp,
  GamePhaseInitApplyPrelude,
  GameStatusActive,
  PlayerDto,
  SpectatorDto,
} from "@/types/generated/api-types.ts";
import { StandardProject } from "@/types/cards.tsx";
import { Z_INDEX } from "@/constants/zIndex";
import { canPerformActions } from "@/utils/actionUtils.ts";
import GameHamburgerMenu from "../../ui/GameHamburgerMenu.tsx";
import TravelPopover from "../../ui/popover/TravelPopover.tsx";
import StandardProjectPopover from "../../ui/popover/StandardProjectPopover.tsx";
import MilestonePopover from "../../ui/popover/MilestonePopover.tsx";
import AwardPopover from "../../ui/popover/AwardPopover.tsx";
import ColonyPopover from "../../ui/popover/ColonyPopover.tsx";
import ProjectFundingPopover from "../../ui/popover/ProjectFundingPopover.tsx";
import { GamePopover } from "../../ui/GamePopover";
import { HamburgerIcon, EyeIcon } from "../../ui/menuIcons.tsx";
import GameButton, { ANGLE_INDENT, BUTTON_SPACING } from "../../ui/buttons/GameButton.tsx";
import MilestoneAwardStatusStrip from "./MilestoneAwardStatusStrip.tsx";
import GameModeRules from "../../ui/lobby/GameModeRules.tsx";

const HAMBURGER_WIDTH = 65;
const TRAVEL_WIDTH = 140;

const MilestoneAlertIndicator: React.FC<{ visible: boolean; top: number }> = ({ visible, top }) => (
  <div
    className={`absolute left-1/2 -translate-x-1/2 pointer-events-none transition-opacity duration-500 ${
      visible ? "opacity-100" : "opacity-0"
    }`}
    style={{ top }}
  >
    <span
      className="font-orbitron font-bold text-[11px] leading-none animate-[milestoneIndicatorPulse_2s_ease-in-out_infinite] flex items-center justify-center w-5 h-5 rounded-full border border-yellow-400/70"
      style={{ color: "#facc15", paddingTop: 1 }}
    >
      !
    </span>
  </div>
);

function EndgameTabButton({
  label,
  width,
  height,
  isActive,
  onClick,
}: {
  label: string;
  width: number;
  height: number;
  isActive: boolean;
  onClick: () => void;
}) {
  return (
    <GameButton
      shape="toolbar"
      surface="console"
      emphasis="secondary"
      selected={isActive}
      aria-pressed={isActive}
      width={width}
      height={height}
      leftEdge="slope-left"
      rightEdge="slope-right"
      onClick={onClick}
      style={{ marginRight: -ANGLE_INDENT + BUTTON_SPACING }}
    >
      {label}
    </GameButton>
  );
}

interface EndgamePanelButton {
  id: "score" | "graphs" | "replay";
  label: string;
  width: number;
}

const ENDGAME_BUTTONS: EndgamePanelButton[] = [
  { id: "score", label: "SCORE", width: 120 },
  { id: "graphs", label: "GRAPHS", width: 130 },
  { id: "replay", label: "REPLAY", width: 130 },
];

interface TopMenuBarProps {
  gameState: GameDto;
  currentPlayer?: PlayerDto | null;
  onStandardProjectSelect?: (project: StandardProject) => void;
  onLeaveGame?: () => void;
  onEndGame?: () => void;
  gameId?: string;
  isEndgame?: boolean;
  activeEndgamePanel?: "score" | "graphs" | "replay";
  onEndgamePanelChange?: (panel: "score" | "graphs" | "replay") => void;
  hasHistory?: boolean;
  /** Render only the hamburger menu (used during the pre-game corporation showcase). */
  minimal?: boolean;
  contentClassName?: string;
}

const TopMenuBar: React.FC<TopMenuBarProps> = ({
  gameState,
  currentPlayer,
  onStandardProjectSelect,
  onLeaveGame,
  onEndGame,
  gameId,
  isEndgame = false,
  activeEndgamePanel,
  onEndgamePanelChange,
  hasHistory = false,
  minimal = false,
  contentClassName = "",
}) => {
  const { activePlanet } = usePlanetFocus();
  const [menuOpen, setMenuOpen] = useState(false);
  const [showTravelPopover, setShowTravelPopover] = useState(false);
  const [showModeRules, setShowModeRules] = useState(false);
  const gameMode = gameState.settings.availableGameModes.find(
    (mode) => mode.id === gameState.settings.gameModeId,
  );
  const travelButtonRef = useRef<HTMLButtonElement>(null);

  const [spectatorsOpen, setSpectatorsOpen] = useState(false);
  const hamburgerButtonRef = useRef<HTMLButtonElement>(null);
  const eyeButtonRef = useRef<HTMLButtonElement>(null);

  const [showStandardProjectsPopover, setShowStandardProjectsPopover] = useState(false);
  const [showMilestonePopover, setShowMilestonePopover] = useState(false);
  const [showAwardPopover, setShowAwardPopover] = useState(false);
  const [showColonyPopover, setShowColonyPopover] = useState(false);
  const [showProjectFundingPopover, setShowProjectFundingPopover] = useState(false);
  const standardProjectsButtonRef = useRef<HTMLButtonElement>(null);
  const milestonesButtonRef = useRef<HTMLButtonElement>(null);
  const awardsButtonRef = useRef<HTMLButtonElement>(null);
  const coloniesButtonRef = useRef<HTMLButtonElement>(null);
  const projectFundingButtonRef = useRef<HTMLButtonElement>(null);

  const handleStandardProjectSelect = (project: StandardProject) => {
    setShowStandardProjectsPopover(false);
    onStandardProjectSelect?.(project);
  };

  const isInitPhase =
    gameState?.currentPhase === GamePhaseInitApplyCorp ||
    gameState?.currentPhase === GamePhaseInitApplyPrelude;

  const hasColonies = (gameState?.colonies?.length ?? 0) > 0;
  const hasProjectFunding = (gameState?.projectFunding?.length ?? 0) > 0;

  const hasEligibleMilestones = useMemo(() => {
    const isGameActive = gameState?.status === GameStatusActive;
    const isActionPhase = gameState?.currentPhase === GamePhaseAction;
    const isCurrentPlayerTurn = gameState?.currentTurn === gameState?.viewingPlayerId;
    if (!isGameActive || !isActionPhase || !isCurrentPlayerTurn || !canPerformActions(gameState)) {
      return false;
    }
    const milestones = gameState?.currentPlayer?.milestones ?? [];
    return milestones.some(
      (m) => !m.isClaimed && !(m.errors ?? []).some((e) => e.category === "requirement"),
    );
  }, [gameState]);

  const menuItems: { id: string; label: string; isOpen: boolean }[] = [
    { id: "projects", label: "STANDARD PROJECTS", isOpen: showStandardProjectsPopover },
    { id: "milestones", label: "MILESTONES", isOpen: showMilestonePopover },
    { id: "awards", label: "AWARDS", isOpen: showAwardPopover },
    ...(hasColonies ? [{ id: "colonies", label: "COLONIES", isOpen: showColonyPopover }] : []),
    ...(hasProjectFunding
      ? [{ id: "funding", label: "FUNDING", isOpen: showProjectFundingPopover }]
      : []),
  ];

  const handleTabClick = (tabId: string) => {
    if (currentPlayer?.pendingTileSelection) return;

    if (tabId === "projects") {
      setShowStandardProjectsPopover((prev) => !prev);
    } else if (tabId === "milestones") {
      setShowMilestonePopover((prev) => !prev);
    } else if (tabId === "awards") {
      setShowAwardPopover((prev) => !prev);
    } else if (tabId === "colonies") {
      setShowColonyPopover((prev) => !prev);
    } else if (tabId === "funding") {
      setShowProjectFundingPopover((prev) => !prev);
    }
  };

  const getButtonRef = (itemId: string) => {
    if (itemId === "projects") return standardProjectsButtonRef;
    if (itemId === "milestones") return milestonesButtonRef;
    if (itemId === "awards") return awardsButtonRef;
    if (itemId === "colonies") return coloniesButtonRef;
    if (itemId === "funding") return projectFundingButtonRef;
    return null;
  };

  const calcScale = () => Math.min(1, Math.max(0.75, window.innerWidth / 2200));
  const [topBarScale, setTopBarScale] = useState(calcScale);

  useEffect(() => {
    const handleResize = () => setTopBarScale(calcScale());
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  const spectators: SpectatorDto[] = gameState?.spectators || [];
  const baseWidths = [250, 190, 160];
  let buttonWidths = [...baseWidths];
  if (hasColonies) {
    buttonWidths.push(170);
  }
  if (hasProjectFunding) {
    buttonWidths.push(160);
  }
  const buttonHeight = 40;
  const EYE_WIDTH = 68;

  return (
    <>
      {!minimal && (
        <div
          className={`bg-transparent relative pointer-events-none ${contentClassName}`}
          style={{ zIndex: Z_INDEX.TOP_MENU_BAR }}
        >
          <div className="flex justify-between items-center h-[60px] max-lg:h-[50px] max-md:flex-wrap">
            <div
              className={`flex max-md:order-2 max-md:flex-[0_0_100%] max-md:mt-2.5 origin-top-left transition-opacity duration-500 ease-in-out ${activePlanet === "solar-system" ? "opacity-0 pointer-events-none" : "opacity-100 pointer-events-auto"}`}
              inert={activePlanet === "solar-system"}
              style={{ transform: `scale(${topBarScale})` }}
            >
              {!isInitPhase &&
                !isEndgame &&
                menuItems.map((item, index) => (
                  <div key={item.id} className="relative">
                    <GameButton
                      shape="toolbar"
                      surface="console"
                      emphasis="secondary"
                      width={buttonWidths[index]}
                      height={buttonHeight}
                      onClick={() => handleTabClick(item.id)}
                      ref={getButtonRef(item.id) as React.RefObject<HTMLButtonElement | null>}
                      leftEdge={index === 0 ? "flat" : "slope-right"}
                      rightEdge="slope-left"
                      style={{
                        marginLeft: index === 0 ? 0 : -ANGLE_INDENT + BUTTON_SPACING,
                        zIndex: Z_INDEX.UI_BASE - index,
                      }}
                      selected={item.isOpen}
                      aria-expanded={item.isOpen}
                    >
                      {item.label}
                    </GameButton>
                    {item.id === "milestones" && (
                      <MilestoneAlertIndicator
                        visible={hasEligibleMilestones && !showMilestonePopover}
                        top={buttonHeight + 6}
                      />
                    )}
                  </div>
                ))}
            </div>

            <div style={{ marginRight: HAMBURGER_WIDTH * topBarScale }}>
              <div
                className="origin-top-right flex items-center pointer-events-auto"
                style={{ transform: `scale(${topBarScale})` }}
              >
                {isEndgame && onEndgamePanelChange && (
                  <div className="flex items-center pointer-events-auto">
                    {ENDGAME_BUTTONS.filter(
                      (btn) => (btn.id !== "graphs" && btn.id !== "replay") || hasHistory,
                    ).map((btn) => {
                      const isActive = activeEndgamePanel === btn.id;
                      return (
                        <EndgameTabButton
                          key={btn.id}
                          label={btn.label}
                          width={btn.width}
                          height={buttonHeight}
                          isActive={isActive}
                          onClick={() => onEndgamePanelChange(btn.id)}
                        />
                      );
                    })}
                  </div>
                )}
                {spectators.length > 0 && (
                  <GameButton
                    shape="toolbar"
                    surface="console"
                    emphasis="secondary"
                    width={EYE_WIDTH}
                    height={buttonHeight}
                    onClick={() => setSpectatorsOpen(!spectatorsOpen)}
                    ref={eyeButtonRef}
                    selected={spectatorsOpen}
                    aria-expanded={spectatorsOpen}
                    aria-label="Spectators"
                    leftEdge="slope-left"
                    rightEdge="slope-right"
                    style={{ marginRight: -ANGLE_INDENT + BUTTON_SPACING }}
                  >
                    <EyeIcon />
                  </GameButton>
                )}
                {gameMode && (
                  <GameButton
                    shape="toolbar"
                    surface="console"
                    emphasis="secondary"
                    width={165}
                    height={buttonHeight}
                    onClick={() => setShowModeRules(true)}
                    selected={showModeRules}
                    aria-expanded={showModeRules}
                    aria-label="Gamemode"
                    leftEdge="slope-left"
                    rightEdge="slope-right"
                    style={{ marginRight: -ANGLE_INDENT + BUTTON_SPACING }}
                  >
                    GAMEMODE
                  </GameButton>
                )}
                <GameButton
                  shape="toolbar"
                  surface="console"
                  emphasis="secondary"
                  width={TRAVEL_WIDTH}
                  height={buttonHeight}
                  onClick={() => setShowTravelPopover((prev) => !prev)}
                  ref={travelButtonRef}
                  selected={showTravelPopover}
                  aria-expanded={showTravelPopover}
                  leftEdge="slope-left"
                  rightEdge="slope-right"
                  style={{ marginRight: -ANGLE_INDENT + BUTTON_SPACING }}
                >
                  TRAVEL
                </GameButton>
              </div>
            </div>
          </div>

          {!isInitPhase && !isEndgame && (
            <div
              className={`absolute left-1/2 top-0 h-[60px] max-lg:h-[50px] flex items-center origin-top transition-opacity duration-500 ease-in-out ${
                activePlanet === "solar-system"
                  ? "opacity-0 pointer-events-none"
                  : "opacity-100 pointer-events-auto"
              }`}
              inert={activePlanet === "solar-system"}
              style={{ transform: `translateX(-50%) scale(${topBarScale})` }}
            >
              <MilestoneAwardStatusStrip />
            </div>
          )}

          <TravelPopover
            isVisible={showTravelPopover}
            onClose={() => setShowTravelPopover(false)}
            anchorRef={travelButtonRef}
          />
          {gameMode && (
            <GameModeRules
              mode={gameMode}
              parameters={gameState.settings.gameModeParameters}
              open={showModeRules}
              onClose={() => setShowModeRules(false)}
            />
          )}

          <StandardProjectPopover
            isVisible={showStandardProjectsPopover}
            onClose={() => setShowStandardProjectsPopover(false)}
            onProjectSelect={handleStandardProjectSelect}
            gameState={gameState}
            anchorRef={standardProjectsButtonRef}
          />

          <MilestonePopover
            isVisible={showMilestonePopover}
            onClose={() => setShowMilestonePopover(false)}
            gameState={gameState}
            anchorRef={milestonesButtonRef}
          />

          <AwardPopover
            isVisible={showAwardPopover}
            onClose={() => setShowAwardPopover(false)}
            gameState={gameState}
            anchorRef={awardsButtonRef}
          />

          {hasColonies && (
            <ColonyPopover
              isVisible={showColonyPopover}
              onClose={() => setShowColonyPopover(false)}
              gameState={gameState}
              anchorRef={coloniesButtonRef}
            />
          )}

          {hasProjectFunding && (
            <ProjectFundingPopover
              isVisible={showProjectFundingPopover}
              onClose={() => setShowProjectFundingPopover(false)}
              gameState={gameState}
              anchorRef={projectFundingButtonRef}
            />
          )}

          {spectators.length > 0 && (
            <GamePopover
              isVisible={spectatorsOpen}
              onClose={() => setSpectatorsOpen(false)}
              position={{
                type: "anchor",
                anchorRef: eyeButtonRef,
                placement: "below",
              }}
              theme="menu"
              width={180}
              maxHeight="auto"
              animation="slideDown"
              excludeRef={eyeButtonRef}
              zIndex={Z_INDEX.TOP_MENU_ALWAYS_ON_TOP + 1}
            >
              <div className="py-2 px-3">
                <div className="text-white/40 text-[10px] font-orbitron font-bold uppercase tracking-wider mb-2">
                  Spectators ({spectators.length})
                </div>
                <div className="flex flex-col gap-1.5">
                  {spectators.map((s) => (
                    <div key={s.id} className="flex items-center gap-2">
                      <span
                        className="w-2 h-2 rounded-full shrink-0"
                        style={{ backgroundColor: s.color }}
                      />
                      <span className="text-white/80 text-sm truncate">{s.name}</span>
                    </div>
                  ))}
                </div>
              </div>
            </GamePopover>
          )}
        </div>
      )}
      <div
        className="fixed right-0 h-[60px] max-lg:h-[50px] flex items-center pointer-events-none origin-top-right"
        style={{
          zIndex: Z_INDEX.TOP_MENU_ALWAYS_ON_TOP,
          transform: `scale(${topBarScale})`,
          top: topBarScale < 1 ? 1 : 0,
        }}
      >
        <GameButton
          shape="toolbar"
          surface="console"
          emphasis="secondary"
          className="pointer-events-auto"
          aria-label="Menu"
          width={HAMBURGER_WIDTH}
          height={buttonHeight}
          onClick={() => setMenuOpen(!menuOpen)}
          ref={hamburgerButtonRef}
          selected={menuOpen}
          aria-expanded={menuOpen}
          leftEdge="slope-left"
          rightEdge="flat"
        >
          <HamburgerIcon />
        </GameButton>
      </div>
      <GameHamburgerMenu
        isOpen={menuOpen}
        onClose={() => setMenuOpen(false)}
        anchorRef={hamburgerButtonRef}
        gameId={gameId}
        isHost={currentPlayer?.id === gameState.hostPlayerId}
        developmentMode={gameState.settings.developmentMode}
        onLeaveGame={onLeaveGame}
        onEndGame={currentPlayer?.id === gameState.hostPlayerId ? onEndGame : undefined}
      />
    </>
  );
};

export default TopMenuBar;
