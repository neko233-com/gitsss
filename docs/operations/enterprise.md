# Enterprise defaults and compatibility

GitSSS serves internal development organizations. Fresh installations bind
`0.0.0.0:5030`, disable self-registration, and require sign-in to view content.
Explicit existing configuration continues to take precedence. Match the source
Gitea's registration, visibility, organization and repository permissions before
cutover.

The signed-in home page lists the viewer's accessible organizations first.
`/?view=personal` opens the personal dashboard. Organization membership and
repository access are still checked on the server.

Teams have an optional `display_name` for editable human-readable names, including
the Owners team. The existing team ID, `name`, `lower_name`, routes and owner-team
identity remain unchanged. The label grants no permissions. Empty labels display
the original name. Web settings and the team API accept the label.

GitSSS extension version 1 adds `team.display_name` and preserves project fields
through a separate `gitsss_schema_version` table. Upstream database version 343
retains its upstream meaning. No Git, LFS, database files or storage roots move.
The previous 1.26-based GitSSS binary cannot read version 343: rollback requires
its pre-upgrade database backup as well as its binary. A native Gitea 1.27.1
rollback uses the pre-cutover snapshot and original configuration. See the
[compatibility report](upstream-compatibility.md).

## Parallel migration

Keep Gitea on `3000` and GitSSS on `5030` until the operator requests cutover.
During rehearsal disable the built-in runner and scheduled tasks. Do not attach
production runners or allow imported webhooks to deploy production changes.

HTTP import cannot preserve passwords, API tokens, SSH keys, runner credentials,
Actions history/secrets, or webhook state. A site using those features requires a
stopped native database/data takeover for final compatibility, with its original
secrets and an explicit path/configuration mapping. Do not declare an HTTP-only
pilot a complete replacement. Native takeover must use an independent database
snapshot; never run both applications against one writable database or shared
mutable repository tree.

An HTTP collaborator permission of `owner` is inherited from the Owners team.
It is copied through that team after membership verification; it must never be
converted into a new direct admin grant. HTTP does not expose any underlying
direct grant masked by that ownership. Review the plan warning before cutover.

## Repeatable operator checks

The offline HTML manual is bundled at `/help/`, English by default. It works
without GitHub Pages or a GitHub Actions workflow. Native JavaScript Actions
require Git, Git LFS and Node.js on the runner. Host labels follow the host OS;
custom production labels remain explicit configuration.

```sh
python3 tools/agent-ops.py status --url http://gitsss.example.internal:5030 --token-file target.token
python3 tools/verify-git.py --url http://gitsss.example.internal:5030 --token-file target.token --ssh-known-hosts trusted-hosts
python3 tools/rehearse-native.py --source-work-path /path/to/gitea --work-path /path/to/new-rehearsal --binary /path/to/gitsss --apply
```

The native rehearsal tool requires a new independent directory, creates a private
SQLite backup, disables external side effects in its copied configuration, and
compares every original table column and row ID after migration. It neither
starts a server nor copies Git/LFS data. Configuration and logs stay private.
The Git check creates disposable repositories/users/keys and removes them; run
`--actions` only on an isolated instance with a dedicated runner label.
Agent dispatch/rerun operations require `--apply`; status is read-only.

Incremental Git sync records a private per-target checkpoint in the mirror cache.
It rejects any independent destination ref changes before writing, including a
destination-only branch. Keep the same cache between runs. A cache without a
checkpoint can initialize an existing destination only when its refs already
match the source. Source deletions prune the previously synchronized target refs;
non-fast-forward updates remain refused. Metadata after initial HTTP import is
not synchronized by this Git-only mode.
