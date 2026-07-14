# Git Smart HTTP v1

gitsss hosts bare repositories below `GITSSS_REPOSITORY_ROOT` and exposes Git protocol v0/v2 at `/git/{repository}.git`. Repository metadata is stored in embedded SQLite; paths are never accepted from API callers or returned in API responses.

## Create and use a repository

`POST /api/v1/repositories` requires `repositories:write` and accepts a lowercase slug, display name, and default branch:

```json
{
  "id": "art-team",
  "name": "Art Team",
  "default_branch": "main"
}
```

Creation is no-replace. gitsss initializes a new bare repository, persists its catalog record, and writes an audit entry. An invalid branch, path-shaped slug, duplicate catalog entry, existing target directory, persistence failure, or audit failure prevents publication.

Clone requires `repositories:clone`; fetch uses the same scope. Reference advertisement and pack transfer use `git upload-pack --stateless-rpc` without a shell:

```text
git clone https://gitsss.example/git/art-team.git
```

Push and receive-pack advertisement require `repositories:push`. Git credential helpers may supply any non-empty username and use the custom Token as the password. Automation may send an exact `Authorization: Bearer TOKEN` header. Tokens in URLs and malformed `BearerTOKEN` headers are rejected.

## Security boundary

- Only repository IDs already present in the catalog can reach a Git subprocess.
- API and Git permissions use separate least-privilege scopes.
- Every advertisement and RPC is authorized and audited before Git runs; receive results are audited as accepted or failed.
- `Git-Protocol` is bounded and rejected when it contains control characters.
- Smart HTTP command responses and diagnostics are bounded; upload pack data streams with request cancellation.
- The service never invokes a shell and never accepts a filesystem path, executable, or Git service name from the request.

This transport covers Git objects and refs. LFS transfer, branch policies, repository-scoped membership, partial-clone policy, and SSH transport remain separate services so they can enforce their own quotas and authorization rules.
