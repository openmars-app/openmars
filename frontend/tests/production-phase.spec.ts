import { expect, test } from "@playwright/test";

interface ProductionProbe {
  submitted: boolean;
  productionReappeared: boolean;
  update: (selectionComplete: boolean) => void;
}

declare global {
  interface Window {
    productionProbe: ProductionProbe;
  }
}

for (const layout of ["desktop", "compact"] as const) {
  test(`production stays dismissed after buying cards in ${layout} layout`, async ({ page }) => {
    if (layout === "desktop") {
      await page.setViewportSize({ width: 1440, height: 900 });
    }
    await page.goto("/create?quick");
    await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible();
    await page.evaluate(async () => {
      const load = (path: string) => import(/* @vite-ignore */ path);
      const [React, ReactDOM, modal, socket] = await Promise.all(
        [
          "/node_modules/.vite/deps/react.js",
          "/node_modules/.vite/deps/react-dom_client.js",
          "/src/components/ui/modals/ProductionPhaseModal.tsx",
          "/src/services/globalWebSocketManager.ts",
        ].map(load),
      );
      const R = React.default;
      const host = document.createElement("div");
      document.body.append(host);
      const root = ReactDOM.default.createRoot(host);
      const production = { credits: 0, steel: 0, titanium: 0, plants: 0, energy: 0, heat: 0 };
      let game = {
        id: "production-test",
        generation: 2,
        currentPhase: "production_and_card_draw",
        settings: {},
        otherPlayers: [],
        currentPlayer: {
          id: "player",
          name: "You",
          terraformRating: 20,
          productionPhase: {
            selectionComplete: false,
            beforeResources: production,
            afterResources: { ...production, credits: 20 },
            productionUsed: production,
            creditsIncome: 20,
            energyConverted: 0,
            availableCards: [
              {
                id: "008",
                name: "Capital",
                type: "automated",
                cost: 26,
                gameModes: ["classic"],
                description: [],
                tags: [],
                behaviors: [],
              },
            ],
          },
        },
      };
      const probe = { submitted: false, productionReappeared: false } as ProductionProbe;
      window.productionProbe = probe;
      socket.globalWebSocketManager.confirmProductionCards = async () => {
        probe.submitted = true;
        return "request-sent";
      };
      new MutationObserver((records) => {
        for (const record of records) {
          for (const node of record.addedNodes) {
            if (!(node instanceof Element)) {
              continue;
            }
            const headings = [node, ...node.querySelectorAll("h2")];
            if (
              headings.some(
                (heading) => heading.matches("h2") && heading.textContent === "Production",
              )
            ) {
              probe.productionReappeared = true;
            }
          }
        }
      }).observe(host, { childList: true, subtree: true });
      const render = (isOpen = true) =>
        root.render(
          R.createElement(
            R.StrictMode,
            null,
            R.createElement(modal.default, {
              isOpen,
              gameState: game,
              onClose: () => render(false),
            }),
          ),
        );
      probe.update = (selectionComplete) => {
        game = {
          ...game,
          currentPlayer: {
            ...game.currentPlayer,
            productionPhase: { ...game.currentPlayer.productionPhase, selectionComplete },
          },
        };
        render();
      };
      render();
    });
    await expect(page.getByRole("heading", { name: "Production", exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Buy cards", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Select Cards to Buy" })).toBeVisible();
    await page.evaluate(() => {
      window.productionProbe.productionReappeared = false;
    });
    await page.getByRole("button", { name: "Skip Buy Cards", exact: true }).click();
    await page.getByRole("button", { name: "Confirm Skip", exact: true }).click();
    await expect.poll(() => page.evaluate(() => window.productionProbe.submitted)).toBe(true);
    await expect(page.getByRole("heading", { name: "Select Cards to Buy" })).not.toBeVisible();
    await expect(page.getByRole("heading", { name: "Production", exact: true })).not.toBeVisible();
    await page.evaluate(() => window.productionProbe.update(false));
    await page.evaluate(
      () =>
        new Promise<void>((resolve) =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
        ),
    );
    expect(await page.evaluate(() => window.productionProbe.productionReappeared)).toBe(false);
    await page.evaluate(() => window.productionProbe.update(true));
    await expect(page.getByRole("heading", { name: "Production", exact: true })).not.toBeVisible();
    await page.evaluate(() => window.productionProbe.update(false));
    await expect(page.getByRole("heading", { name: "Production", exact: true })).toBeVisible();
  });
}
