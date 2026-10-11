import { expect, test, type Page } from "@playwright/test";

interface SoundProbe {
  played: HTMLMediaElement[];
  stopped: HTMLMediaElement[];
  warning: () => void;
  lost: () => void;
  ready: () => void;
  closeProduction: () => void;
  turn: (id: string) => void;
  award: () => void;
  dismiss: () => void;
  rerender: () => void;
  reconnect: (connected: boolean) => void;
  changeGame: () => void;
  volume: (value: number) => void;
  enabled: (value: boolean) => void;
  url: (soundId: string) => string;
}

declare global {
  interface Window {
    announcementSoundProbe: SoundProbe;
  }
}

async function mountAnnouncements(page: Page, playerId = "p1") {
  await page.goto("/create?quick");
  await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible();
  const now = new Date("2030-01-01T00:00:00Z");
  await page.clock.install({ time: now });
  await page.clock.pauseAt(now);
  await page.evaluate(async (viewer) => {
    const load = (path: string) => import(/* @vite-ignore */ path);
    const [React, ReactDOM, banner, queue, overlays, audio, assets] = await Promise.all(
      [
        "/node_modules/.vite/deps/react.js",
        "/node_modules/.vite/deps/react-dom_client.js",
        "/src/components/ui/overlay/GameEventBanner.tsx",
        "/src/hooks/useGameEvent.ts",
        "/src/stores/uiOverlayStore.ts",
        "/src/services/audioService.ts",
        "/src/assets/index.ts",
      ].map(load),
    );
    const R = React.default;
    const host = document.createElement("div");
    document.body.append(host);
    const root = ReactDOM.default.createRoot(host);
    const probe = { played: [], stopped: [] } as unknown as SoundProbe;
    window.announcementSoundProbe = probe;
    HTMLMediaElement.prototype.play = function () {
      probe.played.push(this);
      return Promise.resolve();
    };
    HTMLMediaElement.prototype.pause = function () {
      probe.stopped.push(this);
    };
    const service = audio.audioService;
    service.setEnabled(true);
    service.setVolume(0.5);
    probe.volume = (value) => service.setVolume(value);
    probe.enabled = (value) => service.setEnabled(value);
    probe.url = (id) => assets.assetUrl(`audio/effects/${id}`);
    let connected = true;
    let game = {
      id: "sound-test",
      generation: 6,
      currentPlayer: { id: viewer },
      currentTurn: "p1",
      currentPhase: "production_and_card_draw",
      awards: [{ type: "banker", name: "Banker", isFunded: false, fundedBy: "p1" }],
      modeRuntime: {
        events: [] as Array<{
          id: string;
          kind: string;
          state: string;
          title: string;
          soundId: string;
        }>,
      },
    };
    function Harness() {
      const { currentEvent, dismissCurrent } = queue.useGameEvent(game, connected);
      probe.dismiss = dismissCurrent;
      return R.createElement(banner.default, { event: currentEvent, onDismiss: dismissCurrent });
    }
    const render = () => root.render(R.createElement(R.StrictMode, null, R.createElement(Harness)));
    probe.rerender = render;
    const modeEvent = (id: string, title: string, kind: string, state: string, soundId: string) => {
      game = {
        ...game,
        modeRuntime: { events: [...game.modeRuntime.events, { id, title, kind, state, soundId }] },
      };
      render();
    };
    probe.warning = () => {
      overlays.useUIOverlayStore.setState({ showProductionPhaseModal: true });
      modeEvent(
        "warning",
        "Last generation of Earth support",
        "deadline-warning",
        "withdrawal-generation",
        "earth-withdrawal-warning",
      );
    };
    probe.lost = () =>
      modeEvent(
        "lost",
        "Earth contact closed",
        "state-change",
        "earth-contact",
        "earth-contact-lost",
      );
    probe.ready = () => {
      game = { ...game, currentPhase: "action" };
      render();
    };
    probe.turn = (id) => {
      game = { ...game, currentTurn: id };
      render();
    };
    probe.award = () => {
      game = { ...game, awards: [{ ...game.awards[0], isFunded: true }] };
      render();
    };
    probe.changeGame = () => {
      game = { ...game, id: "different-game" };
      render();
    };
    probe.closeProduction = () =>
      overlays.useUIOverlayStore.setState({ showProductionPhaseModal: false });
    probe.reconnect = (value) => {
      connected = value;
      render();
    };
    render();
  }, playerId);
  await page.clock.runFor(1);
}

