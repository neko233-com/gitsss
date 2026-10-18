# Enterprise defaults and compatibility

GitSSS serves internal development organizations. Fresh installations bind
`0.0.0.0:5030`, disable self-registration, and require sign-in to view content.
Explicit existing configuration continues to take precedence. Match the source
Gitea's registration, visibility, organization and repository permissions before
cutover.

Public HTTPS deployments include adaptive login verification without permanent
shared-IP bans. Configure the trusted proxy chain and keep tunnel backend ports
private; see [public access and login protection](public-access.md).

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

The server's operating-system SSH account, password and port 22 are independent
of GitSSS's Git SSH listener (default 2222) and remain unchanged. An SSH client
that offers many agent keys can exhaust OpenSSH's per-connection attempt limit;
for password login select `PreferredAuthentications=password` and
`PubkeyAuthentication=no` on that connection. This does not change server policy
or the account password.

Incremental Git sync records a private per-target checkpoint in the mirror cache.
It rejects any independent destination ref changes before writing, including a
destination-only branch. Keep the same cache between runs. A cache without a
checkpoint can initialize an existing destination only when its refs already
match the source. Source deletions prune the previously synchronized target refs;
non-fast-forward updates remain refused. Metadata after initial HTTP import is
not synchronized by this Git-only mode.

## Language and readable themes

The default UI language follows the system language supplied by the browser in
`Accept-Language`. Automatic detection is not saved as a permanent account choice
at login. The footer offers Automatic and individual languages; account settings
under Appearance also offer Automatic. Explicit account language choices are
retained. Existing accounts with an explicit choice can select Automatic again.
The document language and custom React views follow the selected UI language.
Core UI uses the upstream language catalog; new custom strings provide Chinese
and English, with English fallback for other locales. README.md and the offline
manual's initial page remain English by default.

GitSSS light/dark themes use readable secondary text, placeholders and form
labels. The dark navigation bar's colors are limited to its top-level controls;
dropdown panels use their own surface and text colors. Worker captions are larger.

## One organization board and repository boards

Open `/{organization}/-/kanban` to enter the organization's single shared board.
There is no project selector or repository prerequisite. Each column has a quick
entry: type a task and press Enter; `@username` assigns the last mentioned
organization member. The member suggestions contain organization members only.
Cards have To do, In progress and Done states; drag between columns or use Edit.
Search matches task text and `@username`; the assignee filter can be combined
with search. Search is immediate for loaded cards; press Search to query stored
tasks. At most 500 matching cards are rendered and a limit notice asks for a
narrower query. Each card supports details and an explicit assignee selection.

Only signed-in organization members and site administrators can access this
board. Restricted guest accounts cannot access it unless they own the
organization. Members collaborate on cards; deletion requires the creator or an
organization owner. Assignees must belong to the same organization. Server-side
organization scoping applies to reads, edits, moves and deletes.

Repository boards remain at `/{organization}/{repository}/projects`. Legacy
owner project routes, records, fields and issue links remain compatible. New
standalone cards use the additive `org_task` table under GitSSS extension schema
version 2; upstream schema 343 and Git/LFS paths and bytes are unchanged. Version
1 binaries refuse extension version 2: rollback requires the matching database
backup as well as the old binary. Include trial task cards in the final takeover
export and verify organization/user ID mapping before importing them.

## Desktop layout and local avatars

The desktop layout uses the available browser width, while profile forms retain
a readable width. Actions search has an accessible label without visible label
text squeezed into the input. Statistic labels, dropdowns and placeholders use
the current theme's readable colors. Mobile has a separate future web scope.

Settings → Profile offers 24 ready-made DiceBear Adventurer/Lorelei avatars, embedded locally without
external image requests. Selecting one uses the existing avatar upload/storage
pipeline. Custom uploads are automatically cropped to square, downscaled to
the configured rendered size without upscaling, and compressed as PNG or JPEG;
transparency remains PNG. New GIF uploads use the first frame. New automatically generated personal avatars use a deterministic built-in
choice; existing avatars and organization identities are preserved. Upload and
remove-avatar controls remain available. Asset sources, licenses and checksums are
recorded in `modules/avatar/builtin/README.md`.

## Protect the source's SSH authorization

Parallel instances need independent `SSH_ROOT_PATH` directories and both
`SSH_CREATE_AUTHORIZED_KEYS_FILE = false` and
`SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE = false`. Their runners and tests also need
an isolated HOME (USERPROFILE on Windows). A disabled SSH instance must never
regenerate the operating system's authorization files, even if a legacy flag is
true. Preserve the source authorization file and verify its hash before and after
Git/Actions acceptance. If that file is lost, regenerate it with the original
Gitea binary and original database/configuration, then verify the existing runner
key and actual source repository read. Do not change system login passwords or
SSH daemon policy to work around a missing application key.

