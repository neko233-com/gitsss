# gitsss 重构计划

## 产品目标

gitsss 是面向团队所有成员（不仅是开发者）的自托管代码协作与交付平台。它以清晰的下一步操作为中心：用户能从首页看见团队状态，使用 `Ctrl+K` 立即找到对象或执行动作，并通过短、可撤销的向导完成授权、仓库、通知和发布配置。

首个可发布版本必须同时满足以下结果：

- 以 **gitsss** 作为唯一产品、CLI、页面、文档与交付品牌；不保留面向用户的 Gitea 名称、Logo、图标、链接或文案。
- 运行于 Go 1.26.x，采用 React 19、Vite 8、TypeScript 7、shadcn/ui 与 ReactBits 构建新的前端应用；ReactBits 只承担有反馈价值的轻量动画，并尊重 `prefers-reduced-motion`。
- 服务端只使用随二进制内置的 SQLite 数据库，不要求用户部署 PostgreSQL、MySQL、Redis、Elasticsearch 或外部任务队列；接管旧实例所需的 PostgreSQL/MySQL 读取驱动也编译进 Agent。
- 首页为精简的可操作工作台，而不是传统仓库列表：数据大盘、近期事件流、存储感知和 Git 客户端下载为四个顶层 Tab。
- 近期事件流以有界的 LRU 策略维护每位用户的短期预览数据，避免无限增长。
- 可安全接管既有内网 Gitea 实例：先评估、备份、验证，再切换；不以直接覆盖生产数据库为迁移方式。
- 提供可审计的通知与 CI/发布能力，并让管理员能以向导完成配置。
- Actions Runner 在安装/升级时由 gitsss 自动部署、注册、健康检查和修复；管理员不需要另行安装一个分离服务。
- 提供跨平台 gitsss Agent、原生 CLI 与受自定义 Token 保护的 HTTP API，用于内网接管、备份和受控运维。
- 支持可验证的批量备份/导入、macOS 与 Windows 开机自启，以及签名校验的客户端和服务器更新。
- 面向游戏资产提供存储预算、增长预测、容量预警、仓库级归因、二进制传输优化和高性能克隆建议；不改变 Git 对象或美术资源本身的可验证性。

> 品牌重构不移除开源许可证、第三方版权和上游来源等法律要求的保留内容；这些内容将集中于 `NOTICE`、许可证及依赖声明中，不出现在产品体验或对外品牌中。

## 当前基线（2026-07-14）

