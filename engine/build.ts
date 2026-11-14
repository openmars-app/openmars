import { existsSync } from "node:fs";
import { mkdir } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";

const root = import.meta.dir;
const engineProfile = process.env.OPENMARS_ENGINE_PROFILE ?? "release";
if (engineProfile !== "release" && engineProfile !== "profiling") {
  throw new Error("OPENMARS_ENGINE_PROFILE must be release or profiling");
}
const env = { ...process.env, PATH: `${path.join(homedir(), ".cargo/bin")}:${process.env.PATH}` };
async function run(command: string, args: string[]) {
  const child = Bun.spawn([command, ...args], {
    cwd: root,
    env,
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
  });
  const code = await child.exited;
  if (code !== 0) process.exit(code);
}

const bindgen = path.join(root, ".tools/bin/wasm-bindgen");
if (!existsSync(bindgen)) {
  await run("cargo", [
    "install",
    "--locked",
    "--root",
    ".tools",
    "--version",
    "0.2.129",
    "wasm-bindgen-cli",
  ]);
}
await run("cargo", [
  "run",
  "--quiet",
  "--locked",
  "-p",
  "openmars-scene",
  "--features",
  "export-types",
  "--bin",
  "export-types",
]);
for (const backend of ["webgpu", "webgl2"]) {
  await run("cargo", [
    "build",
    "--locked",
    "--profile",
    engineProfile,
    "--target",
    "wasm32-unknown-unknown",
    "--target-dir",
    `target/${backend}`,
    "--no-default-features",
    "--features",
    backend,
  ]);
  const output = path.join(root, "pkg", backend);
  await mkdir(output, { recursive: true });
  await run(bindgen, [
    "--target",
    "web",
    "--out-dir",
    output,
    "--out-name",
    "openmars_engine",
    ...(engineProfile === "profiling" ? ["--keep-debug"] : []),
    `target/${backend}/wasm32-unknown-unknown/${engineProfile}/openmars_engine.wasm`,
  ]);
}

await run("cargo", [
  "build",
  "--locked",
  "-p",
  "openmars-terrain-worker",
  "--profile",
  engineProfile,
  "--target",
  "wasm32-unknown-unknown",
  "--target-dir",
  "target/preparation",
]);
await mkdir(path.join(root, "pkg/preparation"), { recursive: true });
await run(bindgen, [
  "--target",
  "web",
  "--out-dir",
  path.join(root, "pkg/preparation"),
  "--out-name",
  "terrain_worker",
  `target/preparation/wasm32-unknown-unknown/${engineProfile}/openmars_terrain_worker.wasm`,
]);
