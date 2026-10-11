import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { flushSync } from "react-dom";
import Game3DView from "../../components/game/view/Game3DView";
import { World3DSettingsProvider } from "../../contexts/World3DSettingsContext";
import { PlanetFocusProvider } from "../../contexts/PlanetFocusContext";
import { ScenePreparationContext } from "../../contexts/ScenePreparationContext";
import LayoutModeAttribute from "../../components/LayoutModeAttribute";
import RotateDeviceOverlay from "../../components/ui/overlay/RotateDeviceOverlay";
import { audioService } from "../../services/audioService";
import { GRAPHICS } from "../../utils/graphicsQuality";
import { cardTargets, label, resolveTarget, tileTypes } from "./catalog";
import {
  createScene,
  type SceneOptions,
  type InspectionView,
  type ClimatePreset,
  type InspectionState,
} from "./scene";
import { InspectionDriver, FPS } from "./driver";
import { Z_INDEX } from "../../constants/zIndex";
import "./inspection.css";

import type { InspectionBridge } from "./protocol";
const query = new URLSearchParams(window.location.search);
function initialOptions(): SceneOptions | null {
  if (!query.has("card") && !query.has("tile") && query.get("scene") !== "structures") {
    return null;
  }
  if (!Number.isSafeInteger(Number(query.get("seed") ?? 42))) {
    throw new Error("Seed must be an integer");
  }
  if (
    !["comparison", "structures", "neighbors", "destruction"].includes(
      query.get("scene") ?? "comparison",
    )
  ) {
    throw new Error("Unknown scene");
  }
  if (!["barren", "growing", "terraformed"].includes(query.get("climate") ?? "growing")) {
    throw new Error("Unknown climate");
  }
  if (!["settled", "placement"].includes(query.get("state") ?? "settled")) {
    throw new Error("Unknown state");
  }
  if (!/^#[\da-f]{6}$/i.test(query.get("color") ?? "#e43c32")) {
    throw new Error("Invalid colour");
  }
  const frame = Number(query.get("frame") ?? 0);
  if (!Number.isSafeInteger(frame) || frame < 0 || frame > 36000) {
    throw new Error("Frame must be an integer between 0 and 36000.");
  }
  return {
    target: resolveTarget(
      query.get("card"),
      query.get("tile") ?? (query.get("scene") === "structures" ? "city" : null),
    ),
    scene: (query.get("scene") ?? "comparison") as SceneOptions["scene"],
    seed: Number(query.get("seed") ?? 42),
    color: query.get("color") ?? "#e43c32",
    climate: (query.get("climate") ?? "growing") as ClimatePreset,
    state: (query.get("state") ?? "settled") as InspectionState,
  };
}
function configuredURL(options: SceneOptions, view: InspectionView, frame: number) {
  const params = new URLSearchParams({
    scene: options.scene ?? "comparison",
    tile: options.target.tile,
    seed: String(options.seed),
    color: options.color,
    climate: options.climate,
    state: options.state,
    view,
    frame: String(frame),
    gfx: GRAPHICS.tier,
  });
  if (options.target.card) {
    params.set("card", options.target.card);
  }
  return `/dev/board?${params}`;
}
function Inspector({ initial }: { initial: SceneOptions | null }) {
  const [options, setOptions] = useState(initial);
  const [view, setView] = useState<InspectionView>(
    query.get("view") === "close" ? "close" : "gameplay",
  );
  const [frame, setFrame] = useState(Number(query.get("frame") ?? 0));
  const [requestedFrame, setRequestedFrame] = useState(String(frame));
  useEffect(() => setRequestedFrame(String(frame)), [frame]);
  const [epoch, setEpoch] = useState(0);
  const [placed, setPlaced] = useState(initial?.state !== "placement");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [collapsed, setCollapsed] = useState(query.has("capture"));
  const [playing, setPlaying] = useState(false);
  const driver = useRef(new InspectionDriver());
  const mounted = useRef(true);
  const initialSeek = useRef(true);
  const operation = useRef(false);
  const seekRef = useRef<(frame: number, reset?: boolean, settle?: boolean) => Promise<void>>(
    async () => {},
  );
  const bridge = useRef<InspectionBridge>({
    ready: false,
    error: null,
    seek: (next) => seekRef.current(next),
    setView: async () => {},
    report: () => ({}),
  });
  const current = useRef({ options, view, frame });
  current.current = { options, view, frame };

  const seek = useCallback(async (next: number, reset = false, settle = true) => {
    const config = current.current;
    if (!config.options) {
      return;
    }
    if (!Number.isSafeInteger(next) || next < 0 || next > 36_000) {
      throw new Error("Frame must be an integer between 0 and 36000 (10 minutes).");
    }
    if (operation.current) {
      throw new Error("Inspection is already preparing a frame");
    }
    operation.current = true;
    bridge.current.ready = false;
    setBusy(true);
    setError(null);
    bridge.current.error = null;
    try {
      if (reset || !driver.current.ready || next < driver.current.frame) {
        driver.current.disposed = true;
        const fresh = new InspectionDriver();
        driver.current = fresh;
        flushSync(() => {
          setPlaced(config.options!.state !== "placement");
          setEpoch((value) => value + 1);
        });
        await fresh.prepare(config.view, config.options.scene);
        if (config.options.state === "placement") {
          flushSync(() => setPlaced(true));
          await fresh.drain();
        }
      }
      await driver.current.forward(next, settle);
      driver.current.ready = true;
      bridge.current.ready = true;
      setFrame(next);
      if (settle) {
        history.replaceState(
          null,
          "",
          configuredURL(config.options, config.view, next) +
            (query.has("capture") ? "&capture" : ""),
        );
      }
    } catch (reason) {
      if (!mounted.current) {
        return;
      }
      const message = reason instanceof Error ? reason.message : String(reason);
      setError(message);
      bridge.current.error = message;
      throw reason;
    } finally {
      operation.current = false;
      if (mounted.current) {
        setBusy(false);
      }
    }
  }, []);
  seekRef.current = seek;
  bridge.current.setView = async (next) => {
    if (next !== "close" && next !== "gameplay") {
      throw new Error("Unknown camera view");
    }
    if (operation.current) {
      throw new Error("Inspection is busy");
    }
    operation.current = true;
    setBusy(true);
    try {
      current.current.view = next;
      setView(next);
      driver.current.focus(next, current.current.options?.scene);
      await driver.current.drain();
      if (current.current.options) {
        history.replaceState(
          null,
          "",
          configuredURL(current.current.options, next, driver.current.frame) +
            (query.has("capture") ? "&capture" : ""),
        );
      }
    } finally {
      operation.current = false;
      setBusy(false);
    }
  };
  bridge.current.report = () => ({
    ...current.current.options,
    view: current.current.view,
    ...driver.current.diagnostics(),
    graphicsTier: GRAPHICS.tier,
    url: location.href,
    viewport: { width: innerWidth, height: innerHeight, devicePixelRatio },
    captureSucceeded: bridge.current.ready,
    visualReview: "required",
  });
  useEffect(() => {
    mounted.current = true;
    audioService.setEnabled(false);
    audioService.setMusicEnabled(false);
    window.__openMarsInspection = bridge.current;
    return () => {
      mounted.current = false;
      driver.current.disposed = true;
      delete window.__openMarsInspection;
    };
  }, []);
  useEffect(() => {
    const timer = setTimeout(() => {
      if (options) {
        const next = initialSeek.current ? Number(query.get("frame") ?? 0) : 0;
        initialSeek.current = false;
        void seekRef.current(next, true).catch(() => {});
      }
    }, 0);
    return () => clearTimeout(timer);
  }, [options]);
  useEffect(() => {
    if (!playing) {
      return;
    }
    let cancelled = false;
    const play = async () => {
      while (!cancelled) {
        if (!operation.current) {
          await seekRef.current(driver.current.frame + 1, false, false).catch(() => {
            cancelled = true;
            setPlaying(false);
          });
        }
        await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      }
      const config = current.current;
      if (mounted.current && config.options) {
        history.replaceState(
          null,
          "",
          configuredURL(config.options, config.view, driver.current.frame),
        );
      }
    };
    void play();
    return () => {
      cancelled = true;
    };
  }, [playing]);
  // Continue drawing frozen frames for camera input, without advancing effects.
  useEffect(() => {
    let handle = 0;
    const draw = () => {
      if (!operation.current && driver.current.ready) {
        driver.current.root?.advance(driver.current.seconds, false);
      }
      handle = requestAnimationFrame(draw);
    };
    handle = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(handle);
  }, []);
  const change = (updates: Partial<SceneOptions>) => {
    setPlaying(false);
    if (options) {
      setOptions({ ...options, ...updates });
    }
  };
  const select = (value: string) => {
    const [kind, id, tile] = value.split(":");
    setPlaying(false);
    setOptions({
      target: kind === "card" ? resolveTarget(id, tile) : resolveTarget(null, id),
      seed: options?.seed ?? 42,
      color: options?.color ?? "#e43c32",
      climate: options?.climate ?? "growing",
      state: options?.state ?? "settled",
    });
  };
  const scene = useMemo(() => (options ? createScene(options, placed) : null), [options, placed]);
  let subject = "";
  if (options) {
    subject = options.target.card
      ? `card:${options.target.card}:${options.target.tile}`
      : `tile:${options.target.tile}`;
  }
  let status = "Choose a subject to load the scene.";
  if (options) {
    status = `Ready · frame ${frame}`;
  }
  if (busy) {
    status = "Preparing scene…";
  }
  return (
    <main className="inspection" data-inspection-ready={driver.current.ready && !busy && !error}>
      <LayoutModeAttribute />
      <RotateDeviceOverlay />
      {scene && (
        <World3DSettingsProvider key={epoch}>
          <PlanetFocusProvider>
            <ScenePreparationContext.Provider value={driver.current.preparation}>
              <Game3DView
                gameState={scene}
                onHexClick={() => {}}
                manualFrames
                showUI={false}
                onCreated={(root) => {
                  driver.current.root = root;
                }}
                onGpuReady={() => {
                  driver.current.gpuReady = true;
                }}
                onSkyboxReady={() => {
                  driver.current.skyboxReady = true;
                }}
              />
            </ScenePreparationContext.Provider>
          </PlanetFocusProvider>
        </World3DSettingsProvider>
      )}
      {!query.has("capture") && (
        <button
          className="inspection-toggle"
          style={{ zIndex: Z_INDEX.TOP_MENU_BAR }}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "Inspection tools" : "Hide tools"}
        </button>
      )}
      {!collapsed && (
        <section
          className="inspection-tools"
          style={{ zIndex: Z_INDEX.TOP_MENU_BAR }}
          aria-label="Board inspection tools"
        >
          <h1>Board inspection</h1>
          <p>Production renderer · {GRAPHICS.tier} graphics · development only</p>
          <label>
            Subject
            <select
              aria-label="Subject"
              disabled={busy}
              value={subject}
              onChange={(event) => select(event.target.value)}
            >
              <option value="" disabled>
                Select a card or tile
              </option>
              <optgroup label="Tiles">
                {tileTypes.map((tile) => (
                  <option key={tile} value={`tile:${tile}`}>
                    {label(tile)}
                  </option>
                ))}
              </optgroup>
              <optgroup label="Cards">
                {cardTargets.map((target) => (
                  <option
                    key={`${target.card}:${target.tile}`}
                    value={`card:${target.card}:${target.tile}`}
                  >
                    {target.card} · {target.name}
                    {target.name !== label(target.tile) && ` · ${label(target.tile)}`}
                  </option>
                ))}
              </optgroup>
            </select>
          </label>
          {options && (
            <>
              <label>
                Scene
                <select
                  disabled={busy}
                  value={options.scene ?? "comparison"}
                  onChange={(e) =>
                    change({
                      scene: e.target.value as
                        | "comparison"
                        | "structures"
                        | "neighbors"
                        | "destruction",
                      state: e.target.value === "destruction" ? "placement" : "settled",
                    })
                  }
                >
                  <option value="comparison">Tile comparison</option>
                  <option value="structures">Structure styles</option>
                  <option value="neighbors">Neighboring structures</option>
                  <option value="destruction">Destruction</option>
                </select>
              </label>
              <p>
                Subject beside a city, greenery and ocean. Drag to pan, scroll or pinch to zoom.
              </p>
              <div className="inspection-row">
                <label>
                  Climate
                  <select
                    disabled={busy}
                    value={options.climate}
                    onChange={(e) => change({ climate: e.target.value as ClimatePreset })}
                  >
                    <option value="barren">Barren</option>
                    <option value="growing">Growing</option>
                    <option value="terraformed">Terraformed</option>
                  </select>
                </label>
                <label>
                  Owner
                  <input
                    aria-label="Owner colour"
                    disabled={busy}
                    type="color"
                    value={options.color}
                    onChange={(e) => change({ color: e.target.value })}
                  />
                </label>
              </div>
              <label>
                State
                <select
                  disabled={busy || options.scene === "structures"}
                  value={options.state}
                  onChange={(e) => change({ state: e.target.value as InspectionState })}
                >
                  <option value="settled">Settled</option>
                  <option value="placement">Placement</option>
                </select>
              </label>
              <div className="inspection-row">
                <button disabled={busy} onClick={() => void bridge.current.setView("gameplay")}>
                  Gameplay
                </button>
                <button disabled={busy} onClick={() => void bridge.current.setView("close")}>
                  Close
                </button>
                <button disabled={busy} onClick={() => void bridge.current.setView(view)}>
                  Reset camera
                </button>
              </div>
              <label>
                Frame · {(frame / FPS).toFixed(2)} s
                <input
                  aria-label="Frame"
                  type="number"
                  min="0"
                  max="36000"
                  disabled={busy || playing}
                  value={requestedFrame}
                  onChange={(e) => setRequestedFrame(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      void seek(Number(requestedFrame)).catch(() => {});
                    }
                  }}
                />
              </label>
              <button
                disabled={busy || playing}
                onClick={() => void seek(Number(requestedFrame)).catch(() => {})}
              >
                Go to frame
              </button>
              <input
                aria-label="Timeline"
                type="range"
                min="0"
                max={Math.max(900, frame)}
                step="1"
                disabled={busy || playing}
                value={Number(requestedFrame) || 0}
                onChange={(e) => setRequestedFrame(e.target.value)}
                onPointerUp={(e) => void seek(Number(e.currentTarget.value)).catch(() => {})}
                onKeyUp={(e) => void seek(Number(e.currentTarget.value)).catch(() => {})}
              />
              <div className="inspection-row">
                <button disabled={busy && !playing} onClick={() => setPlaying(!playing)}>
                  {playing ? "Pause" : "Play"}
                </button>
                <button
                  disabled={busy || playing}
                  onClick={() => void seek(Math.max(0, frame - 1)).catch(() => {})}
                >
                  −1 frame
                </button>
                <button
                  disabled={busy || playing}
                  onClick={() => void seek(frame + 1).catch(() => {})}
                >
                  +1 frame
                </button>
                <button
                  disabled={busy}
                  onClick={() => {
                    setPlaying(false);
                    void seek(0, true).catch(() => {});
                  }}
                >
                  Replay
                </button>
              </div>
              <button onClick={() => void navigator.clipboard.writeText(location.href)}>
                Copy inspection link
              </button>
            </>
          )}
          <p role="status">{status}</p>
          {error && <p role="alert">{error}</p>}
        </section>
      )}
      {query.has("capture") && error && (
        <p className="inspection-error" role="alert">
          {error}
        </p>
      )}
    </main>
  );
}
export default function InspectionApp() {
  let options: SceneOptions | null;
  try {
    options = initialOptions();
  } catch (reason) {
    return (
      <main className="inspection">
        <LayoutModeAttribute />
        <RotateDeviceOverlay />
        <p className="inspection-error" role="alert">
          {String(reason)}
        </p>
      </main>
    );
  }
  return <Inspector initial={options} />;
}