- 已建立 `neko233-com/gitsss` 私有仓库与 `main` 分支；新的 `PLAN.md`、Go 服务和 React 客户端已提交，原 Gitea 源码快照仍作为未跟踪的兼容输入保留在工作区。
- `server/` 已提供自定义 Token、审计、事件预览、真实卷容量与保留空间保护、持久化 bare 仓库目录、受独立 clone/push 作用域保护并经真实 Git 客户端验证的 Smart HTTP、仓库树/文件/搜索 API、Runner 调和、通知 Outbox、Gitea `app.ini` 只读接管评估与审计上报、loopback 限定的本地 Agent HTTP、由 Agent 主动领取的类型化 AI 作业协议，以及带逐文件 SHA-256 清单、分块 AES-GCM、原子提交、只读完整性验证、导入冲突预览、显式确认隔离恢复和 Git/LFS 恢复审计的加密备份 v1；主服务已默认创建权限收紧、WAL、FULL 同步、busy timeout 和版本化 schema 的内置 SQLite 文件，自定义 Token 摘要与过期/撤销状态、仓库目录、Agent 作业、接管评估、飞书/企微/邮件配置、通知 Outbox、按 Token ID 隔离的 Dashboard 事件预览与主服务审计记录均通过重启恢复测试；首次启动凭据只创建 24 小时 bootstrap，具名 Token 由服务生成 256 位随机明文并只返回一次，数据库仅保存摘要，签发/撤销受作用域控制并审计；Dashboard 预览正文加密、主体 ID 哈希化并按事件 ID 执行有界 LRU，清空只影响当前主体且不会淘汰合规审计；通知密钥、正文和审计记录使用独立主密钥进行 AES-256-GCM 身份绑定加密，Windows 使用原生受保护 DACL，审计保留期清理以单个事务删除持久化记录，敏感操作遇到审计落盘失败会显式失败，投递状态具备持久化 claim、指数退避、死信和崩溃后 at-least-once 恢复；独立 loopback Agent 也默认打开用户级受保护 SQLite 与主密钥文件，审计加密跨重启恢复并在清理或追加失败时关闭对应请求；AI 作业已真实贯通只读评估、冻结证据、加密导出、隔离恢复、恢复审计和影子差异预检；SQLite 使用内嵌 online-backup API 吸收 WAL，PostgreSQL/MySQL 使用编译进 Agent 的纯 Go 驱动导出确定性 JSONL 快照，不依赖 `pg_dump`、`mysqldump` 或目标侧数据库服务；远程源库备份必须绑定明确的旧 Gitea 停服证据，影子预检会再次生成快照并比较 SHA-256；常驻 Agent Worker 已支持环境/受保护文件 Token、可取消请求、空闲/故障退避和完成回传重试去重，并提供不含密钥的 Windows Task Scheduler、macOS launchd、Linux systemd 计划、无覆盖失败回滚的本机安装/卸载事务，以及绑定版本/commit/平台/大小/SHA-256 的 canonical Ed25519 更新验证和有界、无链接、失败清理的隔离 staging；稳定 `gitsss-launcher`、不可变 generation active-state、独占激活锁、启动回执、认证健康探测和失败时更高 generation 回滚已接入 autostart 更新事务；`web/` 已提供首页四 Tab、命令面板、仓库浏览、通知连接和七阶段线性 AI 接管向导，并通过 ReactBits `AnimatedContent` 的按需 GSAP 分包为 Tab/功能区切换提供克制动画；快速自动化测试会逐字节验证 PSD、FBX、MP3、MP4 的加密备份与恢复，并验证 SQLite WAL、数据库漂移、有提交/空 bare Git、LFS、篡改阻断和 live refs 漂移；GitHub Actions 被仓库级 `AGENTS.md` 明确禁止，交付只走本机完整验证、Linux/macOS/Windows 六平台打包、远端 digest 复核与 `gh` 直传；此前 alpha 只作为开发基线保留，后续不再按功能发布，必须完成全部范围审计后一次性发布整合版本。
- Gitsss Pipelines 已以嵌入式 SQLite 运行队列、不可变 commit、固定本地模板、两分钟 lease、三次故障接管、日志限额/Token 脱敏和审计闭环落地；同一个签名 Agent 包提供 `pipeline-work`，`service-install --runner` 会生成独立的 Windows Task Scheduler、macOS launchd 或 Linux systemd 自启动服务，不依赖 GitHub Actions 或另行下载的 Runner。仓库代码只在专用 Runner 服务账户下执行，命令/工作目录/超时不能由 API 调用者注入。
- 原生 Git LFS Basic 已与 Smart HTTP、仓库 Token、审计和真实卷写入保护打通：按 SHA-256 OID 跨仓库去重、同卷 no-replace 发布、GET/HEAD/Range、顺序 `Content-Range` 续传和 PSD 真实客户端逐字节 push/clone 测试均已落地；新建托管仓库默认启用 protocol v2 部分克隆 filter、reachable wants、commit-graph 与 bitmap 配置，自动化测试以 `--filter=blob:none` 验证 promisor，而不是只在文档宣称高性能。
- 原 Gitea 模块名仍为 `gitea.dev`，并含 Vue 3 前端与大量 Gitea 品牌资产、文档和工作流；这些内容尚未接入新远端，也尚未达到可安全删除的替代覆盖率。
- 本机可用 Go 1.26.4、Node 24.14.0、pnpm 11.7.0；新 `web/` 已固定 Vite 8.1.4、TypeScript 7.0.2、React 19.2.7，未跟踪的上游兼容快照仍包含 Vue 3 与旧 TypeScript 依赖。
- Windows 环境没有 `make` 别名，但可使用 `mingw32-make`；后续应补一个兼容入口或在开发文档中明确该映射。
- 现有启动与迁移逻辑已经以 `app.ini`、工作目录和数据库迁移为边界，适合作为“接管评估器”和兼容适配层的起点。

