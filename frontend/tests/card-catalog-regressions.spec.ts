import { test, expect } from "@playwright/test";

test.setTimeout(90_000);

test("card requirements, wrapped production and action tooltips stay scoped", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&cId=186&cId=058&cId=C02&cId=C03&cId=C41");
  const radSuits = page.locator('[data-card-id="186"]');
  await expect(radSuits.locator('.game-card-requirements img[alt="city-tile"]')).toHaveCount(1, {
    timeout: 30_000,
  });
  await expect(radSuits.locator(".game-card-requirements")).toContainText("2+");
  await expect(page.locator('[data-card-id="C02"] .game-card-requirements > *')).toHaveCount(0);
  const beam = page.locator('[data-card-id="058"]');
  await beam.scrollIntoViewIfNeeded();
  const production = beam.locator("[data-production-box]");
  await expect(production.locator('img[alt="energy-production"]')).toHaveCount(3);
  await expect(production.locator('img[alt="heat-production"]')).toHaveCount(3);
  await expect
    .poll(() => production.evaluate((el) => el.getBoundingClientRect().width))
    .toBeLessThan(110);
  await beam.screenshot({ path: info.outputPath("beam-compact.png") });
  await radSuits.scrollIntoViewIfNeeded();
  await radSuits.screenshot({ path: info.outputPath("rad-suits.png") });
  const atmo = page.locator('[data-card-id="C03"]');
  await atmo.scrollIntoViewIfNeeded();
  const actionBox = atmo.locator(".behavior-fitting").last();
  if (info.project.use.hasTouch) {
    await actionBox.tap();
  } else {
    await actionBox.hover();
  }
  const tooltip = page.getByRole("tooltip");
  await expect(tooltip).toContainText("spend 1 floater here");
  await expect(tooltip).not.toContainText("Add 2 floaters");
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("atmo-action-tooltip.png"),
  });
});

test("virtual card rows remain aligned through scrolling and size changes", async ({
  page,
}, info) => {
  await page.goto("/cards");
  const viewport = page.getByRole("region", { name: "Card results" }).first();
  await expect(page.locator('[data-card-id="001"]')).toBeVisible({ timeout: 30_000 });
  for (const offset of [250, 900, 1600, 600, 0]) {
    await viewport.evaluate((el, offset) => {
      el.scrollTop = offset;
    }, offset);
    await expect.poll(() => viewport.evaluate((el) => Math.round(el.scrollTop))).toBe(offset);
    const rows = viewport.locator("[data-card-row]");
    expect(await rows.count()).toBeLessThan(12);
    expect(
      await rows.evaluateAll((els) =>
        els.every((row) => {
          const tops = [...row.querySelectorAll(".game-card-body")].map(
            (card) => card.getBoundingClientRect().top,
          );
          return tops.every((top) => Math.abs(top - tops[0]) < 1);
        }),
      ),
    ).toBeTruthy();
  }
  await viewport.hover();
  await page.mouse.wheel(0, 520);
  await page.screenshot({ animations: "disabled", path: info.outputPath("catalog-scrolling.png") });
  await page.goto("/cards?size=large&cId=058");
  await expect(page.locator('[data-card-id="058"]')).toBeVisible({ timeout: 30_000 });
  const box = page.locator('[data-card-id="058"] [data-production-box]');
  await expect(box.locator("img")).toHaveCount(6);
  await box.scrollIntoViewIfNeeded();
  await expect(box.locator("img").first()).toBeVisible();
  await box.screenshot({ path: info.outputPath("beam-large-production.png") });
});
