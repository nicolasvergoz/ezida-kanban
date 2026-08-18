## 1. Viewer — refresh state and control

- [x] 1.1 Add `IconRefresh` one-liner beside the other icons in
      `internal/server/web/app.jsx` (around lines 216-227), using the
      Feather refresh path over the shared `Icon` component.
- [x] 1.2 In `App`, add a `refreshing` state plus a `refreshingRef`
      guard and a `refreshBoard` callback that calls the existing
      `fetchBoard`, sets `refreshing` true while in flight, and clears
      it in a `finally` (see design D1).
- [x] 1.3 Pass `onRefresh={refreshBoard}` and `refreshing={refreshing}`
      into `TopBar` at its call site.
- [x] 1.4 In `TopBar`, add the refresh `iconbtn` after `ServerStatus`
      with `aria-label="Refresh"`, `title="Refresh"`, `disabled` when
      `refreshing`, and a `refreshing` class hook; wire `onClick` to
      `onRefresh` (see design D2).

## 2. Viewer — busy-state styling

- [x] 2.1 In `internal/server/web/styles.css`, add `.iconbtn svg`
      spin behaviour: a `@keyframes spin` rule and
      `.iconbtn.refreshing svg { animation: spin 0.9s linear infinite; }`,
      using design tokens only (no hex outside `:root`).
- [x] 2.2 Confirm the resting refresh button matches the existing
      `iconbtn` styling (same label, size, spacing) with no visual
      regression on a board without the feature in play.

## 3. Browser test

- [x] 3.1 Add `e2e/reload.spec.ts` with `test.use({ fixture:
      "plain.toml" })`. Open the board, register `page.on("request")`,
      record `GET /api/board` count, click the `Refresh` button, and
      assert the count incremented by exactly one (design D5; poll
      with a short timeout).
- [x] 3.2 In the same spec, assert the button is present and enabled
      when the SSE connection is offline: close the `EventSource`
      via `page.evaluate`, and confirm the `Refresh` button remains
      visible and enabled.

## 4. Verify

- [x] 4.1 Run `./scripts/verify.sh` (Go gate + browser tests) and
      confirm everything passes, including the new reload spec.
- [x] 4.2 Confirm no viewer-viewer regression: `plain.toml` board
      renders pixel-identically to the pre-feature baseline other than
      the new topbar control (no conditional rendering regressions).
