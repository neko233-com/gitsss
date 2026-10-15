# gitsss

内网、自托管 Git 协作平台。Gitsss 以单个离线 Docker 镜像运行，包含仓库、Issue、Pull Request、Actions、Packages、Release 与 Git LFS，并保留成熟 Git 服务的兼容语义。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 前置条件

- Docker Engine、Docker Desktop，或可执行 `docker`、`docker compose` 的兼容运行时；使用 vbox 时不需要 Docker daemon。
- 离线镜像包，例如 `gitsss-image.tar`；也可预先执行 `docker load --input gitsss-image.tar`。
- Windows 使用 PowerShell 5.1+；Linux/macOS 使用 Bash。

安装器从离线包加载镜像，且始终使用 `--pull never`，不会访问镜像仓库。

本机容器运行使用 GitHub `neko233-com/vbox` checkout：
`C:\Users\14170\Desktop\Code\neko233-Projects\vbox`。这里的 vbox 是 OCI
容器运行器，不是 Oracle VirtualBox 或 Vagrant。它替代的是 `docker` 命令本身，默认安装
同目录的 `docker.exe` 兼容别名；Dockerfile、镜像名、卷和常用 Docker/Compose 配置无需改写，
vbox 本身不启动也不依赖 Docker Desktop 或 Docker daemon。

## 默认配置

| 项目 | 默认值 |
| --- | --- |
| Web 端口 | `5300` |
| SSH 端口 | `2222` |
| 默认地址 | `http://localhost:5300/` |
| 初始管理员 | `root` |
| 初始密码 | `root` |
| 最小密码长度 | `4` |
| Docker 镜像 | `gitsss/gitsss:latest` |
| 数据卷 | `gitsss-data` |

初始管理员仅在空数据卷创建，首次登录必须修改密码。生产环境请在安装时覆盖默认凭据。新安装默认监听 `5300`，避免和仍在运行的 Gitea 抢 `3000`。导入账号、组织和仓库后，管理员在「接管 Gitea」页面把端口热切换到 `3000`，再停止旧的 Gitea。已有配置里写明的 `HTTP_PORT` 不会被改掉。

## 内网生产能力

- 管理员可生成有时效、一次性账号邀请链接；邀请用户只需设置账号密码。
- 登录失败采用账号 + 来源地址的渐进式延迟，成功登录立即恢复，不永久锁死账号。
- 仓库页面提供 GitHub 风格主题、商业版 Git 导航布局和可折叠宽屏侧栏；窄屏自动切换为可关闭抽屉。侧栏覆盖代码、Issue、PR、Actions、Packages、任务看板、Release、Wiki、外部链接与设置，并支持拖拽/键盘指派成员与本地 WIP 限制。
- 服务进程默认内置 Actions Runner；管理员可在 Actions → Runners 查看、启动、停止、重启或禁用，外部 Runner 仍兼容。默认内置 Runner 不读取 Docker socket，vbox 等 Docker 命令兼容运行时可直接使用；只有切换到外部 Docker Runner 才需要可用 socket。
- 首页提供 Actions Worker 运营面板：运行中任务固定置顶，排队任务可搜索，已完成任务隐藏，并保留最近一次成功/失败结果与 24 小时统计。
- 游戏仓库支持 Git LFS、3D 模型、常见图片/音视频、Unity/Godot 文本资产，以及 TGA/DDS 纹理预览；无法安全解码的格式保留原始资产入口。`gitsss admin lfs-gc` 和管理页 `/-/admin/lfs-gc` 可以预览并清理未被引用的 LFS 对象，默认保留一周。
- 登录后的首页用 React 19 显示企业公告，并带一层页内 WebGL 着色背景。仓库的代码、Issue、Pull Request、Actions、Wiki、Release、看板和设置摘要也使用同一套 React 界面；地址加上 `?classic=1` 回到服务端页面。管理员在「接管 Gitea」页面发布公告。
- 仓库任务看板可以新增、改名和删除列，也可以用 JSON 保存自定义字段。
- 纯前端插件放到 `custom/public/assets/plugins/`，刷新即可生效，后端不用改。

## 安装

### Windows PowerShell

在仓库根目录运行。将 `192.168.110.42` 换成用户可访问此服务的服务器 IP 或域名。

