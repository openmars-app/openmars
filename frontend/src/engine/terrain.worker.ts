import init, { TerrainPreparer } from "../../../engine/pkg/preparation/terrain_worker.js";

interface Request {
  id: number;
  bytes: Uint8Array;
}
const scope = globalThis as unknown as {
  addEventListener(type: "message", listener: (event: MessageEvent<Request>) => void): void;
  postMessage(message: unknown, transfer?: Transferable[]): void;
};
const ready = init({
  module_or_path: new URL(
    "../../../engine/pkg/preparation/terrain_worker_bg.wasm",
    import.meta.url,
  ),
}).then(() => new TerrainPreparer());
void ready.catch(() => {});
// One dedicated owner retains map relief, noise and patch caches between jobs.
let queued = Promise.resolve();
scope.addEventListener("message", ({ data }) => {
  queued = queued.then(async () => {
    try {
      const preparer = await ready;
      const started = performance.timeOrigin + performance.now();
      const bytes = preparer.prepare(data.bytes);
      const duration = performance.timeOrigin + performance.now() - started;
      const metadata = JSON.parse(preparer.metadata()) as Record<string, number>;
      scope.postMessage({ type: "prepared", id: data.id, bytes, started, duration, metadata }, [
        bytes.buffer as ArrayBuffer,
      ]);
    } catch (error) {
      scope.postMessage({ type: "error", id: data.id, message: String(error) }, []);
    }
  });
});
