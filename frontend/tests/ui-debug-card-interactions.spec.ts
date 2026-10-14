import { expect, test, type Page } from "@playwright/test";

declare global {
  interface Window {
    cardPreviewState: () => string;
  }
}

async function mountCatalog(page: Page, liveFlow = false) {
  await page.goto("/create?quick");
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible({
    timeout: 20_000,
  });
  await page.evaluate(async (showLiveFlow) => {
    const load = (path: string) => import(/* @vite-ignore */ path);
    const [React, DOM, windowModule, manager, game, flow, payment, flowPopover] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/debug/uiDebug/UIDebugWindow.tsx",
        "/src/components/ui/debug/WindowManager.tsx",
        "/src/stores/gameStore.ts",
        "/src/stores/cardPlayFlowStore.ts",
        "/src/stores/paymentStore.ts",
        "/src/components/ui/popover/GameFlowPopover.tsx",
      ].map(load),
    );
    const host = document.createElement("div");
    document.body.append(host);
    const root = DOM.default.createRoot(host);
    window.cardPreviewState = () =>
      JSON.stringify({
        game: game.useGameStore.getState(),
        flow: flow.useCardPlayFlowStore.getState(),
        payment: payment.usePaymentStore.getState(),
      });
    root.render(
      React.default.createElement(
        React.default.Fragment,
        null,
        showLiveFlow &&
          React.default.createElement(
            flowPopover.GameFlowPopover,
            { isVisible: true, type: "interactive-mandatory" },
            "Live effect waiting in the background",
          ),
        React.default.createElement(
          manager.WindowManagerProvider,
          null,
          React.default.createElement(windowModule.default, { onClose: () => root.unmount() }),
        ),
      ),
    );
  }, liveFlow);
  await expect(page.getByRole("button", { name: "Open preview", exact: true })).toBeEnabled({
    timeout: 20_000,
  });
}

async function openPreview(page: Page, id: string, presetId: string) {
  await page.locator(`[data-preview-id="${id}"]`).click();
  await page.getByRole("combobox", { name: "Sample scenario" }).selectOption(presetId);
  await page.getByRole("button", { name: "Open preview", exact: true }).click();
  await expect(page.locator(`[data-ui-debug-preview="${id}"]`)).toBeVisible();
}

const back = (page: Page) =>
  page.getByRole("button", { name: "Back to UI Debug", exact: true }).click();

