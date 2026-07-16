# Platform smoke verification

Date: 2026-07-16

## linux/amd64

- Image: `gitsss/gitsss:dev`
- Image ID: `sha256:c0669bd180e9fead3c1760c916e06b0a38cf1bdf2c82e7ec8e11aecb8ebe9d42`
- Uncompressed image size: 338,529,591 bytes
- Base runtime: Ubuntu 24.04
- Health: `{"status":"pass","description":"gitsss"}`
- CLI: `/usr/local/bin/gitsss`; no `gitea` or public runner command
- Migration CLI: `gitsss import-gitea-http`
- Data root: `/data/gitsss`
- HTTP port: 3000
- Toolchain: Git LFS 3.4.1, Node.js 24.18.0, npm 11.16.0,
  pnpm 10.33.0, Go 1.26.5
- Root `compose.yaml` health reached `healthy` with HTTP 200 on port 3000
- Compose applied `no-new-privileges`, disabled the update checker, mounted one
  `/data` volume, and exposed only the `gitsss` public command

## linux/arm64

- Temporary image ID: `sha256:b9e2fd469f2bd7435bd2817fe6ad0c66ea8b34aa06af8944753b7777c9806bb4`
- Uncompressed image size: 324,364,013 bytes
- Built as a native `linux/arm64` Ubuntu 24.04 image with CGO and SQLite
- Executed under arm64 emulation and reported `aarch64`
- HTTP health returned 200 on the mapped port
- Built-in Actions runner registered as `arch=arm64`, `os=linux`
- Migration CLI, Git LFS 3.4.1, Node.js 24.18.0, pnpm 10.33.0, and
  Go 1.26.5 executed inside the arm64 container
- CLI, data root, and port matched the amd64 image

The temporary arm64 tag, both smoke containers, and both smoke volumes were
removed after verification. The only retained gitsss product image tag is
`gitsss/gitsss:dev`.

## Local gates

- `make fmt`
- Go lint: `0 issues`
- Full `make test-backend` on the Ubuntu 24.04 runtime with Git 2.43
- HTTP migration package race test and `go vet`
- Feishu notification-template limits, missing-key behavior, fallback, race
  test, and `go vet`
- Vitest: 35 files and 92 frontend tests passed with embedded pnpm 10.33
- ESLint with zero warnings and Vue TypeScript checking
- Linux SQLite tests for storage awareness, webhooks, issue services, and
  repository web routes
- Built-in Action materialization test in the embedded runner module

## Browser UI

The Codex in-app browser opened the final amd64 container through localhost
port `33004`, not the live Gitea service. Verified:

- public page title and branding were `gitsss`;
- the preserved gitsss Light skin rendered on the Gitea-compatible layout;
- authenticated navigation exposed issues, pull requests, milestones,
  repositories, organizations, notifications, and creation menus;
- the dashboard rendered the contribution graph and repository/organization
  panels;
- the browser console contained no warnings or errors.

The browser smoke container and volume were removed after verification. No tag
or release is produced from this verification.
