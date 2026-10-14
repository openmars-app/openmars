import { test, expect } from "@playwright/test";

test("colony bonus previews require player state, including an empty bonus", async ({ page }) => {
  await page.goto("/cards?quick&cId=C30");
  const card = page.locator('[data-card-id="C30"]');
  await expect(card).toBeVisible();
  await expect(card.locator("[data-colony-bonus-preview]")).toHaveCount(0);
  for (const outputs of [[], [{ resourceType: "credit", amount: 4, isScaled: true }]]) {
    await page.route("**/api/v1/cards?**", async (route) => {
      const response = await route.fetch();
      const data = await response.json();
      const outpost = data.cards.find((item: { id: string }) => item.id === "C30");
      Object.assign(outpost, {
        available: true,
        effectiveCost: 0,
        computedValues: [{ target: "behaviors::0", outputs }],
      });
      await route.fulfill({ response, json: data });
    });
    await page.reload();
    const preview = card.locator("[data-colony-bonus-preview]");
    await expect(preview).toHaveCount(1);
    if (outputs.length) await expect(preview).toContainText("4");
    else await expect(preview.locator("svg")).toHaveCount(1);
    await page.unroute("**/api/v1/cards?**");
  }
});

test("Molecular Printing keeps city and colony credit previews distinct", async ({
  page,
}, info) => {
  await page.route("**/api/v1/cards?**", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    const card = data.cards.find((item: { id: string }) => item.id === "C27");
    if (card) {
      Object.assign(card, {
        available: true,
        effectiveCost: card.cost,
        computedValues: [
          {
            target: "behaviors::0",
            outputs: [
              { resourceType: "credit", amount: 3, isScaled: true },
              { resourceType: "credit", amount: 1, isScaled: true },
            ],
          },
        ],
      });
    }
    await route.fulfill({ response, json: data });
  });
  for (const size of ["small", "large"]) {
    await page.goto(`/cards?quick&cId=C27&size=${size}`);
    const card = page.locator('[data-card-id="C27"]');
    const computed = card.locator("[data-computed-amount]");
    await expect(computed).toHaveCount(2);
    await expect(computed.nth(0)).toHaveAttribute("data-computed-amount", "3");
    await expect(computed.nth(1)).toHaveAttribute("data-computed-amount", "1");
    await expect(computed.nth(0)).toContainText("3");
    await expect(computed.nth(1)).toContainText("1");
    const behavior = card.locator(".behavior-section");
    await behavior.scrollIntoViewIfNeeded();
    await behavior.screenshot({ path: info.outputPath(`molecular-printing-${size}.png`) });
  }
});