## 非目标

- 不在首版重新实现 Git 协议、LFS、Actions 运行器或全部 Gitea 边缘功能。
- 不在未备份、未校验版本和未获得管理员凭据的内网实例上执行写操作。
- 不以全仓库机械替换完成改造；模块路径、协议、配置、数据库和界面会分层迁移并有回滚点。
- 不保留与 gitsss 目标无关、无人使用或已被新能力替代的上游集成、页面、脚本、运行器假设和 Legacy 兼容层；删除前必须具备依赖清单、迁移替代和测试证明。

## 目标架构

```text
React 19 + Vite 8 + TypeScript 7 + shadcn/ui
              │ HTTPS / SSE
Go 1.26 gitsss API ── command/search/notification services ── embedded SQLite
              │
Compatibility gateway ── Gitea config + DB + Git repos + object storage
              │
gitsss Agent (CLI + authenticated local HTTP) ── existing Gitea intranet server
              │                                 (read-only assessment → staged takeover)
              └── macOS / Windows startup and signed self-update
```

### 前端与设计系统

- 在 `web/` 建立独立 React/Vite 应用，逐步替代 `web_src/` 的 Vue 页面，避免在一个页面内混用两套渲染模型。
- Vue 前端作为待删除 Legacy：React 的路由、构建、测试和主流程验证完成后，删除 `web_src/`、Vue 依赖、Vue Vite 插件、Vue 专用测试/文档与旧构建目标；不得长期保留双前端。
- 用 shadcn/ui 作为可访问的基础组件，不采用其默认视觉作为产品外观；建立 gitsss tokens、动效、焦点、阴影、空白和深浅色主题。
- 视觉方向为 Apple/Spotlight：留白优先、单一主操作、短层级、键盘优先、轻量半透明浮层，支持 `prefers-reduced-motion`。
- 页面采用精简的 API 工作台式信息层级：一个当前 Tab、一个主要决策区、一个可展开的细节区；禁止把所有监控数据同时堆满首页。
- 重新设计品牌标志与完整的产品图标系统（导航、状态、操作、通知、CI），禁止复用 Gitea 标志、Octicons 品牌隐喻或旧图标资源。
- 先交付可审阅的设计基线：导航、命令面板、数据卡、列表、详情、空状态、表单、移动端与关键交互状态；再批量实现页面。

### 面向非开发者的线性体验

- 首页只呈现“需要处理”“正在进行”“团队健康度”三类信息，并给出下一步按钮。
- 常见任务使用向导：连接实例 → 验证权限 → 预览影响 → 执行 → 可下载的结果与回滚说明。
- `Ctrl+K` / `Cmd+K` 全局命令面板支持模糊搜索仓库、成员、PR、工单、运行记录和设置，也支持直接执行安全动作；每项显示作用域与权限提示。
- 把复杂授权收敛为角色模板、最小权限推荐、即时预览、有效期和审计记录；SSO/OIDC、LDAP、SCIM、PAT、SSH Key、Passkey 分阶段接入。

### 平台能力

