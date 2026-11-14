export type TraceArgs = Record<string, string | number | boolean | null>;
export interface NativeTraceEvent {
  name: string;
  ts: number;
  dur: number;
  cat?: string;
  args?: TraceArgs;
}
export interface TraceEvent {
  name: string;
  cat: string;
  ph: "X" | "i" | "C" | "M" | "s" | "f";
  ts: number;
  pid: number;
  tid: number;
  dur?: number;
  s?: "t";
  id?: number;
  args?: TraceArgs;
}
export interface ProfileSummary {
  worstCpuFrameMs: number | null;
  longestFrameGapMs: number | null;
  hottestStage: string | null;
  hottestStageMs: number | null;
}

const clean = (value: string) => value.slice(0, 512).replace(/\?[^\s)]*/g, "?…");
function cleanArgs(args?: TraceArgs): TraceArgs | undefined {
  if (!args) {
    return undefined;
  }
  const result: TraceArgs = {};
  for (const [key, value] of Object.entries(args).slice(0, 16)) {
    if (typeof value === "number" && Number.isFinite(value)) {
      result[key.slice(0, 64)] = value;
    } else if (typeof value === "string") {
      result[key.slice(0, 64)] = clean(value);
    } else if (typeof value === "boolean" || value === null) {
      result[key.slice(0, 64)] = value;
    }
  }
  return result;
}

/** Bounded phase trace. It records instrumented work, not sampled call stacks or VRAM. */
export class ProfileRecorder {
  active = false;
  startedAt = 0;
  stoppedAt = 0;
  durationMs = 10_000;
  mode: "manual" | "reload" = "manual";
  dropped = 0;
  private generation = 0;
  private events: TraceEvent[] = [];
  private bytes = 0;
  private worstCpuFrame: number | null = null;
  private longestFrameGap: number | null = null;
  private previousAnimationFrame: number | null = null;
  private stages: NativeTraceEvent[] = [];
  private totals = new Map<string, { total: number; count: number }>();
  private details: TraceArgs = {};
  constructor(
    private readonly now = () => performance.now(),
    private readonly maxEvents = 100_000,
    private readonly maxBytes = 24 * 1024 * 1024,
  ) {}

  start(mode: "manual" | "reload" = "manual", durationMs = 10_000) {
    this.generation++;
    this.active = true;
    this.startedAt = mode === "reload" ? 0 : this.now();
    this.stoppedAt = 0;
    this.durationMs = durationMs;
    this.mode = mode;
    this.dropped = 0;
    this.bytes = 0;
    this.events = [];
    this.totals.clear();
    this.resetSummary();
    this.details = {};
    this.mark("capture.start", { mode });
  }

  stop(reason = "manual") {
    if (!this.active) {
      return;
    }
    this.mark("capture.stop", { reason });
    this.stoppedAt = this.now();
    this.active = false;
  }

  clear() {
    this.active = false;
    this.generation++;
    this.events = [];
    this.bytes = 0;
    this.dropped = 0;
    this.totals.clear();
    this.resetSummary();
  }

  get count() {
    return this.events.length;
  }
  get elapsedMs() {
    return this.count === 0
      ? 0
      : Math.max(0, (this.active ? this.now() : this.stoppedAt) - this.startedAt);
  }
  metadata(args: TraceArgs) {
    if (this.active) {
      Object.assign(this.details, cleanArgs(args));
    }
  }
  lose(count: number) {
    if (this.active && Number.isFinite(count)) {
      this.dropped += Math.max(0, Math.floor(count));
    }
  }

