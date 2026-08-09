import { test, expect, openBoard, card, visibleCardIds, openFilter } from "./fixtures";

const RELATIONS = "rl4m9x";
const RELATIONS_KIDS = ["f20wbo", "wrshlo", "q7t6z2"];

test.describe("focusing an epic", () => {
  test("the chip is the way in, and does not also open the modal", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();

    const ids = await visibleCardIds(page);
    expect(new Set(ids)).toEqual(new Set([RELATIONS, ...RELATIONS_KIDS]));
    await expect(page.locator(".modal-overlay")).toHaveCount(0);
  });

  test("the parent survives its own focus, with its progress intact", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();

    // The clause has to be `card.epic === id || card.id === id`.
    // Matching on `epic` alone hides the epic — and with it the bar
    // and counter that are the reason to focus it.
    await expect(card(page, RELATIONS)).toBeVisible();
    await expect(card(page, RELATIONS).locator(".epic-count")).toHaveText("1/3");
  });

  test("clicking the chip again releases the focus", async ({ page, board }) => {
    await openBoard(page, board);
    const chip = card(page, "f20wbo").locator(".card-epic-chip");

    await chip.click();
    expect(await visibleCardIds(page)).toHaveLength(4);
    await chip.click();
    expect(await visibleCardIds(page)).toHaveLength(7);
  });

  test("the button goes active and the badge counts the parent too", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();

    await expect(page.locator(".iconbtn-badge")).toHaveText("4");
  });

  test("the chip inside the modal stays inert", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-title").click();

    await page.locator(".modal-epic-parent .card-epic-chip").click();

    await expect(page.locator(".modal-overlay")).toBeVisible();
    await expect(page.locator(".iconbtn-badge")).toHaveCount(0);
  });
});

test.describe("the Epic section of the filter popover", () => {
  test("lists one pill per epic, in payload order, with its colour", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);

    const pills = pop.locator(".filter-pill-epic");
    // `vw01k2` is declared before `rl4m9x` in the fixture.
    await expect(pills).toHaveText([
      "Viewer polish",
      "Card relations and a deliberately long epic title",
    ]);
    await expect(pills.nth(0).locator(".filter-pill-dot")).toHaveCSS(
      "background-color",
      "rgb(16, 185, 129)",
    );
    await expect(pop.getByRole("button", { name: "No epic" })).toBeVisible();
  });

  test("a long pill label truncates but keeps the full title", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);
    const long = pop.locator(".filter-pill-epic").nth(1);

    await expect(long).toHaveAttribute(
      "title",
      "Card relations and a deliberately long epic title",
    );
    const label = long.locator(".filter-pill-label");
    const [scroll, client] = await label.evaluate((e) => [e.scrollWidth, e.clientWidth]);
    expect(scroll).toBeGreaterThan(client);
  });

  test("a pill toggles, and reports its state to assistive tech", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);
    const pill = pop.locator(".filter-pill-epic").nth(1);

    await expect(pill).toHaveAttribute("aria-pressed", "false");
    await pill.click();
    await expect(pill).toHaveAttribute("aria-pressed", "true");
    expect(await visibleCardIds(page)).toHaveLength(4);

    await pill.click();
    await expect(pill).toHaveAttribute("aria-pressed", "false");
    expect(await visibleCardIds(page)).toHaveLength(7);
  });

  test("two epics widen the dimension rather than narrowing it", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);

    await pop.locator(".filter-pill-epic").nth(0).click();
    await pop.locator(".filter-pill-epic").nth(1).click();

    const ids = new Set(await visibleCardIds(page));
    expect(ids).toEqual(new Set([RELATIONS, ...RELATIONS_KIDS, "vw01k2", "l76gjt"]));
    expect(ids.has("a3f2k9")).toBe(false);
  });

  test("clicking a chip lights the matching pill", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();
    const pop = await openFilter(page);

    await expect(pop.locator(".filter-pill-epic").nth(1)).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  test("Clear all releases the epic scope with the rest", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);

    await pop.locator(".filter-pill-epic").nth(1).click();
    await pop.getByRole("button", { name: "Clear all" }).click();

    expect(await visibleCardIds(page)).toHaveLength(7);
    await expect(page.locator(".iconbtn-badge")).toHaveCount(0);
  });
});

