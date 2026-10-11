import { expect, test, type Page } from "@playwright/test";

interface ScoringProbe {
  state: {
    isActive: boolean;
    isComplete: boolean;
    currentPhaseId: string | null;
    playerAccumulatedVP: Record<string, number>;
    highlightedTiles: Set<string>;
  };
  highlights: string[];
  phases: string[];
  show: (gameId: string | null, withScores?: boolean, suspend?: boolean) => void;
  release: () => void;
  replay: () => void;
  skip: () => void;
}

declare global {
  interface Window {
    scoringProbe: ScoringProbe;
  }
}

async function mountScoring(page: Page) {
  await page.goto("/create?quick");
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible();
  const now = new Date("2030-01-01T00:00:00Z");
  await page.clock.install({ time: now });
  await page.clock.pauseAt(now);
  await page.evaluate(async () => {
    const load = (path: string) => import(/* @vite-ignore */ path);
    // Match Vite's optimized React instance, including its dependency version.
    const source = await fetch("/src/hooks/useVPCountingAnimation.ts").then((r) => r.text());
    const reactUrl = source.match(/"([^"\n]*\/react\.js[^"\n]*)"/)![1];
    const version = new URL(reactUrl, location.href).search;
    const [React, ReactDOM, animation, context, bar] = await Promise.all(
      [
        reactUrl,
        `/node_modules/.vite/deps/react-dom_client.js${version}`,
        "/src/hooks/useVPCountingAnimation.ts",
        "/src/contexts/VPCountingContext.tsx",
        "/src/components/ui/endgame/EndGameBottomBar.tsx",
      ].map(load),
    );
    const R = React.default;
    const host = document.createElement("div");
    document.body.append(host);
    const root = ReactDOM.default.createRoot(host);
    const probe = { phases: [], highlights: [] } as unknown as ScoringProbe;
    window.scoringProbe = probe;
    const scores = [
      {
        playerId: "emil",
        playerName: "Emil",
        isWinner: true,
        placement: 1,
        vpBreakdown: {
          terraformRating: 24,
          greeneryVP: 1,
          greeneryVPDetails: [{ coordinate: "0,1,-1", vp: 1 }],
          cityVP: 1,
          cityVPDetails: [{ cityCoordinate: "0,0,0", adjacentGreeneries: ["0,1,-1"], vp: 1 }],
          cardVP: 2,
          cardVPDetails: [],
          milestoneVP: 5,
          awardVP: 5,
          totalVP: 38,
        },
      },
    ];
    let gameId: string | null = null;
    let withScores = false;
    let suspended: Promise<void> | null = null;
    function Harness() {
      const counting = animation.useVPCountingAnimation(gameId, withScores ? [...scores] : []);
      probe.replay = counting.start;
      probe.skip = counting.skip;
      R.useEffect(() => {
        probe.state = counting.state;
        probe.highlights = [...counting.state.highlightedTiles];
        if (counting.state.currentPhaseId) {
          probe.phases.push(counting.state.currentPhaseId);
        }
      }, [counting.state]);
      if (suspended) {
        throw suspended;
      }
      return R.createElement(
        context.VPCountingProvider,
        { state: counting.state, controls: counting },
        R.createElement(bar.default, {
          game: {
            id: gameId,
            finalScores: withScores ? scores : [],
            currentPlayer: { id: "emil", name: "Emil", color: "#ed3333" },
            otherPlayers: [],
          },
          playerId: "emil",
          activePanel: "score",
        }),
      );
    }
    const render = () =>
      root.render(
        R.createElement(
          R.StrictMode,
          null,
          R.createElement(R.Suspense, { fallback: "Loading results" }, R.createElement(Harness)),
        ),
      );
    probe.show = (id, available = true, suspend = false) => {
      gameId = id;
      withScores = available;
      if (suspend) {
        suspended = new Promise<void>((resolve) => {
          probe.release = () => {
            suspended = null;
            resolve();
          };
        });
      }
      render();
    };
    render();
  });
  await expect.poll(() => page.evaluate(() => window.scoringProbe.state?.isActive)).toBe(false);
}

