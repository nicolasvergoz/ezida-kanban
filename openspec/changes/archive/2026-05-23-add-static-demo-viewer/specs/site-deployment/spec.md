## MODIFIED Requirements

### Requirement: Pages workflow deploys `site/` directory
The repository SHALL provide a GitHub Actions workflow that publishes the contents of `site/` to GitHub Pages using the official first-party Pages actions (`actions/configure-pages`, `actions/upload-pages-artifact`, `actions/deploy-pages`). The workflow SHALL also regenerate `site/demo/board.json` from the current `kanban.toml` and substitute the snapshot SHA into the demo banner before uploading the artifact.

#### Scenario: Push to main touching site/
- **WHEN** a commit on `main` modifies any file under `site/**`
- **THEN** the workflow runs and deploys the new contents to GitHub Pages

#### Scenario: Push to main touching only kanban.toml
- **WHEN** a commit on `main` modifies only `kanban.toml`
- **THEN** the workflow runs, regenerates `site/demo/board.json` from the new `kanban.toml`, and deploys

#### Scenario: Push to main not touching site/ or kanban.toml
- **WHEN** a commit on `main` modifies only files outside `site/**` and `kanban.toml`
- **THEN** the Pages workflow does not run

#### Scenario: Manual redeploy
- **WHEN** a maintainer triggers the workflow via `workflow_dispatch`
- **THEN** the current `main` contents of `site/` are deployed regardless of recent changes, with `board.json` freshly generated

### Requirement: Snapshot SHA substitution
The workflow SHALL replace the literal placeholder `__SNAPSHOT_SHA__` inside `site/demo/index.html` with the first 7 characters of `$GITHUB_SHA` before upload. The build SHALL fail if the placeholder still appears in the artifact.

#### Scenario: Placeholder substituted
- **WHEN** the workflow's "substitute snapshot SHA" step runs
- **THEN** the resulting `site/demo/index.html` contains no occurrence of `__SNAPSHOT_SHA__`
