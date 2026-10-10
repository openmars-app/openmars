import { expect, test, type Page } from "@playwright/test";

interface Box {
  label: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

interface Route {
  path: string;
  needsServer: boolean;
  ready: (page: Page) => Promise<void>;
}

const MIN_LONG_SIDE = 44;
const MIN_SHORT_SIDE = 32;
const MIN_INPUT_FONT_PX = 16;
const CHROME_SELECTOR = ".menu-chrome-top-left, .menu-chrome-top-right, .menu-footer";

const ROUTES: Route[] = [
  {
    path: "/",
    needsServer: true,
    ready: async (page) => {
      await expect(page.getByRole("link", { name: "New game" })).toBeVisible();
    },
  },
  {
    path: "/create",
    needsServer: true,
    ready: async (page) => {
      await expect(page.getByRole("heading", { name: "Create game" })).toBeVisible();
      await expect(page.getByText("Loading options…")).toHaveCount(0);
      const options = page.locator("summary", { hasText: "Game options" });
      if ((await options.count()) > 0) {
        await options.click();
      }
    },
  },
  {
    path: "/join",
    needsServer: true,
    ready: async (page) => {
      await expect(page.getByRole("heading", { name: "Browse games" })).toBeVisible();
      await expect(page.getByText("Loading games...")).toHaveCount(0);
    },
  },
  {
    path: "/cards",
    needsServer: false,
    ready: async (page) => {
      await expect(
        page.getByRole("complementary", { name: "Card browser controls" }),
      ).toBeAttached();
      await expect(page.getByRole("status")).toHaveCount(0);
    },
  },
];

async function serverReachable(page: Page): Promise<boolean> {
  try {
    const response = await page.request.get("/api/v1/health", { timeout: 3000 });
    return response.ok();
  } catch {
    return false;
  }
}

async function settle(page: Page) {
  await page.waitForFunction(() =>
    document
      .getAnimations()
      .filter((animation) => {
        const effect = animation.effect;
        if (effect?.getComputedTiming().iterations === Infinity) return false;
        // Cached card families use content-visibility:hidden. Firefox keeps
        // their suspended entrance animations in the running state.
        const target = effect instanceof KeyframeEffect ? effect.target : null;
        return !target || target.checkVisibility({ visibilityProperty: true });
      })
      .every((animation) => animation.playState !== "running"),
  );
}

function collectTargets(page: Page, chromeSelector: string) {
  return page.evaluate((chrome) => {
    const describe = (element: Element) => {
      const text = (element.getAttribute("aria-label") ?? element.textContent ?? "").trim();
      return `<${element.tagName.toLowerCase()}> "${text.slice(0, 40)}"`;
    };
    const isShown = (element: Element) => {
      const rect = element.getBoundingClientRect();
      return (
        rect.width > 0 &&
        rect.height > 0 &&
        element.checkVisibility({ opacityProperty: true, visibilityProperty: true }) &&
        element.closest("[inert], [aria-hidden='true']") === null
      );
    };
    const toBox = (element: Element) => {
      const rect = element.getBoundingClientRect();
      return {
        label: describe(element),
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height,
      };
    };
    const interactive = [...document.querySelectorAll("button, a[href], [role='button']")].filter(
      isShown,
    );
    const chromeTargets = interactive.filter((element) => element.closest(chrome) !== null);
    const contentTargets = interactive.filter((element) => element.closest(chrome) === null);
    const smallInputs = [
      ...document.querySelectorAll(
        "input:not([type='checkbox']):not([type='radio']):not([type='range']), textarea, select",
      ),
    ]
      .filter(isShown)
      .map((element) => ({
        label: describe(element) || element.getAttribute("placeholder") || "input",
        fontSize: parseFloat(getComputedStyle(element).fontSize),
      }));
    return {
      chrome: chromeTargets.map(toBox),
      content: contentTargets.map(toBox),
      inputs: smallInputs,
    };
  }, chromeSelector);
}

function intersects(a: Box, b: Box) {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

function scrollTo(page: Page, edge: "top" | "bottom") {
  return page.evaluate((target) => {
    for (const element of [document.documentElement, document.body]) {
      element.scrollTop = target === "top" ? 0 : element.scrollHeight;
    }
  }, edge);
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("openmars.installBar.dismissedUntil", String(Date.now() + 86_400_000));
  });
});

for (const route of ROUTES) {
  test(`menu ${route.path} fits the viewport`, async ({ page }, testInfo) => {
    const compactExpected = testInfo.project.use.isMobile === true;
    if (route.needsServer) {
      test.skip(!(await serverReachable(page)), "Go backend is not running on port 3001");
    }

    await page.goto(`${route.path}?quick`);
    await expect(page.getByRole("button", { name: "Menu" })).toBeVisible();
    await route.ready(page);
    await settle(page);

    await expect(page.locator("html")).toHaveAttribute(
      "data-layout",
      compactExpected ? "compact" : "desktop",
    );

    const isPortrait = await page.evaluate(() => window.innerHeight > window.innerWidth);
    if (compactExpected && isPortrait) {
      await expect(page.getByText("Rotate your device")).toBeVisible();
      return;
    }

    const overflow = await page.evaluate(() => ({
      scrollWidth: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth),
      innerWidth: window.innerWidth,
    }));
    expect(overflow.scrollWidth, "page scrolls horizontally").toBeLessThanOrEqual(
      overflow.innerWidth,
    );

    if (!compactExpected) {
      return;
    }

    const coveredAtEveryScroll: Set<string>[] = [];
    for (const edge of ["top", "bottom"] as const) {
      await scrollTo(page, edge);
      const { chrome, content, inputs } = await collectTargets(page, CHROME_SELECTOR);

      const tooSmall = [...chrome, ...content]
        .filter((box) => {
          const long = Math.max(box.width, box.height);
          const short = Math.min(box.width, box.height);
          return long < MIN_LONG_SIDE || short < MIN_SHORT_SIDE;
        })
        .map((box) => `${box.label} ${Math.round(box.width)}x${Math.round(box.height)}`);
      expect(tooSmall, `touch targets below 44x32 (scrolled to ${edge})`).toEqual([]);

      const smallText = inputs
        .filter((input) => input.fontSize < MIN_INPUT_FONT_PX)
        .map((input) => `${input.label} ${input.fontSize}px`);
      expect(smallText, "inputs below 16px").toEqual([]);

      coveredAtEveryScroll.push(
        new Set(
          chrome.flatMap((fixed) =>
            content
              .map((target, index) => ({ target, index }))
              .filter(({ target }) => intersects(fixed, target))
              .map(({ target, index }) => `${fixed.label} covers #${index} ${target.label}`),
          ),
        ),
      );
    }
    const [atTop, atBottom] = coveredAtEveryScroll;
    const unreachable = [...atTop].filter((overlap) => atBottom.has(overlap));
    expect(unreachable, "fixed chrome covers content at every scroll position").toEqual([]);
  });
}
