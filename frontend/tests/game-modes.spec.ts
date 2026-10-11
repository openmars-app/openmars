import { expect, test } from "@playwright/test";

test("mode selection exposes scenario rules, parameters and independent options", async ({
  page,
}, info) => {
  await page.goto("/create?quick");
  await page.getByRole("button", { name: "Options", exact: true }).click();
  const venus = page.getByRole("checkbox", { name: /^Venus/ });
  const colonies = page.getByRole("checkbox", { name: /^Colonies/ });
  const preludes = page.getByRole("checkbox", { name: /^Preludes/ });
  await expect(venus).not.toBeChecked();
  await expect(colonies).not.toBeChecked();
  await expect(preludes).toBeChecked();
  await venus.check();
  await colonies.check();
  await expect(venus).toBeChecked();
  await expect(colonies).toBeChecked();
  await page
    .getByRole("group", { name: "Options", exact: true })
    .screenshot({ path: info.outputPath("classic-options.png") });
  await page.getByRole("button", { name: "Game mode", exact: true }).click();
  await page.getByRole("option", { name: "Earth Withdrawal", exact: true }).click();
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toContainText(
    "Earth Withdrawal",
  );
  await expect(venus).toHaveCount(0);
  await expect(colonies).toHaveCount(0);
  await expect(page.getByRole("checkbox", { name: /^Project Funding/ })).toHaveCount(0);
  await expect(preludes).toBeChecked();
  await preludes.uncheck();
  await expect(preludes).not.toBeChecked();
  const parameter = page.getByRole("button", {
    name: /withdrawal.*generation|last.*generation|contact.*generation/i,
  });
  await expect(parameter).toContainText("6");
  await parameter.click();
  await page.getByRole("option", { name: "4", exact: true }).click();
  await expect(parameter).toContainText("4");
  await page.getByRole("button", { name: "Rules", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Earth Withdrawal rules" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Make effects permanent", exact: true }),
  ).toBeVisible();
  await expect(page.locator("[data-mode-overview] section")).toHaveCount(4);
  const details = page.getByRole("button", { name: "Detailed rules", exact: true });
  await expect(details).toHaveAttribute("aria-expanded", "false");
  await page.screenshot({ path: info.outputPath("rules-overview.png"), animations: "disabled" });
  const overview = page.locator("[data-mode-overview]");
  const overviewBefore = await overview.boundingBox();
  expect(
    await overview.evaluate((el) => getComputedStyle(el).gridTemplateColumns.split(" ").length),
  ).toBe(2);
  await details.click();
  expect((await overview.boundingBox())?.width).toBe(overviewBefore?.width);
  await expect(
    page.getByRole("heading", { name: "Changing the deadline", exact: true }),
  ).toBeVisible();
  await details.click();
  await page.getByRole("button", { name: "Card symbols", exact: true }).click();
  const legend = page.getByRole("region", { name: "Card symbols", exact: true });
  await expect(legend).not.toContainText("Any standard resource");
  await expect(legend).not.toContainText("Special rule or use limit");
  await expect(legend).not.toContainText("Research boundary");
  await expect(legend.locator('[data-semantic-icon="permanent"]')).toHaveCount(1);
  await page.getByRole("button", { name: "Close rules", exact: true }).click();
  await page.getByRole("button", { name: "Game mode", exact: true }).click();
  await page.getByRole("option", { name: "Classic", exact: true }).click();
  await expect(parameter).toHaveCount(0);
  await expect(venus).not.toBeChecked();
  await expect(colonies).not.toBeChecked();
  await expect(preludes).not.toBeChecked();
});

