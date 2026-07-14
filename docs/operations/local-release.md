# 本机验证与发布

GitHub Actions 额度不可用期间，gitsss 不注册远端 CI 或发布工作流。`tools/package-gitsss-release.ps1` 在一台受信任的 Windows 构建机上执行等价验证、六平台交叉编译、归档、校验和发布。

## 前置条件

- Go 1.26.x；race 测试需要本机 CGO 工具链
- Node.js 24.14.x 与 pnpm 11.7.x
- PowerShell、`tar`、Git
- 上传时需要已登录且有仓库写权限的 GitHub CLI `gh`

脚本要求 tracked worktree 与 index 都干净，并把产物绑定到精确的 40 位 commit SHA。产品树只包含 gitsss 自有运行代码；迁移兼容说明只在 `NOTICE` 和接管文档中保留。

## 仅构建

```powershell
.\tools\package-gitsss-release.ps1 -Version 0.1.0
```

默认输出到系统临时目录中的唯一目录。脚本依次执行 frozen pnpm install、lint、typecheck、Vitest、Vite build、Go race tests 和 vet，再以 `CGO_ENABLED=0` 构建：

- Linux amd64 / arm64 (`.tar.gz`)
- macOS amd64 / arm64 (`.tar.gz`)
- Windows amd64 / arm64 (`.zip`)

每个包包含 `gitsss`、`gitsss-agent`、`gitsss-launcher`、React web bundle、README、LICENSE、NOTICE、Agent/服务端操作文档和绑定版本/commit/平台的 `BUILDINFO.json`。脚本通过 `go version -m` 复核每个二进制的 GOOS/GOARCH，并为六个归档生成无 BOM UTF-8 `SHA256SUMS`。

提供已托管的 Ed25519 seed 环境变量时，脚本还会生成 canonical `update-manifest.json`、`update-manifest.sig` 和 `release.pub`，并把这些文件纳入 `SHA256SUMS`：

```powershell
.\tools\package-gitsss-release.ps1 `
  -Version 0.1.0 `
  -SigningKeyEnvironment GITSSS_RELEASE_SIGNING_KEY
```

seed 只从命名环境变量读取，不得放入参数、脚本、仓库或 release asset。脚本强制签名派生公钥与仓库的 `keys/release-ed25519.pub`（Key ID `11d4f94bdcb5c016`）一致，并把该公钥嵌入每个平台包。`release.pub` 用于分发和指纹核对，但不能仅因与更新包一同下载就自动成为信任根；安装器/Agent 必须预置管理员已经核对的同一公钥。

## 构建并上传 Release

```powershell
.\tools\package-gitsss-release.ps1 `
  -Version 0.1.0 `
  -SigningKeyEnvironment GITSSS_RELEASE_SIGNING_KEY `
  -UploadTag v0.1.0
```

`-UploadTag` 先创建 draft release，并把 tag 绑定到本次构建的 commit，而不是可移动的分支头；随后逐个上传资产，将 GitHub 返回的每个 SHA-256 digest 与本地文件复核，全部一致后才发布。上传失败会保留本地归档和远端 draft 供诊断，不会冒充完整版本。若 tag/release 已存在，命令失败且不会覆盖已有资产。只有显式传入 `-Prerelease` 才会标记预发布；默认生成稳定 Release。仅用于归档的 staging 目录会在生成校验清单后安全删除；`-SkipTests` 只用于本地脚本 smoke test，不能用于正式发布。

未配置 `-SigningKeyEnvironment` 时，脚本发布六个平台归档和 `SHA256SUMS`，不生成签名 manifest；这种 Release 适用于手动安装和校验，不作为签名自动更新输入。不得为了生成签名而临时更换仓库信任公钥：发布私钥必须来自既有的受保护托管，并与 `keys/release-ed25519.pub` 匹配。
