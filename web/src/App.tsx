import { useEffect, useState } from "react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
} from "@/components/ui/command"
import {
  AlertIcon,
  ArrowUpIcon,
  BrandMark,
  ClockIcon,
  OverviewIcon,
  RepositoryIcon,
  SearchIcon,
  SettingsIcon,
  WorkIcon,
} from "@/components/ui/icons"
import { Separator } from "@/components/ui/separator"
import { RecentEventsPreview, type RecentEvent } from "@/lib/recent-events"

const initialEvents: RecentEvent[] = [
  {
    id: "merge",
    action: "feature/export 合并到 main",
    actor: "张晨",
    time: "10:24",
    kind: "change",
  },
  {
    id: "build",
    action: "构建 #1942 失败（支付服务）",
    actor: "CI",
    time: "10:18",
    kind: "build",
  },
  {
    id: "review",
    action: "新建 PR #482：修复导出报表错误",
    actor: "李明",
    time: "09:52",
    kind: "review",
  },
  {
    id: "release",
    action: "部署到 staging 环境成功",
    actor: "CD",
    time: "09:15",
    kind: "release",
  },
  {
    id: "member",
    action: "新成员加入：赵磊",
    actor: "系统",
    time: "08:47",
    kind: "member",
  },
]

const nextSteps = [
  {
    id: "review",
    initials: "明",
    action: "审查合并 PR #482：修复用户导出报表错误",
    time: "15 分钟前",
    label: "查看",
    priority: true,
  },
  {
    id: "build",
    initials: "晨",
    action: "修复支付服务构建失败",
    time: "1 小时前",
    label: "处理",
    priority: false,
  },
  {
    id: "release",
    initials: "薇",
    action: "发布 v2.4.1 到生产环境",
    time: "3 小时前",
    label: "发布",
    priority: false,
  },
]

const repositories = [
  {
    name: "payment-service",
    branch: "main",
    status: "失败",
    detail: "修复订单金额计算错误",
    owner: "张晨",
    updated: "10 分钟前",
    tone: "text-red-600",
  },
  {
    name: "user-service",
    branch: "main",
    status: "成功",
    detail: "优化用户缓存策略",
    owner: "李明",
    updated: "28 分钟前",
    tone: "text-emerald-600",
  },
  {
    name: "frontend-app",
    branch: "main",
    status: "运行中",
    detail: "feat: 新增数据看板组件",
    owner: "王芳",
    updated: "5 分钟前",
    tone: "text-blue-600",
  },
  {
    name: "admin-service",
    branch: "develop",
    status: "成功",
    detail: "修复权限校验问题",
    owner: "张晨",
    updated: "1 小时前",
    tone: "text-emerald-600",
  },
]

const commands = [
  { label: "创建仓库", shortcut: "R", message: "已打开创建仓库向导" },
  { label: "开始一次备份", shortcut: "B", message: "已准备备份范围预览" },
  { label: "连接内网 Agent", shortcut: "A", message: "已打开 Agent 连接向导" },
  { label: "查看失败构建", shortcut: "F", message: "已筛选 3 个失败构建" },
]

function EventGlyph({ kind }: Pick<RecentEvent, "kind">) {
  const className = "size-4 shrink-0 text-zinc-700"
  if (kind === "build") return <AlertIcon className={className} />
  if (kind === "release") return <ArrowUpIcon className={className} />
  if (kind === "review") return <WorkIcon className={className} />
  if (kind === "member") return <OverviewIcon className={className} />
  return <RepositoryIcon className={className} />
}

