# gitsss server foundation

This Go 1.26 module runs the gitsss API and serves the sole React client from the same origin:

- scoped custom-token verification using constant-time hash comparison;
- dashboard and bounded event-preview HTTP endpoints;
- live filesystem capacity, reserve-aware write protection, bounded inventory and game-asset attribution endpoints;
- a persistent bundled Runner lifecycle with install/update/pause/repair/verify decisions, health and disk protection;
- embedded, lease-based Gitsss Pipelines with fixed local templates and a cross-platform Agent Runner mode;
- persistent bare-repository creation, authenticated Smart HTTP clone/push, and allowlisted tree, text preview and search endpoints;
- native Git LFS batch/upload/download with SHA-256 deduplication, disk admission, ranges and resumable chunks;
- email, Feishu and WeCom notification configuration, test delivery and transactional outbox endpoints;
- bounded, read-only source `app.ini` assessment with redacted configuration fingerprints and audited Agent report ingestion;
- encrypted, atomic takeover export with per-file SHA-256 manifests and read-only verification;
- loopback-only Agent HTTP with expiring scoped custom tokens, AI-tool-ready JSON contracts and retained audit records;
- an outbound Agent job protocol with persistent claims, heartbeats, identity-bound work tokens, idempotent completion and strict task allowlisting;
- audited preview clearing with retention-based audit cleanup;
- a native Agent CLI probe over authenticated HTTP.

## Local verification

```powershell
$env:GITSSS_API_TOKEN = "replace-with-at-least-24-random-characters"
$env:GITSSS_DATA_PATH = "$PWD\data\gitsss.db"
$env:GITSSS_MASTER_KEY_PATH = "$PWD\data\gitsss.db.key"
go run ./cmd/gitsss
go run ./cmd/gitsss-agent probe --server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN
go run ./cmd/gitsss-agent runner-report --server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN --id local-agent --platform windows/amd64
```

Run `pnpm build` in `web/` before starting from source. A packaged `gitsss` binary discovers the adjacent `web/` bundle automatically; source runs discover `web/dist`. `GITSSS_WEB_ROOT` can select another absolute or relative bundle directory containing a plain `index.html`. Requests below `/api/` always stay on the API handler, while React routes receive the SPA shell and hashed assets use immutable caching.

The server creates one permission-restricted embedded SQLite file at `GITSSS_DATA_PATH` (default `data/gitsss.db`) with WAL, full synchronous writes, a busy timeout and a versioned schema. Custom-token hashes and lifecycle metadata, Agent jobs, takeover assessments, notification configurations, the notification outbox, per-principal Dashboard event previews and the main-service audit log survive process restarts. Notification secrets, message content, Dashboard previews and audit entries are AES-256-GCM encrypted with identity-bound associated data and a generated 32-byte key at `GITSSS_MASTER_KEY_PATH` (default `<data-path>.key`). Dashboard previews use bounded event-ID LRU independently for each Token ID; clearing one principal never clears another, and compliance audit records are outside that LRU. Retention cleanup deletes expired audit documents in one transaction before changing the in-memory view. Unix uses `0600`; Windows applies a protected DACL for the service user, SYSTEM and Administrators without calling `icacls`. Losing the key intentionally makes startup fail instead of silently discarding encrypted state. See [`docs/architecture/dashboard-preview-v1.md`](../docs/architecture/dashboard-preview-v1.md).

Storage responses are measured from the live data volume and protect a 10 GiB operator reserve by default. Optional LFS, artifact, backup and attachment roots provide bounded category and game-asset attribution; measurement failures reject projected writes. Configure the reserve and roots with the `GITSSS_STORAGE_*` environment variables described in [`docs/architecture/storage-awareness-v1.md`](../docs/architecture/storage-awareness-v1.md).

On the first startup only, `GITSSS_API_TOKEN` seeds a full-scope `bootstrap` token that expires after 24 hours. Preview one of six least-privilege human/Agent role templates through `/api/v1/auth/roles`, then issue a named replacement through `POST /api/v1/auth/tokens`; the generated secret is returned once, while later lists expose metadata only. Role duration is bounded by risk, and advanced callers may still provide explicit scopes. Existing token state is never overwritten from the environment. Listing requires `auth:read`; issuing and revoking require `auth:write`. See [`docs/architecture/auth-token-v1.md`](../docs/architecture/auth-token-v1.md).

Start the local Agent HTTP control plane with an expiring token read from the environment:

```powershell
$env:GITSSS_AGENT_HTTP_TOKEN = "replace-with-at-least-24-random-characters"
go run ./cmd/gitsss-agent serve --listen 127.0.0.1:8790 --token-ttl 24h
```

