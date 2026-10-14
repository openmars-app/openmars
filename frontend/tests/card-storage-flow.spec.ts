import { expect, test } from "@playwright/test";

declare global {
  interface Window {
    storageFlowSubmissions: Array<{ cardId: string; targets: string[] }>;
  }
}

test("Ecology Research skips its animal separately from placing microbes on Ants", async ({
  page,
}, info) => {
  await page.goto("/create?quick");
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible({
    timeout: 20_000,
  });
  await page.evaluate(async () => {
    const load = (path: string) => import(/* @vite-ignore */ path);
    const [React, DOM, hook, popover, game, flow, socket] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/hooks/useCardPlayFlow.ts",
        "/src/components/ui/popover/CardStorageSelectionPopover.tsx",
        "/src/stores/gameStore.ts",
        "/src/stores/cardPlayFlowStore.ts",
        "/src/services/globalWebSocketManager.ts",
      ].map(load),
    );
    const { cards } = await (await fetch("/api/v1/cards?limit=1000")).json();
    const ecology = cards.find((card: { name: string }) => card.name === "Ecology Research");
    const ants = cards.find((card: { name: string }) => card.name === "Ants");
    const resources = { credits: 100, steel: 0, titanium: 0, plants: 0, energy: 0, heat: 0 };
    const currentPlayer = {
      id: "player",
      cards: [ecology],
      playedCards: [ants],
      resources,
      resourceStorage: {},
    };
    game.useGameStore.setState({
      game: {
        id: "storage-probe",
        currentTurn: "player",
        viewingPlayerId: "player",
        currentPlayer,
        otherPlayers: [],
      },
      currentPlayer,
    });
    flow.useCardPlayFlowStore.getState().resetAll();
    window.storageFlowSubmissions = [];
    socket.globalWebSocketManager.playCard = async (
      cardId: string,
      _payment: unknown,
      _choice: unknown,
      targets: string[],
    ) => {
      window.storageFlowSubmissions.push({ cardId, targets });
    };
    const host = document.createElement("div");
    document.body.append(host);
    const root = DOM.default.createRoot(host);
    function Probe() {
      const handlers = hook.useCardPlayFlow();
      const state = flow.useCardPlayFlowStore();
      const pending = state.pendingCardStorage;
      return React.default.createElement(
        React.default.Fragment,
        null,
        React.default.createElement(
          "button",
          {
            onClick: () => void handlers.handlePlayCard(ecology.id),
            style: { position: "fixed", left: 20, top: 20, zIndex: 10000, color: "white" },
          },
          "Play Ecology Research",
        ),
        pending &&
          React.default.createElement(popover.default, {
            resourceType: pending.resourceType,
            amount: pending.amount,
            playedCards: [ants, ecology],
            resourceStorage: {},
            onCardSelect: handlers.handleCardStorageSelect,
            onCancel: handlers.handleCardStorageCancel,
            isVisible: state.showCardStorageSelection,
          }),
      );
    }
    root.render(React.default.createElement(Probe));
  });
  await page.getByRole("button", { name: "Play Ecology Research", exact: true }).click();
  await expect(page.getByText("No animal storage available", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Continuing skips this gain of 1 animal.", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Ants", { exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => window.storageFlowSubmissions)).toEqual([]);
  await page.screenshot({ path: info.outputPath("animal-warning.png") });
  await page.getByRole("button", { name: "Continue without animal", exact: true }).click();
  await expect(page.getByText("Place 2 microbes", { exact: true })).toBeVisible();
  await expect(page.getByText("No animal storage available", { exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => window.storageFlowSubmissions)).toEqual([]);
  await page.screenshot({ path: info.outputPath("microbe-selection.png") });
  await page.locator(".game-choice").filter({ hasText: "Ants" }).click();
  await expect
    .poll(() => page.evaluate(() => window.storageFlowSubmissions))
    .toEqual([{ cardId: "C09", targets: ["", "035"] }]);
  await expect(page.getByText("Place 2 microbes", { exact: true })).toHaveCount(0);
});
