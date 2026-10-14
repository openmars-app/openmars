import { expect, test } from "@playwright/test";

interface CardPlayChoiceProbe {
  play: (kind: string) => Promise<{
    submitted: number;
    choiceIndex?: number;
    choice: boolean;
    storage: boolean;
    amount: boolean;
    target: boolean;
  }>;
}

declare global {
  interface Window {
    cardPlayChoiceProbe?: CardPlayChoiceProbe;
  }
}

test("card play prompts only for immediate behaviors, not triggered or manual effects", async ({
  page,
}) => {
  await page.goto("/create?quick");
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible();
  await page.evaluate(async () => {
    const load = (path: string) => import(/* @vite-ignore */ path);
    const [React, DOM, hook, game, flow, socket] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/hooks/useCardPlayFlow.ts",
        "/src/stores/gameStore.ts",
        "/src/stores/cardPlayFlowStore.ts",
        "/src/services/globalWebSocketManager.ts",
      ].map(load),
    );
    const { cards } = await (await fetch("/api/v1/cards?limit=1000")).json();
    const olympus = cards.find((card: { name: string }) => card.name === "Olympus Conference");
    const host = document.createElement("div");
    document.body.append(host);
    const root = DOM.default.createRoot(host);
    function Probe() {
      const handlers = hook.useCardPlayFlow();
      window.cardPlayChoiceProbe = {
        play: async (kind) => {
          flow.useCardPlayFlowStore.getState().resetAll();
          const card = structuredClone(olympus);
          if (kind !== "olympus") {
            const trigger = kind === "manual" ? { type: "manual" } : { type: "auto" };
            if (kind.startsWith("passive")) {
              Object.assign(trigger, { condition: { type: "tag-played" } });
            }
            card.behaviors = [
              {
                triggers: [trigger],
                optional: kind === "optional",
                choices: [
                  {
                    originalIndex: 0,
                    outputs: [{ type: "science", amount: 1, target: "self-card" }],
                  },
                  {
                    originalIndex: 1,
                    outputs: [{ type: "card-draw", amount: 1, target: "self-player" }],
                  },
                ],
              },
            ];
            if (kind === "passive-inputs") {
              card.behaviors = [
                {
                  triggers: [trigger],
                  inputs: [
                    { type: "science", amount: 1, target: "any-card", variableAmount: true },
                  ],
                  outputs: [
                    { type: "microbe", amount: 1, target: "any-card" },
                    { type: "plant", amount: -1, target: "any-player" },
                  ],
                  inputOptions: {
                    storageSources: [["source"]],
                    variableAmount: { resourceType: "science", min: 0, max: 2 },
                  },
                },
              ];
            }
          }
          const resources = { credits: 100, steel: 0, titanium: 0, plants: 10, energy: 0, heat: 0 };
          game.useGameStore.setState({
            game: {
              id: "choice-probe",
              currentTurn: "player",
              viewingPlayerId: "player",
              otherPlayers: [{ id: "opponent", resources }],
            },
            currentPlayer: { id: "player", cards: [card], resources, resourceStorage: {} },
          });
          const submissions: Array<{ choiceIndex?: number }> = [];
          socket.globalWebSocketManager.playCard = async (
            _id: string,
            _payment: unknown,
            choiceIndex?: number,
          ) => {
            submissions.push({ choiceIndex });
          };
          await handlers.handlePlayCard(card.id);
          const state = flow.useCardPlayFlowStore.getState();
          return {
            submitted: submissions.length,
            choiceIndex: submissions[0]?.choiceIndex,
            choice: state.showChoiceSelection,
            storage: state.showCardStorageSelection || state.showCardResourceSelection,
            amount: state.showAmountSelection,
            target: state.showTargetPlayerSelection,
          };
        },
      };
      return null;
    }
    root.render(React.default.createElement(Probe));
  });
  await expect.poll(() => page.evaluate(() => Boolean(window.cardPlayChoiceProbe))).toBeTruthy();
  for (const kind of ["olympus", "passive", "passive-inputs", "manual", "optional"]) {
    const result = await page.evaluate((value) => window.cardPlayChoiceProbe!.play(value), kind);
    expect(result, kind).toEqual({
      submitted: 1,
      choiceIndex: undefined,
      choice: false,
      storage: false,
      amount: false,
      target: false,
    });
  }
  const immediate = await page.evaluate(() => window.cardPlayChoiceProbe!.play("immediate"));
  expect(immediate).toEqual({
    submitted: 0,
    choiceIndex: undefined,
    choice: true,
    storage: false,
    amount: false,
    target: false,
  });
});
