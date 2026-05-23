## Why

The landing page at `site/index.html` overflows horizontally on mobile viewports. The hero illustration (`.hero-art img`) is forced to 440px by an inline `<style>` rule in `<head>` that wins over the responsive `max-width: 220px` declared in `site/assets/site.css` (same specificity, later source). On a 375px-wide viewport, the page scrollWidth reaches 486px; users see a horizontal scrollbar and a clipped icon. The `.brand-version` badge in the nav also tightens the header on small screens.

## What Changes

- Add responsive `max-width` overrides for `.v-tablet .hero-art img` inside the inline `<style>` block so the image clamps to 220px ≤ 880px viewport and 180px ≤ 540px viewport.
- Add `@media (max-width: 540px)` rule hiding `.brand-version` so the brand + GitHub link fit cleanly.
- Tighten hero vertical padding on small viewports.
- No content/markup changes beyond the inline `<style>` block in `index.html`.

## Capabilities

### New Capabilities
- `landing-page-responsive`: requirements for the public landing page's behaviour at common mobile and tablet viewports.

### Modified Capabilities
<!-- none -->

## Impact

- `site/index.html` — inline `<style>` block only.
- `site/assets/site.css` — optional reinforcement.
- No changes to JS, deployment, or Go code.