test("card browser uses resolved costs and previews dependency state", async ({ page }) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=110&preview=connected");
  await expect(page.getByText("Business Network", { exact: true }).first()).toBeVisible();
  await expect(page.locator("[data-dependency-status=active]").first()).toBeVisible();
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=110&preview=disconnected");
  await expect(page.locator("[data-dependency-status=inactive]").first()).toBeVisible();
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

test("card symbols preserve existing effects and scenario conditions without printed prose", async ({
  page,
}) => {
  const cases = [
    { id: "173", name: "Protected Habitats", icon: "defense" },
    { id: "C14", name: "Heavy Taxation", icon: "connection-open" },
    { id: "ES013", name: "Subsidy Renunciation", icon: "operation-retired" },
    { id: "ES021", name: "Technology Import Agreement", icon: "technology-import" },
    { id: "ES031", name: "Last-Mile Warehouse", icon: "transition" },
    { id: "ES041", name: "Deep-Space Relay Reacquisition", icon: "contact-window-generation" },
    { id: "ES043", name: "Packetized Supply Burst", icon: "contact-window-output" },
  ];
  test.setTimeout(90_000);
  for (const [index, example] of cases.entries()) {
    if (index % 3 === 0) {
      const selection = cases
        .slice(index, index + 3)
        .map(({ id }) => `cId=${id}`)
        .join("&");
      await page.goto(`/cards?quick&mode=earth-withdrawal&${selection}`);
    }
    const card = page.locator(`[data-card-name="${example.name}"]`);
    await expect(card).toBeVisible();
    await expect(card.locator(`[data-semantic-icon="${example.icon}"]`).first()).toBeVisible();
    await expect(card.locator("[data-unsupported-icon]")).toHaveCount(0);
    const printed = card.locator(".game-card-behaviors,.game-card-requirements");
    const words = (await printed.allTextContents())
      .join(" ")
      .replaceAll("OR", "")
      .replaceAll("VP", "");
    expect(words).not.toMatch(/[a-z]{3,}/i);
    expect(
      await card.locator(".behavior-flow").evaluateAll((rows) =>
        rows.some((row) => {
          const bounds = row.closest(".behavior-section")!.getBoundingClientRect();
          const rect = row.getBoundingClientRect();
          return rect.left < bounds.left - 2 || rect.left + row.scrollWidth > bounds.right + 2;
        }),
      ),
    ).toBeFalsy();
    if (example.id === "C14") {
      await expect(card.locator('.game-card-requirements img[alt="earth-tag"]')).toBeVisible();
      await expect(
        card.locator('.game-card-requirements [data-state-symbol="earth-contact"]'),
      ).toBeVisible();
      const section = card.locator(".behavior-section");
      expect(
        await section.evaluate((element) => element.scrollHeight <= element.clientHeight + 2),
      ).toBeTruthy();
    }
    if (example.id === "ES041" || example.id === "ES043") {
      const window = card.locator('[data-behavior-atom="state-window"]');
      await expect(window.locator(`[data-semantic-icon="${example.icon}"]`)).toBeVisible();
    }
  }
});

test("conditional VP keeps state symbols while ordinary VP preserves its established display", async ({
  page,
}) => {
  await page.goto("/cards?quick&mode=classic&cId=005");
  const classic = page.locator('[data-card-name="Search For Life"]');
  await expect(classic).toBeVisible();
  await expect(classic.locator(".game-card-labels")).not.toContainText("≤");
  for (const id of ["ES039", "ES040"]) {
    await page.goto(`/cards?quick&mode=earth-withdrawal&cId=${id}`);
    const label = page.locator(".game-card-labels");
    await expect(label.locator('[data-state-symbol="withdrawal-occurred"]')).toBeVisible();
    await expect(label).not.toContainText("≤");
    await expect(label).not.toContainText("type");
  }
});

test("value modifiers and optional payments keep all effects without an empty alternative", async ({
  page,
}) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=071&cId=ES011");
  const alloys = page.locator('[data-card-name="Advanced Alloys"]');
  await expect(alloys.locator("[data-value-modifier]")).toHaveCount(2);
  await expect(alloys.locator('[data-value-modifier] img[alt="steel"]')).toBeVisible();
  await expect(alloys.locator('[data-value-modifier] img[alt="titanium"]')).toBeVisible();
  await expect(alloys.locator('[data-value-modifier] img[alt="credit"]')).toHaveCount(2);
  const handover = page.locator('[data-card-name="Launch Control Handover"] .behavior-section');
  await expect(handover.locator('[data-optional-choice="true"]')).toHaveCount(1);
  await expect(handover.getByText("OR", { exact: true })).toHaveCount(0);
  await expect(handover.locator('[data-behavior-atom="state-change"]')).toBeVisible();
  const cardResponse = await page.request.get(
    "/api/v1/cards?gameModeId=earth-withdrawal&limit=1000",
  );
  const cards = (await cardResponse.json()).cards as {
    id: string;
    behaviors: { choices?: { inputs?: unknown[]; outputs?: unknown[] }[] }[];
  }[];
  const payment = cards
    .find((card) => card.id === "ES011")
    ?.behaviors.find((behavior) => behavior.choices?.length);
  expect(payment?.choices).toHaveLength(2);
  expect(
    payment?.choices?.some((choice) => !choice.inputs?.length && !choice.outputs?.length),
  ).toBeTruthy();
});
