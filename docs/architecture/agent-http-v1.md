# gitsss Agent HTTP v1

## 安全边界

`gitsss-agent serve` 是内网接管 AI Agent 的本地主机工具面，不是公网 API。v1 强制监听明确的 `127.0.0.0/8` 或 `::1` IP，默认 `127.0.0.1:8790`；主机名（包括 `localhost`）也被拒绝，避免受 hosts/DNS 解析影响。需要跨主机控制时，后续版本必须先加入设备身份、双向 TLS 和明确私网策略，不能直接改成 `0.0.0.0`。

启动 Token 从 `--token-env` 指定的环境变量读取，至少 24 个字符；内存中只保存 SHA-256，校验使用常量时间比较。Token 必须有最长 30 天的有效期，重启时可轮换；默认 24 小时。请求只接受严格的 `Authorization: Bearer TOKEN`，URL 中任何名称包含 `token` 的查询参数都会在路由前被拒绝并审计。

每个请求生成 96 位随机 request ID，并通过 `X-Request-ID` 返回。受保护操作记录非敏感 Token ID、动作、目标、结果、时间和 request ID；原始 Token、备份口令和配置秘密不进入审计。内存审计按 30 天保留策略清理，查询最多返回最近 500 条。发布前还需替换为加密、追加式持久存储。

## AI 工具接口

| 方法与路径 | Scope | 行为 |
| --- | --- | --- |
| `GET /healthz` | 无 | 只返回 Agent 存活状态，不暴露能力或主机信息 |
| `GET /api/v1/capabilities` | `agent:read` | 返回版本化、可供 AI 选择的能力 ID |
| `POST /api/v1/assessments` | `takeover:assess` | 读取指定 `app.ini` 并生成脱敏只读评估 |
| `POST /api/v1/backups/verify` | `backup:verify` | 从指定环境变量取口令，只读验证加密备份 |
| `GET /api/v1/audit` | `audit:read` | 返回最近 500 条 Agent 审计记录 |

所有 JSON 写请求有大小上限、拒绝未知字段，并且只能包含一个 JSON 对象。评估请求字段为 `agent_id`、`config_path`、可选 `work_path`、`gitea_version` 和 `max_entries`。备份验证字段为 `input_path` 与 `passphrase_env`；API 不接受明文口令字段。

AI Agent v1 只能执行只读评估和验证。备份导出、隔离恢复、Runner 安装、服务切换和回滚仍保留在人工 CLI 确认路径；把它们开放为 HTTP 工具前，必须加入作业 ID、幂等键、计划摘要确认、进度流、取消、设备绑定和持久审计。
