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

Not imported: non-generic package bytes (npm, Maven, container, and the other
package protocols), webhooks,
Actions secrets, access tokens, SSH keys, TOTP secrets, OAuth grants, or
password hashes. Merged pull requests are closed on the target; the merge
commit is not replayed. Issue numbers match the source only when the target
tracker is empty and the source numbers have no gaps. An existing target
repository is not overwritten, and its issues are left untouched.

## Rehearsal

Run target temporarily on another port or host, for example source port `3000`
and target port `3001`. Create site-administrator tokens on both systems and
store them in protected files:

```sh
chmod 600 source.token target.token

gitsss import-gitea-http \
  --source-url http://192.168.110.42:3000 \
  --source-token-file source.token \
  --target-url http://127.0.0.1:5300 \
  --target-token-file target.token \
  --repo poke-game/data-collect-server-go \
  --repo poke-game/config233-editor-server
```

Username and password work in place of the token files. Keep the password files
out of the shell history:

```sh
chmod 600 source.password target.password

gitsss import-gitea-http \
  --source-url http://192.168.110.42:3000 \
  --source-user root \
  --source-password-file source.password \
  --target-url http://127.0.0.1:5300 \
  --target-user root \
  --target-password-file target.password \
  --all-repos
```

Review the JSON plan. It must name only expected accounts, organization/RBAC,
and repositories. Dry-run does not create target records.

## Apply

Prevent pushes to the selected source repositories during the short apply and
verification window. The importer is a one-shot snapshot, not continuous
replication. Use a fresh target for final cutover:

```sh
gitsss import-gitea-http \
  --source-url http://192.168.110.42:3000 \
  --source-token-file source.token \
  --target-url http://127.0.0.1:5300 \
  --target-token-file target.token \
  --repo poke-game/data-collect-server-go \
  --repo poke-game/config233-editor-server \
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

## Cutover to port 3000

After acceptance:

1. Keep source rollback data intact.
2. Stop source Gitea only during the approved cutover window.
3. Change gitsss published HTTP port/root URL from the temporary port, usually `5300`, to `3000`.
4. Start gitsss and verify `/api/healthz`, web login, clone, push, LFS, issues, and RBAC.
5. Remove source tokens and securely archive or destroy temporary credentials.

The stopped-volume takeover helper remains an optional full-fidelity fallback
when password hashes, TOTP, SSH keys, wiki, releases, or other database state
must be preserved. Never run it against a live source volume.