async function playbackCount(page: Page, count: number) {
  await expect
    .poll(async () => {
      await page.clock.runFor(10);
      return page.evaluate(() => window.announcementSoundProbe.played.length);
    })
    .toBe(count);
}

for (const viewer of ["p1", "p2"]) {
  test(`generation announcement precedes turns for ${viewer}`, async ({ page }) => {
    await mountAnnouncements(page, viewer);
    await page.evaluate(() => window.announcementSoundProbe.warning());
    await page.clock.runFor(2000);
    await expect(page.getByRole("status")).toHaveCount(0);
    // One player has closed their modal, but the server is still waiting for everyone.
    await page.evaluate(() => window.announcementSoundProbe.closeProduction());
    await page.clock.runFor(2000);
    await expect(page.getByRole("status")).toHaveCount(0);
    await page.evaluate(() => window.announcementSoundProbe.ready());
    await page.clock.runFor(999);
    await expect(page.getByRole("status")).toHaveCount(0);
    await page.clock.runFor(2);
    await expect(page.getByRole("status")).toHaveText("Last generation of Earth support");
    await playbackCount(page, 1);
    expect(await page.evaluate(() => window.announcementSoundProbe.played[0].volume)).toBeCloseTo(
      0.35,
    );
    await page.evaluate(() => {
      const p = window.announcementSoundProbe;
      p.rerender();
      p.volume(0.2);
      p.enabled(false);
    });
    expect(await page.evaluate(() => window.announcementSoundProbe.played[0].volume)).toBeCloseTo(
      0.14,
    );
    expect(await page.evaluate(() => window.announcementSoundProbe.played[0].muted)).toBe(true);
    await page.evaluate(() => {
      window.announcementSoundProbe.enabled(true);
      window.announcementSoundProbe.dismiss();
    });
    await page.clock.runFor(20);
    if (viewer === "p1") {
      await expect(page.getByRole("status")).toHaveText("YOUR TURN");
      await playbackCount(page, 2);
    } else {
      await expect(page.getByRole("status")).toHaveCount(0);
      expect(await page.evaluate(() => window.announcementSoundProbe.played.length)).toBe(1);
    }
    await page.evaluate(() => {
      window.announcementSoundProbe.dismiss();
      window.announcementSoundProbe.reconnect(false);
    });
    await page.clock.runFor(1);
    await page.evaluate(() => window.announcementSoundProbe.reconnect(true));
    await page.clock.runFor(11000);
    await expect(page.getByRole("status")).toHaveCount(0);
  });
}

test("local production gates banners, stale turns are dropped and game changes clear queues", async ({
  page,
}) => {
  await mountAnnouncements(page);
  await page.evaluate(() => window.announcementSoundProbe.warning());
  await page.clock.runFor(1);
  await page.evaluate(() => window.announcementSoundProbe.ready());
  await page.clock.runFor(2000);
  await expect(page.getByRole("status")).toHaveCount(0);
  await page.evaluate(() => window.announcementSoundProbe.closeProduction());
  await page.clock.runFor(1010);
  await expect(page.getByRole("status")).toHaveText("Last generation of Earth support");
  await playbackCount(page, 1);
  await page.evaluate(() => {
    const p = window.announcementSoundProbe;
    p.turn("p2");
    p.lost();
  });
  await page.clock.runFor(1);
  await page.evaluate(() => window.announcementSoundProbe.dismiss());
  await page.clock.runFor(1010);
  await expect(page.getByRole("status")).toHaveText("Earth contact closed");
  await playbackCount(page, 2);
  expect(
    await page.evaluate(() => {
      const p = window.announcementSoundProbe;
      return (
        p.stopped.includes(p.played[0]) && p.played[1].src.endsWith(p.url("earth-contact-lost"))
      );
    }),
  ).toBe(true);
  await page.evaluate(() => window.announcementSoundProbe.award());
  await page.clock.runFor(1);
  await page.evaluate(() => window.announcementSoundProbe.dismiss());
  await page.clock.runFor(20);
  await expect(page.getByRole("status")).toContainText("AWARD FUNDED");
  await playbackCount(page, 3);
  await page.evaluate(() => window.announcementSoundProbe.turn("p1"));
  await page.clock.runFor(1);
  await page.evaluate(() => window.announcementSoundProbe.changeGame());
  await page.clock.runFor(11000);
  await expect(page.getByRole("status")).toHaveCount(0);
  expect(await page.evaluate(() => window.announcementSoundProbe.played.length)).toBe(3);
});