  private push(event: TraceEvent) {
    if (!this.active) {
      return false;
    }
    event.name = clean(event.name);
    event.args = cleanArgs(event.args);
    const bytes =
      128 + event.name.length * 2 + (event.args ? JSON.stringify(event.args).length * 2 : 0);
    if (this.events.length >= this.maxEvents || this.bytes + bytes > this.maxBytes) {
      this.dropped++;
      return false;
    }
    this.bytes += bytes;
    this.events.push(event);
    return true;
  }
  mark(name: string, args?: TraceArgs, thread = 2) {
    if (this.active) {
      this.push({
        name,
        cat: "lifecycle",
        ph: "i",
        s: "t",
        ts: this.now() * 1000,
        pid: 1,
        tid: thread,
        args,
      });
    }
  }
  counter(name: string, args: TraceArgs) {
    if (this.active) {
      this.push({ name, cat: "counter", ph: "C", ts: this.now() * 1000, pid: 1, tid: 3, args });
    }
  }
  flow(name: string, id: number, end = false, args?: TraceArgs, at?: number) {
    if (this.active) {
      this.push({
        name,
        cat: "scene",
        ph: end ? "f" : "s",
        ts: (at ?? this.now()) * 1000,
        pid: 1,
        tid: 2,
        id,
        args,
      });
    }
  }
  span(name: string, args?: TraceArgs, category = "browser.cpu") {
    if (!this.active) {
      return noop;
    }
    const start = this.now();
    const generation = this.generation;
    let ended = false;
    return () => {
      if (ended || !this.active || generation !== this.generation) {
        return;
      }
      ended = true;
      this.complete({
        name,
        cat: category,
        ts: start * 1000,
        dur: Math.max(0, this.now() - start) * 1000,
        args,
      });
    };
  }
  complete(event: NativeTraceEvent) {
    if (
      !this.active ||
      !Number.isFinite(event.ts) ||
      !Number.isFinite(event.dur) ||
      event.dur < 0 ||
      !event.name
    ) {
      return;
    }
    const cat = event.cat ?? "native.cpu";
    const cpuThread = cat === "worker.cpu" ? 4 : 1;
    if (
      !this.push({
        ...event,
        cat,
        ph: "X",
        pid: 1,
        tid: cat.includes("wait") ? 2 : cpuThread,
      })
    ) {
      return;
    }
    if (event.name === "native.frame") this.commitStages();
    if (
      cat.includes("cpu") &&
      cat !== "profiler.cpu" &&
      cat !== "worker.cpu" &&
      event.args?.type !== "longtask" &&
      ![
        "engine.frame",
        "browser.frame",
        "native.frame",
        "native.app.update",
        "native.prepare_cpu",
        "bevy.main.First-to-Last",
      ].includes(event.name) &&
      event.dur > 0
    ) {
      this.stages.push(event);
    }
  }
  cpuFrame(durationMs: number) {
    if (!this.active || !Number.isFinite(durationMs) || durationMs < 0) {
      return;
    }
    this.worstCpuFrame = Math.max(this.worstCpuFrame ?? 0, durationMs);
  }
  resetFrameGap() {
    this.previousAnimationFrame = null;
  }
  animationFrame(timestamp: number, active: boolean) {
    if (!this.active || !active || !Number.isFinite(timestamp)) {
      this.resetFrameGap();
      return;
    }
    const previous = this.previousAnimationFrame;
    this.previousAnimationFrame = timestamp;
    if (previous === null || timestamp < previous) return;
    const gap = timestamp - previous;
    this.longestFrameGap = Math.max(this.longestFrameGap ?? 0, gap);
    this.complete({
      name: "browser.frame.interval",
      cat: "frame.wait",
      ts: previous * 1000,
      dur: gap * 1000,
    });
  }
  private resetSummary() {
    this.worstCpuFrame = this.longestFrameGap = null;
    this.resetFrameGap();
    this.stages = [];
  }
  private stageTotals() {
    const totals = new Map([...this.totals].map(([name, value]) => [name, { ...value }]));
    // Native spans complete inside-out. Select the outermost measured stages only;
    // detailed children remain in the trace, without inflating the main-stage summary.
    // oxlint-disable-next-line unicorn/no-array-sort -- Sort a bounded copy; test libraries target ES2022.
    const ordered = this.stages.slice().sort((a, b) => a.ts - b.ts || b.dur - a.dur);
    let end = -Infinity;
    for (const event of ordered) {
      if (event.ts < end) continue;
      end = event.ts + event.dur;
      const value = totals.get(event.name) ?? { total: 0, count: 0 };
      value.total += event.dur / 1000;
      value.count++;
      totals.set(event.name, value);
    }
    return totals;
  }
  private commitStages() {
    this.totals = this.stageTotals();
    this.stages = [];
  }
  summary(): ProfileSummary {
    let hottest: [string, { total: number; count: number }] | undefined;
    for (const value of this.stageTotals())
      if (!hottest || value[1].total > hottest[1].total) {
        hottest = value;
      }
    return {
      worstCpuFrameMs: this.worstCpuFrame,
      longestFrameGapMs: this.longestFrameGap,
      hottestStage: hottest?.[0] ?? null,
      hottestStageMs: hottest ? hottest[1].total / hottest[1].count : null,
    };
  }
  export() {
    const metadata: TraceEvent[] = [
      {
        name: "process_name",
        cat: "metadata",
        ph: "M",
        pid: 1,
        tid: 1,
        ts: 0,
        args: { name: "Open Mars" },
      },
      ...[
        [1, "Browser and WASM CPU"],
        [4, "Terrain worker"],
        [2, "Scene and loading"],
        [3, "Allocation counters"],
      ].map(([id, name]) => ({
        name: "thread_name",
        cat: "metadata",
        ph: "M" as const,
        pid: 1,
        tid: Number(id),
        ts: 0,
        args: { name: String(name) },
      })),
    ];
    return {
      // oxlint-disable-next-line unicorn/no-array-sort -- Sort a fresh export array; tests also target ES2022.
      traceEvents: [...metadata, ...this.events].sort((a, b) => a.ts - b.ts),
      displayTimeUnit: "ms",
      capture: {
        version: 2,
        mode: this.mode,
        durationMs: this.elapsedMs,
        dropped: this.dropped,
        estimatedEventBytes: this.bytes,
        ...this.details,
        summary: this.summary(),
        timing:
          "CPU frame duration and active visible animation-frame intervals are separate measurements. Main stages exclude nested double-counting. Async waits use a separate track; GPU execution time is not measured.",
        memory:
          "GPU counters describe observed requests and explicit destruction during capture, excluding earlier allocations and driver overhead. They are not actual VRAM usage.",
      },
    };
  }
}
const noop = () => {};
