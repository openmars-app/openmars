import { devices, expect, test } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";

test.skip(!process.env.E2E_BACKEND, "Requires an explicitly selected Go verification server");
test.beforeEach(() => test.skip(!test.info().project.name.startsWith("engine")));

for (const layout of ["desktop", "iphone", "android"] as const) {
  test(`lobby separates map and settings for host and guest on ${layout}`, async ({
    browser,
    baseURL,
  }) => {
    test.setTimeout(180_000);
    const profile =
      layout === "desktop"
        ? { viewport: { width: 1280, height: 720 } }
        : {
            ...devices[layout === "iphone" ? "iPhone 14 landscape" : "Pixel 7 landscape"],
            deviceScaleFactor: 1,
          };
    const context = await browser.newContext({ ...profile, baseURL, reducedMotion: "reduce" });
    const guestContext = await browser.newContext({ ...profile, baseURL, reducedMotion: "reduce" });
    const backend = test.info().project.name === "engine-webgl2" ? "webgl2" : "webgpu";
    for (const target of [context, guestContext]) {
      await target.addInitScript((renderer) => {
        const url = new URL(location.href);
        url.searchParams.set("renderer", renderer);
        history.replaceState(history.state, "", url);
      }, backend);
    }
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const output = path.resolve(import.meta.dirname, "../../tmp/settings-selectors");
    try {
      await mkdir(output, { recursive: true });
      await page.goto("/create");
      await page.getByLabel("Your name", { exact: true }).fill(`Lobby ${layout}`);
      await page.getByRole("button", { name: "Create lobby", exact: true }).click();
      if (layout === "desktop") {
        await expect(page.getByRole("heading", { name: "Game lobby", exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: "Map", exact: true })).toBeVisible();
        await expect(page.getByRole("button", { name: "Game mode", exact: true })).toBeVisible();
        await expect(page.getByRole("tablist", { name: "Lobby", exact: true })).toHaveCount(0);
        await page.screenshot({ path: path.join(output, "lobby-desktop.png") });
        return;
      }
      const lobby = page.getByTestId("mobile-lobby");
      const tabs = lobby.getByRole("tablist", { name: "Lobby", exact: true });
      await expect(tabs.getByRole("tab")).toHaveText(["Players", "Chat", "Map", "Settings"]);
      for (const tab of await tabs.getByRole("tab").all()) {
        await expect(tab).toBeInViewport();
        await expect
          .poll(async () => (await tab.boundingBox())?.height ?? 0)
          .toBeGreaterThanOrEqual(44);
      }
      await tabs.getByRole("tab", { name: "Map", exact: true }).click();
      await expect(lobby.getByRole("button", { name: "Map", exact: true })).toBeVisible();
      await expect(lobby.getByRole("img", { name: "Map preview", exact: true })).toBeVisible();
      await expect(lobby.getByRole("button", { name: "Game mode", exact: true })).toHaveCount(0);
      await page.screenshot({ path: path.join(output, `lobby-${layout}-map.png`) });
      await tabs.getByRole("tab", { name: "Settings", exact: true }).click();
      await expect(lobby.getByRole("button", { name: "Game mode", exact: true })).toBeVisible();
      await expect(lobby.getByRole("button", { name: "Map", exact: true })).toHaveCount(0);
      await expect(lobby.getByRole("img", { name: "Map preview", exact: true })).toHaveCount(0);
      await expect(lobby.getByRole("group", { name: "Options", exact: true })).toBeVisible();
      await page.screenshot({ path: path.join(output, `lobby-${layout}-settings.png`) });
      const gameId = await page.evaluate(async () => {
        const modulePath = "/src/stores/gameStore.ts";
        const { useGameStore } = await import(/* @vite-ignore */ modulePath);
        return useGameStore.getState().game!.id;
      });
      const guest = await guestContext.newPage();
      guest.on("pageerror", (error) => errors.push(error.message));
      await guest.goto(`/join?code=${gameId}`);
      await guest.getByPlaceholder("Enter your name", { exact: true }).fill(`Guest ${layout}`);
      await guest.getByPlaceholder("Enter your name", { exact: true }).press("Enter");
      const guestLobby = guest.getByTestId("mobile-lobby");
      await expect(guestLobby).toBeVisible();
      await guestLobby.getByRole("tab", { name: "Settings", exact: true }).click();
      await expect(guestLobby.getByText("Game mode", { exact: true })).toBeVisible();
      await expect(guestLobby.getByRole("button", { name: "Game mode", exact: true })).toHaveCount(
        0,
      );
      for (const checkbox of await guestLobby.getByRole("checkbox").all()) {
        await expect(checkbox).toBeDisabled();
      }
      await guest.screenshot({ path: path.join(output, `lobby-${layout}-guest-settings.png`) });
      await tabs.getByRole("tab", { name: "Map", exact: true }).click();
      await lobby.getByRole("button", { name: "Map", exact: true }).click();
      await page.getByRole("option", { name: "Hellas", exact: true }).click();
      await expect(lobby.getByRole("button", { name: "Map", exact: true })).toHaveText("Hellas");
      await guestLobby.getByRole("tab", { name: "Map", exact: true }).click();
      await expect(guestLobby.getByText("Hellas", { exact: true })).toBeVisible();
      await expect(guestLobby.getByRole("button", { name: "Map", exact: true })).toHaveCount(0);
      expect(errors).toEqual([]);
    } finally {
      await guestContext.close();
      await context.close();
    }
  });
}
