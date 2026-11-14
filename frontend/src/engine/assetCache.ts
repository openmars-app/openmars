import { MAX_ASSET_FILE_BYTES, SCENE_ASSET_CACHE_BYTES } from "./scene.generated";

/** Completed, immutable downloads only. Cancellation belongs to each consumer. */
export class AssetByteCache {
  private entries = new Map<string, Uint8Array>();
  private bytes = 0;

  constructor(
    private capacity: number = SCENE_ASSET_CACHE_BYTES,
    private fetcher: (url: string, init: RequestInit) => Promise<Response> = (url, init) =>
      fetch(url, init),
  ) {
    this.setCapacity(capacity);
  }

  setCapacity(capacity: number) {
    if (!Number.isSafeInteger(capacity) || capacity < 0) {
      throw new RangeError("Invalid asset cache capacity");
    }
    this.capacity = capacity;
    this.trim();
  }

  private trim(incoming = 0) {
    while (this.bytes + incoming > this.capacity && this.entries.size) {
      const oldest = this.entries.keys().next().value!;
      this.bytes -= this.entries.get(oldest)!.length;
      this.entries.delete(oldest);
    }
  }

  clear() {
    this.entries.clear();
    this.bytes = 0;
  }

  async load(
    url: string,
    signal: AbortSignal,
    retain: boolean,
    progress: (loaded: number, total: number | null) => void,
    limit = MAX_ASSET_FILE_BYTES,
  ): Promise<Uint8Array> {
    signal.throwIfAborted();
    const cached = this.entries.get(url);
    if (cached) {
      if (cached.length > limit) {
        throw new Error("World asset exceeds the download limit");
      }
      this.entries.delete(url);
      this.entries.set(url, cached);
      progress(cached.length, cached.length);
      return cached;
    }
    const response = await this.fetcher(url, { signal });
    if (!response.ok) {
      await response.body?.cancel();
      throw new Error(`World asset download failed (${response.status})`);
    }
    const length = Number(response.headers.get("content-length"));
    const total = !response.headers.get("content-encoding") && length > 0 ? length : null;
    if (total && total > limit) {
      await response.body?.cancel();
      throw new Error("World asset exceeds the download limit");
    }
    if (!response.body) {
      throw new Error("World asset response has no body");
    }
    const reader = response.body.getReader();
    const chunks: Uint8Array[] = [];
    let loaded = 0;
    progress(0, total);
    try {
      while (true) {
        signal.throwIfAborted();
        const chunk = await reader.read();
        if (chunk.done) {
          break;
        }
        loaded += chunk.value.length;
        if (loaded > limit) {
          throw new Error("World asset exceeds the download limit");
        }
        chunks.push(chunk.value);
        progress(loaded, total);
      }
      signal.throwIfAborted();
    } catch (error) {
      await reader.cancel().catch(() => {});
      throw error;
    } finally {
      reader.releaseLock();
    }
    if (loaded === 0 || (total !== null && loaded !== total)) {
      throw new Error("World asset download was incomplete");
    }
    const bytes = new Uint8Array(loaded);
    let offset = 0;
    for (const chunk of chunks) {
      bytes.set(chunk, offset);
      offset += chunk.length;
    }
    if (retain && bytes.length <= this.capacity) {
      const previous = this.entries.get(url);
      if (previous) {
        this.bytes -= previous.length;
        this.entries.delete(url);
      }
      this.trim(bytes.length);
      this.entries.set(url, bytes);
      this.bytes += bytes.length;
    }
    progress(loaded, loaded);
    return bytes;
  }
}
