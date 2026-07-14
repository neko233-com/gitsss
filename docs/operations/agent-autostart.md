# gitsss Agent 开机启动计划

`gitsss-agent service-plan` 为目标系统生成确定性的服务定义、Token 文件位置以及安装/卸载命令。该命令只输出 JSON，不写系统目录、不执行提权命令，适合安装前审阅。

| 平台 | 启动机制 | 定义位置 | 默认 Token 文件 |
| --- | --- | --- | --- |
| Windows | Task Scheduler 启动触发器，SYSTEM 身份，失败重启 | `C:\ProgramData\gitsss\gitsss-agent-task.xml` | `C:\ProgramData\gitsss\agent.token` |
| macOS | system launchd，RunAtLoad + KeepAlive | `/Library/LaunchDaemons/com.gitsss.agent.plist` | `/Library/Application Support/gitsss/agent.token` |
| Linux Server | systemd，network-online 后启动并自动恢复 | `/etc/systemd/system/gitsss-agent.service` | `/etc/gitsss/agent.token` |

已安装的发行包会自动发现同目录的 `gitsss-launcher` 和 `BUILDINFO.json`。平台服务只执行稳定 launcher 与 `--state-dir`；generation 1 state 再指向当前版本 Agent，并包含 `--token-file` 路径，不包含 Token 值。Agent 拒绝相对 Token 路径、符号链接、非常规文件、空文件和超过 4096 字节的文件；在 Unix 上还要求 Token 文件不能被 group/other 访问。Windows 安装事务为 Token 文件设置只允许 SYSTEM 和 Administrators 读取的 ACL。

## 安装与卸载

先以管理员/root 身份打开终端，将 scoped work Token 放入环境变量，再执行本机安装。Token 不会进入命令行、服务定义或计划 JSON：

```powershell
$env:GITSSS_AGENT_WORK_TOKEN = "replace-with-a-scoped-work-token"
gitsss-agent service-install --server https://gitsss.internal --id studio-agent
Remove-Item Env:GITSSS_AGENT_WORK_TOKEN
```

`service-install` 只接受本机平台，并且默认不覆盖已有 Token、launcher state 或服务定义。它先创建目录，把 Token 以无覆盖原子方式发布为 Unix `0600` 文件或 Windows SYSTEM/Administrators 只读 ACL 文件，再发布 generation 1、服务定义并启动服务。任一安装命令失败都会尝试卸载服务并回滚本次创建的文件。

卸载不需要再次提供 Token、服务器地址或 Agent ID：

```powershell
gitsss-agent service-uninstall
```

只有平台服务卸载命令全部成功后，卸载事务才删除 Token、初始 generation 和服务定义；更新产生的后续 generation、非空工作/日志目录会保留，避免误删审计历史或运行数据。自定义过 `--token-file`、`--state-directory`、`--working-directory` 或 `--log-directory` 时，卸载必须传入相同路径。

运行已经安装到最终位置的二进制时，可以省略 `--platform` 和 `--executable`：

```powershell
gitsss-agent service-plan `
  --server https://gitsss.internal `
  --id studio-agent
```

使用 `go run` 或为发布包预生成其他平台计划时，必须提供目标平台的最终绝对二进制路径：

```powershell
go run ./cmd/gitsss-agent service-plan --platform windows --executable "C:\Program Files\gitsss\gitsss-agent.exe" --server https://gitsss.internal --id studio-agent
go run ./cmd/gitsss-agent service-plan --platform darwin --executable /usr/local/bin/gitsss-agent --server https://gitsss.internal --id studio-agent
go run ./cmd/gitsss-agent service-plan --platform linux --executable /usr/local/bin/gitsss-agent --server https://gitsss.internal --id studio-agent
```

计划会验证 HTTP(S) 控制面地址、Agent ID 和所有目标路径，拒绝 URL 用户信息、查询 Token、换行和相对路径。Windows 参数按 CommandLineToArgvW 规则引用；launchd 使用独立 `ProgramArguments`；systemd 对引号、反斜杠和 `%` 做转义。

`service-plan` 本身不代表服务已安装；只有 `service-install` 成功返回才表示服务已注册并启动。

后续签名更新由 `update-stage` 写入全新版本目录，再由独立管理员终端运行 `update-activate --target <版本目录> --state-dir <state 目录>`。激活会等待 launcher 对新 generation 的不可变启动回执，并使用同一受保护 Token 做不领取作业的控制面健康探测；失败时自动发布更高 generation 回滚。详见 [`signed-updates.md`](signed-updates.md)。
