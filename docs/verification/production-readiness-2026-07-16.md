# Production readiness audit

Date: 2026-07-16

## Product baseline

- Gitea v1.26.2 source was copied into this repository at upstream commit
  `2c749ce548b1da0a52835861a87b888e67842d29`.
- The application module is `github.com/neko233-com/gitsss`; production code
  does not import the Gitea application module or SDK.
- Gitea repository, account, issue, pull request, project, package, release,
  Actions, administration, and API behavior remain the platform baseline.
- The only public executable is `gitsss`.

## gitsss extensions

- The git storage foundation, storage awareness/write admission, and
  operational dashboard are integrated into the platform-native service and
  administration UI.
- The gitsss Light skin renders over the Gitea-compatible page layout.
- Feishu webhook delivery supports signed requests and bounded,
  administrator-defined templates. Existing DingTalk, Slack, Discord,
  Telegram, Microsoft Teams, Matrix, and WeChat Work integrations remain.
- GitLab conventions include `.gitlab/CODEOWNERS`, issue templates, merge
  request templates, approval rules, protected branches, scoped labels,
  artifacts, and merge-after-checks behavior.

## Offline operation

- One Ubuntu 24.04 image contains the server, Actions runner, supported
  Actions, Git LFS, Node.js, pnpm, Go, and build tools.
- Supported Actions resolve from `/app/gitsss/builtin-actions`; unknown remote
  Actions fail closed.
- Update checking is disabled by default and has no external endpoint.
- Compose uses one image, one data volume, default HTTP port 3000, a health
  check, `no-new-privileges`, and a two-minute graceful-stop window.
- Shell and PowerShell installers load an offline image archive, never pull
  implicitly, and support changing the host HTTP port while retaining the same
  data volume and internal port 3000.
- PowerShell installation started on host port 3001, returned HTTP health 200,
  then recreated on port 3000 with the same named volume. A persisted marker
  remained intact after the switch.

## HTTP migration

- Migration uses source API and Git HTTP reads only; source SSH, filesystem,
  Docker volume, and database access are not used.
- Dry-run is the default. Apply requires a target token and an exclusive mode
  `0600` JSONL temporary-credential file.
- The disposable Gitea 1.26.2 rehearsal migrated four accounts, organization
  ownership, custom-team and direct RBAC, the two requested repository names,
  five refs, and one 16 KiB LFS object.
- Source and target Git object IDs matched. Imported accounts were active,
  preserved administrator/restricted/login state, and required password
  change.
- Live `192.168.110.42:3000` was not accessed or changed.

## Platform verification

- Full backend test suite passed in Ubuntu 24.04 with Git 2.43.
- Go formatting, race tests, vet, and lint (`0 issues`) passed.
- Frontend lint, Vue type checking, 35 Vitest files, and 92 tests passed.
- Markdown lint passed for root and verification/operations documentation.
- Final `linux/amd64` image:
  `sha256:c0669bd180e9fead3c1760c916e06b0a38cf1bdf2c82e7ec8e11aecb8ebe9d42`.
- Final-source `linux/arm64` image built and ran under emulation; HTTP health
  returned 200 and the embedded runner registered as arm64/Linux.
- The in-app browser verified public and authenticated UI, gitsss branding and
  skin, Gitea-compatible navigation/dashboard layout, and an empty browser
  warning/error console.

## Cleanup and handoff

- All gitsss test containers, volumes, networks, source images, gate images,
  and the temporary arm64 tag were removed.
- Production installation defaults to the single image repository
  `gitsss/gitsss:latest`; formal releases also carry their immutable version
  tag in the same repository.
- Mac mini M4 offline transfer, startup, migration rehearsal, cutover, backup,
  and rollback commands are documented under `docs/operations/`.

## Isolated Actions publishing and storage cleanup

- Runner labels execute jobs in `gitsss/gitsss`; host execution is disabled.
- Every job starts a private Docker daemon. Repositories do not share its
  images, containers, networks, or volumes.
- Interrupted `GITEA-ACTIONS-TASK-*` Docker resources are removed before the
  runner accepts new work.
- Bundled `actions/checkout@v4` and `actions/upload-artifact@v4` ran with
  offline mode enabled.
- Isolated `publish-express233.yml` run 8 passed: checkout, empty private
  Docker inventory check, scratch image build and inspection, and artifact
  upload.
- The uploaded artifact was persisted at 189 bytes.
- Storage awareness lists Git, LFS, and total bytes per repository.
- Admin manual cleanup passed the full scan, review, confirmation, deletion,
  and revalidation flow against an expired managed temporary file.
- Final cleanup left zero verification/task containers, networks, and volumes.
- Final amd64 image:
  `sha256:a3c82c452519de1de0d3dc94cacca36a8efdee8c5ab328e862941b77d144b604`.

## Actions compatibility

- Gitea-compatible SQLite integration passed for manual inputs, job
  dependencies, failure propagation, matrix output aggregation, and workflow
  concurrency.
- Gitea service tests passed for workflow/job scheduling, rerun, cancellation,
  and concurrency.
- The test hook target was corrected from the inherited `gitea` executable
  name to the only supported command, `gitsss`.
- Detailed scope and evidence are recorded in
  `docs/verification/actions-compatibility-2026-07-16.md`.
