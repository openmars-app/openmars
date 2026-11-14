import { describe, expect, test } from "bun:test";
import { ProfileRecorder } from "../../src/engine/profileRecorder";
import { observeGpu, textureBytes } from "../../src/engine/profileGpu";
import { EngineProfiler } from "../../src/engine/profiler";

describe("bounded phase traces", () => {
  test("CPU duration and visible animation-frame gaps are distinct and pause-safe", () => {
    const trace = new ProfileRecorder(() => 0);
    trace.start();
    trace.animationFrame(100, true);
    trace.animationFrame(260, true);
    trace.cpuFrame(3);
    trace.animationFrame(5000, false);
    trace.animationFrame(9000, true);
    trace.animationFrame(9016, true);
    trace.resetFrameGap();
    trace.animationFrame(20000, true);
    trace.animationFrame(20016, true);
    expect(trace.summary().worstCpuFrameMs).toBe(3);
    expect(trace.summary().longestFrameGapMs).toBe(160);
    const exported = trace.export();
    expect(exported.capture.version).toBe(2);
    expect(
      exported.traceEvents
        .filter((event) => event.name === "browser.frame.interval")
        .map((event) => event.dur),
    ).toEqual([160000, 16000, 16000]);
    trace.stop();
    trace.start();
    expect(trace.summary().worstCpuFrameMs).toBeNull();
    expect(trace.summary().longestFrameGapMs).toBeNull();
  });
  test("main stages include Bevy render without nested CPU double-counting", () => {
    const trace = new ProfileRecorder(() => 0);
    trace.start();
    // Nested/recursive leaves would outrank render if their inclusive totals were summed.
    for (let i = 0; i < 2; i++) trace.complete({ name: "leaf", ts: 1000, dur: 8000 });
    trace.complete({ name: "bevy.render", ts: 0, dur: 10000 });
    trace.complete({ name: "native.app.update", ts: 0, dur: 12000 });
    trace.complete({ name: "native.frame", ts: 0, dur: 15000 });
    // Delayed LongTask reports include Native work already accounted above.
    trace.complete({
      name: "self",
      cat: "browser.cpu",
      ts: 0,
      dur: 1000000,
      args: { type: "longtask" },
    });
    trace.complete({ name: "profiler.drain", cat: "profiler.cpu", ts: 15000, dur: 999000 });
    expect(trace.summary().hottestStage).toBe("bevy.render");
    expect(trace.summary().hottestStageMs).toBe(10);
    expect(trace.export().traceEvents.filter((event) => event.name === "leaf")).toHaveLength(2);
    expect(trace.export().traceEvents.some((event) => event.name === "self")).toBe(true);
  });
  test("disabled spans do not read clocks, restarted spans cannot enter a new capture", () => {
    let calls = 0;
    let now = 20;
    const trace = new ProfileRecorder(() => {
      calls++;
      return now;
    });
    trace.span("disabled")();
    expect(calls).toBe(0);
    trace.start();
    const old = trace.span("stale");
    trace.stop();
    trace.start();
    old();
    const end = trace.span("work");
    now = 25;
    end();
    end();
    trace.stop();
    const events = trace.export().traceEvents;
    expect(events.filter((e) => e.name === "stale")).toHaveLength(0);
    expect(events.filter((e) => e.name === "work")).toHaveLength(1);
    expect(events.find((e) => e.name === "work")?.dur).toBe(5000);
  });
  test("event and byte limits preserve valid export and report lost records", () => {
    const trace = new ProfileRecorder(() => 0, 3, 1024);
    trace.start();
    for (let i = 0; i < 20; i++) trace.mark("event", { i });
    trace.stop();
    expect(trace.count).toBe(3);
    expect(trace.dropped).toBe(19);
    expect(JSON.parse(JSON.stringify(trace.export())).traceEvents.length).toBeGreaterThan(3);
    const bytes = new ProfileRecorder(() => 0, 1000, 200);
    bytes.start();
    bytes.mark("large", { detail: "x".repeat(1000) });
    expect(bytes.count).toBe(1);
    expect(bytes.dropped).toBe(1);
  });
  test("unknown durations and non-finite arguments cannot corrupt the export", () => {
    const trace = new ProfileRecorder(() => 2);
    trace.start();
    trace.complete({ name: "bad", ts: 0, dur: NaN });
    trace.mark("safe", { a: Infinity, url: "/api/game?playerId=secret" });
    const event = trace.export().traceEvents.find((item) => item.name === "safe");
    expect(event?.args).toEqual({ url: "/api/game?…" });
    expect(trace.export().traceEvents.some((item) => item.name === "bad")).toBe(false);
  });
});

