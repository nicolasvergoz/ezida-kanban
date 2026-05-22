## 1. Workflow file

- [x] 1.1 Create `.github/workflows/pages.yml` with `name: pages`, triggers `push` on `main` filtered to `site/**` plus `workflow_dispatch`
- [x] 1.2 Declare workflow-scoped permissions: `contents: read`, `pages: write`, `id-token: write`
- [x] 1.3 Declare concurrency group `pages` with `cancel-in-progress: false`
- [x] 1.4 Add `build` job: checkout, `actions/configure-pages@v5`, `actions/upload-pages-artifact@v3` with `path: site`
- [x] 1.5 Add `deploy` job: `needs: build`, `environment: github-pages` (with `url`), `actions/deploy-pages@v4`

## 2. Validation

- [x] 2.1 YAML syntax check: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/pages.yml'))"`
- [x] 2.2 `openspec validate github-pages-deploy --strict` passes
