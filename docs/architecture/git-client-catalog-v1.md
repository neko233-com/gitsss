# Git 客户端目录 v1

`GET /api/v1/git-clients` 是公开、只读的安装来源目录。React 客户端不保存下载地址，也不会在按钮点击后伪造“已打开镜像”的成功状态；页面只渲染服务端实际返回的来源、版本、架构、命令、大小与 SHA-256。

Windows 目录固定到 Git for Windows 的明确 release tag，同时提供 GitHub 官方资产与已验证可访问的国内镜像。相同架构的两个来源必须声明同一个 SHA-256 和字节数，使用者可以在下载后独立核对。目录更新时，维护者必须从 GitHub Release API 读取官方 asset digest 和 size，并分别对官方与镜像 URL 做可达性检查；不能使用 `latest` 可移动链接。

macOS 官方已不再维护独立二进制安装器，因此目录只提供 Xcode Command Line Tools、Homebrew 和国内 Homebrew 镜像配置，不分发停留在旧版本的安装包。命令会原样显示给用户，不由网页自动执行。

v1 目录编译进受版本控制的 Go 服务，`trust` 为 `embedded-release-catalog`，并携带 `verified_at`。这使六平台发行包、源码审阅与页面显示绑定到同一提交。后续若允许管理员在线替换目录，必须先加入 canonical JSON、Ed25519 签名、可信公钥轮换和审计，不能直接读取任意远程 JSON。