test("frame harvesting is bounded and stop drains every remaining Native batch", () => {
  const profiler = new EngineProfiler();
  let enabled = false;
  const events = Array.from({ length: 400 }, (_, index) => ({
    name: "queued.native",
    ts: index * 100,
    dur: 10,
  }));
  const limits: number[] = [];
  const detach = profiler.native({
    setEnabled(active) {
      enabled = active;
    },
    setContext() {},
    drain(limit) {
      limits.push(limit);
      const batch = events.splice(0, limit);
      return { events: batch, dropped: 0, remaining: events.length };
    },
  });
  profiler.start();
  profiler.afterFrame();
  expect(limits).toEqual([128]);
  expect(events).toHaveLength(272);
  profiler.afterFrame();
  expect(events).toHaveLength(144);
  profiler.stop();
  expect(enabled).toBe(false);
  expect(events).toHaveLength(0);
  expect(limits.every((limit) => limit === 128)).toBe(true);
  expect(
    profiler.export().traceEvents.filter((event) => event.name === "queued.native"),
  ).toHaveLength(400);
  detach();
});

test("renderer failure fully drains a buffered backlog before stopping", () => {
  const profiler = new EngineProfiler();
  const events = Array.from({ length: 500 }, (_, index) => ({
    name: "failure.native",
    ts: index,
    dur: 1,
  }));
  const detach = profiler.native({
    setEnabled() {},
    setContext() {},
    drain(limit) {
      return { events: events.splice(0, limit), dropped: 0, remaining: events.length };
    },
  });
  profiler.start();
  profiler.failure(new Error("Device lost"));
  expect(events).toHaveLength(0);
  expect(
    profiler.export().traceEvents.filter((event) => event.name === "failure.native"),
  ).toHaveLength(500);
  expect(profiler.snapshot().phase).toBe("ready");
  detach();
});

describe("GPU request observation", () => {
  test("estimates mips and distinguishes arrays from volumes, unknown formats remain unknown", () => {
    expect(textureBytes({ size: [4, 4, 2], format: "rgba8unorm", mipLevelCount: 3 })).toBe(168);
    expect(
      textureBytes({ size: [4, 4, 2], dimension: "3d", format: "rgba8unorm", mipLevelCount: 3 }),
    ).toBe(148);
    expect(textureBytes({ size: [4, 4], format: "rgba16float", sampleCount: 4 })).toBe(512);
    expect(textureBytes({ size: [4, 4], format: "depth24plus" })).toBeNull();
  });
  test("observes exact requested descriptors, explicit destroy once and restores methods", () => {
    const trace = new ProfileRecorder(() => 1);
    trace.start();
    let destroyed = 0;
    const gpu = {
      createTexture: (_d: Record<string, unknown>) => ({
        destroy() {
          destroyed++;
        },
      }),
      createBuffer: (_d: Record<string, unknown>) => ({ destroy() {} }),
    };
    const original = gpu.createTexture;
    const detach = observeGpu(gpu, trace);
    const texture = gpu.createTexture({
      size: [4, 4],
      format: "rgba8unorm",
      label: "main_texture_a",
    });
    texture.destroy();
    texture.destroy();
    detach();
    expect(gpu.createTexture).toBe(original);
    expect(destroyed).toBe(2);
    const events = trace.export().traceEvents;
    expect(events.find((event) => event.name === "gpu.texture.create")?.args?.estimatedBytes).toBe(
      64,
    );
    expect(events.filter((event) => event.name === "gpu.texture.destroy")).toHaveLength(1);
  });
  test("device registration survives consecutive recordings and releases only its own reference", () => {
    const profiler = new EngineProfiler();
    const gpu = {
      createTexture: (_d: Record<string, unknown>) => ({ destroy() {} }),
      createBuffer: (_d: Record<string, unknown>) => ({ destroy() {} }),
    };
    const release = profiler.device(gpu);
    for (let i = 0; i < 2; i++) {
      profiler.start();
      gpu.createBuffer({ size: 16 });
      profiler.stop();
      expect(
        profiler.export().traceEvents.filter((event) => event.name === "gpu.buffer.create"),
      ).toHaveLength(1);
    }
    release();
    profiler.clear();
  });
});

