# gitsss

gitsss 是面向团队与非开发人员的内网 Git 协作平台。它使用 Go 1.26、React 19、Vite 8、TypeScript 7、shadcn/ui 与 ReactBits，提供数据大盘、键盘优先的全局搜索、线性仓库工作流、模板化 CI、最小权限授权、飞书/企业微信/邮件通知，以及可审计的旧实例接管。

## 技术边界

- 单个 Go 服务同源提供 API、Smart HTTP、Git LFS 与唯一的 React 客户端。
- 内置 SQLite 保存 Token、仓库目录、流水线、通知 Outbox、Agent 作业和审计状态，无需外部数据库或队列。
- `gitsss-agent` 只执行固定类型的评估、加密备份、隔离恢复、差异审计、原子切换和受保护回滚，不接受远程 shell。
- CI 使用内置固定模板与独立 Runner；项目自身只通过本地可信脚本构建和上传，不使用 GitHub Actions。

完整范围、已完成能力和退出标准见 [PLAN.md](PLAN.md)。

## 本地开发

需要 Go 1.26、Node.js 24 与 pnpm。先查看统一入口：

```powershell
make help
```

Windows 没有 `make` 别名时使用 `mingw32-make help`。常用验证：

```powershell
mingw32-make lint-go
mingw32-make lint-js
mingw32-make test
mingw32-make build
```

首次启动需要至少 24 字节的 bootstrap Token：

```powershell
$env:GITSSS_API_TOKEN = "replace-with-at-least-24-random-characters"
cd web
pnpm install --frozen-lockfile
pnpm build
cd ../server
go run ./cmd/gitsss
```

默认地址为 `http://127.0.0.1:8787`。首次 Token 仅存续 24 小时，应立即在授权页签发具名最小权限 Token。

## 发布

正式发布必须从干净提交运行 `tools/package-gitsss-release.ps1`，完成 race、vet、前端检查、六平台交叉编译、SHA-256 与可选 Ed25519 签名后，再通过 `gh` 上传。详见 [本地发布手册](docs/operations/local-release.md)。