The listener rejects non-loopback addresses. Its default token scopes are `agent:read`, `takeover:assess`, `backup:verify` and `audit:read`; narrow them with `--scopes`. Tokens are accepted only as strict `Authorization: Bearer ...` headers and are never written to audit output. Agent HTTP audit entries are encrypted in a permission-restricted SQLite file under the OS user configuration directory and survive restarts; use `--state` and `--master-key` to place both files on a service volume. Retention or append failures fail the protected HTTP request. `/api/v1/capabilities`, `/api/v1/assessments`, `/api/v1/backups/verify` and `/api/v1/audit` provide the first stable JSON tools for a local AI Agent. Backup verification accepts an environment-variable name rather than a passphrase. See [`docs/architecture/agent-http-v1.md`](../docs/architecture/agent-http-v1.md).

For centrally orchestrated takeover, create a typed job through `/api/v1/agent/jobs`, then let the old server initiate the connection:

```powershell
$env:GITSSS_AGENT_WORK_TOKEN = "replace-with-a-scoped-work-token"
go run ./cmd/gitsss-agent work --server http://gitsss.internal:8787 --id old-server
```

The persistent worker drains available jobs immediately, backs empty queues off from one to thirty seconds, and backs control-plane failures off to two minutes. If execution finishes but its completion response is lost, the running process retains the result and retries completion without executing the task again. `work-once` remains available for diagnostics; its `--token-env` form avoids exposing the token in process arguments.

Generate a reviewable boot-service plan without changing the host:

```powershell
gitsss-agent service-plan --server https://gitsss.internal --id old-server
```

Windows Task Scheduler, macOS launchd and Linux systemd plans reference a protected `--token-file`; the token value is never embedded in the service definition or plan JSON. See [`docs/operations/agent-autostart.md`](../docs/operations/agent-autostart.md).

Install the native package as administrator/root with the scoped token supplied through `GITSSS_AGENT_WORK_TOKEN`:

```powershell
gitsss-agent service-install --server https://gitsss.internal --id old-server
gitsss-agent service-uninstall
```

Installation is no-replace and transactional: it atomically publishes the protected token and service definition before registration, then rolls both back if service activation fails. Uninstall retains credentials when the platform service cannot be removed.

The queue accepts only `takeover.assess`, `takeover.freeze-check`, `backup.export`, `backup.restore-isolated`, `backup.audit-restore`, `takeover.inventory`, `takeover.shadow-audit`, `takeover.cutover`, `takeover.rollback` and `backup.verify`; it cannot execute shell commands. A work Token ID must equal the Agent ID, and long-running jobs renew their five-minute leases. gitsss itself uses embedded SQLite and has no external database runtime. SQLite takeover includes WAL through the embedded online-backup API. PostgreSQL/MySQL takeover uses pure-Go drivers compiled into the Agent, produces a deterministic typed JSONL snapshot, and requires current offline freeze evidence; no `pg_dump` or `mysqldump` installation is needed. Business inventory validates and counts core collaboration objects before shadow audit. Cutover repeats both checks, atomically swaps same-filesystem directories and preserves both generations; rollback refuses to discard post-cutover writes. Passphrases remain in a named environment variable on the old server. See [`docs/architecture/agent-jobs-v1.md`](../docs/architecture/agent-jobs-v1.md).

Notification control-plane routes use the `notifications:read`, `notifications:write` and `notifications:dispatch` scopes. Configure a channel, send a test, and inspect the outbox through `/api/v1/notifications/channels`, `/api/v1/notifications/channels/{channel}/test` and `/api/v1/notifications/outbox`. The server automatically dispatches due messages every 15 seconds. Delivery claims and preflight audits are durable before network I/O; failures retry with exponential backoff and move to `dead` after four attempts. A process crash or expired 15-minute lease recovers with explicit at-least-once semantics. Pipeline results enqueue configured channels automatically. Webhooks require official HTTPS hosts, and email requires implicit TLS or verified STARTTLS. See [`docs/architecture/notifications-v1.md`](../docs/architecture/notifications-v1.md).

Runner reports persist registration, version, health, work-volume free space and errors across restart. Reconciliation returns one of install, update, pause, repair or verify; low disk pauses new work, and missing reports become offline after two minutes. State and audit failures retain the previous record and fail the request. See [`docs/architecture/runner-lifecycle-v1.md`](../docs/architecture/runner-lifecycle-v1.md).

Pipeline templates and runs are stored in the same embedded database. A maintainer queues an immutable commit, and `gitsss-agent service-install --runner` installs the bundled worker as a separate Windows, macOS or Linux boot service. The Runner accepts only templates compiled into both sides, never an API-supplied shell command; it uses bounded leases, retries, logs, Token redaction, fixed step timeouts and isolated per-run workspaces. See [`docs/architecture/pipelines-v1.md`](../docs/architecture/pipelines-v1.md).

GitHub-hosted CI is disabled while the repository has no Actions quota. The trusted Windows release path runs the complete local checks, cross-builds six Linux/macOS/Windows packages, verifies binary architecture, writes SHA-256 checksums and can upload a commit-bound prerelease. See [`docs/operations/local-release.md`](../docs/operations/local-release.md).

