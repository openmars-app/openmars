import { useCallback, useRef, useState } from "react";
import type { GameDto, OtherPlayerDto, PlayerDto } from "@/types/generated/api-types.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { getPhaseDisplayName } from "@/constants/gameConstants.ts";
import { isPlayerActionPhase } from "@/utils/actionUtils.ts";
import { usePassAction } from "@/hooks/usePassAction.ts";
import { GameMenuItems } from "../ui/GameHamburgerMenu.tsx";
import TravelPopover from "../ui/popover/TravelPopover.tsx";
import GameButton from "../ui/buttons/GameButton.tsx";
import PassConfirmDialog from "../ui/modals/PassConfirmDialog.tsx";
import { useMobileGame } from "./MobileGameContext.tsx";
import MobileMenuDrawer, { MobileMenuButton } from "./MobileMenuDrawer.tsx";
import EarthSupportProduction, {
  earthSupportLabel,
} from "../ui/display/EarthSupportProduction.tsx";
import GameModeRules from "../ui/lobby/GameModeRules.tsx";

const BUTTON_HEIGHT = 36;
const HIT_AREA_CLASS =
  "!min-h-0 after:absolute after:inset-x-0 after:-inset-y-1 after:content-['']";

function findPlayer(
  gameState: GameDto,
  playerId: string | undefined,
): PlayerDto | OtherPlayerDto | undefined {
  if (!playerId) {
    return undefined;
  }
  if (gameState.currentPlayer?.id === playerId) {
    return gameState.currentPlayer;
  }
  return gameState.otherPlayers?.find((player) => player.id === playerId);
}

function describeTurn(gameState: GameDto, viewer: PlayerDto | null): string {
  const phaseName = getPhaseDisplayName(gameState.currentPhase);
  if (!isPlayerActionPhase(gameState.currentPhase)) {
    return phaseName;
  }
  const turnPlayer = findPlayer(gameState, gameState.currentTurn);
  if (!turnPlayer) {
    return phaseName;
  }
  if (viewer && turnPlayer.id === viewer.id) {
    const actions = viewer.availableActions;
    if (actions === -1) {
      return "Your turn";
    }
    const noun = actions === 1 ? "action" : "actions";
    return `Your turn · ${actions} ${noun} left`;
  }
  if (viewer) {
    return `Waiting for ${turnPlayer.name}`;
  }
  return `${turnPlayer.name}'s turn`;
}

interface MobileTopBarProps {
  showStatus: boolean;
  showActions: boolean;
  spectatingPlayer?: PlayerDto | OtherPlayerDto | null;
  onStopSpectating?: () => void;
  className?: string;
}