test("interrupted results render still counts every category before completing", async ({
  page,
}) => {
  await mountScoring(page);
  await page.evaluate(() => window.scoringProbe.show("game-1", true, true));
  await page.clock.runFor(3000);
  // No animation may run behind an uncommitted results screen.
  expect(await page.evaluate(() => window.scoringProbe.phases)).toEqual([]);
  await page.evaluate(() => window.scoringProbe.release());
  await expect.poll(() => page.evaluate(() => window.scoringProbe.state.isActive)).toBe(true);
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP)).toEqual({
    emil: 0,
  });
  await expect(page.getByRole("button", { name: "Skip", exact: true })).toBeVisible();
  await page.clock.runFor(2100);
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil)).toBe(24);
  expect(await page.evaluate(() => window.scoringProbe.state.isComplete)).toBe(false);
  await page.clock.runFor(6500);
  expect(await page.evaluate(() => window.scoringProbe.highlights)).toEqual(["0,1,-1"]);
  await page.clock.runFor(900);
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil)).toBe(25);
  await page.clock.runFor(4000);
  expect(await page.evaluate(() => window.scoringProbe.highlights)).toEqual(["0,0,0"]);
  expect(await page.evaluate(() => window.scoringProbe.state.isComplete)).toBe(false);
  await page.clock.runFor(25000);
  expect(await page.evaluate(() => [...new Set(window.scoringProbe.phases)])).toEqual([
    "tr",
    "greeneries",
    "cities",
    "cards",
    "milestones",
    "awards",
  ]);
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil)).toBe(38);
  expect(await page.evaluate(() => window.scoringProbe.state.isComplete)).toBe(true);
  await expect(page.getByRole("button", { name: "Skip", exact: true })).toHaveCount(0);
});

test("late scores, repeated updates and replay keep only the current counting sequence", async ({
  page,
}) => {
  await mountScoring(page);
  await page.evaluate(() => window.scoringProbe.show("game-1", false));
  await page.clock.runFor(3000);
  expect(await page.evaluate(() => window.scoringProbe.state.isComplete)).toBe(false);
  await page.evaluate(() => window.scoringProbe.show("game-1"));
  await expect.poll(() => page.evaluate(() => window.scoringProbe.state.isActive)).toBe(true);
  await page.clock.runFor(2100);
  await page.evaluate(() => window.scoringProbe.show("game-1"));
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil)).toBe(24);
  await page.evaluate(() => window.scoringProbe.skip());
  await expect.poll(() => page.evaluate(() => window.scoringProbe.state.isComplete)).toBe(true);
  await page.evaluate(() => window.scoringProbe.replay());
  await page.clock.runFor(6500);
  // The old run would already be counting greeneries by now if replay revived it.
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil)).toBe(24);
  expect(await page.evaluate(() => window.scoringProbe.state.isComplete)).toBe(false);
  await page.evaluate(() => window.scoringProbe.show("game-2"));
  await expect
    .poll(() => page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil))
    .toBe(0);
  await page.clock.runFor(500);
  expect(await page.evaluate(() => window.scoringProbe.state.currentPhaseId)).toBeNull();
  await page.clock.runFor(1600);
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil)).toBe(24);
  await page.evaluate(() => window.scoringProbe.show(null));
  await page.clock.runFor(60000);
  expect(await page.evaluate(() => window.scoringProbe.state.isActive)).toBe(false);
  expect(await page.evaluate(() => window.scoringProbe.state.isComplete)).toBe(false);
  await page.evaluate(() => window.scoringProbe.show("game-2"));
  await expect.poll(() => page.evaluate(() => window.scoringProbe.state.isActive)).toBe(true);
  expect(await page.evaluate(() => window.scoringProbe.state.playerAccumulatedVP.emil)).toBe(0);
});
