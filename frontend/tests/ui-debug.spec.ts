import { expect, test, type Page } from "@playwright/test";
import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";

const cards = JSON.parse(
  readFileSync(resolve(import.meta.dirname, "../../backend/assets/cards.json"), "utf8"),
);

async function mount(page: Page, failCatalog = false) {
  let failed = false;
  await page.route("**/cards?**", (route) => {
    if (failCatalog && !failed) {
      failed = true;
      return route.fulfill({ status: 503, json: { error: "Sample catalog unavailable" } });
    }
    const url = new URL(route.request().url());
    const offset = Number(url.searchParams.get("offset"));
    const limit = Number(url.searchParams.get("limit"));
    return route.fulfill({
      json: { cards: cards.slice(offset, offset + limit), totalCount: cards.length, offset, limit },
    });
  });
  await page.goto("/create?quick");
  await page.evaluate(async () => {
    const [React, ReactDOM, ui, windows, game, overlay] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/debug/uiDebug/UIDebugWindow.tsx",
        "/src/components/ui/debug/WindowManager.tsx",
        "/src/stores/gameStore.ts",
        "/src/stores/uiOverlayStore.ts",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const element = document.createElement("div");
    element.id = "ui-debug-test";
    document.body.append(element);
    const root = ReactDOM.default.createRoot(element);
    overlay.useUIOverlayStore.getState().setShowUIDebug(true);
    (window as unknown as { debugSnapshot: string }).debugSnapshot = JSON.stringify(
      game.useGameStore.getState(),
    );
    root.render(
      React.default.createElement(
        windows.WindowManagerProvider,
        null,
        React.default.createElement(ui.default, { onClose: () => root.unmount() }),
      ),
    );
  });
  if (!failCatalog)
    await expect(page.getByRole("button", { name: "Open preview", exact: true })).toBeEnabled();
}

async function select(page: Page, id: string, preset?: string) {
  await page.locator(`[data-preview-id="${id}"]`).click();
  if (preset) await page.getByLabel("Sample scenario").selectOption(preset);
  await page.getByRole("button", { name: "Open preview", exact: true }).click();
  await expect(page.locator("[data-ui-debug-preview]")).toBeVisible();
}

test("every registered scenario opens, resets and dismisses without live commands", async ({
  page,
}) => {
  test.setTimeout(240_000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await mount(page);
  const mutations: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD"].includes(request.method())) mutations.push(request.url());
  });
  const registry = await page.evaluate(async () => {
    const path = "/src/components/ui/debug/uiDebug/registry.ts";
    const { previews } = await import(/* @vite-ignore */ path);
    return previews.map((entry: { id: string; presets: { id: string }[] }) => ({
      id: entry.id,
      presets: entry.presets.map((preset) => preset.id),
    }));
  });
  for (const entry of registry) {
    for (const preset of entry.presets) {
      await test.step(`${entry.id}: ${preset}`, async () => {
        await select(page, entry.id, preset);
        await expect(
          page.getByText("This preview could not render:", { exact: false }),
        ).toHaveCount(0);
        await page.getByRole("button", { name: "Reset", exact: true }).click();
        await expect(
          page.getByText("This preview could not render:", { exact: false }),
        ).toHaveCount(0);
        await page.getByRole("button", { name: "Back to UI Debug", exact: true }).last().click();
        await expect(page.getByRole("dialog", { name: "UI Debug", exact: true })).toBeVisible();
        await expect(page.locator("[data-ui-debug-preview]")).toHaveCount(0);
      });
    }
  }
  expect(errors).toEqual([]);
  expect(mutations).toEqual([]);
  const unchanged = await page.evaluate(async () => {
    const path = "/src/stores/gameStore.ts";
    const { useGameStore } = await import(/* @vite-ignore */ path);
    return (
      JSON.stringify(useGameStore.getState()) ===
      (window as unknown as { debugSnapshot: string }).debugSnapshot
    );
  });
  expect(unchanged).toBe(true);
  await page.getByRole("button", { name: "Close UI Debug" }).click();
  await expect(page.locator("body > [inert]")).toHaveCount(0);
});

