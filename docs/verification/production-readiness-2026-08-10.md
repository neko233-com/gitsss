# Production readiness audit

Date: 2026-08-10

## Scope

- Final local branch: `main`, 16 commits ahead of `origin/main` after the
  local product upgrade.
- No `.github/workflows/` files were created, changed, enabled, or executed.
- Git protocol, repository paths, LFS paths, database schema, API contracts,
  configuration aliases, and Actions compatibility identifiers remain stable.

## Product verification

- Account invites are administrator-only, one-time, expiring, revocable, and
  consumed atomically with account creation.
- Login throttling uses progressive per-account and per-client delays without
  account lockout, so a remote attacker cannot permanently deny the owner
  access. A successful login clears the delay state.
- The local UI uses GitHub-style dark colors with the commercial repository
  layout, team/member assignment drag-and-drop, project task board placement
  beside milestones and explore, runner controls, and game-asset preview
  fallbacks.
- Swagger is branded `Gitsss`; compatibility references to Gitea identifiers
  remain only where protocol, migration, test, or upstream compatibility
  requires them.

## Windows local verification

- Built and started the Windows binary with pure-Go SQLite compatibility tags:
  `go build -tags sqlite,sqlite_unlock_notify`.
- Browser checks used the in-app Browser only. The public home page rendered
  the Gitsss heading, GitHub dark theme, installation/cross-platform/value
  sections, and Gitsss footer on desktop and 390x844 mobile layouts.
- Mobile navigation collapsed to `导航菜单`; both desktop and mobile browser
  console checks returned no warning or error entries.
- `/api/healthz`, `/api/swagger`, the authenticated Actions Worker/admin
  surfaces, runner controls, and responsive admin layout were validated during
  the same local build session. One intentionally incorrect local login was
  not retried, avoiding account-throttle noise.

## Remote data-collect validation

Read-only checks against `http://git.example.internal:3000/engineering/service-api`
passed with the provided CI account:

- health, Swagger, repository page, Smart HTTP refs, and three workflow files;
- `main` resolved to `dbdf0012eb66d7c78e645165b64cac13515bb806`;
- Git LFS 3.7.1 and an LFS batch download request returned HTTP 200;
- the Actions page rendered 71 workflow runs, including successful and
  cancelled states.

The remote deployment remains the existing Gitea 1.27.1 service. No remote
repository, branch, tag, workflow, or deployment state was modified.

## Git operation verification

An exact temporary directory was removed after the run. Full clone, status,
branch creation, empty commit, merge, annotated tag, checkout, diff check,
log, full fsck, Git LFS listing, mirror clone, local mirror push/fetch, fast
forward pull, and remote push dry-run all passed. Partial-clone mirror creation
was not used because Git promisor objects are intentionally lazy; the full
clone path is the production-safe mirror path.

## Local gates

- Go race tests passed for `./services/auth ./services/actions` with SQLite
  compatibility tags.
- Full native `go test ./...` on Windows produced no package output for five
  minutes and was stopped with its exact test process tree; the focused auth,
  Actions, LDAP, vet, and race gates passed.
- Frontend Vitest, ESLint, Vue type checking, Stylelint, and Vite production
  build passed; the frozen pnpm install passed.
- Native Go formatting/build checks passed.
- `task fmt`, `task lint:go`, and `task lint:js` could not start because the
  Docker Desktop Linux engine was unavailable; their native equivalents were
  run and recorded instead.

## Cleanup and handoff

- Only task-scoped temporary validation directories and caches are eligible for
  cleanup. User data, the repository database, dependencies, and persistent
  build outputs are not removed.
- The validation process and its temporary Windows executable are stopped and
  removed before final handoff; the port is checked to be free.
- Release publication is gated on local cross-platform artifacts, checksums,
  source archive verification, and the final GitHub push.
