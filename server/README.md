# gitsss server foundation

This Go 1.26 module establishes the first server boundaries for gitsss:

- scoped custom-token verification using constant-time hash comparison;
- dashboard and bounded event-preview HTTP endpoints;
- storage capacity, write-protection and admission endpoints;
- a bundled Runner reconciliation contract for Agent-driven install, update and health verification;
- allowlisted Git tree, text preview and repository search endpoints backed by immutable commit snapshots;
- email, Feishu and WeCom notification configuration, test delivery and transactional outbox endpoints;
- bounded, read-only Gitea `app.ini` assessment with redacted configuration fingerprints and audited Agent report ingestion;
- encrypted, atomic takeover export with per-file SHA-256 manifests and read-only verification;
- loopback-only Agent HTTP with expiring scoped custom tokens, AI-tool-ready JSON contracts and retained audit records;
- an outbound Agent job protocol with atomic claims, leases, idempotent completion and strict task allowlisting;
- audited preview clearing with retention-based audit cleanup;
- a native Agent CLI probe over authenticated HTTP.

## Local verification

```powershell
$env:GITSSS_API_TOKEN = "replace-with-a-local-test-token"
go run ./cmd/gitsss
go run ./cmd/gitsss-agent probe --server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN
go run ./cmd/gitsss-agent runner-report --server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN --id local-agent --platform windows/amd64
```

The current in-memory store is an implementation slice, not production persistence. Tokens, notification secrets, outbox messages and audit records will move to encrypted, durable storage before release. Notification API responses and audit targets never include webhook URLs, SMTP passwords or signing secrets. Feishu and WeCom webhooks are restricted to their official HTTPS hosts; an internal relay requires a future explicit administrator allowlist.

Start the local Agent HTTP control plane with an expiring token read from the environment:

```powershell
$env:GITSSS_AGENT_HTTP_TOKEN = "replace-with-at-least-24-random-characters"
go run ./cmd/gitsss-agent serve --listen 127.0.0.1:8790 --token-ttl 24h
```

The listener rejects non-loopback addresses. Its default token scopes are `agent:read`, `takeover:assess`, `backup:verify` and `audit:read`; narrow them with `--scopes`. Tokens are accepted only as strict `Authorization: Bearer ...` headers and are never written to audit output. `/api/v1/capabilities`, `/api/v1/assessments`, `/api/v1/backups/verify` and `/api/v1/audit` provide the first stable JSON tools for a local AI Agent. Backup verification accepts an environment-variable name rather than a passphrase. See [`docs/architecture/agent-http-v1.md`](../docs/architecture/agent-http-v1.md).

For centrally orchestrated takeover, create a typed job through `/api/v1/agent/jobs`, then let the old server initiate the connection:

```powershell
$env:GITSSS_AGENT_WORK_TOKEN = "replace-with-a-scoped-work-token"
go run ./cmd/gitsss-agent work --server http://gitsss.internal:8787 --id old-gitea
```

The persistent worker drains available jobs immediately, backs empty queues off from one to thirty seconds, and backs control-plane failures off to two minutes. If execution finishes but its completion response is lost, the running process retains the result and retries completion without executing the task again. `work-once` remains available for diagnostics; its `--token-env` form avoids exposing the token in process arguments.

Generate a reviewable boot-service plan without changing the host:

```powershell
gitsss-agent service-plan --server https://gitsss.internal --id old-gitea
```

Windows Task Scheduler, macOS launchd and Linux systemd plans reference a protected `--token-file`; the token value is never embedded in the service definition or plan JSON. See [`docs/operations/agent-autostart.md`](../docs/operations/agent-autostart.md).

Install the native package as administrator/root with the scoped token supplied through `GITSSS_AGENT_WORK_TOKEN`:

```powershell
gitsss-agent service-install --server https://gitsss.internal --id old-gitea
gitsss-agent service-uninstall
```

Installation is no-replace and transactional: it atomically publishes the protected token and service definition before registration, then rolls both back if service activation fails. Uninstall retains credentials when the platform service cannot be removed.

The queue currently accepts only `takeover.assess`, `backup.export`, `backup.restore-isolated`, `backup.audit-restore` and `backup.verify`; it cannot execute shell commands. An export re-runs assessment and requires its resulting ID to match the administrator's explicit confirmation before creating a no-replace encrypted container. An isolated restore binds both authenticated reads to the confirmed container SHA-256 before publishing a new directory. The post-restore audit is read-only and reports file, strict Git and LFS evidence separately from its shadow-mode readiness decision. Passphrases remain in a named environment variable on the old server. A claim has a five-minute lease, is bound to one Agent ID, and terminal completion is idempotent. See [`docs/architecture/agent-jobs-v1.md`](../docs/architecture/agent-jobs-v1.md).

