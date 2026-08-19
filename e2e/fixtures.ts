import { test as base, expect, type Page } from "@playwright/test";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { copyFileSync, existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { BIN } from "./global-setup";

const FIXTURES = path.join(__dirname, "fixtures");

export type Board = {
  /** Base URL of the running viewer, e.g. http://127.0.0.1:7801 */
  url: string;
  /** Directory holding the throwaway kanban.toml. */
  dir: string;
  /** Current on-disk board file, for asserting what a mutation wrote. */
  read(): string;
  /**
   * Current on-disk archive file, or null when it does not exist —
   * either because the fixture had no `.archive.toml` sibling, or
   * because the last archived card has since been restored. A spec
   * asserting "the archive is gone" checks for null, not empty text.
   */
  readArchive(): string | null;
};

/**
 * Boot `ezida serve` on a throwaway copy of a fixture board.
 *
 * The port comes from the server's own startup banner rather than from
 * a number we picked: `ezida` falls back across a window of eleven
 * ports when one is busy, and with parallel workers that fallback is
 * the normal case, not the edge case.
 */
async function startServer(fixture: string): Promise<{ board: Board; stop: () => void }> {
  const dir = mkdtempSync(path.join(tmpdir(), "ezida-e2e-"));
  const boardPath = path.join(dir, "kanban.toml");
  copyFileSync(path.join(FIXTURES, fixture), boardPath);

  const archivePath = path.join(dir, "kanban.archive.toml");
  const archiveSrc = path.join(FIXTURES, fixture.replace(/\.toml$/, ".archive.toml"));
  if (existsSync(archiveSrc)) copyFileSync(archiveSrc, archivePath);

  const proc: ChildProcessWithoutNullStreams = spawn(
    BIN,
    ["serve", "--port", String(7800 + Math.floor(Math.random() * 150)), "--no-open"],
    { cwd: dir },
  );

  const url = await new Promise<string>((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error(`ezida serve did not report a URL within 10s\n${stderr}`)),
      10_000,
    );
    let stderr = "";
    proc.stderr.on("data", (d) => (stderr += String(d)));
    proc.stdout.on("data", (d) => {
      const m = /http:\/\/127\.0\.0\.1:\d+/.exec(String(d));
      if (m) {
        clearTimeout(timer);
        resolve(m[0]);
      }
    });
    proc.on("exit", (code) => {
      clearTimeout(timer);
      reject(new Error(`ezida serve exited with ${code}\n${stderr}`));
    });
  });

  // The banner prints on bind, which is a moment before the handler is
  // reachable. Poll until the board actually answers.
  for (let i = 0; i < 50; i++) {
    try {
      const r = await fetch(`${url}/api/board`);
      if (r.ok) break;
    } catch {
      /* not up yet */
    }
    await new Promise((r) => setTimeout(r, 100));
  }

  return {
    board: {
      url,
      dir,
      read: () => readFileSync(boardPath, "utf8"),
      readArchive: () => (existsSync(archivePath) ? readFileSync(archivePath, "utf8") : null),
    },
    stop: () => {
      proc.kill("SIGTERM");
      rmSync(dir, { recursive: true, force: true });
    },
  };
}

type Fixtures = {
  /** Board fixture file to serve. Override per file or per test. */
  fixture: string;
  /** The running viewer. */
  board: Board;
  /** "light" | "dark" — applied before the app boots. */
  theme: "light" | "dark";
};

