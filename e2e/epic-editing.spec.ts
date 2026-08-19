import { test, expect, openBoard, card, rgbOf } from "./fixtures";
import type { Page } from "@playwright/test";

/**
 * Editing the epic relation and its colour from the detail modal.
 *
 * The fixture holds two epics — `rl4m9x` with three children and
 * `vw01k2` with one — plus `a3f2k9`, which belongs to neither. Every
 * assertion goes through the real server, so a test that passes here
 * covers the handler, the 400 mapping, the wire and the rendering.
 */

/** Open a card's detail modal. */
async function openCard(page: Page, id: string) {
  await card(page, id).locator(".card-title").click();
  await page.locator(".modal-overlay").waitFor();
}

/** The section that holds the picker for a card with no epic. */
const attach = (page: Page) => page.locator(".modal-epic-empty");
const picker = (page: Page) => page.locator(".epic-picker-input");
const options = (page: Page) => page.locator(".epic-picker-item");

test.describe("attaching, reassigning and detaching", () => {
  test("a card with no epic acquires one, and the chip appears on the board", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "a3f2k9");

    await attach(page).click();
    await picker(page).fill("relations");
    await options(page).first().click();

    await expect(page.locator(".modal-epic-parent .card-epic-chip")).toContainText("Card relations");
    await page.locator(".modal-action", { hasText: "" }).last().click(); // close
    await expect(card(page, "a3f2k9").locator(".card-epic-chip")).toContainText("Card relations");
    // The counter reads the board, so a fourth child moves the total.
    await expect(card(page, "rl4m9x").locator(".epic-count")).toHaveText("1/4");
    expect(board.read()).toContain("epic = 'rl4m9x'");
  });

  test("a child is reassigned to another epic", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "f20wbo");

    await page.locator(".modal-epic-parent-actions button", { hasText: "Change" }).click();
    await picker(page).fill("polish");
    await options(page).first().click();

    await expect(page.locator(".modal-epic-parent .modal-epic-id")).toHaveText("vw01k2");
    await expect(card(page, "f20wbo").locator(".card-epic-chip")).toContainText("Viewer polish");
    await expect(card(page, "rl4m9x").locator(".epic-count")).toHaveText("1/2");
    await expect(card(page, "vw01k2").locator(".epic-count")).toHaveText("0/2");
  });

  test("detaching clears the relation and leaves the attach affordance", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "wrshlo");

    await page.locator(".modal-epic-parent-actions button", { hasText: "Detach" }).click();

    await expect(attach(page)).toBeVisible();
    await expect(page.locator(".modal-epic-parent")).toHaveCount(0);
    await expect(card(page, "wrshlo").locator(".card-epic-chip")).toHaveCount(0);
    // wrshlo was the epic's only card in a terminal column.
    await expect(card(page, "rl4m9x").locator(".epic-count")).toHaveText("0/2");
    expect(board.read().match(/epic = 'rl4m9x'/g)).toHaveLength(2);
  });
});

test.describe("the parent side", () => {
  test("a child is added and removed without ever writing the parent", async ({ page, board }) => {
    await openBoard(page, board);

    // Every relation write is a PATCH on the child; the parent must
    // never be the subject of a request.
    const patched: string[] = [];
    await page.route("**/api/cards/*", (route) => {
      if (route.request().method() === "PATCH") patched.push(route.request().url());
      return route.continue();
    });

    await openCard(page, "vw01k2");
    await page.locator(".modal-epic .modal-section-head button", { hasText: "Add" }).click();
    await picker(page).fill("auth");
    await options(page).first().click();

    const rows = page.locator(".modal-epic-child");
    await expect(rows).toHaveCount(2);
    await expect(page.locator(".modal-epic .epic-count")).toHaveText("0/2");

    await rows.filter({ hasText: "Markdown rendering" }).locator(".modal-epic-child-remove").click();
    await expect(rows).toHaveCount(1);
    await expect(page.locator(".modal-epic .epic-count")).toHaveText("0/1");

    expect(patched).toHaveLength(2);
    for (const url of patched) expect(url).not.toContain("vw01k2");
  });
});

