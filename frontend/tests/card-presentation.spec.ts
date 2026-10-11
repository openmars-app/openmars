import { test, expect, type Page } from "@playwright/test";

async function openCards(page: Page) {
  await page.goto(
    "/cards?quick&mode=earth-withdrawal&family=corporation&cId=EC01&cId=EC02&cId=TC5&cId=PC3",
  );
  await expect(page.locator("[data-card-id=EC01]")).toBeVisible();
}

test("corporation production rendering uses one horizontal group and complete exception explanations", async ({
  page,
}) => {
  await openCards(page);
  const corp = page.locator('[data-card-id="EC02"]');
  const group = corp.locator("[data-shared-action]");
  await expect(group).toHaveCount(1);
  await expect(group).toContainText("OR");
  await expect(group.getByRole("img", { name: "action", exact: true })).toHaveCount(0);
  await expect(group.locator("[data-rule-exception]")).toHaveCount(1);
  await expect(group.locator('[data-state-symbol="withdrawal-occurred"]')).toHaveCount(0);
  expect(
    await group.evaluate(
      (el) => el.getBoundingClientRect().width > el.getBoundingClientRect().height * 2,
    ),
  ).toBeTruthy();
  await expect(corp).toContainText("once per game before withdrawal");
  const passive = page.locator('[data-card-id="EC01"]');
  await expect(passive.locator("[data-shared-effect]")).toHaveCount(1);
  await expect(passive.locator("[data-state-effect]")).toHaveCount(2);
  await expect(
    passive.locator("[data-state-effect] legend [aria-label='Earth connected']"),
  ).toHaveCount(1);
  await expect(
    passive.locator("[data-state-effect] legend [aria-label='Earth disconnected']"),
  ).toHaveCount(1);
  await expect(passive.locator("[data-state-effect] [data-behavior-qualifiers]")).toHaveCount(0);

  await expect(passive.locator("[data-rule-exception]")).toHaveCount(1);
  await expect(passive.locator('[data-state-symbol="withdrawal-occurred"]')).toHaveCount(0);
  await expect(passive).not.toContainText("disconnected after withdrawal");
  await expect(passive).toContainText("Once per generation");
  await expect(passive).toContainText("a project with an Earth-connected effect");
  const badge = passive.locator("[data-dependent-project]");
  expect(
    await badge.evaluate((el) => {
      const card = el.querySelector('img[alt="card"]')!.getBoundingClientRect();
      const overlay = el.querySelector("span.absolute")!.getBoundingClientRect();
      return overlay.left < card.right && overlay.top < card.bottom && overlay.width < card.width;
    }),
  ).toBeTruthy();
});

test("standard resource choices and one-row colored withdrawal exchanges", async ({
  page,
}, info) => {
  await openCards(page);
  const utopia = page.locator('[data-card-id="TC5"]');
  const shortcut = utopia.locator('[data-standard-choice="exchange"]');
  await expect(shortcut).toHaveAttribute("data-choice-indices", "0,1,2,3,4,5");
  await expect(
    utopia.locator(".corporation-card-description strong").filter({ hasText: "of the same kind" }),
  ).toHaveCount(1);
  if (info.project.use.hasTouch) {
    await page.mouse.move(0, 0);
  }
  await shortcut.scrollIntoViewIfNeeded();
  if (info.project.use.hasTouch) {
    await shortcut.tap();
  } else {
    await shortcut.hover();
  }
  await expect(
    utopia
      .locator(".behavior-fitting")
      .filter({ has: page.locator('[data-standard-choice="exchange"]') }),
  ).toHaveAttribute("aria-expanded", "true");
  await expect(
    page.getByRole("tooltip").locator("strong").filter({ hasText: "of the same kind" }),
  ).toBeVisible();
  await expect(
    page.locator('[data-card-id="PC3"] [data-standard-choice="production"]'),
  ).toHaveCount(1);
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES011&cId=ES016");
  const launch = page.locator('[data-card-id="ES011"]');
  const exchange = launch.locator("[data-compact-exchange]");
  await expect(exchange).toHaveCount(1);
  await expect(exchange.locator('img[alt="energy"]')).toHaveCount(1);
  await expect(exchange).toContainText("3");
  await expect(exchange.locator('[data-withdrawal-direction="earlier"]')).toHaveText("");
  await expect(exchange.getByRole("img", { name: "withdrawal-earlier", exact: true })).toHaveCount(
    1,
  );
  const delay = page.locator('[data-card-id="ES016"] [data-withdrawal-direction="later"]');
  await expect(delay).toHaveText("");
  await expect(delay.getByRole("img", { name: "withdrawal-later", exact: true })).toHaveCount(1);
  expect(
    await exchange.evaluate(
      (el) => el.getBoundingClientRect().height < 35 && el.scrollWidth <= el.clientWidth + 1,
    ),
  ).toBeTruthy();
  expect(
    await launch
      .locator(".behavior-section")
      .evaluate((el) => el.scrollHeight <= el.clientHeight + 2),
  ).toBeTruthy();
});

