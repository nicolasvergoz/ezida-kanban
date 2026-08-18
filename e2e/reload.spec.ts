import { test, expect, openBoard } from "./fixtures";
import type { Page } from "@playwright/test";

test.use({ fixture: "plain.toml" });

/**
 * Count `GET /api/board` requests on the page. The refresh button
 * reuses the same load path as the initial fetch, so the count is the
 * only observable that isolates the button's contribution without
 * depending on the SSE path (design D5).
 */
function countBoardFetches(page: Page) {
  let n = 0;
  page.on("request", (r) => {
    if (r.method() === "GET" && r.url().includes("/api/board")) n += 1;
  });
  return () => n;
}

const refreshButton = (page: Page) => page.getByRole("button", { name: "Refresh" });

test("clicking Refresh issues exactly one more GET /api/board", async ({ page, board }) => {
  const count = countBoardFetches(page);
  await openBoard(page, board);

  const button = refreshButton(page);
  await expect(button).toBeVisible();
  await expect(button).toBeEnabled();

  const before = count();
  const url = page.url();

  await button.click();

  // The click's request must be the only new one: no SSE-triggered
  // refetch may have fired between the snapshot and the assertion.
  await expect.poll(() => count(), { timeout: 3000 }).toBe(before + 1);

  // Re-fetch in place: same URL, no reload.
  expect(page.url()).toBe(url);
});

test("the Refresh button stays present and enabled while SSE is offline", async ({
  page,
  board,
}) => {
  await openBoard(page, board);
  await expect(page.locator(".server-status")).toHaveAttribute("data-status", "online");

  // Sever the live EventSource stream and keep reconnects failing, so
  // the status flips to offline and stays there.
  await page.route("**/api/events", (route) => route.abort());
  await page.evaluate(() => window.stop());

  await expect(page.locator(".server-status")).toHaveAttribute("data-status", "offline");

  // The refresh control is the manual fallback: always visible, always
  // enabled, regardless of the SSE connection state.
  const button = refreshButton(page);
  await expect(button).toBeVisible();
  await expect(button).toBeEnabled();
});