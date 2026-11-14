import type { EngineBackend } from "./backend";

export interface NativePerformance {
  submittedFrames: number;
  completedFrames: number;
  pendingFrames: number;
  frameLimit: number;
  frameBudget: number;
  /** Latest completion acknowledgment latency, including GPU work and browser
   * scheduling/IPC; not GPU execution time. Null before the first acknowledgment. */
  completionAckMs: number | null;
  entities: number;
  visibleMeshes: number;
  residentMeshes: number;
  residentVertices: number;
  residentTriangles: number;
  residentImages: number;
  residentMaterials: number;
}

export interface EnginePerformance extends NativePerformance {
  backend: EngineBackend;
  state: "rendering" | "paused" | "preparing";
  fps: number | null;
  cpuMs: number | null;
  drawCalls: number | null;
  submittedTriangles: number | null;
  indirectDraws: number | null;
  renderPasses: number | null;
  wasmBytes: number;
}

export interface PerformanceSource {
  setActive(active: boolean): void;
  sample(): EnginePerformance | null;
}

interface Work {
  draws: number;
  triangles: number;
  indirect: number;
  passes: number;
  unknownDraws: boolean;
  unknownTriangles: boolean;
}

const emptyWork = (): Work => ({
  draws: 0,
  triangles: 0,
  indirect: 0,
  passes: 0,
  unknownDraws: false,
  unknownTriangles: false,
});

function merge(target: Work, source: Work) {
  target.draws += source.draws;
  target.triangles += source.triangles;
  target.indirect += source.indirect;
  target.passes += source.passes;
  target.unknownDraws ||= source.unknownDraws;
  target.unknownTriangles ||= source.unknownTriangles;
}

export function triangleCount(topology: unknown, count: number, instances = 1): number | null {
  if (topology === "triangle-list" || topology === 4) return Math.floor(count / 3) * instances;
  if (
    topology === "triangle-strip" ||
    topology === "triangle-fan" ||
    topology === 5 ||
    topology === 6
  )
    return Math.max(0, count - 2) * instances;
  if (["point-list", "line-list", "line-strip", 0, 1, 2, 3].includes(topology as string | number))
    return 0;
  return null;
}

type BrowserObject = object;
type Call = (...args: unknown[]) => unknown;
const object = (value: unknown): BrowserObject | null =>
  typeof value === "object" && value !== null ? value : null;

/** Observes only this engine's actual device/context. No prototypes or globals are replaced. */
export class DrawCounter {
  private restores = new Set<() => void>();
  private active = false;
  private work = emptyWork();
  private buffers = new WeakMap<BrowserObject, Work>();
  private bundles = new WeakMap<BrowserObject, Work>();
  private topologies = new WeakMap<BrowserObject, unknown>();
  available = false;

  constructor(
    private backend: EngineBackend,
    private device: unknown,
    private gl: WebGL2RenderingContext | null,
  ) {}

  setActive(active: boolean) {
    if (active === this.active) return;
    this.active = active;
    this.work = emptyWork();
    if (!active) {
      for (const restore of [...this.restores]) restore();
      return;
    }
    try {
      this.available = this.backend === "webgl2" ? !!this.gl : !!object(this.device);
      if (this.backend === "webgl2") this.observeGl();
      else this.observeGpu();
    } catch {
      for (const restore of [...this.restores]) restore();
      this.available = false;
    }
  }

  beginFrame() {
    this.work = emptyWork();
  }
  read() {
    return { ...this.work };
  }

  private patch(
    target: BrowserObject,
    key: string,
    handler: (call: Call, args: unknown[]) => unknown,
  ) {
    const original = Reflect.get(target, key);
    if (typeof original !== "function") return () => {};
    const descriptor = Object.getOwnPropertyDescriptor(target, key);
    const call: Call = (...values) => Reflect.apply(original, target, values);
    const wrapped = (...args: unknown[]) => handler(call, args);
    Object.defineProperty(target, key, { configurable: true, writable: true, value: wrapped });
    const restore = () => {
      if (Reflect.get(target, key) === wrapped) {
        if (descriptor) Object.defineProperty(target, key, descriptor);
        else Reflect.deleteProperty(target, key);
      }
      this.restores.delete(restore);
    };
    this.restores.add(restore);
    return restore;
  }

  private record(work: Work, topology: unknown, count: number, instances: number) {
    work.draws++;
    const triangles = triangleCount(topology, count, instances);
    if (triangles === null) work.unknownTriangles = true;
    else work.triangles += triangles;
  }

  private observeGl() {
    if (!this.gl) return;
    for (const [method, count, instances] of [
      ["drawArrays", 2, null],
      ["drawElements", 1, null],
      ["drawRangeElements", 3, null],
      ["drawArraysInstanced", 2, 3],
      ["drawElementsInstanced", 1, 4],
    ] as const) {
      this.patch(this.gl, method, (call, args) => {
        const result = call(...args);
        this.record(
          this.work,
          args[0],
          Number(args[count]),
          instances === null ? 1 : Number(args[instances]),
        );
        return result;
      });
    }
  }

