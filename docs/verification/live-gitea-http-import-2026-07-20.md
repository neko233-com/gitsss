# Live Gitea HTTP import verification

Date: 2026-07-20

## Isolation

- Source: `http://git.example.internal:3000`, Gitea 1.26.1.
- Target: a disposable `gitsss/gitsss:acceptance` container and fresh volume.
- The source was accessed through read-only HTTP API and Git HTTP operations.
- Temporary source and target access tokens were deleted after verification.

## Imported scope

- 20 accounts, including administrator, active, restricted, and login state.
- The `engineering` organization, seven teams, 30 team memberships, nine team
  repository grants, and 18 public memberships.
- `engineering/service-api`.
- `engineering/config-service`.

The importer created 20 mode-`0600` temporary credential records and marked
imported accounts for password change. HTTP migration does not copy password
hashes, TOTP secrets, SSH keys, or access tokens.

## Repository evidence

| Repository | Source and target `main` object ID |
| --- | --- |
| `service-api` | `dbdf0012eb66d7c78e645165b64cac13515bb806` |
| `config-service` | `946249db6dcff68f174703425351673c724b8728` |

Fresh clone, temporary branch push, and branch deletion succeeded against the
target. The temporary branch was removed.

## Port cutover evidence

The PowerShell installer started the imported volume on host port 3001, then
recreated GitSSS on host port 3000 using the same installation directory and
volume. Health checks passed on both ports, and
`service-api/main` remained
`dbdf0012eb66d7c78e645165b64cac13515bb806`.

The disposable target container was removed after the rehearsal. The migrated
candidate volume is retained only until final release verification.
