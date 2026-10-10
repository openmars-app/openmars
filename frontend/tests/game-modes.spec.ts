import { expect, test } from "@playwright/test";

test("mode selection exposes scenario rules, parameters and independent options", async ({
  page,
}) => {
  await page.goto("/create?quick");
  await page.getByRole("button", { name: "Game mode", exact: true }).click();
  await page.getByRole("option", { name: "Earth Withdrawal", exact: true }).click();
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toContainText(
    "Earth Withdrawal",
  );
  await expect(page.getByText("Experimental features", { exact: true })).toBeVisible();
  const parameter = page.getByRole("button", {
    name: /withdrawal.*generation|last.*generation|contact.*generation/i,
  });
  await expect(parameter).toContainText("6");
  await parameter.click();
  await page.getByRole("option", { name: "4", exact: true }).click();
  await expect(parameter).toContainText("4");
  await page.getByRole("button", { name: "Rules", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Earth Withdrawal rules" })).toBeVisible();
  await expect(page.getByText(/technology/i).first()).toBeVisible();
  await page.getByRole("button", { name: "Close rules", exact: true }).click();
  await page.getByRole("button", { name: "Game mode", exact: true }).click();
  await page.getByRole("option", { name: "Classic", exact: true }).click();
  await expect(parameter).toHaveCount(0);
});

test("card browser uses resolved costs and previews dependency state", async ({ page }) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=110&preview=connected");
  await expect(page.getByText("Business Network", { exact: true }).first()).toBeVisible();
  await expect(page.getByText(/Earth.*Online/i).first()).toBeVisible();
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=110&preview=disconnected");
  await expect(page.getByText(/Earth.*Offline/i).first()).toBeVisible();
  const connected = await page.request.get(
    "/api/v1/cards?gameModeId=earth-withdrawal&preview=connected&limit=1000",
  );
  const classic = await page.request.get("/api/v1/cards?gameModeId=classic&limit=1000");
  expect(connected.ok()).toBeTruthy();
  expect(classic.ok()).toBeTruthy();
  const scenarioCards = (await connected.json()).cards as { id: string; cost: number }[];
  const classicCards = (await classic.json()).cards as { id: string; cost: number }[];
  expect(scenarioCards.find((card) => card.id === "110")?.cost).toBe(2);
  expect(classicCards.find((card) => card.id === "110")?.cost).toBe(4);
  expect(classicCards.some((card) => card.id.startsWith("ES"))).toBeFalsy();

  await page.goto("/cards?quick&mode=earth-withdrawal&family=corporation&cId=EC01&cId=EC02");
  for (const name of ["Terran Supply Authority", "Martian Independence Cooperative"]) {
    const logo = page.getByRole("img", { name, exact: true });
    await expect(logo).toBeVisible();
    await expect(logo).toHaveJSProperty("complete", true);
    await expect(logo).not.toHaveJSProperty("naturalWidth", 0);
  }
});
