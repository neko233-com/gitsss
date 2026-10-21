# git-next：下一代 Git 客户端

[Git-Terminal 0.2.0](https://github.com/neko233-com/git-terminal/releases/tag/v0.2.0) 提供 SSH 分组管理、SFTP 拖拽上传/右键下载和只读 Linux 监控。组件更新保留 git-next 0.3.0 核心；已有安装使用匹配架构和 user/machine 范围的 MSI，附加 `REINSTALL=ALL REINSTALLMODE=vamus`。Git 产品自动更新保持原有行为，此独立终端组件更新需手动安装。

[Windows 完整新版 MSI](https://github.com/neko233-com/git-terminal/releases/tag/v0.2.0) · [Linux / macOS 正式版](https://github.com/neko233-com/git-next/releases/latest) · [安装与更新文档](https://github.com/neko233-com/git-next/blob/main/README.md)

git-next 保持 Git 的命令行、仓库格式、协议和 LFS 使用习惯。安装后的默认命令仍是 `git`，`git --version` 会明确显示 git-next 身份。GitSSS 的首页、导航与帮助文档都提供本页入口，下载无需 GitSSS 账号。

## 选择安装包

| 系统 | 安装包 |
| --- | --- |
| Windows x64 | `windows-x64.msi`；无管理员权限时选 `windows-x64-user.msi` |
| Windows ARM64 | `windows-arm64.msi` 或 `windows-arm64-user.msi` |
| macOS Apple Silicon | `macos-arm64.tar.gz` |
| Linux x64 / ARM64 | 对应架构的 `linux-*.tar.gz` |

文件名带实际版本前缀；请在正式版附件中选择，并用同一版本的 `SHA256SUMS` 校验。macOS 当前仅发布 ARM64 包，不能把交叉打包当成真实 Mac 验收。

## Windows 安装与接管

关闭正在执行写操作的 Git / 终端，运行 MSI。组合安装包包含 git-next、内置 Git LFS、自有 Bash 和独立 Rust + GPUI Fast 的 Git-Terminal；没有 Git GUI。机器版与用户版使用不同安装范围，不要混装。

安装保留仓库、SSH 密钥、known_hosts、Git 配置和凭据存储。接管和回滚采用客户端保护的迁移事务。安装后重新打开终端并检查：

```powershell
where.exe git
git --version
git language get
git update check
```

`git` 应首先解析到 git-next 安装目录，版本行包含 `git-next`。已经运行的进程可能仍持有旧 PATH，应重开该进程。手动选择语言可用 `git language set chinese`、`english` 或 `auto`。更新器从官方 GitHub 获取版本身份，比较官方及镜像下载路线，并校验下载字节；镜像不决定版本身份。

## LF 与已有仓库

默认采用 LF，文本规范化比较避免纯换行符造成的假改动。已有 `.gitattributes`、自定义 hooks 和二进制资源规则保留；安装不会改写仓库历史。Unity / Tuanjie 的文本资源可明确声明 `text eol=lf`；纹理、FBX、音视频和二进制资源采用对应 LFS 规则，不强制所有文件作为文本。

## Git-Terminal

Ctrl+T 新建标签，Ctrl+W 关闭，Ctrl+Tab 切换。原生一体化标题栏带应用图标、拖动与系统窗口控制；本地支持 Bash、PowerShell 和 CMD。Bash 输入高亮、目录与分支提示保留标准 readline 行为。

SSH 管理、SFTP 和远程监控的使用、认证与验证边界以独立 [Git-Terminal 手册](https://github.com/neko233-com/git-terminal/blob/main/README.md) 和对应发布附件为准。GitSSS 服务端与客户端分别安装，不会因为下载客户端而替换服务器的 Git、SSH 或存储目录。

## 开发与验收边界

GitSSS 仅记录同级 `git-next/` 与 `git-terminal/` submodule；源码、依赖和构建独立。检查子仓库时显式运行 `git -C git-next status` 与 `git -C git-terminal status`，父仓库默认忽略其脏状态。只在完整范围本地回归、包装和校验通过后发布综合版本；不创建或运行 GitHub Actions。客户端验收回执见 [公开记录](https://github.com/neko233-com/git-next/tree/main/docs/benchmarks)。

使用 Git-Terminal 的 `tools/install_bundle.ps1 -Package <MSI 路径>`，自动沿用原 MSI 源文件名并更新同版本配套文件，同时保留原组件标识和许可文件。
