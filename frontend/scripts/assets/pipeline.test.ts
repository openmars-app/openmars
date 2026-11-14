import { afterEach, expect, test } from "bun:test";
import { Document, NodeIO } from "@gltf-transform/core";
import { ALL_EXTENSIONS } from "@gltf-transform/extensions";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import sharp from "sharp";
import { inflateSync } from "node:zlib";
import { buildAssets, loadCatalog, safePath, type Variant } from "./pipeline.ts";

const roots: string[] = [];
async function fixture() {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "openmars-assets-test-"));
  roots.push(root);
  await fs.mkdir(path.join(root, "assets/original"), { recursive: true });
  const image = await sharp({
    create: {
      width: 80,
      height: 40,
      channels: 4,
      background: { r: 220, g: 20, b: 30, alpha: 0.5 },
    },
  })
    .png()
    .toBuffer();
  await fs.writeFile(path.join(root, "assets/original/test.png"), image);
  await fs.writeFile(
    path.join(root, "assets/catalog.json"),
    JSON.stringify({
      version: 1,
      assets: [{ id: "icons/test", source: "test.png", profile: "icon" }],
    }),
  );
  await fs.writeFile(
    path.join(root, "assets/profiles.json"),
    JSON.stringify({ icon: { sizes: [32, 64, 128, 256], lossless: true } }),
  );
  return root;
}
async function state(root: string) {
  return JSON.parse(
    await fs.readFile(path.join(root, "frontend/.cache/assets/state.json"), "utf8"),
  );
}
afterEach(async () => {
  for (const root of roots.splice(0)) {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test("graphics tiers publish smaller cube and array variants with real memory reductions", async () => {
  const root = await fixture();
  await fs.copyFile(
    path.join(import.meta.dirname, "fixtures/environment.exr"),
    path.join(root, "assets/original/tiny.exr"),
  );
  const profiles = {
    icon: { sizes: [32], lossless: true },
    array: { textureArray: { size: 32, sizes: [16, 32], srgb: true } },
    sky: {
      environment: {
        encoding: "rgb9e5-ufloat",
        faceSize: 4,
        faceSizes: [2, 4],
        recipe: "ordinary-linear",
        samples: 16,
      },
    },
  };
  await fs.writeFile(path.join(root, "assets/profiles.json"), JSON.stringify(profiles));
  const catalog = {
    version: 1,
    assets: [
      { id: "icons/test", source: "test.png", profile: "icon" },
      { id: "textures/array", layers: ["icons/test"], profile: "array" },
      { id: "textures/sky", source: "tiny.exr", profile: "sky" },
    ],
  };
  await fs.writeFile(path.join(root, "assets/catalog.json"), JSON.stringify(catalog));
  await buildAssets(root);
  const result = await state(root);
  for (const [id, sizes] of [
    ["textures/array", [16, 32]],
    ["textures/sky", [2, 4]],
  ] as const) {
    const variants = result.entries[id].asset.variants;
    expect(variants.map((v: Variant) => v.width)).toEqual(sizes);
    const memory = [];
    for (const variant of variants) {
      const bytes = await fs.readFile(path.join(root, "frontend/public", variant.url));
      expect(bytes.readUInt32LE(20)).toBe(variant.width);
      expect(bytes.readUInt32LE(40)).toBe(Math.log2(variant.width) + 1);
      memory.push(Number(bytes.readBigUInt64LE(96)));
    }
    expect(memory[1] / memory[0]).toBe(4);
  }
  expect((await buildAssets(root)).built).toBe(0);
  profiles.sky.environment.faceSizes = [3, 4];
  await fs.writeFile(path.join(root, "assets/profiles.json"), JSON.stringify(profiles));
  await expect(loadCatalog(root)).rejects.toThrow("Invalid environment profile");
});

test("prepared texture arrays preserve order and alpha, cache dependencies and prune replaced outputs", async () => {
  const root = await fixture();
  const original = path.join(root, "assets/original/second.png");
  const writeColor = async (r: number, g: number, b: number) => {
    await sharp({
      create: { width: 16, height: 16, channels: 4, background: { r, g, b, alpha: 1 } },
    })
      .png()
      .toFile(original);
  };
  await writeColor(20, 40, 220);
  const catalogPath = path.join(root, "assets/catalog.json");
  const catalog = JSON.parse(await fs.readFile(catalogPath, "utf8"));
  catalog.assets.push(
    { id: "textures/second", source: "second.png", profile: "copy" },
    { id: "textures/array", layers: ["icons/test", "textures/second"], profile: "array" },
  );
  await fs.writeFile(catalogPath, JSON.stringify(catalog));
  const profilesPath = path.join(root, "assets/profiles.json");
  const profiles = JSON.parse(await fs.readFile(profilesPath, "utf8"));
  profiles.copy = { copy: true };
  profiles.array = { textureArray: { size: 16, srgb: true } };
  await fs.writeFile(profilesPath, JSON.stringify(profiles));
  expect((await buildAssets(root)).built).toBe(3);
  const getArray = async () => {
    const current = await state(root);
    const url = current.entries["textures/array"].asset.variants[0].url;
    const file = path.join(root, "frontend/public", url);
    const bytes = await fs.readFile(file);
    expect(bytes.readUInt32LE(12)).toBe(43); // VK_FORMAT_R8G8B8A8_SRGB
    expect(bytes.readUInt32LE(32)).toBe(2);
    expect(bytes.readUInt32LE(40)).toBe(5);
    expect(bytes.readUInt32LE(44)).toBe(3); // lossless KTX2 zlib
    const offset = Number(bytes.readBigUInt64LE(80));
    const length = Number(bytes.readBigUInt64LE(88));
    return { file, pixels: inflateSync(bytes.subarray(offset, offset + length)) };
  };
  const first = await getArray();
  expect([...first.pixels.subarray(0, 4)]).toEqual([220, 20, 30, 128]);
  expect([...first.pixels.subarray(16 * 16 * 4, 16 * 16 * 4 + 4)]).toEqual([20, 40, 220, 255]);
  expect((await buildAssets(root)).built).toBe(0);
  await writeColor(50, 60, 70);
  await expect(buildAssets(root, true)).rejects.toThrow("needs generation");
  expect((await buildAssets(root)).built).toBe(2);
  const second = await getArray();
  expect(second.file).not.toBe(first.file);
  expect(await fs.stat(first.file).catch(() => null)).toBeNull();
  catalog.assets.at(-1).layers.reverse();
  await fs.writeFile(catalogPath, JSON.stringify(catalog));
  expect((await buildAssets(root)).built).toBe(1);
  const reordered = await getArray();
  expect([...reordered.pixels.subarray(0, 4)]).toEqual([50, 60, 70, 255]);
});

test("texture arrays reject invalid dependency graphs and dimensions", async () => {
  const root = await fixture();
  const catalogPath = path.join(root, "assets/catalog.json");
  const catalog = JSON.parse(await fs.readFile(catalogPath, "utf8"));
  const profilesPath = path.join(root, "assets/profiles.json");
  const profiles = JSON.parse(await fs.readFile(profilesPath, "utf8"));
  profiles.array = { textureArray: { size: 16, srgb: false } };
  await fs.writeFile(profilesPath, JSON.stringify(profiles));
  const array = { id: "textures/array", layers: ["missing/source"], profile: "array" };
  catalog.assets.push(array);
  await fs.writeFile(catalogPath, JSON.stringify(catalog));
  await expect(loadCatalog(root)).rejects.toThrow("requires an unprepared raster");
  array.layers = [array.id];
  await fs.writeFile(catalogPath, JSON.stringify(catalog));
  await expect(loadCatalog(root)).rejects.toThrow("requires an unprepared raster");
  array.layers = Array(33).fill("icons/test");
  await fs.writeFile(catalogPath, JSON.stringify(catalog));
  await expect(loadCatalog(root)).rejects.toThrow("Invalid texture array profile");
  array.layers = ["icons/test"];
  await fs.writeFile(catalogPath, JSON.stringify(catalog));
  profiles.array.textureArray.size = 15;
  await fs.writeFile(profilesPath, JSON.stringify(profiles));
  await expect(loadCatalog(root)).rejects.toThrow("Invalid texture array profile");
});

test("exports retain transparency and aspect ratio without upscaling; unchanged runs are no-ops", async () => {
  const root = await fixture();
  expect((await buildAssets(root)).built).toBe(1);
  const first = await state(root);
  const variants = first.entries["icons/test"].asset.variants;
  expect(variants.map((v: { width: number; height: number }) => [v.width, v.height])).toEqual([
    [32, 16],
    [64, 32],
    [80, 40],
  ]);
  const registry = path.join(root, "frontend/src/assets/generated/registry.ts");
  const mtime = (await fs.stat(registry)).mtimeMs;
  const pixels = await sharp(path.join(root, "frontend/public", variants[0].url))
    .raw()
    .toBuffer();
  expect(pixels[3]).toBe(128);
  expect((await buildAssets(root)).built).toBe(0);
  expect((await fs.stat(registry)).mtimeMs).toBe(mtime);
  expect(await state(root)).toEqual(first);
  await buildAssets(root, true);
});

test("missing and corrupted outputs regenerate; changes replace hashes and prune only owned outputs", async () => {
  const root = await fixture();
  await buildAssets(root);
  const previous = await state(root);
  const output = Object.keys(previous.entries["icons/test"].outputs)[0];
  const full = path.join(root, "frontend/public", output);
  await fs.unlink(full);
  expect((await buildAssets(root)).built).toBe(1);
  await fs.writeFile(full, "corrupt");
  expect((await buildAssets(root)).built).toBe(1);
  const sentinel = path.join(root, "frontend/public/keep.txt");
  await fs.writeFile(sentinel, "unowned");
  await fs.writeFile(
    path.join(root, "assets/profiles.json"),
    JSON.stringify({ icon: { sizes: [20], lossless: true } }),
  );
  expect((await buildAssets(root)).built).toBe(1);
  expect(
    await fs.access(full).then(
      () => true,
      () => false,
    ),
  ).toBe(false);
  expect(await fs.readFile(sentinel, "utf8")).toBe("unowned");
  await fs.writeFile(
    path.join(root, "assets/original/test.png"),
    await sharp({ create: { width: 80, height: 40, channels: 4, background: "blue" } })
      .png()
      .toBuffer(),
  );
  expect((await buildAssets(root)).built).toBe(1);
});

test("failed builds preserve the published registry and previously owned files", async () => {
  const root = await fixture();
  await buildAssets(root);
  const registry = path.join(root, "frontend/src/assets/generated/registry.ts");
  const before = await fs.readFile(registry);
  const previous = await state(root);
  await fs.writeFile(
    path.join(root, "assets/original/test.png"),
    "version https://git-lfs.github.com/spec/v1\noid sha256:123\n",
  );
  await expect(buildAssets(root)).rejects.toThrow("Unresolved Git LFS");
  expect(await fs.readFile(registry)).toEqual(before);
  expect(await state(root)).toEqual(previous);
  for (const file of Object.keys(previous.entries["icons/test"].outputs)) {
    expect(await fs.stat(path.join(root, "frontend/public", file))).toBeTruthy();
  }
  await fs.unlink(path.join(root, "assets/original/test.png"));
  await expect(buildAssets(root)).rejects.toThrow();
});

test("rejects duplicate IDs, output collisions, unsafe paths and invalid profiles", async () => {
  const root = await fixture();
  const file = path.join(root, "assets/catalog.json");
  const initial = JSON.parse(await fs.readFile(file, "utf8"));
  await fs.writeFile(
    file,
    JSON.stringify({ version: 1, assets: [...initial.assets, ...initial.assets] }),
  );
  await expect(buildAssets(root)).rejects.toThrow("duplicate asset ID");
  await fs.writeFile(
    file,
    JSON.stringify({ version: 1, assets: [{ ...initial.assets[0], source: "../secret" }] }),
  );
  await expect(buildAssets(root)).rejects.toThrow("Unsafe asset path");
  await fs.writeFile(
    file,
    JSON.stringify({
      version: 1,
      assets: [
        { ...initial.assets[0], publicName: "test.png" },
        { ...initial.assets[0], id: "icons/second", publicName: "test.png" },
      ],
    }),
  );
  await expect(buildAssets(root)).rejects.toThrow("duplicate public output");
  await fs.writeFile(file, JSON.stringify(initial));
  await fs.writeFile(
    path.join(root, "assets/profiles.json"),
    JSON.stringify({ icon: { sizes: [-1] } }),
  );
  await expect(buildAssets(root)).rejects.toThrow("Invalid profile");
  expect(() => safePath(root, "../outside")).toThrow();
});

test("copies original bytes and serializes overlapping builds", async () => {
  const root = await fixture();
  const profiles = path.join(root, "assets/profiles.json");
  await fs.writeFile(profiles, JSON.stringify({ icon: { copy: true } }));
  const results = await Promise.all([buildAssets(root), buildAssets(root)]);
  expect(results.map((r) => r.built).sort()).toEqual([0, 1]);
  const output = Object.keys((await state(root)).entries["icons/test"].outputs)[0];
  expect(await fs.readFile(path.join(root, "frontend/public", output))).toEqual(
    await fs.readFile(path.join(root, "assets/original/test.png")),
  );
});

test("bakes the charcoal matte into alpha and crops logo margins", async () => {
  const root = await fixture();
  const pixels = Buffer.from([
    16, 20, 25, 255, 255, 255, 255, 255, 220, 30, 10, 255, 16, 20, 25, 255,
  ]);
  await fs.writeFile(
    path.join(root, "assets/original/test.png"),
    await sharp(pixels, { raw: { width: 4, height: 1, channels: 4 } })
      .png()
      .toBuffer(),
  );
  await fs.writeFile(
    path.join(root, "assets/catalog.json"),
    JSON.stringify({
      version: 1,
      assets: [
        {
          id: "icons/test",
          source: "test.png",
          profile: "icon",
          prepare: { crop: [0, 0, 0.75, 1], charcoalMatte: true },
        },
      ],
    }),
  );
  await buildAssets(root);
  const asset = (await state(root)).entries["icons/test"].asset;
  const { data, info } = await sharp(path.join(root, "frontend/public", asset.variants[0].url))
    .raw()
    .toBuffer({ resolveWithObject: true });
  expect(info.width).toBe(3);
  expect(data[3]).toBe(0);
  expect(data[7]).toBe(255);
  expect(data[11]).toBe(255);
});

test("preserves case-sensitive card IDs in the registry and output paths", async () => {
  const root = await fixture();
  await fs.writeFile(
    path.join(root, "assets/catalog.json"),
    JSON.stringify({
      version: 1,
      assets: [{ id: "cards/C01", source: "test.png", profile: "icon" }],
    }),
  );
  await buildAssets(root);
  const entry = (await state(root)).entries["cards/C01"];
  expect(entry.asset.variants[0].url).toContain("/C01.");
  expect(
    await fs.readFile(path.join(root, "frontend/src/assets/generated/registry.ts"), "utf8"),
  ).toContain('"cards/C01"');
});

test("model profiles shrink GLB textures and keep the scene graph and geometry", async () => {
  const root = await fixture();
  const document = new Document();
  const buffer = document.createBuffer();
  const positions = new Float32Array([0, 0, 0, 1.5, 0, 0, 0, 2.25, 0]);
  const position = document.createAccessor().setType("VEC3").setArray(positions).setBuffer(buffer);
  const uv = document
    .createAccessor()
    .setType("VEC2")
    .setArray(new Float32Array([0, 0, 1, 0, 0, 1]))
    .setBuffer(buffer);
  const pixels = Buffer.alloc(256 * 256 * 3);
  for (let i = 0; i < pixels.length; i++) {
    pixels[i] = (i * 7919) % 251;
  }
  const image = await sharp(pixels, { raw: { width: 256, height: 256, channels: 3 } })
    .png()
    .toBuffer();
  const texture = document.createTexture("color").setImage(image).setMimeType("image/png");
  const material = document.createMaterial("leaf").setBaseColorTexture(texture);
  const primitive = document
    .createPrimitive()
    .setAttribute("POSITION", position)
    .setAttribute("TEXCOORD_0", uv)
    .setMaterial(material);
  const mesh = document.createMesh("Tree").addPrimitive(primitive);
  const node = document.createNode("Tree-01-1").setMesh(mesh).setTranslation([1, 2, 3]);
  document.createScene().addChild(node).addChild(document.createNode("Marker"));
  const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
  const source = Buffer.from(await io.writeBinary(document));
  await fs.writeFile(path.join(root, "assets/original/test.glb"), source);
  await fs.writeFile(
    path.join(root, "assets/catalog.json"),
    JSON.stringify({
      version: 1,
      assets: [{ id: "models/test", source: "test.glb", profile: "model" }],
    }),
  );
  await fs.writeFile(
    path.join(root, "assets/profiles.json"),
    JSON.stringify({ model: { gltf: { maxTextureSize: 64 }, quality: 90 } }),
  );
  await buildAssets(root);
  const asset = (await state(root)).entries["models/test"].asset;
  expect(asset.variants[0].url).toMatch(/^\/assets\/models\/test\.[0-9a-f]{16}\.glb$/);
  const output = await fs.readFile(path.join(root, "frontend/public", asset.variants[0].url));
  expect(output.length).toBeLessThan(source.length);
  const optimized = (await io.readBinary(new Uint8Array(output))).getRoot();
  expect(optimized.listNodes().map((n) => n.getName())).toEqual(["Tree-01-1", "Marker"]);
  expect(optimized.listNodes()[0].getTranslation()).toEqual([1, 2, 3]);
  const [optimizedTexture] = optimized.listTextures();
  expect(optimizedTexture.getMimeType()).toBe("image/png");
  expect(
    optimized.listExtensionsRequired().map((extension) => extension.extensionName),
  ).not.toContain("EXT_texture_webp");
  expect(optimizedTexture.getSize()).toEqual([64, 64]);
  const optimizedPosition = optimized.listMeshes()[0].listPrimitives()[0].getAttribute("POSITION")!;
  expect(optimizedPosition.getComponentType()).toBe(5126);
  expect(Array.from(optimizedPosition.getArray()!)).toEqual(Array.from(positions));
  await fs.writeFile(
    path.join(root, "assets/catalog.json"),
    JSON.stringify({
      version: 1,
      assets: [{ id: "models/test", source: "test.png", profile: "model" }],
    }),
  );
  await expect(buildAssets(root)).rejects.toThrow("Invalid model profile");
});

test("environment recipes build linear HDR maps, cache their policy and prune replaced specular maps", async () => {
  const root = await fixture();
  await fs.copyFile(
    path.join(import.meta.dirname, "fixtures/environment.exr"),
    path.join(root, "assets/original/tiny.exr"),
  );
  const profiles = {
    background: {
      environment: {
        encoding: "rgb9e5-ufloat",
        faceSize: 4,
        recipe: "ordinary-linear",
        samples: 16,
      },
    },
    diffuse: {
      environment: {
        encoding: "rgba16-float",
        faceSize: 2,
        recipe: "normalized-irradiance",
        samples: 0,
        diffuseGrid: { width: 4, height: 2 },
      },
    },
    specular: {
      environment: { encoding: "rgba16-float", faceSize: 4, recipe: "ggx", samples: 16 },
    },
  };
  await fs.writeFile(path.join(root, "assets/profiles.json"), JSON.stringify(profiles));
  await fs.writeFile(
    path.join(root, "assets/catalog.json"),
    JSON.stringify({
      version: 1,
      assets: Object.keys(profiles).map((profile) => ({
        id: `environment/${profile}`,
        source: "tiny.exr",
        profile,
      })),
    }),
  );
  expect((await buildAssets(root)).built).toBe(3);
  const current = await state(root);
  for (const [name, levels] of [
    ["background", 3],
    ["diffuse", 1],
    ["specular", 3],
  ] as const) {
    const url = current.entries[`environment/${name}`].asset.variants[0].url;
    const bytes = await fs.readFile(path.join(root, "frontend/public", url));
    expect(bytes.readUInt32LE(12)).toBe(name === "background" ? 123 : 97); // RGB9E5 background; binary16 lighting
    expect(bytes.readUInt32LE(36)).toBe(6);
    expect(bytes.readUInt32LE(40)).toBe(levels);
    const offset = Number(bytes.readBigUInt64LE(80));
    if (name === "background") {
      expect(bytes.readUInt32LE(44)).toBe(3); // lossless KTX2 zlib
      const length = Number(bytes.readBigUInt64LE(88));
      const raw = inflateSync(bytes.subarray(offset, offset + length));
      expect(raw.length).toBe(4 * 4 * 6 * 4);
      const word = raw.readUInt32LE(0),
        scale = 2 ** ((word >>> 27) - 24);
      expect([word & 511, (word >>> 9) & 511, (word >>> 18) & 511].map((m) => m * scale)).toEqual([
        0.125, 0.25, 2,
      ]);
    } else {
      expect([
        bytes.readUInt16LE(offset),
        bytes.readUInt16LE(offset + 2),
        bytes.readUInt16LE(offset + 4),
      ]).toEqual([0x3000, 0x3400, 0x4000]);
    }
  }
  expect((await buildAssets(root)).built).toBe(0);
  const previous = current.entries["environment/specular"].asset.variants[0].url;
  profiles.specular.environment.faceSize = 8;
  await fs.writeFile(path.join(root, "assets/profiles.json"), JSON.stringify(profiles));
  expect((await buildAssets(root)).built).toBe(1);
  expect(
    await fs
      .stat(path.join(root, "frontend/public", previous))
      .then(() => true)
      .catch(() => false),
  ).toBe(false);
  profiles.diffuse.environment.samples = 16;
  await fs.writeFile(path.join(root, "assets/profiles.json"), JSON.stringify(profiles));
  await expect(loadCatalog(root)).rejects.toThrow("Invalid environment profile");
});
