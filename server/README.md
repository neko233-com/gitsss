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

Notification control-plane routes use the `notifications:read`, `notifications:write` and `notifications:dispatch` scopes. Configure a channel, send a test, and inspect the outbox through `/api/v1/notifications/channels`, `/api/v1/notifications/channels/{channel}/test` and `/api/v1/notifications/outbox`. Failed deliveries retry with exponential backoff and move to `dead` after four attempts.

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

The passphrase is accepted only through the named environment variable, never a command-line value. Export refuses symlinks, incomplete assessments, changed source files and existing output paths. It writes and syncs a same-directory temporary file, then publishes it with an atomic no-replace link only after the encrypted tar stream has closed successfully. Verification is read-only and rejects wrong passphrases, altered headers or payloads, truncation, appended bytes, unsafe paths, duplicate entries, and checksum mismatches. Import planning also remains read-only: after full container verification, it marks every destination as `new`, `identical`, `conflict`, or `blocked` and refuses to traverse symlinks. Isolated restore requires the exact assessment ID, refuses existing targets, verifies twice around extraction, and publishes plaintext only under the new target's `content/` directory. Post-restore audit repeats the manifest comparison, runs strict Git object verification, fingerprints refs, counts objects, and checks LFS OIDs; it exits non-zero unless `ready_for_shadow_mode` is true. None of these commands imports into a live database or Gitea path. See [`docs/migration/backup-container-v1.md`](../docs/migration/backup-container-v1.md) for the versioned format and current limitations.