- **数据大盘**：待审 PR、失败构建、发布风险、仓库活跃度、交付周期、通知失败与安全待办；所有指标可追溯到原始对象并按权限裁剪。
- **首页 Tab**：数据大盘聚焦下一步与团队健康度；近期事件流聚焦可继续活动并以 LRU 清理预览；存储感知聚焦容量、增长和风险；Git 客户端下载为非开发人员提供操作系统、官方源/镜像源、校验和和安装向导。
- **事件流**：显示最近活动、需要关注的变化和可继续的操作；预览索引按用户和作用域设置容量与 LRU 淘汰，持久化业务数据及合规审计记录不受此淘汰影响。
- **存储感知**：实时采集数据库、Git 对象、LFS、附件、构建制品和备份的占用与增长率，提供配额、预测、阈值预警、写入保护和逐仓库/路径归因。容量不足时先阻止高风险新写入并展示可理解的清理/扩容向导，绝不静默删除用户数据。
- **游戏资产与克隆性能**：保持 Git SHA 和 LFS OID 兼容；为 `.psd`、`.fbx`、音频和视频提供大文件识别、分块/可续传 LFS 传输、对象存储生命周期、去重统计和二进制预览元数据。启用 Git protocol v2、commit-graph、multi-pack-index、bitmap、部分克隆/稀疏检出、bundle URI、并发 LFS 下载和局域网缓存；不对已压缩媒体做无收益的二次压缩。
- **仓库浏览与搜索**：仓库页提供常规 Git 视图之外的固定左侧文件树、按路径/内容/符号搜索、文本预览和二进制资产元信息；大文件搜索经权限过滤、索引限额和审计后执行。
- **搜索**：统一索引接口、权限过滤、快捷键、最近访问、命令与对象的统一排序；索引不可用时降级到数据库查询。
- **通知**：事务 outbox、模板、规则、偏好、抑制、重试/死信和投递审计；首批适配器为邮件、飞书和企业微信，预留 Slack、Webhook 与自定义渠道。
- **CI 与发布**：将现有 Actions 能力包装为 Gitsss Pipelines，提供模板化流水线、运行日志、制品、环境审批、发布记录和回滚入口；gitsss 自身的发布只允许本机可信脚本与 `gh` 直传，不使用 GitHub Actions。
- **开箱即用 Runner**：安装器和 Agent 根据平台自动拉起、注册和升级受控 Runner；Runner 状态、工作目录、磁盘预留、失败自愈和离线告警在产品内可见，且支持离线内网软件包源。
- **备份与导入**：提供带版本清单、校验和、加密元数据与演练恢复的批量导出/导入；数据包涵盖内置数据库、Git 仓库、LFS、附件、配置映射和扩展数据，并支持断点续传与差异报告。旧 PostgreSQL/MySQL 仅作为接管数据源，由 Agent 内嵌驱动读取后迁入 gitsss 内置库。
- **Agent、CLI 与 HTTP**：提供原生 `gitsss-agent` 与 `gitsss` CLI；Agent 本地 HTTP API 以签发/轮换/撤销的自定义 Token、作用域、过期时间、设备身份和重放防护保护，默认仅监听 loopback 或受控私网地址。
- **安全与运维**：敏感动作写入追加式审计日志，记录发起人、Agent/设备、目标、授权、请求摘要、结果与关联 ID；事件预览可 LRU 清理，审计日志则按审批的保留策略分层归档和删除，不能被 LRU 清除。密钥加密、最小化服务账户、健康检查、指标、备份验证、结构化日志和升级前检查为必备能力。
- **跨平台交付**：macOS 采用 `launchd`，Windows 采用 Windows Service 或 Task Scheduler（由安装向导选择），服务器采用 systemd/容器；所有更新包、Agent 和服务端二进制必须签名、可分批发布、可健康检查并可回滚。

### Gitea 内网接管策略

1. Agent 注册与只读发现：管理员在内网主机安装 Agent，通过一次性注册令牌建立设备身份；Agent 在本地读取指定 `app.ini` 和环境覆盖，识别 Gitea 版本、数据库、仓库根目录、LFS、对象存储、OAuth、SMTP、Actions、LDAP 与外部 URL。
2. 预检与冻结：验证连接、权限、容量和版本兼容性；远程源库要求旧 Gitea 本地监听明确停止，超时或网络不可达不能冒充停服证据。
3. 备份与验证：内嵌驱动生成数据库/Git/LFS/附件备份清单；所有敏感字段只显示掩码，冻结 ID、评估 ID 和容器摘要贯穿后续步骤。
4. 影子模式：gitsss 以只读方式连接生产副本，比较用户、组织、仓库、权限、Issue/PR、Actions 和附件计数，输出差异报告。
5. 试运行迁移：对克隆实例执行可重复导入，验证 Git SHA、LFS OID、权限、链接和通知；通过后才允许产生切换计划。
6. 受控切换：由 Agent 在维护窗口内执行已签名的计划，最终复核、健康检查、DNS/反向代理切换，并保留原实例与明确的回退步骤。
7. 兼容期：保留 Gitea 配置映射、旧 URL 重定向与管理命令别名，直到每个实例完成验收后再删除。

