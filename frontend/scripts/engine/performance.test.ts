import { describe, expect, test } from "bun:test";
import { DrawCounter, EngineMeter, triangleCount } from "../../src/engine/performance";

function gpu() {
  const pass = () => ({
    setPipeline(_pipeline: object) {},
    draw(_count: number, _instances = 1) {},
    drawIndexed(_count: number, _instances = 1) {},
    drawIndirect() {},
    drawIndexedIndirect() {},
    executeBundles(_bundles: object[]) {},
    end() {},
  });
  return {
    queue: { submit(_buffers: object[]) {} },
    createRenderPipeline(_descriptor: object) {
      return {};
    },
    createRenderPipelineAsync(_descriptor: object) {
      return Promise.resolve({});
    },
    createCommandEncoder() {
      return {
        beginRenderPass: pass,
        finish() {
          return {};
        },
      };
    },
    createRenderBundleEncoder() {
      return {
        ...pass(),
        finish() {
          return {};
        },
      };
    },
  };
}

describe("per-engine GPU performance counters", () => {
  test("counts submitted work, not merely encoded work, and restores exact methods", () => {
    const device = gpu();
    const original = device.createCommandEncoder;
    const counter = new DrawCounter("webgpu", device, null);
    counter.setActive(true);
    const pipeline = device.createRenderPipeline({ primitive: { topology: "triangle-list" } });
    const encoder = device.createCommandEncoder();
    const pass = encoder.beginRenderPass();
    pass.setPipeline(pipeline);
    pass.drawIndexed(12, 3);
    pass.end();
    expect(counter.read().draws).toBe(0);
    device.queue.submit([encoder.finish()]);
    expect(counter.read()).toMatchObject({
      draws: 1,
      triangles: 12,
      passes: 1,
      unknownTriangles: false,
    });
    counter.setActive(false);
    expect(device.createCommandEncoder).toBe(original);
    expect(Object.hasOwn(device.queue, "submit")).toBe(true);
  });

  test("counts every render-bundle execution while leaving indirect triangles unknown", () => {
    const device = gpu();
    const counter = new DrawCounter("webgpu", device, null);
    counter.setActive(true);
    const pipeline = device.createRenderPipeline({});
    const bundleEncoder = device.createRenderBundleEncoder();
    bundleEncoder.setPipeline(pipeline);
    bundleEncoder.draw(6);
    const bundle = bundleEncoder.finish();
    const encoder = device.createCommandEncoder();
    const pass = encoder.beginRenderPass();
    pass.executeBundles([bundle, bundle]);
    pass.drawIndirect();
    pass.end();
    device.queue.submit([encoder.finish()]);
    expect(counter.read()).toMatchObject({
      draws: 3,
      triangles: 4,
      indirect: 1,
      unknownTriangles: true,
    });
    counter.setActive(false);
  });

  test("does not invent topology or draw counts for pre-existing opaque commands", () => {
    const device = gpu();
    const pipeline = device.createRenderPipeline({});
    const counter = new DrawCounter("webgpu", device, null);
    counter.setActive(true);
    const encoder = device.createCommandEncoder();
    const pass = encoder.beginRenderPass();
    pass.setPipeline(pipeline);
    pass.draw(9);
    pass.end();
    device.queue.submit([encoder.finish()]);
    expect(counter.read()).toMatchObject({ draws: 1, unknownTriangles: true, unknownDraws: false });
    counter.beginFrame();
    device.queue.submit([{}]);
    expect(counter.read().unknownDraws).toBe(true);
    counter.setActive(false);
  });

  test("WebGL counts instanced triangles and preserves line topology", () => {
    const gl = {
      drawElementsInstanced(
        _mode: number,
        _count: number,
        _type: number,
        _offset: number,
        _instances: number,
      ) {},
      drawArrays(_mode: number, _first: number, _count: number) {},
    };
    const original = gl.drawArrays;
    const counter = new DrawCounter("webgl2", null, gl as unknown as WebGL2RenderingContext);
    counter.setActive(true);
    gl.drawElementsInstanced(4, 9, 0, 0, 4);
    gl.drawArrays(1, 0, 80);
    expect(counter.read()).toMatchObject({ draws: 2, triangles: 12, unknownTriangles: false });
    counter.setActive(false);
    expect(gl.drawArrays).toBe(original);
    expect(triangleCount("triangle-strip", 5, 2)).toBe(6);
  });

  test("engine FPS uses submissions, paused values are unavailable, and WASM memory is measured", () => {
    let time = 0;
    let frames = 20;
    const memory = new WebAssembly.Memory({ initial: 1 });
    const engine = {
      submitted_frames: () => frames,
      browser_device: () => gpu(),
      linear_memory: () => memory,
      performance_stats: () =>
        JSON.stringify({
          submittedFrames: frames,
          completedFrames: frames - 1,
          pendingFrames: 1,
          frameLimit: 4,
          frameBudget: 4,
          completionAckMs: 16,
          entities: 4,
          visibleMeshes: 2,
          residentMeshes: 2,
          residentVertices: 6,
          residentTriangles: 2,
          residentImages: 1,
          residentMaterials: 1,
        }),
    };
    const meter = new EngineMeter(engine, "webgpu", {} as HTMLCanvasElement, () => time);
    meter.setActive(true);
    const before = meter.beforeFrame();
    frames++;
    meter.afterFrame(before, 3.5);
    time = 250;
    expect(meter.sample("rendering")).toMatchObject({ fps: 4, cpuMs: 3.5, wasmBytes: 65536 });
    memory.grow(1);
    time = 500;
    expect(meter.sample("paused")).toMatchObject({ fps: null, cpuMs: null, wasmBytes: 131072 });
    meter.setActive(false);
    expect(meter.sample("rendering")).toBeNull();
  });
});
