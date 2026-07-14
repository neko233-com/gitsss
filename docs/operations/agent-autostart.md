# gitsss Agent 开机启动计划

`gitsss-agent service-plan` 为目标系统生成确定性的服务定义、Token 文件位置以及安装/卸载命令。该命令只输出 JSON，不写系统目录、不执行提权命令，适合先审阅、纳入安装包，再由后续安装事务应用。

| 平台 | 启动机制 | 定义位置 | 默认 Token 文件 |
| --- | --- | --- | --- |
| Windows | Task Scheduler 启动触发器，SYSTEM 身份，失败重启 | `C:\ProgramData\gitsss\gitsss-agent-task.xml` | `C:\ProgramData\gitsss\agent.token` |
| macOS | system launchd，RunAtLoad + KeepAlive | `/Library/LaunchDaemons/com.gitsss.agent.plist` | `/Library/Application Support/gitsss/agent.token` |
| Linux Server | systemd，network-online 后启动并自动恢复 | `/etc/systemd/system/gitsss-agent.service` | `/etc/gitsss/agent.token` |

服务定义只包含 `--token-file` 路径，不包含 Token 值。Agent 拒绝相对路径、符号链接、非常规文件、空文件和超过 4096 字节的文件；在 Unix 上还要求文件不能被 group/other 访问。Windows 安装事务需要为 Token 文件设置只允许 SYSTEM 和 Administrators 读取的 ACL。

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

下一步的安装事务必须先原子写入 Token 和服务定义、设置平台权限，再运行计划中的安装命令；任一步失败都要卸载服务并删除尚未激活的定义。`service-plan` 本身不代表服务已安装。
