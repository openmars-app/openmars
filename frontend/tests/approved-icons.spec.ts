import { expect, test } from "@playwright/test";
import { readFileSync } from "node:fs";

const definitions = ["awards", "milestones", "colonies"].flatMap((kind) =>
  JSON.parse(readFileSync(new URL(`../../backend/assets/${kind}.json`, import.meta.url), "utf8")),
) as { id: string; style: { icon: string } }[];

test("approved icons reach production cards and destination badges", async ({ page }, info) => {
  test.setTimeout(180_000);
  const samples = [
    "097",
    "081",
    "220",
    "008",
    "017",
    "035",
    "044",
    "085",
    "086",
    "107",
    "123",
    "140",
    "231",
    "C30",
    "EXP001",
    "EXP004",
    "P37",
    "219",
    "P11",
    "C23",
    "C36",
    "C07",
    "C02",
    "172",
    "156",
    "244",
    "C48",
    "PC4",
    "V03",
  ];
  await page.goto(`/cards?quick&size=large&${samples.map((id) => `cId=${id}`).join("&")}`);
  for (const id of samples) {
    // The browser virtualizes rows; a focused catalog query exercises its real card presentation.
    await page.goto(`/cards?quick&size=large&cId=${id}`);
    const card = page.locator(`[data-card-id="${id}"]`);
    await expect(card).toBeVisible({ timeout: 30_000 });
    await expect(card.locator("[data-unsupported-icon]")).toHaveCount(0);
    if (id === "P11") {
      await expect(card.locator("[data-card-play-discount]")).toContainText("-25");
    }
    if (id === "081")
      await expect(
        card.locator('[data-qualified-icon="city-placement"] img[alt="jovian-tag"]'),
      ).toHaveCount(1);
    if (id === "008")
      await expect(
        card.locator('[data-qualified-icon="city-placement"] img[alt="ocean-tile"]'),
      ).toHaveCount(1);
    if (id === "017") {
      for (const size of ["small", "large"]) {
        await page.goto(`/cards?quick&size=${size}&cId=${id}`);
        const tile = card.locator('[data-qualified-icon="city-placement"] > img');
        await expect(tile).toBeVisible();
        expect(await tile.evaluate((el) => parseFloat(getComputedStyle(el).width))).toBeGreaterThan(
          30,
        );
        const tileBox = await tile.boundingBox();
        const productionBox = await card.locator("[data-production-box]").boundingBox();
        expect(tileBox && productionBox).toBeTruthy();
        expect(
          Math.abs(tileBox!.y + tileBox!.height / 2 - productionBox!.y - productionBox!.height / 2),
        ).toBeLessThan(2);
        await card.screenshot({
          path: info.outputPath(`noctis-${size}.png`),
          animations: "disabled",
          scale: "css",
        });
      }
    }
    if (id === "PC4") await expect(card.locator('img[src*="prelude-card"]')).toHaveCount(3);
    if (id === "V03") await expect(card.locator('img[alt="resources"]')).toHaveCount(2);
    if (id === "220")
      await expect(card.locator('[data-qualified-icon] img[alt="venus-tag"]')).toHaveCount(0);
    if (id === "C23") {
      await expect(card.locator('[data-colony-action="track"]')).toHaveCount(2);
      await expect(card.locator('[data-colony-action="track"]').first()).not.toContainText("1");
      await expect(card.locator('[data-colony-action="track"]').first()).not.toContainText("*");
      await expect(card.locator('[data-colony-action="track"]').last()).not.toContainText("*");
    }
    if (id === "C02") await expect(card.locator("[data-immediate-exchange]")).toHaveCount(1);
    if (id === "172") await expect(card.locator('img[alt="animal-protection"]')).toHaveCount(1);
    if (id === "156") {
      const trigger = card.locator("[data-standard-project-exception]");
      await expect(trigger).toHaveText("*");
      await expect(trigger.locator('img[alt="standard-project"]')).toHaveCount(1);
      await expect(trigger.locator("img")).toHaveCount(1);
    }
    if (id === "035")
      await expect(card.locator('img[alt="action"]')).toHaveAttribute("src", /behaviors\/action\./);
    if (id === "EXP001")
      await expect(card.locator('img[alt="extra-actions"]')).toHaveAttribute(
        "src",
        /behaviors\/actions\./,
      );
    if (id === "140") {
      await expect(card.locator('.behavior-section img[alt="volcano-placement"]')).toHaveCount(1);
      await expect(card.locator(".behavior-section")).not.toContainText("*");
    }
    if (id === "EXP004")
      await expect(card.locator('[data-tile-replacement] img[alt="nuclear"]')).toHaveCount(1);
    if (id === "231")
      await expect(
        card.locator('[data-qualified-icon="floater"] img[alt="venus-tag"]'),
      ).toHaveCount(1);
    if (id === "C30") await expect(card.locator("[data-colony-bonus-preview]")).toHaveCount(0);
    await expect
      .poll(() =>
        card
          .locator("img")
          .evaluateAll((images) =>
            images.every(
              (image) =>
                image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0,
            ),
          ),
      )
      .toBe(true);
    await card.screenshot({
      path: info.outputPath(`card-${id}.png`),
      animations: "disabled",
      scale: "css",
    });
    if (info.project.use.hasTouch) {
      await page.evaluate(async (cardId) => {
        const [React, DOM, detail] = await Promise.all(
          [
            "/node_modules/.vite/deps/react.js",
            "/node_modules/.vite/deps/react-dom_client.js",
            "/src/components/ui/overlay/CardDetailPortal.tsx",
          ].map((path) => import(/* @vite-ignore */ path)),
        );
        const { cards } = await (await fetch("/api/v1/cards?limit=1000")).json();
        const host = document.createElement("div");
        document.body.append(host);
        DOM.default.createRoot(host).render(
          React.default.createElement(detail.default, {
            card: cards.find((candidate: { id: string }) => candidate.id === cardId),
            onClose: () => {},
          }),
        );
      }, id);
      const detail = page.getByRole("dialog");
      await expect(detail.getByRole("button", { name: "Close card" })).toBeVisible();
      await expect(detail.locator(".game-card, .corporation-card").first()).toBeInViewport({
        ratio: 1,
      });
      await page.screenshot({
        path: info.outputPath(`detail-${id}.png`),
        animations: "disabled",
        scale: "css",
      });
    }
  }
});

