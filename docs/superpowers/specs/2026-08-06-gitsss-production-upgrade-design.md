# gitsss production upgrade — design

Date: 2026-08-06
Status: approved by user

## 1. Goal

Turn the gitsss platform (a Gitea-derived fork, module `github.com/neko233-com/gitsss`)
into a self-contained production product with:

- a fully built-in, visually managed Actions runner (no separate runner installation);
- a GitHub-team style visual theme with Gitea-Pro-style left sidebar layout;
- a 任务看板 (Task Board) kanban with drag-drop member assignment;
- drag-drop team/member management in the admin/org surfaces;
- T0 game repository support with preview of common game asset formats;
- a hardened, documented, stable storage foundation with a compatibility freeze;
- modern API documentation; full local verification plus handoff scripts for the
  live intranet server.

## 2. Non-goals / frozen contracts

- No GitHub Actions workflow files are created, modified, or run under
  `.github/workflows/` (no CI budget; tracked workflows must remain absent).
- Internal identifiers remain Gitea-compatible (config keys, `gitea.db`,
  module paths, `/projects` URLs, API contracts, storage paths, LFS) so
  upstream behavior and compatibility contracts stay intact.
- No storage re-architecture; the existing pluggable storage abstraction
  (local/S3/MinIO) + LFS is hardened, verified, documented, and frozen.
- No full codebase rename of internal gitea identifiers (product-facing
  rebrand only).
- The task board reuses the existing Projects model/DB/API; no new DB schema.
- Live verification against `http://192.168.110.42:3000` is impossible from
  this machine (network unreachable); deliver verified handoff scripts instead.

## 3. Workstreams

### WS0 — Built-in in-process Actions runner (priority: highest)

Problem today: users must register and run a separate `gitsss-runner` binary
(CLI registration, config files). The Docker image hides this behind s6 scripts,
but bare-metal deployments get nothing and there is no visual management.

Solution: run the runner poller inside the gitsss server process.

1. New exported package in the runner module: `runner/pkg/builtin`.
   It wraps `run.NewRunner`, `poll.New`, and the task loop from
   `runner/internal/app/cmd/daemon.go` (client, declare, poll, supervise)
   so the main server binary can call it as a library.
2. New server service `services/actions/builtin_runner.go`:
   - On web start, when Actions is enabled and
     `[actions] BUILTIN_RUNNER_ENABLED = true` (default), auto-provision a
     global runner `gitsss-builtin` directly in the DB (model-level, no HTTP
     round-trip), then start the in-process poller against the server's own
     Actions API (localhost).
   - Supervise: restart on panic/exit, graceful shutdown with server.
   - Persist settings: enabled flag, capacity, labels.
3. Visual admin management (replaces CLI for the common case):
   - Admin → Actions → built-in runner panel: status (online/offline),
     version, labels, capacity, job counters, Start / Stop / Restart buttons.
   - The runner appears in the existing runner list with live status.
   - Org/repo runner registration remains supported for extra capacity.
4. Labels default: `self-hosted:host, ubuntu-latest:host, ubuntu-24.04:host`
   (host mode, no Docker required; Docker-backed jobs work when a socket is
   available, e.g. the compose deployment).
5. Docker s6 actions-runner service: keep as fallback only; in-process runner
   is the default path and requires no user action.
6. Config key: `[actions] BUILTIN_RUNNER_ENABLED` (default true) and
   `[actions] BUILTIN_RUNNER_CAPACITY` (default 2). Documented in
   `docs/site/configuration.html` and `actions.html`.

Acceptance: fresh `gitsss web` start on a host without any runner binary shows
one online built-in runner in the admin UI, and a pushed workflow runs to
completion without any manual registration.

### WS1 — Product-facing de-Gitea rebrand (verify + finish)

Working tree already contains most rebranding (themes, webtheme marker,
APP_NAME, docs). Remaining: sweep user-visible `gitea` strings in footer,
install page, admin panel, swagger page title, package content templates,
locale files, CLI help text. Internal identifiers stay untouched.

### WS2 — GitHub-team theme + interaction polish

Working tree contains `theme-gitsss-github-light.css`, `theme-gitsss-github-dark.css`,
`theme-gitsss-light.css`, and `gitsss-polish.css`. Verify:

- theme meta parsing (`gitsss-theme-meta-info` marker with legacy fallback);
- theme picker lists all themes; default `gitsss-github-dark` in app.ini and
  `gitsss-github-light` as the code default;
- colorblind variants still work (they derive from gitea base themes);
- extend polish to admin/org/repo surfaces: button press feedback, focus
  rings, hover states, subtle animations, loading states; dark-theme contrast.

### WS3 — Gitea-Pro-style left sidebar layout

- Repo pages: replace top tab bar with a collapsible left sidebar
  (Code, Issues, Pulls, Actions, Packages, 任务看板, Releases, Wiki,
  Activity, Settings).
