# 本机验证与发布

GitHub Actions 额度不可用期间，gitsss 不注册远端 CI 或发布工作流。`tools/package-gitsss-release.ps1` 在一台受信任的 Windows 构建机上执行等价验证、六平台交叉编译、归档、校验和发布。

## 前置条件

- Go 1.26.x；race 测试需要本机 CGO 工具链
- Node.js 24.14.x 与 pnpm 11.7.x
- PowerShell、`tar`、Git
- 上传时需要已登录且有仓库写权限的 GitHub CLI `gh`

脚本要求 tracked worktree 与 index 都干净，并把产物绑定到精确的 40 位 commit SHA。未跟踪的上游 Gitea 兼容快照不会进入新包。

## 仅构建

```powershell
.\tools\package-gitsss-release.ps1 -Version 0.1.0-alpha.1
```

默认输出到系统临时目录中的唯一目录。脚本依次执行 frozen pnpm install、lint、typecheck、Vitest、Vite build、Go race tests 和 vet，再以 `CGO_ENABLED=0` 构建：

- Linux amd64 / arm64 (`.tar.gz`)
- macOS amd64 / arm64 (`.tar.gz`)
- Windows amd64 / arm64 (`.zip`)

每个包包含 `gitsss`、`gitsss-agent`、React web bundle、Agent 开机启动文档和绑定版本/commit/平台的 `BUILDINFO.json`。脚本通过 `go version -m` 复核每个二进制的 GOOS/GOARCH，并为六个归档生成无 BOM UTF-8 `SHA256SUMS`。

## 构建并上传预发布

```powershell
.\tools\package-gitsss-release.ps1 `
  -Version 0.1.0-alpha.1 `
  -UploadTag v0.1.0-alpha.1
```

`-UploadTag` 显式创建 GitHub prerelease，并把 tag 绑定到本次构建的 commit，而不是可移动的分支头。若 tag/release 已存在，命令失败且不会覆盖已有资产。`-SkipTests` 只用于本地脚本 smoke test，不能用于正式发布。
