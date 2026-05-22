## ADDED Requirements

### Requirement: Pages workflow deploys `site/` directory
The repository SHALL provide a GitHub Actions workflow that publishes the contents of `site/` to GitHub Pages using the official first-party Pages actions (`actions/configure-pages`, `actions/upload-pages-artifact`, `actions/deploy-pages`).

#### Scenario: Push to main touching site/
- **WHEN** a commit on `main` modifies any file under `site/**`
- **THEN** the workflow runs and deploys the new contents to GitHub Pages

#### Scenario: Push to main not touching site/
- **WHEN** a commit on `main` modifies only files outside `site/**`
- **THEN** the Pages workflow does not run

#### Scenario: Manual redeploy
- **WHEN** a maintainer triggers the workflow via `workflow_dispatch`
- **THEN** the current `main` contents of `site/` are deployed regardless of recent changes

### Requirement: Workflow runs independently of the Go test gate
The Pages workflow SHALL NOT depend on the `ci` workflow's success. A failing Go test MUST NOT block a site deployment, and a failing Pages deploy MUST NOT block code merges.

#### Scenario: Go tests broken, site change pushed
- **WHEN** `ci.gate` is failing and a commit modifies `site/index.html`
- **THEN** the Pages workflow still executes and deploys the change

### Requirement: Workflow uses least-privilege permissions and safe concurrency
The Pages workflow SHALL declare only the permissions required by the official Pages actions (`contents: read`, `pages: write`, `id-token: write`) and SHALL serialise deploys via a concurrency group named `pages` with `cancel-in-progress: false`.

#### Scenario: Two deploys triggered close together
- **WHEN** a second deploy is queued while one is in progress
- **THEN** the in-progress deploy completes uninterrupted and the queued run starts after it

#### Scenario: Permissions check
- **WHEN** the workflow file is inspected
- **THEN** it grants exactly `contents: read`, `pages: write`, `id-token: write` at the job (or workflow) scope and no other permissions
