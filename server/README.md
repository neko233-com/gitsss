# gitsss server foundation

This Go 1.26 module establishes the first server boundaries for gitsss:

- scoped custom-token verification using constant-time hash comparison;
- dashboard and bounded event-preview HTTP endpoints;
- storage capacity, write-protection and admission endpoints;
- a bundled Runner reconciliation contract for Agent-driven install, update and health verification;
- audited preview clearing with retention-based audit cleanup;
- a native Agent CLI probe over authenticated HTTP.

## Local verification

```powershell
$env:GITSSS_API_TOKEN = "replace-with-a-local-test-token"
go run ./cmd/gitsss
go run ./cmd/gitsss-agent probe --server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN
go run ./cmd/gitsss-agent runner-report --server http://127.0.0.1:8787 --token $env:GITSSS_API_TOKEN --id local-agent --platform windows/amd64
```

The current in-memory store is an implementation slice, not production persistence. Tokens and audit records will move to encrypted, durable storage before release.
