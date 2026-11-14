import {
  ProfileRecorder,
  type NativeTraceEvent,
  type ProfileSummary,
  type TraceArgs,
} from "./profileRecorder";
import { observeGpu } from "./profileGpu";
import type { SceneSnapshot } from "./scene.generated";

interface NativeSource {
  setEnabled(active: boolean, now: number): void;
  setContext(context: number): void;
  drain(maxEvents: number): { events: NativeTraceEvent[]; dropped: number; remaining: number };
}
export interface ProfilerSnapshot {
  phase: "idle" | "recording" | "ready";
  armed: boolean;
  elapsedMs: number;
  durationMs: number;
  eventCount: number;
  dropped: number;
  error: string | null;
  summary: ProfileSummary;
}
const RELOAD_KEY = "openmars.profile-next-reload";
const HARVEST_EVENTS = 128;

export class EngineProfiler {
  private readonly recorder = new ProfileRecorder();
  private readonly listeners = new Set<(snapshot: ProfilerSnapshot) => void>();
  private readonly sources = new Set<NativeSource>();
  private readonly devices = new Map<
    unknown,
    { cleanup: (() => void) | null; references: number }
  >();
  private details: TraceArgs = {};
  private observers: PerformanceObserver[] = [];
  private interval: ReturnType<typeof setInterval> | undefined;
  private deadline: ReturnType<typeof setTimeout> | undefined;
  private adapterCleanup: (() => void) | undefined;
  private armed = false;
  private error: string | null = null;
  private context = 0;
  private pending = new Map<number, number>();
  private landscapeContexts = new Map<number, number>();
  private stopped = false;
  private workerEpoch = 0;
  private lastFrameHarvest = -Infinity;

  get active() {
    return this.recorder.active;
  }
  snapshot(): ProfilerSnapshot {
    return {
      phase: this.active ? "recording" : this.recorder.count ? "ready" : "idle",
      armed: this.armed,
      elapsedMs: this.recorder.elapsedMs,
      durationMs: this.recorder.durationMs,
      eventCount: this.recorder.count,
      dropped: this.recorder.dropped,
      error: this.error,
      summary: this.recorder.summary(),
    };
  }
  subscribe(listener: (snapshot: ProfilerSnapshot) => void) {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }
  private publish() {
    const state = this.snapshot();
    for (const listener of this.listeners) listener(state);
  }

