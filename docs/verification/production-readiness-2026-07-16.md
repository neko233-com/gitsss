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
- Production Linux hosts are limited to Ubuntu 24.04 LTS and Debian-family
  systems. Formal images cover `linux/amd64` and `linux/arm64`.
- Supported Actions resolve from `/app/gitsss/builtin-actions`; unknown remote
  Actions fail closed.
- Update checking is disabled by default and has no external endpoint.
- Compose uses one image, one data volume, default HTTP port 3000, a health
  check, `no-new-privileges`, and a two-minute graceful-stop window.
- Shell and PowerShell installers load an offline image archive, never pull
  implicitly, and support changing the host HTTP port while retaining the same
  data volume and internal port 3000.
- Installers configure silent background startup after Docker is ready: a
  hidden Windows task (with a hidden `wscript` Startup-folder fallback), a
  systemd unit on Linux, and a launchd agent on macOS. Background output is
  written below the installation directory's `logs/` directory.
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
- Live `git.example.internal:3000` was not accessed or changed.

## Platform verification

- Full backend test suite passed in Ubuntu 24.04 with Git 2.43.
- Go formatting, race tests, vet, and lint (`0 issues`) passed.
- Frontend lint, Vue type checking, 35 Vitest files, and 92 tests passed.
- Markdown lint passed for root and verification/operations documentation.
- Final `linux/amd64` image:
  `sha256:9798abea88f05dba5394820ec692b7a08573f0ce217477c0fcd013bda812b845`.
- Final `linux/arm64` image:
  `sha256:cf555d3b670734901abd1313d748d2450013ed65cc8f21a9f2cd3e937f2c79ff`.
- Both image architectures started successfully, returned HTTP health 200,
  served all eight linked login-page assets with HTTP 200, and logged no theme
  loading errors.
- The source archive built and ran with container networking disabled,
  `GOPROXY=off`, and `-mod=vendor`.
- Browser acceptance created the public Gitea source repository
  `acceptance-admin/test-gitsss-repo` with an initialized README, then migrated
  it through the GitSSS Gitea migration UI. Both `main` refs resolve to
  `72ec7fd00ee2e7024868e6fd1d76c13131e5ac0e`; GitSSS renders the repository,
  README, branch/commit counts, and empty Actions page without fresh console
  warnings or errors. The local-network migration allowance was present only
  on the disposable GitSSS acceptance container.
- On the Windows acceptance host, `C:\gitsss` starts `gitsss/gitsss:latest`
  in the background with `restart: unless-stopped`; HTTP health returned 200.
  Local task registration was policy-blocked, so the installer selected its
  hidden `wscript` Startup-folder fallback and verified its background log.

## Release artifacts

- Raw and XZ-compressed binaries exist for macOS amd64/arm64, Linux
  amd64/arm64, and Windows 386/amd64/arm64.
- Windows also has amd64/arm64/386 Go-Git variants.
- Offline image archives exist for Linux amd64 and arm64.
- Shell and PowerShell installer bundles and the vendored source archive are
  present.
- All 25 release payloads have matching SHA-256 files, and every checksum
  verification passed.
- The release checksum target is idempotent and excludes existing checksum
  files, preventing nested `.sha256.sha256` artifacts on repeat verification.

## Cleanup and handoff

- All GitSSS test containers, workflow containers, test/cache volumes,
  networks, source images, gate images, the temporary arm64 tag, and the xgo
  builder image were removed.
- Only `gitsss/gitsss:1.0.0` and `gitsss/gitsss:latest` remain; both point to
  the final amd64 image.
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
  `sha256:9798abea88f05dba5394820ec692b7a08573f0ce217477c0fcd013bda812b845`.

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

## Outstanding release gates

- Restore GitHub authentication, push the branch and release tag, then upload
  the locally built artifacts without GitHub Actions.
