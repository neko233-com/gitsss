# 签名更新验证

gitsss 更新验证采用 Ed25519 签名的 canonical `gitsss-update-v1` manifest。manifest 将版本、40 位 source commit、UTC 发布时间以及每个 Linux/macOS/Windows amd64/arm64 归档的文件名、字节数和 SHA-256 绑定到同一签名。

Agent 只持有通过安装包、配置管理或管理员指纹核对预置的发布公钥。不能把与更新包一同下载、但从未被信任的公钥当作信任根；HTTPS、GitHub asset digest 和 `SHA256SUMS` 是额外的传输完整性证据，不替代发布者签名。

当前信任根位于 `keys/release-ed25519.pub`，Key ID 为 `11d4f94bdcb5c016`。发布脚本会把它嵌入安装包，并拒绝使用派生公钥不一致的私钥签名。密钥轮换必须由旧密钥签署过渡声明并经管理员显式更新信任，不能只替换仓库文件。

下载到隔离 staging 目录后，使用绝对路径验证：

```powershell
gitsss-agent update-verify `
  --manifest C:\gitsss-update\update-manifest.json `
  --signature C:\gitsss-update\update-manifest.sig `
  --public-key C:\ProgramData\gitsss\release.pub `
  --package C:\gitsss-update\gitsss-0.1.0-windows-amd64.zip
```

验证会按顺序拒绝：未知/非 canonical JSON、未知字段、错误 key ID、无效 Ed25519 签名、当前平台无对应 artifact、文件名或大小不符、符号链接/非常规文件、打开期间被替换，以及 SHA-256 不符。成功输出已验证的 version、commit、发布时间、key ID 和 artifact 元数据。

验证后可把本机平台包解压到一个全新的隔离目录：

```powershell
gitsss-agent update-stage `
  --manifest C:\gitsss-update\update-manifest.json `
  --signature C:\gitsss-update\update-manifest.sig `
  --public-key C:\ProgramData\gitsss\release.pub `
  --package C:\gitsss-update\gitsss-0.1.0-windows-amd64.zip `
  --target C:\ProgramData\gitsss\updates\0.1.0
```

`update-stage` 只接受当前 Agent 的 OS/architecture 和不存在的绝对目标目录。它在同一打开文件句柄上重验签名清单对应的大小与 SHA-256，再限制解压条目数、单文件/总大小，并拒绝路径穿越、绝对路径、符号/硬链接、设备文件、重复/大小写冲突路径和 Windows 保留名。ready 元数据最后写入；任一步失败会删除本次新建的 staging 目录。

`update-verify` 和 `update-stage` 都不下载、不替换正在运行的服务，也不执行服务切换。确认 staging 输出后，在独立的管理员终端激活：

```powershell
gitsss-agent update-activate `
  --target C:\ProgramData\gitsss\updates\0.1.0 `
  --state-dir C:\ProgramData\gitsss\launcher-state
```

`update-activate` 只接受当前主机平台，严格核对 ready 元数据、包内 `BUILDINFO.json`、旧/新 Agent 常规文件及平台。它以不可覆盖的独占锁串行化更新，发布下一 launcher generation 后重启 Task Scheduler、launchd 或 systemd 服务；只有观察到该 generation 的 launcher 启动回执，并由 staging 中的新 Agent 使用受保护 work Token 通过 `/api/v1/agent/health` 认证探测，才算成功。探测不领取或修改作业。

重启、回执或健康探测失败时，事务发布更高 generation 指回旧目录，再重启并验证旧版本；不会改写或删除已有 generation。并发更新会直接失败。进程异常退出遗留的 `.gitsss-update-activation.lock` 不会自动过期：管理员必须先核对最新 active state 与服务进程，再只删除该锁。当前事务不下载更新，也不自动回收旧版本目录；磁盘预留和经审批的历史版本清理仍属于后续安装器策略。

没有 `update-manifest.json` 和 `update-manifest.sig` 的 checksum-only Release 只能用于人工安装，并必须使用同一 Release 中的 `SHA256SUMS` 校验；它不会被签名更新命令接受。历史包不能在事后追认为已签名。只有发布构建机持有与预置 `release.pub` 匹配的私钥时，才允许生成并发布自动更新清单。

发布构建机使用 `go run ./cmd/gitsss-release keygen` 一次性生成 Ed25519 seed、公钥和 key ID。该命令的 JSON 含私钥材料，只能捕获到受保护的密钥托管流程，不能粘贴到工单、日志或提交中。实际打包通过 `-SigningKeyEnvironment` 指定环境变量名称；发布工具从环境读取 seed，扫描并绑定恰好六个目标归档，私钥不会写入输出目录。
