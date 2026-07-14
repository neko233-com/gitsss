# 内置状态与通知 Outbox v1

gitsss 主服务只打开一个随二进制运行的 SQLite 数据库，不依赖外部数据库、Redis 或任务队列。默认数据库为 `data/gitsss.db`，可通过 `GITSSS_DATA_PATH` 调整；数据库使用 WAL、`synchronous=FULL`、5 秒 busy timeout 和版本化 schema。

## 文件保护与加密

- SQLite 主文件与 WAL/SHM sidecar 在 Unix 上使用 `0600`；Windows 通过原生安全 API设置 protected DACL，只允许当前服务账户、SYSTEM 和 Administrators，不调用 `icacls`。
- 通知配置、Webhook、SMTP 密码、收件人、主题和正文在写入 SQLite 前使用 AES-256-GCM 加密。
- 每个密文把 namespace 与 document ID 作为 associated data；把密文移动到另一个通道或消息 ID 会校验失败。
- 32 字节主密钥默认位于 `<data-path>.key`，可通过 `GITSSS_MASTER_KEY_PATH` 放到独立受保护卷。密钥丢失、长度错误或密文篡改都会阻止服务启动，不会静默重置通知。

## Outbox 语义

消息以 `pending → delivering → delivered | retrying → dead` 的线性状态运行。Dispatcher 在执行 HTTP/SMTP I/O 前先持久化 `delivering` claim，因此并发 Dispatcher 不会同时领取同一条消息。

外部通知无法在本地 SQLite 与远端服务之间形成原子事务，所以 v1 明确采用 at-least-once：进程在远端已接收、但本地尚未写入 `delivered` 时崩溃，重启会把结果未知的消息放回重试；15 分钟未完成的 delivery lease 也会被回收。所有失败按 1、2、4 分钟退避，第四次失败进入死信。

## 当前持久化范围

已持久化 Agent 作业、接管评估、通知配置和通知 Outbox。Bootstrap Token、Dashboard 事件预览和审计记录仍需迁入同一内置状态边界；在这些状态完成前，v1 仍是开发预览而不是正式发布版本。
