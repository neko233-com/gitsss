# Gitsss Pipelines v1

Gitsss Pipelines is an embedded-SQLite control plane plus a Runner mode inside the packaged `gitsss-agent`. It does not require GitHub Actions, a separate queue, Redis, a database server, or an independently downloaded Runner.

## Linear run lifecycle

1. A maintainer selects a repository, immutable Git ref and built-in template through `POST /api/v1/pipelines/runs`.
2. gitsss resolves the ref to a full commit before queuing the run; a Runner never executes a moving branch name.
3. A Token whose ID equals the Runner ID claims the oldest eligible run with a two-minute lease.
4. The Runner clones through authenticated Git protocol v2, checks out the exact commit, validates the server template byte-for-byte against its local allowlist, and runs each argv step without a shell.
5. Heartbeats extend the lease and upload bounded logs. Completion is idempotent and survives a lost response without re-executing in the same process.
6. A lost Runner may retry a run at most three times; a third expired lease closes the run as failed instead of looping forever.

The first templates are `go-verify`, `web-verify`, and `gitsss-verify`. API callers choose a template ID only; they cannot submit commands, executable paths, working directories, timeouts or download URLs.

## Runner isolation boundary

- Clone credentials are passed only to the trusted Git child through temporary Git configuration environment variables; they are absent from command arguments and removed before repository code runs.
- Step processes receive an allowlisted environment that excludes Agent and Runner Tokens.
- Each run gets a no-reuse workspace protected with `0700` on Unix or a protected Windows DACL. The checkout is deleted after completion.
- Step working directories are resolved after symlink evaluation and must remain inside the checkout.
- Each step has a fixed timeout and bounded synchronized stdout/stderr capture.
- Server logs are capped at 128 KiB, heartbeat chunks at 32 KiB, and `gitsss_...` credentials are redacted even when split across chunks.
- Repository code still executes with the dedicated Runner service account's OS authority. Production hosts should keep that account isolated from gitsss state, backup keys and administrator credentials.

## Installation

Issue a `runner-agent` role Token whose ID matches `--id`, then install the same packaged Agent binary in Runner mode:

```powershell
$env:GITSSS_RUNNER_TOKEN = "replace-with-the-issued-token"
gitsss-agent service-install --runner --server https://gitsss.internal --id runner-1
```

`--runner` produces a separate `gitsss-runner` Task Scheduler, launchd or systemd definition, a protected `runner.token`, an isolated `runner-work` directory, and the `pipeline-work` command. The stable launcher/update transaction preserves those arguments across signed upgrades.