test("compact tiles, badges and immediate gains stay legible", async ({ page }, info) => {
  test.setTimeout(120_000);
  for (const id of [
    "035",
    "044",
    "085",
    "086",
    "107",
    "123",
    "140",
    "231",
    "C30",
    "C02",
    "EXP004",
    "X31",
  ]) {
    await page.goto(`/cards?quick&size=small&cId=${id}`);
    const card = page.locator(`[data-card-id="${id}"]`);
    await expect(card).toBeVisible();
    if (["C02", "X31"].includes(id))
      await expect(card.locator('[data-immediate-exchange] [aria-label="then"]')).toHaveCount(0);
    if (id === "107") {
      const section = card.locator(".behavior-section");
      const icon = section.locator('img[alt="event"]');
      await expect(icon).toBeVisible();
      const bounds = await section.boundingBox();
      const iconBounds = await icon.boundingBox();
      expect(iconBounds!.y - bounds!.y).toBeGreaterThanOrEqual(4);
      expect(
        bounds!.y + bounds!.height - iconBounds!.y - iconBounds!.height,
      ).toBeGreaterThanOrEqual(4);
    }
    await card.screenshot({
      path: info.outputPath(`compact-${id}.png`),
      animations: "disabled",
      scale: "css",
    });
  }
});

test("two stored resources repeat the icon and optional symbols have no MAY label", async ({
  page,
}, info) => {
  await page.goto("/cards?quick&mode=earth-withdrawal&cId=ES018&cId=ES014&cId=ES017");
  const stored = page.locator('[data-card-id="ES018"] [data-stored-resources]');
  await expect(stored.locator('img[alt="card-resource"]')).toHaveCount(2);
  await expect(stored).toHaveText("");
  for (const id of ["ES014", "ES017"]) {
    await expect(page.locator(`[data-card-id="${id}"] [data-optional-choice]`)).not.toContainText(
      "MAY",
    );
    await expect(
      page.locator(
        `[data-card-id="${id}"] [data-compact-exchange] [data-state-symbol="withdrawal-occurred"]`,
      ),
    ).toHaveAttribute("data-state-value", "0");
    await expect(page.locator(`[data-card-id="${id}"] [data-compact-exchange]`)).not.toContainText(
      "*",
    );
  }
  await page.screenshot({
    path: info.outputPath("optional-and-storage-symbols.png"),
    animations: "disabled",
    scale: "css",
  });
});

