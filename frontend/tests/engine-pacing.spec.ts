import { expect, test, type Page } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import type { EnginePerformance } from "../src/engine/performance";

test.skip(
  !process.env.E2E_BACKEND || !process.env.E2E_SESSION_SAVE,
  "Provide a running Go backend and exported game via E2E_BACKEND/E2E_SESSION_SAVE",
);
test.beforeEach(() => test.skip(!test.info().project.name.startsWith("engine")));
test.use({ actionTimeout: 15_000, trace: { mode: "retain-on-failure", screenshots: false } });

type PacingSample = EnginePerformance & {
  completedFrames?: number;
  pendingFrames?: number;
  frameLimit?: number;
};

// Only this test's fresh browser context is instrumented. Return the original Promise;
// do not add a queue fence, hold a callback, or change the engine's acknowledgement path.
function installQueueObserver() {
  const prototype = (Reflect.get(globalThis, "GPUQueue") as { prototype: GPUQueue } | undefined)
    ?.prototype;
  const descriptor = prototype && Object.getOwnPropertyDescriptor(prototype, "onSubmittedWorkDone");
  const original = prototype?.onSubmittedWorkDone;
  let active = false;
  let outstanding = 0;
  let peak = 0;
  let registrations = 0;
  let completions = 0;
  let rejected = 0;
  let latencies: number[] = [];
  let generation = 0;
  const wrapped = function (this: GPUQueue) {
    const started = performance.now();
    const measured = active;
    const measuredGeneration = generation;
    const promise = original!.call(this);
    outstanding++;
    if (measured) {
      registrations++;
      peak = Math.max(peak, outstanding);
    }
    const settled = (failed: boolean) => {
      outstanding--;
      if (measured && active && generation === measuredGeneration) {
        completions++;
        rejected += Number(failed);
        latencies.push(performance.now() - started);
      }
    };
    void promise.then(
      () => settled(false),
      () => settled(true),
    );
    return promise;
  };
  if (prototype && original) {
    Object.defineProperty(prototype, "onSubmittedWorkDone", {
      configurable: true,
      writable: true,
      value: wrapped,
    });
  }
  Reflect.set(window, "enginePacingProbe", {
    start() {
      generation++;
      active = true;
      peak = outstanding;
      registrations = 0;
      completions = 0;
      rejected = 0;
      latencies = [];
    },
    stop() {
      active = false;
      return {
        available: !!original,
        registrations,
        completions,
        rejected,
        outstandingAtEnd: outstanding,
        outstandingPeak: peak,
        registrationToResolutionMs: latencies.slice(),
      };
    },
    restore() {
      active = false;
      if (prototype && original && prototype.onSubmittedWorkDone === wrapped) {
        if (descriptor) Object.defineProperty(prototype, "onSubmittedWorkDone", descriptor);
        else Reflect.deleteProperty(prototype, "onSubmittedWorkDone");
      }
    },
  });
}

function distribution(values: number[]) {
  // oxlint-disable-next-line unicorn/no-array-sort -- Tests target ES2022; sort a copied array.
  const sorted = values.slice().sort((a, b) => a - b);
  const percentile = (fraction: number) =>
    sorted.length
      ? sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * fraction) - 1)]
      : null;
  return {
    count: sorted.length,
    min: sorted[0] ?? null,
    median: percentile(0.5),
    p95: percentile(0.95),
    p99: percentile(0.99),
    max: sorted.at(-1) ?? null,
    mean: sorted.length ? sorted.reduce((sum, value) => sum + value, 0) / sorted.length : null,
  };
}