  start(mode: "manual" | "reload" = "manual") {
    if (this.active) {
      this.stop();
    }
    this.error = null;
    this.context = 0;
    this.workerEpoch++;
    this.pending.clear();
    this.landscapeContexts.clear();
    this.lastFrameHarvest = -Infinity;
    this.recorder.start(mode, mode === "reload" ? 30_000 : 10_000);
    this.recorder.metadata({
      backend: "pending",
      userAgent: typeof navigator === "undefined" ? "unknown" : navigator.userAgent,
      memoryCoverage: "Only allocations observed while recording; no driver VRAM measurement",
      clock: "performance.now, microseconds",
    });
    for (const source of this.sources) this.enable(source, true);
    this.recorder.metadata(this.details);
    for (const [device, entry] of this.devices) entry.cleanup = observeGpu(device, this.recorder);
    this.observeAdapters();
    this.observeBrowser();
    this.interval = setInterval(() => {
      // No frame work may occur while loading/paused/hidden. Keep that fallback
      // bounded too; normal capture harvesting happens after each active frame.
      if (performance.now() - this.lastFrameHarvest >= 250) this.flush();
      this.publish();
    }, 250);
    this.deadline = setTimeout(() => this.stop("duration"), this.recorder.durationMs);
    this.publish();
  }
  stop(reason = "manual") {
    if (!this.active || this.stopped) {
      return;
    }
    this.stopped = true;
    for (const source of this.sources) this.enable(source, false);
    // Drain once more after disabling, including spans that ended at stop time.
    this.flush(true);
    for (const context of this.pending.keys()) this.finishContext(context, "capture ended");
    this.recorder.stop(reason);
    clearInterval(this.interval);
    clearTimeout(this.deadline);
    this.interval = undefined;
    this.deadline = undefined;
    for (const observer of this.observers) observer.disconnect();
    this.observers = [];
    this.adapterCleanup?.();
    this.adapterCleanup = undefined;
    for (const entry of this.devices.values()) {
      entry.cleanup?.();
      entry.cleanup = null;
    }
    this.stopped = false;
    this.publish();
  }
  clear() {
    this.stop();
    this.recorder.clear();
    this.error = null;
    this.publish();
  }
  toggleReload() {
    try {
      if (this.armed) {
        sessionStorage.removeItem(RELOAD_KEY);
      } else {
        sessionStorage.setItem(RELOAD_KEY, "1");
      }
      this.armed = !this.armed;
      this.error = null;
    } catch {
      this.error = "Reload capture is unavailable in this browser.";
    }
    this.publish();
  }
  consumeReload() {
    try {
      if (sessionStorage.getItem(RELOAD_KEY) !== "1") {
        return;
      }
      sessionStorage.removeItem(RELOAD_KEY);
      this.armed = false;
      this.start("reload");
    } catch {
      /* Normal rendering does not depend on browser storage. */
    }
  }
  download() {
    if (this.active) {
      this.stop();
    }
    if (!this.recorder.count) {
      return;
    }
    try {
      const blob = new Blob([JSON.stringify(this.recorder.export())], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `openmars-trace-${new Date().toISOString().replaceAll(":", "-")}.json`;
      document.body.append(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch {
      this.error = "Could not download the trace.";
      this.publish();
    }
  }
  export() {
    this.flush();
    return this.recorder.export();
  }
  workerCapture() {
    return this.active ? this.workerEpoch : -1;
  }
  worker(capture: number, started: number, duration: number, args: TraceArgs) {
    if (!this.active || capture !== this.workerEpoch) return;
    this.recorder.complete({
      name: "terrain.worker.generate",
      cat: "worker.cpu",
      ts: (started - performance.timeOrigin) * 1000,
      dur: duration * 1000,
      args,
    });
  }
  span(name: string, args?: TraceArgs, category?: string) {
    return this.recorder.span(name, args, category);
  }
  mark(name: string, args?: TraceArgs) {
    this.recorder.mark(name, args);
  }
  cpuFrame(durationMs: number) {
    this.recorder.cpuFrame(durationMs);
  }
  animationFrame(timestamp: number, active: boolean) {
    this.recorder.animationFrame(timestamp, active);
  }
  resetFrameGap() {
    this.recorder.resetFrameGap();
  }
  afterFrame() {
    if (!this.active) return;
    this.lastFrameHarvest = performance.now();
    this.flush();
  }
  metadata(args: TraceArgs) {
    Object.assign(this.details, args);
    this.recorder.metadata(args);
  }
  stateReceived() {
    this.mark("state.received");
  }
  failure(error: unknown, args?: TraceArgs) {
    if (!this.active) {
      return;
    }
    this.mark("renderer.failure", {
      ...args,
      message: error instanceof Error ? error.message : String(error),
    });
    this.stop("renderer failure");
  }
  scene(next: SceneSnapshot | null, previous: SceneSnapshot | null): number {
    if (!this.active) {
      return 0;
    }
    const changed =
      next?.tiles.length !== previous?.tiles.length ||
      next?.gameId !== previous?.gameId ||
      next?.mapId !== previous?.mapId ||
      next?.temperature !== previous?.temperature ||
      next?.oxygen !== previous?.oxygen ||
      next?.oceans !== previous?.oceans ||
      next?.venus !== previous?.venus ||
      next?.tiles.some(
        (tile, index) =>
          tile.occupant !== previous?.tiles[index]?.occupant ||
          tile.visual?.seed !== previous?.tiles[index]?.visual?.seed,
      );
    if (!changed) {
      return this.context;
    }
    for (const old of this.pending.keys()) this.finishContext(old, "superseded");
    const context = ++this.context;
    this.pending.set(context, performance.now());
    this.recorder.flow("Scene update", context, false, {
      context,
      tiles: next?.tiles.filter((tile) => tile.occupant !== null).length ?? 0,
    });
    this.mark("scene.applied", { context, kind: next === null ? "menu" : "world" });
    return context;
  }
  private finishContext(context: number, outcome: string, ended = performance.now()) {
    const started = this.pending.get(context);
    if (started === undefined) {
      return;
    }
    this.recorder.complete({
      name: "Scene update context",
      cat: "scene.wait",
      ts: started * 1000,
      dur: Math.max(0, ended - started) * 1000,
      args: { context, outcome },
    });
    this.recorder.flow("Scene update", context, true, { context, outcome }, ended);
    this.pending.delete(context);
  }
  private enable(source: NativeSource, active: boolean) {
    try {
      source.setEnabled(active, performance.now());
      source.setContext(this.context);
    } catch (error) {
      this.mark("profiler.native.unavailable", { message: String(error) });
    }
  }
  native(source: NativeSource) {
    this.sources.add(source);
    this.enable(source, this.active);
    return () => {
      this.enable(source, false);
      this.flushSource(source, true);
      this.sources.delete(source);
    };
  }
  private flushSource(source: NativeSource, complete = false) {
    if (!this.active) {
      return;
    }
    const endDrain = this.recorder.span("profiler.drain", undefined, "profiler.cpu");
    try {
      let remaining: number;
      do {
        const data = source.drain(HARVEST_EVENTS);
        remaining = complete ? data.remaining : 0;
        this.recorder.lose(data.dropped);
        for (const event of data.events) {
          this.recorder.complete(event);
          const context = Number(event.args?.context);
          const revision = Number(event.args?.landscapeRevision);
          if (event.name === "terrain.build.begin" && Number.isFinite(revision)) {
            this.landscapeContexts.set(revision, context);
          }
          if (
            [
              "terrain.publish",
              "structures.publish",
              "vegetation.publish",
              "vegetation.animation.frame",
            ].includes(event.name)
          ) {
            const owner = this.landscapeContexts.get(revision) ?? context;
            const started = this.pending.get(owner);
            if (started !== undefined) {
              this.recorder.complete({
                name: `Scene update → ${event.name}`,
                cat: "scene.wait",
                ts: started * 1000,
                dur: Math.max(0, event.ts - started * 1000),
                args: { context: owner, landscapeRevision: revision },
              });
            }
          }
          if (event.name === "terrain.publish") {
            const owner = this.landscapeContexts.get(revision) ?? context;
            // Terrain publication is one stage, not proof of the first visible frame.
            this.mark("scene.terrainPublished", { context: owner, landscapeRevision: revision });
          }
          if (event.name === "scene.ready") {
            this.finishContext(context, "queue callback ready", event.ts / 1000);
          }
        }
      } while (remaining > 0);
    } catch (error) {
      this.mark("profiler.drain.failed", { message: String(error) });
    } finally {
      endDrain();
    }
  }
  flush(complete = false) {
    if (this.active) {
      for (const source of this.sources) {
        this.flushSource(source, complete);
      }
    }
  }
  device(device: unknown) {
    if (!device) {
      return () => {};
    }
    let entry = this.devices.get(device);
    if (!entry) {
      entry = { cleanup: this.active ? observeGpu(device, this.recorder) : null, references: 0 };
      this.devices.set(device, entry);
    }
    entry.references++;
    let released = false;
    return () => {
      if (released) {
        return;
      }
      released = true;
      if (--entry.references === 0) {
        entry.cleanup?.();
        this.devices.delete(device);
      }
    };
  }
  private observeAdapters() {
    if (typeof GPUAdapter === "undefined") {
      return;
    }
    const prototype = GPUAdapter.prototype;
    const original = prototype.requestDevice;
    const profiler = this;
    const created: (() => void)[] = [];
    let observing = true;
    const wrapped: typeof original = function (this: GPUAdapter, ...args) {
      const promise = Reflect.apply(original, this, args) as Promise<GPUDevice>;
      void promise.then(
        (device) => {
          if (observing && profiler.active) {
            const info = this.info;
            profiler.metadata({
              adapterVendor: info?.vendor ?? "unknown",
              adapterArchitecture: info?.architecture ?? "unknown",
              adapterDescription: info?.description ?? "unknown",
            });
            created.push(profiler.device(device));
          }
        },
        () => {},
      );
      return promise;
    };
    try {
      prototype.requestDevice = wrapped;
      this.adapterCleanup = () => {
        observing = false;
        if (prototype.requestDevice === wrapped) {
          prototype.requestDevice = original;
        }
        for (const cleanup of created) cleanup();
      };
    } catch {
      this.mark("profiler.adapter.unavailable");
    }
  }
  private observeBrowser() {
    if (typeof PerformanceObserver === "undefined") {
      return;
    }
    for (const type of ["resource", "navigation", "longtask"]) {
      if (!PerformanceObserver.supportedEntryTypes.includes(type)) {
        continue;
      }
      const observer = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (!this.active) {
            continue;
          }
          if (this.recorder.mode !== "reload" && entry.startTime < this.recorder.startedAt) {
            continue;
          }
          let name = entry.name;
          if (type === "resource" || type === "navigation") {
            try {
              const url = new URL(name);
              name = url.pathname;
            } catch {
              name = type;
            }
          }
          this.recorder.complete({
            name,
            cat: type === "longtask" ? "browser.cpu" : "network.wait",
            ts: entry.startTime * 1000,
            dur: entry.duration * 1000,
            args: { type },
          });
        }
      });
      try {
        observer.observe({ type, buffered: true });
        this.observers.push(observer);
      } catch {
        observer.disconnect();
      }
    }
  }
}

export const engineProfiler = new EngineProfiler();
if (typeof window !== "undefined") {
  engineProfiler.consumeReload();
}