  private observeGpu() {
    const device = object(this.device);
    if (!device) return;
    for (const method of ["createRenderPipeline", "createRenderPipelineAsync"]) {
      this.patch(device, method, (call, args) => {
        const descriptor = args[0] as { primitive?: { topology?: string } };
        const topology = descriptor.primitive?.topology ?? "triangle-list";
        const result = call(...args);
        const record = (pipeline: unknown) => {
          const value = object(pipeline);
          if (value) this.topologies.set(value, topology);
          return pipeline;
        };
        return method.endsWith("Async") ? Promise.resolve(result).then(record) : record(result);
      });
    }
    this.patch(device, "createCommandEncoder", (call, args) => {
      const encoder = object(call(...args));
      if (!encoder) return encoder;
      const work = emptyWork();
      const restorePass = this.patch(encoder, "beginRenderPass", (begin, values) => {
        const pass = object(begin(...values));
        if (pass) {
          work.passes++;
          this.observePass(pass, work, "end");
        }
        return pass;
      });
      const restoreFinish = this.patch(encoder, "finish", (finish, values) => {
        const buffer = object(finish(...values));
        if (buffer) this.buffers.set(buffer, work);
        restorePass();
        restoreFinish();
        return buffer;
      });
      return encoder;
    });
    this.patch(device, "createRenderBundleEncoder", (call, args) => {
      const encoder = object(call(...args));
      if (encoder) this.observePass(encoder, emptyWork(), "finish");
      return encoder;
    });
    const queue = object(Reflect.get(device, "queue"));
    if (queue)
      this.patch(queue, "submit", (call, args) => {
        const buffers = Array.from(args[0] as Iterable<BrowserObject>);
        const result = call(buffers);
        for (const buffer of buffers) {
          const recorded = this.buffers.get(buffer);
          if (recorded) merge(this.work, recorded);
          else {
            this.work.unknownDraws = true;
            this.work.unknownTriangles = true;
          }
        }
        return result;
      });
  }

  private observePass(pass: BrowserObject, work: Work, terminal: "end" | "finish") {
    let topology: unknown;
    const restores: (() => void)[] = [];
    const patch = (key: string, handler: (call: Call, args: unknown[]) => unknown) => {
      restores.push(this.patch(pass, key, handler));
    };
    patch("setPipeline", (call, args) => {
      const pipeline = object(args[0]);
      topology = pipeline ? this.topologies.get(pipeline) : undefined;
      return call(...args);
    });
    for (const method of ["draw", "drawIndexed"])
      patch(method, (call, args) => {
        const result = call(...args);
        this.record(work, topology, Number(args[0]), Number(args[1] ?? 1));
        return result;
      });
    for (const method of ["drawIndirect", "drawIndexedIndirect"])
      patch(method, (call, args) => {
        const result = call(...args);
        work.draws++;
        work.indirect++;
        work.unknownTriangles = true;
        return result;
      });
    patch("executeBundles", (call, args) => {
      const bundles = Array.from(args[0] as Iterable<BrowserObject>);
      const result = call(bundles);
      for (const bundle of bundles) {
        const recorded = this.bundles.get(bundle);
        if (recorded) merge(work, recorded);
        else {
          work.unknownDraws = true;
          work.unknownTriangles = true;
        }
      }
      return result;
    });
    patch(terminal, (call, args) => {
      const result = call(...args);
      if (terminal === "finish") {
        const bundle = object(result);
        if (bundle) this.bundles.set(bundle, work);
      }
      for (const restore of restores) restore();
      return result;
    });
  }
}

interface MeasuredEngine {
  submitted_frames(): number;
  performance_stats(): string;
  browser_device(): unknown;
  linear_memory(): WebAssembly.Memory;
}

/** Sampling is opt-in. Inactive game rendering pays no measurement or command-hook cost. */
export class EngineMeter {
  active = false;
  private counter: DrawCounter;
  private lastSample = 0;
  private lastFrames = 0;
  private cpuTotal = 0;
  private cpuFrames = 0;
  private lastWork: Work | null = null;

  constructor(
    private engine: MeasuredEngine,
    private backend: EngineBackend,
    canvas: HTMLCanvasElement,
    private now = () => performance.now(),
  ) {
    this.counter = new DrawCounter(
      backend,
      engine.browser_device(),
      backend === "webgl2" ? canvas.getContext("webgl2") : null,
    );
  }

  setActive(active: boolean) {
    if (this.active === active) return;
    this.active = active;
    this.lastSample = this.now();
    this.lastFrames = this.engine.submitted_frames();
    this.cpuTotal = 0;
    this.cpuFrames = 0;
    this.lastWork = null;
    this.counter.setActive(active);
  }

  beforeFrame() {
    this.counter.beginFrame();
    return this.engine.submitted_frames();
  }

  afterFrame(before: number, cpuMs: number) {
    if (this.engine.submitted_frames() > before) {
      this.cpuTotal += cpuMs;
      this.cpuFrames++;
      this.lastWork = this.counter.read();
    }
  }

  sample(state: EnginePerformance["state"]): EnginePerformance | null {
    if (!this.active) return null;
    const now = this.now();
    const stats = JSON.parse(this.engine.performance_stats()) as NativePerformance;
    const elapsed = now - this.lastSample;
    const fps =
      elapsed > 0 ? (1000 * Math.max(0, stats.submittedFrames - this.lastFrames)) / elapsed : 0;
    const cpuMs = this.cpuFrames ? this.cpuTotal / this.cpuFrames : null;
    this.lastSample = now;
    this.lastFrames = stats.submittedFrames;
    this.cpuFrames = 0;
    this.cpuTotal = 0;
    const work = this.lastWork;
    const measured = this.counter.available && work;
    return {
      ...stats,
      backend: this.backend,
      state,
      fps: state === "paused" ? null : fps,
      cpuMs: state === "paused" ? null : cpuMs,
      drawCalls: measured && !work.unknownDraws ? work.draws : null,
      submittedTriangles: measured && !work.unknownTriangles ? work.triangles : null,
      indirectDraws: measured ? work.indirect : null,
      renderPasses: measured && this.backend === "webgpu" ? work.passes : null,
      wasmBytes: this.engine.linear_memory().buffer.byteLength,
    };
  }
}
