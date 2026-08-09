import { test, expect, openBoard, card, visibleCardIds } from "./fixtures";

test.describe("epic rendering", () => {
  test("a child wears a chip named after its parent", async ({ page, board }) => {
    await openBoard(page, board);

    const chip = card(page, "f20wbo").locator(".card-epic-chip");
    await expect(chip).toHaveText("Card relations and a deliberately long epic title");
    await expect(chip).toHaveAttribute("title", /Card relations/);

    // Coloured from the parent's hex, never from the child's own.
    await expect(chip).toHaveCSS("--epic-color", "#8b5cf6");
  });

  test("the chip precedes the tags and follows the priority pill", async ({ page, board }) => {
    await openBoard(page, board);

    const order = await card(page, "q7t6z2")
      .locator(".card-foot > *")
      .evaluateAll((els) => els.map((e) => e.className.split(" ")[0]));

    expect(order[0]).toBe("card-prio-pill");
    expect(order[1]).toBe("card-epic-chip");
    expect(order.indexOf("card-tags")).toBeGreaterThan(1);
  });

  test("a parent renders the glyph, a tinted border and a progress bar", async ({ page, board }) => {
    await openBoard(page, board);
    const parent = card(page, "rl4m9x");

    await expect(parent).toHaveClass(/is-epic/);
    await expect(parent.locator(".card-title .card-epic-glyph")).toBeVisible();
    await expect(parent).toHaveCSS("--epic-color", "#8b5cf6");

    // One of three children sits in `shipped`, a terminal column.
    await expect(parent.locator(".epic-count")).toHaveText("1/3");
    await expect(parent.locator(".epic-bar")).toHaveAttribute("aria-valuenow", "1");
    await expect(parent.locator(".epic-bar")).toHaveAttribute("aria-valuemax", "3");
  });

  test("an epic with nothing done renders an empty bar, not a missing one", async ({ page, board }) => {
    await openBoard(page, board);
    const parent = card(page, "vw01k2");

    await expect(parent.locator(".epic-count")).toHaveText("0/1");
    await expect(parent.locator(".epic-bar")).toBeVisible();
    await expect(parent.locator(".epic-bar-fill")).toHaveCSS("width", "0px");
  });

  test("a card that is neither child nor parent carries no epic chrome", async ({ page, board }) => {
    await openBoard(page, board);
    const plain = card(page, "a3f2k9");

    await expect(plain).not.toHaveClass(/is-epic/);
    await expect(plain.locator(".card-epic-chip")).toHaveCount(0);
    await expect(plain.locator(".epic-progress")).toHaveCount(0);
  });

  test("terminal columns are marked and the on-disk `*` never reaches the DOM", async ({ page, board }) => {
    await openBoard(page, board);

    const marked = page.locator(".list", { has: page.locator(".list-done-mark") });
    await expect(marked).toHaveCount(2);
    await expect(
      page.locator(".list", { hasText: "SHIPPED" }).locator(".list-done-mark"),
    ).toHaveAttribute("aria-label", /terminal/i);
    await expect(
      page.locator(".list", { hasText: "BACKLOG" }).locator(".list-done-mark"),
    ).toHaveCount(0);

    // The marker is a kanban.toml spelling. It is on disk...
    expect(board.read()).toContain("shipped*");
    // ...and nowhere in what the browser received.
    const body = await page.locator("#root").innerHTML();
    expect(body).not.toContain("*");
  });

  test("the counter reports the board even when the children are filtered away", async ({ page, board }) => {
    await openBoard(page, board);

    // Matches the parent's title only; every child disappears.
    await page.getByRole("button", { name: "Filter" }).click();
    await page.locator(".filter-input").fill("deliberately");

    const ids = await visibleCardIds(page);
    expect(ids).toEqual(["rl4m9x"]);
    await expect(card(page, "rl4m9x").locator(".epic-count")).toHaveText("1/3");
  });
});

test.describe("the detail modal reports the relation, read-only", () => {
  test("a child names its parent", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "f20wbo").locator(".card-title").click();

    const section = page.locator(".modal-epic");
    await expect(section.locator(".modal-label")).toHaveText("Epic");
    await expect(section.locator(".card-epic-chip")).toContainText("Card relations");
    await expect(section.locator(".modal-epic-id")).toHaveText("rl4m9x");
    await expect(page.locator(".modal-epic-children")).toHaveCount(0);
  });

  test("a parent lists its children in board order with their columns", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "rl4m9x").locator(".card-title").click();

    const rows = page.locator(".modal-epic-child");
    await expect(rows).toHaveCount(3);
    await expect(rows.locator(".modal-epic-child-title")).toHaveText([
      "Card dependencies",
      "Card due dates",
      "Card colors",
    ]);
    await expect(rows.nth(1).locator(".modal-epic-child-col")).toHaveText("SHIPPED");
    await expect(page.locator(".modal-epic .epic-count")).toHaveText("1/3");
    await expect(page.locator(".modal-epic-parent")).toHaveCount(0);
  });

  test("an unrelated card shows no relation section", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "a3f2k9").locator(".card-title").click();

    await expect(page.locator(".modal-overlay")).toBeVisible();
    await expect(page.locator(".modal-epic")).toHaveCount(0);
  });

  test("neither section offers a way to change the relation", async ({ page, board }) => {
    await openBoard(page, board);
    await card(page, "rl4m9x").locator(".card-title").click();

    // Read-only until the modal change lands: no picker, no swatch, no
    // add or remove control anywhere in the section.
    await expect(page.locator(".modal-epic button")).toHaveCount(0);
    await expect(page.locator(".modal-epic input")).toHaveCount(0);
  });
});

test.describe("a board with no epics", () => {
  test.use({ fixture: "plain.toml" });

  test("renders none of the epic chrome", async ({ page, board }) => {
    await openBoard(page, board);

    await expect(page.locator(".card-epic-chip")).toHaveCount(0);
    await expect(page.locator(".card-epic-glyph")).toHaveCount(0);
    await expect(page.locator(".epic-progress")).toHaveCount(0);
    await expect(page.locator(".list-done-mark")).toHaveCount(0);
    await expect(page.locator(".card.is-epic")).toHaveCount(0);
  });

  test("omits `epic` and `color` from the wire entirely", async ({ page, board }) => {
    const payload = await (await fetch(`${board.url}/api/board`)).json();

    for (const c of payload.cards) {
      expect(Object.keys(c)).not.toContain("epic");
      expect(Object.keys(c)).not.toContain("color");
    }
    expect(payload.done_columns).toEqual([]);
    expect(payload.schema_version).toBe(2);
  });
});
