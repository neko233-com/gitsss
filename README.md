# gitsss

> 面向研发团队内网的自维护 Git 协作平台。基于 Gitea v1.26.2，完全离线运行，唯一命令 `gitsss`。

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Quick Start

**默认端口** `3000` · **默认账号** `root` · **默认密码** `root`（首次登录强制修改）

### Linux / macOS

```sh
./install/install.sh \
  --image-archive ./gitsss-image.tar \
  --install-dir /opt/gitsss \
  --port 3000 \
  --root-url http://192.168.110.42:3000/
```

### Windows (PowerShell)

```powershell
.\install\install.ps1 `
  -ImageArchive .\gitsss-image.tar `
  -InstallDir C:\gitsss `
  -Port 3000 `
  -RootUrl http://192.168.110.42:3000/
```

> 安装器不拉取外部镜像，适合纯内网环境。迁移验证时可先使用其他端口，再重跑脚本切回 `3000`。

## Features

- **完全离线** — 服务端 + Actions Runner + Node.js + Go + Git LFS + Docker 打包在一个 Ubuntu 24.04 镜像中，启动后无需公网
- **Gitea 兼容** — 仓库、组织、Issue、PR、项目、Packages、Release、Actions、管理后台、API 均保留 Gitea 行为与页面布局
- **飞书通知** — 内置飞书签名通知 + 自定义通知模板，适合企业协作
- **GitLab 风格增强** — Approval Rules、Protected Branches、Scoped Labels、CODEOWNERS、Artifacts、Merge-after-checks
- **离线 Actions** — 内置 `checkout`、`cache`、`upload-artifact`、`download-artifact`、`setup-node`、`setup-go`；未知远程 Action 明确失败，不访问公网
- **跨平台** — linux/amd64、linux/arm64、macOS arm64（Mac mini M4）、Windows

## 默认配置

| 配置项 | 默认值 |
|--------|--------|
| HTTP 端口 | `3000` |
| 管理员账号 | `root` |
| 管理员密码 | `root` |
| 最小密码长度 | `4` |
| Docker 镜像 | `gitsss/gitsss:latest` |
| 数据卷 | `gitsss-data` |

安装器支持 `-InitialAdminUser` / `-InitialAdminPassword`（PowerShell）或 `--initial-admin-user` / `--initial-admin-password`（Shell）覆盖初始化凭据。

## 自动启动

安装器默认配置 Docker 就绪后静默后台启动：

| 平台 | 机制 |
|------|------|
| Windows | 隐藏计划任务（受策略限制时回退到 `wscript` 启动器） |
| Linux | systemd |
| macOS | launchd |

日志位于安装目录的 `logs/`。使用 `-NoAutostart`（PowerShell）或 `--no-autostart`（Shell）可禁用。

## 开发

先查看统一入口：

```sh
task help
```

Taskfile 通过本地 `gitsss/gitsss:latest` 容器执行兼容 Makefile 目标，开发主机只需安装 [Task](https://taskfile.dev/) 和 Docker。

```sh
task fmt        # 格式化
task lint:go    # Go 静态检查
task lint:js    # JS/TS 静态检查
task test       # 测试
```

项目不使用 GitHub Actions。格式化、静态检查、测试、竞态检查、镜像验证、跨平台打包和发布验证均在本地完成。

## 文档

- [生产计划](PLAN.md)
- [离线生产策略](docs/operations/offline-production.md)
- [Mac mini M4 部署](docs/operations/mac-mini-m4.md)
- [Gitea HTTP 迁移验证](docs/verification/gitea-http-import-1.26.2.md)
- [Actions 兼容验证](docs/verification/actions-compatibility-2026-07-16.md)
- [生产就绪审计](docs/verification/production-readiness-2026-07-16.md)

## License

[MIT](LICENSE)
