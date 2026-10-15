- Use `task help` to find available development targets
- Run `task fmt` to format `.go` files, and run `task lint:go` to lint them
- Run `task lint:js` to lint `.ts` files
- Run `task tidy` after any `go.mod` changes
- Run single go tests with `go test -run '^TestName$' ./modulepath/`
- Run single js test files with `pnpm exec vitest <path-filter>`
- Run single playwright e2e test files with `GITEA_TEST_E2E_FLAGS='<filepath>' task test:e2e`
- Add the current year into the copyright header of new `.go` files
- Ensure no trailing whitespace in edited files
- Use Conventional Commits for commit messages and PR titles, e.g. `type(scope): subject`; `!` before the colon if breaking. Use `test` type for test-only changes.
- Never force-push, amend, or squash unless asked. Use new commits and normal push for pull request updates
- Preserve existing code comments, do not remove or rewrite comments that are still relevant
- Keep comments short, prefer same-line, explain why, never narrate code
- Prefer unit tests over integration tests when logic is testable in isolation
- Aim for sub-2s local runtime for integration and e2e tests
- In TypeScript, use `!` (non-null assertion) instead of `?.`/`??` when a value is known to always exist
- For CSS layout, prefer `flex-*` helpers over per-child `tw-ml-*` / `tw-mr-*` margins; fall back to `tw-*` utilities when specificity requires `!important`
- Include authorship attribution in issue and pull request comments
- Always add `Assisted-By` trailers to commit messages in format `Assisted-by: AGENT_NAME:MODEL_VERSION`
- Never add `Co-Authored-By` `Signed-off-by` trailer to commit messages. Sign off must be done by a human.
- Never create, modify, enable, or run GitHub Actions or any workflow under `.github/workflows/`; this repository has no GitHub Actions budget and tracked workflows must remain absent
- Keep documentation current with product behavior. Any user-visible feature, configuration, installation, upgrade, removal, API, security, architecture, or workflow change must update the relevant Markdown and `docs/site/` HTML pages in the same change. Add new HTML pages to `docs/site/index.html` and shared navigation; update `docs/site/agents.html` when agent workflow or repository rules change.
- Do not use GitHub CI status as a release gate. Run formatting, linting, tests, race checks, asset verification, packaging, and release verification on the local machine
- Before publishing a release, cross-compile and package the supported Linux, macOS, and Windows targets locally, verify checksums and any configured signatures locally, then publish the tag and packages directly with `gh`
- Do not publish a tag, package, or GitHub Release until all required local gates pass; fix failures locally and rebuild the artifacts
- Do not publish per-feature or incremental releases. Complete the full requested product scope and requirement-by-requirement verification first, then create one consolidated tag and package release

## Compatibility baseline

- Treat Git protocol, database schema and migrations, storage paths, Git LFS, API contracts, Actions workflow directories, configuration keys, and environment aliases as compatibility contracts.
- Keep this file and the platform change surface stable. If a change requires a broad rule or platform update, document its scope, migration and rollback path, and compatibility impact first.
- Preserve legacy identifiers and paths unless an explicit migration is part of the request; do not silently rename internal compatibility names.

## Storage stability freeze

- Do not redesign Git, LFS, local/S3/MinIO storage, database paths, or Actions artifact/log storage without a compatibility report covering migration, rollback, capacity and performance impact.
- Additive storage drivers and observability are allowed when existing paths, providers, keys and APIs remain unchanged; validate both CGO and pure-Go SQLite builds locally.
- Git layout contract v1 is `data/gitsss/layout.json`. Upgrades may add `reserved` keys and files under `data/gitsss/extensions/<id>/` only. They must not move, rename, repack, or rewrite bare repositories, Git objects, refs, or LFS object bytes. A binary that does not understand `layout_version` must stop without writing those trees. Rollback is the previous binary; Git and LFS data need no migration.