Parallel instances on the same hostname also need distinct `[session] COOKIE_NAME`, `[security] COOKIE_REMEMBER_NAME` and `[i18n] COOKIE_NAME` values: cookies are shared across ports.
Final native takeover retains the original site's settings and credentials.
Export any organization boards created during the trial and migrate their contents
before final acceptance; never discard trial-only work when replacing the database.

The frontend pins one CodeMirror state/view version through pnpm overrides to
avoid incompatible duplicate editor instances. TSX is included in ESLint and
TypeScript checking; run the React and Vue builds before embedding assets.

Run `python3 tools/verify-enterprise.py --url http://gitsss.example.internal:5030 --token-file target.token` to verify shared cards, organization isolation, 24 avatar choices and actual multipart upload normalization. It creates disposable private organizations/accounts and removes its own fixtures afterward. Never point it at the original Gitea.

## Consistent organization workspace

Activity, repositories, the shared board and team management use one compact
organization header and navigation. Top-level issue/pull/milestone/board links
retain the current organization. Switching organizations retains the current
workspace section when supported. Organization and team dropdowns support search
by display label and stable name. Team selection on Activity updates the feed
and repository list in place, preserves repository search/filter settings, and
supports browser Back/Forward. A failed request retains the current contents.

Board quick-add, editing, deletion and stored search update the same page;
columns, filter inputs and scroll context remain in place. Failed saves retain
the draft. Dragging and explicit status changes both persist on the server.
Normal form submissions and existing URLs still work without JavaScript.

## Homepage quick links

The dedicated Quick links tab on organization and personal workspaces offers shared shortcuts managed
by site administrators and personal shortcuts managed by each signed-in user.
Expand the corresponding editor to add, rename or remove multiple links, then save
in place. Links open in a new tab so the workspace stays open. Each list supports
32 ordered links, with a 120-character name and an HTTP/HTTPS URL up to 2048 bytes.
Credentials in URLs and other URL schemes are rejected; the server does not fetch
these destinations. Failed saves retain drafts; stale editors receive a conflict
instead of overwriting another saved list. Personal ownership comes from the
authenticated server identity, and shared writes require site administrator access.

POST `/home-links` uses the normal signed-in web session and CSRF protection. Form
fields are `scope` (`shared` or `personal`), `revision` from the editor, and repeated
`title` / `url` pairs. A successful response is the refreshed HTML component;
400 indicates invalid input, 403 denied shared write, and 409 a stale revision.

Records use existing app-state keys `enterprise-home-links` and
`personal-home-links-{userID}`. No schema or Git/LFS storage changes are required.
Product defaults contain no private network URLs: configure enterprise destinations
on the deployed site. Export trial-only app-state records with task cards before
final takeover, mapping personal user IDs; rollback uses the matching database
backup. Desktop dashboard content has 16px side gutters; account forms keep their
readable width.

## Compact desktop layout and automatic teams

Activity uses three columns: repositories and filters on the left, activity in the
center, CI/CD and published announcements on the right. Contribution history is
visible initially and can be collapsed. Quick links have a separate tab (`?tab=links`) and do not
appear in Activity. Desktop text defaults to 14px with 16px page gutters, compact
headers/buttons and a single-line organization member table (28px avatars).
Code viewers retain their existing line-height settings. Browser zoom remains a
user preference; no page scaling or transforms are applied.

Organization owners open **Settings → Automatic team assignment** at
`/org/{org}/settings/team-rules`. Add a Go/RE2 regular expression matching the
repository name and select one or more teams. Examples: `^server-`, `-go$`, and
`(?i)^(client|tools)-`. Matching is case sensitive unless requested in the pattern.
Preview lists existing matches before saving; **Add teams to existing matches**
is an explicit, repeatable, additive operation. Saving a rule alone does not
change existing repository permissions.

Enabled rules run in the shared creation transaction, including API creation,
forks, template generation and imports, and when transferring a repository into
an organization. All matching rules are unioned. Existing team permissions are
used without escalation. Owners/all-repository teams retain their normal default
access and cannot be selected. Rule edits, disabled/deleted rules and repository
renames do not revoke or recompute existing grants. Team display-name changes are
safe because rules store stable team IDs. Deleting a team removes its references;
a new team with the same name does not inherit the old rule.

Up to 32 rules, 512 bytes per pattern and 64 teams per rule are supported.
Go/RE2 avoids backtracking expressions; unsupported look-around/backreferences are
rejected. Mutations require organization-owner authority and normal CSRF checks;
foreign organization team IDs are rejected, and stale revisions return 409.
Rules use `app_state` key `org-repo-team-rules-{orgID}` with optimistic revision
checks, without changing schema or Git/LFS paths. Include these records in trial
data export and map organization/team IDs during a later native takeover. An old
binary ignores rules but retains existing grants; rollback does not revoke access.

Native Express233 CI upload is documented in [the upload guide](express233-upload.md).

