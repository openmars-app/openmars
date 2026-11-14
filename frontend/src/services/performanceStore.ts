import { useEngineStore } from "../stores/engineStore";
import type { EnginePerformance, PerformanceSource } from "../engine/performance";

export interface PerformanceSnapshot {
  timestamp: number;
  fps: number;
  frameTimeMs: number;
  jsHeapUsedMB: number | null;
  jsHeapTotalMB: number | null;
  engine: EnginePerformance | null;
}

const SAMPLE_INTERVAL_MS = 250;
const MAX_SAMPLES = 120;

class PerformanceStoreService {
  private buffer: PerformanceSnapshot[] = [];
  private listeners = new Set<(snapshots: PerformanceSnapshot[]) => void>();
  private rafId: number | null = null;
  private frameCount = 0;
  private lastSampleTime = 0;
  private refCount = 0;
  private source: PerformanceSource | null = null;
  private unsubscribeEngine: (() => void) | undefined;
  private resetVisibility = () => {
    this.frameCount = 0;
    this.lastSampleTime = performance.now();
    this.source?.setActive(!document.hidden);
  };

  start() {
    this.refCount++;
    if (this.rafId !== null) return;
    this.buffer = [];
    this.frameCount = 0;
    this.lastSampleTime = performance.now();
    const synchronize = () => {
      const next = useEngineStore.getState().performance;
      if (next === this.source) return;
      this.source?.setActive(false);
      this.source = next;
      this.source?.setActive(!document.hidden);
    };
    this.unsubscribeEngine = useEngineStore.subscribe(synchronize);
    document.addEventListener("visibilitychange", this.resetVisibility);
    synchronize();
    this.rafId = requestAnimationFrame(this.tick);
  }

  stop() {
    this.refCount = Math.max(0, this.refCount - 1);
    if (this.refCount > 0) return;
    if (this.rafId !== null) cancelAnimationFrame(this.rafId);
    this.rafId = null;
    this.unsubscribeEngine?.();
    this.unsubscribeEngine = undefined;
    document.removeEventListener("visibilitychange", this.resetVisibility);
    this.source?.setActive(false);
    this.source = null;
  }

  subscribe(listener: (snapshots: PerformanceSnapshot[]) => void): () => void {
    this.listeners.add(listener);
    listener([...this.buffer]);
    return () => {
      this.listeners.delete(listener);
    };
  }

  getSnapshots(): PerformanceSnapshot[] {
    return [...this.buffer];
  }

  private tick = () => {
    if (this.refCount === 0) return;
    const now = performance.now();
    if (document.hidden) {
      this.frameCount = 0;
      this.lastSampleTime = now;
      this.rafId = requestAnimationFrame(this.tick);
      return;
    }
    this.frameCount++;
    const elapsed = now - this.lastSampleTime;
    if (elapsed >= SAMPLE_INTERVAL_MS) {
      const memory = (
        performance as Performance & {
          memory?: { usedJSHeapSize: number; totalJSHeapSize: number };
        }
      ).memory;
      let engine: EnginePerformance | null = null;
      try {
        engine = this.source?.sample() ?? null;
      } catch {
        /* Device disposal can race one sample. */
      }
      this.buffer.push({
        timestamp: now,
        fps: (1000 * this.frameCount) / elapsed,
        frameTimeMs: elapsed / this.frameCount,
        jsHeapUsedMB: memory ? memory.usedJSHeapSize / 1048576 : null,
        jsHeapTotalMB: memory ? memory.totalJSHeapSize / 1048576 : null,
        engine,
      });
      if (this.buffer.length > MAX_SAMPLES) this.buffer.shift();
      this.frameCount = 0;
      this.lastSampleTime = now;
      const copy = [...this.buffer];
      this.listeners.forEach((listener) => listener(copy));
    }
    if (this.refCount > 0) this.rafId = requestAnimationFrame(this.tick);
  };
}

export const performanceStore = new PerformanceStoreService();

type TraceDetail = Record<string, unknown>;
const TRACE_PREFIX = "openmars:cold-start";
const NOOP = () => {};

function traceEnabled() {
  try {
    return localStorage.getItem("openmars.perfTrace") === "1";
  } catch {
    return false;
  }
}

// Opt in before reloading: localStorage.setItem("openmars.perfTrace", "1").
class ColdStartTrace {
  readonly enabled = traceEnabled();
  private sequence = 0;
  private runs = new Map<string, number>();
  private current: {
    id: string;
    start: number;
    rows: { stage: string; offsetMs: number; durationMs: number; detail: TraceDetail }[];
    maxFrameGapMs: number;
    frameCount: number;
    resources: PerformanceResourceTiming[];
  } | null = null;
  private frame = 0;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private observer: PerformanceObserver | undefined;
  private entries: string[] = [];

  get active() {
    return this.current !== null;
  }