## 交付阶段

### 0. 仓库与基线

- 创建 `neko233-com/gitsss` 私有 GitHub 仓库，初始化本地 Git 与 `origin`，提交本计划作为独立的首个提交。
- 建立 `docs/architecture/`、`docs/migration/`、`NOTICE`、品牌资产目录和 ADR 模板。
- 建立可重复的 Windows/Linux 开发入口，确认 `mingw32-make` 与 CI 中的 `make` 目标一致。
- 记录现有构建、测试、迁移和许可证基线；随后将旧上游工作流替换为 gitsss 所有者与发布策略。
- 创建功能/依赖清单，将源码划分为“核心保留”“接管兼容期”“可选插件”“待删除 Legacy”四类；先禁止新增对待删除层的依赖，再按小批次移除代码、资产、文档、CI、第三方集成和配置项。

### 1. 品牌与可运行外壳

- 将二进制、模块路径、环境变量、配置路径、默认 URL、产品元数据和文档改为 gitsss；按包/接口逐步迁移 import path，并在每步执行 `go mod tidy`。
- 替换登录、安装、错误页、邮件、CLI 帮助、API 信息和静态资源中的可见品牌。
- 新建 gitsss 设计 token、标志和图标目录，完成 App Shell、响应式导航、通知中心与命令面板。
- 逐步移除旧 Vue 页面、无人使用的功能开关、废弃配置、上游专用机器人/发布脚本、重复前端依赖和项目无关集成；每次删除均以引用扫描、构建和兼容测试为准。
- 退出标准：不扫描到任何面向用户的 Gitea 文案或资产引用；许可证和 `NOTICE` 中的合规保留项除外。

### 2. React 数据大盘与核心工作流

- 实现 `/` 的四个首页 Tab：大盘、事件流、存储感知和 Git 客户端下载；工作队列、仓库概览、PR/Issue/流水线详情和全局搜索由权限感知的聚合 API 提供数据。
- 以“选择对象 → 选择动作 → 预览 → 确认 → 结果”的统一流程替换高频且易出错的旧交互。
- Git 客户端下载 Tab 根据 macOS/Windows、CPU 架构和网络可达性提供官方与经过签名/校验验证的镜像源；源地址、版本与校验和来自可审计的服务器端目录，不能硬编码不受控第三方链接。
- 为大盘、命令面板、权限变更和移动端核心路径编写单元、组件与 Playwright 测试。
- 退出标准：非开发者可在不理解 Git 术语的情况下完成查看风险、请求权限、创建仓库和启动模板化发布。

### 3. 身份、授权与通知

- 建立统一身份领域模型、角色模板、资源范围、审核、审计和短时授权；首先兼容现有本地账号、LDAP 和 OIDC。
- 实现通知策略、邮件、飞书和企业微信适配器，支持连接测试、签名校验、重试、失败告警与投递审计。
- 退出标准：每次权限和通知策略变更均可预览、追责和撤销；通知失败不会静默丢失。

### 3A. Agent、备份与跨平台运维

- 实现 `gitsss-agent`、`gitsss` CLI 和本地 HTTP API：注册、能力发现、任务领取、进度流、离线重试、诊断包与安全注销。
- 实现自定义 Token 生命周期：服务端只存哈希、最小作用域、短有效期、轮换、撤销、设备绑定、IP/网络策略和审计关联；不得以 URL 查询参数传递 Token。
- 实现批量导出、加密导入、清单校验、断点续传和恢复演练；导入前必做冲突预览，导入后输出对象、Git SHA 与 LFS OID 差异报告。
- 打包 macOS universal、Windows x64/ARM64 与 Linux Server 发行物；实现用户选择的开机自启、静默更新策略、签名验证、健康探测和回滚。
- 退出标准：隔离网络中的 Agent 可在不暴露数据库凭据给控制台的前提下完成只读发现、加密备份、试运行导入与受控接管。

