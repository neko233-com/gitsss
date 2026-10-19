# Google 账号登录

1. 进入 [Google Cloud Console](https://console.cloud.google.com/)，选定项目，在 Google Auth Platform 配置 Branding、Audience 和 Data Access。企业内部受众与外部受众按实际主体选择；测试模式添加被允许的测试用户。
2. 创建 OAuth Client，应用类型选择 **Web application**。不是桌面应用、API key 或服务账号。
3. 添加完整 Authorized redirect URI，例如 `https://git.example.com/user/oauth2/google/callback`。使用供应方认可的 HTTPS hostname 与证书；localhost 开发例外不代表任意私有 IP 的 HTTP 都能使用。
4. GitSSS 添加 OAuth2 源，名称建议 `google`，Provider **Google**，填写 Client ID 和 Client secret。内部兼容键仍为 `gplus`，不要为改显示名称重写旧绑定。
5. 填写与白名单一致的独立回调站点并启用。GitSSS 请求基本身份与邮箱，并使用 `prompt=select_account`：浏览器已登录 Google 也会选择账号。
6. 登录原 GitSSS 账号后绑定，退出再用 Google 登录验证。Google 返回明确验证为真的邮箱且未被其他账号使用时，GitSSS 可加入邮箱别名，**不覆盖手填主邮箱**。

## 常见错误

| 现象 | 原因与处理 |
| --- | --- |
| redirect_uri_mismatch | 完整 URI 与 Google 白名单不一致，检查源名称、协议、端口、子路径、大小写 |
| 测试用户被拒绝 | 应用仍为 Testing，用户未加入测试名单，或受众选择为其他企业内部 |
| 内网 HTTP 被拒绝 | 配置合规 HTTPS 域名；GitSSS 允许某种回调格式不代表 Google 接受 |
| 邮箱未加入 | 供应方未明确验证邮箱或地址已被另一账号占用；不抢占、不覆盖主邮箱 |
| 选择了错误 Google 账号 | 重新发起登录选择账号；已绑定身份替换必须通过原 GitSSS 账号的安全设置 |

不要申请 Gmail、Drive、Calendar 等与登录无关的权限。正式发布前按 Google 控制台的受众与验证要求操作。参考：[OAuth Web Server](https://developers.google.com/identity/protocols/oauth2/web-server)、[OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect)。