`gitsss-agent update-verify` verifies a canonical update manifest, its Ed25519 signature and the exact OS/architecture package against an out-of-band trusted public key. `update-stage` re-verifies the same package handle and extracts it into a new bounded, link-free staging directory with ready metadata. Neither command downloads or activates the package. `update-activate` serializes activation, publishes a new immutable launcher generation, restarts the platform service, requires both a launcher start receipt and authenticated Agent control-plane health, and publishes a verified rollback generation on failure. See [`docs/operations/signed-updates.md`](../docs/operations/signed-updates.md).

Release packages also include the stable `gitsss-launcher` entrypoint. It executes the newest immutable canonical active-state generation without a shell, allowing a platform service to keep the launcher stable while versioned Agent binaries change. `service-install` auto-detects packaged launcher/BUILDINFO files, publishes generation 1 and registers the platform service against the launcher. See [`docs/architecture/launcher-state-v1.md`](../docs/architecture/launcher-state-v1.md).

Set `GITSSS_REPOSITORY_PATH` to a working tree, subdirectory, or bare repository. The server resolves working-tree subdirectories to the repository root and exposes only configured repository IDs; API callers cannot supply arbitrary filesystem paths. Text previews are capped at 1 MiB, binary game assets return metadata only, and search results are bounded.

New repositories are created under `GITSSS_REPOSITORY_ROOT` (default `<data-directory>/repositories`) through `POST /api/v1/repositories`. Clone/fetch and push use separate `repositories:clone` and `repositories:push` Token scopes at `/git/{repository}.git`; Git credential helpers can use the Token as the password. The catalog survives restart, creation is no-replace and audited, and filesystem paths never appear in API responses. See [`docs/architecture/git-smart-http-v1.md`](../docs/architecture/git-smart-http-v1.md).

Git LFS uses `GITSSS_LFS_ROOT` (default `<data-directory>/lfs`) and the same repository scopes. Objects are content-addressed and streamed without recompression; batch upload checks the live storage reserve, and custom clients can resume sequential `Content-Range` chunks. New hosted repositories also advertise protocol v2 partial-clone filters and maintain commit-graph/bitmap settings. See [`docs/architecture/lfs-game-assets-v1.md`](../docs/architecture/lfs-game-assets-v1.md).

Run a takeover assessment locally without uploading credentials:

```powershell
go run ./cmd/gitsss-agent assess --agent-id old-server --config C:\old-service\custom\conf\app.ini
```

Add `--server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN` to submit the redacted report. The scanner reads at most 2 MiB of configuration, never follows symlinks, caps each asset category at 10,000 files by default, strips URL paths and queries, and does not return database passwords or integration secrets. Submission uses the `takeover:write` scope; report listing uses `takeover:read`.

Export an assessed instance to a new encrypted container and verify it without extracting files:

```powershell
$env:GITSSS_BACKUP_PASSPHRASE = "replace-with-a-long-secret"
go run ./cmd/gitsss-agent backup-export --agent-id old-server --config C:\old-service\custom\conf\app.ini --output C:\backups\source.gitsss-backup
go run ./cmd/gitsss-agent backup-verify --input C:\backups\source.gitsss-backup
go run ./cmd/gitsss-agent backup-import-plan --input C:\backups\source.gitsss-backup --target C:\gitsss-import\source
go run ./cmd/gitsss-agent backup-restore-isolated --input C:\backups\source.gitsss-backup --target C:\gitsss-import\source --confirm assessment-0123456789abcdef
go run ./cmd/gitsss-agent backup-audit-restore --input C:\backups\source.gitsss-backup --target C:\gitsss-import\source
```

The passphrase is accepted only through the named environment variable, never a command-line value. Export refuses symlinks, incomplete assessments, changed source files and existing output paths. It writes and syncs a same-directory temporary file, then publishes it with an atomic no-replace link only after the encrypted tar stream has closed successfully. Verification is read-only, reports the raw container byte count and SHA-256, and rejects wrong passphrases, altered headers or payloads, truncation, appended bytes, unsafe paths, duplicate entries, and checksum mismatches. Import planning also remains read-only: after full container verification, it marks every destination as `new`, `identical`, `conflict`, or `blocked` and refuses to traverse symlinks. Isolated restore requires the exact assessment ID, refuses existing targets, verifies twice around extraction, and publishes plaintext only under the new target's `content/` directory. Post-restore audit repeats the manifest comparison, runs strict Git object verification, fingerprints refs, counts objects, and checks LFS OIDs; it exits non-zero unless `ready_for_shadow_mode` is true. None of these commands imports into a live source or gitsss path. See [`docs/migration/backup-container-v1.md`](../docs/migration/backup-container-v1.md) for the versioned format and current limitations.