test("contact duration is centered in the loop and independent of its start offset", async ({
  page,
}) => {
  await page.route("**/api/v1/cards?**", async (route) => {
    const response = await route.fetch();
    const data = await response.json();
    const card = data.cards.find((item: { id: string }) => item.id === "ES041");
    for (const behavior of card.behaviors) {
      for (const output of behavior.outputs ?? []) {
        if (output.type === "schedule-state-window") {
          output.end.generationOffset = output.start.generationOffset + 1;
        }
      }
    }
    await route.fulfill({ response, json: data });
  });
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES041&cId=ES042&cId=ES043");
  const relay = page.locator('[data-card-id="ES041"] [data-behavior-atom="state-window"]');
  await expect(relay).toHaveText("2");
  await expect(relay.locator("span.absolute")).toHaveText("2");
  await expect(
    page.locator('[data-card-id="ES042"] [data-behavior-atom="state-window"]'),
  ).toHaveText("1");
  await expect(
    page.locator('[data-card-id="ES043"] [data-behavior-atom="state-window"]'),
  ).toHaveText("");
});

test("dependency groups preserve two rows, state changes and initial delivery", async ({
  page,
}) => {
  for (const preview of ["connected", "disconnected"]) {
    await page.goto(`/cards?quick&mode=earth-withdrawal&cId=110&cId=C14&preview=${preview}`);
    const business = page.locator('[data-card-id="110"]');
    const group = business.locator("[data-dependency-group]");
    await expect(group).toHaveCount(1);
    const bounds = await group.locator(":scope > div > .behavior-fitting").evaluateAll((els) =>
      els.map((el) => ({
        top: el.getBoundingClientRect().top,
        bottom: el.getBoundingClientRect().bottom,
      })),
    );
    expect(bounds).toHaveLength(2);
    expect(bounds[1].top).toBeGreaterThanOrEqual(bounds[0].bottom);
    await expect(
      group.locator(
        `[data-dependency-status="${preview === "connected" ? "active" : "inactive"}"]`,
      ),
    ).toHaveCount(1);
    await expect(group.locator('[data-semantic-icon="inactive"]')).toHaveCount(0);
    const heavy = page.locator('[data-card-id="C14"]');
    await expect(heavy.locator("[data-dependency-group]")).toHaveText("2");
    await expect(heavy.locator(".behavior-section")).toContainText("4");
  }
});

test("permanent production and immediate imports keep their meaning", async ({ page }, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES042&cId=ES025");
  const badge = page.locator('[data-permanent-production="energy"]');
  await expect(badge.locator('img[alt="energy"]')).toHaveCount(1);
  await expect(badge.getByRole("img", { name: "permanent", exact: true })).toHaveCount(1);
  if (info.project.use.hasTouch) await badge.tap();
  else await badge.hover();
  await expect(
    page.getByText("2 permanent energy production and Earth disconnected.", {
      exact: true,
    }),
  ).toBeVisible();
  const exchange = page.locator('[data-card-id="ES025"] .behavior-section > .behavior-fitting');
  await expect(exchange).toHaveCount(1);
  expect(await exchange.evaluate((el) => getComputedStyle(el).borderTopWidth)).toBe("0px");
  await expect(exchange.getByRole("img", { name: "titanium", exact: true })).toHaveCount(1);
  if (info.project.use.hasTouch) await exchange.tap();
  else await exchange.hover();
  await expect(page.getByRole("tooltip")).toHaveText(
    "Pay 1 titanium to make the Earth-dependent effects of one of your projects permanent.",
  );
});

