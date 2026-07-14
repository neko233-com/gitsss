# 签名更新验证

gitsss 更新验证采用 Ed25519 签名的 canonical `gitsss-update-v1` manifest。manifest 将版本、40 位 source commit、UTC 发布时间以及每个 Linux/macOS/Windows amd64/arm64 归档的文件名、字节数和 SHA-256 绑定到同一签名。

Agent 只持有通过安装包、配置管理或管理员指纹核对预置的发布公钥。不能把与更新包一同下载、但从未被信任的公钥当作信任根；HTTPS、GitHub asset digest 和 `SHA256SUMS` 是额外的传输完整性证据，不替代发布者签名。

下载到隔离 staging 目录后，使用绝对路径验证：

```powershell
gitsss-agent update-verify `
  --manifest C:\gitsss-update\update-manifest.json `
  --signature C:\gitsss-update\update-manifest.sig `
  --public-key C:\ProgramData\gitsss\release.pub `
  --package C:\gitsss-update\gitsss-0.1.0-windows-amd64.zip
```

验证会按顺序拒绝：未知/非 canonical JSON、未知字段、错误 key ID、无效 Ed25519 签名、当前平台无对应 artifact、文件名或大小不符、符号链接/非常规文件、打开期间被替换，以及 SHA-256 不符。成功输出已验证的 version、commit、发布时间、key ID 和 artifact 元数据。

`update-verify` 只验证已经下载的归档，不下载、不解压、不替换正在运行的服务，也不执行回滚。自动更新事务必须在此验证之后另行完成解压边界检查、磁盘预留、原版本快照、服务切换、健康探测和失败回滚。

`v0.1.0-alpha.1` 是切换到本机发布流程的 SHA-256 基线包，尚未包含此验证命令或 Ed25519 manifest。签名发布从完成离线私钥托管和公钥预置的后续预发布开始，不能追认旧包为已签名。

发布构建机使用 `go run ./cmd/gitsss-release keygen` 一次性生成 Ed25519 seed、公钥和 key ID。该命令的 JSON 含私钥材料，只能捕获到受保护的密钥托管流程，不能粘贴到工单、日志或提交中。实际打包通过 `-SigningKeyEnvironment` 指定环境变量名称；发布工具从环境读取 seed，扫描并绑定恰好六个目标归档，私钥不会写入输出目录。
