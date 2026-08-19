import { test, expect, openBoard, card, liveCardIds, archivedCardIds } from "./fixtures";
import type { Page } from "@playwright/test";

/**
 * The archive/unarchive round trip driven entirely through the modal
 * and the column menu — the non-drag affordances CLAUDE.md requires,
 * since Playwright cannot drive real HTML5 drag-and-drop.
 *
 * `archive-source.toml` boots with NO archive sibling (unlike
 * `archived.toml`, which is paired with `archived.archive.toml` for
 * archive-render.spec.ts — reusing that stem here would silently pull
 * in that pairing by startServer's naming convention): the
 * highest-value assertion in this file is that the Archive section is
 * created by the first archive action and removed again when the last
 * card is restored — the omitempty wire contract made visible in the
 * DOM.
 */
test.use({ fixture: "archive-source.toml" });

const archiveSection = (page: Page) => page.locator("[data-archive]");
const list = (page: Page, name: string) => page.locator(`.list[data-column="${name}"]`);

async function openCard(page: Page, id: string) {
  await card(page, id).locator(".card-title").click();
  await page.locator(".modal-overlay").waitFor();
}

async function openArchivedCard(page: Page, id: string) {
  await archiveSection(page).getByRole("button").first().click(); // expand
  await card(page, id).click();
  await page.locator(".modal-overlay").waitFor();
}

async function openListMenu(page: Page, column: string) {
  await list(page, column).getByRole("button", { name: "More options" }).click();
  const pop = list(page, column).locator(".popover");
  await pop.waitFor();
  return pop;
}

test.describe("archiving and restoring a single card", () => {
  test("the Archive section is created on first archive and removed on last restore", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);
    await expect(archiveSection(page)).toHaveCount(0);
    expect(board.readArchive()).toBeNull();

    await openCard(page, "a3f2k9");
    await page.getByTitle("Archive card").click();

    await expect(page.locator(".modal-overlay")).toHaveCount(0);
    await expect(card(page, "a3f2k9")).toHaveCount(0);
    await expect(archiveSection(page)).toHaveCount(1);
    await expect.poll(() => board.readArchive()).not.toBeNull();
    expect(board.readArchive()).toContain("a3f2k9");

    await openArchivedCard(page, "a3f2k9");
    await page.getByTitle("Restore card").click();

    await expect(page.locator(".modal-overlay")).toHaveCount(0);
    await expect(archiveSection(page)).toHaveCount(0);
    await expect.poll(() => board.readArchive()).toBeNull();
    await expect(card(page, "a3f2k9")).toHaveCount(1);
  });
});

test.describe("archiving an epic cascades its children", () => {
  test("confirms before archiving, then both parent and child move together", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);

    page.once("dialog", (d) => d.accept());
    await openCard(page, "rl4m9x");
    await page.getByTitle("Archive card").click();

    await expect(page.locator(".modal-overlay")).toHaveCount(0);
    const live = await liveCardIds(page);
    expect(live).not.toContain("rl4m9x");
    expect(live).not.toContain("f20wbo");

    await archiveSection(page).getByRole("button").first().click();
    const archived = await archivedCardIds(page);
    expect(archived).toContain("rl4m9x");
    expect(archived).toContain("f20wbo");
  });

  test("declining the confirmation archives nothing", async ({ page, board }) => {
    await openBoard(page, board);
    const before = board.read();

    page.once("dialog", (d) => d.dismiss());
    await openCard(page, "rl4m9x");
    await page.getByTitle("Archive card").click();

    // Declining leaves the modal open — it never sent the request.
    await expect(page.locator(".modal-overlay")).toHaveCount(1);
    expect(board.read()).toBe(before);
  });
});

test.describe("archiving a column from its menu", () => {
  test("archives every card in the column with no confirmation when nothing cascades", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);

    const menu = await openListMenu(page, "ongoing");
    await menu.getByRole("button", { name: /Archive all cards/ }).click();

    await expect(list(page, "ongoing").locator(".card")).toHaveCount(0);
    await expect(archiveSection(page)).toHaveCount(1);
  });

  test("archiving unblocks deleting a column that previously had cards", async ({
    page,
    board,
  }) => {
    await openBoard(page, board);

    const menu = await openListMenu(page, "ongoing");
    await menu.getByRole("button", { name: /Archive all cards/ }).click();
    await expect(list(page, "ongoing").locator(".card")).toHaveCount(0);

    const menu2 = await openListMenu(page, "ongoing");
    await menu2.getByRole("button", { name: "Delete list" }).click();

    await expect(list(page, "ongoing")).toHaveCount(0);
  });

  test("a cascade outside the column surfaces a post-hoc notice", async ({ page, board }) => {
    await openBoard(page, board);

    // "todo" holds rl4m9x (an epic) whose child f20wbo lives in
    // "ongoing" — archiving "todo" cascades into "ongoing" with no
    // pre-flight prompt; the server-computed cascade is reported after
    // the fact via a plain alert().
    let dialogMessage = "";
    page.once("dialog", (d) => { dialogMessage = d.message(); d.accept(); });

    const menu = await openListMenu(page, "todo");
    await menu.getByRole("button", { name: /Archive all cards/ }).click();

    await expect.poll(() => dialogMessage).toContain("1");
    await archiveSection(page).getByRole("button").first().click(); // expand
    const archived = await archivedCardIds(page);
    expect(archived).toContain("f20wbo");
  });

  test("the entry is hidden on an empty column", async ({ page, board }) => {
    await openBoard(page, board);
    const menu = await openListMenu(page, "done");
    await expect(menu.getByRole("button", { name: /Archive all cards/ })).toHaveCount(0);
  });
});