test("catalog, colony choice and keyboard layouts", async ({ page }, testInfo) => {
  await mount(page);
  await page.screenshot({ animations: "disabled", path: testInfo.outputPath("catalog.png") });
  await select(page, "market-manipulation");
  await page.screenshot({
    animations: "disabled",
    path: testInfo.outputPath("market-manipulation.png"),
  });
  await page.getByRole("button", { name: "Back to UI Debug", exact: true }).last().click();
  await select(page, "chat");
  const input = page
    .locator("[data-ui-debug-preview] input, [data-ui-debug-preview] textarea")
    .first();
  await input.fill("Preview chat stays local");
  await page.screenshot({ animations: "disabled", path: testInfo.outputPath("chat-focused.png") });
});

test("catalog inventory accounts for every modal and prompt", async ({ page }) => {
  await mount(page);
  const paths: string[] = await page.evaluate(async () => {
    const path = "/src/components/ui/debug/uiDebug/registry.ts";
    const { previews } = await import(/* @vite-ignore */ path);
    return previews.flatMap((entry: { componentPaths: string[] }) => entry.componentPaths);
  });
  const excluded = new Set([
    "GameFlowPopover.tsx",
    "GameMenuModal.tsx", // Shared layout primitives.
    "BlurredOverlay.tsx",
    "HexagonalShieldOverlay.tsx",
    "RotateDeviceOverlay.tsx", // Decorative/mandatory guard.
    "CardFanOverlay.tsx", // Permanent hand presentation; card detail and played-card screens are cataloged.
  ]);
  const missing: string[] = [];
  for (const directory of ["overlay", "popover", "modals"]) {
    for (const file of readdirSync(
      resolve(import.meta.dirname, `../src/components/ui/${directory}`),
    )) {
      if (!/(Modal|Dialog|Overlay|Popover)\.tsx$/.test(file) || excluded.has(file)) continue;
      const path = `components/ui/${directory}/${file}`;
      if (!paths.includes(path)) missing.push(path);
    }
  }
  expect(missing).toEqual([]);
});

test("catalog retries errors and movable controls stay accessible", async ({ page }, testInfo) => {
  await mount(page, true);
  await expect(page.getByRole("alert")).toContainText("Sample catalog unavailable");
  await page.screenshot({ animations: "disabled", path: testInfo.outputPath("catalog-error.png") });
  await page.getByRole("button", { name: "Retry loading samples" }).click();
  await expect(page.getByRole("button", { name: "Open preview", exact: true })).toBeEnabled();
  await select(page, "market-manipulation");
  const move = page.getByRole("button", { name: "Move preview controls" });
  const bounds = (await move.boundingBox())!;
  await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2);
  await page.mouse.down();
  await page.mouse.move(500, 150);
  await page.mouse.up();
  await page.getByRole("button", { name: "Collapse preview controls" }).click();
  await expect(page.getByRole("button", { name: "UI Debug", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "UI Debug", exact: true }).click();
  await page.getByRole("button", { name: "Back to UI Debug", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: "UI Debug", exact: true })).toHaveCount(0);
});

test("portrait guard covers previews on phones", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "firefox", "Phone portrait guard");
  await mount(page);
  await select(page, "market-manipulation");
  await page.setViewportSize({ width: 393, height: 852 });
  await expect(
    page.locator("[data-ui-debug-preview]").getByText("Rotate your device"),
  ).toBeVisible();
  const covered = await page.evaluate(() => {
    const el = document.elementFromPoint(innerWidth / 2, innerHeight / 2);
    return el?.parentElement?.textContent?.includes("Rotate your device");
  });
  expect(covered).toBe(true);
  await page.screenshot({ animations: "disabled", path: testInfo.outputPath("portrait.png") });
});

