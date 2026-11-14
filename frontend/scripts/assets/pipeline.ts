import { createHash } from "node:crypto";
import { promises as fs } from "node:fs";
import { createReadStream } from "node:fs";
import path from "node:path";
import { NodeIO, VERSION as gltfVersion } from "@gltf-transform/core";
import { ALL_EXTENSIONS } from "@gltf-transform/extensions";
import { dedup, prune, textureCompress } from "@gltf-transform/functions";
import sharp from "sharp";
import {
  assetToolVersion,
  prepareEnvironment,
  prepareTextureArray,
  validEnvironment,
  type EnvironmentSpec,
} from "./environment.ts";

export interface Profile {
  copy?: boolean;
  sizes?: number[];
  lossless?: boolean;
  quality?: number;
  copyMatchingWebp?: boolean;
  gltf?: { maxTextureSize: number };
  environment?: EnvironmentSpec;
  textureArray?: { size: number; sizes?: number[]; srgb: boolean };
}
export interface Entry {
  id: string;
  source?: string;
  layers?: string[];
  profile: string;
  prepare?: { crop: [number, number, number, number]; charcoalMatte: boolean };
  corporation?: { key: string; name: string; color: string };
  font?: { family: string; weight: number };
  publicName?: string;
  provenance?: { url: string; license: string; maps: string };
}
export interface Variant {
  url: string;
  width: number;
  height: number;
}
export interface Asset {
  variants: Variant[];
}
interface Cached {
  key: string;
  asset: Asset;
  outputs: Record<string, string>;
}
interface State {
  entries: Record<string, Cached>;
}
export const repoRoot = path.resolve(import.meta.dirname, "../../..");
const hash = (data: string | Buffer) => createHash("sha256").update(data).digest("hex");
const raster = /\.(png|jpe?g|webp|gif|avif)$/i;
const arrayRaster = /\.(png|jpe?g|webp)$/i;
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export function safePath(root: string, relative: string): string {
  const target = path.resolve(root, relative);
  if (!relative || path.isAbsolute(relative) || !target.startsWith(root + path.sep)) {
    throw new Error(`Unsafe asset path: ${relative}`);
  }
  return target;
}
async function readJson<T>(file: string, fallback?: T): Promise<T> {
  try {
    return JSON.parse(await fs.readFile(file, "utf8")) as T;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT" && fallback !== undefined) {
      return fallback;
    }
    throw error;
  }
}
async function atomicWrite(file: string, data: string | Buffer) {
  await fs.mkdir(path.dirname(file), { recursive: true });
  try {
    if ((await fs.readFile(file)).equals(Buffer.from(data))) {
      return;
    }
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") {
      throw error;
    }
  }
  const temp = `${file}.${process.pid}.tmp`;
  await fs.writeFile(temp, data);
  await fs.rename(temp, file);
}
async function lock(cache: string): Promise<() => Promise<void>> {
  await fs.mkdir(cache, { recursive: true });
  const file = path.join(cache, "lock");
  for (let attempt = 0; attempt < 2400; attempt++) {
    try {
      const handle = await fs.open(file, "wx");
      await handle.writeFile(String(process.pid));
      await handle.close();
      return () => fs.unlink(file);
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "EEXIST") {
        throw error;
      }
      const owner = Number(await fs.readFile(file, "utf8").catch(() => ""));
      if (owner > 0) {
        try {
          process.kill(owner, 0);
        } catch (e) {
          if ((e as NodeJS.ErrnoException).code === "ESRCH") {
            await fs.unlink(file).catch(() => {});
          }
        }
      }
      await sleep(100);
    }
  }
  throw new Error("Timed out waiting for asset generation");
}
export async function loadCatalog(root: string) {
  const catalog = await readJson<{ version: number; assets: Entry[] }>(
    path.join(root, "assets/catalog.json"),
  );
  const profiles = await readJson<Record<string, Profile>>(path.join(root, "assets/profiles.json"));
  if (catalog.version !== 1 || !Array.isArray(catalog.assets)) {
    throw new Error("Invalid asset catalog");
  }
  const ids = new Set<string>();
  const destinations = new Set<string>();
  for (const entry of catalog.assets) {
    if (!/^[a-zA-Z0-9-]+(?:\/[a-zA-Z0-9-]+)+$/.test(entry.id) || ids.has(entry.id)) {
      throw new Error(`Invalid or duplicate asset ID: ${entry.id}`);
    }
    ids.add(entry.id);
    if (typeof entry.source === "string") {
      safePath(path.join(root, "assets/original"), entry.source);
    } else if (!Array.isArray(entry.layers)) {
      throw new Error(`Missing source or layers for ${entry.id}`);
    }
    if (entry.source !== undefined && entry.layers !== undefined) {
      throw new Error(`Ambiguous source and layers for ${entry.id}`);
    }
    const profile = profiles[entry.profile];
    if (
      !profile ||
      (!profile.copy &&
        !profile.gltf &&
        !profile.environment &&
        !profile.textureArray &&
        (!profile.sizes?.length || profile.sizes.some((n) => !Number.isInteger(n) || n < 1)))
    ) {
      throw new Error(`Invalid profile for ${entry.id}`);
    }
    if (
      profile.gltf &&
      (!/\.glb$/i.test(entry.source ?? "") ||
        !Number.isInteger(profile.gltf.maxTextureSize) ||
        profile.gltf.maxTextureSize < 1)
    ) {
      throw new Error(`Invalid model profile for ${entry.id}`);
    }
    if (
      profile.environment &&
      (!/\.exr$/i.test(entry.source ?? "") || !validEnvironment(profile.environment))
    ) {
      throw new Error(`Invalid environment profile for ${entry.id}`);
    }
    if (
      Boolean(profile.textureArray) !== Boolean(entry.layers) ||
      (profile.textureArray &&
        (!Number.isInteger(profile.textureArray.size) ||
          profile.textureArray.size < 16 ||
          profile.textureArray.size > 1024 ||
          (profile.textureArray.sizes &&
            (!profile.textureArray.sizes.length ||
              profile.textureArray.sizes.length > 4 ||
              new Set(profile.textureArray.sizes).size !== profile.textureArray.sizes.length ||
              !profile.textureArray.sizes.includes(profile.textureArray.size) ||
              profile.textureArray.sizes.some(
                (size) =>
                  !Number.isInteger(size) ||
                  size < 16 ||
                  size > profile.textureArray!.size ||
                  (size & (size - 1)) !== 0,
              ))) ||
          (profile.textureArray.size & (profile.textureArray.size - 1)) !== 0 ||
          typeof profile.textureArray.srgb !== "boolean" ||
          !entry.layers?.length ||
          entry.layers.length > 32 ||
          profile.copy ||
          profile.gltf ||
          profile.environment ||
          profile.sizes ||
          entry.prepare ||
          entry.publicName))
    ) {
      throw new Error(`Invalid texture array profile for ${entry.id}`);
    }
    if (profile.quality !== undefined && (profile.quality < 1 || profile.quality > 100)) {
      throw new Error(`Invalid quality for ${entry.id}`);
    }
    if (
      entry.prepare &&
      (entry.prepare.crop.length !== 4 ||
        entry.prepare.crop.some((n) => !Number.isFinite(n) || n < 0 || n > 1) ||
        entry.prepare.crop[2] === 0 ||
        entry.prepare.crop[3] === 0)
    ) {
      throw new Error(`Invalid crop for ${entry.id}`);
    }
    if (entry.publicName) {
      if (!/^[-a-z0-9]+\.(png|ico)$/.test(entry.publicName) || destinations.has(entry.publicName)) {
        throw new Error(`Invalid or duplicate public output: ${entry.publicName}`);
      }
      destinations.add(entry.publicName);
    }
  }
  const byId = new Map(catalog.assets.map((entry) => [entry.id, entry]));
  for (const entry of catalog.assets) {
    for (const id of entry.layers ?? []) {
      const source = byId.get(id);
      if (!source?.source || !arrayRaster.test(source.source) || source.prepare) {
        throw new Error(`Texture array ${entry.id} requires an unprepared raster source: ${id}`);
      }
    }
  }
  return { entries: catalog.assets, profiles };
}
async function preparedImage(data: Buffer, entry: Entry): Promise<Buffer> {
  if (!entry.prepare) {
    return data;
  }
  const { data: pixels, info } = await sharp(data)
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });
  if (entry.prepare.charcoalMatte) {
    // Bake the existing sRGB SVG color-matrix/composite into the alpha channel.
    for (let i = 0; i < pixels.length; i += 4) {
      const mask = Math.max(
        0,
        Math.min(1, (3 * (pixels[i] + pixels[i + 1] + pixels[i + 2])) / 255 - 0.945),
      );
      pixels[i + 3] = Math.round(pixels[i + 3] * mask);
    }
  }
  const [x, y, w, h] = entry.prepare.crop;
  const left = Math.round(x * info.width);
  const top = Math.round(y * info.height);
  const width = Math.min(info.width - left, Math.round(w * info.width));
  const height = Math.min(info.height - top, Math.round(h * info.height));
  return sharp(pixels, { raw: info }).extract({ left, top, width, height }).png().toBuffer();
}
async function optimizedModel(data: Buffer, profile: Profile): Promise<Buffer> {
  const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
  const document = await io.readBinary(new Uint8Array(data));
  const max = profile.gltf!.maxTextureSize;
  // Renderers look up nodes by name, merge primitives on the CPU and read texture maps, so the
  // scene graph, vertex attributes and geometry precision stay exactly as authored.
  await document.transform(
    dedup(),
    prune({ keepLeaves: true, keepAttributes: true, keepSolidTextures: true }),
    textureCompress({
      encoder: sharp,
      // Core glTF embeds PNG/JPEG directly. This matches the sibling Rust
      // pipeline's portable GLB output and avoids EXT_texture_webp at runtime.
      targetFormat: "png",
      resize: [max, max],
    }),
  );
  return Buffer.from(await io.writeBinary(document));
}
function outputName(
  entry: Entry,
  width: number,
  bytes: Buffer,
  extension: string,
  sized: boolean,
): string {
  if (entry.publicName) {
    return entry.publicName;
  }
  const parts = entry.id.split("/");
  const filename = `${parts.pop()}.${hash(bytes).slice(0, 16)}${extension}`;
  if (sized) {
    parts.splice(parts[0] === "textures" ? 2 : 1, 0, String(width));
  }
  return ["assets", ...parts, filename].join("/");
}
export async function buildAssets(root = repoRoot, verifyOnly = false) {
  const cache = path.join(root, "frontend/.cache/assets");
  const release = await lock(cache);
  try {
    const { entries, profiles } = await loadCatalog(root);
    const publicDir = path.join(root, "frontend/public");
    const previous = await readJson<State>(path.join(cache, "state.json"), { entries: {} });
    const next: State = { entries: {} };
    const registry: Record<string, Asset> = {};
    const toolVersion =
      hash(await fs.readFile(path.join(repoRoot, "frontend/scripts/assets/pipeline.ts"))) +
      JSON.stringify(sharp.versions) +
      gltfVersion;
    const nativeVersion = entries.some(
      (entry) => profiles[entry.profile].environment || profiles[entry.profile].textureArray,
    )
      ? hash(await assetToolVersion(repoRoot))
      : "";
    const byId = new Map(entries.map((entry) => [entry.id, entry]));
    let built = 0;
    let cursor = 0;
    const fixedOutputs: Array<[string, Buffer]> = [];
    const processEntry = async (entry: Entry) => {
      const profile = profiles[entry.profile];
      const names = entry.layers?.map((id) => byId.get(id)!.source!) ?? [entry.source!];
      const sources = names.map((name) => safePath(path.join(root, "assets/original"), name));
      const source = sources[0];
      const bytes = entry.layers ? Buffer.alloc(0) : await fs.readFile(source);
      if (
        bytes.subarray(0, 100).toString().startsWith("version https://git-lfs.github.com/spec/v1")
      ) {
        throw new Error(`Unresolved Git LFS source: ${names[0]}. Run git lfs pull.`);
      }
      const bindings: { path: string; sha256: string }[] = [];
      if (entry.layers) {
        for (const [index, file] of sources.entries()) {
          if ((await fs.stat(file)).size > 64 * 1024 * 1024)
            throw new Error(`Texture array source exceeds byte bound: ${names[index]}`);
          const digest = createHash("sha256");
          let prefix = "";
          for await (const chunk of createReadStream(file)) {
            if (!prefix) prefix = chunk.subarray(0, 100).toString();
            digest.update(chunk);
          }
          if (prefix.startsWith("version https://git-lfs.github.com/spec/v1"))
            throw new Error(`Unresolved Git LFS source: ${names[index]}. Run git lfs pull.`);
          bindings.push({ path: file, sha256: digest.digest("hex") });
        }
      }
      const key = hash(
        JSON.stringify({
          sources: entry.layers ? bindings.map((binding) => binding.sha256) : [hash(bytes)],
          entry,
          profile,
          toolVersion,
          nativeVersion: profile.environment || profile.textureArray ? nativeVersion : undefined,
        }),
      );
      const cached = previous.entries[entry.id];
      let valid = cached?.key === key;
      if (valid) {
        for (const [relative, digest] of Object.entries(cached.outputs)) {
          const output = await fs.readFile(safePath(publicDir, relative)).catch(() => null);
          if (!output || hash(output) !== digest) {
            valid = false;
            break;
          }
        }
      }
      if (valid) {
        next.entries[entry.id] = cached;
        registry[entry.id] = cached.asset;
        return;
      }
      if (verifyOnly) {
        throw new Error(`Asset needs generation: ${entry.id}`);
      }
      const outputs: Record<string, string> = {};
      const variants: Variant[] = [];
      const publish = async (
        data: Buffer,
        width: number,
        height: number,
        extension: string,
        bucket: number,
        sized: boolean,
      ) => {
        const relative = outputName(entry, bucket, data, extension, sized);
        const target = safePath(publicDir, relative);
        if (entry.publicName) {
          fixedOutputs.push([target, data]);
        } else {
          await atomicWrite(target, data);
        }
        outputs[relative] = hash(data);
        variants.push({ url: "/" + relative, width, height });
      };
      if (profile.textureArray) {
        const { srgb } = profile.textureArray;
        const sizes = [...(profile.textureArray.sizes ?? [profile.textureArray.size])].sort(
          (a, b) => a - b,
        );
        for (const size of sizes)
          await publish(
            await prepareTextureArray(repoRoot, bindings, size, srgb),
            size,
            size,
            ".ktx2",
            size,
            sizes.length > 1,
          );
      } else if (profile.copy) {
        let width = 0;
        let height = 0;
        if (raster.test(entry.source!)) {
          const m = await sharp(bytes).metadata();
          width = m.width ?? 0;
          height = m.height ?? 0;
        }
        const isPlanet = entry.id.startsWith("textures/planets/");
        await publish(bytes, width, height, path.extname(entry.source!), width, isPlanet);
      } else if (profile.gltf) {
        await publish(await optimizedModel(bytes, profile), 0, 0, ".glb", 0, false);
      } else if (profile.environment) {
        const sizes = [...(profile.environment.faceSizes ?? [profile.environment.faceSize])].sort(
          (a, b) => a - b,
        );
        for (const size of sizes)
          await publish(
            await prepareEnvironment(
              repoRoot,
              source,
              entry.id,
              { ...profile.environment, faceSize: size },
              hash(bytes),
            ),
            size,
            size,
            ".ktx2",
            size,
            sizes.length > 1,
          );
      } else {
        const prepared = await preparedImage(bytes, entry);
        const metadata = await sharp(prepared).metadata();
        const sourceWidth = metadata.width!;
        const sourceHeight = metadata.height!;
        const usedSizes = new Set<string>();
        for (const size of [...profile.sizes!].sort((a, b) => a - b)) {
          const factor = Math.min(1, size / Math.max(sourceWidth, sourceHeight));
          const width = Math.max(1, Math.round(sourceWidth * factor));
          const height = Math.max(1, Math.round(sourceHeight * factor));
          const dimensions = `${width}x${height}`;
          if (usedSizes.has(dimensions)) {
            continue;
          }
          usedSizes.add(dimensions);
          const data =
            profile.copyMatchingWebp &&
            metadata.format === "webp" &&
            width === sourceWidth &&
            height === sourceHeight
              ? prepared
              : await sharp(prepared)
                  .resize(width, height)
                  .webp({
                    lossless: profile.lossless ?? false,
                    quality: profile.quality ?? 90,
                    effort: 4,
                  })
                  .toBuffer();
          await publish(data, width, height, ".webp", size, true);
        }
      }
      const asset = { variants };
      registry[entry.id] = asset;
      next.entries[entry.id] = { key, asset, outputs };
      built++;
    };
    const workers = Array.from({ length: 3 }, async () => {
      while (cursor < entries.length) {
        await processEntry(entries[cursor++]);
      }
    });
    const results = await Promise.allSettled(workers);
    for (const result of results) {
      if (result.status === "rejected") {
        throw result.reason;
      }
    }
    const sorted = Object.fromEntries(
      Object.entries(registry).sort(([a], [b]) => a.localeCompare(b)),
    );
    const corporations = Object.fromEntries(
      entries
        .filter((e) => e.corporation)
        .map((e) => [e.corporation!.key, { id: e.id, ...e.corporation! }]),
    );
    const generated = path.join(root, "frontend/src/assets/generated");
    const ts = `// Generated by the asset pipeline.\nexport type AssetId = ${Object.keys(sorted)
      .map((id) => JSON.stringify(id))
      .join(
        " | ",
      )};\nexport interface AssetVariant { url: string; width: number; height: number }\nexport interface Asset { variants: AssetVariant[] }\nexport const assets: Record<AssetId, Asset> = ${JSON.stringify(sorted, null, 2)};\nexport const corporations: Record<string, { id: AssetId; key: string; name: string; color: string }> = ${JSON.stringify(corporations, null, 2)};\n`;
    const css =
      entries
        .filter((e) => e.font)
        .map(
          (e) =>
            `@font-face { font-family: "${e.font!.family}"; src: url("${registry[e.id].variants[0].url}") format("truetype"); font-weight: ${e.font!.weight}; font-style: normal; font-display: swap; }`,
        )
        .join("\n") + "\n";
    if (verifyOnly) {
      if (
        (await fs.readFile(path.join(generated, "registry.ts"), "utf8")) !== ts ||
        (await fs.readFile(path.join(generated, "fonts.css"), "utf8")) !== css
      ) {
        throw new Error("Generated registry or font CSS is stale");
      }
    } else {
      for (const [file, data] of fixedOutputs) {
        await atomicWrite(file, data);
      }
      await atomicWrite(path.join(generated, "registry.ts"), ts);
      await atomicWrite(path.join(generated, "fonts.css"), css);
      const owned = new Set(Object.values(next.entries).flatMap((e) => Object.keys(e.outputs)));
      for (const old of Object.values(previous.entries)) {
        for (const relative of Object.keys(old.outputs)) {
          if (!owned.has(relative)) {
            await fs.rm(safePath(publicDir, relative), { force: true });
          }
        }
      }
      await atomicWrite(path.join(cache, "state.json"), JSON.stringify(next));
    }
    return { total: entries.length, built };
  } finally {
    await release();
  }
}
