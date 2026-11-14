import { TerrainPreparation } from "./terrainPreparation";
import { backendPreference, type BackendPreference, type EngineBackend } from "./backend";
import { readHudInsets } from "../hooks/useHudInsets";
import { getLayoutMode } from "../hooks/useLayoutMode";
import type { SceneDiagnostics } from "./diagnostics";
import type { EngineStatus } from "./status";
import type {
  AssetDemand,
  FocusTarget,
  OverlayFrame,
  MenuPlacement,
  WorldEntry,
  SceneSound,
  SceneEvent,
  SceneSnapshot,
} from "./scene.generated";
import { invalidateWorldAssets, loadWorldAssets } from "./worldAssets";
import { EngineMeter, type EnginePerformance } from "./performance";
import { CameraMotion, wheelZoom } from "./cameraMotion";
import { useCameraSettingsStore } from "../stores/cameraSettingsStore";
import { engineProfiler } from "./profiler";
import { FrameLimiter, renderDimensions, residentGraphicsKey } from "./graphics";
import { useGraphicsSettingsStore } from "../stores/graphicsSettingsStore";
import type { GraphicsSettings } from "./scene.generated";

interface Renderer {
  submitted_frames(): number;
  performance_stats(): string;
  browser_device(): unknown;
  linear_memory(): WebAssembly.Memory;
  set_profiling(active: boolean, hostTimeMs: number): void;
  set_profile_context(context: number): void;
  drain_profile(maxEvents: number): string;
  frame(width: number, height: number, deltaSeconds: number): boolean;
  prepare_cpu(): boolean;
  terrain_request(): Uint8Array;
  accept_terrain(bytes: Uint8Array): boolean;
  preparation(): string;
  supports_compute(): boolean;
  supports_storage_buffers(): boolean;
  max_texture_dimension_2d(): number;
  take_sounds(): string;
  set_scene(json: string, settled: boolean): string;
  set_menu(json: string): string;
  set_world_entry(json: string): void;
  set_graphics(json: string): void;
  set_focus(json: string): string;
  menu_pointer(x: number, y: number): void;
  asset_demand(): string;
  set_reduced_motion(reduced: boolean): void;
  install_asset(request: number, id: string, filename: string, bytes: Uint8Array): void;
  asset_stats(): string;
  pointer(x: number, y: number, clientX: number, clientY: number, inspect: boolean): string;
  hover(x: number, y: number, clientX: number, clientY: number): string;
  camera_input(yaw: number, pitch: number, zoom: number, reset: boolean): void;
  viewport(
    metrics: Float32Array,
    physicalWidth: number,
    physicalHeight: number,
    compact: boolean,
  ): void;
  scene_stats(): string;
  overlay_frame(): string;
  free(): void;
}

interface WasmModule {
  default(input: { module_or_path: Uint8Array }): Promise<unknown>;
  Engine: { create(canvas: HTMLCanvasElement): Promise<Renderer> };
}
const modules = new Map<EngineBackend, Promise<WasmModule>>();
const initialized = new Set<EngineBackend>();
const loaders = {
  webgpu: () => import("../../../engine/pkg/webgpu/openmars_engine.js"),
  webgl2: () => import("../../../engine/pkg/webgl2/openmars_engine.js"),
};
const urls = {
  webgpu: new URL("../../../engine/pkg/webgpu/openmars_engine_bg.wasm", import.meta.url),
  webgl2: new URL("../../../engine/pkg/webgl2/openmars_engine_bg.wasm", import.meta.url),
};

async function loadModule(backend: EngineBackend) {
  let promise = modules.get(backend);
  if (!promise) {
    promise = loaders[backend]();
    modules.set(backend, promise);
  }
  try {
    return await promise;
  } catch (error) {
    modules.delete(backend);
    throw error;
  }
}

