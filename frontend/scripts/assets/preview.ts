import { promises as fs } from "node:fs";
import path from "node:path";
import { buildAssets, loadCatalog, repoRoot } from "./pipeline.ts";

await buildAssets();
const { entries } = await loadCatalog(repoRoot);
const { assets } = await import("../../src/assets/generated/registry.ts");
const escape = (s: string) =>
  s.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;");
const tiles = entries
  .map((entry) => {
    const asset = assets[entry.id as keyof typeof assets];
    const variant = asset.variants.find((v) => v.width >= 256) ?? asset.variants[0];
    const url = "../../frontend/public" + variant.url;
    let visual = `<a href="${escape(url)}">Open runtime file</a>`;
    if (variant.width && /\.(png|jpe?g|webp|avif|gif)$/i.test(variant.url)) {
      visual = `<img src="${escape(url)}" alt="${escape(entry.id)}"><div class="samples">${[24, 48, 96].map((size) => `<img src="${escape(url)}" alt="${size}px" style="width:${size}px;height:${size}px">`).join("")}</div>`;
    } else if (entry.id.startsWith("audio/")) {
      visual = `<audio controls preload="none" src="${escape(url)}"></audio>`;
    }
    const originals = entry.source
      ? [entry.source]
      : entry.layers!.map((id) => entries.find((candidate) => candidate.id === id)!.source!);
    const links = originals
      .map(
        (source, index) =>
          `<a href="../../assets/original/${escape(source)}">${entry.layers ? `Layer ${index + 1}` : "Original"}</a>`,
      )
      .join(" · ");
    return `<article data-family="${entry.id.split("/")[0]}"><h2>${escape(entry.id)}</h2>${visual}<p>${asset.variants.map((v) => `${v.width} × ${v.height}`).join(" · ")}</p>${links}</article>`;
  })
  .join("\n");
const html = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Game assets</title><style>*{box-sizing:border-box}body{margin:24px;background:#101419;color:#eee;font:14px system-ui}header{position:sticky;top:0;background:inherit;padding:16px 0}main{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:16px}article{padding:16px;border:1px solid #60666d;background:var(--sample,#000)}h2{font-size:13px;overflow-wrap:anywhere}img{display:block;width:100%;height:160px;object-fit:contain}.samples{display:flex;align-items:center;gap:16px;margin-top:16px}a{color:#eab86c}select,button{padding:8px;margin-right:12px}audio{max-width:100%}</style><header><h1>Current game assets</h1><select id="family"><option value="">All families</option>${[...new Set(entries.map((e) => e.id.split("/")[0]))].map((f) => `<option>${f}</option>`).join("")}</select><button id="background">Light background</button></header><main>${tiles}</main><script>document.querySelector('#family').onchange=e=>{document.querySelectorAll('article').forEach(a=>a.hidden=!!e.target.value&&a.dataset.family!==e.target.value)};let light=false;document.querySelector('#background').onclick=e=>{light=!light;document.body.style.setProperty('--sample',light?'#ddd':'#000');e.target.textContent=light?'Dark background':'Light background'};</script></html>`;
const output = path.join(repoRoot, "output/asset-preview/index.html");
await fs.mkdir(path.dirname(output), { recursive: true });
await fs.writeFile(output, html);
console.log(output);
