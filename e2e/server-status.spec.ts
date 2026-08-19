import { test, expect, openBoard } from "./fixtures";
import type { Page } from "@playwright/test";

test.use({ fixture: "plain.toml" });

/**
 * Open the ServerStatus overlay by clicking the status dot in the
 * topbar, and return the popover. The dot is the only element inside
 * `.server-status`; its button carries aria-label "Server status: …".
 */
async function openStatusOverlay(page: Page) {
  await page.locator(".server-dot-btn").click();
  const pop = page.locator(".server-popover");
  await pop.waitFor();
  return pop;
}

test("the Status overlay shows Status, Storage and Version rows", async ({ page, board }) => {
  await openBoard(page, board);
  const pop = await openStatusOverlay(page);

  const labels = await pop.locator(".server-row-label").allTextContents();
  // The rows keep the overlay's information order: Status, Storage,
  // then the build version (viewer-ui spec).
  expect(labels.map((l) => l.trim())).toEqual(["Status", "Storage", "Version"]);

  const versionRow = pop.locator(".server-row", { hasText: "Version" });
  // The suite compiles the CLI from the working tree without ldflags,
  // so the honest value is "dev" — never blank, never a missing key.
  await expect(versionRow.locator(".server-row-value")).toHaveText("dev");
});

test("the Status overlay closes on outside click", async ({ page, board }) => {
  await openBoard(page, board);
  const pop = await openStatusOverlay(page);
  await expect(pop).toBeVisible();

  await page.locator(".board").click({ position: { x: 5, y: 5 } });
  await expect(pop).toHaveCount(0);
});

test("the Status overlay closes on Escape", async ({ page, board }) => {
  await openBoard(page, board);
  const pop = await openStatusOverlay(page);
  await expect(pop).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(pop).toHaveCount(0);
});
