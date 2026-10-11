import { expect, test, type Locator, type Page } from "@playwright/test";

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

test.skip(!process.env.E2E_BACKEND, "Set E2E_BACKEND=1 with the Go backend running on port 3001");
test.skip(({ isMobile }) => !isMobile, "Compact in-game layout only");

const TILE_FIRST_ACTION_CORPORATION = "Tharsis Republic";
const STARTING_FLOW_BUTTONS = ["Start game", "Next", "Confirm Selection", "Confirm Skip"];

async function clickIfReady(locator: Locator): Promise<boolean> {
  if ((await locator.count()) !== 1) {
    return false;
  }
  if (!(await locator.isVisible()) || !(await locator.isEnabled())) {
    return false;
  }
  await locator.click({ timeout: 2000 });
  return true;
}

async function pickStartingChoices(page: Page) {
  const currentStep = page.getByRole("list", { name: "Steps" }).locator("[aria-current='step']");
  if (!(await currentStep.isVisible())) {
    return;
  }
  const step = (await currentStep.textContent()) ?? "";
  if (step.includes("Corp")) {
    const chosen = page.locator(".corporation-card[data-module-state='armed']");
    if ((await chosen.count()) === 0) {
      await page
        .locator(".corporation-card")
        .filter({ hasNotText: TILE_FIRST_ACTION_CORPORATION })
        .first()
        .click();
    }
    return;
  }
  if (step.includes("Prelude")) {
    const next = page.getByRole("button", { name: "Next", exact: true });
    const open = page.locator("[role='checkbox'][aria-checked='false'][aria-disabled='false']");
    if (!(await next.isEnabled()) && (await open.count()) > 0) {
      await open.first().click();
    }
  }
}

async function advanceStartingFlow(page: Page) {
  await pickStartingChoices(page);
  const prompt = page.getByRole("dialog").getByRole("button", { name: "Close", exact: true });
  if (await clickIfReady(prompt)) {
    return;
  }
  for (const name of STARTING_FLOW_BUTTONS) {
    if (await clickIfReady(page.getByRole("button", { name, exact: true }))) {
      return;
    }
  }
}

async function boxOf(locator: Locator): Promise<Box> {
  const box = await locator.boundingBox();
  if (!box) {
    throw new Error("HUD part has no bounding box");
  }
  return box;
}

