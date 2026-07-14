# gitsss 加密备份容器 v1

## 用途与边界

`gitsss-agent backup-export` 把一次只读 Gitea 评估覆盖的配置、内嵌驱动生成的数据库快照、Git 仓库、LFS、附件、软件包和 Actions 数据打包为单个加密文件。v1 用于离线保管、传输和恢复演练前验证；`backup-verify` 不解压、不写入数据库，也不修改源实例。

gitsss 自身只运行内置 SQLite，不要求 PostgreSQL、MySQL、Redis 或外部任务队列。旧 Gitea 的 SQLite 使用内嵌 online-backup API；旧 PostgreSQL/MySQL 使用编译进 Agent 的纯 Go 驱动读取，不需要安装数据库 CLI。MSSQL 与外部对象存储在适配器完成前仍明确阻断，不会伪装成已备份。

## 文件与密码学格式

容器按以下顺序编码：

1. 40 字节明文头：8 字节魔数 `GITSSSB1`、1 字节版本、3 字节分块大小、16 字节随机盐、8 字节随机 nonce 前缀和 4 字节 PBKDF2 迭代次数。
2. PBKDF2-HMAC-SHA256 使用 600,000 次迭代，从环境变量中的口令派生 256 位 AES 密钥。
3. gzip 压缩后的 tar 流按 1 MiB 明文分块，每块使用 AES-256-GCM 独立认证。nonce 为随机 8 字节前缀加单调递增的 32 位计数器。
4. 每个块的认证附加数据包含完整容器头、块类型和明文长度，因此头部、顺序和长度不能被静默修改。
5. 最后写入一个经过 GCM 认证的结束块。验证器拒绝缺失结束块、截断和结束块后的任何尾随数据。

v1 不存储口令、派生密钥或绝对源路径。口令不允许作为命令参数传入，以避免进入进程列表和 shell 历史。管理员仍应使用受控密钥管理器向 Agent 注入环境变量，并限制备份文件访问权限。

## 清单与原子性

tar 的第一项固定为 `manifest.json`。清单包含格式版本、创建时间、来源评估 ID、脱敏配置指纹，以及每个归档相对路径的类别、大小、修改时间和 SHA-256。路径必须规范、排序且唯一，不能包含反斜杠、控制字符、绝对路径或 `..` 穿越。

导出前先哈希全部常规文件；写入时再次计算大小和 SHA-256。两次结果不一致时，导出以 `source changed` 失败。符号链接、特殊文件、不完整的评估或不可读资产也会使导出失败，避免产生看似成功但不完整的备份。

目标文件必须尚不存在。Agent 在目标目录创建权限为 `0600` 的临时文件，完整关闭 tar、gzip 和加密结束块后执行 `sync`，最后用同目录的原子 no-replace 硬链接发布并移除临时名称。并发创建同名目标会使发布失败，不会覆盖已有备份；其他失败路径也会删除临时文件。

## 验证、切换与回滚

`backup-verify` 依次验证容器头、每个 GCM 块、gzip 校验、tar 结构、清单约束和逐文件 SHA-256，并输出只读摘要。它不会把任何条目写到磁盘，因此路径穿越内容也没有落盘机会。

`backup-import-plan` 在完成同样的全量验证后，把归档相对路径映射到管理员指定的隔离目标，并只读检查现状：不存在为 `new`，大小和 SHA-256 均一致为 `identical`，内容不同或目标不是常规文件为 `conflict`，无法读取、父路径不是目录或任一目标组件是符号链接为 `blocked`。输出包含逐项原因和汇总，命令本身不会创建目标目录或文件。后续恢复命令只能消费没有 `blocked` 的最新计划，并在发生 `conflict` 时要求显式策略。

`backup-restore-isolated` 只接受尚不存在的目标，并要求 `--confirm` 与清单中的 assessment ID 完全一致。命令先全量验证容器，再解密到目标同盘的随机隐藏暂存目录；第二次认证流处理成功后，才创建目标作为 no-replace 预留，并把暂存目录原子移动为 `<target>/content`。AI Agent 作业还要求两次读取计算出的原始容器 SHA-256 都与管理员确认值一致。恢复器会在暂存区为清单内识别出的 bare Git 补齐未被文件清单表达的空 `objects/refs` 目录，因此尚无提交的空仓库仍可被 Git 识别。错误确认、已有目标、父级符号链接、容器替换、第二次篡改或写入失败不会覆盖任何既有路径。该命令只产生隔离恢复副本，不能把内容直接写回在线 Gitea 或 gitsss 数据目录。

