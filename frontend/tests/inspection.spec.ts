import { test, expect } from "@playwright/test";
import type {} from "../src/dev/inspection/protocol";

test("inspector works without a backend and replays the same placement frame", async ({
  page,
}, info) => {
  test.setTimeout(240_000);
  const errors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") {
      errors.push(message.text());
    }
  });
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.startsWith("/api/")) {
      errors.push(`Backend request: ${request.url()}`);
    }
  });
  const tier = info.project.use.hasTouch ? "low" : "high";
  await page.goto(`/dev/board?card=123&state=placement&view=close&frame=30&gfx=${tier}`);
  await page.waitForFunction(() => window.__openMarsInspection?.ready, undefined, {
    timeout: 180_000,
  });
  await expect(page.getByRole("spinbutton", { name: "Frame", exact: true })).toHaveValue("30");
  await page.screenshot({ path: info.outputPath("tools.png") });
  await page.getByRole("button", { name: "Hide tools", exact: true }).click();
  await page.mouse.move(-10, -10);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
  const first = await page.locator("canvas").screenshot();
  await page.evaluate(() => window.__openMarsInspection!.seek(120));
  const later = await page.locator("canvas").screenshot();
  expect(later.equals(first)).toBe(false);
  await page.evaluate(() => window.__openMarsInspection!.seek(30));
  const replay = await page.locator("canvas").screenshot();
  await info.attach("placement-frame-30", { body: first, contentType: "image/png" });
  await info.attach("replayed-frame-30", { body: replay, contentType: "image/png" });
  const maxChannelDifference = await page.evaluate(
    async ([first, replay]) => {
      const pixels = async (png: string) => {
        const image = new Image();
        image.src = `data:image/png;base64,${png}`;
        await image.decode();
        const canvas = document.createElement("canvas");
        canvas.width = image.width;
        canvas.height = image.height;
        const context = canvas.getContext("2d")!;
        context.drawImage(image, 0, 0);
        return context.getImageData(0, 0, image.width, image.height).data;
      };
      const [a, b] = await Promise.all([pixels(first), pixels(replay)]);
      let max = 0;
      for (let i = 0; i < a.length; i++) {
        max = Math.max(max, Math.abs(a[i] - b[i]));
      }
      return max;
    },
    [first.toString("base64"), replay.toString("base64")],
  );
  // Firefox can round a few antialiased channels differently after rebuilding a WebGL context.
  expect(maxChannelDifference).toBeLessThanOrEqual(2);
  expect(await page.evaluate(() => window.__openMarsInspection!.report())).toMatchObject({
    frame: 30,
    ready: true,
    pending: [],
    graphicsTier: tier,
  });
  expect(errors).toEqual([]);
  await page.getByRole("button", { name: "Inspection tools", exact: true }).click();
  await page
    .getByRole("button", { name: "Copy inspection link", exact: true })
    .scrollIntoViewIfNeeded();
  await expect(
    page.getByRole("button", { name: "Copy inspection link", exact: true }),
  ).toBeVisible();
  await page.screenshot({ path: info.outputPath("tools-scrolled.png") });
  const frameInput = page.getByRole("spinbutton", { name: "Frame", exact: true });
  await frameInput.fill("110");
  expect(await page.evaluate(() => window.__openMarsInspection!.report())).toMatchObject({
    frame: 30,
  });
  await page.getByRole("button", { name: "Go to frame", exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => window.__openMarsInspection!.report()))
    .toMatchObject({ frame: 110, ready: true });
  await page.getByRole("button", { name: "Play", exact: true }).click();
  await expect
    .poll(() =>
      page.evaluate(() => (window.__openMarsInspection!.report() as { frame: number }).frame),
    )
    .toBeGreaterThan(110);
  await page.getByRole("button", { name: "Pause", exact: true }).click();
  await expect
    .poll(() =>
      page.evaluate(() => {
        const frame = (window.__openMarsInspection!.report() as { frame: number }).frame;
        return new URLSearchParams(location.search).get("frame") === String(frame);
      }),
    )
    .toBe(true);
});

test("invalid inspector targets show an actionable error", async ({ page }) => {
  await page.goto("/dev/board?tile=unsupported");
  await expect(page.getByRole("alert")).toContainText("Use --list");
});
