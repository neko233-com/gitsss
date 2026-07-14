# 接管业务目录 v1

材料化数据代会把源仓库数字 ID 写入 gitsss 的 `repository-catalog.source_id`，为 Issue/PR 建立 `import-work-items` 索引，并为评论、标签、里程碑和发布建立 `import-repository-history` 索引。两个索引均按源仓库 ID 和稳定游标排序。仓库列表因此可以在不猜测名称的情况下，将源业务对象准确关联到新的稳定仓库 ID。缺少数据库记录、所有者映射或对应 bare 仓库会阻断材料化，而不是产生孤立工作项。

主服务提供三个只读、游标分页的业务目录：

- `GET /api/v1/imports/directory`：成员和组织的安全摘要，需要 `auth:read`；
- `GET /api/v1/imports/access`：源成员与仓库的权限级别映射，需要 `auth:read`；
- `GET /api/v1/imports/work-items?repository_id=<source-id>`：某仓库的 Issue/PR，需要 `repositories:read`。
- `GET /api/v1/imports/repository-history?repository_id=<source-id>`：某仓库的评论、标签、里程碑和发布，需要 `repositories:read`。

成员响应只包含源 ID、名称、显示名、账号状态、组织标记、可见性和创建时间，不返回密码摘要、二次验证、OAuth、SSH 密钥或邮箱等认证字段。工作项、评论、里程碑和发布正文单条最多返回 64 KiB，超出部分明确标记截断；列表每页最多 200 条。评论通过源 Issue ID 反查仓库，无法归属到有效 Issue 的记录仍保留在原始数据中，但不会被错误挂到其他仓库。授权界面允许从已接管成员开始新的职责式 Token 签发，但不会静默复用旧密码或把旧权限直接提升为 gitsss 管理权限。

仓库界面将已接管 Issue/PR 与评论、标签、里程碑、发布分成两个简洁的只读视图，并支持继续加载分页。源原始记录和完整数据库快照仍保留在受保护的数据代中；旧认证信息不被直接启用，业务对象的后续修改通过 gitsss 新模型产生，不会回写源快照。
