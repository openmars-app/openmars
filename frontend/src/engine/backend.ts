export type EngineBackend = "webgpu" | "webgl2";
export type BackendPreference = EngineBackend | "auto";

export interface RendererCapabilities {
  computeShaders: boolean;
  storageBuffers: boolean;
  maxTextureDimension2D: number;
}

export function backendPreference(): BackendPreference {
  if (import.meta.env.DEV) {
    const value = new URLSearchParams(window.location.search).get("renderer");
    if (value === "webgpu" || value === "webgl2") return value;
  }
  return "auto";
}
