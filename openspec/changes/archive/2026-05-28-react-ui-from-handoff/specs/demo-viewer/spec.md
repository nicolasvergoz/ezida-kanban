## MODIFIED Requirements

### Requirement: Demo shares viewer assets via symlinks

The demo directory SHALL link viewer assets via symlinks so the
demo stays byte-identical to the real viewer: `site/demo/app.jsx`,
`site/demo/styles.css`, and `site/demo/vendor` MUST be symbolic
links into `internal/server/web/`. The legacy symlinks
`site/demo/app.js` and `site/demo/style.css` MUST NOT exist (the
files they pointed at no longer exist either).

#### Scenario: Asset divergence audit

- **WHEN** `site/demo/app.jsx` is inspected
- **THEN** it is a symlink whose target resolves to
  `internal/server/web/app.jsx`

- **WHEN** `site/demo/styles.css` is inspected
- **THEN** it is a symlink whose target resolves to
  `internal/server/web/styles.css`

- **WHEN** `site/demo/vendor` is inspected
- **THEN** it is a symlink whose target resolves to
  `internal/server/web/vendor`