test("UI Debug card choices, failure recovery and Ecology flow stay local", async ({
  page,
}, info) => {
  test.setTimeout(90_000);
  await mountCatalog(page);
  const before = await page.evaluate(() => ({
    state: window.cardPreviewState(),
    history: history.length,
  }));
  const sent: string[] = [];
  page.on("request", (request) => {
    if (request.method() !== "GET") sent.push(`${request.method()} ${request.url()}`);
  });
  page.on("websocket", (socket) =>
    socket.on("framesent", ({ payload }) => sent.push(String(payload))),
  );
  await openPreview(page, "behavior-resolution", "unavailable");
  let scope = page.locator("[data-ui-debug-preview]");
  await expect(scope.getByText("Not enough resources on Olympus Conference")).toBeVisible();
  await expect(scope.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();
  await page.screenshot({
    path: info.outputPath("olympus-unavailable.png"),
    animations: "disabled",
  });
  await scope.getByRole("button", { pressed: false }).first().click();
  await scope.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Sample effects resolved locally");

  await openPreview(page, "behavior-resolution", "error");
  scope = page.locator("[data-ui-debug-preview]");
  await scope.getByRole("button", { pressed: false }).first().click();
  await scope.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(scope.getByRole("alert")).toContainText("connection lost");
  await scope.getByRole("button", { name: "Reset", exact: true }).click();
  await expect(scope.getByRole("alert")).toHaveCount(0);
  await expect(scope.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();
  await back(page);

  await openPreview(page, "behavior-resolution", "multiple");
  scope = page.locator("[data-ui-debug-preview]");
  await scope.getByRole("button", { name: "Mars University · 1", exact: true }).click();
  await scope.getByRole("button", { name: "Skip", exact: true }).click();
  await expect(scope.getByText("Resolve effects · 1 remaining", { exact: true })).toBeVisible();
  await scope.getByRole("button", { pressed: false }).first().click();
  await scope.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Sample effects resolved locally");

  await openPreview(page, "ecology-storage", "sequence");
  scope = page.locator("[data-ui-debug-preview]");
  await expect(scope.getByText("No animal storage available")).toBeVisible();
  await scope.getByRole("button", { name: "Continue without animal", exact: true }).click();
  await expect(scope.getByText("Place 2 microbes", { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath("ecology-microbes.png"), animations: "disabled" });
  await scope.locator(".game-choice").filter({ hasText: "Ants" }).click();
  await expect(page.getByRole("status")).toContainText("Placed 2 sample microbes on Ants");

  await openPreview(page, "optional-effect", "unavailable");
  scope = page.locator("[data-ui-debug-preview]");
  await expect(scope.getByRole("button", { name: "Pay 3 M€", exact: true })).toBeDisabled();
  await scope.getByRole("button", { name: "Skip", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Sample optional effect skipped");
  expect(
    await page.evaluate(() => ({ state: window.cardPreviewState(), history: history.length })),
  ).toEqual(before);
  expect(sent).toEqual([]);
});

test("UI Debug receipts, payment, removal, amounts and played-card search are interactive", async ({
  page,
}, info) => {
  test.setTimeout(90_000);
  await mountCatalog(page);
  const before = await page.evaluate(() => window.cardPreviewState());
  await openPreview(page, "card-receipt", "olympus");
  let scope = page.locator("[data-ui-debug-preview]");
  await expect(scope.getByRole("dialog", { name: "Cards received" })).toBeVisible();
  await expect(scope.getByText("Olympus Conference", { exact: true })).toBeVisible();
  await scope.getByRole("button", { name: "Close", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Sample receipt acknowledged");
  await openPreview(page, "payment", "poor");
  await page.screenshot({
    path: info.outputPath("payment-unaffordable.png"),
    animations: "disabled",
  });
  scope = page.locator("[data-ui-debug-preview]");
  await expect(scope.getByRole("button", { name: /Confirm/ })).toBeDisabled();
  await back(page);
  await openPreview(page, "resource-removal", "normal");
  scope = page.locator("[data-ui-debug-preview]");
  await scope.getByRole("button", { name: /^Helion/ }).click();
  await scope.getByRole("spinbutton", { name: "Amount to remove" }).fill("1");
  await scope.getByRole("button", { name: /Confirm|Remove/, exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Removed 1 sample plants");
  await openPreview(page, "resource-amount", "normal");
  scope = page.locator("[data-ui-debug-preview]");
  await scope.getByRole("button", { name: "-", exact: true }).click();
  await scope.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("Selected 4 sample energy");
  await openPreview(page, "played-cards", "many");
  scope = page.locator("[data-ui-debug-preview]");
  await scope.getByPlaceholder("Search cards...").fill("No possible card matches this phrase");
  await expect(scope.getByText("No Cards Found", { exact: true })).toBeVisible();
  await scope.getByPlaceholder("Search cards...").fill("");
  await expect(scope.locator(".game-card").first()).toBeVisible();
  await page.screenshot({ path: info.outputPath("played-cards.png"), animations: "disabled" });
  await back(page);
  expect(await page.evaluate(() => window.cardPreviewState())).toEqual(before);
});

test("nested mobile card detail consumes Escape before its parent preview", async ({
  page,
}, info) => {
  test.skip(
    info.project.name === "firefox",
    "Nested mobile detail uses the compact production path",
  );
  await mountCatalog(page);
  const before = await page.evaluate(() => ({
    state: window.cardPreviewState(),
    history: history.length,
  }));
  await openPreview(page, "played-cards", "normal");
  const scope = page.locator('[data-ui-debug-preview="played-cards"]');
  await scope.locator(".game-card").first().click();
  await expect(scope.getByRole("button", { name: "Close card", exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(scope.getByRole("button", { name: "Close card", exact: true })).toHaveCount(0);
  await expect(scope.getByPlaceholder("Search cards...")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "Open preview", exact: true })).toBeVisible();
  expect(
    await page.evaluate(() => ({ state: window.cardPreviewState(), history: history.length })),
  ).toEqual(before);
});

test("a live mandatory prompt does not intercept scrolling in a UI Debug preview", async ({
  page,
}, info) => {
  test.skip(info.project.name !== "firefox", "Desktop collection uses vertical scrolling");
  await mountCatalog(page, true);
  await openPreview(page, "played-cards", "many");
  const scope = page.locator('[data-ui-debug-preview="played-cards"]');
  const scroller = scope.getByRole("dialog").locator(".overflow-y-auto").first();
  await scroller.hover({ position: { x: 5, y: 5 } });
  await page.mouse.wheel(0, 450);
  await expect.poll(() => scroller.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  await back(page);
});
