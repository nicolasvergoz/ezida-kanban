import { test, expect, openBoard, archivedCardIds, card } from "./fixtures";

test.use({ fixture: "archived.toml" });

const archiveStrip = (page: import("@playwright/test").Page) => page.locator("[data-archive]");

test.describe("the Archive section, paired with an existing archive", () => {
  test("renders collapsed by default with the correct count", async ({ page, board }) => {
    await openBoard(page, board);

    await expect(archiveStrip(page)).toHaveCount(1);
    await expect(archiveStrip(page)).toHaveClass(/collapsed/);
    await expect(archiveStrip(page).locator(".list-archive-count")).toHaveText("4");
  });

  test("expands on click to list exactly the archived ids", async ({ page, board }) => {
    await openBoard(page, board);

    await archiveStrip(page).getByRole("button").first().click();
    await expect(archiveStrip(page)).not.toHaveClass(/collapsed/);

    const ids = await archivedCardIds(page);
    expect(new Set(ids)).toEqual(new Set(["b7m1p4", "vw01k2", "q7t6z2", "t07htj"]));
  });

  test("collapses again on a second click", async ({ page, board }) => {
    await openBoard(page, board);

    await archiveStrip(page).getByRole("button").first().click();
    await expect(archiveStrip(page)).not.toHaveClass(/collapsed/);

    await archiveStrip(page).locator(".list-archive-collapse").click();
    await expect(archiveStrip(page)).toHaveClass(/collapsed/);
  });

  test("an archived card shows its archived date and origin column", async ({ page, board }) => {
    await openBoard(page, board);
    await archiveStrip(page).getByRole("button").first().click();

    const c = card(page, "b7m1p4");
    await expect(c.locator(".card-archived-col")).toHaveText("done");
    await expect(c.locator(".card-archived-at")).toBeVisible();
  });

  test("an archived child shows its epic chip, naming the archived parent", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);
    await archiveStrip(page).getByRole("button").first().click();

    // q7t6z2's epic (vw01k2) is archived alongside it — the chip must
    // still resolve, even though buildEpicIndex for the LIVE board
    // never sees either of them.
    const chip = card(page, "q7t6z2").locator(".card-epic-chip");
    await expect(chip).toContainText("Archived epic");
  });

  test("the archived card's detail view names its epic, marked as archived", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);
    await archiveStrip(page).getByRole("button").first().click();
    await card(page, "q7t6z2").click();

    await expect(page.locator(".modal-overlay")).toHaveCount(1);
    const epicSection = page.locator(".modal-section", { hasText: "Epic" });
    await expect(epicSection).toContainText("Archived epic");
    await expect(epicSection).toContainText("(archived)");
  });

  test("a card whose stored column no longer exists still shows it", async ({ page, board }) => {
    await openBoard(page, board);
    await archiveStrip(page).getByRole("button").first().click();

    // "review" is not in archived.toml's [board].columns — the chip
    // must still show the recorded value, not silently fall back.
    await expect(card(page, "t07htj").locator(".card-archived-col")).toHaveText("review");
  });

  test("a real column named 'archive' would stay addressable by data-column", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);
    // archived.toml has no column literally named "archive", so this
    // asserts the negative: [data-column="archive"] matches nothing,
    // while [data-archive="true"] matches exactly the virtual section.
    await expect(page.locator('[data-column="archive"]')).toHaveCount(0);
    await expect(page.locator('[data-archive="true"]')).toHaveCount(1);
  });
});

test.describe("archived cards are read-only", () => {
  test("no delete button, no tag controls, no drag on an archived card", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);
    await archiveStrip(page).getByRole("button").first().click();

    const c = card(page, "b7m1p4");
    await expect(c.locator(".card-delete")).toHaveCount(0);
    await expect(c).toHaveAttribute("draggable", "false");
  });
});

test.describe("a board with no archive", () => {
  test.use({ fixture: "plain.toml" });

  test("no Archive section renders at all", async ({ page, board }) => {
    await openBoard(page, board);

    await expect(page.locator("[data-archive]")).toHaveCount(0);
    await expect(page.locator(".list-archive")).toHaveCount(0);

    const payload = await page.evaluate(() => fetch("/api/board").then((r) => r.json()));
    expect(Object.keys(payload)).not.toContain("archived_cards");
  });
});
