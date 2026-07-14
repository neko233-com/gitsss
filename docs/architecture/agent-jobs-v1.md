# gitsss AI Agent 作业协议 v1

## 为什么使用出站领取

控制台网页不能安全地直接访问旧服务器上的 loopback Agent：远程 HTTPS 页面访问本地 HTTP 会遇到混合内容、Private Network Access、CORS 和 Token 暴露问题；让旧服务器开放入站端口又扩大了接管攻击面。

v1 因此采用出站领取：管理员在 gitsss 创建类型受限的作业，旧服务器上的常驻 `gitsss-agent work` 使用 `agents:work` Token 主动访问控制面，原子领取任务，在本机执行后回传结构化结果。浏览器只持有 gitsss 管理员 Token，不接触 Agent 工作 Token。一次性 `work-once` 仅用于安装诊断。

## 状态机和权限

```text
queued → running (5 分钟租约) → succeeded | failed
             ├── 心跳续租
             └── 租约过期 → queued
```

- `agents:dispatch`：创建白名单作业；不能创建 shell、脚本或任意命令任务。
- `agents:read`：读取作业状态和脱敏结果，供线性接管 UI 使用。
- `agents:work`：领取、续租和完成任务；工作 Token 的 ID 必须与 Agent ID 完全一致，领取后其他 Agent 无法续租或完成。
- 重复完成同一终态作业是幂等操作，不会产生第二次执行结果。
- 创建、领取和完成分别写入审计，包含 request ID、非敏感操作者、作业 ID 和状态。
- 常驻 Worker 在执行长任务时每分钟续租，在空队列和控制面故障时分别使用有上限的指数退避；Token 默认只从环境变量或受保护文件读取。
- 执行已结束但完成请求失败时，进程会保留待回传结果并只重试完成请求，不在同一进程中重复执行作业。

v1 白名单仅包含 `takeover.assess`、`takeover.freeze-check`、`backup.export`、`backup.restore-isolated`、`backup.audit-restore`、`takeover.inventory`、`takeover.shadow-audit`、`takeover.cutover`、`takeover.rollback` 与 `backup.verify`。所有带 Agent 身份的输入都强制 `agent_id` 与执行 Worker 一致；备份口令只按环境变量名称在旧服务器本地读取，不进入作业内容或结果。

`takeover.freeze-check` 直接探测 `app.ini` 配置的本地 TCP 或 Unix socket。监听仍可连接时结果明确为 online；只有 connection-refused 或 Unix socket 不存在才生成 offline freeze ID，超时、权限错误和网络不可达不会被误判为停服。PostgreSQL/MySQL 导出必须携带该 ID，Agent 会在导出前重新探测并把 ID 写入加密清单。

`backup.export` 必须携带管理员刚刚确认的评估 ID。Agent 会在导出前重新读取配置和资产路径，只有新评估 ID 与确认值完全一致才会生成容器；配置或范围在确认后发生变化时，任务失败且不会创建输出。输出路径使用 no-replace 原子发布，成功结果只回传文件数、字节数、容器路径和 SHA-256。

`backup.restore-isolated` 必须同时携带评估 ID 和管理员确认的容器 SHA-256。Agent 在两次完整认证解密中计算原始容器摘要，只有两次摘要与确认值都相等才会以 no-replace 方式发布全新的隔离目录；在线 Gitea 和 gitsss 数据路径不在该任务的写入范围内。

`backup.audit-restore` 继承同一评估 ID、容器 SHA-256 和隔离目标，任务本身只读。成功回传只表示恢复内容审计已执行；只有逐文件清单全部一致、所有 Git 仓库严格校验通过且 LFS OID 无异常时，线性向导才允许下发业务盘点。

`takeover.inventory` 在隔离副本中只读打开 SQLite online-backup 快照，或逐行验证 PostgreSQL/MySQL 的确定性 JSONL，按实际记录统计成员、组织、仓库、Issue、PR、评论、发布、标签、里程碑、附件、团队和 Actions 运行。结果包含表级计数与 canonical SHA-256；缺少唯一数据库快照、格式漂移、重复表声明、超大行或记录上限都会阻断。影子审计与切换必须绑定该清单。

`backup.export` 对 SQLite 不再复制可能遗漏 WAL 的主文件，而是用内嵌 SQLite online-backup API 生成权限为 `0600` 的临时一致性快照，通过 `quick_check` 后才把 `database-snapshot` 写入加密容器。PostgreSQL/MySQL 由编译进 Agent 的纯 Go 驱动开启只读 repeatable-read 事务，按 schema/table/row 稳定排序并编码为带类型的确定性 JSONL；不调用 `pg_dump`、`mysqldump`，也不要求目标部署外部数据库。临时明文目录在导出结束后删除。

`takeover.shadow-audit` 再次读取 live `app.ini`、资产清单和 bare Git refs，并与已确认容器及隔离恢复逐项比较。SQLite 再次执行 online backup；PostgreSQL/MySQL 必须仍处于同一 freeze ID 的 offline 状态，并由内嵌驱动再次生成确定性快照。数据库快照大小与 SHA-256、配置、普通资产和 Git refs 任一变化都会阻断。旧式直接复制数据库的备份会被要求重新导出；MSSQL 在内嵌适配器完成前继续返回 blocker。

`takeover.cutover` 不信任先前显示的结果，会在执行时重新完成业务清单和影子审计。只有业务清单 SHA-256、当前源状态、冻结证据、评估 ID 与容器 SHA-256 仍完全匹配，且管理员输入精确的 `CUTOVER <assessment-id>`，Agent 才会计算隔离树摘要、写入受保护的 prepared 标记并执行同卷目录级原子切换。旧活动目录以不可覆盖的 rollback generation 保留。

`takeover.rollback` 要求精确的 `ROLLBACK <cutover-id>`，并在回退前重新计算当前活动树摘要。切换后产生任何新写入都会使回退失败，避免静默丢弃新数据；管理员必须先另行备份或处理差异。成功回退也不会删除新代，而是把它归档为 rolled-back generation。prepared 标记可恢复两个 rename 之间的进程中断。

## HTTP 接口

| 方法与路径 | Scope | 用途 |
| --- | --- | --- |
| `POST /api/v1/agent/jobs` | `agents:dispatch` | 创建类型化作业 |
| `GET /api/v1/agent/jobs` | `agents:read` | 查询线性流程状态 |
| `POST /api/v1/agent/jobs/claim` | `agents:work` | 原子领取一个作业 |
| `POST /api/v1/agent/jobs/{id}/heartbeat` | `agents:work` | 为当前 Agent 的运行中作业续租 |
| `POST /api/v1/agent/jobs/{id}/complete` | `agents:work` | 回传成功结果或失败原因 |

队列、租约、幂等完成结果和审计记录已持久化在 gitsss 的事务 SQLite 中；进程重启后可恢复，过期租约会重新入队。工作 Token ID 形成最小设备绑定，常驻 Worker 负责续租与离线退避。v1 尚不提供任务取消、细粒度进度事件和远程任意命令；Runner 使用独立的固定模板协议，接管切换只接受上述类型化参数。
