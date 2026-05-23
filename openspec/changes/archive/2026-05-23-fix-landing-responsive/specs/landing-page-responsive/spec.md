## ADDED Requirements

### Requirement: No horizontal scroll on mobile viewports
The landing page SHALL render without a horizontal scrollbar at viewport widths from 320px to 540px and 540px to 880px.

#### Scenario: 375px portrait viewport
- **WHEN** the page loads at a 375px-wide viewport
- **THEN** `document.body.scrollWidth` does not exceed `window.innerWidth`

#### Scenario: 540px viewport
- **WHEN** the page loads at a 540px-wide viewport
- **THEN** no descendant element of `<body>` overflows its container horizontally

### Requirement: Hero illustration scales down on small viewports
The `.hero-art img` element SHALL clamp to 220px or less at viewports ≤ 880px and to 180px or less at viewports ≤ 540px, regardless of any non-responsive `max-width` rule in the inline `<style>` block.

#### Scenario: 800px viewport
- **WHEN** the page loads at an 800px-wide viewport
- **THEN** the computed `max-width` of `.hero-art img` is ≤ 220px

#### Scenario: 375px viewport
- **WHEN** the page loads at a 375px-wide viewport
- **THEN** the computed `max-width` of `.hero-art img` is ≤ 180px

### Requirement: Nav header fits on small viewports
At viewport widths ≤ 540px the nav SHALL display the brand (logo + name) and the GitHub link without overflow; the `.brand-version` badge MAY be hidden to make room.

#### Scenario: 360px viewport
- **WHEN** the page loads at a 360px-wide viewport
- **THEN** `header.nav` has `scrollWidth` equal to its `clientWidth`
