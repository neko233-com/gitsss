# Gitsss Contribution Guide

Gitsss is a self-hosted Git collaboration platform for private networks and
game repositories. This guide describes the repository's current workflow;
the root [AGENTS.md](AGENTS.md) remains the authoritative execution contract.

## Before changing code

1. Read [AGENTS.md](AGENTS.md) and the relevant document under `docs/`.
2. Run `task help` to discover the current development targets.
3. Keep the worktree focused. Do not overwrite unrelated user changes.
4. For a user-visible feature, update the relevant Markdown, `docs/site/` HTML,
   and primary locale strings in the same change.

## Development

Backend dependencies use Go modules. Frontend dependencies use pnpm and the
lockfile. pnpm 10+ reads overrides and build approvals from
`pnpm-workspace.yaml`; installs are frozen and Scarf analytics are disabled.
Use the repository targets so the local toolchain matches release validation:

```text
task fmt
task lint:go
task lint:js
task test:go
task test:js
task build
```

If the Docker daemon is unavailable, run the equivalent local Go and pnpm commands and record the limitation in the validation report; do not treat an unexecuted container target as a passed gate.

Run focused checks while iterating:

```text
go test -run '^TestName$' ./module/path/
pnpm exec vitest <path-filter>
GITEA_TEST_E2E_FLAGS='<filepath>' task test:e2e
```

The `GITEA_TEST_E2E_*` names are retained as compatibility aliases for the
existing test harness. They do not change the product name.

After any `go.mod` change, run `task tidy`. New Go files use the current year
in their copyright header. Edited files must not contain trailing whitespace.

## Compatibility and storage

Git protocol behavior, database migrations, repository paths, Git LFS,
Actions workflow directories, API contracts, configuration keys, and
environment aliases are compatibility contracts. Do not rename or remove one
silently. A required compatibility change must include:

- affected identifiers and consumers;
- migration and rollback steps;
- capacity and performance impact;
- a verification result for old and new behavior.

Storage changes are additive by default. Keep local, S3/MinIO, Azure Blob,
Git object, LFS, package, artifact, and log paths stable. For game assets,
prefer Git LFS for large binary files and verify clone, fetch, push, checkout,
range reads, and recovery before changing defaults.

## Frontend and product behavior

UI redesign and review use the project-local
[Oil UI skill](.agents/skills/oil-ui/SKILL.md), adapted to the existing template,
TypeScript/CSS, Fomantic, Vue and React entrypoints. It does not migrate frameworks.
See [installation, tools and updates](docs/features/project-ui-skill.md).

Use the existing commercial repository layout and GitHub-style Primer palette.
Buttons need visible hover, focus, active, loading, and error states. Respect
`prefers-reduced-motion` and preserve keyboard access. Drag-and-drop controls
must keep a normal form or button path for users who cannot drag.

Asset previews must fail closed: never execute repository content in the
browser. If an offline decoder is unavailable, show metadata and a raw/source
link instead of fabricating a preview.

## Pull requests and commits

Use a Conventional Commit subject, for example:

```text
feat(actions): add local runner diagnostics
fix(ui): keep repository navigation visible on narrow screens
```

Every commit includes an `Assisted-by` trailer in this form:

```text
Assisted-by: AGENT_NAME:MODEL_VERSION
```

Do not add `Co-Authored-By` or `Signed-off-by`. Do not force-push, amend, or
squash unless explicitly requested. Include authorship attribution in issue
and pull-request comments.

Never create, modify, enable, or run a workflow under `.github/workflows/`.
GitHub CI is not a release gate; run formatting, lint, tests, race checks,
asset verification, packaging, and release checks locally.

## Security

Report vulnerabilities through the private process in [SECURITY.md](SECURITY.md).
Do not publish credentials, invitation links, tokens, private repository data,
or internal target passwords in issues, commits, logs, screenshots, or test
fixtures.

## License and upstream notices

Keep the repository's `LICENSE` and `NOTICE` files intact. They document legal
attribution and compatibility provenance; they are not product branding.
