import { test, expect, openBoard, type Board } from "./fixtures";
import type { Page } from "@playwright/test";

test.use({ fixture: "terminal.toml" });

/** Every column PATCH the page fired, in order, with its parsed body. */
function recordColumnPatches(page: Page) {
  const seen: { url: string; body: any }[] = [];
  page.on("request", (r) => {
    if (r.method() !== "PATCH") return;
    if (!r.url().includes("/api/columns/")) return;
    seen.push({ url: r.url(), body: JSON.parse(r.postData() ?? "{}") });
  });
  return seen;
}

const list = (page: Page, name: string) => page.locator(`.list[data-column="${name}"]`);

/** Open the ⋯ menu on a column and return the popover. */
async function openListMenu(page: Page, column: string) {
  await list(page, column).getByRole("button", { name: "More options" }).click();
  const pop = list(page, column).locator(".popover");
  await pop.waitFor();
  return pop;
}

/** Click the column title to enter rename mode; returns the input. */
async function openRename(page: Page, column: string) {
  await list(page, column).locator("span.list-title").click();
  const input = list(page, column).locator("input.list-title");
  await input.waitFor();
  return input;
}

const stagedCheck = (page: Page, column: string) =>
  list(page, column).getByRole("checkbox", { name: /Terminal column/ });

test.describe("the ⋯ menu toggles the terminal marker", () => {
  test("marking a column terminal sends done alone and shows the marker", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    await expect(list(page, "review").locator(".list-done-mark")).toHaveCount(0);

    const menu = await openListMenu(page, "review");
    await menu.getByRole("menuitemcheckbox", { name: "Terminal column" }).click();

    // The marker appearing proves the round trip: write, SSE, refetch.
    await expect(list(page, "review").locator(".list-done-mark")).toHaveCount(1);
    await expect(menu).toHaveCount(0);

    expect(patches).toHaveLength(1);
    expect(patches[0].url).toContain("/api/columns/review");
    expect(patches[0].body).toEqual({ done: true });
    // A stale client-side name must never ride along as a rename.
    expect(patches[0].body).not.toHaveProperty("name");

    expect(board.read()).toContain("review*");
  });

  test("clearing the marker sends done:false", async ({ page, board }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    await expect(list(page, "done").locator(".list-done-mark")).toHaveCount(1);

    const menu = await openListMenu(page, "done");
    await menu.getByRole("menuitemcheckbox", { name: "Terminal column" }).click();

    await expect(list(page, "done").locator(".list-done-mark")).toHaveCount(0);
    expect(patches).toEqual([
      expect.objectContaining({ body: { done: false } }),
    ]);
    expect(board.read()).not.toContain("done*");
  });

  test("the entry reports the column's current state", async ({ page, board }) => {
    await openBoard(page, board);

    const terminal = await openListMenu(page, "done");
    await expect(
      terminal.getByRole("menuitemcheckbox", { name: "Terminal column" }),
    ).toHaveAttribute("aria-checked", "true");

    await page.keyboard.press("Escape");
    await list(page, "done").getByRole("button", { name: "More options" }).click();

    const plain = await openListMenu(page, "review");
    await expect(
      plain.getByRole("menuitemcheckbox", { name: "Terminal column" }),
    ).toHaveAttribute("aria-checked", "false");
  });
});

