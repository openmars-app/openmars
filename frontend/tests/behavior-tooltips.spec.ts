import { expect, test, type Locator, type Page } from "@playwright/test";

test.setTimeout(60_000);

async function expectInsideViewport(tooltip: Locator, page: Page) {
  await expect(tooltip).toBeVisible();
  const box = await tooltip.boundingBox();
  expect(box!.x).toBeGreaterThanOrEqual(11);
  expect(box!.y).toBeGreaterThanOrEqual(11);
  expect(box!.x + box!.width).toBeLessThanOrEqual(page.viewportSize()!.width - 11);
  expect(box!.y + box!.height).toBeLessThanOrEqual(page.viewportSize()!.height - 11);
}

test("Atmo Collectors action tooltip stays within the card catalog viewport", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&cId=186&cId=058&cId=C02&cId=C03&cId=C41");
  const card = page.locator('[data-card-id="C03"]');
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.scrollIntoViewIfNeeded();
  const action = card.locator(".behavior-fitting").last();
  if (info.project.use.hasTouch) {
    await action.tap();
  } else {
    await action.hover();
  }
  const tooltip = page.getByRole("tooltip");
  await expect(tooltip).toContainText("spend 1 floater here");
  await expect(tooltip).not.toContainText("Add 2 floaters");
  await expectInsideViewport(tooltip, page);
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("atmo-tooltip-contained.png"),
  });
});

test("behavior tooltips flip above bottom anchors and clamp at both horizontal edges", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&cId=C03");
  await expect(page.locator('[data-card-id="C03"]')).toBeVisible({ timeout: 30_000 });
  await page.evaluate(async () => {
    const [React, DOM, behavior] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/cards/BehaviorSection/BehaviorSection.tsx",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const host = document.createElement("div");
    host.id = "edge-behavior-probe";
    Object.assign(host.style, {
      position: "fixed",
      right: "0",
      bottom: "12px",
      width: "100px",
      zIndex: "20000",
    });
    document.body.append(host);
    DOM.default.createRoot(host).render(
      React.default.createElement(behavior.default, {
        behaviors: [
          {
            id: "edge-action",
            triggers: [{ type: "manual" }],
            outputs: [{ type: "card-draw", amount: 1, target: "self-player" }],
            description:
              "Draw 1 card. This description must remain readable even when the action is against the bottom edge of the viewport.",
          },
        ],
      }),
    );
  });
  const host = page.locator("#edge-behavior-probe");
  const action = host.locator(".behavior-fitting");
  async function verifyEdge(edge: "right" | "left") {
    if (edge === "left") {
      await page.mouse.move(page.viewportSize()!.width / 2, 20);
      await host.evaluate((element) => {
        element.style.right = "auto";
        element.style.left = "0";
      });
    }
    if (info.project.use.hasTouch) {
      await action.tap();
    } else {
      await action.hover();
    }
    const tooltip = page.getByRole("tooltip");
    await expect(tooltip).toContainText("This description must remain readable");
    await expectInsideViewport(tooltip, page);
    const anchorBox = await action.boundingBox();
    const tooltipBox = await tooltip.boundingBox();
    expect(tooltipBox!.y + tooltipBox!.height).toBeLessThanOrEqual(anchorBox!.y + 1);
    await page.screenshot({
      animations: "disabled",
      path: info.outputPath(`tooltip-${edge}-bottom.png`),
    });
    if (info.project.use.hasTouch) {
      await action.tap();
      await expect(tooltip).toHaveCount(0);
    }
  }
  await verifyEdge("right");
  await verifyEdge("left");
});
