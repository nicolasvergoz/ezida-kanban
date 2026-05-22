## Context

The landing-page assets sit under `site/` (HTML, CSS, JS, OG image, sitemap, llms.txt). GitHub Pages' built-in branch-source mode only serves the repo root or `/docs`. To publish from `site/` we must use the Actions-based Pages deployment. The repo already has `.github/workflows/ci.yml` for Go tests; the Pages job runs orthogonally.

## Goals / Non-Goals

**Goals:**
- Deploy `site/` to GitHub Pages on every relevant push to `main`.
- Allow manual redeploy from the Actions UI.
- Keep the workflow decoupled from `ci.yml` so a Go-test failure does not block a content fix.

**Non-Goals:**
- No build step (no SSG, no bundler). Site is hand-authored static files.
- No custom domain handling. Default `*.github.io` URL is fine and already declared as canonical in `site/index.html`.
- No PR preview deployments.

## Decisions

**Use official GitHub Pages actions over `peaceiris/actions-gh-pages`.**
The first-party actions (`actions/configure-pages@v5`, `actions/upload-pages-artifact@v3`, `actions/deploy-pages@v4`) are the documented path, require no PAT, and integrate with the repo's Pages environment. The third-party alternative needs a deploy key or PAT and pushes to a `gh-pages` branch — extra moving parts for no benefit.

**Path-filtered trigger (`paths: [site/**]`) plus `workflow_dispatch`.**
Most pushes to `main` touch Go code, not the site. Filtering on `site/**` avoids burning CI minutes on no-op deploys. `workflow_dispatch` covers force-redeploy needs (e.g., re-running after toggling the Pages setting).

**Independent of `ci.yml`.**
The landing page has no shared build artifacts with the Go binary. Coupling the two would mean a flaky Go test blocks a typo fix on the marketing site. Decoupled is safer for content velocity.

**Concurrency group `pages` with `cancel-in-progress: false`.**
Standard pattern recommended by GitHub: serialise deploys, don't cancel an in-flight one (cancelling can leave Pages in a partial state).

## Risks / Trade-offs

- **One-time manual setting required** → maintainer must flip Settings → Pages → Source to "GitHub Actions". First workflow run will fail loudly if not done. Documented in `proposal.md` and the workflow comment header.
- **Path filter could miss site-affecting changes outside `site/`** (e.g., a future shared asset). → Mitigation: `workflow_dispatch` lets us force a redeploy any time.
- **No rollback automation** → GitHub Pages keeps the last successful deploy live until the next one succeeds; a bad commit means re-push or revert. Acceptable for a static landing page.

## Migration Plan

1. Merge the workflow file to `main`.
2. Maintainer toggles repo Settings → Pages → Source = "GitHub Actions".
3. Trigger workflow manually (`workflow_dispatch`) for the first deploy.
4. Verify `https://nicolasvergoz.github.io/ezida-kanban/` serves the new site.

No rollback needed — if it fails, revert the workflow file; the prior Pages state (or none) is preserved.
