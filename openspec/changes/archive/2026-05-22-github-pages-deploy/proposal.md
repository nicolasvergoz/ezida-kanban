## Why

The landing page sources live in `site/` but GitHub Pages only auto-serves `/` or `/docs`. Without a workflow, the marketing site cannot ship from its chosen folder. A GitHub Actions deploy unblocks publishing from `site/` while keeping the repo layout clean.

## What Changes

- Add `.github/workflows/pages.yml` that builds and deploys `site/` to GitHub Pages via the official Pages actions (`configure-pages`, `upload-pages-artifact`, `deploy-pages`).
- Trigger on push to `main` when `site/**` changes, plus `workflow_dispatch` for manual redeploys.
- Workflow runs independently of `ci.yml` (no dependency on Go gate).
- Document the one-time repo setting: **Settings → Pages → Source: GitHub Actions**.

## Capabilities

### New Capabilities
- `site-deployment`: Static landing-page deployment pipeline targeting GitHub Pages from the `site/` directory.

### Modified Capabilities
<!-- none -->

## Impact

- New file: `.github/workflows/pages.yml`.
- Repo setting change required by maintainer (Pages source = GitHub Actions).
- No code changes, no runtime impact on the Go binary or skill.
- Published URL: `https://nicolasvergoz.github.io/ezida-kanban/` (matches the canonical URL already in `site/index.html`).
