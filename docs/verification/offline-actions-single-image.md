# Offline Actions single-image verification

Disconnected workflow verified on 2026-07-16 with `gitsss/gitsss:dev` image ID
`sha256:a9462bc06a5f5511a66b3236f661011814ba371f9a044b3440703ae18121ddc3`.
The production workflow compatibility case was reverified on 2026-07-20 with
`gitsss/gitsss:acceptance` image ID
`sha256:ba4129412591dd829af5d9eac011fc2040e4479402b0953a4b2590ed3c39b141`.

## Runtime

- Ubuntu 24.04, linux/amd64
- only the public `gitsss` command is installed
- Node.js 24.18.0, npm 11.16.0, pnpm 10.33.0
- Go 1.25.5, Go 1.26.5, and the Ubuntu build toolchain
- default HTTP port 3000
- persisted data root `/data/gitsss`

## Disconnected workflow

The container was disconnected from every Docker network before the runner
accepted the job. `docker inspect` reported `NetworkSettings.Networks={}`.
The built-in runner completed the following workflow with conclusion `success`:

1. `actions/checkout@v4`
2. `actions/setup-node@v7`
3. `actions/setup-go@v6`
4. Go test plus npm and pnpm execution
5. `actions/cache/save@v6` and `actions/cache/restore@v6`
6. `actions/upload-artifact@v7`
7. `actions/download-artifact@v8` and payload comparison

The same persisted `/data` volume was restarted with the final image and the
fully disconnected workflow succeeded again. This also verified the runner
startup health wait and revisioned built-in action cache after an image update.

The synthetic container, repository, credentials, and volume were removed
after verification. They are not production fixtures.

## Production workflow compatibility

The workflow `publish-express233.yml` from
`poke-game/data-collect-server-go` was reproduced without modification in the
temporary `luohaojun/test-gitsss-repo` repository. Browser dispatch and
re-run interaction completed with conclusion `success`:

1. `actions/checkout@v4` resolved from the image
2. `actions/setup-go@v5` resolved from the image
3. the exact requested Go 1.25.5 toolchain was selected
4. `go test ./...` succeeded

After the job, no `GITEA-ACTIONS-TASK-*` container, network, or volume
remained.

Later product rebuilds retained the same embedded Actions tree. The final image
and its built-in runner/runtime registration are recorded in
`platform-smoke-2026-07-16.md`; this document preserves the disconnected
workflow evidence and its exact image identity.