function overlaps(a: Box, b: Box) {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

function elementsOutsideViewport(page: Page, selector: string) {
  return page.evaluate((rootSelector) => {
    const scrolls = (element: Element) => {
      const { overflowX, overflowY } = getComputedStyle(element);
      return [overflowX, overflowY].some((value) => value === "auto" || value === "scroll");
    };
    const insideScroller = (element: Element, root: Element) => {
      for (let node = element.parentElement; node && node !== root; node = node.parentElement) {
        if (scrolls(node)) {
          return true;
        }
      }
      return false;
    };
    const offenders: string[] = [];
    for (const root of document.querySelectorAll(rootSelector)) {
      for (const element of [root, ...root.querySelectorAll("*")]) {
        if (element !== root && insideScroller(element, root)) {
          continue;
        }
        const rect = element.getBoundingClientRect();
        if (rect.width === 0 || rect.height === 0) {
          continue;
        }
        const outside =
          rect.left < -1 ||
          rect.top < -1 ||
          rect.right > window.innerWidth + 1 ||
          rect.bottom > window.innerHeight + 1;
        if (outside) {
          const label = (element.getAttribute("aria-label") ?? element.textContent ?? "").trim();
          offenders.push(`<${element.tagName.toLowerCase()}> "${label.slice(0, 40)}"`);
        }
      }
    }
    return offenders;
  }, selector);
}

test("compact in-game HUD with scenario status", async ({ page }) => {
  test.setTimeout(240_000);
  const landscape = page.viewportSize();
  if (!landscape) {
    throw new Error("Project has no viewport");
  }

  await page.goto("/create?quick");
  await page.getByLabel("Your name").fill("Mobile E2E");
  await page.getByRole("button", { name: "Game mode", exact: true }).click();
  await page.getByRole("option", { name: "Earth Withdrawal", exact: true }).click();
  await page.getByRole("button", { name: "Options", exact: true }).click();
  await page.getByRole("checkbox", { name: /^Preludes/ }).uncheck();
  await page.getByRole("button", { name: "Create lobby" }).click();

  const lobbyTopBar = page.getByTestId("mobile-lobby-top-bar");
  await expect(lobbyTopBar).toBeVisible();
  await expect(lobbyTopBar.getByRole("tab")).toHaveText(["Players", "Chat", "Map"]);
  await expect(lobbyTopBar.getByRole("button", { name: "Start game", exact: true })).toBeVisible();
  expect(
    await elementsOutsideViewport(page, "[data-testid='mobile-lobby']"),
    "Lobby outside viewport",
  ).toEqual([]);

  await lobbyTopBar.getByRole("tab", { name: "Map" }).click();
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toContainText(
    "Earth Withdrawal",
  );
  await expect(page.getByRole("checkbox", { name: /^Preludes/ })).not.toBeChecked();
  await lobbyTopBar.getByRole("tab", { name: "Players" }).click();

  const dock = page.getByRole("navigation", { name: "Game dock" });
  await expect(async () => {
    await advanceStartingFlow(page);
    await expect(dock).toBeVisible({ timeout: 1000 });
    await expect(page.getByRole("dialog")).toHaveCount(0, { timeout: 1000 });
  }).toPass({ timeout: 180_000, intervals: [500] });

  const topBar = page.getByTestId("mobile-top-bar");
  const leftRail = page.getByTestId("mobile-resource-rail");
  const rightRail = page.getByTestId("mobile-status-rail");
  const hud = [topBar, leftRail, rightRail, dock];
  for (const part of hud) {
    await expect(part).toBeVisible();
  }

  const boxes = await Promise.all(hud.map(boxOf));
  const names = ["top bar", "resource rail", "status rail", "dock"];
  const overlapping: string[] = [];
  boxes.forEach((box, i) => {
    boxes.slice(i + 1).forEach((other, offset) => {
      if (overlaps(box, other)) {
        overlapping.push(`${names[i]} / ${names[i + 1 + offset]}`);
      }
    });
  });
  expect(overlapping, "HUD parts overlap").toEqual([]);

  const hudSelector =
    "[data-testid='mobile-top-bar'], [data-testid='mobile-resource-rail'], [data-testid='mobile-status-rail'], nav[aria-label='Game dock']";
  expect(await elementsOutsideViewport(page, hudSelector), "HUD outside viewport").toEqual([]);

  await expect(rightRail.getByRole("button", { name: /status and rules/ })).toHaveCount(0);
  const gamemodeButton = topBar.getByRole("button", {
    name: "Gamemode",
    exact: true,
  });
  const travelButton = topBar.getByRole("button", {
    name: "TRAVEL",
    exact: true,
  });
  const modeBox = await boxOf(gamemodeButton);
  const travelBox = await boxOf(travelButton);
  expect(modeBox.x + modeBox.width).toBeLessThanOrEqual(travelBox.x);
  await gamemodeButton.click();
  await expect(page.getByRole("heading", { name: "Earth Withdrawal rules" })).toBeVisible();
  const rulesBox = await boxOf(page.getByRole("dialog", { name: "Earth Withdrawal rules" }));
  expect(rulesBox.x).toBe(0);
  expect(rulesBox.y).toBe(0);
  expect(rulesBox.width).toBe(landscape.width);
  expect(rulesBox.height).toBe(landscape.height);
  await expect(page.getByRole("heading", { name: /Current status/ })).toHaveCount(0);
  await expect(page.getByRole("columnheader", { name: "Permanent", exact: true })).toHaveCount(0);
  await expect(page.locator("[data-mode-overview] > section")).toHaveCount(4);
  await page.getByRole("button", { name: "Close rules", exact: true }).click();

  const entries = dock.getByRole("button");
  const entryCount = await entries.count();
  expect(entryCount).toBeGreaterThan(0);
  for (let index = 0; index < entryCount; index++) {
    await entries.nth(index).click();
    const screen = page.getByRole("dialog");
    await expect(screen).toBeVisible();
    await screen.getByRole("button", { name: /^Close / }).click();
    await expect(screen).toBeHidden();
  }

  const rotate = page.getByText("Rotate your device");
  await page.setViewportSize({
    width: landscape.height,
    height: landscape.width,
  });
  await expect(rotate).toBeVisible();
  await page.setViewportSize(landscape);
  await expect(rotate).toBeHidden();
  await expect(dock).toBeVisible();
});
