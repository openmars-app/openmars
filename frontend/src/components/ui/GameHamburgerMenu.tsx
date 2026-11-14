import { SaveFeedback } from "./SaveGameLayout.tsx";
import { apiService } from "@/services/apiService.ts";
import { useGameStore } from "@/stores/gameStore.ts";
import React, { useCallback, useEffect, useState } from "react";
import { GamePopover } from "./GamePopover";
import { MenuPopoverItem, MenuPopoverDivider, MenuPopoverVersion } from "./MenuPopoverItem.tsx";
import {
  CopyIcon,
  DownloadIcon,
  FullscreenIcon,
  ExitFullscreenIcon,
  PerformanceIcon,
  FeedbackIcon,
  LeaveIcon,
  EndGameIcon,
  BugIcon,
  CardsIcon,
  InstallIcon,
  SettingsIcon,
  ChangelogIcon,
} from "./menuIcons.tsx";
import SoundToggleButton from "./buttons/SoundToggleButton.tsx";
import { useHoverSound } from "@/hooks/useHoverSound.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import { useVersionStore } from "@/stores/versionStore.ts";
import {
  isFullscreen as readIsFullscreen,
  isFullscreenSupported,
  toggleFullscreen,
} from "@/utils/fullscreen.ts";
import { useInstallOffer } from "@/utils/installApp.ts";
import { serverLink } from "@/utils/gateway.ts";

interface GameMenuItemsProps {
  onClose: () => void;
  gameId?: string;
  isHost: boolean;
  developmentMode?: boolean;
  onLeaveGame?: () => void;
  onEndGame?: () => void;
}

interface GameHamburgerMenuProps extends GameMenuItemsProps {
  isOpen: boolean;
  anchorRef: React.RefObject<HTMLButtonElement | null>;
}

// Shared by the gameplay menu and the setup/showcase menu.
export function SaveGameMenuItem({ gameId }: { gameId?: string }) {
  const [saveStatus, setSaveStatus] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveFailed, setSaveFailed] = useState(false);
  const playerId = useGameStore((s) => s.playerId);
  const gameStatus = useGameStore((s) => s.game?.status);
  const saveGame = async () => {
    if (!gameId || !playerId || saving) {
      return;
    }
    setSaving(true);
    setSaveStatus("");
    setSaveFailed(false);
    try {
      await apiService.downloadGameSave(gameId, playerId);
      setSaveStatus("Game save downloaded");
    } catch (err) {
      setSaveFailed(true);
      setSaveStatus(err instanceof Error ? err.message : "Could not save game");
    } finally {
      setSaving(false);
    }
  };
  const isHost = useGameStore((s) => s.game?.id === gameId && s.game?.hostPlayerId === s.playerId);
  if (!isHost || gameStatus !== "active") {
    return null;
  }
  return (
    <>
      <MenuPopoverItem
        icon={<DownloadIcon />}
        label={saving ? "Saving game…" : "Save game"}
        onClick={() => void saveGame()}
      />
      <div className="px-4 max-w-72">
        <SaveFeedback message={saveStatus} error={saveFailed} />
      </div>
    </>
  );
}