test("corporation state effects align horizontally", async ({ page }) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&family=corporation&cId=B04&cId=B12");
  for (const id of ["B04", "B12"]) {
    const card = page.locator(`[data-card-id="${id}"]`);
    await expect(card).toBeVisible();
    const rows = await card.locator(".behavior-section > *").evaluateAll((items) =>
      items.map((item) => {
        const rect = item.getBoundingClientRect();
        return { left: rect.left, right: rect.right, center: rect.top + rect.height / 2 };
      }),
    );
    expect(rows).toHaveLength(2);
    expect(rows[1].left).toBeGreaterThanOrEqual(rows[0].right);
    expect(Math.abs(rows[0].center - rows[1].center)).toBeLessThan(2);
  }
});

test("single dependency boxes carry their state icon on their own top border", async ({ page }) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=013&cId=C14&cId=137");
  for (const id of ["013", "C14", "137"]) {
    const group = page.locator(`[data-card-id="${id}"] [data-dependency-group]`);
    await expect(group).toHaveCount(1);
    await expect(group.locator("fieldset")).toHaveCount(0);
    const label = group.locator("[data-behavior-border-label]");
    await expect(label).toHaveCount(1);
    const alignment = await label.evaluate((element) => {
      const badge = element.getBoundingClientRect();
      const box = element.parentElement!.getBoundingClientRect();
      return {
        vertical: Math.abs(badge.top + badge.height / 2 - box.top),
        horizontal: Math.abs(badge.left + badge.width / 2 - (box.left + box.width / 2)),
        border: getComputedStyle(element.parentElement!).borderTopWidth,
      };
    });
    expect(alignment.vertical).toBeLessThan(2);
    expect(alignment.horizontal).toBeLessThan(2);
    expect(alignment.border).toBe("1px");
  }
});

test("three resource alternatives use quantities and conditioned alternatives stay horizontal", async ({
  page,
}) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=019&cId=ES010");
  const hydrogen = page.locator('[data-card-id="019"] .behavior-section');
  for (const resource of ["plant", "microbe", "animal"]) {
    await expect(hydrogen.locator(`img[alt="${resource}"]`)).toHaveCount(1);
  }
  await expect(hydrogen.locator('[data-choice-index="0"]')).toContainText("3");
  await expect(hydrogen.locator('[data-choice-index="1"]')).toContainText("3");
  await expect(hydrogen.locator('[data-choice-index="2"]')).toContainText("2");
  const choices = page.locator('[data-card-id="ES010"] [data-choice-index]');
  await expect(choices).toHaveCount(2);
  const bounds = await choices.evaluateAll((elements) =>
    elements.map((element) => {
      const rect = element.getBoundingClientRect();
      return { left: rect.left, right: rect.right, center: rect.top + rect.height / 2 };
    }),
  );
  expect(bounds[1].left).toBeGreaterThanOrEqual(bounds[0].right);
  expect(Math.abs(bounds[0].center - bounds[1].center)).toBeLessThan(2);
});

test("permanent card markers preserve card size and action conditions stay legible", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES024&size=large");
  const relocation = page.locator('[data-card-id="ES024"]');
  await expect(relocation).toBeVisible();
  const marked = relocation.locator('[data-semantic-icon="technology-import"] img');
  const plain = relocation.locator('.behavior-section [role="img"][aria-label="Card"] img');
  const markedBounds = await marked.boundingBox();
  const plainBounds = await plain.boundingBox();
  expect(markedBounds!.width).toBeCloseTo(plainBounds!.width, 0);
  expect(markedBounds!.height).toBeCloseTo(plainBounds!.height, 0);
  const marker = await relocation.locator('[data-card-operation-marker="permanent"]').boundingBox();
  expect(marker!.x).toBeGreaterThan(markedBounds!.x + markedBounds!.width / 2);
  expect(marker!.y).toBeGreaterThan(markedBounds!.y + markedBounds!.height / 2);
  await relocation
    .locator(".behavior-section")
    .screenshot({ path: info.outputPath("permanent-card.png") });

  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES020&size=large");
  const archive = page.locator('[data-card-id="ES020"]');
  const action = archive.locator("[data-shared-action]");
  await expect(action.locator("[data-rule-exception]")).toHaveCount(1);
  await expect(action).not.toContainText("≤");
  const state = await action.locator('[data-semantic-icon="transition-pending"]').boundingBox();
  const credit = await action.locator('img[alt="credit"]').boundingBox();
  expect(state!.width).toBeCloseTo(credit!.width, 0);
  await action.screenshot({ path: info.outputPath("archive-action.png") });
});

