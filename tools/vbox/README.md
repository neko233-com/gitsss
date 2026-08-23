# gitsss on GitHub neko233-com/vbox

The `vbox` referenced by this project is the local checkout at
`C:\tools\vbox`, published as
`github.com/neko233-com/vbox`. It replaces the `docker` command with a
Docker-compatible OCI runner; it is not Oracle VirtualBox and not Vagrant.

Use the helper from the gitsss repository root on Windows PowerShell:

```powershell
# Confirm the local GitHub neko233-com/vbox build and its WSL2 diagnostics
.\tools\vbox\gitsss.ps1 check

# Pull the GitSSS image without Docker
.\tools\vbox\gitsss.ps1 pull

# Or load the offline archive produced by the GitSSS release process
.\tools\vbox\gitsss.ps1 load -ImageArchive .\gitsss-image.tar

# Install/run GitSSS with persistent host data and verify HTTP health
.\tools\vbox\gitsss.ps1 install
.\tools\vbox\gitsss.ps1 verify
```

The helper uses `vbox run` with the GitSSS image's native entrypoint,
`127.0.0.1:3000` for HTTP, `127.0.0.1:2222` for Git SSH, and a persistent
`%LOCALAPPDATA%\gitsss\vbox-data` directory mounted at `/data`. It does not
invoke Docker or Compose. Use `-KeepRunning` with `verify` to leave the
service running, `logs` to inspect it, and `remove` to remove only the
container while retaining the data directory.

The installer also places `docker.exe` beside `vbox.exe`, so existing
Dockerfiles, image names, volume mappings, and common Compose configuration can
continue to use the `docker` command without changing the file contents. For
Docker clients that use the Engine API, run `vbox server start --docker-api`
and set `DOCKER_HOST=tcp://127.0.0.1:2375`. vbox itself does not start or
depend on Docker Desktop or a Docker daemon.

The helper requires a built `vbox.exe` either on `PATH`, at
`C:\tools\vbox\target\release\vbox.exe`,
or at `%LOCALAPPDATA%\vbox\bin\vbox.exe`. Build/install that project first:

```powershell
Set-Location 'C:\tools\vbox'
.\install.ps1
```

`vbox` must support `--volume HOST:CONTAINER`; this is required to preserve
GitSSS data between runs. It also supports `vbox load --input` for Docker
`save` archives, so an offline GitSSS image can be verified without Docker or
a registry login. The existing Docker installer and Compose contract remain
unchanged for production image deployments.