async function measurePacing(page: Page, durationMs: number) {
  const captured = await page.evaluate(async (duration) => {
    const module = "/src/stores/engineStore.ts";
    const { useEngineStore } = await import(/* @vite-ignore */ module);
    const source = useEngineStore.getState().performance;
    if (!source) throw new Error("Production Native performance source is unavailable");
    const probe = Reflect.get(window, "enginePacingProbe") as {
      start(): void;
      stop(): {
        available: boolean;
        registrations: number;
        completions: number;
        rejected: number;
        outstandingAtEnd: number;
        outstandingPeak: number;
        registrationToResolutionMs: number[];
      };
    };
    let timedOut = false;
    const nextFrame = () =>
      new Promise<number>((resolve, reject) =>
        requestAnimationFrame((time) => {
          if (timedOut) reject(new Error("Animation-frame measurement deadline elapsed"));
          else resolve(time);
        }),
      );
    source.setActive(true);
    const timeout = window.setTimeout(() => {
      // The outer Playwright timeout also bounds an unresponsive page. This deadline
      // diagnoses a page that remains responsive but does not submit Native frames.
      timedOut = true;
    }, 90_000);
    try {
      const initial = source.sample() as PacingSample | null;
      if (!initial) throw new Error("Native counters are inactive");
      let warmed = initial;
      while (warmed.submittedFrames - initial.submittedFrames < 40) {
        // oxlint-disable-next-line no-await-in-loop -- Warm-up follows successive submitted frames.
        await nextFrame();
        if (timedOut) throw new Error("Native renderer did not submit 40 warm-up frames");
        warmed = source.sample() as PacingSample;
      }
      // Sampling resets the meter's interval; this sample anchors actual frame deltas.
      const start = source.sample() as PacingSample;
      const startTime = performance.now();
      const rafIntervals: number[] = [];
      const samples: (PacingSample & { atMs: number })[] = [];
      let previousRaf: number | null = null;
      let lastSampleTime = startTime;
      probe.start();
      let now = startTime;
      do {
        // oxlint-disable-next-line no-await-in-loop -- Cadence requires successive animation frames.
        const timestamp = await nextFrame();
        if (timedOut) throw new Error("Measurement deadline elapsed");
        if (previousRaf !== null) rafIntervals.push(timestamp - previousRaf);
        previousRaf = timestamp;
        now = performance.now();
        if (now - lastSampleTime >= 1000) {
          const sample = source.sample() as PacingSample | null;
          if (!sample) throw new Error("Native source disappeared during measurement");
          samples.push({ ...sample, atMs: now - startTime });
          lastSampleTime = now;
        }
      } while (now - startTime < duration);
      const endTime = performance.now();
      const end = source.sample() as PacingSample;
      const queue = probe.stop();
      samples.push({ ...end, atMs: endTime - startTime });
      return {
        elapsedMs: endTime - startTime,
        start,
        end,
        warmSubmittedFrames: warmed.submittedFrames - initial.submittedFrames,
        submittedFrames: end.submittedFrames - start.submittedFrames,
        nativeSubmittedFps:
          (1000 * (end.submittedFrames - start.submittedFrames)) / (endTime - startTime),
        rafIntervals,
        samples,
        queue,
      };
    } finally {
      window.clearTimeout(timeout);
      probe.stop();
      source.setActive(false);
    }
  }, durationMs);
  return {
    ...captured,
    uiCadence: {
      intervalMs: distribution(captured.rafIntervals),
      fps:
        captured.rafIntervals.length && captured.rafIntervals.reduce((sum, value) => sum + value, 0)
          ? (1000 * captured.rafIntervals.length) /
            captured.rafIntervals.reduce((sum, value) => sum + value, 0)
          : null,
    },
    cpuMs: distribution(
      captured.samples.flatMap((sample) => (sample.cpuMs === null ? [] : [sample.cpuMs])),
    ),
    acknowledgementLatencyMs: distribution(captured.queue.registrationToResolutionMs),
    nativePendingFrames: captured.samples.flatMap((sample) =>
      sample.pendingFrames === undefined ? [] : [sample.pendingFrames],
    ),
  };
}