test("inspection labels scale and cards resist native selection", async ({ page }, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES040");
  const label = page.locator('[data-card-id="ES040"] .game-card-labels');
  await expect(label).toBeVisible();
  const smallHeight = (await label.boundingBox())!.height;
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES040&size=large");
  await expect(label).toBeVisible();
  expect((await label.boundingBox())!.height).toBeGreaterThan(smallHeight * 1.5);
  await expect(label).not.toContainText("≤");
  await expect(label.locator('[data-semantic-icon="standard-resource"]')).toHaveCount(1);
  await expect(label.locator('[data-semantic-icon="permanent"]')).toHaveCount(1);
  const card = page.locator('[data-card-id="ES040"] .game-card');
  const selectable = await card.evaluate(
    (element) =>
      [element, ...element.querySelectorAll("*")].filter(
        (node) => getComputedStyle(node).userSelect !== "none",
      ).length,
  );
  expect(selectable).toBe(0);
  const title = await card.locator(".game-card-title").boundingBox();
  await page.mouse.move(title!.x + 5, title!.y + 10);
  await page.mouse.down();
  await page.mouse.move(title!.x + title!.width - 5, title!.y + title!.height - 5, { steps: 10 });
  await page.mouse.up();
  expect(await page.evaluate(() => window.getSelection()?.toString())).toBe("");
  await label.screenshot({ path: info.outputPath("inspection-vp.png") });
});

test("requirements have a description label and Municipal Power Exchange requires a city on play", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES038&size=large");
  const card = page.locator('[data-card-id="ES038"]');
  const description = card.locator(".game-card-description");
  await expect(description.locator("strong").filter({ hasText: "Requirement:" })).toHaveCount(1);
  await expect(description).toContainText("Requirement: 1 city you own on Mars.");
  await expect(card.locator(".game-card-requirements")).not.toBeEmpty();
  await expect(card.locator("[data-behavior-qualifiers] [data-rule-exception]")).toHaveCount(0);
  await expect(card.locator(".behavior-section")).toContainText("*");
  await description.screenshot({ path: info.outputPath("requirement-description.png") });
});

test("action choices automatically wrap to their available width and retain shared costs", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES006&size=large");
  const card = page.locator('[data-card-id="ES006"] .game-card');
  const choices = card.locator("[data-action-choices]");
  await expect(choices).toBeVisible();
  await expect(choices.locator("[data-shared-choice-cost]")).toHaveText("3");
  await expect(choices.locator("[data-action-choice]")).toHaveCount(3);
  const rowCount = () =>
    choices
      .locator("[data-action-choice]")
      .evaluateAll(
        (elements) =>
          new Set(elements.map((el) => Math.round(el.getBoundingClientRect().top))).size,
      );
  await expect.poll(rowCount).toBe(1);
  await choices.screenshot({ path: info.outputPath("dispatch-wide.png") });
  await card.evaluate((element) => {
    element.style.width = "190px";
  });
  await expect.poll(rowCount).toBeGreaterThan(1);
  expect(
    await choices.evaluate((element) => element.scrollWidth <= element.clientWidth + 1),
  ).toBeTruthy();
  await choices.screenshot({ path: info.outputPath("dispatch-narrow.png") });
  await card.evaluate((element) => {
    element.style.removeProperty("width");
  });
  await expect.poll(rowCount).toBe(1);
});

declare global {
  interface Window {
    optionalEffectProbe: {
      show: (id: string, available: boolean) => void;
      requests: Array<{ resolutionId: string; accept: boolean }>;
    };
  }
}

