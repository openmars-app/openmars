import { test, expect, type Page } from "@playwright/test";

declare global {
  interface Window {
    uiColonyPreviewProbe: {
      render: (id: string, preset?: string) => void;
      events: string[];
      ids: string[];
    };
  }
}

const mutationAttempts = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const attempts: string[] = [];
  mutationAttempts.set(page, attempts);
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    if (["GET", "HEAD", "OPTIONS"].includes(request.method())) {
      await route.continue();
    } else {
      attempts.push(`${request.method()} ${request.url()}`);
      await route.abort();
    }
  });
  // Preview checks do not need a live game transport. Do not forward socket messages.
  await page.routeWebSocket("**/*", (socket) => {
    socket.onMessage((message) => {
      const payload = JSON.parse(String(message));
      if (!["ping", "authenticate", "subscribe"].includes(payload.type)) {
        attempts.push(`WebSocket ${payload.type}`);
      }
    });
  });
});
test.afterEach(({ page }) => {
  expect(mutationAttempts.get(page)).toEqual([]);
});

async function prepare(page: Page) {
  await page.goto("/create?quick");
  await expect(page.locator("html")).toHaveAttribute("data-layout", /desktop|compact/);
  await page.evaluate(async () => {
    const [React, DOM, previews, environment] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/debug/uiDebug/colonyPreviews.tsx",
        "/src/contexts/OverlayEnvironment.tsx",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const { cards } = await (await fetch("/api/v1/cards?limit=1000")).json();
    const host = document.createElement("div");
    host.id = "ui-colony-preview";
    host.dataset.uiDebugPreview = "true";
    Object.assign(host.style, {
      position: "fixed",
      inset: "0",
      zIndex: "100000",
      background: "#100e0d",
      color: "white",
      contain: "layout paint",
      overflow: "auto",
    });
    document.body.append(host);
    const root = DOM.default.createRoot(host);
    let version = 0;
    const events: string[] = [];
    window.uiColonyPreviewProbe = {
      events,
      ids: previews.colonyPreviews.map((preview: { id: string }) => preview.id),
      render: (id, preset = "default") => {
        const entry = previews.colonyPreviews.find((preview: { id: string }) => preview.id === id);
        root.render(
          React.default.createElement(
            environment.OverlayEnvironment.Provider,
            { value: host },
            React.default.createElement(entry.Component, {
              key: ++version,
              presetId: preset,
              cards,
              onComplete: (message?: string) => events.push(message ?? "complete"),
            }),
          ),
        );
      },
    };
  });
  mutationAttempts.get(page)!.length = 0;
}
async function show(page: Page, id: string, preset = "default") {
  await page.evaluate((fixture) => window.uiColonyPreviewProbe.render(fixture.id, fixture.preset), {
    id,
    preset,
  });
}

test("Market Manipulation preview preserves selections through local errors", async ({
  page,
}, info) => {
  await prepare(page);
  await show(page, "market-manipulation", "error");
  await page
    .getByRole("group", { name: "Decrease", exact: true })
    .getByRole("button", { name: "Pluto" })
    .click();
  await page
    .getByRole("group", { name: "Increase", exact: true })
    .getByRole("button", { name: "Luna" })
    .click();
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Preview request failed");
  await page.screenshot({ path: info.outputPath("market-error.png"), animations: "disabled" });
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => window.uiColonyPreviewProbe.events))
    .toEqual(["Submitted option 5."]);
});

test("colony preview trade and build update only local fixture", async ({ page }, info) => {
  await prepare(page);
  await show(page, "colony-trade", "screen");
  const luna = page.locator("#ui-colony-preview .popover-list-item").filter({ hasText: "Luna" });
  await luna.getByRole("button", { name: "Trade", exact: true }).click();
  await expect(luna).toContainText("Already Traded");
  await page.getByRole("button", { name: "Build", exact: true }).first().click();
  await luna.getByRole("button", { name: "Build", exact: true }).click();
  await expect(luna.getByRole("button", { name: "Build", exact: true })).toBeDisabled();
  await page.screenshot({ path: info.outputPath("colony-build.png"), animations: "disabled" });
  await expect.poll(() => page.evaluate(() => window.uiColonyPreviewProbe.events.length)).toBe(2);
  await show(page, "colony-trade", "unavailable");
  await expect(
    page
      .locator("#ui-colony-preview .popover-list-item")
      .first()
      .getByRole("button", { name: "Trade", exact: true }),
  ).toBeDisabled();
});

test("information previews mount production views and chat remains local", async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await prepare(page);
  const ids = await page.evaluate(() => window.uiColonyPreviewProbe.ids);
  await ids.reduce(async (previous, id) => {
    await previous;
    await show(page, id);
    await expect(page.locator("#ui-colony-preview")).not.toBeEmpty();
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    await expect.poll(() => errors).toEqual([]);
  }, Promise.resolve());
  await show(page, "chat", "typing");
  const input = page.locator("#ui-colony-preview input");
  await input.fill("Local preview message");
  await input.press("Enter");
  await expect(page.locator("#ui-colony-preview")).toContainText("Local preview message");
  await page.screenshot({ path: info.outputPath("chat.png"), animations: "disabled" });
  await expect.poll(() => errors).toEqual([]);
});

test("free award and funding payment previews submit local callbacks", async ({ page }, info) => {
  await prepare(page);
  await show(page, "award-fund-selection");
  await page
    .locator("#ui-colony-preview")
    .getByRole("button", { name: /Banker/ })
    .click();
  await expect
    .poll(() => page.evaluate(() => window.uiColonyPreviewProbe.events))
    .toEqual(["Funded award-0 for free."]);
  await show(page, "project-funding");
  await page.getByRole("button", { name: "Buy Seat", exact: true }).first().click();
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect
    .poll(() =>
      page.evaluate(() =>
        window.uiColonyPreviewProbe.events.filter((event) => event.startsWith("Seat purchased:")),
      ),
    )
    .toHaveLength(1);
  await expect(page.locator("#ui-colony-preview")).toContainText("2/4");
  await page.screenshot({ path: info.outputPath("funding.png"), animations: "disabled" });
});

test("achievement and colony popovers use their approved art", async ({ page }, info) => {
  await prepare(page);
  for (const [id, icon] of [
    ["awards", "award-banker"],
    ["milestones", "milestone-terraformer"],
    ["colony-trade", "colony-pluto"],
  ]) {
    for (const preset of ["default", "long"]) {
      await show(page, id, preset);
      const image = page.locator(`#ui-colony-preview img[alt="${icon}"]`).first();
      await expect(image).toBeVisible();
      await expect(image).toHaveJSProperty("complete", true);
      await expect(image).not.toHaveJSProperty("naturalWidth", 0);
      await page.screenshot({
        path: info.outputPath(`${id}-${preset}.png`),
        animations: "disabled",
      });
    }
  }
});
