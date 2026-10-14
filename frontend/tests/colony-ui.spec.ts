import { expect, test, type Page } from "@playwright/test";
interface ColonyUiProbe {
  submissions: number[];
  trades: { colonyId: string; payment: string; trackSteps: number }[];
  finish: (error?: string) => void;
}
declare global {
  interface Window {
    colonyUiProbe: ColonyUiProbe;
  }
}
async function mountColonyUi(page: Page, scenario: "tracks" | "generic" | "list") {
  await page.goto("/create?quick");
  await expect(page.locator("html")).toHaveAttribute("data-layout", /desktop|compact/);
  await page.evaluate(async (fixture) => {
    const [React, ReactDOM, overlay, list, socket] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/overlay/EffectSelectionOverlay.tsx",
        "/src/components/ui/popover/content/ColonyList.tsx",
        "/src/services/webSocketService.ts",
      ].map((path) => import(/* @vite-ignore */ path)),
    );
    const R = React.default;
    const host = document.createElement("div");
    host.id = "colony-ui-probe";
    Object.assign(host.style, {
      position: "fixed",
      inset: "0",
      zIndex: "100000",
      overflow: "auto",
      padding: fixture === "list" ? "16px" : "0",
      background: "#110e0b",
      color: "white",
    });
    document.body.append(host);
    const root = ReactDOM.default.createRoot(host);
    const probe: ColonyUiProbe = { submissions: [], trades: [], finish: () => {} };
    window.colonyUiProbe = probe;
    const colonies = ["Miranda", "Europa", "Pluto", "Callisto", "Ceres"].map((name, index) => ({
      id: name.toLowerCase(),
      name,
      active: true,
      markerPosition: 3,
      location: "solar-system",
      steps: Array.from({ length: 7 }, (_, step) => ({
        outputs: [{ type: index === 0 ? "animal" : "credit", amount: step }],
      })),
      colonyBonus: [{ type: index === 0 ? "animal" : "credit", amount: 1 }],
      colonies: Array.from({ length: 3 }, () => ({ reward: [{ type: "credit", amount: 2 }] })),
      playerColonies: [],
      tradedThisGen: false,
      traderId: "",
      style: { color: index === 0 ? "#c473a3" : "#67b0d1" },
      tradeOptions: [
        { trackSteps: 0, markerPosition: 3, outputs: [{ type: "credit", amount: 3 }] },
      ],
      tradeAvailable: true,
      buildAvailable: true,
      tradeErrors: [],
      buildErrors: [],
    }));
    if (fixture === "list") {
      const game = {
        id: "colony-test",
        status: "active",
        currentPhase: "action",
        currentTurn: "player",
        viewingPlayerId: "player",
        otherPlayers: [],
        tradeFleets: { player: { available: 1, total: 1 } },
        currentPlayer: {
          id: "player",
          name: "You",
          color: "#e44",
          availableActions: 2,
          playedCards: [],
          actionCosts: [
            {
              actionType: "colony-trade",
              costs: [
                { resource: "credit", effectiveCost: 9, paymentCapacity: 8 },
                { resource: "energy", effectiveCost: 3, paymentCapacity: 3 },
                { resource: "titanium", effectiveCost: 3, paymentCapacity: 2 },
              ],
            },
          ],
        },
        colonies: [
          {
            ...colonies[0],
            active: false,
            markerPosition: -1,
            tradeAvailable: false,
            tradeOptions: [],
          },
          colonies[1],
        ],
      };
      socket.webSocketService.tradeWithColony = async (
        colonyId: string,
        payment: string,
        trackSteps: number,
      ) => {
        probe.trades.push({ colonyId, payment, trackSteps });
      };
      function ColonyFixture() {
        const controls = list.useColonyControls(game);
        return R.createElement(
          "div",
          { style: { maxWidth: "600px", margin: "0 auto" } },
          R.createElement(list.default, { gameState: game, controls, density: "screen" }),
        );
      }
      root.render(R.createElement(ColonyFixture));
      return;
    }
    const selection =
      fixture === "tracks"
        ? {
            source: "Market Manipulation",
            sourceCardId: "C24",
            outputs: [
              { type: "colony-track-step", amount: 1, target: "colony" },
              { type: "colony-track-step", amount: -1, target: "colony" },
            ],
            // Server order is intentionally non-cartesian and non-alphabetical.
            options: [
              { colonyIds: ["pluto", "europa"] },
              { colonyIds: ["europa", "miranda"] },
              { colonyIds: ["miranda", "pluto"] },
              { colonyIds: ["callisto", "ceres"] },
              { colonyIds: ["ceres", "callisto"] },
              { colonyIds: ["pluto", "miranda"] },
            ],
          }
        : {
            source: "Choose a reward",
            sourceCardId: "test",
            outputs: [{ type: "credit", amount: 2, target: "self-player" }],
            options: [
              {
                targetPlayerId: "alice",
                outputs: [{ type: "credit", amount: 2, target: "self-player" }],
              },
              {
                targetPlayerId: "bob",
                outputs: [{ type: "credit", amount: 3, target: "self-player" }],
              },
            ],
          };
    root.render(
      R.createElement(overlay.default, {
        selection,
        colonies,
        cards: [],
        players: [
          { id: "alice", name: "Alice" },
          { id: "bob", name: "Bob" },
        ],
        onConfirm: (index: number) =>
          new Promise<void>((resolve, reject) => {
            probe.submissions.push(index);
            probe.finish = (error?: string) => (error ? reject(new Error(error)) : resolve());
          }),
      }),
    );
  }, scenario);
}