test.describe("the rename input stages the terminal marker", () => {
  test("the check reflects the column state when the editor opens", async ({ page, board }) => {
    await openBoard(page, board);

    await openRename(page, "done");
    await expect(stagedCheck(page, "done")).toHaveAttribute("aria-checked", "true");

    await page.keyboard.press("Escape");

    await openRename(page, "review");
    await expect(stagedCheck(page, "review")).toHaveAttribute("aria-checked", "false");
  });

  test("the resting mark stands down while the editor is open", async ({ page, board }) => {
    await openBoard(page, board);
    const done = list(page, "done");
    await expect(done.locator(".list-done-mark")).toHaveCount(1);

    const input = await openRename(page, "done");
    // One terminal signal at a time: two marks read as two settings.
    await expect(done.locator(".list-done-mark")).toHaveCount(0);
    await expect(stagedCheck(page, "done")).toHaveAttribute("aria-checked", "true");

    await input.press("Escape");
    await expect(done.locator(".list-done-mark")).toHaveCount(1);
  });

  test("activating the check fires nothing and keeps the input focused", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    const input = await openRename(page, "review");
    await expect(input).toBeFocused();

    await stagedCheck(page, "review").click();

    // The trap this control exists to avoid: a click that blurs the
    // input commits the rename before the marker can ride along.
    await expect(input).toBeFocused();
    await expect(stagedCheck(page, "review")).toHaveAttribute("aria-checked", "true");
    expect(patches).toEqual([]);
  });

  test("a rename and a staged marker commit as ONE request", async ({ page, board }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    const input = await openRename(page, "review");
    await input.fill("shipped");
    await stagedCheck(page, "review").click();
    await input.press("Enter");

    await expect(list(page, "shipped").locator(".list-done-mark")).toHaveCount(1);

    // The whole point of staging: the name never lands without the
    // marker it was committed with.
    expect(patches).toHaveLength(1);
    expect(patches[0].url).toContain("/api/columns/review");
    expect(patches[0].body).toEqual({ name: "shipped", done: true });

    expect(board.read()).toContain("shipped*");
  });

  test("a staged marker alone commits done without a name", async ({ page, board }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    const input = await openRename(page, "review");
    await stagedCheck(page, "review").click();
    await input.press("Enter");

    await expect(list(page, "review").locator(".list-done-mark")).toHaveCount(1);
    expect(patches).toHaveLength(1);
    expect(patches[0].body).toEqual({ done: true });
  });

  test("a rename alone commits a name without a marker key", async ({ page, board }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    const input = await openRename(page, "review");
    await input.fill("shipped");
    await input.press("Enter");

    await expect(list(page, "shipped")).toHaveCount(1);
    expect(patches).toHaveLength(1);
    expect(patches[0].body).toEqual({ name: "shipped" });
  });

  test("Escape discards the staged marker", async ({ page, board }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    const input = await openRename(page, "review");
    await stagedCheck(page, "review").click();
    await input.press("Escape");

    await expect(list(page, "review").locator("span.list-title")).toBeVisible();
    await expect(list(page, "review").locator(".list-done-mark")).toHaveCount(0);
    expect(patches).toEqual([]);
  });

  test("an emptied name discards the staged marker too", async ({ page, board }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    const input = await openRename(page, "review");
    await stagedCheck(page, "review").click();
    await input.fill("");
    await input.press("Enter");

    await expect(list(page, "review").locator("span.list-title")).toBeVisible();
    await expect(list(page, "review").locator(".list-done-mark")).toHaveCount(0);
    expect(patches).toEqual([]);
  });

  test("toggling the check back commits nothing", async ({ page, board }) => {
    await openBoard(page, board);
    const patches = recordColumnPatches(page);

    const input = await openRename(page, "review");
    await stagedCheck(page, "review").click();
    await stagedCheck(page, "review").click();
    await input.press("Enter");

    await expect(list(page, "review").locator("span.list-title")).toBeVisible();
    expect(patches).toEqual([]);
  });

  test("the check is not a column drag handle", async ({ page, board }) => {
    await openBoard(page, board);

    const moves: string[] = [];
    page.on("request", (r) => {
      if (r.url().includes("/api/columns/move")) moves.push(r.url());
    });

    await openRename(page, "review");
    const check = stagedCheck(page, "review");
    await expect(check).toHaveAttribute("draggable", "false");

    // Playwright cannot drive native dataTransfer, so this asserts the
    // observable half: pressing and dragging the check reorders nothing
    // and calls no move endpoint (see CLAUDE.md on HTML5 drag).
    const box = (await check.boundingBox())!;
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + 300, box.y, { steps: 10 });
    await page.mouse.up();

    expect(moves).toEqual([]);
    await expect(page.locator(".list")).toHaveCount(3);
    const order = await page
      .locator(".list")
      .evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.column));
    expect(order).toEqual(["todo", "review", "done"]);
  });
});

test.describe("a board that uses none of it", () => {
  test.use({ fixture: "plain.toml" });

  test("no column is marked and the controls still work", async ({ page, board }) => {
    await openBoard(page, board);

    await expect(page.locator(".list-done-mark")).toHaveCount(0);

    // The header controls exist regardless; a board with no terminal
    // column is the normal way to acquire one.
    const menu = await openListMenu(page, "done");
    await expect(
      menu.getByRole("menuitemcheckbox", { name: "Terminal column" }),
    ).toHaveAttribute("aria-checked", "false");

    expect(board.read()).not.toContain("*");
  });
});
