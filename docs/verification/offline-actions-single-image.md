# Offline Actions single-image verification

Disconnected workflow verified on 2026-07-16 with `gitsss/gitsss:dev` image ID
`sha256:a9462bc06a5f5511a66b3236f661011814ba371f9a044b3440703ae18121ddc3`.

## Runtime

- Ubuntu 24.04, linux/amd64
- only the public `gitsss` command is installed
- Node.js 24.18.0, npm 11.16.0, pnpm 10.33.0
- Go 1.26.5 and the Ubuntu build toolchain
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

Later product rebuilds retained the same embedded Actions tree. The final image
and its built-in runner/runtime registration are recorded in
`platform-smoke-2026-07-16.md`; this document preserves the disconnected
workflow evidence and its exact image identity.
