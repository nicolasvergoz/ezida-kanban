import { test, expect, openBoard, card, visual, rgbOf, contrast } from "./fixtures";

const CHIP = '[data-card-id="f20wbo"] .card-epic-chip';
const CHIP_CARD = '[data-card-id="f20wbo"]';
const TAG = '[data-card-id="f20wbo"] .card-tag-chip';

/**
 * One stored hex has to stay legible on both the cream and the navy
 * ground. The chip never paints the hex as text; it mixes it toward
 * the theme's `--text`. These assert the mix actually lands somewhere
 * readable — the part a screenshot would not tell us.
 */
for (const theme of ["light", "dark"] as const) {
  test.describe(`${theme} theme`, () => {
    test.use({ theme });

    test("the epic chip's label is legible on the card", async ({ page, board }) => {
      await openBoard(page, board);

      const fg = await rgbOf(page, CHIP, "color");
      const bg = await rgbOf(page, CHIP_CARD, "backgroundColor");

      // 3:1 is the WCAG floor for non-body UI text. The chip is 11px
      // and 500 weight, so this is a floor, not a target.
      expect(contrast(fg, bg), `chip ${fg} on card ${bg}`).toBeGreaterThan(3);
    });

    test("tag chips stay neutral, so colour alone separates the two", async ({ page, board }) => {
      await openBoard(page, board);

      const tag = await rgbOf(page, TAG, "color");
      const chip = await rgbOf(page, CHIP, "color");

      expect(tag).not.toEqual(chip);
      // Grey: the three channels sit close together.
      expect(Math.max(...tag) - Math.min(...tag), `tag colour ${tag}`).toBeLessThan(24);
      // Coloured: they do not.
      expect(Math.max(...chip) - Math.min(...chip), `chip colour ${chip}`).toBeGreaterThan(24);
    });

    test("the parent's border is tinted away from the plain border", async ({ page, board }) => {
      await openBoard(page, board);

      const epic = await rgbOf(page, '[data-card-id="rl4m9x"]', "borderColor");
      const plain = await rgbOf(page, '[data-card-id="a3f2k9"]', "borderColor");

      expect(epic).not.toEqual(plain);
    });
  });
}

test("the same stored hex renders differently in each theme", async ({ browser, board }) => {
  const read = async (theme: string) => {
    const ctx = await browser.newContext();
    await ctx.addInitScript((t) => localStorage.setItem("kanban.theme", t), theme);
    const page = await ctx.newPage();
    await page.goto(board.url);
    await page.locator(".card").first().waitFor();
    const c = await rgbOf(page, CHIP, "color");
    await ctx.close();
    return c;
  };

  expect(await read("light")).not.toEqual(await read("dark"));
});

visual("the board matches its baseline", async ({ page, board }) => {
  await openBoard(page, board);
  await expect(page).toHaveScreenshot("board-light.png", { fullPage: true });
});
