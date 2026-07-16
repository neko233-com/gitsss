# Actions compatibility verification

Date: 2026-07-16

## Scope

gitsss keeps the Gitea v1.26.2 Actions workflow model, scheduler, API, pages,
runner protocol, rerun behavior, cancellation behavior, and retention model.
It does not claim that every GitHub Marketplace Action can run offline.

Unknown remote Actions fail closed. Common Actions are immutable copies in the
single product image.

## Compatibility matrix

| Mechanism | Verification |
| --- | --- |
| Push and manual triggers | Gitea trigger tests retained; SQLite `workflow_dispatch` input integration passed |
| Job dependencies and conditions | SQLite `needs`, failure propagation, and `always()` integration passed |
| Matrix and job outputs | SQLite matrix output aggregation integration passed |
| Workflow and job concurrency | SQLite workflow concurrency integration passed; service concurrency tests passed |
| Secrets, variables, contexts, expressions | Gitea parser/service tests retained and passed |
| Rerun and cancellation | Gitea service rerun, validation, cancellation, and concurrency tests passed |
| Reusable workflows | Copied Gitea runner supports local and remote reusable workflows, `workflow_call`, inputs, secrets, and outputs |
| Job and service containers | Copied Gitea runner parser/executor retained; gitsss jobs run in isolated Docker containers |
| Cache | Bundled `actions/cache`; toolkit-compatible authenticated cache protocol tests passed |
| Artifacts | Bundled upload/download Actions; isolated publish workflow uploaded a persisted artifact |
| Cleanup and retention | Gitea run/artifact retention retained; stale task resources and expired managed work/cache entries are removable |

## Isolated publish proof

The `poke-game/data-collect-server-go` fixture used
`publish-express233.yml`. Run 8 completed without an external registry:

1. bundled `actions/checkout@v4`;
2. empty private Docker inventory check;
3. `FROM scratch` image build;
4. image label and file inspection;
5. bundled `actions/upload-artifact@v4`;
6. persisted 189-byte artifact.

The task received no host Docker socket. Restart cleanup removed the
interrupted task containers, networks, and volumes.

## Test environment

- Ubuntu 24.04 product image
- Git 2.43
- Go 1.26
- SQLite integration backend
- public command and hook target: `gitsss`