export function App() {
  const [commandOpen, setCommandOpen] = useState(false)
  const [notice, setNotice] = useState("今天有 3 项需要你处理")
  const [eventPreview, setEventPreview] = useState(() =>
    RecentEventsPreview.from(6, initialEvents)
  )

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault()
        setCommandOpen((open) => !open)
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [])

  const selectCommand = (message: string) => {
    setNotice(message)
    setCommandOpen(false)
  }

  const clearPreview = () => {
    setEventPreview((preview) => {
      const nextPreview = preview.clone()
      nextPreview.clear()
      return nextPreview
    })
    setNotice("近期事件预览已清除；审计记录不受影响")
  }

  return (
    <div className="min-h-svh bg-white text-zinc-950 selection:bg-blue-100">
      <div className="grid min-h-svh lg:grid-cols-[13.5rem_minmax(0,1fr)]">
        <aside className="hidden border-r border-zinc-200/80 px-4 py-6 lg:flex lg:flex-col">
          <a
            className="flex items-center gap-2 px-2 text-[17px] font-semibold tracking-[-0.03em]"
            href="#overview"
          >
            <BrandMark className="size-6" />
            gitsss
          </a>
          <nav aria-label="主导航" className="mt-12 flex flex-col gap-1">
            <a
              className="flex h-11 items-center gap-3 rounded-xl bg-blue-50 px-3 text-sm font-medium text-blue-700"
              href="#overview"
            >
              <OverviewIcon className="size-5" />
              概览
            </a>
            <a
              className="flex h-11 items-center gap-3 rounded-xl px-3 text-sm text-zinc-600 transition-colors hover:bg-zinc-100 hover:text-zinc-950"
              href="#next"
            >
              <WorkIcon className="size-5" />
              工作
            </a>
            <a
              className="flex h-11 items-center gap-3 rounded-xl px-3 text-sm text-zinc-600 transition-colors hover:bg-zinc-100 hover:text-zinc-950"
              href="#repositories"
            >
              <RepositoryIcon className="size-5" />
              仓库
            </a>
            <a
              className="flex h-11 items-center gap-3 rounded-xl px-3 text-sm text-zinc-600 transition-colors hover:bg-zinc-100 hover:text-zinc-950"
              href="#settings"
            >
              <SettingsIcon className="size-5" />
              设置
            </a>
          </nav>
          <p className="mt-auto px-2 text-xs leading-5 text-zinc-400">
            gitsss 预览版
            <br />
            键盘优先的团队工作台
          </p>
        </aside>

        <main className="min-w-0 px-5 py-5 sm:px-8 lg:px-10 lg:py-7 xl:px-14">
          <header className="flex items-center justify-between gap-4 border-b border-zinc-200/80 pb-5 sm:pb-6">
            <div>
              <div className="flex items-center gap-2 lg:hidden">
                <BrandMark className="size-5" />
                <span className="font-semibold tracking-[-0.03em]">gitsss</span>
              </div>
              <h1 className="mt-2 text-[28px] font-semibold tracking-[-0.045em] sm:text-[32px] lg:mt-0">
                今天
              </h1>
            </div>
            <div className="flex items-center gap-3">
              <button
                aria-label="打开命令面板"
                className="hidden w-[min(34vw,34rem)] items-center gap-3 rounded-xl border border-blue-500 bg-white px-4 py-3 text-left text-sm text-zinc-500 shadow-[0_0_0_3px_rgba(59,130,246,0.09)] transition-shadow hover:shadow-[0_0_0_4px_rgba(59,130,246,0.13)] sm:flex"
                onClick={() => setCommandOpen(true)}
                type="button"
              >
                <SearchIcon className="size-5 text-zinc-600" />
                <span className="flex-1">搜索或执行命令</span>
                <kbd className="font-sans text-xs text-zinc-500">⌘ K</kbd>
              </button>
              <Button
                className="sm:hidden"
                onClick={() => setCommandOpen(true)}
                size="icon"
                variant="outline"
              >
                <SearchIcon />
              </Button>
              <Avatar className="size-9 border border-zinc-200">
                <AvatarFallback className="bg-zinc-100 text-xs font-medium text-zinc-700">
                  林
                </AvatarFallback>
              </Avatar>
            </div>
          </header>

          <p aria-live="polite" className="sr-only">
            {notice}
          </p>

          <div className="grid gap-12 pt-8 xl:grid-cols-[minmax(0,1fr)_19.5rem] xl:gap-10">
            <div className="min-w-0">
              <section
                aria-label="团队概览"
                className="grid grid-cols-3 divide-x divide-zinc-200"
              >
                <div className="pr-4">
                  <p className="text-xs font-medium text-zinc-500">待处理</p>
                  <p className="mt-1 text-3xl font-medium tracking-[-0.05em]">
                    12
                  </p>
                </div>
                <div className="px-4">
                  <p className="text-xs font-medium text-zinc-500">构建失败</p>
                  <p className="mt-1 text-3xl font-medium tracking-[-0.05em]">
                    3
                  </p>
                </div>
                <div className="pl-4">
                  <p className="text-xs font-medium text-zinc-500">待发布</p>
                  <p className="mt-1 text-3xl font-medium tracking-[-0.05em]">
                    2
                  </p>
                </div>
              </section>

              <section className="mt-10" id="next">
                <h2 className="text-lg font-semibold tracking-[-0.03em]">
                  下一步
                </h2>
                <div className="mt-4 divide-y divide-zinc-200 border-y border-zinc-200">
                  {nextSteps.map((step) => (
                    <div
                      className="flex items-center gap-3 py-4 sm:gap-4"
                      key={step.id}
                    >
                      <span
                        aria-hidden="true"
                        className={
                          step.priority
                            ? "size-2 shrink-0 rounded-full bg-blue-600"
                            : "size-2 shrink-0"
                        }
                      />
                      <Avatar className="size-9 shrink-0">
                        <AvatarFallback className="bg-zinc-100 text-xs font-medium">
                          {step.initials}
                        </AvatarFallback>
                      </Avatar>
                      <p className="min-w-0 flex-1 text-sm leading-5 font-medium text-zinc-800">
                        {step.action}
                      </p>
                      <span className="hidden items-center gap-1 text-xs text-zinc-500 md:flex">
                        <ClockIcon className="size-4" />
                        {step.time}
                      </span>
                      <Button
                        className="rounded-lg px-4"
                        onClick={() => setNotice(`已选择：${step.action}`)}
                        size="sm"
                      >
                        {step.label}
                      </Button>
                    </div>
                  ))}
                </div>
              </section>

              <section
                className="mt-10 grid gap-10 lg:grid-cols-[minmax(0,.78fr)_minmax(0,1.22fr)]"
                id="repositories"
              >
                <div>
                  <h2 className="text-lg font-semibold tracking-[-0.03em]">
                    流水线健康度
                  </h2>
                  <div className="mt-5 flex flex-col gap-5">
                    {[
                      {
                        name: "支付服务",
                        states: ["done", "done", "done", "failed", "idle"],
                      },
                      {
                        name: "用户服务",
                        states: ["done", "done", "done", "done", "idle"],
                      },
                      {
                        name: "前端应用",
                        states: ["done", "done", "done", "running", "idle"],
                      },
                    ].map((pipeline) => (
                      <div
                        className="grid grid-cols-[5.5rem_minmax(0,1fr)] items-center gap-3"
                        key={pipeline.name}
                      >
                        <span className="text-sm text-zinc-700">
                          {pipeline.name}
                        </span>
                        <div
                          aria-label={`${pipeline.name} 流水线状态`}
                          className="flex items-center gap-2"
                        >
                          {pipeline.states.map((state, index) => (
                            <span
                              className={`size-3 rounded-full border ${state === "done" ? "border-emerald-600 bg-emerald-500" : state === "failed" ? "border-red-600 bg-red-500" : state === "running" ? "border-blue-600 bg-white ring-2 ring-blue-100" : "border-zinc-300 bg-white"}`}
                              key={`${pipeline.name}-${index}`}
                            />
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div>
                  <h2 className="text-lg font-semibold tracking-[-0.03em]">
                    仓库
                  </h2>
                  <div className="mt-3 overflow-x-auto">
                    <table className="w-full min-w-[31rem] border-collapse text-left text-xs">
                      <thead className="border-b border-zinc-200 text-zinc-500">
                        <tr>
                          <th className="py-2 font-medium">仓库</th>
                          <th className="py-2 font-medium">分支</th>
                          <th className="py-2 font-medium">状态</th>
                          <th className="py-2 font-medium">最近提交</th>
                          <th className="py-2 text-right font-medium">更新</th>
                        </tr>
                      </thead>
                      <tbody>
                        {repositories.map((repository) => (
                          <tr
                            className="border-b border-zinc-100 last:border-0"
                            key={repository.name}
                          >
                            <td className="py-3 font-medium text-zinc-800">
                              {repository.name}
                            </td>
                            <td className="py-3 text-zinc-600">
                              {repository.branch}
                            </td>
                            <td
                              className={`py-3 font-medium ${repository.tone}`}
                            >
                              {repository.status}
                            </td>
                            <td className="py-3">
                              <span className="block text-zinc-700">
                                {repository.detail}
                              </span>
                              <span className="text-zinc-400">
                                {repository.owner}
                              </span>
                            </td>
                            <td className="py-3 text-right text-zinc-500">
                              {repository.updated}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </section>
            </div>

            <aside className="border-t border-zinc-200 pt-8 xl:border-t-0 xl:border-l xl:pt-0 xl:pl-8">
              <div className="flex items-baseline justify-between">
                <h2 className="text-lg font-semibold tracking-[-0.03em]">
                  近期事件
                </h2>
                <span className="text-xs text-zinc-400">LRU 预览</span>
              </div>
              {eventPreview.items().length > 0 ? (
                <ol className="mt-5 flex flex-col gap-5">
                  {eventPreview.items().map((event) => (
                    <li className="flex gap-3" key={event.id}>
                      <EventGlyph kind={event.kind} />
                      <time className="w-9 shrink-0 text-xs text-zinc-500">
                        {event.time}
                      </time>
                      <div className="min-w-0">
                        <p className="text-sm leading-5 text-zinc-800">
                          {event.action}
                        </p>
                        <p className="mt-0.5 text-xs text-zinc-500">
                          由 {event.actor} 触发
                        </p>
                      </div>
                    </li>
                  ))}
                </ol>
              ) : (
                <p className="mt-5 text-sm leading-6 text-zinc-500">
                  预览已清除。新的活动会在这里出现。
                </p>
              )}
              <Separator className="mt-8" />
              <Button
                className="mt-4 -ml-2 text-zinc-700"
                disabled={eventPreview.items().length === 0}
                onClick={clearPreview}
                variant="ghost"
              >
                清除预览
              </Button>
            </aside>
          </div>
        </main>
      </div>

      <CommandDialog
        description="搜索仓库、工作项或直接执行安全操作。"
        onOpenChange={setCommandOpen}
        open={commandOpen}
        title="gitsss 命令面板"
      >
        <Command>
          <CommandInput placeholder="搜索或执行命令" />
          <CommandList>
            <CommandEmpty>没有找到匹配的命令。</CommandEmpty>
            <CommandGroup heading="快速操作">
              {commands.map((command) => (
                <CommandItem
                  key={command.label}
                  onSelect={() => selectCommand(command.message)}
                >
                  {command.label}
                  <CommandShortcut>⌘ {command.shortcut}</CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </CommandDialog>
    </div>
  )
}

export default App
