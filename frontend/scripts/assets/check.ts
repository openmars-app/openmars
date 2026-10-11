import { promises as fs } from "node:fs";
import path from "node:path";
import { buildAssets, loadCatalog, repoRoot } from "./pipeline.ts";

await buildAssets(repoRoot, true);
const { entries } = await loadCatalog(repoRoot);
const ids = new Set(entries.map((entry) => entry.id));
const cards = JSON.parse(
  await fs.readFile(path.join(repoRoot, "backend/assets/cards.json"), "utf8"),
) as Array<{ id: string; name: string; type: string }>;
for (const card of cards) {
  const key = card.name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  const id = card.type === "corporation" ? `corporations/${key}` : `cards/${card.id}`;
  if (!ids.has(id)) {
    throw new Error(`Missing card artwork: ${id}`);
  }
}
const modes = JSON.parse(
  await fs.readFile(path.join(repoRoot, "backend/assets/gamemodes.json"), "utf8"),
) as Array<{
  id: string;
  runtime?: {
    deadlines?: Array<{ warningSoundId?: string }>;
    presentationRules?: Array<{ soundId?: string }>;
  };
}>;
for (const mode of modes) {
  const sounds = [
    ...(mode.runtime?.deadlines ?? []).map((deadline) => deadline.warningSoundId),
    ...(mode.runtime?.presentationRules ?? []).map((rule) => rule.soundId),
  ];
  for (const soundId of sounds) {
    if (soundId && !ids.has(`audio/effects/${soundId}`)) {
      throw new Error(`Unknown announcement sound ${soundId} in mode ${mode.id}`);
    }
  }
}
const originalDir = path.join(repoRoot, "assets/original");
const sources = new Set(entries.map((entry) => entry.source));
for (const relative of await fs.readdir(originalDir, { recursive: true })) {
  if ((await fs.stat(path.join(originalDir, relative))).isFile() && !sources.has(relative)) {
    throw new Error(`Uncatalogued original: ${relative}`);
  }
}
const src = path.join(repoRoot, "frontend/src");
for (const relative of await fs.readdir(src, { recursive: true })) {
  if (!/\.(tsx?|jsx?|css)$/.test(relative) || relative.startsWith("assets/generated/")) {
    continue;
  }
  const text = await fs.readFile(path.join(src, relative), "utf8");
  if (/["'`]\/(?:assets|sounds|fonts)\//.test(text)) {
    throw new Error(`Hardcoded media URL: ${relative}`);
  }
}
console.log(
  `Validated ${entries.length} assets, all card identities, sources, and generated outputs.`,
);