Notification control-plane routes use the `notifications:read`, `notifications:write` and `notifications:dispatch` scopes. Configure a channel, send a test, and inspect the outbox through `/api/v1/notifications/channels`, `/api/v1/notifications/channels/{channel}/test` and `/api/v1/notifications/outbox`. Failed deliveries retry with exponential backoff and move to `dead` after four attempts.

GitHub-hosted CI is disabled while the repository has no Actions quota. The trusted Windows release path runs the complete local checks, cross-builds six Linux/macOS/Windows packages, verifies binary architecture, writes SHA-256 checksums and can upload a commit-bound prerelease. See [`docs/operations/local-release.md`](../docs/operations/local-release.md).

`gitsss-agent update-verify` verifies a canonical update manifest, its Ed25519 signature and the exact OS/architecture package against an out-of-band trusted public key. `update-stage` re-verifies the same package handle and extracts it into a new bounded, link-free staging directory with ready metadata. Neither command downloads or activates the package. `update-activate` serializes activation, publishes a new immutable launcher generation, restarts the platform service, requires both a launcher start receipt and authenticated Agent control-plane health, and publishes a verified rollback generation on failure. See [`docs/operations/signed-updates.md`](../docs/operations/signed-updates.md).

Release packages also include the stable `gitsss-launcher` entrypoint. It executes the newest immutable canonical active-state generation without a shell, allowing a platform service to keep the launcher stable while versioned Agent binaries change. `service-install` auto-detects packaged launcher/BUILDINFO files, publishes generation 1 and registers the platform service against the launcher. See [`docs/architecture/launcher-state-v1.md`](../docs/architecture/launcher-state-v1.md).

Set `GITSSS_REPOSITORY_PATH` to a working tree, subdirectory, or bare repository. The server resolves working-tree subdirectories to the repository root and exposes only configured repository IDs; API callers cannot supply arbitrary filesystem paths. Text previews are capped at 1 MiB, binary game assets return metadata only, and search results are bounded.

Run a takeover assessment locally without uploading credentials:

```powershell
go run ./cmd/gitsss-agent assess --agent-id old-gitea --config C:\gitea\custom\conf\app.ini
```

Add `--server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN` to submit the redacted report. The scanner reads at most 2 MiB of configuration, never follows symlinks, caps each asset category at 10,000 files by default, strips URL paths and queries, and does not return database passwords or integration secrets. Submission uses the `takeover:write` scope; report listing uses `takeover:read`.

Export an assessed instance to a new encrypted container and verify it without extracting files:

```powershell
$env:GITSSS_BACKUP_PASSPHRASE = "replace-with-a-long-secret"
go run ./cmd/gitsss-agent backup-export --agent-id old-gitea --config C:\gitea\custom\conf\app.ini --output C:\backups\gitea.gitsss-backup
go run ./cmd/gitsss-agent backup-verify --input C:\backups\gitea.gitsss-backup
go run ./cmd/gitsss-agent backup-import-plan --input C:\backups\gitea.gitsss-backup --target C:\gitsss-import\old-gitea
go run ./cmd/gitsss-agent backup-restore-isolated --input C:\backups\gitea.gitsss-backup --target C:\gitsss-import\old-gitea --confirm assessment-0123456789abcdef
go run ./cmd/gitsss-agent backup-audit-restore --input C:\backups\gitea.gitsss-backup --target C:\gitsss-import\old-gitea
```

The passphrase is accepted only through the named environment variable, never a command-line value. Export refuses symlinks, incomplete assessments, changed source files and existing output paths. It writes and syncs a same-directory temporary file, then publishes it with an atomic no-replace link only after the encrypted tar stream has closed successfully. Verification is read-only, reports the raw container byte count and SHA-256, and rejects wrong passphrases, altered headers or payloads, truncation, appended bytes, unsafe paths, duplicate entries, and checksum mismatches. Import planning also remains read-only: after full container verification, it marks every destination as `new`, `identical`, `conflict`, or `blocked` and refuses to traverse symlinks. Isolated restore requires the exact assessment ID, refuses existing targets, verifies twice around extraction, and publishes plaintext only under the new target's `content/` directory. Post-restore audit repeats the manifest comparison, runs strict Git object verification, fingerprints refs, counts objects, and checks LFS OIDs; it exits non-zero unless `ready_for_shadow_mode` is true. None of these commands imports into a live database or Gitea path. See [`docs/migration/backup-container-v1.md`](../docs/migration/backup-container-v1.md) for the versioned format and current limitations.