test.describe("the picker", () => {
  test("offers only what the server would accept, on each side", async ({ page, board }) => {
    await openBoard(page, board);

    // Choosing an epic: an existing epic is the ordinary target, a
    // card that already carries one is not, and neither is the card
    // being edited.
    await openCard(page, "a3f2k9");
    await attach(page).click();
    let ids = await options(page).locator(".epic-picker-id").allTextContents();
    expect(ids).toContain("rl4m9x");
    expect(ids).toContain("vw01k2");
    expect(ids).not.toContain("f20wbo");
    expect(ids).not.toContain("a3f2k9");

    await page.keyboard.press("Escape");
    await page.locator(".modal-action").last().click();

    // Choosing a child: a card with children of its own would nest two
    // levels deep, and a card already attached here is pointless.
    await openCard(page, "vw01k2");
    await page.locator(".modal-epic .modal-section-head button", { hasText: "Add" }).click();
    ids = await options(page).locator(".epic-picker-id").allTextContents();
    expect(ids).toContain("a3f2k9");
    expect(ids).toContain("f20wbo");
    expect(ids).not.toContain("rl4m9x");
    expect(ids).not.toContain("l76gjt");
    expect(ids).not.toContain("vw01k2");
  });

  test("filters by title and by id prefix", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "a3f2k9");
    await attach(page).click();

    await picker(page).fill("POLISH");
    await expect(options(page)).toHaveCount(1);
    await expect(options(page).locator(".epic-picker-id")).toHaveText("vw01k2");

    await picker(page).fill("rl4");
    await expect(options(page)).toHaveCount(1);
    await expect(options(page).locator(".epic-picker-id")).toHaveText("rl4m9x");

    await picker(page).fill("nothing matches this");
    await expect(options(page)).toHaveCount(0);
    await expect(page.locator(".epic-picker-empty")).toBeVisible();
  });

  test("arrow keys move the highlight and Enter commits it", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "a3f2k9");
    await attach(page).click();

    // Two candidates in payload order: vw01k2 then rl4m9x.
    await expect(options(page)).toHaveCount(2);
    await page.keyboard.press("ArrowDown");
    await expect(options(page).nth(1)).toHaveClass(/active/);
    await page.keyboard.press("ArrowUp");
    await expect(options(page).nth(0)).toHaveClass(/active/);
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Enter");

    await expect(page.locator(".modal-epic-parent .modal-epic-id")).toHaveText("rl4m9x");
  });

  // The modal closes on Escape from a document-level listener, so
  // abandoning a search must not take the modal with it.
  test("Escape closes the picker and leaves the modal open", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "a3f2k9");
    await attach(page).click();
    await expect(picker(page)).toBeVisible();

    await page.keyboard.press("Escape");

    await expect(picker(page)).toHaveCount(0);
    await expect(page.locator(".modal-overlay")).toBeVisible();
  });

  test("clicking outside closes the picker without committing", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "a3f2k9");
    await attach(page).click();

    await page.locator(".modal-title").click();
    await expect(picker(page)).toHaveCount(0);
    await expect(page.locator(".modal-epic-parent")).toHaveCount(0);
  });
});

test.describe("refusals", () => {
  // The client filters out candidates the server would refuse, so the
  // disagreement is manufactured: the request leaves with an id no card
  // carries, exactly as a board rewritten between fetch and commit
  // would produce. The 400 comes from the real handler.
  test("a rejected target renders the server's sentence and keeps the console clean", async ({ page, board }) => {
    await openBoard(page, board);
    await page.route("**/api/cards/*", (route) => {
      if (route.request().method() !== "PATCH") return route.continue();
      return route.continue({ postData: JSON.stringify({ epic: "zzzzzz" }) });
    });

    await openCard(page, "a3f2k9");
    await attach(page).click();
    await options(page).first().click();

    await expect(page.locator(".modal-error")).toContainText("no card on this board carries that id");
    await expect(page.locator(".modal-epic-parent")).toHaveCount(0);
    // The fixture's console-error guard fails this test if the handled
    // 400 also reached console.error.
  });

  test("the message clears on the next successful mutation", async ({ page, board }) => {
    await openBoard(page, board);
    let sabotage = true;
    await page.route("**/api/cards/*", (route) => {
      if (route.request().method() !== "PATCH") return route.continue();
      if (!sabotage) return route.continue();
      sabotage = false;
      return route.continue({ postData: JSON.stringify({ epic: "zzzzzz" }) });
    });

    await openCard(page, "a3f2k9");
    await attach(page).click();
    await options(page).first().click();
    await expect(page.locator(".modal-error")).toBeVisible();

    await attach(page).click();
    await options(page).first().click();
    await expect(page.locator(".modal-error")).toHaveCount(0);
    await expect(page.locator(".modal-epic-parent")).toBeVisible();
  });
});