test("profiles actual UI cadence Native submissions and queue acknowledgements", async ({
  browser,
  baseURL,
}) => {
  test.setTimeout(360_000);
  const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
  const label = (process.env.E2E_PACING_LABEL ?? "baseline").replace(/[^a-zA-Z0-9_-]/g, "-");
  const duration = Number(process.env.E2E_PACING_MS ?? 8000);
  expect(Number.isFinite(duration) && duration >= 2000 && duration <= 60_000).toBe(true);
  const context = await browser.newContext({
    baseURL,
    viewport: { width: 1280, height: 720 },
    reducedMotion: "reduce",
  });
  await context.addInitScript(installQueueObserver);
  await context.addInitScript((renderer) => {
    const url = new URL(location.href);
    url.searchParams.set("renderer", renderer);
    history.replaceState(history.state, "", url);
  }, backend);
  const page = await context.newPage();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const output = path.resolve("../tmp/bevy-scaffold");
  await mkdir(output, { recursive: true });
  const evidence: Record<string, unknown> = {
    backend,
    label,
    durationMs: duration,
    note:
      "Queue latency measures Promise registration to resolution, not GPU execution time. " +
      "Outstanding counts include existing engine queue acknowledgements, not logical frame depth. " +
      "Native pendingFrames/frameLimit are present only when exposed by the measured build.",
  };
  try {
    await page.goto(`/?renderer=${backend}`);
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      {
        timeout: 120_000,
      },
    );
    await expect(page.getByRole("link", { name: "Load game", exact: true })).toBeVisible();
    evidence.menu = await measurePacing(page, duration);

    await page.getByRole("link", { name: "Load game", exact: true }).click();
    await page
      .getByLabel("Select an exported Game file")
      .setInputFiles(process.env.E2E_SESSION_SAVE!);
    await page.getByLabel("Your name").fill(`Pacing ${backend} ${label}`);
    await page.getByRole("button", { name: "Load game", exact: true }).click();
    await page.getByRole("button", { name: "Resume game", exact: true }).click();
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      {
        timeout: 120_000,
      },
    );
    const received = page.getByRole("heading", { name: "Cards received", exact: true });
    await expect(received).toBeVisible({ timeout: 120_000 });
    await expect
      .poll(
        () =>
          page.evaluate(async () => {
            const ep = "/src/stores/engineStore.ts";
            const { useEngineStore } = await import(/* @vite-ignore */ ep);
            const engine = useEngineStore.getState();
            return engine.status.phase === "ready" && engine.inspectScene?.()?.boardReady;
          }),
        { timeout: 120_000 },
      )
      .toBe(true);
    await expect(page.getByRole("status")).toHaveCount(0, { timeout: 120_000 });
    await page.getByRole("button", { name: "Close", exact: true }).click();
    await expect(received).toHaveCount(0);
    await expect(async () => {
      const current = await page.evaluate(async () => {
        const ep = "/src/stores/engineStore.ts";
        const gp = "/src/stores/gameStore.ts";
        const pp = "/src/stores/appPhaseStore.ts";
        const [{ useEngineStore }, { useGameStore }, { useAppPhaseStore, gameIdOf }] =
          await Promise.all([
            import(/* @vite-ignore */ ep),
            import(/* @vite-ignore */ gp),
            import(/* @vite-ignore */ pp),
          ]);
        const game = useGameStore.getState();
        const engine = useEngineStore.getState();
        return {
          connected: game.isConnected,
          id: game.game?.id,
          phase: useAppPhaseStore.getState().phase.kind,
          phaseGameId: gameIdOf(useAppPhaseStore.getState().phase),
          status: engine.status,
          boardReady: engine.inspectScene?.()?.boardReady,
        };
      });
      expect(current.connected).toBe(true);
      expect(current.id).toBeTruthy();
      expect(current.phase).toBe("playing");
      expect(current.phaseGameId).toBe(current.id);
      expect(current.status.phase).toBe("ready");
      expect(current.status.backend).toBe(backend);
      expect(current.boardReady).toBe(true);
    }).toPass({ timeout: 120_000, intervals: [200] });
    await expect(page.getByRole("status")).toHaveCount(0);
    await expect(page.locator("canvas")).toHaveCount(1);
    const world = await measurePacing(page, duration);
    evidence.world = world;
    // Hardware refresh and throughput are observations, never hard-coded expectations.
    expect(world.warmSubmittedFrames).toBeGreaterThanOrEqual(40);
    expect(world.submittedFrames).toBeGreaterThan(0);
    expect(world.rafIntervals.length).toBeGreaterThan(0);
    if (backend === "webgpu") {
      expect(world.queue.available).toBe(true);
      expect(world.queue.registrations).toBeGreaterThan(0);
    }
    expect(errors).toEqual([]);
  } finally {
    evidence.errors = errors;
    if (!page.isClosed()) {
      await page
        .evaluate(() => Reflect.get(window, "enginePacingProbe")?.restore())
        .catch(() => {});
    }
    await writeFile(
      path.join(output, `phase6-pacing-${backend}-${label}.json`),
      JSON.stringify(evidence, null, 2) + "\n",
    );
    await context.close();
  }
});

