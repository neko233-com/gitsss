# Bundled Runner lifecycle v1

The Runner control plane stores every Agent's registration, platform, installed and desired versions, health result, bounded error summary, work-volume free space, and last report time in embedded SQLite. Restarting gitsss does not reset provisioning state.

`POST /api/v1/runners/reconcile` requires `runners:write` and returns exactly one linear next action:

1. `install` when the Runner is unknown or registration is missing;
2. `update` when the installed package differs from the server target;
3. `pause` when the work volume reports less than 5 GiB free;
4. `repair` when registration exists but health fails;
5. `verify` when registration, version, disk and health are ready.

The state change and its actor are audited. A persistence or audit failure rejects reconciliation and retains the previous state. `GET /api/v1/runners` requires `runners:read` and derives `offline` when no report arrives for two minutes; this presentation state does not overwrite the last durable report.

The native Agent `runner-report` command accepts explicit registration, health, error and free-space evidence. Its response is an action contract, not a shell command: only the built-in install/update/pause/repair/verify handlers may consume it. `service-install --runner` installs a separate cross-platform `pipeline-work` service from the same signed Agent package. Arbitrary commands, executable paths and download URLs are never accepted from the API. See [`pipelines-v1.md`](pipelines-v1.md).