test.describe("the palette", () => {
  test("a swatch recolours the epic and every chip it lends its colour to", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "rl4m9x");

    const swatches = page.locator(".modal-epic-swatch");
    await expect(swatches).toHaveCount(9); // eight palette entries plus clear
    await expect(swatches.nth(0)).toHaveClass(/selected/); // violet, the current value

    await swatches.nth(1).click(); // emerald
    await expect(swatches.nth(1)).toHaveClass(/selected/);
    await page.locator(".modal-action").last().click();

    // The chip mixes the stored hex toward the theme's text colour, so
    // it is not the hex itself — but it must be nearer emerald than the
    // violet it replaced. Polled rather than sampled once: the chip
    // repaints on the refetch that follows the PATCH, not on the click.
    const emerald: [number, number, number] = [0x10, 0xb9, 0x81];
    const violet: [number, number, number] = [0x8b, 0x5c, 0xf6];
    const near = (a: [number, number, number], b: [number, number, number]) =>
      Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2]);
    await expect
      .poll(async () => {
        const chip = await rgbOf(page, `[data-card-id="f20wbo"] .card-epic-chip`, "backgroundColor");
        return near(chip, emerald) < near(chip, violet);
      })
      .toBe(true);
    expect(board.read()).toContain("color = '#10b981'");
  });

  test("an off-palette colour is shown rather than silently dropped", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "rl4m9x");

    // Hand-edited colours are legal on disk; the row must represent the
    // current value or a user overwrites it by accident.
    await page.route("**/api/cards/*", (route) => {
      if (route.request().method() !== "PATCH") return route.continue();
      return route.continue({ postData: JSON.stringify({ color: "#123456" }) });
    });
    await page.locator(".modal-epic-swatch").nth(2).click();

    const swatches = page.locator(".modal-epic-swatch");
    await expect(swatches).toHaveCount(10);
    await expect(swatches.nth(8)).toHaveClass(/selected/);
  });

  test("clearing the colour leaves the epic uncoloured", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "rl4m9x");

    await page.locator(".modal-epic-swatch.is-clear").click();
    await expect(page.locator(".modal-epic-swatch.is-clear")).toHaveClass(/selected/);
    expect(board.read()).not.toContain("color = '#8b5cf6'");
  });

  // Colour has no rendering consequence on a card with no children, so
  // the control is not offered there.
  test("no swatch row on a card without children", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "a3f2k9");
    await expect(page.locator(".modal-epic-swatch")).toHaveCount(0);
  });
});

test.describe("navigation between epic and children in modal", () => {
  test("clicking a child in the epic detail modal navigates to that child card's detail modal", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "rl4m9x");

    // Initially open on the epic card.
    await expect(page.locator(".modal-title")).toHaveText("Card relations and a deliberately long epic title");
    await expect(page.locator(".modal-epic-children")).toBeVisible();

    // Click on child card 'f20wbo' (Card dependencies).
    const childBtn = page.locator(".modal-epic-child-main", { hasText: "Card dependencies" });
    await childBtn.click();

    // Modal is now open on child card 'f20wbo'.
    await expect(page.locator(".modal-title")).toHaveText("Card dependencies");
    await expect(page.locator(".modal-epic-parent .modal-epic-id")).toHaveText("rl4m9x");
    await expect(page.locator(".modal-epic-parent .card-epic-chip")).toContainText("Card relations");
    await expect(page.locator(".modal-epic-children")).toHaveCount(0);
  });

  test("clicking the parent epic chip in the child detail modal navigates to the parent epic modal", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "f20wbo");

    // Initially open on the child card.
    await expect(page.locator(".modal-title")).toHaveText("Card dependencies");
    await expect(page.locator(".modal-epic-parent .modal-epic-id")).toHaveText("rl4m9x");

    // Click on parent epic chip.
    await page.locator(".modal-epic-parent .card-epic-chip").click();

    // Modal is now open on the parent epic card.
    await expect(page.locator(".modal-title")).toHaveText("Card relations and a deliberately long epic title");
    await expect(page.locator(".modal-epic-children")).toBeVisible();
    await expect(page.locator(".modal-epic-parent")).toHaveCount(0);
  });

  test("clicking remove on a child does not navigate to that child", async ({ page, board }) => {
    await openBoard(page, board);
    await openCard(page, "vw01k2");

    // Initially open on epic 'vw01k2' with child 'l76gjt' (Markdown rendering).
    await expect(page.locator(".modal-title")).toHaveText("Viewer polish");
    const row = page.locator(".modal-epic-child", { hasText: "Markdown rendering" });
    await expect(row).toBeVisible();

    // Click the remove button on the child row.
    await row.locator(".modal-epic-child-remove").click();

    // The child is removed, and the modal remains on the parent epic.
    await expect(page.locator(".modal-title")).toHaveText("Viewer polish");
    await expect(page.locator(".modal-epic-child")).toHaveCount(0);
  });
});