test.describe("the No epic scope", () => {
  test("means unrelated, so it hides parents as well as children", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);
    await pop.getByRole("button", { name: "No epic" }).click();

    // A card six others point at is the least epic-less card there is.
    expect(await visibleCardIds(page)).toEqual(["a3f2k9"]);
  });

  test("unions with a named epic", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);
    await pop.getByRole("button", { name: "No epic" }).click();
    await pop.locator(".filter-pill-epic").nth(1).click();

    const ids = new Set(await visibleCardIds(page));
    expect(ids).toEqual(new Set(["a3f2k9", RELATIONS, ...RELATIONS_KIDS]));
  });
});

test.describe("the epic dimension combines with the others", () => {
  test("AND with the query", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);
    await pop.locator(".filter-pill-epic").nth(1).click();
    await page.locator(".filter-input").fill("due");

    expect(await visibleCardIds(page)).toEqual(["wrshlo"]);
  });

  test("AND with a priority", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);
    await pop.locator(".filter-pill-epic").nth(1).click();
    await pop.getByRole("button", { name: "Low" }).click();

    expect(await visibleCardIds(page)).toEqual(["q7t6z2"]);
  });
});

test.describe("focus is transient", () => {
  test("a reload clears it, and nothing was persisted", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();
    expect(await visibleCardIds(page)).toHaveLength(4);

    const keys = await page.evaluate(() => Object.keys(localStorage));
    expect(keys).toEqual(["kanban.theme"]);
    expect(page.url()).not.toContain("epic");

    await page.reload();
    await page.locator(".card").first().waitFor();
    expect(await visibleCardIds(page)).toHaveLength(7);
  });
});

test.describe("focus does not break the rest of the board", () => {
  test("delete still works and the focus survives it", async ({ page, board }) => {
    await openBoard(page, board);
    page.on("dialog", (d) => d.accept());
    await card(page, "f20wbo").locator(".card-epic-chip").click();

    await card(page, "q7t6z2").hover();
    await card(page, "q7t6z2").locator(".card-delete").click();

    await expect(card(page, "q7t6z2")).toHaveCount(0);
    await expect(card(page, RELATIONS)).toBeVisible();
    await expect(page.locator(".iconbtn-badge")).toHaveText("3");
    expect(board.read()).not.toContain("q7t6z2");
  });

  test("cards stay draggable under an active focus", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();

    await expect(card(page, RELATIONS)).toHaveAttribute("draggable", "true");
    await expect(card(page, "wrshlo")).toHaveAttribute("draggable", "true");
  });

  /**
   * A card title has one gesture and it opens the modal. The inline
   * composer that `onDoubleClick` used to reach was unreachable with a
   * mouse — the card's own `onClick` opened the modal before the
   * second click landed — and the handler, its state and its whole
   * prop chain were removed rather than untangled: editing a title is
   * the modal's job.
   *
   * Kept as a test because a double-click is still a double-click, and
   * the second one must not do something surprising.
   */
  test("double-clicking a title opens the modal and nothing else", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();

    await card(page, "wrshlo").locator(".card-title").dblclick();

    await expect(page.locator(".modal-overlay")).toBeVisible();
    await expect(page.locator(".card-composer")).toHaveCount(0);
  });

  test("a card still opens its modal from the title", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-epic-chip").click();
    await card(page, RELATIONS).locator(".card-title").click();

    await expect(page.locator(".modal-overlay")).toBeVisible();
  });
});

test.describe("a board with no epics", () => {
  test.use({ fixture: "plain.toml" });

  test("renders no Epic section at all", async ({ page, board }) => {
    await openBoard(page, board);
    const pop = await openFilter(page);

    await expect(pop.locator(".filter-pill-epic")).toHaveCount(0);
    await expect(pop.getByRole("button", { name: "No epic" })).toHaveCount(0);
    await expect(pop.locator(".popover-sub")).toHaveText(["Search in", "Priority"]);
  });
});
