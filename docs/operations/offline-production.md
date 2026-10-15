# Offline production policy

gitsss starts and serves repositories without contacting an external update,
runner, or Action service.

## Update manifests

The inherited external Gitea update endpoint is removed. The update checker is
disabled by default and does not contact GitHub during startup, including when
gitsss takes over a Gitea configuration that enabled the old checker without
declaring an endpoint.

An administrator can check GitHub Releases from `/-/admin/updates` or
`gitsss update check`. The check probes `api.github.com` and the configured
HTTP mirrors in parallel, prefers the official manifest when GitHub answers,
and downloads through the lowest-latency healthy mirror with the remaining
mirrors as fallbacks. An empty mirror list uses `https://ghfast.top`. Asset
downloads are kept only after the sha256 matches the selected manifest. The
command writes a file; it does not replace the running server.

`CHECK_GITHUB` defaults to false. Leaving both `CHECK_GITHUB` unset and
`HTTP_ENDPOINT` empty performs no network request even if the task is enabled.
An intranet manifest remains optional:

```ini
[cron.update_checker]
ENABLED = true
CHECK_GITHUB = true
GITHUB_REPO = neko233-com/gitsss
MIRRORS = https://ghfast.top
PROBE_TIMEOUT = 4s
HTTP_ENDPOINT = http://updates.intranet.example/gitsss/version.json
```

Requests reject non-success responses and limit the manifest to 1 MiB. A GitHub
release needs `tag_name`. The legacy intranet document needs `latest.version`.

```sh
gitsss update check --json
gitsss update check --mirror https://mirror.example --save
gitsss update download --asset gitsss-linux-amd64.tar.gz --output /tmp
```

## Actions

The embedded runner resolves the supported Actions from the image. Unknown
remote Actions fail closed. Node.js, Go 1.27.2, Go 1.26.5, Go 1.25.5, npm, pnpm, checkout,
cache, and artifact runtimes are shipped in the same Ubuntu 24.04 image. Git
LFS is also installed for HTTP migration and normal repository use. Server
startup and supported workflow execution do not require an external package or
Action registry.

Image builds accept `NPM_REGISTRY`, `GOPROXY`, `UBUNTU_MIRROR`, and
`UBUNTU_PORTS_MIRROR` build arguments. Point them at company-controlled mirrors
when release builders cannot access public registries. `UBUNTU_MIRROR` serves
amd64 packages and `UBUNTU_PORTS_MIRROR` serves arm64 packages. These settings
affect build dependency acquisition only; they are not configured in runtime.

The server-owned runner is started in the `gitsss web` process by default and
uses the existing Actions API. Host-mode jobs need no separate runner binary or
Docker socket; this is also the default path for Docker-command-compatible
runtimes such as vbox. An external runner fleet remains supported by setting
`[actions] BUILTIN_RUNNER_ENABLED = false`. Docker-backed labels use the host
Docker socket only to create job containers, and this fallback requires a real
Docker-compatible socket. The socket is not mounted into jobs. Each job starts
a private Docker daemon, so repositories do not share
Docker images, containers, networks, or volumes. The runner removes
interrupted `GITEA-ACTIONS-TASK-*` resources on startup.

## Remote handoff validation

The repository includes read-only validation scripts for an internal target.
They check health, Swagger, repository access, smart HTTP refs, Git LFS, and
workflow discovery without pushing or changing remote data. Supply the secret
through `GITSSS_REMOTE_PASSWORD` or the hidden PowerShell prompt:

```powershell
$env:GITSSS_REMOTE_PASSWORD = Read-Host "Remote password" -AsPlainText
.\scripts\verify-remote.ps1 -BaseUrl http://192.168.110.42:3000
```

```bash
GITSSS_REMOTE_PASSWORD='...' ./scripts/verify-remote.sh
```

The scripts use `.gitea/workflows` and `.github/workflows` as compatible
workflow paths and perform no push, tag, or release operation.

Storage awareness is available at `/-/admin/storage`. It reports volume
health, classified storage, and Git/LFS bytes per repository. Manual cleanup
uses a short-lived scan token and explicit confirmation. It only deletes
expired entries below managed temp, Actions work, and Actions cache roots.
Repositories, LFS objects, databases, and unknown paths are excluded.

## Releases

Local release targets produce only `gitsss-*` binary, source, compressed, and
checksum filenames. The cross-build tool is pinned, and the Go toolchain line is
fixed to Go 1.27.2. The removed legacy upgrade helper cannot download or install
an external server binary. Upgrades replace that binary only. Bare repositories,
Git objects, refs, and LFS object bytes stay in place; new metadata belongs in
`gitsss/layout.json` and `gitsss/extensions/`.

Run <code>make release-native VERSION=1.2.0</code> on the release host. It
cross-compiles with the local Go toolchain, two packages at a time by default,
and compresses with <code>xz -6</code> in parallel. Set <code>RELEASE_JOBS</code>,
<code>XZ_LEVEL=9</code>, or <code>RELEASE_GOGIT=1</code> when a release needs more
parallelism, smaller archives, or the Windows Go-Git binaries. <code>make release</code>
remains the Docker xgo path and now uses the same parallel xz setting.

The host must provide Go, Make, Node.js, and pnpm. Docker is required only for
<code>make release</code> and for building the container image. Dependency
downloads are build-time only; published binaries, image archives, and
installers remain self-contained for offline deployment.

No release is published until local multi-platform, checksum, configured
signature, migration, backup, and rollback gates have passed.
