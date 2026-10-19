# Discord 登录

1. 在 [Discord Developer Portal](https://discord.com/developers/applications) 创建应用，设置清晰名称和图标。
2. 进入 **OAuth2**，获取 Client ID／Client Secret，登记完整 Redirect URI，例如 `https://git.example.com/user/oauth2/discord/callback`。登录不需要创建 Bot，也不需要 Bot Token。
3. GitSSS 添加 OAuth2 源，名称建议 `discord`，Provider **Discord**，填入 Client ID／Secret 和回调站点，核对回调预览后启用。
4. 仅请求 Provider 用于身份的基本 scope，不申请服务器管理、机器人加入服务器或读取消息等权限。
5. 原 GitSSS 账号进入安全设置绑定；退出再进行真实 Discord 授权，核对仍是同一 GitSSS 用户与组织权限。

Discord 的通知 Webhook、Bot Token 与 OAuth Client Secret 是三种不同用途，不能混用。昵称可变，不能用昵称匹配或抢占账号。一个 Discord 身份不能同时绑定两个 GitSSS 账号；换绑与解除受到身份归属和最后登录方式保护。

如失败，先检查 Redirect URI 完全相同、源已启用、Secret 完整、应用可用以及服务器外网连通性。不要在排障截图中展示 Secret 或 access token。参考：[官方 OAuth2 文档](https://docs.discord.com/developers/topics/oauth2)。