export const test = base.extend<Fixtures>({
  fixture: ["epics.toml", { option: true }],
  theme: ["light", { option: true }],

  board: async ({ fixture }, use) => {
    const { board, stop } = await startServer(fixture);
    await use(board);
    stop();
  },

  page: async ({ page, theme }, use) => {
    // Pin the theme before the app reads localStorage, so a run does
    // not inherit the machine's OS appearance.
    await page.addInitScript((t) => localStorage.setItem("kanban.theme", t), theme);

    // An uncaught exception fails the test that caused it. The viewer
    // resolves relations client-side, so a bad payload surfaces here
    // rather than as a quietly missing chip.
    const errors: string[] = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    page.on("console", (m) => {
      // Chromium logs its own line for every failed request, whoever
      // made it. A refused mutation is something the viewer displays,
      // so that line says nothing about the page's own behaviour —
      // unlike a console.error, which only the app can emit.
      if (m.type() === "error" && !m.text().startsWith("Failed to load resource:")) {
        errors.push(m.text());
      }
    });

    await use(page);

    expect(errors, "the page logged errors").toEqual([]);
  },
});

export { expect };

/** Open the board and wait for the first card to render. */
export async function openBoard(page: Page, board: Board) {
  await page.goto(board.url);
  await page.locator(".card").first().waitFor();
}

/** Locator for a card by its six-character id. */
export function card(page: Page, id: string) {
  return page.locator(`[data-card-id="${id}"]`);
}

/**
 * Ids of every card currently rendered, in DOM order — live AND
 * archived (an expanded Archive section renders `.card` elements
 * too). Kept for existing callers; `liveCardIds` / `archivedCardIds`
 * below are the scoped alternatives for specs that care which is
 * which.
 */
export async function visibleCardIds(page: Page): Promise<string[]> {
  return page.locator(".card").evaluateAll((els) =>
    els.map((e) => (e as HTMLElement).dataset.cardId ?? ""),
  );
}

/** Ids of cards rendered inside a real column, in DOM order. */
export async function liveCardIds(page: Page): Promise<string[]> {
  return page.locator(".list[data-column] .card").evaluateAll((els) =>
    els.map((e) => (e as HTMLElement).dataset.cardId ?? ""),
  );
}

/** Ids of cards rendered inside the expanded Archive section. */
export async function archivedCardIds(page: Page): Promise<string[]> {
  return page.locator("[data-archive] .card").evaluateAll((els) =>
    els.map((e) => (e as HTMLElement).dataset.cardId ?? ""),
  );
}

/** Open the filter popover and return it. */
export async function openFilter(page: Page) {
  await page.getByRole("button", { name: "Filter" }).click();
  const pop = page.locator(".filter-popover");
  await pop.waitFor();
  return pop;
}

/**
 * Visual comparisons need committed baselines captured on one font
 * stack, so they are opt-in via PW_VISUAL=1. Declared as `test.skip`
 * rather than gated with a top-level `skip()` call, which would apply
 * to every file that imports this module.
 */
export const visual = process.env.PW_VISUAL ? test : test.skip;

/**
 * Resolve a computed colour to sRGB bytes.
 *
 * `getComputedStyle` hands back whatever form the cascade produced —
 * `color-mix(in oklch, …)` resolves to `oklab(…)`, not `rgb(…)` — so
 * the value is painted to a canvas and read back rather than parsed.
 */
export async function rgbOf(
  page: Page,
  selector: string,
  prop: "color" | "backgroundColor" | "borderColor",
): Promise<[number, number, number]> {
  return page.evaluate(
    ([sel, p]) => {
      const el = document.querySelector(sel as string);
      if (!el) throw new Error(`no element matches ${sel}`);
      const value = getComputedStyle(el)[p as any] as string;
      const ctx = document.createElement("canvas").getContext("2d")!;
      ctx.fillStyle = value;
      ctx.fillRect(0, 0, 1, 1);
      const d = ctx.getImageData(0, 0, 1, 1).data;
      return [d[0], d[1], d[2]] as [number, number, number];
    },
    [selector, prop] as const,
  );
}

/** WCAG relative-luminance contrast ratio between two sRGB triples. */
export function contrast(a: [number, number, number], b: [number, number, number]): number {
  const lum = (c: [number, number, number]) => {
    const [r, g, bl] = c.map((v) => {
      const s = v / 255;
      return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
  };
  const [hi, lo] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (hi + 0.05) / (lo + 0.05);
}
