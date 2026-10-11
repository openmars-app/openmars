import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { parseArgs } from "node:util";
import { mkdir, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { createServer } from "vite";
import { cardTargets, resolveTarget, tileTypes } from "../../src/dev/inspection/catalog";
import type { InspectionBridge } from "../../src/dev/inspection/protocol";
import { chromiumLaunchOptions, inspectionDevices, type InspectionDevice } from "./devices";

const { values } = parseArgs({
  options: {
    scene: { type: "string", default: "comparison" },
    card: { type: "string" },
    tile: { type: "string" },
    list: { type: "boolean" },
    open: { type: "boolean" },
    views: { type: "string", default: "gameplay,close" },
    devices: { type: "string", default: "desktop,iphone,android" },
    state: { type: "string", default: "settled" },
    frames: { type: "string", default: "0" },
    seed: { type: "string", default: "42" },
    climate: { type: "string", default: "growing" },
    color: { type: "string", default: "#e43c32" },
    json: { type: "boolean" },
    help: { type: "boolean" },
    output: { type: "string" },
  },
  strict: true,
});
if (values.help) {
  console.log(
    "Usage: just frontend inspect --card ID | --tile TYPE [--open] [--state settled|placement] [--frames 0,30,120] [--views gameplay,close] [--devices desktop,iphone,android] [--scene comparison|structures|neighbors|destruction] [--seed 42] [--climate barren|growing|terraformed] [--color '#e43c32'] [--json] [--output DIR]\nUse --list for supported targets. --open without a target opens the subject selector. Frames are at 60 fps. Captures require visual review.",
  );
  process.exit(0);
}
if (values.list) {
  console.log(
    JSON.stringify(
      {
        tiles: tileTypes,
        cards: cardTargets,
        scenes: ["comparison", "structures", "neighbors", "destruction"],
      },
      null,
      2,
    ),
  );
  process.exit(0);
}
const tile = values.tile ?? (values.scene === "structures" ? "city" : undefined);
const target = values.card || tile ? resolveTarget(values.card, tile) : null;
if (!target && !values.open) {
  throw new Error("Specify --card ID or --tile TYPE, or use --open to browse.");
}
if (!["comparison", "structures", "neighbors", "destruction"].includes(values.scene!)) {
  throw new Error("Unknown scene");
}
if (values.scene === "structures" && values.state !== "settled") {
  throw new Error("Structure styles supports settled state only");
}
const views = values.views!.split(",");
const selectedDevices = values.devices!.split(",") as InspectionDevice[];
const frames = values.frames!.split(",").map(Number);
if (views.some((v) => !["gameplay", "close"].includes(v))) {
  throw new Error("Views must be gameplay or close");
}
if (selectedDevices.some((d) => !(d in inspectionDevices))) {
  throw new Error("Devices must be desktop, iphone or android");
}
if (frames.some((f) => !Number.isSafeInteger(f) || f < 0 || f > 36000)) {
  throw new Error("Frames must be integers from 0 to 36000");
}
if (!Number.isSafeInteger(Number(values.seed))) {
  throw new Error("Seed must be an integer");
}
if (!["settled", "placement"].includes(values.state!)) {
  throw new Error("State must be settled or placement");
}
if (!["barren", "growing", "terraformed"].includes(values.climate!)) {
  throw new Error("Unknown climate preset");
}
if (!/^#[\da-f]{6}$/i.test(values.color!)) {
  throw new Error("Colour must be a six-digit hex value");
}
const frontend = resolve(import.meta.dirname, "../..");
const assets = await promisify(execFile)("node", ["scripts/assets/build.ts"], { cwd: frontend });
process.stderr.write(assets.stdout + assets.stderr);
const run = new Date().toISOString().replaceAll(":", "-");
const output = resolve(
  values.output ??
    `${frontend}/output/inspection/${target?.card ?? target?.tile ?? "browser"}/${run}`,
);
await mkdir(output, { recursive: true });
const captures: object[] = [];
const captureCounts = new Map<string, number>();
const errors: string[] = [];
const browsers: Awaited<ReturnType<typeof inspectionDevices.desktop.browser.launch>>[] = [];
let interrupted = false;
const server = await createServer({
  root: frontend,
  server: { host: "127.0.0.1", port: 0, strictPort: true, origin: "" },
  logLevel: "error",
});
const finish = async () => {
  await Promise.allSettled(browsers.map((b) => b.close()));
  await server.close();
};
process.once("SIGINT", () => {
  interrupted = true;
  void finish();
});
process.once("SIGTERM", () => {
  interrupted = true;
  void finish();
});
try {
  await server.listen();
  const address = server.httpServer!.address();
  if (!address || typeof address === "string") {
    throw new Error("Inspection server did not start");
  }
  const origin = `http://127.0.0.1:${address.port}`;
  for (const name of selectedDevices) {
    const device = inspectionDevices[name];
    const browser = await device.browser.launch({
      ...(device.browser.name() === "chromium" ? chromiumLaunchOptions : {}),
      headless: !values.open,
    });
    browsers.push(browser);
    const context = await browser.newContext(device.context);
    const page = await context.newPage();
    page.setDefaultTimeout(180_000);
    const warnings: string[] = [];
    page.on("console", (event) => {
      if (event.type() === "error") {
        errors.push(`${name}: ${event.text()}`);
      } else if (event.type() === "warning") {
        warnings.push(event.text());
      }
    });
    page.on(
      "pageerror",
      (error) => (
        errors.push(`${name}: ${error.message}`),
        process.stderr.write(`${name}: ${error.message}\n`)
      ),
    );
    page.on("requestfailed", (request) =>
      errors.push(`${name}: ${request.url()} ${request.failure()?.errorText}`),
    );
    page.on("response", (response) => {
      if (response.status() >= 400) {
        errors.push(`${name}: HTTP ${response.status()} ${response.url()}`);
      }
    });
    page.on("websocket", (socket) => {
      const url = new URL(socket.url());
      if (url.host !== new URL(origin).host || url.pathname !== "/") {
        errors.push(`Unexpected socket: ${socket.url()}`);
      }
    });
    await page.route("**/api/**", (route) => {
      errors.push(`Unexpected backend request: ${route.request().url()}`);
      return route.abort();
    });
    const params = new URLSearchParams({
      scene: values.scene!,
      gfx: device.tier,
      state: values.state!,
      seed: values.seed!,
      climate: values.climate!,
      color: values.color!,
      view: views[0],
    });
    if (target) {
      params.set("tile", target.tile);
      if (target.card) {
        params.set("card", target.card);
      }
    }
    if (!values.open) {
      params.set("capture", "1");
    }
    const url = `${origin}/dev/board?${params}`;
    process.stderr.write(`${name}: ${url}\n`);
    await page.goto(url);
    if (values.open) {
      await new Promise<void>((resolveClosed) => {
        browser.on("disconnected", () => resolveClosed());
        page.on("close", () => resolveClosed());
      });
      break;
    }
    await page.waitForFunction(() => {
      const bridge = window.__openMarsInspection;
      if (bridge?.error) {
        throw new Error(bridge.error);
      }
      return bridge?.ready;
    });
    for (const view of views) {
      await page.evaluate(
        (next) => window.__openMarsInspection!.setView(next as "gameplay" | "close"),
        view,
      );
      for (const frame of frames) {
        process.stderr.write(`${name}: ${view}, frame ${frame}\n`);
        await page.evaluate((next) => window.__openMarsInspection!.seek(next), frame);
        const key = `${name}-${view}-${values.state}-f${frame}`;
        const count = captureCounts.get(key) ?? 0;
        captureCounts.set(key, count + 1);
        const suffix = count > 0 ? `-repeat${count}` : "";
        const path = resolve(output, `${key}${suffix}.png`);
        await page.screenshot({ path });
        const state = await page.evaluate(() => window.__openMarsInspection!.report());
        captures.push({
          device: name,
          browser: browser.browserType().name(),
          browserVersion: browser.version(),
          emulation: name === "iphone" ? "iPhone landscape using Chromium, not WebKit" : name,
          screenshot: path,
          ...state,
          warnings,
        });
      }
    }
    await browser.close();
  }
} catch (reason) {
  if (!interrupted) {
    errors.push(reason instanceof Error ? (reason.stack ?? reason.message) : String(reason));
  }
} finally {
  await finish();
}
const report = {
  target,
  captureSucceeded: !values.open && captures.length > 0 && errors.length === 0,
  visualReview: "required",
  output,
  captures,
  errors,
};
await writeFile(resolve(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
if (values.json) {
  console.log(JSON.stringify(report));
} else {
  let message = "Capture failed.";
  if (values.open && errors.length === 0) {
    message = "Inspector closed.";
  }
  if (report.captureSucceeded) {
    message = "Capture complete. Open and inspect the images.";
  }
  console.log(`${message}\n${resolve(output, "report.json")}`);
}
if (errors.length) {
  process.stderr.write(errors.join("\n") + "\n");
  process.exitCode = 1;
}
export type { InspectionBridge };

if (interrupted) {
  process.exitCode = 130;
}
