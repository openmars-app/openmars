import { SaveGameMenuItem } from "../GameHamburgerMenu.tsx";
import React, { useState, useRef, useEffect, useCallback } from "react";
import SoundToggleButton from "./SoundToggleButton.tsx";
import GameButton from "./GameButton.tsx";
import { GamePopover } from "../GamePopover";
import { MenuPopoverItem, MenuPopoverDivider, MenuPopoverVersion } from "../MenuPopoverItem.tsx";
import {
  CopyIcon,
  FullscreenIcon,
  ExitFullscreenIcon,
  FeedbackIcon,
  LeaveIcon,
  EndGameIcon,
  HamburgerIcon,
  InstallIcon,
  HomeIcon,
  ServerIcon,
  CardsIcon,
  ChangelogIcon,
  SettingsIcon,
} from "../menuIcons.tsx";
import MobileMenuDrawer, { MobileMenuButton } from "../../mobile/MobileMenuDrawer.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { useGameStore } from "@/stores/gameStore.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import { useVersionStore } from "@/stores/versionStore.ts";
import {
  isFullscreen as readIsFullscreen,
  isFullscreenSupported,
  toggleFullscreen,
} from "@/utils/fullscreen.ts";
import { useInstallOffer } from "@/utils/installApp.ts";
import { useLocation, useNavigate } from "react-router-dom";
import { changeServer, gatewayServer, serverLink } from "@/utils/gateway.ts";

interface MainMenuProps {
  gameId?: string;
  onLeaveGame?: () => void;
  onEndGame?: () => void;
}

export const MainMenuItems: React.FC<MainMenuProps & { onClose: () => void }> = ({
  gameId,
  onLeaveGame,
  onEndGame,
  onClose,
}) => {
  const [isFullscreen, setIsFullscreen] = useState(readIsFullscreen);
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const showMainMenuLink = !gameId && pathname !== "/";
  const fullscreenSupported = isFullscreenSupported();
  const { platform: installPlatform, install } = useInstallOffer();
  const { isCompact } = useLayoutMode();
  // Desktop has these in the menu footer; phones have no footer
  const showChangeServer = isCompact && !gameId && !!gatewayServer;
  const showViewCards = isCompact && !gameId && pathname !== "/cards";

  useEffect(() => {
    const handleFullscreenChange = () => {
      setIsFullscreen(readIsFullscreen());
    };
    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", handleFullscreenChange);
  }, []);

  const handleToggleFullscreen = useCallback(() => {
    void toggleFullscreen();
    onClose();
  }, [onClose]);

  const handleChangeServer = useCallback(() => {
    onClose();
    if (gatewayServer?.servers) {
      useUIOverlayStore.getState().setShowServerSelector(true);
    } else {
      changeServer();
    }
  }, [onClose]);

  const handleChangelog = useCallback(() => {
    onClose();
    useVersionStore.getState().openChangelog();
  }, [onClose]);

  const handleFeedback = useCallback(() => {
    onClose();
    window.dispatchEvent(new CustomEvent("toggle-feedback-window"));
  }, [onClose]);

  const handleCopyGameLink = useCallback(async () => {
    if (gameId) {
      const url = serverLink(`/game/${gameId}`);
      await navigator.clipboard.writeText(url);
      onClose();
    }
  }, [gameId, onClose]);

  return (
    <>
      {gameId && (
        <>
          <MenuPopoverItem
            icon={<CopyIcon />}
            label="Copy game link"
            onClick={() => void handleCopyGameLink()}
          />
          <SaveGameMenuItem gameId={gameId} />
          <MenuPopoverDivider />
        </>
      )}
      {showMainMenuLink && (
        <>
          <MenuPopoverItem
            icon={<HomeIcon />}
            label="Main menu"
            onClick={() => {
              onClose();
              navigate("/");
            }}
          />
          <MenuPopoverDivider />
        </>
      )}
      {showViewCards && (
        <>
          <MenuPopoverItem
            icon={<CardsIcon />}
            label="View cards"
            onClick={() => {
              onClose();
              navigate("/cards");
            }}
          />
          <MenuPopoverDivider />
        </>
      )}
      <SoundToggleButton />
      <MenuPopoverItem
        icon={<SettingsIcon />}
        label="Settings"
        onClick={() => {
          onClose();
          useUIOverlayStore.getState().setShowSettings(true);
        }}
      />
      {fullscreenSupported && (
        <>
          <MenuPopoverDivider />
          <MenuPopoverItem
            icon={isFullscreen ? <ExitFullscreenIcon /> : <FullscreenIcon />}
            label={isFullscreen ? "Exit Fullscreen" : "Fullscreen"}
            onClick={handleToggleFullscreen}
          />
        </>
      )}
      {installPlatform && (
        <>
          <MenuPopoverDivider />
          <MenuPopoverItem
            icon={<InstallIcon />}
            label="Install app"
            onClick={() => {
              onClose();
              void install();
            }}
          />
        </>
      )}
      {showChangeServer && (
        <>
          <MenuPopoverDivider />
          <MenuPopoverItem
            icon={<ServerIcon />}
            label="Change server"
            onClick={handleChangeServer}
          />
        </>
      )}
      <MenuPopoverDivider />
      <MenuPopoverItem icon={<ChangelogIcon />} label="Changelog" onClick={handleChangelog} />
      <MenuPopoverDivider />
      <MenuPopoverItem icon={<FeedbackIcon />} label="Feedback" onClick={handleFeedback} />
      {onLeaveGame && (
        <>
          <MenuPopoverDivider />
          <MenuPopoverItem
            icon={<LeaveIcon />}
            label="Leave game"
            variant="danger"
            onClick={() => {
              onClose();
              onLeaveGame();
            }}
          />
        </>
      )}
      {onEndGame && (
        <>
          <MenuPopoverDivider />
          <MenuPopoverItem
            icon={<EndGameIcon />}
            label="End game"
            variant="danger"
            onClick={() => {
              onClose();
              onEndGame();
            }}
          />
        </>
      )}
      <MenuPopoverDivider />
      <MenuPopoverVersion />
    </>
  );
};

