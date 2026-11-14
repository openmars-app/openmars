import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import EngineCanvas from "./EngineCanvas";
import { useEngineStore } from "@/stores/engineStore";
import { Z_INDEX } from "@/constants/zIndex";
import GameButton from "@/components/ui/buttons/GameButton";
import { useGameStore } from "@/stores/gameStore";
import { gameIdOf, isInitializing, needsGameScene, useAppPhaseStore } from "@/stores/appPhaseStore";
import { sendTileSelection, usePlacementSelectionStore } from "@/stores/placementSelectionStore";
import { getLayoutMode } from "@/hooks/useLayoutMode";
import { toggleInspectedHex, useTileInspectStore } from "@/stores/tileInspectStore";
import {
  placementDestination,
  routeSceneEvent,
  toSceneSnapshot,
  worldEntryForPhase,
} from "./scene";
import { audioService } from "@/services/audioService";
import { describeTile } from "./tileInspection";
import ClientSettings from "../components/ui/overlay/ClientSettings";
import SolarLabels from "./SolarLabels";
import StationMarker from "./StationMarker";
import type { MenuPlacement, SceneEvent } from "./scene.generated";
import { useEngineProfilerStore } from "../stores/engineProfilerStore";
import { engineProfiler } from "./profiler";
import { useGraphicsSettingsStore } from "../stores/graphicsSettingsStore";
import { residentGraphicsKey } from "./graphics";

export default function EngineBackground({
  paused = false,
  menu = true,
}: {
  paused?: boolean;
  menu?: boolean;
}) {
  const [attempt, setAttempt] = useState(0);
  const graphicsKey = useGraphicsSettingsStore((s) => residentGraphicsKey(s.settings));
  const hasCapture = useEngineProfilerStore((state) => state.eventCount > 0);
  const downloadTrace = useEngineProfilerStore((state) => state.download);
  const status = useEngineStore((s) => s.status);
  const setStatus = useEngineStore((s) => s.setStatus);
  const game = useGameStore((s) => s.game);
  const connected = useGameStore((s) => s.isConnected);
  const phase = useAppPhaseStore((s) => s.phase);
  const selected = usePlacementSelectionStore((s) => s.selectedHex);
  // Reconnecting needs the authoritative world before it can leave initialization.
  const prepareGame = needsGameScene(phase) || (isInitializing(phase) && connected);
  const activeGame = prepareGame && gameIdOf(phase) === game?.id ? game : null;
  let menuPlacement: MenuPlacement | null = null;
  if (menu) {
    menuPlacement = "lobby";
    if (phase.kind === "menu") {
      menuPlacement = phase.route === "landing" ? "landing" : "panel";
    }
  }
  const scene = useMemo(() => {
    const endSnapshot = engineProfiler.span("scene.convert", undefined, "bridge.cpu");
    try {
      return toSceneSnapshot(activeGame, selected);
    } finally {
      endSnapshot();
    }
  }, [activeGame, selected]);
  const placementKey = JSON.stringify([
    activeGame?.id,
    activeGame?.settings.mapId,
    activeGame?.currentPlayer.pendingTileSelection,
  ]);
  const previousPlacement = useRef<string | null>(null);
  useEffect(() => {
    useEngineStore.getState().setFocus({ type: "body", body: "mars" });
    usePlacementSelectionStore.getState().clear();
    useTileInspectStore.getState().clear();
  }, [activeGame?.id, activeGame?.settings.mapId]);
  useEffect(() => {
    if (previousPlacement.current === placementKey) {
      return;
    }
    previousPlacement.current = placementKey;
    const destination = placementDestination(scene);
    const state = useEngineStore.getState();
    if (destination && (state.focus.type !== "body" || state.focus.body !== destination)) {
      void audioService.playTravelSound();
      state.setFocus({ type: "body", body: destination });
    }
  }, [placementKey, scene]);
  useEffect(() => {
    const inspection = useTileInspectStore.getState();
    if (!inspection.inspectedHex || !inspection.position) {
      return;
    }
    const info = describeTile(activeGame, inspection.inspectedHex);
    if (info) {
      inspection.inspect(inspection.inspectedHex, info, inspection.position);
    } else {
      inspection.clear();
    }
  }, [activeGame]);
  const onEvent = (event: SceneEvent) => {
    routeSceneEvent(event, activeGame, {
      focus: (body) => {
        void audioService.playTravelSound();
        useEngineStore.getState().setFocus({ type: "body", body });
      },
      select: (id) => {
        const placement = usePlacementSelectionStore.getState();
        if (getLayoutMode().isCompact && placement.selectedHex !== id) {
          placement.select(id);
          return;
        }
        sendTileSelection(id);
      },
      hover: (id, x, y) => {
        if (getLayoutMode().isCompact) {
          return;
        }
        const inspection = useTileInspectStore.getState();
        const info = id ? describeTile(activeGame, id) : null;
        if (id && info) {
          inspection.inspect(id, info, { x, y });
        } else {
          inspection.clear();
        }
      },
      inspect: (id, x, y) => {
        if (getLayoutMode().isCompact) {
          toggleInspectedHex(id, (key) => describeTile(activeGame, key), { x, y });
        } else {
          const info = describeTile(activeGame, id);
          if (info) {
            useTileInspectStore.getState().inspect(id, info, { x, y });
          }
        }
      },
    });
  };
  return (
    <>
      <ClientSettings />
      <div
        className="fixed inset-0 bg-black"
        style={{ zIndex: Z_INDEX.GAME_BOARD_BACKGROUND }}
        data-engine-state={status.phase}
        data-engine-backend={status.backend}
      >
        <EngineCanvas
          key={`${attempt}:${graphicsKey}`}
          onStatus={setStatus}
          paused={paused}
          scene={scene}
          menuPlacement={menuPlacement}
          entry={worldEntryForPhase(phase)}
          connected={connected}
          onEvent={onEvent}
        />
        {menu && !activeGame && (
          <div
            className="pointer-events-none absolute inset-0"
            style={{ background: "linear-gradient(to right, rgba(0,0,0,0.5), transparent 70%)" }}
          />
        )}
      </div>
      {activeGame && status.phase === "ready" && !paused && (
        <>
          <SolarLabels key={activeGame.id} gameId={activeGame.id} />
          <StationMarker key={`station-${activeGame.id}`} gameId={activeGame.id} />
        </>
      )}
      {status.phase === "error" &&
        createPortal(
          <div
            role="alert"
            className="fixed inset-0 flex flex-col items-center justify-center gap-5 bg-black p-6 text-center text-white"
            style={{ zIndex: Z_INDEX.ENGINE_ERROR }}
          >
            <h1 className="font-orbitron text-xl">Unable to start the renderer</h1>
            <p className="max-w-lg">{status.message}</p>
            <div className="flex flex-wrap justify-center gap-3">
              <GameButton onClick={() => setAttempt((value) => value + 1)}>Try again</GameButton>
              {hasCapture && <GameButton onClick={downloadTrace}>Download trace</GameButton>}
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}