test("pipelines real Native frames under simulated queue notification latency", async ({
  browser,
  baseURL,
}) => {
  test.skip(
    test.info().project.name !== "engine",
    "GPUQueue notification simulation is WebGPU-only",
  );
  test.skip(
    !process.env.E2E_EXPECT_FRAME_LIMIT,
    "Select baseline/candidate frame budget explicitly",
  );
  test.setTimeout(90_000);
  const expectedLimit = Number(process.env.E2E_EXPECT_FRAME_LIMIT);
  expect([1, 4], "Explicitly select baseline=1 or candidate=4").toContain(expectedLimit);
  const context = await browser.newContext({
    baseURL,
    viewport: { width: 1280, height: 720 },
    reducedMotion: "reduce",
  });
  const page = await context.newPage();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const label = (process.env.E2E_PACING_LABEL ?? `limit-${expectedLimit}`).replace(
    /[^a-zA-Z0-9_-]/g,
    "-",
  );
  const output = path.resolve("../tmp/bevy-scaffold");
  const evidence: Record<string, unknown> = {
    kind: "controlled-queue-notification-delay",
    expectedLimit,
    delayMs: 60,
    note:
      "Simulates callback delivery latency after real queue completion. " +
      "This is not GPU execution time or a hardware throughput benchmark.",
  };
  try {
    await page.goto("/dev/engine?renderer=webgpu");
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      {
        timeout: 60_000,
      },
    );
    await expect(page.locator("canvas")).toHaveCount(1);
    const result = await page.evaluate(
      async ({ budget, delayMs }) => {
        const module = "/src/stores/engineStore.ts";
        const { useEngineStore } = await import(/* @vite-ignore */ module);
        const source = useEngineStore.getState().performance;
        if (!source) throw new Error("Actual Native performance source is missing");
        type Frame = {
          submittedFrames: number;
          completedFrames?: number;
          pendingFrames?: number;
          frameLimit?: number;
        };
        const sample = () => {
          const value = source.sample() as Frame | null;
          if (!value) throw new Error("Native counters are inactive");
          return value;
        };
        const deadline = performance.now() + 60_000;
        const nextRaf = () =>
          new Promise<number>((resolve, reject) => {
            requestAnimationFrame((timestamp) => {
              if (performance.now() > deadline) reject(new Error("Real-frame deadline exceeded"));
              else resolve(timestamp);
            });
          });
        const waitNativeFrames = async (count: number) => {
          const start = sample().submittedFrames;
          while (sample().submittedFrames - start < count) {
            // Serial sampling follows actual rAF callbacks; no synthesized frame count.
            // oxlint-disable-next-line no-await-in-loop -- Warm-up follows successive real frames.
            await nextRaf();
          }
        };
        const measure = async (durationMs: number) => {
          const start = sample();
          const startTime = performance.now();
          let now = startTime;
          let previous: number | null = null;
          const rafIntervals: number[] = [];
          let peakPending: number | null = start.pendingFrames ?? null;
          do {
            // oxlint-disable-next-line no-await-in-loop -- Cadence measures successive callbacks.
            const rafTime = await nextRaf();
            if (previous !== null) rafIntervals.push(rafTime - previous);
            previous = rafTime;
            const frame = sample();
            if (frame.pendingFrames !== undefined) {
              peakPending = Math.max(peakPending ?? 0, frame.pendingFrames);
            }
            now = performance.now();
          } while (now - startTime < durationMs);
          const end = sample();
          const elapsedMs = now - startTime;
          const rafSpan = rafIntervals.reduce((sum, interval) => sum + interval, 0);
          return {
            elapsedMs,
            start,
            end,
            nativeFrames: end.submittedFrames - start.submittedFrames,
            nativeFps: (1000 * (end.submittedFrames - start.submittedFrames)) / elapsedMs,
            rafFrames: rafIntervals.length,
            rafFps: rafSpan ? (1000 * rafIntervals.length) / rafSpan : 0,
            rafIntervals,
            peakPending,
          };
        };
        const prototype = GPUQueue.prototype;
        const original = prototype.onSubmittedWorkDone;
        const descriptor = Object.getOwnPropertyDescriptor(prototype, "onSubmittedWorkDone");
        const pendingTimers = new Map<number, () => void>();
        let delayedRegistrations = 0;
        let delayedCompletions = 0;
        let delayedOutstanding = 0;
        let delayedPeak = 0;
        let simulate = false;
        const wrapped = function (this: GPUQueue) {
          const realCompletion = original.call(this);
          if (!simulate) return realCompletion;
          delayedRegistrations++;
          delayedOutstanding++;
          delayedPeak = Math.max(delayedPeak, delayedOutstanding);
          return realCompletion.then(
            () => {
              if (!simulate) {
                delayedOutstanding--;
                return;
              }
              return new Promise<void>((resolve) => {
                const finish = () => {
                  delayedOutstanding--;
                  delayedCompletions++;
                  resolve();
                };
                // The fixed delay deliberately simulates notification latency. It never
                // marks readiness or invents progress/frames; Native still owns submission.
                const timer = window.setTimeout(() => {
                  pendingTimers.delete(timer);
                  finish();
                }, delayMs);
                pendingTimers.set(timer, finish);
              });
            },
            (error: unknown) => {
              delayedOutstanding--;
              throw error;
            },
          );
        };
        source.setActive(true);
        try {
          await waitNativeFrames(24);
          const undelayed = await measure(2500);
          if (undelayed.end.frameLimit !== undefined && undelayed.end.frameLimit !== budget) {
            throw new Error(
              `Requested frame limit ${budget}; Native reports ${undelayed.end.frameLimit}`,
            );
          }
          Object.defineProperty(prototype, "onSubmittedWorkDone", {
            configurable: true,
            writable: true,
            value: wrapped,
          });
          simulate = true;
          await waitNativeFrames(16);
          const delayed = await measure(3000);
          return {
            undelayed,
            delayed,
            delayedRegistrations,
            delayedCompletions,
            delayedOutstanding,
            delayedPeak,
            serialCeilingFps: 1000 / delayMs,
            configuredBudget: budget,
            nativeReportedBudget: delayed.end.frameLimit ?? null,
          };
        } finally {
          simulate = false;
          if (prototype.onSubmittedWorkDone === wrapped) {
            if (descriptor) Object.defineProperty(prototype, "onSubmittedWorkDone", descriptor);
            else Reflect.deleteProperty(prototype, "onSubmittedWorkDone");
          }
          // Release delayed notifications while restoring, so the real renderer is not
          // left blocked. No callback can schedule another simulation timer afterwards.
          for (const [timer, finish] of pendingTimers) {
            window.clearTimeout(timer);
            finish();
          }
          pendingTimers.clear();
          source.setActive(false);
        }
      },
      { budget: expectedLimit, delayMs: 60 },
    );
    evidence.result = result;
    expect(errors).toEqual([]);
    expect(result.delayedRegistrations).toBeGreaterThan(0);
    expect(result.delayedCompletions).toBeGreaterThan(0);
    expect(result.delayed.nativeFrames).toBeGreaterThan(0);
    if (result.delayed.peakPending !== null) {
      expect(result.delayed.peakPending).toBeLessThanOrEqual(expectedLimit);
    }
    // If the minimal real renderer itself cannot sustain enough frames, retain the
    // measurements as inconclusive rather than calling host performance a regression.
    const enoughCapacity = result.undelayed.nativeFps >= 35 && result.undelayed.rafFps >= 35;
    evidence.enoughCapacityForControlledProof = enoughCapacity;
    test.skip(
      !enoughCapacity,
      "Host lacks the measured minimal-scene headroom for delay comparison",
    );
    if (expectedLimit === 1) {
      // One Native frame must await each 60ms notification, plus rAF scheduling.
      expect(result.delayed.nativeFps).toBeLessThan(result.serialCeilingFps * 1.15);
    } else {
      // Four real submissions may overlap the simulated delay. Exceeding the serial
      // mathematical ceiling demonstrates pipelining, independently of display Hz.
      expect(result.delayed.nativeFps).toBeGreaterThan(result.serialCeilingFps * 1.5);
    }
  } finally {
    evidence.errors = errors;
    await mkdir(output, { recursive: true });
    await writeFile(
      path.join(output, `phase6-pacing-delay-webgpu-${label}.json`),
      JSON.stringify(evidence, null, 2) + "\n",
    );
    await context.close();
  }
});