Fresh SQLite installs let the complete upstream models create Project/Team primary keys before adding extension-only columns; upgrades keep the existing tables and extension version 2 unchanged.

## One desktop interface and organization notes

Organizations display their stable `name` together with a distinct `full_name`
note. Set the note in organization settings or PATCH the existing organization API
with `full_name`; it does not rename the organization, repositories, clone URLs or
team IDs. Empty or identical notes are omitted. The homepage directory searches
both the identifier and note, supports Escape to clear, and shows an empty-result
message. Export trial organization notes before native takeover and map by name.

Repository code, files, issues, pull requests, Actions, wiki, releases, boards and
settings use the complete server-rendered interface with the same navigation.
The experimental API-driven overlay no longer mounts or hides the page; there is
no classic/new interface switch. Existing URLs, including `classic=1`, continue
to show the same interface. API authentication and permission checks are unchanged;
web sessions do not acquire extra API privileges to render the page.

Desktop navigation is a compact, scrollable rail from 1200px, with accessible
labels and a persistent collapse control. The header and repository content share
the same left alignment. Administration uses 14px text/forms, 13px section labels
and 16px headings; the current navigation group starts expanded and other groups
remain available on demand. The navigation rail scrolls independently. Compactness
comes from spacing rather than tiny text. Storage,
schema and backend operation contracts are unchanged. Rollback is the previous
binary and its embedded templates/assets, preserving the existing data.

## Production takeover and visible contribution history

After explicit operator approval, stop both writers and preserve the original
binary, configuration/secrets, SQLite snapshot and SSH authorization file.
Migrate the original database in place, retaining every account/password hash,
credential, permission, repository ID and configured storage path. Map trial-only
notes, links, team labels and task identities by stable names. Do not copy the
trial runtime-state over the source runtime-state. Keep the original SSH forced
command executable path as a compatibility entry point to the accepted GitSSS
binary, so existing keys and hooks keep working without repository rewrites.

Use one production service and database: canonical HTTP port 3000, optional
5030 additional listener. Disable the previous services at boot and retain their
backups. Resume only the previously active external runners and verify their
existing identities. Keep the managed runner disabled unless it is explicitly
enabled with a dedicated label that no existing workflow targets. During isolated rehearsal keep SSH authorization writes disabled. Once the
source is stopped and GitSSS exclusively owns production, preserve the original
system-SSH key management mode so adding/removing Git keys continues to work.
Operating-system credentials, port and SSH host keys remain unchanged.

Contribution history starts open on personal/organization dashboards when
heatmaps are enabled. Team selection reloads the matching team's heatmap along
with the activity view, cancels superseded charts and keeps localized errors
readable. Narrow desktop columns scroll the calendar horizontally instead of
shrinking its labels. Existing activity visibility rules and endpoints apply.

Administration shows data-volume capacity, used and available space, plus
measured Git/LFS file bytes. These are distinct: volume use includes other apps,
and repository estimates can count shared LFS more than once. The inventory
shows its measurement time, localized categories and an explicit lower-bound
warning on truncation/read errors. CACHE_TTL and MAX_ENTRIES remain the existing
storage_awareness settings. No storage layout, provider, object bytes or API
payload fields change; rollback is the previous binary and embedded assets.

### Actions defaults and managed runner selection

When a native Gitea source omits DEFAULT_ACTIONS_URL, explicitly preserve its
upstream `github` default. GitSSS fresh installs use `self`; inheriting that
fresh-install default can make an external runner fetch a local action mirror
without credentials. Do not relax site sign-in or private repository access to
fix action resolution. Explicit source values continue to take precedence.
See the [upstream action download rules](https://docs.gitea.com/usage/actions/faq/).

A managed runner can coexist with original external runners using an exclusive
label, for example BUILTIN_RUNNER_LABELS=gitsss-native:host and capacity 1. New
jobs opt in with runs-on: gitsss-native; original labels and credentials stay
unchanged. The managed runner provides bundled compatible checkout, setup-node,
cache and artifact actions, including artifact v4+. Upstream GitHub artifact SDKs
on external runners can reject non-GitHub servers; use a supported Gitea action
there or opt in to the native runner. Neither option runs GitHub-hosted workflows.

Native binary installations provide their own Node.js 24 on the service PATH;
the bundled setup-node action keeps that runtime's resolved directory on Linux,
macOS and Windows. It does not download or replace a system toolchain. Use a
private runtime prefix for GitSSS when original external runners need a different
Node.js version.

Administration uses 15px body, table and sidebar text, 14px supporting text and
buttons, and 17px section headings. A scrollable 240px sidebar keeps settings
accessible while preserving a compact desktop layout. Disk totals use four
columns rather than stretching the overview vertically.

Role and token scope are separate. A site administrator's read-only inventory
token must stay read-only. Create a temporary scoped operator token with the
local CLI for disposable write acceptance and revoke it after cleanup; never
reset a source account password to make an acceptance script pass.
