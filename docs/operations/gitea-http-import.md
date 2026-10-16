# Gitea HTTP account and repository import

`gitsss import-gitea-http` imports accounts, selected repositories, and their
organization/team/repository access by using only Gitea HTTP APIs and Git HTTP.
It never needs SSH, the source host filesystem, or the source Docker volume.

## Safety boundary

- Keep source Gitea running and unchanged.
- Use a source site-administrator token, or that administrator's username and
  password. Read the secret from a mode-`0600` file. Do not pass it as a shell
  argument.
- Use a separate target URL during import. Source and target URLs must differ.
- Dry-run is default and performs no target write.
- Source HTTP operations are GET-only. Git clone/LFS reads use HTTP(S).
- Redirects, proxy environment variables, cross-origin clone URLs, embedded
  clone credentials, and unknown collaborator permissions are rejected.
- Tokens and passwords are read from files and redacted from errors. Set files to mode `0600`.

HTTP cannot export password hashes, TOTP secrets, API tokens, or SSH private
state. Imported local users receive random temporary passwords, are marked to
change password at first login, and are written to an exclusive mode-`0600`
JSONL credentials file. Distribute credentials through an approved channel.

## Scope

Imported:

- all individual accounts and account state visible to the admin API;
- organizations owning selected repositories;
- owner and custom teams, unit permissions, members, and repository grants;
- public organization membership and direct repository collaborators;
- selected repository metadata, branches, tags, notes, default branch, archive
  state, complete Git objects, and Git LFS objects;
- labels, milestones, issues, issue comments, pull requests, review threads,
  release notes, release attachment bytes up to 512 MiB, wiki pages, and
  generic packages linked to those repositories.

npm, Maven, PyPI, Helm, Cargo, Debian, Composer, Alpine, NuGet, RubyGems,
and container images are copied with their own upload protocols. Debian and
Alpine use the file `composite_key` (`distribution|component` or
`branch|repository|architecture`) exposed by the package file API. Conan and
Swift stay listed only: their publish APIs are multi-request handshakes, not
one package file.

Not imported: webhooks,
Actions secrets, access tokens, SSH keys, TOTP secrets, OAuth grants, or
password hashes. Merged pull requests are closed on the target; the merge
commit is not replayed. Issue numbers match the source only when the target
tracker is empty and the source numbers have no gaps. An existing target
repository is not overwritten, and its issues are left untouched.

## Rehearsal

Run target temporarily on another port or host, for example source port `3000`
and target port `5030`. Create site-administrator tokens on both systems and
store them in protected files:

```sh
chmod 600 source.token target.token

gitsss import-gitea-http \
  --source-url http://git.example.internal:3000 \
  --source-token-file source.token \
  --target-url http://127.0.0.1:5030 \
  --target-token-file target.token \
  --repo engineering/service-api \
  --repo engineering/config-service
```

Username and password work in place of the token files. Keep the password files
out of the shell history:

```sh
chmod 600 source.password target.password

gitsss import-gitea-http \
  --source-url http://git.example.internal:3000 \
  --source-user root \
  --source-password-file source.password \
  --target-url http://127.0.0.1:5030 \
  --target-user root \
  --target-password-file target.password \
  --all-repos
```

Review the JSON plan. It must name only expected accounts, organization/RBAC,
and repositories. Dry-run does not create target records.

## Apply

The initial import is a snapshot. Keep Gitea running and unchanged during the
rehearsal. Git updates made after this snapshot can be copied with the
Git-only sync mode below; collaboration metadata is not continuously
replicated. Use a separate target data directory:

```sh
gitsss import-gitea-http \
  --source-url http://git.example.internal:3000 \
  --source-token-file source.token \
  --target-url http://127.0.0.1:5030 \
  --target-token-file target.token \
  --repo engineering/service-api \
  --repo engineering/config-service \
  --timeout 30m \
  --apply \
  --credentials-file imported-users.jsonl
```

Default conflict policy is `fail`. It prevents accidental adoption or
overwrite of target identities and repositories. `--conflict skip` is only for
controlled recovery of a known partial run; existing repositories are verified
but not overwritten, and their issues and pull requests are not copied.

Acceptance requires matching selected refs, Git object IDs, default branches,
LFS content, organization/team membership, team repository grants, direct
collaborator permissions, account state, labels, milestones, issues, comments,
pull-request head and base, and mode `0600` credentials output.

## Incremental Git synchronization

After initial import, repeat this command to fetch changed Git objects and LFS
content for existing repositories:

```sh
gitsss import-gitea-http \
  --source-url http://git.example.internal:3000 \
  --source-token-file source.token \
  --target-url http://127.0.0.1:5030 \
  --target-token-file target.token \
  --repo engineering/service-api \
  --repo engineering/config-service \
  --sync-existing-git --apply
```

This mode discovers only repository metadata and Git refs. It incrementally
fetches branches, tags, notes, and LFS objects using a persistent bare cache in
the current user's cache directory; use `--mirror-cache-dir` to select a cache
location with enough free space. Source ref deletions are pruned from the
target. Updates are atomic. A private per-target checkpoint in the cache rejects
independent destination ref changes, including destination-only branches, before
writing. Reuse the same cache between runs; a missing checkpoint can initialize
only when target refs already match the source. Non-fast-forward updates fail
without force-overwriting them. The cache avoids repeatedly downloading existing Git
objects but occupies space proportional to the mirrored Git data.

It does not synchronize issues, pull requests, repository settings, access
permissions, releases, wiki, or packages after initial import. Keep one side
authoritative during the evaluation period. Avoid edits on both servers.

## Two-port evaluation and later cutover

During the evaluation period, keep Gitea on `3000` and GitSSS on `5030`.
After acceptance:

1. Run a final Git sync and retain the source rollback data.
2. Stop source Gitea to release port `3000`.
3. From GitSSS at `5030`, add port `3000` from `/-/admin/takeover` or `POST /api/v1/admin/listen-port/alias` with `{"port":"3000"}`.
4. GitSSS keeps serving `5030` and now also serves `3000`; the public `ROOT_URL` points to `3000`.
5. Keep the Gitea data intact until post-cutover verification is complete.

The listener alias only helps GitSSS claim the source port after the source service
has been stopped. Do not add it while Gitea still owns port `3000`; binding will
fail and the original service remains untouched.

The stopped-volume takeover helper remains an optional full-fidelity fallback
when password hashes, TOTP, SSH keys, wiki, releases, or other database state
must be preserved. Never run it against a live source volume.
