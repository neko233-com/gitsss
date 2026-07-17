# Offline installation

gitsss runs from one image and one persistent Docker volume. Container HTTP port stays `3000`; host port is configurable.

Linux/macOS:

```sh
./install/install.sh \
  --image-archive ./gitsss-image.tar \
  --install-dir /opt/gitsss \
  --port 3001 \
  --root-url http://git.example.internal:3001/
```

PowerShell:

```powershell
.\install\install.ps1 `
  -ImageArchive .\gitsss-image.tar `
  -InstallDir C:\gitsss `
  -Port 3001 `
  -RootUrl http://git.example.internal:3001/
```

Re-run the same command with port `3000` after migration validation. Keep `InstallDir` and `DataVolume` unchanged. Docker recreates the container and retains all data.

Installers never pull images. Load an offline archive or pre-load the configured image first.

An empty data volume receives the `root` administrator with initial password
`root`; the account must change its password at first login. Override it with
`-InitialAdminUser` / `-InitialAdminPassword` in PowerShell or
`--initial-admin-user` / `--initial-admin-password` in the shell installer.

By default the installer enables silent autostart after Docker is available:

- Windows registers the hidden `GitSSS Background` task at user logon. When
  local task registration is prohibited, it uses a hidden `wscript` launcher
  in the current user's Startup folder instead.
- Linux installs and enables `gitsss.service` in systemd.
- macOS installs a user `launchd` agent.

All background startup output is written below `INSTALL_DIR/logs`; no command
window is opened. Use `-NoAutostart` in PowerShell or `--no-autostart` in the
shell installer to opt out.