export function startEngine(
  host: HTMLElement,
  report: (status: EngineStatus) => void,
  preference: BackendPreference = backendPreference(),
  onEvent: (event: SceneEvent) => void = () => {},
  onSound: (event: SceneSound) => void = () => {},
  onOverlay: (frame: OverlayFrame | null) => void = () => {},
) {
  let disposed = false;
  let graphics = useGraphicsSettingsStore.getState().settings;
  let applyGraphics: ((settings: GraphicsSettings) => void) | undefined;
  let paused = false;
  let cleanupAttempt = () => {};
  let sceneJSON = "null";
  let sceneSnapshot: SceneSnapshot | null = null;
  let menuJSON = "null";
  let entryJSON = JSON.stringify({ tiles: "settled", lighting: "steady" } satisfies WorldEntry);
  let focusJSON = JSON.stringify({ type: "body", body: "mars" } satisfies FocusTarget);
  let sceneMapId: string | null = null;
  let applyFocus: ((json: string) => void) | undefined;
  let applyMenu: ((json: string) => void) | undefined;
  let applyEntry: ((json: string) => void) | undefined;
  let sceneGameId: string | null = null;
  let sceneDelivery: "snapshot" | "live" = "snapshot";
  let applyScene: ((json: string, delivery: "snapshot" | "live") => void) | undefined;
  let readSceneStats: (() => SceneDiagnostics | null) | undefined;
  let performanceActive = false;
  let readPerformance: (() => EnginePerformance | null) | undefined;
  let activatePerformance: ((active: boolean) => void) | undefined;
  let resetFrameClock: (() => void) | undefined;
  let cameraCommand: ((zoom: number, reset: boolean) => void) | undefined;
  let readAssetStats: (() => Record<string, number>) | undefined;
  let clearCameraMotion: (() => void) | undefined;
  const emit = (status: EngineStatus) => {
    if (!disposed) report(status);
  };

  const startBackend = async (backend: EngineBackend) => {
    const abort = new AbortController();
    const { signal } = abort;
    const canvas = document.createElement("canvas");
    canvas.setAttribute("aria-label", "Open Mars game world");
    canvas.tabIndex = 0;
    canvas.dataset.engineBackend = backend;
    canvas.style.cssText = "display:block;width:100%;height:100%;touch-action:none";
    host.replaceChildren(canvas);
    let engine: Renderer | undefined;
    let meter: EngineMeter | undefined;
    let unregisterNative: (() => void) | undefined;
    let unregisterDevice: (() => void) | undefined;
    let appliedScene: SceneSnapshot | null = null;
    let frame = 0;
    let ready = false;
    let started = false;
    let preparation = "";
    let overlayJSON = "";
    let assetDownload: AbortController | undefined;
    let assetRequest: number | null = null;
    let requiresAssets = false;
    let downloadingAssets = false;
    let sceneTimer: number | undefined;
    const cpuChannel = new MessageChannel();
    let cpuScheduled = false;
    let cpuToken = 0;
    let terrainPreparation: TerrainPreparation | undefined;
    const cancelCpu = () => {
      cpuScheduled = false;
      cpuToken++;
    };
    let resolveReady: () => void;
    let rejectReady: (reason: unknown) => void;
    const cameraMotion = new CameraMotion();
    clearCameraMotion = () => cameraMotion.clear();
    const firstFrame = new Promise<void>((resolve, reject) => {
      resolveReady = resolve;
      rejectReady = reject;
    });
    // Download/creation may fail before the first-frame promise is awaited.
    void firstFrame.catch(() => {});
    const aborted = new Promise<never>((_, reject) => {
      signal.addEventListener(
        "abort",
        () => {
          reject(signal.reason);
          rejectReady(signal.reason);
        },
        { once: true },
      );
    });
    void aborted.catch(() => {});
    const timer = window.setTimeout(
      () => abort.abort(new Error("The renderer did not finish starting.")),
      60_000,
    );
    cleanupAttempt = () => {
      engineProfiler.resetFrameGap();
      engineProfiler.flush();
      unregisterNative?.();
      unregisterNative = undefined;
      unregisterDevice?.();
      unregisterDevice = undefined;
      meter?.setActive(false);
      meter = undefined;
      readPerformance = undefined;
      activatePerformance = undefined;
      onOverlay(null);
      applyScene = undefined;
      applyMenu = undefined;
      applyEntry = undefined;
      applyFocus = undefined;
      cameraCommand = undefined;
      readAssetStats = undefined;
      readSceneStats = undefined;
      resetFrameClock = undefined;
      cameraMotion.clear();
      clearCameraMotion = undefined;
      assetDownload?.abort();
      window.clearTimeout(sceneTimer);
      cancelCpu();
      cpuChannel.port1.close();
      cpuChannel.port2.close();
      terrainPreparation?.dispose();
      terrainPreparation = undefined;
      window.clearTimeout(timer);
      abort.abort();
      cancelAnimationFrame(frame);
      const previous = engine;
      engine = undefined;
      try {
        previous?.free();
      } catch (error) {
        console.error("Renderer cleanup failed", error);
      }
    };

    if (
      backend === "webgpu" &&
      (!window.isSecureContext || !(navigator as Navigator & { gpu?: unknown }).gpu)
    ) {
      throw new Error("WebGPU is unavailable in this browser or connection.");
    }
    emit({ phase: "loading", backend, message: "Loading renderer", loaded: 0, total: null });
    const endModule = engineProfiler.span("wasm.module", { backend }, "loading.wait");
    let module: WasmModule;
    try {
      module = await Promise.race([loadModule(backend), aborted]);
    } finally {
      endModule();
    }
    if (disposed) throw new Error("Renderer disposed");
    if (!initialized.has(backend)) {
      const endDownload = engineProfiler.span("wasm.download", { backend }, "loading.wait");
      let bytes: Uint8Array;
      try {
        const response = await fetch(urls[backend], { signal });
        if (!response.ok) throw new Error(`Renderer download failed (${response.status})`);
        const rawTotal = Number(response.headers.get("content-length"));
        const total = !response.headers.get("content-encoding") && rawTotal > 0 ? rawTotal : null;
        const chunks: Uint8Array[] = [];
        let loaded = 0;
        if (response.body) {
          const reader = response.body.getReader();
          while (true) {
            const { value, done } = await reader.read();
            if (done) break;
            chunks.push(value);
            loaded += value.byteLength;
            emit({ phase: "loading", backend, message: "Loading renderer", loaded, total });
          }
        } else {
          const chunk = new Uint8Array(await response.arrayBuffer());
          chunks.push(chunk);
          loaded = chunk.length;
        }
        bytes = new Uint8Array(loaded);
        let offset = 0;
        for (const chunk of chunks) {
          bytes.set(chunk, offset);
          offset += chunk.length;
        }
        if (signal.aborted) throw signal.reason;
      } finally {
        endDownload();
      }
      const endCompile = engineProfiler.span("wasm.compile", { backend }, "loading.wait");
      try {
        await module.default({ module_or_path: bytes });
      } finally {
        endCompile();
      }
      initialized.add(backend);
    }
    emit({ phase: "preparing", backend, message: "Preparing renderer" });
    const endCreation = engineProfiler.span("engine.create", { backend }, "loading.wait");
    const creation = module.Engine.create(canvas).then((created) => {
      if (signal.aborted) {
        created.free();
        throw signal.reason;
      }
      return created;
    });
    try {
      engine = await Promise.race([creation, aborted]);
    } finally {
      endCreation();
    }
    const profilingEngine = engine;
    engine.set_graphics(JSON.stringify(graphics));
    applyGraphics = (settings) => engine?.set_graphics(JSON.stringify(settings));
    unregisterNative = engineProfiler.native({
      setEnabled: (active, hostNowMs) => profilingEngine.set_profiling(active, hostNowMs),
      setContext: (context) => profilingEngine.set_profile_context(context),
      drain: (maxEvents) => JSON.parse(profilingEngine.drain_profile(maxEvents)),
    });
    unregisterDevice = engineProfiler.device(profilingEngine.browser_device());
    engineProfiler.metadata({
      backend,
      devicePixelRatio: window.devicePixelRatio,
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
    });
    meter = new EngineMeter(engine, backend, canvas);
    meter.setActive(performanceActive);
    activatePerformance = (active) => meter?.setActive(active);
    readPerformance = () =>
      meter?.sample(paused || document.hidden ? "paused" : ready ? "rendering" : "preparing") ??
      null;
    const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const updateMotion = () => {
      cameraMotion.clear();
      engine?.set_reduced_motion(motion.matches);
    };
    updateMotion();
    motion.addEventListener("change", updateMotion, { signal });
    const moveCamera = (yaw: number, pitch: number, zoom: number) => {
      if (!engine || (yaw === 0 && pitch === 0 && zoom === 0)) {
        return;
      }
      if (motion.matches || useCameraSettingsStore.getState().smoothing === 0) {
        cameraMotion.clear();
        engine.camera_input(yaw, pitch, zoom, false);
      } else {
        cameraMotion.add(yaw, pitch, zoom);
      }
    };
    const resetMenuPointer = () => engine?.menu_pointer(0, 0);
    window.addEventListener(
      "pointermove",
      (event) => {
        if (menuJSON === "null" || sceneGameId !== null) return;
        const typing = document.activeElement?.matches(
          "input,textarea,select,[contenteditable='true']",
        );
        if (
          motion.matches ||
          event.pointerType !== "mouse" ||
          typing ||
          document.querySelector("[role='dialog'][aria-modal='true']")
        ) {
          resetMenuPointer();
          return;
        }
        engine?.menu_pointer(
          (event.clientX / window.innerWidth - 0.5) * 0.12,
          (event.clientY / window.innerHeight - 0.5) * 0.08,
        );
      },
      { signal },
    );
    window.addEventListener("blur", resetMenuPointer, { signal });
    window.addEventListener("focusin", resetMenuPointer, { signal });
    window.addEventListener("pointercancel", resetMenuPointer, { signal });

    readSceneStats = () => (engine ? JSON.parse(engine.scene_stats()) : null);
    readAssetStats = () => (engine ? JSON.parse(engine.asset_stats()) : {});
    const fail = (error: unknown) => {
      let nativePreparation = "unavailable";
      let nativeAssets = "unavailable";
      try {
        nativePreparation = engine?.preparation() ?? "unavailable";
        const assets = readAssetStats?.() ?? {};
        const structures = readSceneStats?.()?.structures;
        // Keep this below the recorder's per-string limit, as valid JSON.
        nativeAssets = JSON.stringify({
          activeCameras: assets.activeCameras,
          terrainRevision: assets.terrainRevision,
          terrainRequestedRevision: assets.terrainRequestedRevision,
          terrainTransitioning: assets.terrainTransitioning,
          foliageModels: assets.foliageModels,
          foliagePlants: assets.foliagePlants,
          structurePlots: structures?.plots,
          structurePending: structures?.pending,
          structureRetiringMeshes: structures?.retiringMeshes,
          structureReady: structures?.ready,
        });
      } catch {
        // A lost device may also make inspection fail. Preserve the original cause.
      }
      const details = { backend, nativePreparation, nativeAssets };
      console.error("Open Mars renderer failed", error, details);
      engineProfiler.failure(error, details);
      if (requiresAssets) {
        invalidateWorldAssets();
      }
      if (!started) {
        rejectReady(error);
      } else {
        cleanupAttempt();
        emit({
          phase: "error",
          backend,
          message: error instanceof Error ? error.message : String(error),
        });
      }
    };
    // Generation lives in a dedicated worker. Bounded packing tasks can also
    // run between animation frames; GPU publication stays in frame().
    const prepareCpu = () => {
      cpuScheduled = false;
      if (!engine || disposed || signal.aborted || document.hidden || (paused && ready)) {
        return;
      }
      try {
        terrainPreparation?.poll();
        if (engine.prepare_cpu()) {
          // MessageChannel yields without nested setTimeout's timer clamping.
          // Native packing still bounds the amount of work in each task.
          cpuScheduled = true;
          cpuChannel.port2.postMessage(++cpuToken);
        }
      } catch (error) {
        fail(error);
      }
    };
    cpuChannel.port1.addEventListener("message", ({ data }) => {
      if (cpuScheduled && data === cpuToken) prepareCpu();
    });
    cpuChannel.port1.start();
    terrainPreparation = new TerrainPreparation(engine, () => prepareCpu(), fail);
    const applyDelivery = (outcome: string) => {
      if (!engine) return;
      if (!["replace", "resync", "live"].includes(outcome)) {
        throw new Error("Unknown scene delivery outcome");
      }
      const demand: AssetDemand = JSON.parse(engine.asset_demand());
      const changed = demand.request !== assetRequest;
      if (outcome === "live" && !changed) return;
      const wasReady = ready;
      ready = false;
      preparation = "";
      emit({ phase: "preparing", backend, message: "Preparing scene" });
      if (!changed) {
        // Quiet snapshots keep the current stream and cannot extend its deadline.
        if (wasReady) {
          sceneTimer = window.setTimeout(
            () => fail(new Error("The world did not finish loading.")),
            60_000,
          );
        }
        return;
      }
      assetDownload?.abort();
      window.clearTimeout(sceneTimer);
      assetRequest = demand.request;
      requiresAssets = demand.assets.length > 0;
      downloadingAssets = false;
      sceneTimer = window.setTimeout(() => {
        if (assetRequest === demand.request) fail(new Error("The world did not finish loading."));
      }, 60_000);
      if (demand.assets.length === 0) return;
      const download = new AbortController();
      assetDownload = download;
      downloadingAssets = true;
      const endAssets = engineProfiler.span(
        "assets.download",
        { backend, assets: demand.assets.length },
        "loading.wait",
      );
      void loadWorldAssets(
        demand,
        download.signal,
        engine.max_texture_dimension_2d(),
        (message, loaded, total) => {
          if (!download.signal.aborted && assetRequest === demand.request) {
            emit({ phase: "loading", backend, message, loaded, total });
          }
        },
        (id, filename, bytes) => {
          download.signal.throwIfAborted();
          if (engine && !signal.aborted && assetRequest === demand.request) {
            const endInstall = engineProfiler.span(
              "asset.install",
              { asset: id, bytes: bytes.byteLength },
              "loading.cpu",
            );
            try {
              engine.install_asset(demand.request, id, filename, bytes);
            } finally {
              endInstall();
            }
          }
        },
      )
        .then(() => {
          endAssets();
          if (!download.signal.aborted && assetRequest === demand.request) {
            downloadingAssets = false;
            emit({ phase: "preparing", backend, message: "Preparing world assets" });
          }
        })
        .catch((error) => {
          endAssets();
          if (!download.signal.aborted && assetRequest === demand.request) fail(error);
        });
    };
    applyScene = (json, delivery) => {
      try {
        const endScene = engineProfiler.span("scene.apply", { delivery }, "scene.cpu");
        try {
          if (engine) {
            engine.set_profile_context(engineProfiler.scene(sceneSnapshot, appliedScene));
            const outcome = engine.set_scene(json, delivery === "snapshot" || !ready);
            appliedScene = sceneSnapshot;
            if (outcome !== "live") {
              cameraMotion.clear();
            }
            if (json !== "null") engine.set_focus(focusJSON);
            applyDelivery(outcome);
          }
        } finally {
          endScene();
        }
      } catch (error) {
        fail(error);
      }
    };
    applyMenu = (json) => {
      try {
        if (engine) applyDelivery(engine.set_menu(json));
      } catch (error) {
        fail(error);
      }
    };
    applyFocus = (json) => {
      try {
        cameraMotion.clear();
        if (engine && sceneGameId !== null) applyDelivery(engine.set_focus(json));
      } catch (error) {
        fail(error);
      }
    };
    applyEntry = (json) => {
      try {
        engine?.set_world_entry(json);
      } catch (error) {
        fail(error);
      }
    };
    // Establish the local backdrop before asking for the authoritative scene.
    // Only the final audience is downloaded, including on a backend retry.
    engine.set_menu(menuJSON);
    applyScene(sceneJSON, sceneDelivery);
    applyEntry(entryJSON);
    cameraCommand = (zoom, reset) => {
      if (engine && ready && !paused) {
        if (reset) {
          cameraMotion.clear();
          engine.camera_input(0, 0, 0, true);
        } else {
          moveCamera(0, 0, zoom * useCameraSettingsStore.getState().zoomSensitivity);
        }
      }
    };
    const pointers = new Map<
      number,
      { x: number; y: number; startX: number; startY: number; button: number; dragged: boolean }
    >();
    const dispatchPointer = (event: PointerEvent, hover: boolean, inspect = false) => {
      if (!engine || paused || !ready || document.hidden) {
        return;
      }
      const bounds = canvas.getBoundingClientRect();
      if (!bounds.width || !bounds.height) {
        return;
      }
      const x = (event.clientX - bounds.left) / bounds.width;
      const y = (event.clientY - bounds.top) / bounds.height;
      try {
        const result: SceneEvent | null = JSON.parse(
          hover
            ? engine.hover(x, y, event.clientX, event.clientY)
            : engine.pointer(x, y, event.clientX, event.clientY, inspect),
        );
        if (result) {
          if (hover) {
            canvas.style.cursor = result.type === "body-hovered" ? "pointer" : "default";
          }
          onEvent(result);
        }
      } catch (error) {
        fail(error);
      }
    };
    const clearHover = () => {
      canvas.style.cursor = "default";
      if (engine && ready) {
        const result: SceneEvent | null = JSON.parse(engine.hover(-1, -1, 0, 0));
        if (result) {
          onEvent(result);
        }
      }
    };
    canvas.addEventListener(
      "pointerdown",
      (event) => {
        if (paused || !ready || (event.button !== 0 && event.button !== 2)) {
          return;
        }
        canvas.setPointerCapture(event.pointerId);
        pointers.set(event.pointerId, {
          x: event.clientX,
          y: event.clientY,
          startX: event.clientX,
          startY: event.clientY,
          button: event.button,
          dragged: false,
        });
        if (pointers.size > 1) {
          for (const pointer of pointers.values()) {
            pointer.dragged = true;
          }
        }
      },
      { signal },
    );
    canvas.addEventListener(
      "pointermove",
      (event) => {
        const previous = pointers.get(event.pointerId);
        if (!previous) {
          if (event.pointerType === "mouse") {
            dispatchPointer(event, true);
          }
          return;
        }
        if (!engine || !ready || paused || document.hidden) {
          pointers.clear();
          return;
        }
        const other = [...pointers.entries()].find(([id]) => id !== event.pointerId)?.[1];
        if (other) {
          const before = Math.hypot(previous.x - other.x, previous.y - other.y);
          const after = Math.hypot(event.clientX - other.x, event.clientY - other.y);
          if (before > 1 && after > 1) {
            moveCamera(
              0,
              0,
              Math.log(before / after) * useCameraSettingsStore.getState().zoomSensitivity,
            );
          }
        } else {
          if (Math.hypot(event.clientX - previous.startX, event.clientY - previous.startY) > 8) {
            previous.dragged = true;
          }
          if (previous.dragged) {
            const sensitivity = 0.006 * useCameraSettingsStore.getState().orbitSensitivity;
            moveCamera(
              -(event.clientX - previous.x) * sensitivity,
              (event.clientY - previous.y) * sensitivity,
              0,
            );
          }
        }
        if (previous.dragged) {
          clearHover();
        }
        previous.x = event.clientX;
        previous.y = event.clientY;
      },
      { signal },
    );
    canvas.addEventListener(
      "pointerup",
      (event) => {
        const previous = pointers.get(event.pointerId);
        pointers.delete(event.pointerId);
        if (canvas.hasPointerCapture(event.pointerId)) {
          canvas.releasePointerCapture(event.pointerId);
        }
        if (
          previous &&
          !previous.dragged &&
          Math.hypot(event.clientX - previous.startX, event.clientY - previous.startY) <= 8
        ) {
          dispatchPointer(event, false, previous.button === 2);
        }
      },
      { signal },
    );
    const cancelPointer = (event: PointerEvent) => {
      if (pointers.delete(event.pointerId)) {
        cameraMotion.clear();
      }
    };
    canvas.addEventListener("pointercancel", cancelPointer, { signal });
    canvas.addEventListener("lostpointercapture", cancelPointer, { signal });
    canvas.addEventListener("pointerleave", clearHover, { signal });
    canvas.addEventListener(
      "wheel",
      (event) => {
        if (!engine || paused || !ready || sceneJSON === "null") {
          return;
        }
        event.preventDefault();
        moveCamera(
          0,
          0,
          wheelZoom(
            event.deltaY,
            event.deltaMode,
            canvas.clientHeight,
            useCameraSettingsStore.getState(),
          ),
        );
        clearHover();
      },
      { signal, passive: false },
    );
    window.addEventListener(
      "keydown",
      (event) => {
        if (
          !engine ||
          paused ||
          !ready ||
          sceneJSON === "null" ||
          event.ctrlKey ||
          event.metaKey ||
          event.altKey
        ) {
          return;
        }
        const target = event.target;
        if (
          target instanceof Element &&
          target.closest("input, textarea, select, button, [contenteditable='true']")
        ) {
          return;
        }
        if (event.key === "Home") {
          event.preventDefault();
          cameraCommand?.(0, true);
        } else if (["+", "=", "-", "_"].includes(event.key)) {
          event.preventDefault();
          cameraCommand?.(event.key === "+" || event.key === "=" ? -0.2 : 0.2, false);
        }
      },
      { signal },
    );
    canvas.addEventListener("contextmenu", (event) => event.preventDefault(), { signal });
    let lastFrameTime: number | undefined;
    const limiter = new FrameLimiter();
    resetFrameClock = () => {
      lastFrameTime = undefined;
      limiter.reset();
      engineProfiler.resetFrameGap();
      cameraMotion.clear();
    };
    document.addEventListener(
      "visibilitychange",
      () => {
        resetFrameClock?.();
        if (document.hidden) {
          cancelCpu();
        }
      },
      { signal },
    );
    const draw = (timestamp: number) => {
      if (disposed || signal.aborted) return;
      if (ready && !limiter.due(timestamp, graphics.frameLimit)) {
        frame = requestAnimationFrame(draw);
        return;
      }
      const profiling = engineProfiler.active;
      const profileStart = profiling ? performance.now() : 0;
      const endDraw = engineProfiler.span("browser.frame", undefined, "browser.cpu");
      let activeFrame = false;
      try {
        if (engine && !document.hidden && (!paused || !ready)) {
          const bounds = canvas.getBoundingClientRect();
          if (bounds.width > 0 && bounds.height > 0) {
            activeFrame = true;
            if (profiling) engineProfiler.animationFrame(timestamp, true);
            const limit = engine.max_texture_dimension_2d();
            const {
              width,
              height,
              ratio: dpr,
            } = renderDimensions(
              bounds.width,
              bounds.height,
              window.devicePixelRatio,
              limit,
              graphics.renderScale,
            );
            if (canvas.width !== width || canvas.height !== height) {
              engineProfiler.mark("canvas.resize", { width, height, dpr });
              engineProfiler.metadata({
                renderWidth: width,
                renderHeight: height,
                renderScale: dpr,
              });
              if (canvas.width !== width) canvas.width = width;
              if (canvas.height !== height) canvas.height = height;
            }
            const insets = readHudInsets();
            engine.viewport(
              new Float32Array([
                bounds.width,
                bounds.height,
                insets.top,
                insets.bottom,
                insets.left,
                insets.right,
              ]),
              width,
              height,
              getLayoutMode().isCompact,
            );
            const delta =
              paused || lastFrameTime === undefined
                ? 0
                : Math.min((timestamp - lastFrameTime) / 1000, 0.1);
            lastFrameTime = timestamp;
            const cameraStep = cameraMotion.step(
              delta,
              useCameraSettingsStore.getState().smoothing,
            );
            if (cameraStep.some((value) => value !== 0)) {
              engine.camera_input(...cameraStep, false);
            }
            const beforeFrames = meter?.active ? meter.beforeFrame() : 0;
            const beforeFrameTime = meter?.active ? performance.now() : 0;
            const rendered = engine.frame(width, height, delta);
            if (meter?.active) meter.afterFrame(beforeFrames, performance.now() - beforeFrameTime);
            if (!cpuScheduled) {
              prepareCpu();
            }
            if (!engine) {
              endDraw();
              if (profiling && activeFrame) {
                engineProfiler.cpuFrame(performance.now() - profileStart);
                engineProfiler.afterFrame();
              }
              return;
            }
            const nextOverlay = engine.overlay_frame();
            if (nextOverlay !== overlayJSON) {
              overlayJSON = nextOverlay;
              const endOverlay = engineProfiler.span("overlay.deliver", undefined, "bridge.cpu");
              try {
                onOverlay(JSON.parse(nextOverlay) as OverlayFrame);
              } finally {
                endOverlay();
              }
            }
            for (const event of JSON.parse(engine.take_sounds()) as SceneSound[]) {
              onSound(event);
            }
            if (!ready && !downloadingAssets) {
              const stage = engine.preparation();
              if (stage !== preparation) {
                preparation = stage;
                emit({ phase: "preparing", backend, message: stage });
              }
            }
            if (rendered && !ready && !downloadingAssets) {
              ready = true;
              started = true;
              window.clearTimeout(timer);
              window.clearTimeout(sceneTimer);
              emit({
                phase: "ready",
                backend,
                message: "Ready",
                scene: appliedScene
                  ? { gameId: appliedScene.gameId, mapId: appliedScene.mapId }
                  : null,
                capabilities: {
                  computeShaders: engine.supports_compute(),
                  storageBuffers: engine.supports_storage_buffers(),
                  maxTextureDimension2D: engine.max_texture_dimension_2d(),
                },
              });
              resolveReady();
            }
          } else {
            lastFrameTime = undefined;
            engineProfiler.resetFrameGap();
          }
        } else {
          lastFrameTime = undefined;
          engineProfiler.resetFrameGap();
        }
        frame = requestAnimationFrame(draw);
      } catch (error) {
        endDraw();
        if (profiling && activeFrame) {
          engineProfiler.cpuFrame(performance.now() - profileStart);
          engineProfiler.afterFrame();
        }
        fail(error);
        return;
      }
      endDraw();
      if (profiling && activeFrame) {
        engineProfiler.cpuFrame(performance.now() - profileStart);
        engineProfiler.afterFrame();
      }
    };
    frame = requestAnimationFrame(draw);
    await firstFrame;
  };

  void (async () => {
    const backends: EngineBackend[] = preference === "auto" ? ["webgpu", "webgl2"] : [preference];
    const failures: string[] = [];
    for (const backend of backends) {
      try {
        await startBackend(backend);
        return;
      } catch (error) {
        if (!disposed) {
          engineProfiler.failure(error, { backend });
        }
        cleanupAttempt();
        if (disposed) return;
        failures.push(`${backend}: ${error instanceof Error ? error.message : String(error)}`);
      }
    }
    emit({ phase: "error", message: failures.join(" ") });
  })();

  return {
    setPerformanceActive(active: boolean) {
      performanceActive = active;
      activatePerformance?.(active);
    },
    performance() {
      return readPerformance?.() ?? null;
    },
    camera(zoom: number, reset = false) {
      cameraCommand?.(zoom, reset);
    },
    sceneStats() {
      return readSceneStats?.() ?? null;
    },
    assetStats() {
      return readAssetStats?.() ?? null;
    },
    setWorldEntry(entry: WorldEntry) {
      if (disposed) {
        return;
      }
      const json = JSON.stringify(entry);
      if (json === entryJSON) {
        return;
      }
      entryJSON = json;
      applyEntry?.(json);
    },
    setMenu(placement: MenuPlacement | null) {
      if (disposed) return;
      const json = JSON.stringify(placement);
      if (json === menuJSON) return;
      menuJSON = json;
      applyMenu?.(json);
    },
    setFocus(focus: FocusTarget) {
      if (disposed) return;
      const json = JSON.stringify(focus);
      if (json === focusJSON) return;
      focusJSON = json;
      applyFocus?.(json);
    },
    setScene(scene: SceneSnapshot | null, delivery: "snapshot" | "live" = "live") {
      if (disposed) {
        return;
      }
      if (scene?.gameId !== sceneGameId || scene?.mapId !== sceneMapId)
        focusJSON = JSON.stringify({ type: "body", body: "mars" } satisfies FocusTarget);
      sceneGameId = scene?.gameId ?? null;
      sceneMapId = scene?.mapId ?? null;
      const endSnapshot = engineProfiler.span("scene.serialize", undefined, "bridge.cpu");
      let json: string;
      try {
        json = JSON.stringify(scene);
      } finally {
        endSnapshot();
      }
      if (json === sceneJSON && delivery === "live") {
        return;
      }
      sceneJSON = json;
      sceneSnapshot = scene;
      sceneDelivery = delivery;
      applyScene?.(json, delivery);
    },
    setGraphics(settings: GraphicsSettings) {
      if (residentGraphicsKey(graphics) !== residentGraphicsKey(settings)) {
        throw new Error("Resident quality changes require a fresh renderer");
      }
      graphics = settings;
      applyGraphics?.(settings);
    },
    setPaused(value: boolean) {
      if (paused !== value) {
        clearCameraMotion?.();
        paused = value;
        resetFrameClock?.();
      }
    },
    dispose() {
      disposed = true;
      cleanupAttempt();
      host.replaceChildren();
    },
  };
}
