## 1. CSS overrides in inline `<style>` of `site/index.html`

- [x] 1.1 Add `@media (max-width: 880px)` rule setting `.v-tablet .hero-art img { max-width: 220px; }` inside the existing inline `<style>` block.
- [x] 1.2 Add `@media (max-width: 540px)` rule setting `.v-tablet .hero-art img { max-width: 180px; }` and `.brand-version { display: none; }`.
- [x] 1.3 Reduce hero vertical padding at ≤ 540px (`.hero { padding-top: 40px; padding-bottom: 32px; }`).

## 2. Verification at mobile widths

- [x] 2.1 Serve `site/` locally and load in Chrome.
- [x] 2.2 With media-query simulation forcing the 880/540 rules on at constrained body width 375px, confirm `document.body.scrollWidth <= 375` and computed `max-width` of `.hero-art img` ≤ 180px.
- [x] 2.3 Confirm `header.nav` does not horizontally overflow (`scrollWidth === clientWidth`).
