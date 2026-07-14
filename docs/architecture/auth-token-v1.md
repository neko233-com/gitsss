# 自定义 Token 生命周期 v1

主服务把自定义 Token 作为内置 SQLite 状态管理，不要求外部身份服务。数据库只保存 Token ID、SHA-256 摘要、排序后的作用域、创建时间、过期时间和撤销时间；256 位随机 Token 明文只在签发响应中返回一次，不进入列表、日志或审计记录。

## 首次启动

空数据库首次启动时，`GITSSS_API_TOKEN` 必须至少 24 字节。服务将它写成具备完整初始作用域、有效期 24 小时的 `bootstrap` 记录。数据库已有任意 Token 记录后，后续启动不再读取环境变量来覆盖、恢复或新增 Token，避免一个长期环境变量绕过撤销状态。

管理员必须在 bootstrap 到期前通过 `POST /api/v1/auth/tokens` 签发具名 Token。签发请求明确给出 Token ID、作用域和 1 至 8760 小时的有效期；服务生成 `gitsss_` 前缀的 256 位随机凭据。`GET /api/v1/auth/tokens` 只返回非敏感元数据。`DELETE /api/v1/auth/tokens/{id}` 持久化撤销时间，当前 Token 不能撤销自身。

## 权限与审计

- `auth:read`：查看 Token ID、作用域和生命周期状态。
- `auth:write`：签发或撤销 Token。
- 签发与撤销分别写入 `auth.token.issue` 和 `auth.token.revoke` 审计记录，只记录操作者、目标 Token ID、请求 ID 和时间。
- 签发后的审计落盘若失败，服务会补偿撤销刚生成的 Token，不把无法追踪的凭据返回给客户端。

校验使用固定长度摘要和常量时间比较。已过期或已撤销的 Token 不能认证；ID 不能重用，防止旧审计记录与新凭据混淆。v1 尚未提供丢失全部管理 Token 后的在线绕过机制，恢复应通过受控的离线运维流程实现。
