# gitsss server foundation

This Go 1.26 module establishes the first server boundaries for gitsss:

- scoped custom-token verification using constant-time hash comparison;
- dashboard and bounded event-preview HTTP endpoints;
- storage capacity, write-protection and admission endpoints;
- a bundled Runner reconciliation contract for Agent-driven install, update and health verification;
- allowlisted Git tree, text preview and repository search endpoints backed by immutable commit snapshots;
- email, Feishu and WeCom notification configuration, test delivery and transactional outbox endpoints;
- bounded, read-only Gitea `app.ini` assessment with redacted configuration fingerprints and audited Agent report ingestion;
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
