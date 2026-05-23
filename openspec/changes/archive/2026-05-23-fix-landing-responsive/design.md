## Context

The landing page styles live in `site/assets/site.css` with two breakpoints (`@media (max-width: 880px)` and `@media (max-width: 540px)`). An inline `<style>` block in `site/index.html` was added to lock the Tablet variation's typographic choices and hero-art sizing. That block declares `.v-tablet .hero-art img { max-width: 440px; }` without any media query. Because it follows `site.css` in source order with identical specificity, it overrides the responsive `max-width: 220px` in `site.css`, leaving the hero icon at 440px on mobile and forcing horizontal scroll.

## Goals / Non-Goals

**Goals:**
- No horizontal scroll at viewport widths 320–540px and 540–880px.
- Hero illustration scales down on mobile (≤ 220px at ≤ 880px, ≤ 180px at ≤ 540px).
- Nav header content (brand, version, GitHub link) fits on a 320px viewport.

**Non-Goals:**
- Visual redesign or restructuring of sections.
- Touching JS, deployment workflow, or Go code.
- Changing the desktop layout above 880px.

## Decisions

1. **Add responsive overrides inside the inline `<style>` block** rather than relying on `site.css`, because the inline block already governs `.v-tablet .hero-art img` and bumping specificity in `site.css` would require `!important` (which we want to avoid). Source-order wins are predictable and self-contained.
2. **Two breakpoints reuse the existing 880 / 540 thresholds** to stay consistent with the rest of the page; no new breakpoints introduced.
3. **`.brand-version` hidden at ≤ 540px** because the version is informational and the brand + nav-gh need the room.
4. **Hero vertical padding reduced at ≤ 540px** to keep the headline closer to the eyebrow on small screens.

## Risks / Trade-offs

- Hiding `.brand-version` on small screens means users on mobile won't see the version chip in the nav. Acceptable: the version is also present in the hero eyebrow and the footer.
- Inline `<style>` block grows by ~10 lines; acceptable for the scope of this fix.