// Keep the existing "bounds submissions while GPU completion is pending" case in
// engine.spec.ts. It already holds real acknowledgements, proves admission stops, then
// releases them and proves rendering resumes. This delay test complements that invariant.

test("cold readiness waits for actual GPU completion delivery", async ({ browser, baseURL }) => {
  test.skip(test.info().project.name !== "engine", "Completion delivery hook is WebGPU-only");
  test.setTimeout(120_000);
  const context = await browser.newContext({
    baseURL,
    viewport: { width: 1280, height: 720 },
    reducedMotion: "reduce",
  });
  await context.addInitScript(() => {
    const prototype = GPUQueue.prototype;
    const descriptor = Object.getOwnPropertyDescriptor(prototype, "onSubmittedWorkDone");
    const original = prototype.onSubmittedWorkDone;
    const pending = new Set<() => void>();
    let holding = true;
    let registrations = 0;
    const wrapped = function (this: GPUQueue) {
      registrations++;
      // Real GPU work must complete first. Only delivery to the renderer is held;
      // no readiness, Native frames, browser cadence or queue completion is invented.
      return original.call(this).then(() => {
        if (holding) {
          return new Promise<void>((resolve) => pending.add(resolve));
        }
      });
    };
    const release = () => {
      holding = false;
      for (const resolve of pending) resolve();
      pending.clear();
    };
    Object.defineProperty(prototype, "onSubmittedWorkDone", {
      configurable: true,
      writable: true,
      value: wrapped,
    });
    Reflect.set(window, "coldReadinessProbe", {
      snapshot: () => ({ held: pending.size, registrations }),
      release,
      restore() {
        if (prototype.onSubmittedWorkDone === wrapped) {
          if (descriptor) Object.defineProperty(prototype, "onSubmittedWorkDone", descriptor);
          else Reflect.deleteProperty(prototype, "onSubmittedWorkDone");
        }
        release();
      },
    });
  });
  const page = await context.newPage();
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    await page.goto("/dev/engine?renderer=webgpu");
    await expect
      .poll(() => page.evaluate(() => Reflect.get(window, "coldReadinessProbe").snapshot().held), {
        timeout: 60_000,
      })
      .toBeGreaterThan(0);
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "preparing",
      { timeout: 60_000 },
    );
    const held = await page.evaluate(async () => {
      const module = "/src/stores/engineStore.ts";
      const { useEngineStore } = await import(/* @vite-ignore */ module);
      const before = Reflect.get(window, "coldReadinessProbe").snapshot();
      for (let frame = 0; frame < 16; frame++) {
        // oxlint-disable-next-line no-await-in-loop -- Observe successive real animation frames.
        await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      }
      return {
        before,
        after: Reflect.get(window, "coldReadinessProbe").snapshot(),
        phase: useEngineStore.getState().status.phase,
      };
    });
    expect(held.before.held).toBeGreaterThan(0);
    expect(held.after.held).toBeGreaterThan(0);
    expect(held.phase).toBe("preparing");
    await expect(page.locator("[data-engine-state]")).not.toHaveAttribute(
      "data-engine-state",
      "ready",
    );
    await page.evaluate(() => Reflect.get(window, "coldReadinessProbe").release());
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      { timeout: 60_000 },
    );
    await expect(page.locator("canvas")).toHaveCount(1);
    expect(errors).toEqual([]);
    console.info("Cold Native readiness fence", held);
  } finally {
    if (!page.isClosed()) {
      await page
        .evaluate(() => Reflect.get(window, "coldReadinessProbe")?.restore())
        .catch(() => {});
    }
    await context.close();
  }
});