  begin(name: string, detail: TraceDetail = {}) {
    if (!this.enabled) {
      return;
    }
    this.finish();
    for (const entry of this.entries) {
      performance.clearMarks(entry);
      performance.clearMeasures(entry);
    }
    this.entries = [];
    const run = (this.runs.get(name) ?? 0) + 1;
    this.runs.set(name, run);
    this.current = {
      id: `${TRACE_PREFIX}:${name}:${++this.sequence}`,
      start: performance.now(),
      rows: [],
      maxFrameGapMs: 0,
      frameCount: 0,
      resources: [],
    };
    this.mark("requested", { ...detail, run, visibility: document.visibilityState });
    let previous = this.current.start;
    const tick = () => {
      const trace = this.current;
      if (!trace) {
        return;
      }
      const now = performance.now();
      const gap = now - previous;
      trace.maxFrameGapMs = Math.max(trace.maxFrameGapMs, gap);
      trace.frameCount++;
      if (trace.frameCount === 1 || gap >= 25) {
        this.record("browser-frame-gap", previous, gap, {
          frame: trace.frameCount,
          visibility: document.visibilityState,
        });
      }
      previous = now;
      this.frame = requestAnimationFrame(tick);
    };
    this.frame = requestAnimationFrame(tick);
    if (typeof PerformanceObserver !== "undefined") {
      const entryTypes = ["longtask", "resource"].filter((type) =>
        PerformanceObserver.supportedEntryTypes.includes(type),
      );
      if (entryTypes.length) {
        this.observer = new PerformanceObserver((list) => this.recordEntries(list.getEntries()));
        this.observer.observe({ entryTypes });
      }
    }
    this.timer = setTimeout(() => this.finish(), 4000);
  }

  mark(stage: string, detail: TraceDetail = {}) {
    if (!this.current) {
      return;
    }
    this.record(stage, performance.now(), 0, detail);
  }

  span(stage: string, detail: TraceDetail = {}) {
    const trace = this.current;
    if (!trace) {
      return NOOP;
    }
    const start = performance.now();
    return () => {
      if (this.current === trace) {
        this.record(stage, start, performance.now() - start, detail);
      }
    };
  }

  record(stage: string, start: number, duration: number, detail: TraceDetail = {}) {
    const trace = this.current;
    if (!trace || trace.rows.length >= 250) {
      return;
    }
    const name = `${trace.id}:${stage}:${trace.rows.length}`;
    if (duration === 0) {
      performance.mark(name, { startTime: start, detail });
    } else {
      performance.measure(name, { start, duration, detail });
    }
    this.entries.push(name);
    trace.rows.push({
      stage,
      offsetMs: Math.round((start - trace.start) * 100) / 100,
      durationMs: Math.round(duration * 100) / 100,
      detail,
    });
  }

  reactRender = (
    id: string,
    phase: string,
    actualDuration: number,
    _baseDuration: number,
    startTime: number,
    commitTime: number,
  ) => {
    this.record(`react:${id}`, startTime, actualDuration, { phase, commitTime });
  };

  private recordEntries(entries: PerformanceEntry[]) {
    for (const entry of entries) {
      if (this.current && entry.startTime + entry.duration >= this.current.start) {
        if (entry.entryType === "longtask") {
          this.record("long-task", entry.startTime, entry.duration);
        } else if (this.current.resources.length < 40) {
          this.current.resources.push(entry as PerformanceResourceTiming);
        }
      }
    }
  }

  private finish() {
    const trace = this.current;
    if (!trace) {
      return;
    }
    clearTimeout(this.timer);
    cancelAnimationFrame(this.frame);
    if (this.observer) {
      this.recordEntries(this.observer.takeRecords());
      this.observer.disconnect();
      this.observer = undefined;
    }
    const resources = trace.resources.map((entry) => ({
      url: entry.name,
      startMs: Math.round(entry.startTime - trace.start),
      durationMs: Math.round(entry.duration),
      transferBytes: entry.transferSize,
    }));
    this.current = null;
    // Log after the capture so DevTools console rendering cannot cause the measured stall.
    console.groupCollapsed(`[${trace.id}] max frame gap ${trace.maxFrameGapMs.toFixed(1)}ms`);
    console.table(trace.rows);
    console.log("Capture", {
      durationMs: performance.now() - trace.start,
      frames: trace.frameCount,
      maxFrameGapMs: trace.maxFrameGapMs,
      rowLimitReached: trace.rows.length === 250,
      longTasksSupported:
        typeof PerformanceObserver !== "undefined" &&
        PerformanceObserver.supportedEntryTypes.includes("longtask"),
    });
    if (resources.length) {
      console.table(resources);
    }
    console.log(
      "CPU submission timings are not GPU execution timings. Frame gaps include browser rendering; use a Performance recording for paint/raster/GPU attribution.",
    );
    console.groupEnd();
  }
}

export const coldStartTrace = new ColdStartTrace();
