import { expect, test } from "bun:test";
import { AssetByteCache } from "../../src/engine/assetCache";
import { MAX_ASSET_FILE_BYTES } from "../../src/engine/scene.generated";

test("per-asset limits apply to headers, streamed bytes and cached entries", async () => {
  const signal = new AbortController().signal;
  const bytes = new Uint8Array([1, 2, 3, 4, 5]);
  for (const declared of [true, false]) {
    const cache = new AssetByteCache(
      16,
      async () =>
        new Response(bytes, declared ? { headers: { "content-length": "5" } } : undefined),
    );
    await expect(cache.load("sky", signal, true, () => {}, 4)).rejects.toThrow("download limit");
    expect(await cache.load("sky", signal, true, () => {}, 6)).toEqual(bytes);
    await expect(cache.load("sky", signal, true, () => {}, 4)).rejects.toThrow("download limit");
  }
});

test("world downloads measure bytes and reuse only completed cache entries", async () => {
  let requests = 0;
  const cache = new AssetByteCache(8, async () => {
    requests++;
    return new Response(new Uint8Array([1, 2, 3]), { headers: { "content-length": "3" } });
  });
  const progress: number[] = [];
  const signal = new AbortController().signal;
  expect(await cache.load("model", signal, true, (loaded) => progress.push(loaded))).toEqual(
    new Uint8Array([1, 2, 3]),
  );
  await cache.load("model", signal, true, () => {});
  expect(requests).toBe(1);
  expect(progress).toEqual([0, 3, 3]);
});

test("failed downloads retry, cancellation prevents publication, and memory is bounded", async () => {
  let requests = 0;
  const cache = new AssetByteCache(4, async () => {
    requests++;
    return new Response(new Uint8Array([1, 2, 3]), { status: requests === 1 ? 503 : 200 });
  });
  const signal = new AbortController().signal;
  await expect(cache.load("a", signal, true, () => {})).rejects.toThrow("503");
  const cancelled = new AbortController();
  await expect(cache.load("a", cancelled.signal, true, () => cancelled.abort())).rejects.toThrow();
  await cache.load("a", signal, true, () => {});
  await cache.load("b", signal, true, () => {});
  await cache.load("a", signal, true, () => {});
  expect(requests).toBe(5);
});

test("refuses oversized and truncated responses", async () => {
  const signal = new AbortController().signal;
  for (const length of [String(MAX_ASSET_FILE_BYTES + 1), "5"]) {
    const cache = new AssetByteCache(
      8,
      async () =>
        new Response(new Uint8Array([1]), {
          headers: { "content-length": length },
        }),
    );
    await expect(cache.load("bad", signal, true, () => {})).rejects.toThrow();
  }
});

test("uncached downloads do not displace retained entries", async () => {
  let requests = 0;
  const cache = new AssetByteCache(4, async () => {
    requests++;
    return new Response(new Uint8Array([1, 2, 3]));
  });
  const signal = new AbortController().signal;
  await cache.load("board", signal, true, () => {});
  await cache.load("closeup", signal, false, () => {});
  await cache.load("closeup", signal, false, () => {});
  await cache.load("board", signal, true, () => {});
  expect(requests).toBe(3);
});

test("shrinking the cache releases older entries immediately and respects later growth", async () => {
  let requests = 0;
  const cache = new AssetByteCache(8, async () => {
    requests++;
    return new Response(new Uint8Array([1, 2, 3, 4]));
  });
  const signal = new AbortController().signal;
  const load = (url: string) => cache.load(url, signal, true, () => {});
  await load("a");
  await load("b");
  await load("a");
  cache.setCapacity(4);
  await load("a");
  expect(requests).toBe(2);
  cache.setCapacity(8);
  await load("b");
  await load("a");
  await load("b");
  expect(requests).toBe(3);
  cache.setCapacity(0);
  await load("a");
  await load("a");
  expect(requests).toBe(5);
  for (const invalid of [-1, NaN, Infinity, 1.5]) {
    expect(() => cache.setCapacity(invalid)).toThrow("Invalid asset cache capacity");
  }
});

test("a downgrade during a download cannot repopulate an oversized cache entry", async () => {
  let requests = 0;
  const cache = new AssetByteCache(8, async () => {
    requests++;
    return new Response(new Uint8Array([1, 2, 3, 4]));
  });
  const signal = new AbortController().signal;
  await cache.load("a", signal, true, () => cache.setCapacity(2));
  await cache.load("a", signal, true, () => {});
  expect(requests).toBe(2);
});

// Every resident destination uses the same generated format contract.
test("renderer declarations match every generated asset format", async () => {
  const { rendererAssets } = await import("../../src/engine/scene.generated");
  const { assets } = await import("../../src/assets/generated/registry");
  for (const requirement of rendererAssets) {
    const asset = assets[requirement.id as keyof typeof assets];
    expect(asset).toBeDefined();
    for (const variant of asset.variants)
      expect(variant.url.endsWith(requirement.extension)).toBe(true);
  }
});