export const MainMenuDrawerButton: React.FC<MainMenuProps> = (props) => {
  const [menuOpen, setMenuOpen] = useState(false);
  const close = useCallback(() => setMenuOpen(false), []);
  return (
    <>
      <MobileMenuButton isOpen={menuOpen} onClick={() => setMenuOpen((open) => !open)} />
      <MobileMenuDrawer isOpen={menuOpen} onClose={close}>
        <MainMenuItems {...props} onClose={close} />
      </MobileMenuDrawer>
    </>
  );
};

export const CurrentGameMenuButton: React.FC = () => {
  const gameId = useGameStore((s) => s.game?.id);
  const isHost = useGameStore((s) => !!s.game && s.playerId === s.game.hostPlayerId);
  const requestLeaveGame = useUIOverlayStore((s) => s.requestLeaveGame);
  const requestEndGame = useUIOverlayStore((s) => s.requestEndGame);
  return (
    <MainMenuDrawerButton
      gameId={gameId}
      onLeaveGame={requestLeaveGame}
      onEndGame={isHost ? requestEndGame : undefined}
    />
  );
};

const MainMenuPopover: React.FC<MainMenuProps> = (props) => {
  const [menuOpen, setMenuOpen] = useState(false);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const close = useCallback(() => setMenuOpen(false), []);

  return (
    <div className="menu-chrome-top-right" data-overlay-layer style={{ zIndex: Z_INDEX.POPOVER }}>
      <GameButton
        ref={buttonRef}
        aria-label="Menu"
        emphasis="secondary"
        size="sm"
        onClick={() => setMenuOpen(!menuOpen)}
        className="p-2.5"
      >
        <HamburgerIcon />
      </GameButton>

      <GamePopover
        className="game-popover-list"
        isVisible={menuOpen}
        onClose={close}
        position={{ type: "anchor", anchorRef: buttonRef, placement: "below" }}
        theme="menu"
        width={200}
        maxHeight="auto"
        animation="slideDown"
        excludeRef={buttonRef}
      >
        <MainMenuItems {...props} onClose={close} />
      </GamePopover>
    </div>
  );
};

const MainMenuHamburger: React.FC<MainMenuProps> = (props) => {
  const { isCompact } = useLayoutMode();
  if (isCompact) {
    return (
      <div className="menu-chrome-top-left" style={{ zIndex: Z_INDEX.POPOVER }}>
        <MainMenuDrawerButton {...props} />
      </div>
    );
  }
  return <MainMenuPopover {...props} />;
};

export default MainMenuHamburger;
