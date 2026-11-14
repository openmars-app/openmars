import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { promises as fs } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";

let tool: { version: string; ready: Promise<string> } | undefined;
async function run(command: string, args: string[], cwd: string) {
  await new Promise<void>((resolve, reject) => {
    const child = spawn(command, args, {
      cwd,
      env: { ...process.env, PATH: `${path.join(homedir(), ".cargo/bin")}:${process.env.PATH}` },
      stdio: ["ignore", "ignore", "inherit"],
    });
    child.on("error", reject);
    child.on("exit", (code) =>
      code === 0 ? resolve() : reject(new Error(`Asset preparation tool failed (${code})`)),
    );
  });
}

export async function prepareAssetTool(root: string) {
  if (process.env.OPENMARS_ASSET_TOOL) {
    return process.env.OPENMARS_ASSET_TOOL;
  }
  const version = createHash("sha256")
    .update(await assetToolVersion(root))
    .digest("hex");
  if (tool?.version !== version) {
    tool = {
      version,
      ready: (async () => {
        await run(
          "cargo",
          ["build", "--locked", "--release", "-p", "openmars-engine-assets"],
          path.join(root, "engine"),
        );
        return path.join(root, "engine/target/release/openmars-engine-assets");
      })(),
    };
  }
  try {
    return await tool.ready;
  } catch (error) {
    tool = undefined;
    throw error;
  }
}

export async function assetToolVersion(root: string): Promise<Buffer> {
  const directory = path.join(root, "engine/asset-pipeline");
  const names = (await fs.readdir(directory, { recursive: true }))
    .filter((name) => /\.(rs|toml)$/.test(name))
    .sort();
  return Buffer.concat(
    await Promise.all([
      fs.readFile(path.join(root, "engine/Cargo.lock")),
      fs.readFile(path.join(root, "engine/Cargo.toml")),
      fs.readFile(path.join(root, "engine/rust-toolchain.toml")),
      fs.readFile(path.join(root, "frontend/scripts/assets/environment.ts")),
      ...names.map((name) => fs.readFile(path.join(directory, name))),
    ]),
  );
}

export interface EnvironmentSpec {
  encoding: "rgba16-float" | "rgb9e5-ufloat";
  faceSize: number;
  faceSizes?: number[];
  recipe: "ordinary-linear" | "normalized-irradiance" | "ggx";
  samples: number;
  diffuseGrid?: { width: number; height: number };
}
export function validEnvironment(spec: EnvironmentSpec): boolean {
  const size = spec.faceSize;
  const maximum =
    spec.encoding === "rgb9e5-ufloat" && spec.recipe === "ordinary-linear" ? 2048 : 1024;
  if (!Number.isInteger(size) || size < 2 || size > maximum || (size & (size - 1)) !== 0)
    return false;
  if (
    spec.faceSizes &&
    (!spec.faceSizes.length ||
      spec.faceSizes.length > 4 ||
      new Set(spec.faceSizes).size !== spec.faceSizes.length ||
      !spec.faceSizes.includes(size) ||
      spec.faceSizes.some(
        (value) =>
          !Number.isInteger(value) || value < 2 || value > size || (value & (value - 1)) !== 0,
      ))
  )
    return false;
  if (
    !["rgba16-float", "rgb9e5-ufloat"].includes(spec.encoding) ||
    (spec.encoding === "rgb9e5-ufloat" && spec.recipe !== "ordinary-linear")
  )
    return false;
  if (spec.recipe === "normalized-irradiance") {
    const grid = spec.diffuseGrid;
    return (
      spec.samples === 0 &&
      !!grid &&
      Number.isInteger(grid.width) &&
      Number.isInteger(grid.height) &&
      grid.width >= 1 &&
      grid.width <= 256 &&
      grid.height >= 1 &&
      grid.height <= 128
    );
  }
  return (
    (spec.recipe === "ordinary-linear" || spec.recipe === "ggx") &&
    spec.diffuseGrid === undefined &&
    Number.isInteger(spec.samples) &&
    spec.samples >= 1 &&
    spec.samples <= 16384 &&
    (spec.recipe !== "ordinary-linear" || spec.samples === 16)
  );
}

export async function prepareEnvironment(
  root: string,
  source: string,
  id: string,
  spec: EnvironmentSpec,
  sha256: string,
): Promise<Buffer> {
  const executable = await prepareAssetTool(root);
  const cache = path.join(root, "frontend/.cache/assets");
  await fs.mkdir(cache, { recursive: true });
  const temporary = await fs.mkdtemp(path.join(cache, "environment-"));
  try {
    const output = path.join(temporary, "cube.ktx2");
    const request = path.join(temporary, "request.json");
    await fs.writeFile(
      request,
      JSON.stringify({
        source,
        binding: { sha256, absent_chromaticities_are_linear_bt709: true },
        cube: {
          id,
          face_size: spec.faceSize,
          encoding: spec.encoding,
          levels: spec.recipe === "normalized-irradiance" ? 1 : Math.log2(spec.faceSize) + 1,
          recipe: spec.recipe,
          samples: spec.samples,
          diffuse_grid: spec.diffuseGrid ?? null,
        },
      }),
    );
    await run(executable, ["environment", request, output], root);
    return await fs.readFile(output);
  } finally {
    await fs.rm(temporary, { recursive: true, force: true });
  }
}

export async function prepareTextureArray(
  root: string,
  sources: { path: string; sha256: string }[],
  size: number,
  srgb: boolean,
): Promise<Buffer> {
  const executable = await prepareAssetTool(root);
  await fs.mkdir(path.join(root, "frontend/.cache/assets"), { recursive: true });
  const temporary = await fs.mkdtemp(path.join(root, "frontend/.cache/assets/array-"));
  try {
    const request = path.join(temporary, "request.json");
    const output = path.join(temporary, "array.ktx2");
    await fs.writeFile(request, JSON.stringify({ sources, size, srgb }));
    await run(executable, ["texture-array", request, output], root);
    return await fs.readFile(output);
  } finally {
    await fs.rm(temporary, { recursive: true, force: true });
  }
}
