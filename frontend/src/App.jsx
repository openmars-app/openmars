import LoadGamePage from "./components/pages/LoadGamePage.tsx";
import ResumeGamePage from "./components/pages/ResumeGamePage.tsx";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import { BrowserRouter as Router, Navigate, Routes, Route, useLocation } from "react-router-dom";
import { useCallback, useEffect, useRef, useState } from "react";
import GameInterface from "./components/layout/main/GameInterface.tsx";
import CreateGamePage from "./components/pages/CreateGamePage.tsx";
import JoinGamePage from "./components/pages/JoinGamePage.tsx";
import CardsPage from "./components/pages/CardsPage.tsx";
import GameLandingPage from "./components/pages/GameLandingPage.tsx";
import ReconnectingPage from "./components/pages/ReconnectingPage.tsx";
import { globalWebSocketManager } from "./services/globalWebSocketManager.ts";
import { SpaceBackgroundProvider, useSpaceBackground } from "./contexts/SpaceBackgroundContext.tsx";
import { SoundProvider } from "./contexts/SoundContext.tsx";
import { NotificationProvider } from "./contexts/NotificationContext.tsx";
import { World3DSettingsProvider } from "./contexts/World3DSettingsContext.tsx";
import NotificationContainer from "./components/ui/notifications/NotificationContainer.tsx";
import { audioService } from "./services/audioService.ts";
import MainMenuHamburger from "./components/ui/buttons/MainMenuHamburger.tsx";
import { useLayoutMode } from "./hooks/useLayoutMode.ts";
import SpaceBackground from "./components/3d/SpaceBackground.tsx";
import LoadingOverlay from "./components/game/view/LoadingOverlay.tsx";
import { useAppPhaseStore, showsSpaceBackground, showsMenuChrome } from "./stores/appPhaseStore.ts";
import FeedbackWindow from "./components/ui/debug/FeedbackWindow.tsx";
import { WindowManagerProvider } from "./components/ui/debug/WindowManager.tsx";
import { useUIOverlayStore } from "./stores/uiOverlayStore.ts";
import { isRenderPaused, useRenderPause, useRenderPauseStore } from "./stores/renderPauseStore.ts";
import { Z_INDEX } from "./constants/zIndex.ts";
import { APP_VERSION } from "./config.ts";
import { changeServer, gatewayServer } from "./utils/gateway.ts";
import ServerSwitchPopover from "./components/ui/popover/ServerSwitchPopover.tsx";
import { useKeepTestingParams } from "./utils/quickMode.ts";
import LayoutModeAttribute from "./components/LayoutModeAttribute.tsx";
import LayoutDebugReadout from "./components/LayoutDebugReadout.tsx";
import RotateDeviceOverlay from "./components/ui/overlay/RotateDeviceOverlay.tsx";
import InstallAppBar from "./components/ui/InstallAppBar.tsx";
import InstallHowToSheet from "./components/ui/InstallHowToSheet.tsx";
import ServerDownDialog from "./components/ui/overlay/ServerDownDialog.tsx";
import ServerSelectorScreen from "./components/ui/servers/ServerSelectorScreen.tsx";
import UpdatePill from "./components/ui/UpdatePill.tsx";
import ChangelogModal from "./components/ui/overlay/ChangelogModal.tsx";
import { useVersionCheck } from "./hooks/useVersionCheck.ts";
import "./App.css";

function App() {
  const [isWebSocketReady, setIsWebSocketReady] = useState(false);

  useEffect(() => {
    const initializeWebSocket = async () => {
      try {
        await globalWebSocketManager.initialize();
        setIsWebSocketReady(true);
      } catch (error) {
        console.error("Failed to initialize WebSocket:", error);
        setIsWebSocketReady(true);
      }
    };

    void initializeWebSocket();
  }, []);

  return (
    <SoundProvider>
      <SpaceBackgroundProvider>
        <World3DSettingsProvider>
          <div className="App" style={{ margin: 0, padding: 0 }}>
            <LayoutModeAttribute />
            <LayoutDebugReadout />
            <RotateDeviceOverlay />
            <Router>
              <NotificationProvider>
                <AppWithBackground connectionReady={isWebSocketReady} />
                <NotificationContainer />
              </NotificationProvider>
            </Router>
          </div>
        </World3DSettingsProvider>
      </SpaceBackgroundProvider>
    </SoundProvider>
  );
}