test("revised prompts recover empty choices and preserve catalog scroll", async ({
  page,
}, info) => {
  await mount(page);
  await page.locator('[data-preview-id="colony-placement"]').click();
  const catalog = page.getByLabel("UI preview catalog");
  const scroll = await catalog.evaluate((el) => el.scrollTop);
  expect(scroll).toBeGreaterThan(0);
  await page.getByLabel("Sample scenario").selectOption("unavailable");
  await page.getByRole("button", { name: "Open preview", exact: true }).click();
  await expect(page.getByText("No colony locations available")).toBeVisible();
  await expect(page.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();
  await page.screenshot({ path: info.outputPath("colony-empty.png"), animations: "disabled" });
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await expect(catalog).toBeVisible();
  expect(await catalog.evaluate((el) => el.scrollTop)).toBe(scroll);
  for (const id of [
    "free-trade",
    "market-manipulation",
    "award-fund-selection",
    "effect-selection",
  ]) {
    await select(page, id, "unavailable");
    await expect(page.getByRole("button", { name: "Continue", exact: true })).toBeEnabled();
    await page.screenshot({ path: info.outputPath(`${id}-empty.png`), animations: "disabled" });
    await page.getByRole("button", { name: "Continue", exact: true }).click();
    await expect(catalog).toBeVisible();
  }
});

test("card playability, normal reveal cards and stable amount controls", async ({ page }, info) => {
  await mount(page);
  const compact = (await page.locator("html").getAttribute("data-layout")) === "compact";
  await select(page, "card-draw", "normal");
  if (!compact) {
    await page.getByRole("button", { name: "Show Playability" }).click();
  }
  await expect(page.getByRole("button", { name: "Why this card can't be played" })).toHaveCount(2);
  await page.screenshot({ path: info.outputPath("cards-playability.png"), animations: "disabled" });
  await page.getByRole("button", { name: "Back to UI Debug", exact: true }).click();
  await select(page, "card-reveal", "many");
  await expect(page.locator('[data-presentation="inspection"]')).toHaveCount(0);
  await expect(page.locator('[data-presentation="compact"]')).toHaveCount(8);
  await page.screenshot({ path: info.outputPath("reveal-many.png"), animations: "disabled" });
  await page.getByRole("button", { name: "Continue", exact: true }).click();
  await select(page, "resource-amount", "large");
  const minus = page.getByRole("button", { name: "-", exact: true });
  const plus = page.getByRole("button", { name: "+", exact: true });
  const original = [await minus.boundingBox(), await plus.boundingBox()];
  for (const amount of [1, 9, 10, 99, 100]) {
    await page.getByRole("slider").fill(String(amount));
    await expect(page.getByRole("slider")).toHaveValue(String(amount));
    expect([await minus.boundingBox(), await plus.boundingBox()]).toEqual(original);
  }
  await page.screenshot({
    path: info.outputPath("amount-three-digits.png"),
    animations: "disabled",
  });
});

test("debug information popovers center and retain their content height", async ({
  page,
}, info) => {
  await mount(page);
  await select(page, "actions", "long");
  const desktop = (await page.locator("html").getAttribute("data-layout")) === "desktop";
  const heading = desktop
    ? page.getByText("Card Actions", { exact: true })
    : page.getByRole("dialog", { name: "actions", exact: true });
  await expect(heading).toBeVisible();
  await page.screenshot({ path: info.outputPath("actions-centered.png"), animations: "disabled" });
  if ((await page.locator("html").getAttribute("data-layout")) === "desktop") {
    const panel = page.locator("[data-ui-debug-preview] .game-panel");
    const box = (await panel.boundingBox())!;
    const viewport = page.viewportSize()!;
    expect(Math.abs(box.x + box.width / 2 - viewport.width / 2)).toBeLessThan(2);
    expect(Math.abs(box.y + box.height / 2 - viewport.height / 2)).toBeLessThan(2);
    expect(box.height).toBeGreaterThan(350);
  }
});

test("revised selection and session surfaces render consistently", async ({ page }, info) => {
  await mount(page);
  for (const [id, preset] of [
    ["resource-removal", "normal"],
    ["target-player", "normal"],
    ["colony-placement", "add-tile"],
    ["free-trade", "default"],
    ["award-fund-selection", "default"],
    ["effect-selection", "default"],
    ["project-funding", "default"],
    ["tab-conflict", "standard"],
  ]) {
    await select(page, id, preset);
    if (id === "resource-removal") {
      await page.getByRole("button", { name: /^Helion/ }).click();
      await page.getByRole("spinbutton").fill("2");
    }
    await page.screenshot({ path: info.outputPath(`${id}.png`), animations: "disabled" });
    await page.getByRole("button", { name: "Back to UI Debug", exact: true }).click();
  }
});