test("optional payments have explicit pay and skip controls with no empty choices", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES014");
  await expect(page.locator('[data-card-id="ES014"] [data-optional-choice]')).toContainText("MAY");
  await page.evaluate(async () => {
    const load = (path: string) => import(/* @vite-ignore */ path);
    const [React, DOM, overlay, visuals, connection] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/overlay/BehaviorResolutionOverlay.tsx",
        "/src/components/ui/cards/CardStateVisuals.tsx",
        "/src/services/globalWebSocketManager.ts",
      ].map(load),
    );
    const { cards } = await (
      await fetch("/api/v1/cards?gameModeId=earth-withdrawal&limit=1000")
    ).json();
    const { gameModes } = await (await fetch("/api/v1/game-modes")).json();
    const host = document.createElement("div");
    document.body.append(host);
    const root = DOM.default.createRoot(host);
    const R = React.default;
    const probe: Window["optionalEffectProbe"] = { show: () => {}, requests: [] };
    window.optionalEffectProbe = probe;
    connection.globalWebSocketManager.confirmOptionalBehavior = async (
      resolutionId: string,
      accept: boolean,
    ) => {
      probe.requests.push({ resolutionId, accept });
      root.render(null);
    };
    probe.show = (id, available) => {
      const card = cards.find((item: { id: string }) => item.id === id);
      const behavior = card.behaviors.find((item: { optional?: boolean }) => item.optional);
      const props = {
        resolutions: [
          {
            id: `optional-${id}`,
            kind: "optional",
            source: card.name,
            sourceCardId: id,
            sourceBehaviorIndex: 1,
            minCards: 0,
            maxCards: 0,
            optionalEffect: {
              behavior,
              available,
              errors: available ? [] : [{ message: "Withdrawal has already happened" }],
            },
          },
        ],
        handCards: [],
        playedCards: [card],
      };
      root.render(
        R.createElement(
          visuals.CardCatalogPresentation,
          { modes: gameModes, modeId: "earth-withdrawal" },
          R.createElement(
            visuals.CardStatePresentation,
            { card },
            R.createElement(overlay.default, props),
          ),
        ),
      );
    };
  });
  for (const [id, label] of [
    ["ES011", "Pay 3 energy"],
    ["ES014", "Pay 3 M€"],
    ["ES017", "Decrease energy production"],
  ]) {
    await page.evaluate((id) => window.optionalEffectProbe.show(id, true), id);
    await expect(page.getByText("Optional effect", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: label, exact: true })).toBeEnabled();
    await expect(page.getByRole("button", { name: "Skip", exact: true })).toBeVisible();
    await expect(page.getByText(/^Choice [12]$/)).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Cancel", exact: true })).toHaveCount(0);
    await page.screenshot({ path: info.outputPath(`optional-${id}.png`), animations: "disabled" });
    await page.getByRole("button", { name: label, exact: true }).click();
    expect(await page.evaluate(() => window.optionalEffectProbe.requests.at(-1))).toEqual({
      resolutionId: `optional-${id}`,
      accept: true,
    });
    await page.evaluate((id) => window.optionalEffectProbe.show(id, false), id);
    await expect(page.getByRole("button", { name: label, exact: true })).toBeDisabled();
    await expect(page.getByText("Withdrawal has already happened", { exact: true })).toBeVisible();
    await page.screenshot({
      path: info.outputPath(`optional-${id}-unavailable.png`),
      animations: "disabled",
    });
    await page.getByRole("button", { name: "Skip", exact: true }).click();
    expect(await page.evaluate(() => window.optionalEffectProbe.requests.at(-1))).toEqual({
      resolutionId: `optional-${id}`,
      accept: false,
    });
  }
});

test("state change notifications reuse the card symbol and keep the VP badge separate", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES014");
  await expect(page.locator('[data-card-id="ES014"]')).toBeVisible();
  await page.evaluate(async () => {
    const load = (path: string) => import(/* @vite-ignore */ path);
    const [React, DOM, toast] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/cards/TriggeredEffectToast.tsx",
      ].map(load),
    );
    const { cards } = await (
      await fetch("/api/v1/cards?gameModeId=earth-withdrawal&limit=1000")
    ).json();
    const card = cards.find((item: { id: string }) => item.id === "ES014");
    const operation = card.behaviors.find((item: { optional?: boolean }) => item.optional)
      .outputs[0];
    const host = document.createElement("div");
    host.id = "notification-preview";
    document.body.append(host);
    DOM.default.createRoot(host).render(
      React.default.createElement(toast.default, {
        item: {
          id: "notification",
          cardName: card.name,
          sourceType: "card_play",
          behaviors: [],
          vpConditions: card.vpConditions,
          calculatedOutputs: [
            { resourceType: "card-draw", amount: 2 },
            { resourceType: "state-change", amount: 1, operation },
          ],
        },
      }),
    );
  });
  const preview = page.locator("#notification-preview");
  await expect(preview.locator('[data-withdrawal-direction="later"]')).toHaveCount(1);
  await expect(preview.locator('[data-semantic-icon="effect"]')).toHaveCount(0);
  await expect(preview).toContainText(/1\s*VP/);
  await preview.screenshot({
    path: info.outputPath("state-notification.png"),
    animations: "disabled",
  });
});