```powershell
.\install\install.ps1 `
  -ImageArchive .\gitsss-image.tar `
  -InstallDir C:\gitsss `
  -Port 5300 `
  -RootUrl http://192.168.110.42:5300/ `
  -InitialAdminUser root `
  -InitialAdminPassword 'change-this-password'
```

安装完成后访问 `RootUrl`。脚本创建 `C:\gitsss\.env`、启动容器，并设置隐藏的登录自启动任务。

### Linux / macOS Shell

```sh
./install/install.sh \
  --image-archive ./gitsss-image.tar \
  --install-dir /opt/gitsss \
  --port 5300 \
  --root-url http://192.168.110.42:5300/ \
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
  -Port 5300 `
  -RootUrl http://192.168.110.42:5300/
```

```sh
./install/install.sh \
  --image-archive ./gitsss-image-new.tar \
  --install-dir /opt/gitsss \
  --data-volume gitsss-data \
  --port 5300 \
  --root-url http://192.168.110.42:5300/
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

## vbox/Docker 命令兼容的本机验证

先在本机 vbox checkout 构建安装，再从 gitsss 根目录执行：

```powershell
Set-Location 'C:\Users\14170\Desktop\Code\neko233-Projects\vbox'
.\install.ps1
Set-Location 'C:\Users\14170\Desktop\Code\neko233-Projects\gitsss'
$env:Path = "$env:LOCALAPPDATA\vbox\bin;$env:Path"
.\tools\vbox\gitsss.ps1 -Action check
.\tools\vbox\gitsss.ps1 -Action pull
.\tools\vbox\gitsss.ps1 -Action install
.\tools\vbox\gitsss.ps1 -Action verify
```

安装后的 `vbox build`、`docker build` 和 `docker compose` 使用同一 vbox 实现，因此现有
Dockerfile 和 Compose 文件不需要任何改动：

```powershell
$env:Path = "$env:LOCALAPPDATA\vbox\bin;$env:Path"
docker compose -f compose.yaml up -d
docker compose -f compose.yaml ps
docker compose -f compose.yaml down
```

上述方式不改变 `Dockerfile`、`compose.yaml`、镜像名或卷路径；vbox 负责执行 Docker
兼容命令和 API。多机编排、Swarm/Kubernetes 等平台能力仍不属于本机 vbox 引擎。

如果私有 registry 暂时不可用，可直接导入 Docker `save` 生成的离线归档，跳过拉取：

```powershell
.\tools\vbox\gitsss.ps1 -Action load -ImageArchive .\gitsss-image.tar
.\tools\vbox\gitsss.ps1 -Action install -ImageArchive .\gitsss-image.tar
.\tools\vbox\gitsss.ps1 -Action verify -ImageArchive .\gitsss-image.tar
# 等价的 Taskfile 调用：task vbox:load IMAGE_ARCHIVE=.\gitsss-image.tar
```

验证使用 vbox 的 `--volume` 将 `%LOCALAPPDATA%\gitsss\vbox-data` 持久化到
镜像 `/data`，健康检查地址为 `http://127.0.0.1:5300/api/healthz`。详见
[vbox 本机验证](docs/operations/virtualbox.md)。

## 检查更新

启动时不会访问 GitHub。管理员在登录后打开「管理后台 → 版本更新」，或在服务器上运行：

```sh
gitsss update check --json
gitsss update check --mirror https://mirror.example --mirror https://backup.example/{url} --save
gitsss update download --output /tmp
```

直连 GitHub 与配置的 HTTP 镜像会同时探测。官方清单可达时，用它的 sha256 校验从最快镜像下载的包；官方清单不可达时，按延迟从低到高回退，并在结果里标明校验和只绑定所选镜像。镜像留空时使用 `https://ghfast.top`。`gitsss update download` 只写出校验通过的文件，不替换正在运行的进程。

## 开发

```sh
# 后端工具链：Go 1.27.2
# 没有 Docker 时 task 直接在本机执行。GITSSS_TASK_NATIVE=1 也可以强制本机。
pnpm install --frozen-lockfile
make backend-quick
make release-native VERSION=1.2.0
```

## 相关文档

- [HTML 文档站](docs/site/index.html)
- [离线生产策略](docs/operations/offline-production.md)
- [Gitea HTTP 迁移](docs/operations/gitea-http-import.md)
- [Mac mini M4 部署](docs/operations/mac-mini-m4.md)
- [生产就绪审计](docs/verification/production-readiness-2026-07-16.md)

## License

[MIT](LICENSE)
