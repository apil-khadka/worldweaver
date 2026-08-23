import { chromium } from "playwright";
import { copyFile, mkdir } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(new URL(".", import.meta.url).pathname);
const captures = path.join(root, "captures");
const rawVideoDir = path.join(root, "build", "raw-video");
const appUrl = process.env.WORLDWEAVER_URL ?? "https://worldweaver.apilkhadka.com.np/play";
await mkdir(captures, { recursive: true });
await mkdir(rawVideoDir, { recursive: true });

const browser = await chromium.launch({
  headless: true,
  // Use the app's Canvas2D fallback for a complete snapshot while the live
  // WebGL texture upload issue is being fixed separately.
  args: ["--disable-webgl"],
});

const context = await browser.newContext({
  viewport: { width: 1920, height: 1080 },
  deviceScaleFactor: 1,
  recordVideo: { dir: rawVideoDir, size: { width: 1920, height: 1080 } },
});

const page = await context.newPage();
page.setDefaultTimeout(15000);
page.on("console", (msg) => console.log("[page]", msg.type(), msg.text()));
page.on("requestfailed", (req) => console.log("[request-failed]", req.url(), req.failure()?.errorText));

await page.goto(appUrl, { waitUntil: "domcontentloaded" });
await page.waitForTimeout(1200);
await page.screenshot({ path: path.join(captures, "01-lobby.png") });

await page.locator("#lobby-nickname").fill("Demo Player");
const firstWorld = page.locator(".lobby-world-row").first();
await firstWorld.locator("button.lobby-join-btn").click();
await page.waitForFunction(
  () => document.getElementById("game-container")?.classList.contains("visible"),
  undefined,
  { timeout: 30000 },
);
await page.waitForTimeout(7000);
await page.screenshot({ path: path.join(captures, "02-game-start.png") });

const canvas = page.locator("#world-canvas");
const box = await canvas.boundingBox();
if (!box) throw new Error("world canvas did not become visible");

const worldPoint = (x, y) => ({
  x: box.x + box.width * x,
  y: box.y + box.height * y,
});

const clickWorld = async (x, y, settleMs = 1600) => {
  const point = worldPoint(x, y);
  await page.mouse.move(point.x, point.y);
  await page.mouse.click(point.x, point.y);
  await page.waitForTimeout(settleMs);
};

// Use the real element catalogue to choose Fire. This gives the viewer a
// meaningful discovery moment before the material is placed on the terrain.
await page.locator("#element-drawer-toggle").click();
await page.waitForTimeout(700);
await page.locator("#element-search").fill("Steam");
await page.waitForTimeout(500);
await page.screenshot({ path: path.join(captures, "03-elements-steam.png") });
await page.locator("#element-drawer-close").click();
await page.waitForTimeout(500);

// One deliberate place action per beat keeps the recording readable and stays
// below the server's action rate limit. The target sits on the visible mountain
// face, so the before/after difference is unambiguous.
await page.locator('.tool-btn[data-tool="place"]').click();
await page.locator('.mat-swatch[title="Fire"]').click();
await clickWorld(0.50, 0.53, 1800);
await page.screenshot({ path: path.join(captures, "04-fire-placed.png") });

// Water placed over the fire creates the visible reaction beat.
await page.locator('.mat-swatch[title="Water"]').click();
await clickWorld(0.50, 0.53, 2500);
await page.screenshot({ path: path.join(captures, "05-water-reaction.png") });

// Finish with one real force action over the existing lake and one growth action
// on the grassy shoreline, each separated long enough for the server to react.
await page.locator(".power-btn").filter({ hasText: "Rain" }).click();
await clickWorld(0.68, 0.56, 2200);
await page.screenshot({ path: path.join(captures, "06-rain.png") });

await page.locator(".power-btn").filter({ hasText: "Growth" }).click();
await clickWorld(0.72, 0.51, 2200);
await page.screenshot({ path: path.join(captures, "07-growth.png") });

// Leave the world running long enough for the judge to see the simulation react,
// rather than cutting immediately after the last input.
await page.waitForTimeout(2600);

const elements = page.locator("button").filter({ hasText: "Elements" });
if (await elements.count()) {
  await elements.first().click();
  await page.waitForTimeout(700);
  await page.screenshot({ path: path.join(captures, "06-elements.png") });
}

// A second client joins the same public world. The side-by-side capture is used
// as a clean multiplayer proof frame in the final edit.
const secondContext = await browser.newContext({
  viewport: { width: 1920, height: 1080 },
  deviceScaleFactor: 1,
});
const second = await secondContext.newPage();
second.setDefaultTimeout(15000);
await second.goto(appUrl, { waitUntil: "domcontentloaded" });
await second.waitForTimeout(1400);
await second.locator("#lobby-nickname").fill("Second Player");
await second.locator(".lobby-world-row").first().locator("button.lobby-join-btn").click();
await second.waitForFunction(
  () => document.getElementById("game-container")?.classList.contains("visible"),
  undefined,
  { timeout: 30000 },
);
await second.waitForTimeout(7000);
await second.screenshot({ path: path.join(captures, "08-player-two.png") });

await page.screenshot({ path: path.join(captures, "09-player-one.png") });

await secondContext.close();
const rawVideo = await page.video().path();
await context.close();
await copyFile(rawVideo, path.join(root, "build", "gameplay.webm"));
await browser.close();

console.log("Captured gameplay and UI frames in", captures);
console.log("Gameplay recording:", path.join(root, "build", "gameplay.webm"));
