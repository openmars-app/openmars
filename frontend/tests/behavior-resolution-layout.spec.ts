import { expect, test } from "@playwright/test";

test("simple effect choices use a compact dialog while discard cards retain room", async ({
  page,
}, info) => {
  await page.goto("/create?quick");
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible();
  await page.evaluate(async () => {
    const [React, DOM, overlay] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/overlay/BehaviorResolutionOverlay.tsx",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const host = document.createElement("div");
    host.id = "resolution-layout-probe";
    document.body.append(host);
    const root = DOM.default.createRoot(host);
    const R = React.default;
    const base = {
      id: "olympus-trigger",
      kind: "choice",
      source: "Olympus Conference",
      sourceCardId: "185",
      sourceBehaviorIndex: 0,
      triggeringCardId: "138",
      triggeringCardName: "Quantum Extractor",
      minCards: 0,
      maxCards: 0,
      choices: [
        {
          originalIndex: 0,
          available: true,
          errors: [],
          inputs: [],
          outputs: [{ type: "science", amount: 1, target: "self-card" }],
          storageTargets: [],
        },
        {
          originalIndex: 1,
          available: true,
          errors: [],
          inputs: [{ type: "science", amount: 1, target: "self-card" }],
          outputs: [{ type: "card-draw", amount: 1, target: "self-player" }],
          storageTargets: [],
        },
      ],
    };
    const show = (cards: boolean) =>
      root.render(
        R.createElement(overlay.default, {
          resolutions: [
            cards
              ? {
                  ...base,
                  id: "discard",
                  kind: "card-discard",
                  minCards: 1,
                  maxCards: 1,
                  outputs: [],
                  choices: [],
                }
              : base,
          ],
          handCards: cards
            ? [
                {
                  id: "003",
                  name: "Deep Well Heating",
                  type: "automated",
                  cost: 13,
                  description: [],
                  gameModes: ["classic"],
                  behaviors: [],
                  tags: ["power", "building"],
                },
                {
                  id: "004",
                  name: "Cloud Seeding",
                  type: "event",
                  cost: 11,
                  description: [],
                  gameModes: ["classic"],
                  behaviors: [],
                  tags: [],
                },
                {
                  id: "005",
                  name: "Search For Life",
                  type: "active",
                  cost: 3,
                  description: [],
                  gameModes: ["classic"],
                  behaviors: [],
                  tags: ["science"],
                },
              ]
            : [],
          playedCards: [],
        }),
      );
    window.behaviorLayoutProbe = { show };
    show(false);
  });
  const panel = page.locator("#resolution-layout-probe .game-panel");
  await expect(panel.getByText("Triggered by Quantum Extractor")).toBeVisible();
  await expect(panel.getByRole("button", { name: "Confirm", exact: true })).toBeDisabled();
  const choices = panel.locator('button[data-emphasis="secondary"]');
  await expect(choices).toHaveCount(2);
  await expect(choices.nth(0)).toBeEnabled();
  await expect(choices.nth(1)).toBeEnabled();
  const size = await panel.boundingBox();
  const compact = info.project.name !== "firefox";
  if (compact) {
    expect(size!.width).toBeGreaterThan(page.viewportSize()!.width * 0.8);
    expect(size!.height).toBeGreaterThan(page.viewportSize()!.height * 0.8);
  } else {
    expect(size!.width).toBeLessThanOrEqual(540);
    expect(size!.width).toBeGreaterThan(400);
  }
  await choices.nth(1).click();
  await expect(choices.nth(1)).toContainText("✓");
  await expect(choices.nth(0)).not.toContainText("✓");
  const confirm = panel.getByRole("button", { name: "Confirm", exact: true });
  await expect(confirm).toBeEnabled();
  const confirmBounds = await confirm.boundingBox();
  expect(confirmBounds!.y + confirmBounds!.height).toBeLessThanOrEqual(page.viewportSize()!.height);
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("olympus-conference-choice.png"),
  });
  await page.evaluate(() => window.behaviorLayoutProbe.show(true));
  await expect(panel.getByRole("checkbox")).toHaveCount(3);
  if (!compact) {
    await expect.poll(async () => (await panel.boundingBox())!.width).toBe(920);
  }
  await page.screenshot({ animations: "disabled", path: info.outputPath("discard-card-grid.png") });
});

declare global {
  interface Window {
    behaviorLayoutProbe: { show: (cards: boolean) => void };
  }
}