test("colony changes preserve server pairs and retry failed submissions", async ({
  page,
}, info) => {
  await mountColonyUi(page, "tracks");
  const dialog = page.getByRole("dialog", { name: "Market Manipulation" });
  const decrease = dialog.getByRole("region", { name: "Decrease colony track" });
  const increase = dialog.getByRole("region", { name: "Increase colony track" });
  const confirm = dialog.getByRole("button", { name: "Confirm", exact: true });
  await expect(decrease).toBeVisible();
  await expect(increase).toBeVisible();
  const left = await decrease.boundingBox();
  const right = await increase.boundingBox();
  expect(left!.x + left!.width).toBeLessThanOrEqual(right!.x);
  await expect(confirm).toBeDisabled();
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("colony-selection-initial.png"),
  });
  await decrease.getByRole("button", { name: "Miranda", exact: true }).click();
  await expect(confirm).toBeDisabled();
  await expect(increase.getByRole("button", { name: "Miranda", exact: true })).toBeDisabled();
  await expect(increase.getByRole("button", { name: "Callisto", exact: true })).toBeDisabled();
  await increase.getByRole("button", { name: "Pluto", exact: true }).click();
  await expect(confirm).toBeEnabled();
  await expect(increase.getByRole("button", { name: "Pluto", exact: true })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await confirm.click();
  await expect.poll(() => page.evaluate(() => window.colonyUiProbe.submissions)).toEqual([5]);
  await expect(confirm).toBeDisabled();
  await expect(decrease.getByRole("button", { name: "Miranda", exact: true })).toBeDisabled();
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("colony-selection-pending.png"),
  });
  await page.evaluate(() => window.colonyUiProbe.finish("The colony state changed. Try again."));
  await expect(dialog.getByRole("alert")).toHaveText("The colony state changed. Try again.");
  await expect(confirm).toBeEnabled();
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("colony-selection-error.png"),
  });
  await increase.getByRole("button", { name: "Europa", exact: true }).click();
  await confirm.click();
  await expect.poll(() => page.evaluate(() => window.colonyUiProbe.submissions)).toEqual([5, 1]);
  await expect(dialog.getByRole("alert")).toHaveCount(0);
  await page.evaluate(() => window.colonyUiProbe.finish());
  await expect(confirm).toBeEnabled();
  const bounds = await confirm.boundingBox();
  expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(page.viewportSize()!.height);
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("colony-selection-final.png"),
  });
  // A completed pair must not trap the player in one disconnected set of valid options.
  await expect(decrease.getByRole("button", { name: "Ceres", exact: true })).toBeDisabled();
  await decrease.getByRole("button", { name: "Miranda", exact: true }).click();
  await expect(decrease.getByRole("button", { name: "Miranda", exact: true })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  await expect(confirm).toBeDisabled();
  await increase.getByRole("button", { name: "Europa", exact: true }).click();
  await expect(increase.getByRole("button", { name: "Europa", exact: true })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  await decrease.getByRole("button", { name: "Ceres", exact: true }).click();
  await expect(confirm).toBeDisabled();
  await increase.getByRole("button", { name: "Callisto", exact: true }).click();
  await expect(confirm).toBeEnabled();
  await confirm.click();
  await expect.poll(() => page.evaluate(() => window.colonyUiProbe.submissions)).toEqual([5, 1, 3]);
  await page.evaluate(() => window.colonyUiProbe.finish());
  await expect(confirm).toBeEnabled();
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("colony-selection-changed-pair.png"),
  });
});

test("non-colony effect selections retain the generic choices", async ({ page }) => {
  await mountColonyUi(page, "generic");
  const dialog = page.getByRole("dialog", { name: "Choose a reward" });
  await expect(dialog.getByRole("region", { name: "Decrease colony track" })).toHaveCount(0);
  const confirm = dialog.getByRole("button", { name: "Confirm", exact: true });
  await expect(confirm).toBeDisabled();
  await dialog.getByRole("button", { name: /Bob/ }).click();
  await confirm.click();
  await expect.poll(() => page.evaluate(() => window.colonyUiProbe.submissions)).toEqual([1]);
  await page.evaluate(() => window.colonyUiProbe.finish());
});

test("colony trading uses selected payment affordability and hides inactive markers", async ({
  page,
}, info) => {
  await mountColonyUi(page, "list");
  const host = page.locator("#colony-ui-probe");
  const miranda = host
    .locator(".popover-list-item")
    .filter({ has: page.getByRole("heading", { name: "Miranda", exact: true }) });
  const europa = host
    .locator(".popover-list-item")
    .filter({ has: page.getByRole("heading", { name: "Europa", exact: true }) });
  await expect(miranda.getByText("Inactive")).toBeVisible();
  await expect(miranda.locator("[data-colony-marker]")).toHaveCount(0);
  await expect(europa.locator("[data-colony-marker]")).toHaveCount(1);
  const trade = europa.getByRole("button", { name: "Trade", exact: true });
  await expect(trade).toHaveAttribute("data-emphasis", "primary");
  await expect(trade).toHaveAttribute("data-shape", "cut");
  await expect(miranda.getByRole("button", { name: "Trade", exact: true })).toBeDisabled();
  await expect(host.getByRole("button", { name: "Pay with energy" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(trade).toBeEnabled();
  await host.getByRole("button", { name: "Pay with credits" }).click();
  await expect(trade).toBeDisabled();
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("colony-trade-unaffordable.png"),
    fullPage: true,
  });
  await host.getByRole("button", { name: "Pay with titanium" }).click();
  await expect(trade).toBeDisabled();
  await host.getByRole("button", { name: "Pay with energy" }).click();
  await expect(trade).toBeEnabled();
  await trade.click();
  await expect
    .poll(() => page.evaluate(() => window.colonyUiProbe.trades))
    .toEqual([{ colonyId: "europa", payment: "energy", trackSteps: 0 }]);
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("colony-trade-affordable.png"),
    fullPage: true,
  });
});
