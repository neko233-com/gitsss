# GitSSS 架构与设计适配

下面的路径从仓库根目录解析。以当前源码为准，不为视觉改动迁移技术栈。

## 找到正确的修改层

| 页面与行为 | 权威源与约定 |
| --- | --- |
| 服务端页面、表单、导航 | `templates/` Go HTML 模板，`routers/web/` 原路由和表单处理 |
| 工作区壳与共享控件 | `web_src/css/features/workspace.css`、`workspace-controls.css` 及对应 TS 初始化 |
| 仓库、迁移、语言图例 | `templates/repo/`、`web_src/css/repo.css`、`web_src/css/repo/home.css`、`web_src/js/features/repo-*.ts` |
| 通用浏览器逻辑 | `web_src/js/features/`、`utils/`、`components/`；保留 Fomantic/jQuery 的现有消费接口 |
| 已有 Vue 组件 | 在已有 Vue 组件内部优化；主 Vite 构建保留 |
| 已有 React 页面 | `web_src/js/react/`，入口 `app.tsx`，独立 `vite.react.config.ts`；不把模板页改挂 React 壳 |
| 配色与代码高亮 | `web_src/css/themes/theme-gitsss-*.css` 及基础主题；复用 CSS token，注释用中性灰 |
| UI 文案 | `options/locale/locale_zh-CN.json`、`locale_en-US.json`，保持稳定 key 与格式参数 |
| 嵌入资产 | `public/assets/` 与 `modules/*/bindata.*` 是生成物，修改源文件后按现有生成链生成 |
| 行为说明 | `docs/features/` Markdown、对应 `docs/site/` HTML；协作流程见 `docs/site/agents.html` |

已有 Go、TypeScript、CSS、Vue、React 和 pnpm/Vite 均保留。Skill 自带 Alpine、字体和
独立 HTML 对比壳仅用于离线设计探索，不引入产品。不要为示例创建第二个 package.json、
应用根目录或新的后端；不得把 prototype 当作已集成页面。

## 工作台规则

- PC 浏览器优先。内容区利用可用宽度，侧栏与主区职责清楚；避免重复顶部导航和无意义大空白。
- 一个区域一个主动作；低频项渐进展开，常用设置原地完成。保留当前值、返回位置和错误上下文。
- 沿用 GitSSS / GitHub 式工作台方向、品牌和图标语法；去掉偶然形成的旧布局，不做全站主题替换。
- 组织与个人显示稳定账号名、备注和类型；仓库名、分支、trackId、触发者等信息可以点击追踪。
- 日期按现有约定同时显示绝对时间、周几和相对时间；不要只剩“前天”。
- 信息密度靠对齐、分组和删冗余提升。工作台正文通常 13–14 px、控件 14 px 起，管理表单按可读性用 14–16 px；
  这些是起点，不能靠继续缩小字号解决溢出。
- 多行消息高度随内容增长；名称截断时保留可访问的完整内容入口。统计名称与占比各有稳定列。
- 明暗主题均使用语义 token；状态同时有文本/图标，键盘焦点可见。搜索、弹窗、菜单能用 Tab/方向键/Enter/Escape。
- 控件复用 Fomantic、原生 dialog 与现有日期/下拉组件。拖拽同时保留正常选择按钮。

## 性能与兼容

- 渲染保持服务端 HTML 优先，JavaScript 渐进增强；不要为一个装饰加载整套新运行时。
- 复用现有分页、请求取消与防抖；过滤本地已有数据时不新增请求。
- 事件委托或受控监听，离开/隐藏页面清理 timer、requestAnimationFrame、观察器与动画。
- 装饰动效不引入轮询；避免大面积 blur、全页 will-change、大量固定图层。代码/列表核心流程保持直接反馈。
- 内网优先使用已打包图标、字体和资产；不新增 CDN、遥测或外部实时依赖。
- 前端资源生成沿用两个 Vite 入口与既有 bindata 链。嵌入前运行 `node tools/frontend-build.mjs check`，
  不把新模板配旧 CSS 视为通过。
- 不改动 Git/SSH/LFS、账号密码与绑定、权限、数据库、存储布局和 API/表单契约。
  若任务确实要求这些变动，按根 AGENTS 的兼容性与验收流程单独处理。

## 检查深度

纯 Skill 安装与说明变更：检查入口 frontmatter、引用路径、上游来源、忽略边界和工具命令。
仅在用户请求验证实际工具时，对非敏感离线小样做截图/对比页冒烟；不登录生产或读取业务仓库。

实际 UI 任务：先 `task help`，再选范围对应的 lint、已有测试、真实 Edge 交互与截图。
pnpm/Make 前核对 package.json engines；无 task/Docker 时读当前 Makefile，使用明确对应的本机命令。
演练隔离 HOME/USERPROFILE 和 SSH_ROOT_PATH；构建、发布和部署遵循根 AGENTS 与已有授权，
不创建或运行 `.github/workflows/`。
