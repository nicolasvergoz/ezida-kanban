import { defineConfig, devices } from "@playwright/test";

/**
 * Browser tests for the embedded viewer.
 *
 * Every test drives the real `ezida serve` binary against a throwaway
 * board, so a run covers the Go handlers, the JSON wire, the adapter,
 * and the rendering in one pass. `globalSetup` compiles the binary
 * once from the working tree — the tests always exercise the code
 * currently checked out, never a stale `./ezida`.
 *
 * Visual comparisons are gated behind PW_VISUAL=1: their baselines are
 * captured on one machine's font stack and would fail everywhere else.
 * Behaviour tests carry no such caveat and run by default.
 */
export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? "list" : [["list"], ["html", { open: "never" }]],
  // The viewer is served from loopback by a binary we start per test;
  // there is no shared baseURL.
  use: {
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  expect: {
    toHaveScreenshot: {
      // The grain overlay and the SSE status dot animate; a couple of
      // stray pixels must not fail a layout comparison.
      maxDiffPixelRatio: 0.002,
    },
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
});
