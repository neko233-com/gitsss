# gitsss

gitsss 是面向公司内网的自维护 Git 协作平台。代码基线复制自 Gitea v1.26.2，运行时不依赖 Gitea 应用库；唯一公开命令为 `gitsss`。

## 产品边界

- 仓库、账号、组织、Issue、Pull Request、项目、软件包、Release、Actions、管理后台和 API 以 Gitea 行为与页面布局为基线。
- 保留 gitsss 皮肤、Git 存储底层、存储感知、写入保护和数据大盘。
- 增加飞书签名通知、自定义通知模板及适合团队协作的 GitLab 功能。
- 默认 HTTP 端口为 `3000`。

## 离线 Actions

单个 Ubuntu 24.04 镜像同时包含服务端、内置 runner、Node.js、pnpm、Go、Git LFS、Docker 和常用 Action。

Actions 工作流机制与 Gitea 基线一致。仓库任务运行在独立容器内，每个任务使用私有 Docker daemon。任务不能访问宿主 Docker socket。内置 `checkout`、`cache`、`upload-artifact`、`download-artifact`、`setup-node` 和 `setup-go` 可离线使用；未知远程 Action 在离线模式下明确失败，不访问公网。

## 安装

Linux/macOS：

```sh
./install/install.sh \
  --image-archive ./gitsss-image.tar \
  --install-dir /opt/gitsss \
  --port 3000 \
  --root-url http://192.168.110.42:3000/
```

PowerShell：

```powershell
.\install\install.ps1 `
  -ImageArchive .\gitsss-image.tar `
  -InstallDir C:\gitsss `
  -Port 3000 `
  -RootUrl http://192.168.110.42:3000/
```

安装器不拉取外部镜像。迁移验证时可先使用其他宿主端口，再用相同
安装目录和数据卷重跑脚本切回 `3000`。默认会配置 Docker 就绪后的静默
后台启动：Windows 使用隐藏计划任务（受策略限制时回退到隐藏 `wscript`
启动器），Linux 使用 systemd，macOS 使用 launchd；日志位于安装目录的
`logs/`。使用 `-NoAutostart` 或 `--no-autostart` 可禁用。

## 开发与验证

先查看统一入口：

```sh
task help
```

Taskfile 通过本地 `gitsss/gitsss:latest` 容器执行兼容 Makefile 目标，因而
开发主机只需要安装 [Task](https://taskfile.dev/) 和 Docker。

常用检查：

```sh
task fmt
task lint:go
task lint:js
task test
```

项目不使用 GitHub Actions。格式化、静态检查、测试、竞态检查、镜像验证、跨平台打包和发布验证均在本地完成。

## 文档

- [生产计划](PLAN.md)
- [离线生产策略](docs/operations/offline-production.md)
- [Mac mini M4 部署](docs/operations/mac-mini-m4.md)
- [Gitea HTTP 迁移验证](docs/verification/gitea-http-import-1.26.2.md)
- [Actions 兼容验证](docs/verification/actions-compatibility-2026-07-16.md)
- [生产就绪审计](docs/verification/production-readiness-2026-07-16.md)