test("records the production engine's actual WebGPU adapter", async ({ browser, baseURL }) => {
  test.skip(test.info().project.name !== "engine", "WebGPU adapter metadata only");
  test.setTimeout(120_000);
  const context = await browser.newContext({ baseURL, viewport: { width: 1280, height: 720 } });
  await context.addInitScript(() => {
    const requests: {
      info: { vendor: string; architecture: string; device: string; description: string } | null;
      infoAvailable: boolean;
      isFallbackAdapter: boolean | null;
      unavailableReason: string | null;
    }[] = [];
    const prototype = (
      Reflect.get(globalThis, "GPUAdapter") as { prototype: GPUAdapter } | undefined
    )?.prototype;
    const descriptor = prototype && Object.getOwnPropertyDescriptor(prototype, "requestDevice");
    const original = prototype?.requestDevice;
    const wrapped = function (this: GPUAdapter, deviceDescriptor?: GPUDeviceDescriptor) {
      let info = null;
      let unavailableReason: string | null = null;
      let fallback: boolean | null = null;
      try {
        const adapterInfo = this.info;
        info = adapterInfo
          ? {
              vendor: adapterInfo.vendor ?? "",
              architecture: adapterInfo.architecture ?? "",
              device: adapterInfo.device ?? "",
              description: adapterInfo.description ?? "",
            }
          : null;
        if (!info || !Object.values(info).some(Boolean)) {
          unavailableReason = "Adapter info is absent or empty in this browser";
        }
        const value = Reflect.get(this, "isFallbackAdapter");
        fallback = typeof value === "boolean" ? value : null;
      } catch (error) {
        unavailableReason = String(error);
      }
      requests.push({
        info,
        infoAvailable: info !== null && Object.values(info).some(Boolean),
        isFallbackAdapter: fallback,
        unavailableReason,
      });
      // Capture this exact adapter synchronously and return the original Promise unchanged.
      return original!.call(this, deviceDescriptor);
    };
    if (prototype && original) {
      Object.defineProperty(prototype, "requestDevice", {
        configurable: true,
        writable: true,
        value: wrapped,
      });
    }
    Reflect.set(window, "verificationAdapterProbe", {
      snapshot: () => ({
        hookAvailable: !!original,
        requests,
        unavailableReason: !original
          ? "GPUAdapter.requestDevice is unavailable"
          : requests.length === 0
            ? "No requestDevice call was captured"
            : null,
      }),
      restore() {
        if (prototype && original && prototype.requestDevice === wrapped) {
          if (descriptor) Object.defineProperty(prototype, "requestDevice", descriptor);
          else Reflect.deleteProperty(prototype, "requestDevice");
        }
      },
    });
  });
  const page = await context.newPage();
  const evidence: Record<string, unknown> = {
    source: "Verification Playwright browser; exact production engine requestDevice adapter",
  };
  try {
    await page.goto("/dev/engine?renderer=webgpu");
    await expect(page.locator("[data-engine-state]")).toHaveAttribute(
      "data-engine-state",
      "ready",
      {
        timeout: 90_000,
      },
    );
    await expect(page.locator("canvas")).toHaveAttribute("data-engine-backend", "webgpu");
    Object.assign(
      evidence,
      await page.evaluate(() => ({
        browserUserAgent: navigator.userAgent,
        backend: document.querySelector("canvas")?.getAttribute("data-engine-backend"),
        adapter: Reflect.get(window, "verificationAdapterProbe").snapshot(),
      })),
    );
  } finally {
    if (!page.isClosed()) {
      await page
        .evaluate(() => Reflect.get(window, "verificationAdapterProbe")?.restore())
        .catch(() => {});
    }
    const directory = path.resolve("../tmp/bevy-scaffold");
    await mkdir(directory, { recursive: true });
    await writeFile(
      path.join(directory, "phase6-verification-adapter-webgpu.json"),
      JSON.stringify(evidence, null, 2) + "\n",
    );
    await context.close();
  }
});
