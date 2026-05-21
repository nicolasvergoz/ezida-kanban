## Why

The viewer ships V1-V4 with utilitarian styling (rounded boxes, system
fonts, single-shade chrome) — readable, but visually flat and off-brand
relative to the Redacto editorial design system documented in
`refs/design.md`. Phase UI-1 of the redesign batch (see
[ADR §D14](../../decisions/0003-ui-redesign-batch.md)) lands the
*look-and-feel foundation*: design tokens, vendored typography, and a
full visual rebrand of the existing read-only surface. No new
behavior, no new endpoints (modulo a single string field) — every
subsequent phase consumes the tokens this phase introduces.

Doing the rebrand as a standalone phase keeps the diff focused on CSS
+ asset vendoring + one server field, isolates visual-regression risk
from behavior changes, and lets the look-and-feel batch ship as a
presentable v1 even if interactive phases (UI-4 / UI-5 / UI-6) are
deferred.

## What Changes

- Vendor Geist (300-800) and Geist Mono (400/500) `.woff2` files under
  `internal/server/web/vendor/fonts/`, embedded via `//go:embed` and
  served through the existing `/static/*` route.
- Introduce a CSS custom-property token system on `:root` in
  `style.css` covering surfaces, borders, text ramp, accent, danger,
  spacing scale (2/4/6/8/10/12/14/18/28), rounded scale
  (xs/sm/md/lg/xl/full), and elevation (shadow ramps for card idle /
  hover / popover). Light theme only — dark overrides land in
  [add-dark-theme](../add-dark-theme/).
- Introduce typography utility classes (`.t-brand`, `.t-list-title`,
  `.t-card-text`, `.t-body-md`, `.t-button`, `.t-tag`,
  `.t-mono-counter`, `.t-mono-label`) keyed to the type ramp in
  [`refs/design.md`](../../../refs/design.md).
- Re-skin the topbar: brand renders the project name (left-aligned,
  Redacto brand typography, uppercase), status dot kept on the right
  and skinned per Redacto. No filter button, no theme toggle (those
  ship in UI-3 / UI-2 respectively).
- Re-skin columns as 296px glass panels (75% surface alpha + 14px
  blur + 140% saturate), cards as 10px-rounded paper with idle + hover
  shadow ramps and a -1px hover lift.
- Re-skin tag chips, priority styles, loading state, and the empty-
  column placeholder per Redacto. Tag chips remain readonly here —
  inline tag editing is UI-5.
- Re-skin the edit modal to Redacto (paper surface, rounded corners,
  floating popover shadow). Form fields stay V3-style; Trello-style
  click-to-edit is UI-5.
- Add page-level surfaces: warm-white `bg-base` background, SVG grain
  overlay at `opacity: 0.04`, top-shade gradient (10% bg-base × 64px,
  fades to transparent).
- Add a drag-scroll affordance to the empty board surface: pointer-
  down on `.board` (not on a card, header, button, or composer)
  enables horizontal drag-scroll until pointer-up.
- **Server**: `/api/board` response gains a top-level string
  `project_name`, computed once at server start as
  `filepath.Base(filepath.Dir(<resolved boardPath>))` with fallback
  `"Ezida"` when the basename is empty or `"."` (see
  [ADR §D4](../../decisions/0003-ui-redesign-batch.md)). No other
  fields change.

## Capabilities

### New Capabilities

- _(none — this phase modifies existing capabilities only)_

### Modified Capabilities

- `viewer-ui`: visual rebrand (token system, surfaces, typography
  utilities, restyled card / tag / modal / status-dot / priority
  treatments, drag-scroll affordance, project-name binding in the
  topbar brand).
- `viewer-server`: `/api/board` payload gains a `project_name` string
  field computed at server start.

## Impact

- **Code touched**:
  - `internal/server/web/index.html` — topbar markup (brand binds to
    `project_name`), apply typography utility classes, restructured
    column / card markup, modal markup tweaks (no behavior change),
    drag-scroll handler attach point.
  - `internal/server/web/app.js` — store `project_name` on
    `load()`, drag-scroll pointer handlers on `.board`.
  - `internal/server/web/style.css` — replaced wholesale with
    token-driven Redacto styles.
  - `internal/server/web/vendor/fonts/` — new directory, vendored
    `.woff2` assets.
  - `internal/server/handlers.go` — `boardResponse` gains
    `ProjectName string \`json:"project_name"\``;
    `handleBoard` reads the field from `serverState`.
  - `internal/server/server.go` — compute `projectName` from the
    resolved board path at boot and store it on `serverState`.
  - `internal/server/server_test.go` (and adjacent tests) — assert
    the new `project_name` field; happy path + fallback case.
- **Assets size**: Geist + Geist Mono `.woff2` adds ~120 KB to the
  embedded payload (ADR 0003 consequences — acceptable).
- **APIs / contracts**: `/api/board` adds one field; existing fields
  unchanged. No new endpoints, no new error codes.
- **Dependencies**: none (fonts vendored locally; no toolchain added,
  per [ADR §D2](../../decisions/0003-ui-redesign-batch.md)).
- **Tests**: existing `server_test.go` snapshot of `/api/board`
  payload needs a fresh field; UI changes are visual-only and gated
  by an automated Chrome MCP smoke at the end of the task list.
