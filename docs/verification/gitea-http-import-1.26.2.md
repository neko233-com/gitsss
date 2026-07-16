# Gitea 1.26.2 HTTP import verification

Date: 2026-07-16

## Isolation

- Source: official `gitea/gitea:1.26.2` in a disposable Docker network, volume,
  and localhost-only port `33001`.
- Target: `gitsss/gitsss:dev`, Ubuntu 24.04, localhost-only port `33002`.
- Live `git.example.internal:3000` was not accessed, stopped, modified, or mounted.
- Test source image, containers, volumes, and network were removed by the test.
- Retained product image: `gitsss/gitsss:dev`, image ID
  `sha256:c0669bd180e9fead3c1760c916e06b0a38cf1bdf2c82e7ec8e11aecb8ebe9d42`.

## Fixture

- Accounts: `source-admin`, `alice`, `bob`, `charlie`.
- Organization: `engineering`.
- Team: `Developers`, `repo.code=write`, members `alice` and `bob`.
- Direct collaborator: `charlie`, read permission on
  `engineering/service-api`.
- Repositories:
  - `engineering/service-api`
  - `engineering/config-service`
- Git refs: five selected branch/tag refs across both repositories.
- LFS: one 16 KiB object in `config-service`.

## Results

Dry-run discovered four users, one organization, and two repositories while the
target remained unchanged. Apply reported:

```json
{
  "users_created": 4,
  "organizations_created": 1,
  "teams_created": 1,
  "teams_reused": 1,
  "repositories_created": 2,
  "team_memberships_applied": 3,
  "team_repositories_applied": 4,
  "collaborators_applied": 1,
  "public_memberships_applied": 2,
  "refs_verified": 5
}
```

Verified:

- exact source/target branch, tag, and note ref/object-ID equality;
- fresh target clone smudged the LFS object to exactly 16,384 bytes;
- organization owner/custom-team membership and both repository grants;
- `charlie` retained direct read permission;
- source administrator retained administrator state;
- all four imported users were active and required password change;
- credentials JSONL was exclusive, mode `0600`, and contained four records;
- each credential was one newline-terminated JSON object under Go 1.26 JSON v2;
- source and target password hashes differed, proving hashes were not copied;
- source API migration traffic was read-only and no SSH/volume access occurred.

Repeatable test:

```sh
GITSSS_E2E_SKIP_BUILD=1 sh tests/migration/gitea-http-e2e.sh
```

Omit `GITSSS_E2E_SKIP_BUILD` when build registries and module caches are
available and the product image must be rebuilt before the fixture run.