export default function MobileTopBar({
  showStatus,
  showActions,
  spectatingPlayer,
  onStopSpectating,
  className = "",
}: MobileTopBarProps) {
  const { gameState, currentPlayer, isSpectator, isReplay, onLeaveGame, onEndGame } =
    useMobileGame();
  const [menuOpen, setMenuOpen] = useState(false);
  const [travelOpen, setTravelOpen] = useState(false);
  const [modeRulesOpen, setModeRulesOpen] = useState(false);
  const gameMode = gameState.settings.availableGameModes.find(
    (mode) => mode.id === gameState.settings.gameModeId,
  );
  const supportLabel = earthSupportLabel(gameState);
  const closeMenu = useCallback(() => setMenuOpen(false), []);
  const travelButtonRef = useRef<HTMLButtonElement>(null);

  const pass = usePassAction({
    player: currentPlayer,
    turnPlayerId: gameState.currentTurn,
    currentPhase: gameState.currentPhase,
  });
  const isHost = !!currentPlayer && currentPlayer.id === gameState.hostPlayerId;
  const showPass = showActions && !isSpectator && !isReplay && pass.canPass;
  const showSpectating = showActions && !!spectatingPlayer && !!onStopSpectating;
  const bandClass = showStatus
    ? "bg-[rgba(3,3,4,0.88)] border-b border-white/10 pointer-events-auto"
    : "pointer-events-none";

  return (
    <div
      data-testid="mobile-top-bar"
      className={`fixed top-0 inset-x-0 flex items-center gap-2 ${bandClass} ${className}`}
      style={{
        zIndex: Z_INDEX.MOBILE_HUD,
        height: "calc(var(--hud-top-h) + var(--safe-top))",
        paddingTop: "var(--safe-top)",
        paddingLeft: "calc(var(--safe-left) + 6px)",
        paddingRight: "calc(var(--safe-right) + 6px)",
      }}
    >
      <MobileMenuButton
        isOpen={menuOpen}
        onClick={() => setMenuOpen((open) => !open)}
        className={`!h-10 ${HIT_AREA_CLASS}`}
      />

      {showStatus && (
        <div className="flex-1 min-w-0 flex items-center gap-2 font-orbitron">
          <span className="shrink-0 text-[13px] font-bold tracking-wider text-white">
            GEN {gameState.generation}
          </span>
          <span className="text-white/30" aria-hidden="true">
            ·
          </span>
          <span className="truncate text-[12px] text-white/80">
            {describeTurn(gameState, currentPlayer)}
          </span>
        </div>
      )}

      {showSpectating && (
        <GameButton
          emphasis="secondary"
          height={BUTTON_HEIGHT}
          accent={spectatingPlayer.color || "#6496ff"}
          aria-label={`Stop viewing ${spectatingPlayer.name}`}
          className={`shrink min-w-0 max-w-[45%] !px-3 !text-[11px] ${HIT_AREA_CLASS}`}
          onClick={onStopSpectating}
        >
          <span className="flex items-center gap-1.5 min-w-0">
            <span
              className="shrink-0 w-2 h-2 rounded-full"
              style={{ backgroundColor: spectatingPlayer.color || "#6496ff" }}
              aria-hidden="true"
            />
            <span className="truncate font-normal normal-case">
              Viewing {spectatingPlayer.name}
            </span>
            <span className="shrink-0 text-white/40" aria-hidden="true">
              ·
            </span>
            <span className="shrink-0">STOP</span>
          </span>
        </GameButton>
      )}

      {showActions && gameMode && (
        <GameButton
          shape="toolbar"
          surface="console"
          emphasis="secondary"
          height={BUTTON_HEIGHT}
          className={`shrink-0 !px-3 !text-[11px] ${HIT_AREA_CLASS}`}
          selected={modeRulesOpen}
          aria-expanded={modeRulesOpen}
          aria-label="Gamemode"
          onClick={() => setModeRulesOpen(true)}
        >
          GAMEMODE
        </GameButton>
      )}
      {showActions && (
        <GameButton
          ref={travelButtonRef}
          shape="toolbar"
          surface="console"
          emphasis="secondary"
          height={BUTTON_HEIGHT}
          className={`shrink-0 !px-3 !text-[11px] ${HIT_AREA_CLASS}`}
          selected={travelOpen}
          aria-expanded={travelOpen}
          onClick={() => setTravelOpen((open) => !open)}
        >
          TRAVEL
        </GameButton>
      )}

      {showPass && (
        <GameButton
          tone="info"
          height={BUTTON_HEIGHT}
          className={`shrink-0 !px-4 !text-[12px] !font-bold tracking-wider ${HIT_AREA_CLASS}`}
          onClick={pass.requestPass}
        >
          {pass.label}
        </GameButton>
      )}

      {showActions && (
        <TravelPopover
          isVisible={travelOpen}
          onClose={() => setTravelOpen(false)}
          anchorRef={travelButtonRef}
        />
      )}
      {showActions && gameMode && (
        <GameModeRules
          mode={gameMode}
          parameters={gameState.settings.gameModeParameters}
          open={modeRulesOpen}
          onClose={() => setModeRulesOpen(false)}
        >
          {supportLabel && (
            <section
              aria-label="Earth support"
              className="mx-auto mb-6 max-w-sm border border-white/15 bg-white/[0.03] p-3"
            >
              <h3 className="font-orbitron text-sm text-center mb-3">{supportLabel}</h3>
              <p className="text-sm text-white/70 text-center mb-1">Your production</p>
              <EarthSupportProduction production={gameState.currentPlayer?.modeProduction} />
            </section>
          )}
        </GameModeRules>
      )}
      <MobileMenuDrawer isOpen={menuOpen} onClose={closeMenu}>
        <GameMenuItems
          onClose={closeMenu}
          gameId={gameState.id}
          isHost={isHost}
          developmentMode={gameState.settings.developmentMode}
          onLeaveGame={onLeaveGame}
          onEndGame={isHost ? onEndGame : undefined}
        />
      </MobileMenuDrawer>
      <PassConfirmDialog confirmation={pass.confirmation} />
    </div>
  );
}
