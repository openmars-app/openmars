import { engineProfiler } from "./profiler";
import terrainWorkerUrl from "./terrain.worker?worker&url";

interface TerrainEngine {
  terrain_request(): Uint8Array;
  accept_terrain(bytes: Uint8Array): boolean;
}
interface Reply {
  type: "prepared" | "error";
  id: number;
  bytes: Uint8Array;
  started: number;
  duration: number;
  metadata: Record<string, number>;
  message?: string;
}
export class TerrainPreparation {
  private readonly worker: Worker;
  private readonly bootstrapUrl: string | undefined;
  private serial = 0;
  private disposed = false;
  private readonly pending = new Map<
    number,
    { timer: ReturnType<typeof setTimeout>; capture: number }
  >();
  constructor(
    private readonly engine: TerrainEngine,
    private readonly completed: () => void,
    private readonly failed: (error: Error) => void,
  ) {
    const source = new URL(terrainWorkerUrl, import.meta.url);
    // The gateway loads the game bundle from the selected server. Worker entry
    // URLs must belong to the document origin; their module imports may use CORS.
    if (source.origin !== location.origin) {
      this.bootstrapUrl = URL.createObjectURL(
        new Blob([`import ${JSON.stringify(source.href)};`], { type: "text/javascript" }),
      );
    }
    try {
      this.worker = new Worker(this.bootstrapUrl ?? source, {
        type: "module",
        name: "Open Mars terrain",
      });
    } catch (error) {
      if (this.bootstrapUrl) {
        URL.revokeObjectURL(this.bootstrapUrl);
      }
      throw error;
    }
    this.worker.addEventListener("error", (event) => {
      if (!this.disposed) this.failed(new Error(event.message || "Terrain worker failed"));
    });
    this.worker.addEventListener("messageerror", () => {
      if (!this.disposed) this.failed(new Error("Invalid terrain worker message"));
    });
    this.worker.addEventListener("message", ({ data }: MessageEvent<Reply>) => {
      const pending = this.pending.get(data.id);
      if (this.disposed || !pending) return;
      clearTimeout(pending.timer);
      this.pending.delete(data.id);
      try {
        if (data.type === "error") throw new Error(data.message || "Terrain preparation failed");
        engineProfiler.worker(pending.capture, data.started, data.duration, data.metadata);
        if (this.engine.accept_terrain(data.bytes)) this.completed();
        this.poll();
      } catch (error) {
        this.failed(error instanceof Error ? error : new Error(String(error)));
      }
    });
  }
  poll() {
    if (this.disposed || this.pending.size > 0) return;
    const bytes = this.engine.terrain_request();
    if (!bytes.length) return;
    // Keep one posted job; newer edits coalesce in the native pending owner.
    const id = ++this.serial;
    const timer = setTimeout(() => {
      if (!this.disposed) this.failed(new Error("Terrain preparation timed out"));
    }, 60_000);
    this.pending.set(id, { timer, capture: engineProfiler.workerCapture() });
    this.worker.postMessage({ id, bytes }, [bytes.buffer as ArrayBuffer]);
  }
  dispose() {
    this.disposed = true;
    for (const pending of this.pending.values()) clearTimeout(pending.timer);
    this.pending.clear();
    this.worker.terminate();
    if (this.bootstrapUrl) {
      URL.revokeObjectURL(this.bootstrapUrl);
    }
  }
}
