# gitsss AI Agent 作业协议 v1

## 为什么使用出站领取

控制台网页不能安全地直接访问旧服务器上的 loopback Agent：远程 HTTPS 页面访问本地 HTTP 会遇到混合内容、Private Network Access、CORS 和 Token 暴露问题；让旧服务器开放入站端口又扩大了接管攻击面。

v1 因此采用出站领取：管理员在 gitsss 创建类型受限的作业，旧服务器上的常驻 `gitsss-agent work` 使用 `agents:work` Token 主动访问控制面，原子领取任务，在本机执行后回传结构化结果。浏览器只持有 gitsss 管理员 Token，不接触 Agent 工作 Token。一次性 `work-once` 仅用于安装诊断。

## 状态机和权限

```text
queued → running (5 分钟租约) → succeeded | failed
             └── 租约过期 → queued
```

- `agents:dispatch`：创建白名单作业；不能创建 shell、脚本或任意命令任务。
- `agents:read`：读取作业状态和脱敏结果，供线性接管 UI 使用。
- `agents:work`：领取和完成任务；领取后绑定 Agent ID，其他 Agent 完成会返回 403。
- 重复完成同一终态作业是幂等操作，不会产生第二次执行结果。
- 创建、领取和完成分别写入审计，包含 request ID、非敏感操作者、作业 ID 和状态。
- 常驻 Worker 在空队列和控制面故障时分别使用有上限的指数退避；Token 默认只从环境变量读取。
- 执行已结束但完成请求失败时，进程会保留待回传结果并只重试完成请求，不在同一进程中重复执行作业。

v1 白名单仅包含 `takeover.assess`、`backup.export`、`backup.restore-isolated`、`backup.audit-restore` 与 `backup.verify`。评估、导出、恢复和审计都强制输入中的 `agent_id` 与执行 Worker 一致；备份口令只按环境变量名称在旧服务器本地读取，不进入作业内容或结果。

`backup.export` 必须携带管理员刚刚确认的评估 ID。Agent 会在导出前重新读取配置和资产路径，只有新评估 ID 与确认值完全一致才会生成容器；配置或范围在确认后发生变化时，任务失败且不会创建输出。输出路径使用 no-replace 原子发布，成功结果只回传文件数、字节数、容器路径和 SHA-256。

`backup.restore-isolated` 必须同时携带评估 ID 和管理员确认的容器 SHA-256。Agent 在两次完整认证解密中计算原始容器摘要，只有两次摘要与确认值都相等才会以 no-replace 方式发布全新的隔离目录；在线 Gitea 和 gitsss 数据路径不在该任务的写入范围内。

`backup.audit-restore` 继承同一评估 ID、容器 SHA-256 和隔离目标，任务本身只读。成功回传只表示审计已执行；只有逐文件清单全部一致、所有 Git 仓库严格校验通过且 LFS OID 无异常时，结果中的 `ready_for_shadow_mode` 才为 true，线性向导才进入影子模式步骤。

## HTTP 接口

| 方法与路径 | Scope | 用途 |
| --- | --- | --- |
| `POST /api/v1/agent/jobs` | `agents:dispatch` | 创建类型化作业 |
| `GET /api/v1/agent/jobs` | `agents:read` | 查询线性流程状态 |
| `POST /api/v1/agent/jobs/claim` | `agents:work` | 原子领取一个作业 |
| `POST /api/v1/agent/jobs/{id}/complete` | `agents:work` | 回传成功结果或失败原因 |

当前队列是进程内实现，用于先固定安全协议和 UI 状态机。生产版本必须迁移到事务数据库，增加设备绑定、租约续期、幂等键、取消、进度事件、持久审计和离线退避；这些能力完成前不会通过作业协议开放在线导入、Runner 安装、服务切换或回滚。