function ConnectionGate({ ready, children }) {
  const location = useLocation();
  if (!ready && location.pathname !== "/cards") {
    return <LoadingOverlay isLoaded={false} showDelayMs={0} message="Connecting to server" />;
  }
  return children;
}

function routeForPathname(pathname) {
  if (pathname === "/load") {
    return "load";
  }
  if (pathname.startsWith("/resume/")) {
    return "resume";
  }
  if (pathname === "/create") {
    return "create";
  }
  if (pathname === "/join") {
    return "join";
  }
  if (pathname === "/cards") {
    return "cards";
  }
  if (pathname === "/reconnecting") {
    return "reconnecting";
  }
  return "landing";
}

function AppWithBackground({ connectionReady }) {
  useKeepTestingParams();
  useEffect(() => {
    document.documentElement.style.setProperty(
      "--surface-layer",
      String(Z_INDEX.CONTROL_DECORATION),
    );
  }, []);
  const location = useLocation();
  const { isLoaded, error } = useSpaceBackground();
  const [overlayVisible, setOverlayVisible] = useState(false);
  const finishBackgroundLoading = useCallback(() => setOverlayVisible(false), []);
  const phase = useAppPhaseStore((s) => s.phase);
  const setPhase = useAppPhaseStore((s) => s.setPhase);

  const inMenuRoute =
    ["/", "/create", "/join", "/cards", "/reconnecting", "/load"].includes(location.pathname) ||
    location.pathname.startsWith("/resume/");
  const isCardsPage = location.pathname === "/cards";
  useVersionCheck(inMenuRoute);
  const { isCompact } = useLayoutMode();
  const ownsMenuChrome =
    isCompact &&
    (location.pathname === "/create" ||
      location.pathname === "/join" ||
      location.pathname === "/load" ||
      location.pathname.startsWith("/resume/") ||
      isCardsPage);
  const browserOpen = useUIOverlayStore((s) => s.showCardBrowser);
  useRenderPause("card-browser", browserOpen);
  const renderPaused = useRenderPauseStore(isRenderPaused);
  const showSpaceBackgroundLayer = !isCardsPage && showsSpaceBackground(phase);
  const showMenuChrome = inMenuRoute && showsMenuChrome(phase);

  useEffect(() => {
    if (!inMenuRoute) {
      return;
    }
    const route = routeForPathname(location.pathname);
    if (phase.kind !== "menu" || phase.route !== route) {
      setPhase({ kind: "menu", route });
    }
  }, [inMenuRoute, location.pathname, phase, setPhase]);

  const skyboxReady = isLoaded || !!error;

  const backgroundPending = inMenuRoute && showSpaceBackgroundLayer && !skyboxReady;
  const showBackgroundLoading =
    backgroundPending || (inMenuRoute && showSpaceBackgroundLayer && overlayVisible);
  // The chrome mounts under the fading overlay, so it is revealed together with the page
  const showMenuChromeNow = showMenuChrome && !backgroundPending && !ownsMenuChrome;

  useEffect(() => {
    if (!inMenuRoute || !showSpaceBackgroundLayer) {
      setOverlayVisible(false);
    } else if (!skyboxReady) {
      setOverlayVisible(true);
    }
  }, [inMenuRoute, showSpaceBackgroundLayer, skyboxReady]);

  useEffect(() => {
    if (showSpaceBackgroundLayer && isLoaded) {
      audioService.playAmbient();
    }
  }, [showSpaceBackgroundLayer, isLoaded]);

  return (
    <>
      <div
        style={{
          opacity: showSpaceBackgroundLayer ? 1 : 0,
          transition: `opacity ${phase.kind === "fadeOutLobby" ? 2500 : 1500}ms ease-out`,
          pointerEvents: showSpaceBackgroundLayer ? "auto" : "none",
        }}
      >
        <SpaceBackground active={showSpaceBackgroundLayer && !renderPaused} />
      </div>
      {showBackgroundLoading && (
        <LoadingOverlay
          isLoaded={skyboxReady}
          onTransitionEnd={finishBackgroundLoading}
          showDelayMs={0}
          minDurationMs={500}
          showProgress
        />
      )}
      {showMenuChromeNow && <MainMenuHamburger />}
      {showMenuChromeNow && <MenuFooter visible={!isCardsPage} />}
      <InstallAppBar active={showMenuChromeNow} />
      <InstallHowToSheet />
      <ServerDownDialog />
      <ServerSelectorScreen />
      <UpdatePill inMenuRoute={inMenuRoute} />
      <ChangelogModal />
      <ConnectionGate ready={connectionReady}>
        <Routes>
          <Route path="/" element={<GameLandingPage />} />
          <Route path="/load" element={<LoadGamePage />} />
          <Route path="/resume/:gameId" element={<ResumeGamePage />} />
          <Route path="/create" element={<CreateGamePage />} />
          <Route path="/join" element={<JoinGamePage />} />
          <Route path="/cards" element={<CardsPage />} />
          <Route path="/reconnecting" element={<ReconnectingPage />} />
          <Route path="/game/:gameId" element={<GameInterface />} />
          <Route path="/game" element={<GameInterface />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </ConnectionGate>
    </>
  );
}

function MenuFooter({ visible = true }) {
  const [showFeedbackWindow, setShowFeedbackWindow] = useState(false);
  const [showServers, setShowServers] = useState(false);
  const changeServerRef = useRef(null);
  const closeServers = useCallback(() => setShowServers(false), []);
  // Older gateways hand over no server list; their picker page does the job
  const openServers = gatewayServer?.servers ? () => setShowServers((open) => !open) : changeServer;

  useEffect(() => {
    const handleToggleFeedback = () => setShowFeedbackWindow((prev) => !prev);
    window.addEventListener("toggle-feedback-window", handleToggleFeedback);
    return () => window.removeEventListener("toggle-feedback-window", handleToggleFeedback);
  }, []);

  return (
    <>
      {visible && (
        <div
          className="menu-footer flex items-center justify-between text-white/30 text-xs select-none pointer-events-none compact:hidden"
          style={{ zIndex: Z_INDEX.COST_DISPLAY }}
        >
          <span className="pointer-events-auto">
            {gatewayServer && (
              <>
                {gatewayServer.name}
                <span className="mx-1">·</span>
              </>
            )}
            {APP_VERSION}
            <span className="mx-1">|</span>
            {gatewayServer && (
              <>
                <GameButton
                  ref={changeServerRef}
                  emphasis="quiet"
                  size="xs"
                  className="!p-0 !min-h-0 compact:!min-h-11 hover:text-white/70 transition-colors cursor-pointer"
                  onClick={openServers}
                >
                  Change server
                </GameButton>
                <span className="mx-1">|</span>
              </>
            )}
            <GameButton
              emphasis="quiet"
              size="xs"
              className="!p-0 !min-h-0 compact:!min-h-11 hover:text-white/70 transition-colors cursor-pointer"
              onClick={() => window.dispatchEvent(new CustomEvent("toggle-feedback-window"))}
            >
              Feedback
            </GameButton>
          </span>
          <GameButton
            as="link"
            to="/cards"
            emphasis="secondary"
            size="sm"
            className="pointer-events-auto"
          >
            View cards
          </GameButton>
        </div>
      )}
      {gatewayServer?.servers && (
        <ServerSwitchPopover
          isVisible={visible && showServers}
          onClose={closeServers}
          anchorRef={changeServerRef}
        />
      )}
      <WindowManagerProvider>
        <FeedbackWindow
          isVisible={showFeedbackWindow}
          onClose={() => setShowFeedbackWindow(false)}
          gameState={null}
        />
      </WindowManagerProvider>
    </>
  );
}

export default App;