### 4. CI、制品与发布

- 提供 Gitsss Pipelines 概览、模板、环境、审批、制品、发布及回滚页，并兼容现有 Actions 工作流的运行与日志。
- 内置 Runner 生命周期管理：在一键安装、Agent 接管和服务器更新中自动部署/注册 Runner，验证容器/本地执行器能力，并在磁盘压力和失败时做可审计的暂停或恢复。
- GitHub Actions 保持关闭；本机可信发布脚本执行格式化/静态检查、单元测试、构建、SBOM、跨平台签名包、更新清单、远端摘要复核与 GitHub Release 上传，不继承上游专用 runner、S3、机器人或镜像命名。
- 退出标准：从提交到可审计发布具备可复现流水线、制品来源和回滚记录。

### 5. 内网接管、性能与正式发布

- 实现由 Agent 驱动的只读发现器、配置映射、预检、影子模式、差异报告、备份导入演练、切换和回滚命令。
- 增加游戏资产存储与传输基准：大 Git 仓库、LFS、PSD/FBX、音视频、部分克隆、LAN 缓存和并发 Runner；用 P50/P95 克隆、首包、LFS 吞吐、存储放大率和恢复时间评估优化，而非只宣称“更快”。
- 为支持的 Gitea 版本、SQLite/MySQL/PostgreSQL、文件/对象存储、LDAP/OIDC 和 Actions 建立矩阵测试。
- 完成安全评审、备份恢复演练、压测、无障碍审计与生产运行手册。
- 退出标准：在隔离克隆中完成端到端接管演练，并由实例管理员确认数据、权限、Git/LFS 和运行路径。

## 质量门槛

- Go 改动执行 `make fmt`、`make lint-go`；TypeScript 改动执行 `make lint-js`。在 Windows 使用 `mingw32-make` 作为兼容入口；若本机缺少 Makefile 依赖的 POSIX 工具，则直接运行等价的格式化、静态检查和测试命令并记录差异。GitHub Actions 不作为任何质量或发布入口。
- 单元测试优先；单个 Go 测试使用 `go test -run '^TestName$' ./modulepath/`，单个前端测试使用 `pnpm exec vitest <path-filter>`，单个 E2E 使用 `GITEA_TEST_E2E_FLAGS='<filepath>' make test-e2e`（完成品牌迁移后同步改名该变量和目标）。
- 每个新 UI 功能必须含键盘操作、焦点状态、屏幕阅读器语义、深浅色和移动端验证；所有命令面板操作必须有权限校验与确认语义。
- 每个迁移写操作必须先通过只读预检、备份验证和可重复演练；迁移报告不得输出密钥或令牌。
- Token、Agent 注册、备份导入、权限变更、服务器更新和切换操作必须进行威胁建模、端到端鉴权测试、篡改检测和审计测试。
- LRU 只可用于可再生的事件预览、缓存或最近访问数据；敏感审计日志必须遵循已批准的保留、归档和安全删除策略。
- 每个新 `.go` 文件带 2026 年版权头；所有提交遵循 Conventional Commits，并含 `Assisted-by: AGENT_NAME:MODEL_VERSION` trailer。

## 决策记录与待确认输入

- 默认远程仓库设为私有，避免在重构前公开包含上游源码快照的内容；需要公开时再显式调整可见性。
- gitsss 运行时数据库固定为内置 SQLite；不提供外接数据库部署模式。旧 SQLite/MySQL/PostgreSQL 只存在于接管读取边界，数据转换完成后不成为 gitsss 运行依赖。
- 接管实施前需要目标实例的版本、数据库类型与备份方式、`app.ini`（脱敏）、Git/LFS/对象存储路径、身份源、反向代理和允许的维护窗口。
- Agent 实施前需要目标主机的操作系统、CPU 架构、网络边界、代理/证书策略、可用服务账户、允许的监听地址与更新策略。
