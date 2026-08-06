# gitsss

内网、自托管 Git 协作平台。Gitsss 以单个离线 Docker 镜像运行，包含仓库、Issue、Pull Request、Actions、Packages、Release 与 Git LFS，并保留成熟 Git 服务的兼容语义。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 前置条件

- Docker Engine 或 Docker Desktop 已启动，并可执行 `docker`、`docker compose`。
- 离线镜像包，例如 `gitsss-image.tar`；也可预先执行 `docker load --input gitsss-image.tar`。
- Windows 使用 PowerShell 5.1+；Linux/macOS 使用 Bash。

安装器从离线包加载镜像，且始终使用 `--pull never`，不会访问镜像仓库。

## 默认配置

| 项目 | 默认值 |
| --- | --- |
| Web 端口 | `3000` |
| SSH 端口 | `2222` |
| 默认地址 | `http://localhost:3000/` |
| 初始管理员 | `root` |
| 初始密码 | `root` |
| 最小密码长度 | `4` |
| Docker 镜像 | `gitsss/gitsss:latest` |
| 数据卷 | `gitsss-data` |

初始管理员仅在空数据卷创建，首次登录必须修改密码。生产环境请在安装时覆盖默认凭据。

## 安装

### Windows PowerShell

在仓库根目录运行。将 `192.168.110.42` 换成用户可访问此服务的服务器 IP 或域名。

```powershell
.\install\install.ps1 `
  -ImageArchive .\gitsss-image.tar `
  -InstallDir C:\gitsss `
  -Port 3000 `
  -RootUrl http://192.168.110.42:3000/ `
  -InitialAdminUser root `
  -InitialAdminPassword 'change-this-password'
```

安装完成后访问 `RootUrl`。脚本创建 `C:\gitsss\.env`、启动容器，并设置隐藏的登录自启动任务。

### Linux / macOS Shell

```sh
./install/install.sh \
  --image-archive ./gitsss-image.tar \
  --install-dir /opt/gitsss \
  --port 3000 \
  --root-url http://192.168.110.42:3000/ \
  --initial-admin-user root \
  --initial-admin-password 'change-this-password'
```

Linux 默认安装并启用 `systemd` 服务；macOS 默认安装用户级 `launchd` Agent。无自启动需求时，追加 `--no-autostart`；仅写入安装文件、不启动服务时，追加 `--no-start`。

## 日常操作

安装目录内保存 `compose.yaml`、`.env` 与日志。将以下 `INSTALL_DIR` 换成实际安装路径。

### 查看状态与日志

```sh
docker compose --project-directory INSTALL_DIR ps
docker compose --project-directory INSTALL_DIR logs --follow
```

PowerShell：

```powershell
docker compose --project-directory C:\gitsss ps
docker compose --project-directory C:\gitsss logs --follow
```

### 更新

更新不会迁移或删除 `gitsss-data`；必须复用原安装目录和数据卷。获得新版离线镜像包后，以原参数重跑安装器。它会加载镜像、更新安装目录文件、重建容器，并保留数据。

```powershell
.\install\install.ps1 `
  -ImageArchive .\gitsss-image-new.tar `
  -InstallDir C:\gitsss `
  -DataVolume gitsss-data `
  -Port 3000 `
  -RootUrl http://192.168.110.42:3000/
```

```sh
./install/install.sh \
  --image-archive ./gitsss-image-new.tar \
  --install-dir /opt/gitsss \
  --data-volume gitsss-data \
  --port 3000 \
  --root-url http://192.168.110.42:3000/
```

不要在已有数据卷的更新命令中传入新的管理员凭据；现有账户密码不会被重置。

### 修改端口

同样重跑安装器，保持 `InstallDir` / `--install-dir` 和数据卷不变，仅修改端口与外部地址。示例：改为 `3001`。

```powershell
.\install\install.ps1 -InstallDir C:\gitsss -DataVolume gitsss-data -Port 3001 -RootUrl http://192.168.110.42:3001/
```

```sh
./install/install.sh --install-dir /opt/gitsss --data-volume gitsss-data --port 3001 --root-url http://192.168.110.42:3001/
```

### 卸载

先停止并移除容器。下列命令保留 `gitsss-data`，可供重新安装或备份恢复。

```powershell
docker compose --project-directory C:\gitsss down
Unregister-ScheduledTask -TaskName 'GitSSS Background' -Confirm:$false -ErrorAction SilentlyContinue
Remove-Item "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\GitSSS Background.vbs" -Force -ErrorAction SilentlyContinue
```

```sh
docker compose --project-directory /opt/gitsss down
# Linux：sudo systemctl disable --now gitsss.service && sudo rm /etc/systemd/system/gitsss.service && sudo systemctl daemon-reload
# macOS：launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.gitsss.autostart.plist"; rm -f "$HOME/Library/LaunchAgents/com.gitsss.autostart.plist"
```

若要永久删除所有仓库、用户、Issue 与 Actions 数据，先确认备份可用，再执行：

```sh
docker volume rm gitsss-data
```

此操作不可恢复。使用非默认数据卷时，将命令中的 `gitsss-data` 替换成实际名称。

## 开发

```sh
task help
task fmt
task lint:go
task lint:js
task test
```

## 相关文档

- [HTML 文档站](docs/site/index.html)
- [离线生产策略](docs/operations/offline-production.md)
- [Gitea HTTP 迁移](docs/operations/gitea-http-import.md)
- [Mac mini M4 部署](docs/operations/mac-mini-m4.md)
- [生产就绪审计](docs/verification/production-readiness-2026-07-16.md)

## License

[MIT](LICENSE)
