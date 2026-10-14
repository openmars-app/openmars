import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const cards = JSON.parse(
  readFileSync(resolve(import.meta.dirname, "../../backend/assets/cards.json"), "utf8"),
);

async function mount(page: Page) {
  await page.route("**/cards?**", (route) => {
    const url = new URL(route.request().url());
    const offset = Number(url.searchParams.get("offset"));
    const limit = Number(url.searchParams.get("limit"));
    return route.fulfill({
      json: { cards: cards.slice(offset, offset + limit), totalCount: cards.length, offset, limit },
    });
  });
  await page.goto("/create?quick");
  await page.evaluate(async () => {
    const [React, ReactDOM, ui, windows, overlay] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/debug/uiDebug/UIDebugWindow.tsx",
        "/src/components/ui/debug/WindowManager.tsx",
        "/src/stores/uiOverlayStore.ts",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const element = document.createElement("div");
    document.body.append(element);
    const root = ReactDOM.default.createRoot(element);
    overlay.useUIOverlayStore.getState().setShowUIDebug(true);
    root.render(
      React.default.createElement(
        windows.WindowManagerProvider,
        null,
        React.default.createElement(ui.default, { onClose: () => root.unmount() }),
      ),
    );
  });
  await expect(page.getByRole("button", { name: "Open preview", exact: true })).toBeEnabled();
}

async function select(page: Page, id: string, preset: string) {
  await page.locator(`[data-preview-id="${id}"]`).click();
  await page.getByLabel("Sample scenario").selectOption(preset);
  await page.getByRole("button", { name: "Open preview", exact: true }).click();
  await expect(page.locator("[data-ui-debug-preview]")).toBeVisible();
  await expect(page.getByText("This preview could not render:", { exact: false })).toHaveCount(0);
}

async function back(page: Page) {
  await page.getByRole("button", { name: "Back to UI Debug", exact: true }).last().click();
  await expect(page.locator("[data-ui-debug-preview]")).toHaveCount(0);
}

async function snapshot(page: Page) {
  return page.evaluate(async () => {
    const paths = [
      "/src/stores/gameStore.ts",
      "/src/stores/botPresenceStore.ts",
      "/src/stores/installAppStore.ts",
      "/src/stores/versionStore.ts",
    ];
    const [game, bot, install, version] = await Promise.all(
      paths.map((path) => import(/* @vite-ignore */ path)),
    );
    return JSON.stringify({
      game: game.useGameStore.getState(),
      bot: bot.useBotThoughtsPreferenceStore.getState(),
      install: install.useInstallAppStore.getState(),
      version: version.useVersionStore.getState(),
      local: { ...localStorage },
      session: { ...sessionStorage },
      url: location.href,
      history: history.length,
    });
  });
}

test("session actions and nested dialogs stay local", async ({ page }) => {
  await mount(page);
  const before = await snapshot(page);
  const mutations: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD"].includes(request.method())) {
      mutations.push(request.url());
    }
  });
  await select(page, "lobby-settings", "general");
  await page.locator('[data-ui-debug-preview] input[type="number"]').first().fill("5");
  await page.getByRole("button", { name: "AI", exact: true }).click();
  await back(page);

  await select(page, "game-setup", "editable");
  await page.locator("[data-ui-debug-preview]").getByRole("button", { name: /rules/i }).click();
  await expect(page.getByRole("heading", { name: "Classic rules" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("heading", { name: "Classic rules" })).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Game setup", exact: true })).toBeVisible();
  await back(page);

  await select(page, "join", "named");
  await page
    .locator("[data-ui-debug-preview]")
    .getByRole("textbox", { name: "Your name" })
    .fill("Local preview player");
  await page.getByRole("button", { name: "Join", exact: true }).click();
  await expect(page.getByRole("status")).toHaveText("Simulated: Joined as Local preview player");
  await expect(page.locator("[data-ui-debug-preview]")).toHaveCount(0);

  await select(page, "server-selector", "mixed");
  await page.getByRole("button", { name: /Hellas Community/ }).click();
  await expect(page.getByRole("status")).toHaveText("Simulated: Selected hellas");
  await expect(page.locator("[data-ui-debug-preview]")).toHaveCount(0);
  expect(await snapshot(page)).toEqual(before);
  expect(mutations).toEqual([]);
});

test("session layouts expose compact controls and long content", async ({ page }, testInfo) => {
  await mount(page);
  for (const [id, preset] of [
    ["waiting-room", "host"],
    ["game-setup", "editable"],
    ["join", "error"],
    ["changelog", "long"],
  ]) {
    await select(page, id, preset);
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all(
        Array.from(document.images, (image) =>
          image.complete
            ? Promise.resolve()
            : new Promise<void>((resolveImage) => {
                image.addEventListener("load", () => resolveImage(), { once: true });
                image.addEventListener("error", () => resolveImage(), { once: true });
              }),
        ),
      );
    });
    await page.getByRole("button", { name: "Collapse preview controls", exact: true }).click();
    await page.screenshot({
      path: testInfo.outputPath(`${id}-${preset}.png`),
      animations: "disabled",
    });
    if (id === "waiting-room") {
      await expect(page.getByRole("button", { name: "Start game", exact: true })).toBeVisible();
    }
    if (id === "join") {
      await expect(page.getByRole("alert")).toContainText("game is full");
    }
    await page.getByRole("button", { name: "UI Debug", exact: true }).click();
    await back(page);
  }
});