test("production and discount each explain only their own effect", async ({ page }, info) => {
  for (const [id, production] of [
    ["079", "4"],
    ["094", "6"],
  ]) {
    await page.goto(`/cards?quick&cId=${id}`);
    const card = page.locator(`[data-card-id="${id}"]`);
    const sections = card.locator(".behavior-fitting");
    for (const [icon, text] of [
      ["energy-production", `Increase your energy production ${production} steps.`],
      ["credit", "Pay 2 M€ less for space cards."],
    ]) {
      const section = sections.filter({ has: page.locator(`img[alt="${icon}"]`) });
      if (info.project.use.hasTouch) {
        await section.tap();
      } else {
        await section.hover();
      }
      await expect(page.getByRole("tooltip")).toHaveText(text);
      await page.screenshot({
        path: info.outputPath(`${id}-${icon}-tooltip.png`),
        animations: "disabled",
        scale: "css",
      });
      if (info.project.use.hasTouch) {
        await section.tap();
      } else {
        await page.mouse.move(0, 0);
      }
    }
  }
});

test("discounted card chooser uses availability and the normal play callback", async ({
  page,
}, info) => {
  await page.goto("/create?quick");
  await page.evaluate(async () => {
    const [React, DOM, overlay] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/overlay/CardPlaySelectionOverlay.tsx",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const { cards } = await (await fetch("/api/v1/cards?limit=1000")).json();
    const host = document.createElement("div");
    host.id = "sponsor-choice-review";
    document.body.append(host);
    DOM.default.createRoot(host).render(
      React.default.createElement(overlay.default, {
        selection: {
          source: "Excentric Sponsor",
          sourceCardId: "P11",
          costReduction: 25,
          availableCards: ["001", "081"].map((id) => ({
            ...cards.find((candidate: { id: string }) => candidate.id === id),
            effectiveCost: 0,
            available: id === "081",
            errors:
              id === "001"
                ? [{ code: "temperature-too-low", category: "requirement", message: "Too cold" }]
                : [],
          })),
        },
        onPlay: (id: string) => (host.dataset.played = id),
      }),
    );
  });
  const dialog = page.getByRole("dialog", { name: "Play a card", exact: true });
  const play = dialog.getByRole("button", { name: "Play selected card" });
  await expect(play).toBeDisabled();
  await dialog.getByRole("checkbox", { name: "Colonizer Training Camp" }).click();
  await expect(play).toBeDisabled();
  await dialog.getByRole("checkbox", { name: "Ganymede Colony" }).click();
  await expect(play).toBeEnabled();
  await expect(play).toBeInViewport({ ratio: 1 });
  await page.screenshot({ path: info.outputPath("sponsor-choice.png"), animations: "disabled" });
  await play.click();
  await expect(page.locator("#sponsor-choice-review")).toHaveAttribute("data-played", "081");
});

test("all achievement and colony definitions resolve approved runtime artwork", async ({
  page,
}, info) => {
  await page.goto("/create?quick");
  const missing = await page.evaluate(async (defs) => {
    const path = "/src/utils/iconStore.ts";
    const { getIconPath } = await import(/* @vite-ignore */ path);
    return defs.filter((definition) => !getIconPath(definition.style.icon));
  }, definitions);
  expect(missing).toEqual([]);
  await page.evaluate(async (defs) => {
    const [React, DOM, icons] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/display/GameIcon.tsx",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const host = document.createElement("div");
    host.id = "approved-art-review";
    Object.assign(host.style, {
      position: "fixed",
      inset: "0",
      overflow: "auto",
      background: "#202e38",
      color: "white",
      padding: "20px",
      zIndex: "100000",
    });
    document.body.append(host);
    DOM.default.createRoot(host).render(
      React.default.createElement(
        "div",
        {
          style: {
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(130px, 1fr))",
            gap: "20px",
          },
        },
        defs.map((definition) =>
          React.default.createElement(
            "div",
            {
              key: definition.style.icon,
              style: { display: "flex", alignItems: "center", gap: "10px", fontSize: "12px" },
            },
            React.default.createElement(icons.default, {
              iconType: definition.style.icon,
              size: "large",
            }),
            definition.id,
          ),
        ),
      ),
    );
  }, definitions);
  await expect(page.locator("#approved-art-review img")).toHaveCount(43);
  await expect
    .poll(() =>
      page
        .locator("#approved-art-review img")
        .evaluateAll((images) =>
          images.every(
            (image) =>
              image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0,
          ),
        ),
    )
    .toBe(true);
  await page
    .locator("#approved-art-review > div")
    .screenshot({ path: info.outputPath("all-achievements-colonies.png") });
});