export const GameMenuItems: React.FC<GameMenuItemsProps> = ({
  onClose,
  gameId,
  isHost,
  developmentMode,
  onLeaveGame,
  onEndGame,
}) => {
  const menuItemHover = useHoverSound();
  const [isFullscreen, setIsFullscreen] = useState(readIsFullscreen);
  const fullscreenSupported = isFullscreenSupported();
  const { platform: installPlatform, install } = useInstallOffer();

  useEffect(() => {
    const handler = () => setIsFullscreen(readIsFullscreen());
    document.addEventListener("fullscreenchange", handler);
    return () => document.removeEventListener("fullscreenchange", handler);
  }, []);

  const handleToggleFullscreen = useCallback(() => {
    void toggleFullscreen();
    onClose();
  }, [onClose]);

  const handleCopyGameLink = useCallback(async () => {
    if (!gameId) {
      return;
    }
    const url = serverLink(`/game/${gameId}`);
    await navigator.clipboard.writeText(url);
    onClose();
  }, [gameId, onClose]);

  const handleLeaveGame = useCallback(() => {
    menuItemHover.onClick?.();
    onClose();
    onLeaveGame?.();
  }, [onLeaveGame, onClose, menuItemHover]);

  const handleEndGame = useCallback(() => {
    menuItemHover.onClick?.();
    onClose();
    onEndGame?.();
  }, [onEndGame, onClose, menuItemHover]);

  return (
    <>
      <MenuPopoverItem
        icon={<CopyIcon />}
        label="Copy game link"
        onClick={() => {
          menuItemHover.onClick?.();
          void handleCopyGameLink();
        }}
        onMouseEnter={menuItemHover.onMouseEnter}
      />
      <SaveGameMenuItem gameId={gameId} />
      <MenuPopoverDivider />
      <SoundToggleButton />
      <MenuPopoverItem
        icon={<SettingsIcon />}
        label="Settings"
        onClick={() => {
          onClose();
          useUIOverlayStore.getState().setShowSettings(true);
        }}
      />
      <MenuPopoverDivider />
      <MenuPopoverItem
        icon={<CardsIcon />}
        label="Cards"
        onClick={() => {
          menuItemHover.onClick?.();
          onClose();
          useUIOverlayStore.getState().setShowCardBrowser(true);
        }}
        onMouseEnter={menuItemHover.onMouseEnter}
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
              menuItemHover.onClick?.();
              onClose();
              void install();
            }}
            onMouseEnter={menuItemHover.onMouseEnter}
          />
        </>
      )}
      <MenuPopoverDivider />
      <MenuPopoverItem
        icon={<PerformanceIcon />}
        label="Performance"
        onClick={() => {
          menuItemHover.onClick?.();
          onClose();
          window.dispatchEvent(new CustomEvent("toggle-performance-window"));
        }}
        onMouseEnter={menuItemHover.onMouseEnter}
      />
      <MenuPopoverDivider />
      <MenuPopoverItem
        icon={<ChangelogIcon />}
        label="Changelog"
        onClick={() => {
          menuItemHover.onClick?.();
          onClose();
          useVersionStore.getState().openChangelog();
        }}
        onMouseEnter={menuItemHover.onMouseEnter}
      />
      <MenuPopoverDivider />
      <MenuPopoverItem
        icon={<FeedbackIcon />}
        label="Feedback"
        onClick={() => {
          menuItemHover.onClick?.();
          onClose();
          window.dispatchEvent(new CustomEvent("toggle-feedback-window"));
        }}
        onMouseEnter={menuItemHover.onMouseEnter}
      />
      {developmentMode && (
        <>
          <MenuPopoverDivider />
          <MenuPopoverItem
            icon={<BugIcon />}
            label="Admin Tools"
            onClick={() => {
              menuItemHover.onClick?.();
              onClose();
              window.dispatchEvent(new CustomEvent("toggle-debug-dropdown"));
            }}
            onMouseEnter={menuItemHover.onMouseEnter}
          />
        </>
      )}
      <MenuPopoverDivider />
      <MenuPopoverItem
        icon={<LeaveIcon />}
        label="Leave game"
        variant="danger"
        onClick={handleLeaveGame}
        onMouseEnter={menuItemHover.onMouseEnter}
      />
      {isHost && onEndGame && (
        <>
          <MenuPopoverDivider />
          <MenuPopoverItem
            icon={<EndGameIcon />}
            label="End game"
            variant="danger"
            onClick={handleEndGame}
            onMouseEnter={menuItemHover.onMouseEnter}
          />
        </>
      )}
      <MenuPopoverDivider />
      <MenuPopoverVersion />
    </>
  );
};

const GameHamburgerMenu: React.FC<GameHamburgerMenuProps> = ({
  isOpen,
  onClose,
  anchorRef,
  ...items
}) => (
  <GamePopover
    className="game-popover-list"
    isVisible={isOpen}
    onClose={onClose}
    position={{
      type: "anchor",
      anchorRef: anchorRef,
      placement: "below",
    }}
    theme="menu"
    width={230}
    maxHeight="auto"
    animation="slideDown"
    excludeRef={anchorRef}
    zIndex={Z_INDEX.TOP_MENU_ALWAYS_ON_TOP + 1}
    overlayLayer
  >
    <div className="py-1">
      <GameMenuItems onClose={onClose} {...items} />
    </div>
  </GamePopover>
);

export default GameHamburgerMenu;
