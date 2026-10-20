# Codex 工具适配

本项目安装的是 dsh-oil-ui 内附的免费 Skill，未安装 DeepSeek Harness 插件。
以下命令从仓库根运行。Python 需 3.10+；Node 使用项目 engines 所要求的兼容版本。
命令参数以脚本当前 `--help` 为准。

| 上游概念 | 在本项目执行 |
| --- | --- |
| `oil_ui_skill_status` | 读取 `SOURCE.json` 和 `SKILL.md`；本地固定版本，无状态服务/自动更新 |
| `oil_ui_explorer` | `python .agents/skills/oil-ui/scripts/build_explorer.py <manifest路径> --output <输出HTML>` |
| `oil_ui_shoot` | 真实应用优先现有 Playwright/用户指定浏览器；离线设计小样可用下面的无依赖截图 CLI |
| `read_image` | 当前宿主 `view_image` 等实际看图工具 |
| `oil_ui_media` / `oil_ui_image` | 按任务使用宿主已授权的检索/生图工具；工作台普通调整不需要素材下载 |
| `subagent` | 仅在用户或宿主工作流允许时独立评审；否则自己检查并标明没有独立评审 |

## 离线设计小样

上游 `references/tools.md` 的 `oil_ui_*` JSON 是 DSH 专用语法，不当作当前工具调用。
CLI 不要求 pnpm 安装，也不改根 package.json：

```text
python .agents/skills/oil-ui/scripts/build_explorer.py --help
node .agents/skills/oil-ui/scripts/shoot.mjs --help
```

有多个真实不同的方向需要比较时，按 `style-explorer.md` 写 manifest；
小改动与已确定方向不为了凑数创建三个方案。比较真实页面时仍以实际应用交互为准。

```text
python .agents/skills/oil-ui/scripts/build_explorer.py output/ui-review/<任务名>/manifest.json --output output/ui-review/<任务名>/style-explorer.html
node .agents/skills/oil-ui/scripts/shoot.mjs output/ui-review/<任务名>/candidate.html --size 1600x1000 --out output/ui-review/<任务名>/shots
```

`shoot.mjs` 默认使用移动端尺寸，因此桌面任务显式传 `--size`。
脚本会启动独立临时浏览器配置与回环 HTTP 服务，不复用用户登录态；需要指定浏览器时，
可在当前 shell 设置脚本支持的 `CHROME_PATH` 为本机 Edge/Chromium 路径。
认证页面用项目现有真实浏览器验证链，不把密码、Token 或 Cookie 写入 CLI、脚本与截图。

截图 CLI 能发现部分溢出、断图、对比度与控制台问题；报告或截图文件生成成功不代表
业务通过或完整无障碍认证。先实际打开图，再按流程检查键盘、失败恢复、读回与权限。

## 版本与更新

`SOURCE.json` 记录固定 commit、上游路径、版本、每个原始文件的 SHA-256 和本地适配文件。
原始入口改存 `UPSTREAM.md`，其相对引用仍有效；其他上游资产、脚本、分册和 MIT/OFL 通知保留。
字体 OFL 通知仅清理行尾空格，许可文字未变；该项同时记录上游原始与安装后哈希。

不执行 `check_update.*`、`recommend_once.py` 或 DSH 自动更新/Pro 推荐流程。
如用户要求更新，在仓库外暂存目录用 skill-installer 的固定 `--ref` 下载，对比上游差异，
保留项目入口与两个适配分册，更新来源和校验值。先检查本地改动，不直接覆盖当前安装。
不复制源仓库的 `.github/workflows/`、`lib/`、`src/` 或插件 package.json。