`backup-audit-restore` 再次把清单中的每个文件与 `<target>/content` 比较，要求全部为 `identical`。对清单中识别出的每个 `.git` 裸仓库，它以参数数组直接调用本机 Git（不经过 shell），执行有超时和输出上限的 `git fsck --full --strict`、`count-objects -v` 和 `show-ref --head -d`，输出对象数、引用数和确定性引用摘要。LFS 对象文件若使用 64 位 OID 作为文件名，则文件名必须等于已经验证的内容 SHA-256。AI Agent 审计还绑定原评估 ID 和容器 SHA-256。任一文件差异、Git 损坏、未分类仓库文件、LFS OID 不一致或 Git 不可用都会令 `ready_for_shadow_mode` 为 false，并使 CLI 以非零状态退出。

SQLite 导出使用内嵌 online-backup API 读取包含 WAL 在内的一致视图，临时快照通过 `PRAGMA quick_check`、同步到磁盘并设为 `0600` 后，才作为 `data/database-snapshot/gitea.db` 进入加密流；无论成功或失败，临时明文目录都在导出调用结束后删除。它只保证数据库本身的时间点一致性，不能让数据库、Git 仓库和附件在 Gitea 仍可写时形成同一事务，因此最终切换前仍需冻结旧实例。

PostgreSQL/MySQL 接管先用 `takeover.freeze-check` 验证旧 Gitea 的本地 TCP/Unix 监听已经明确停止。Agent 以只读 repeatable-read 事务列举用户 schema 的 base table，按稳定表/行顺序把列名、源类型和带类型值编码成 `gitsss-database-v1` JSONL；二进制按 Base64 保存，密码只在 Agent 内存中使用。freeze ID 进入清单，旧实例重新上线、配置变化或证据不匹配都会拒绝后续影子审计。

线性向导先用 `takeover.inventory` 盘点隔离数据库中的业务对象。SQLite 通过只读连接和 `quick_check` 逐表计数；PostgreSQL/MySQL 可移植快照会重新验证 header、表声明、类型和值长度，再统计成员、组织、仓库、Issue、PR、评论、发布、标签、里程碑、附件、团队和 Actions 运行。表级计数与核心分类形成 canonical SHA-256，供最终切换重新核对。

随后用 `takeover.shadow-audit` 重新评估源实例，不写入源或副本。它对 `app.ini` 做原始文件哈希比较（结果不回传哈希或密钥），按普通资产类型比较文件数与总字节数，对数据库再次生成确定性快照并比较大小与 SHA-256，并对最多 10,000 个 bare Git 与恢复副本比较仓库集合及 `show-ref` 确定性摘要；对象数不同但 refs 一致只产生 warning。差异详情限制为 64 条有界记录，完整汇总仍保留。

`takeover.cutover` 在真正写入前再次执行上述影子审计，而不是复用可能已经过期的 UI 结果。管理员必须输入精确的 `CUTOVER <assessment-id>`。Agent 拒绝重叠路径、卷根、符号链接和特殊文件，计算完整隔离树摘要，写入 `<active>.gitsss-cutover.json` prepared 标记，然后把现有活动目录改名为不可覆盖的 rollback generation，再把 `<target>/content` 原子改名为活动目录。两个目录必须位于支持原子 rename 的同一文件系统；跨卷操作会安全失败。旧代不会自动删除。

`takeover.rollback` 必须同时匹配活动路径、受保护状态文件、cutover ID 和精确的 `ROLLBACK <cutover-id>`。Agent 会重新计算活动树摘要；切换后的任意新写入都会阻断自动回退。无新写入时，新代被归档为 rolled-back generation，旧代恢复到原活动路径。若进程在两个 rename 之间中断，prepared 状态会恢复旧代或完成状态识别，避免把半完成操作当成成功。

后续版本在保持 v1 兼容的前提下增加：MSSQL、外部对象存储清单、分卷与断点续传、签名清单，以及源实例与隔离副本之间更高层的用户、权限、Issue、PR 和 Actions 差异报告。当前流程始终遵循“预览 → 明确确认 → 隔离写入 → 验证 → 原子切换 → 保留两代 → 有条件回滚”。
