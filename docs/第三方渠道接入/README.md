# 第三方渠道接入

本目录记录申请应用、配置 GitSSS、绑定账号和真实登录验收。默认保留邮箱／用户名＋密码，一个 GitSSS 账号可以同时绑定多个渠道；未配置完整或未启用的渠道不会显示登录按钮。

| 渠道 | 申请与配置 | GitSSS Provider | 需要准备 |
| --- | --- | --- | --- |
| 飞书 | [飞书.md](飞书.md) | `feishu` | 企业自建应用 App ID、App Secret、可用范围、发布版本 |
| 微信 | [微信.md](微信.md) | `wechat` | 已审核的网站应用 AppID、AppSecret、授权回调域 |
| Google | [Google.md](Google.md) | `gplus` | Web application Client ID、Client secret、同意屏幕 |
| Discord | [Discord.md](Discord.md) | `discord` | OAuth2 Client ID、Client Secret、Redirect URI |
| 邮箱 | [邮箱.md](邮箱.md) | 本地账号 | 唯一邮箱、原密码；发信功能另配 SMTP |

HTML 版：[第三方渠道接入手册](../site/identity-channels.html)。英文说明：[Enterprise identities](../features/enterprise-identities.md)。

## 所有渠道通用的顺序

1. 确定用户实际访问的站点，例如 `https://git.example.com/`，并固定认证源名称，例如小写 `feishu`。名称参与回调路径，避免上线后随意改名。
2. 在渠道官方开发者平台创建对应应用，记录 Client ID／App ID；Secret 直接填入受保护的 GitSSS 管理表单，禁止进入 Git、工单、截图或操作日志。
3. 进入 **管理后台 → 认证源 → 添加认证源 → OAuth2**，选择对应 Provider，配置名称、ID、Secret、启用状态。每个源可独立填写「登录回调站点」。
4. 复制 GitSSS 表单显示的**完整回调地址**到渠道白名单。路径、大小写、协议、端口、子路径都必须一致。例如 `https://git.example.com/user/oauth2/feishu/callback`。
5. 完成渠道权限、企业可用范围、测试用户、审核及发布。仅保存开发者平台草稿通常不足以让全员使用。
6. 从回调所属的**同一站点**登录原 GitSSS 账号，进入 **设置 → 安全 → 已关联账号**绑定。随后退出，用渠道按钮重新登录，核对仍为原账号、组织和仓库权限。

企业部署保持自主注册和 OAuth 自动注册关闭。未绑定的渠道身份应进入已有账号确认／绑定流程，不能自动变成新管理员。最后一种有效登录方式不能移除；换绑应先验证新身份，成功后才替换旧绑定。

## 站点、Cookie 与网络

- 优先使用有效证书的 HTTPS。HTTP 只允许在 GitSSS 回调配置中用于内网 IP、回环 IP 或 localhost；这不代表供应方也接受该地址。
- 从 `https://git.example.com/` 开始绑定，却返回 `http://192.168.1.10:5030/`，会丢失原站点会话。选择一个入口完成整个流程。
- 用户浏览器必须能访问回调；GitSSS 服务器必须能访问供应方 token 和 user-info API。浏览器能打开授权页不代表服务器的外网请求也正常。
- 认证源 `CallbackURLBase` 优先于 `[oauth2_client] CALLBACK_URL_BASE`，最后回退 `[server] ROOT_URL`。只改 OAuth 回调，不改变 Git／SSH／LFS remote。
- 不在排障日志中记录 code、state、Cookie、access token、refresh token 或 Secret。仅记录脱敏错误码和时间。

## 验收与交接

记录应用名称、Provider、认证源名称、回调、负责人、可用范围、审核／发布状态和最近一次实际登录验收时间。不要记录 Secret。验证新增绑定、重新登录、已有密码登录、无权限账号拒绝访问、换绑、取消授权及错误回调。

关闭认证源可立即隐藏入口；已有账号与 Git 数据保留。若回滚到不识别独立回调站点的旧二进制，应先停用该源或恢复与旧 ROOT_URL 一致的回调。Secret 轮换通过供应方与 GitSSS 保护表单完成，操作前保留另一种已验证登录方式。
