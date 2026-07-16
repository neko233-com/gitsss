# Offline installation

gitsss runs from one image and one persistent Docker volume. Container HTTP port stays `3000`; host port is configurable.

Linux/macOS:

```sh
./install/install.sh \
  --image-archive ./gitsss-image.tar \
  --install-dir /opt/gitsss \
  --port 3001 \
  --root-url http://192.168.110.42:3001/
```

PowerShell:

```powershell
.\install\install.ps1 `
  -ImageArchive .\gitsss-image.tar `
  -InstallDir C:\gitsss `
  -Port 3001 `
  -RootUrl http://192.168.110.42:3001/
```

Re-run the same command with port `3000` after migration validation. Keep `InstallDir` and `DataVolume` unchanged. Docker recreates the container and retains all data.

Installers never pull images. Load an offline archive or pre-load the configured image first.
