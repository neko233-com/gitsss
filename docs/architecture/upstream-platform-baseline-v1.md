# Upstream platform baseline v1

## Decision

gitsss uses a copied Gitea source snapshot as its platform baseline. This
replaces the previous plan to rebuild all repository collaboration behavior in
the small standalone `server/` API.

- Upstream tag: `v1.27.1`
- Upstream commit: `a62dfffbe7d3a454c2f2d3c4b2788ba432724a5f`
- Upstream repository: `https://github.com/go-gitea/gitea`
- License: MIT; original notices remain in `LICENSE` and `NOTICE`
- gitsss module: `github.com/neko233-com/gitsss`

The copied platform is first-party source in this repository. Production code
must not import the Gitea application module. Normal third-party dependencies
remain governed by `go.mod`, `package.json`, and their licenses.

The integrated Actions engine uses a copied Gitea Runner v1.0.8 snapshot. It lives in `runner/` as the
independent `github.com/neko233-com/gitsss/runner` module. Its original MIT
license remains in `runner/LICENSE`; upstream CI metadata and agent instructions
are excluded.

## Import boundary

The import excludes `.git`, `.github`, `.gitea`, upstream agent instructions,
and upstream product README files. In particular, no upstream CI workflow is
tracked or enabled.

The previous standalone `server/` and React `web/` implementations were
removed after integrating the three retained gitsss concepts:

1. Git storage foundation
2. Storage awareness
3. Operational dashboard

Their retained behavior now lives in platform-native models, services, routers,
templates and frontend assets. There is no second production entry point or UI.

## Compatibility rule

Database IDs, password hashes, two-factor secrets, SSH keys, access tokens,
repository paths, LFS objects, Actions data, and `app.ini` settings must retain
Gitea-compatible semantics until takeover verification passes. Schema changes
must be additive and reversible during the migration window.

## Offline rule

Runtime startup, repository use, migration, and supported Actions workflows
must not require public internet access. Build inputs and bundled Actions are
mirrored into one release image and verified by digest. The gitsss repository
itself is built and released only through local commands; it does not use
GitHub Actions.

GitSSS extensions use `gitsss_schema_version`; upstream IDs retain their upstream
meaning. See [the compatibility and rollback report](../operations/upstream-compatibility.md).
