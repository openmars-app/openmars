import type { ProfileRecorder, TraceArgs } from "./profileRecorder";

const bytesPerTexel: Record<string, number> = {
  r8unorm: 1,
  r8snorm: 1,
  r8uint: 1,
  r8sint: 1,
  rg8unorm: 2,
  rg8snorm: 2,
  r16float: 2,
  r16uint: 2,
  r16sint: 2,
  rgba8unorm: 4,
  "rgba8unorm-srgb": 4,
  bgra8unorm: 4,
  "bgra8unorm-srgb": 4,
  rgba8snorm: 4,
  rgba8uint: 4,
  rgba8sint: 4,
  rg16float: 4,
  r32float: 4,
  r32uint: 4,
  rg32float: 8,
  rgba16float: 8,
  rgba16uint: 8,
  rgba16sint: 8,
  rgba32float: 16,
  rgba32uint: 16,
  rgba32sint: 16,
  rgb9e5ufloat: 4,
  rg11b10ufloat: 4,
  rgb10a2unorm: 4,
  depth16unorm: 2,
  depth32float: 4,
  "depth32float-stencil8": 8,
};
export function textureBytes(descriptor: Record<string, unknown>): number | null {
  const size = descriptor.size as
    | number[]
    | { width: number; height?: number; depthOrArrayLayers?: number };
  if (!size) {
    return null;
  }
  const [width, height, depth] = Array.isArray(size)
    ? [size[0], size[1] ?? 1, size[2] ?? 1]
    : [size.width, size.height ?? 1, size.depthOrArrayLayers ?? 1];
  const stride = bytesPerTexel[String(descriptor.format)];
  if (!stride || ![width, height, depth].every((n) => Number.isFinite(n) && n > 0)) {
    return null;
  }
  let total = 0;
  const levels = Math.max(1, Math.min(32, Number(descriptor.mipLevelCount ?? 1)));
  for (let level = 0; level < levels; level++)
    total +=
      Math.max(1, width >> level) *
      Math.max(1, height >> level) *
      (descriptor.dimension === "3d" ? Math.max(1, depth >> level) : depth) *
      stride;
  const result = total * Number(descriptor.sampleCount ?? 1);
  return Number.isFinite(result) && result > 0 ? result : null;
}

/** Wrap only the renderer's device. No resource handles are kept alive by this observer. */
export function observeGpu(device: unknown, recorder: ProfileRecorder): () => void {
  const gpu = device as {
    createTexture?: (d: Record<string, unknown>) => { destroy(): void };
    createBuffer?: (d: Record<string, unknown>) => { destroy(): void };
    addEventListener?: (type: string, fn: EventListener) => void;
    removeEventListener?: (type: string, fn: EventListener) => void;
  };
  if (!gpu?.createTexture || !gpu.createBuffer) {
    return () => {};
  }
  const texture = gpu.createTexture;
  const buffer = gpu.createBuffer;
  let requested = 0;
  let destroyed = 0;
  let peak = 0;
  let unknown = 0;
  let serial = 0;
  let observing = true;
  const count = () =>
    recorder.counter("GPU requests during capture", {
      requestedBytes: requested,
      explicitlyDestroyedBytes: destroyed,
      outstandingKnownBytes: requested - destroyed,
      peakKnownBytes: peak,
      unknownTextureFormats: unknown,
    });
  const wrap = <T extends { destroy(): void }>(
    resource: T,
    name: string,
    bytes: number | null,
    args: TraceArgs,
  ) => {
    const id = ++serial;
    if (bytes === null) {
      unknown++;
    } else {
      requested += bytes;
    }
    peak = Math.max(peak, requested - destroyed);
    recorder.mark(`gpu.${name}.create`, { id, ...args, estimatedBytes: bytes });
    count();
    const original = resource.destroy;
    let retired = false;
    try {
      resource.destroy = function () {
        // Capture primitive metadata only; the observer never owns the resource.
        if (!retired && observing && recorder.active) {
          retired = true;
          destroyed += bytes ?? 0;
          recorder.mark(`gpu.${name}.destroy`, { id, estimatedBytes: bytes });
          count();
        }
        return Reflect.apply(original, this, []);
      };
    } catch {
      recorder.mark("gpu.destroyObserver.unavailable", { id });
    }
    return resource;
  };
  const wrappedTexture = function (this: unknown, descriptor: Record<string, unknown>) {
    let resource;
    try {
      resource = Reflect.apply(texture, this, [descriptor]);
    } catch (error) {
      recorder.mark("gpu.texture.createFailed", {
        label: String(descriptor.label ?? ""),
        format: String(descriptor.format),
        size: JSON.stringify(descriptor.size),
        message: String(error),
      });
      throw error;
    }
    return wrap(resource, "texture", textureBytes(descriptor), {
      label: String(descriptor.label ?? ""),
      format: String(descriptor.format),
      size: JSON.stringify(descriptor.size),
      mips: Number(descriptor.mipLevelCount ?? 1),
      samples: Number(descriptor.sampleCount ?? 1),
    });
  };
  const wrappedBuffer = function (this: unknown, descriptor: Record<string, unknown>) {
    return wrap(
      Reflect.apply(buffer, this, [descriptor]),
      "buffer",
      Number.isFinite(Number(descriptor.size)) && Number(descriptor.size) >= 0
        ? Number(descriptor.size)
        : null,
      { label: String(descriptor.label ?? ""), size: Number(descriptor.size) },
    );
  };
  const error: EventListener = (event) => {
    const value = (event as Event & { error?: { message?: string } }).error;
    recorder.mark("gpu.uncapturedError", { message: value?.message ?? "WebGPU error" });
  };
  try {
    gpu.createTexture = wrappedTexture;
    gpu.createBuffer = wrappedBuffer;
    gpu.addEventListener?.("uncapturederror", error);
  } catch {
    if (gpu.createTexture === wrappedTexture) {
      gpu.createTexture = texture;
    }
    if (gpu.createBuffer === wrappedBuffer) {
      gpu.createBuffer = buffer;
    }
    recorder.mark("gpu.observer.unavailable");
    return () => {};
  }
  return () => {
    observing = false;
    if (gpu.createTexture === wrappedTexture) {
      gpu.createTexture = texture;
    }
    if (gpu.createBuffer === wrappedBuffer) {
      gpu.createBuffer = buffer;
    }
    gpu.removeEventListener?.("uncapturederror", error);
  };
}
