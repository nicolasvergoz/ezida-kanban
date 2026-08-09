import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import path from "node:path";

export const BIN_DIR = path.join(__dirname, ".bin");
export const BIN = path.join(BIN_DIR, "ezida");

/**
 * Compile the CLI once per run. Tests launch this binary rather than
 * the repo-root `./ezida`, which is a developer's own build and is
 * routinely several schema versions behind.
 */
export default function globalSetup() {
  mkdirSync(BIN_DIR, { recursive: true });
  execFileSync("go", ["build", "-o", BIN, "./cmd/ezida"], {
    cwd: path.join(__dirname, ".."),
    stdio: "inherit",
  });
}