test("failure keeps the final native events and capture metadata available for download", () => {
  const profiler = new EngineProfiler();
  profiler.metadata({ backend: "webgpu", viewport: "1280x720" });
  let enabled = false;
  let pending = false;
  const detach = profiler.native({
    setEnabled(active) {
      enabled = active;
      if (active) pending = true;
    },
    setContext() {},
    drain() {
      const events = pending
        ? [{ name: "final.native.work", ts: performance.now() * 1000, dur: 20 }]
        : [];
      pending = false;
      return {
        events,
        dropped: 0,
        remaining: 0,
      };
    },
  });
  profiler.start();
  profiler.failure(new Error("OutOfMemory"));
  expect(enabled).toBe(false);
  expect(profiler.snapshot().phase).toBe("ready");
  const trace = profiler.export();
  expect(Reflect.get(trace.capture, "backend")).toBe("webgpu");
  expect(trace.traceEvents.some((event) => event.name === "final.native.work")).toBe(true);
  expect(trace.traceEvents.find((event) => event.name === "renderer.failure")?.args?.message).toBe(
    "OutOfMemory",
  );
  detach();
});

test("a late device from an ended capture cannot attach to the next capture", async () => {
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, "GPUAdapter");
  let resolveDevice: (device: unknown) => void = () => {};
  const gpu = { createTexture: () => ({ destroy() {} }), createBuffer: () => ({ destroy() {} }) };
  const original = gpu.createBuffer;
  class Adapter {
    info = { vendor: "test", architecture: "test", description: "test" };
    requestDevice() {
      return new Promise((resolve) => {
        resolveDevice = resolve;
      });
    }
  }
  Object.defineProperty(globalThis, "GPUAdapter", { configurable: true, value: Adapter });
  const profiler = new EngineProfiler();
  try {
    profiler.start();
    const pending = new Adapter().requestDevice();
    profiler.stop();
    profiler.start();
    resolveDevice(gpu);
    await pending;
    expect(gpu.createBuffer).toBe(original);
    profiler.stop();
  } finally {
    profiler.stop();
    if (descriptor) {
      Object.defineProperty(globalThis, "GPUAdapter", descriptor);
    } else {
      Reflect.deleteProperty(globalThis, "GPUAdapter");
    }
  }
});

test("worker CPU has its own lane and late jobs cannot enter a subsequent capture", () => {
  const profiler = new EngineProfiler();
  profiler.start();
  const previous = profiler.workerCapture();
  profiler.stop();
  profiler.start();
  const context = { context: 7, landscapeRevision: 2 };
  profiler.worker(previous, performance.timeOrigin + performance.now(), 80, context);
  profiler.worker(
    profiler.workerCapture(),
    performance.timeOrigin + performance.now(),
    40,
    context,
  );
  profiler.stop();
  const events = profiler
    .export()
    .traceEvents.filter((event) => event.name === "terrain.worker.generate");
  expect(events).toHaveLength(1);
  expect(events[0].tid).toBe(4);
  expect(events[0].cat).toBe("worker.cpu");
  expect(events[0].args).toEqual(context);
  expect(profiler.export().capture.summary.worstCpuFrameMs).toBeNull();
});