- Admin panel: restyle existing left vertical menu to Pro look.
- Mobile: sidebar collapses to a slide-in drawer; active item highlighted.
- Files: `templates/repo/header.tmpl`, admin `navbar.tmpl`, new CSS + small JS
  (`web_src/js/features/` + `web_src/css/features/`).
- Keep the top repository identity header (owner/repo name, actions row).

### WS4 — 任务看板 (repurpose Projects kanban)

- Rename the repo nav "Projects" tab to 任务看板 / Task Board in zh/en
  locales; URL and API remain `/projects`.
- Wire drag-drop assignee chips onto board cards (project-assignees.ts already
  exists; verify and finish wiring in `templates/projects/view.tmpl`).
- Add per-column WIP limits (new optional field, pure UI filter via CSS class
  on the column; no DB change).
- Member avatars on cards; dark-theme column styling.

### WS5 — Admin + org team/member drag-drop management

- Org team member assignment: drag member rows from an available-members pool
  onto a team member list (reuses `web_src/js/features/org-team.ts` + new
  small feature module; posts to existing team member API).
- Team ↔ repo access grant picker stays functional.
- Admin dashboard: verify existing storage capacity viz, account invites,
  login throttle; polish tables/buttons.

### WS6 — Game asset preview (T0)

Current state: 3D viewer plugin (online-3d-viewer) already covers glTF/glb/obj/
stl/fbx/ply/etc. via `external.go` patterns; PDF and OpenAPI plugins exist.

Additions:
- Audio player plugin (wav/ogg/mp3/flac) — new frontend render plugin.
- Texture formats (tga/dds/ktx2) preview via canvas rendering or metadata +
  download affordance; verify common image formats (png/jpg/webp/avif/gif).
- Textual game formats (Unity `.prefab/.unity/.asset`, Godot `.tscn/.tres`,
  shader files) render via the existing text renderer — verify.
- Spritesheets (aseprite `.aseprite`, sprite atlases) — image/raw view.
- Verify preview works for both Git-backed and LFS-backed files.

### WS7 — Storage hardening + docs + AGENTS.md freeze

- Review and complete `docs/site/storage.html` (already drafted, untracked);
  link it into `docs/site/index.html` navigation.
- Verify storage abstraction (local/S3/MinIO), LFS paths, pack/index
  acceleration settings; document extensibility points.
- Add AGENTS.md "Storage stability freeze": no large-scale storage layer
  changes without a compatibility report; record allowed change scope.

### WS8 — API documentation modernization

- `swagger-ui-dist` 5.32.1 is already bundled; `swagger.v1.json` generation
  works. Polish `/swagger` page (`templates/swagger/openapi-viewer.tmpl`,
  `web_src/css/swagger-standalone.css`) to the new theme.
- Verify in-repo OpenAPI file rendering (frontend-openapi-swagger plugin).
- Keep fully offline (no CDN).

### WS9 — Verification gates + live-server handoff

- Run: `task fmt`, `task lint:go`, `task lint:js`, `task test:go`,
  `task test:js`, targeted playwright e2e, full `task build`.
- Local CI flow: push a workflow to a local repo and prove it executes through
  the in-process runner (Go toolchain embedded for go-based actions).
- Handoff scripts `scripts/verify-remote.ps1` and `scripts/verify-remote.sh`:
  smoke-test health, repo page, clone/push, workflow dispatch against
  `192.168.110.42:3000` using the robot_cicd account; ready to run when the
  network is open.

### WS10 — Docs + commit strategy

- Every user-visible change updates the relevant `docs/site/` page, README,
  and locale files in the same commit.
- Conventional Commits; `Assisted-by: AGENT_NAME:MODEL_VERSION` trailers.
- The existing large uncommitted WIP diff is committed first as logical base
  commits (themes, admin features, docs) so the tree stays reviewable.

## 4. Compatibility

- All DB migrations unchanged; no schema additions (task board reuses
  Projects; WIP limits are UI-only).
- No config key removed; new keys are additive (`[actions]
  BUILTIN_RUNNER_ENABLED`, `BUILTIN_RUNNER_CAPACITY`).
- API contracts unchanged; the built-in runner uses the existing Actions API.
- Storage paths, LFS, Git protocol untouched.
- The runner module remains importable by the main module (already a replace
  dependency); `runner/pkg/builtin` is additive.

## 5. Risks

- In-process runner shares the server process; host-mode steps run as child
  OS processes and a panic is contained and restarted by the supervisor.
  Docker-backed jobs are unaffected (external daemon).
- Sidebar layout regression risk: all repo pages share `header.tmpl`; e2e
  tests must cover repo navigation.
- Theme contrast issues in dark mode: manual pass over admin/org pages.
