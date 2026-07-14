import { useEffect, useState } from "react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { MotionSurface } from "@/components/MotionSurface"
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
  DownloadIcon,
  NotificationIcon,
  OverviewIcon,
  RepositoryIcon,
  SearchIcon,
  SettingsIcon,
  StorageIcon,
  TakeoverIcon,
  WorkIcon,
} from "@/components/ui/icons"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { NotificationCenter } from "@/features/notifications/NotificationCenter"
import { RepositoryBrowser } from "@/features/repository-browser/RepositoryBrowser"
import { TakeoverCenter } from "@/features/takeover/TakeoverCenter"
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

function EventStreamTab({
  events,
  onClear,
}: {
  events: RecentEvent[]
  onClear: () => void
}) {
  return (
    <section className="mx-auto max-w-3xl pt-8">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h2 className="text-xl font-semibold tracking-[-0.035em]">
            近期事件
          </h2>
          <p className="mt-1 text-sm text-zinc-500">
            只保留可继续处理的短期预览，审计记录独立保存。
          </p>
        </div>
        <Button
          disabled={events.length === 0}
          onClick={onClear}
          variant="outline"
        >
          清除预览
        </Button>
      </div>
      <ol className="mt-7 divide-y divide-zinc-200 border-y border-zinc-200">
        {events.length > 0 ? (
          events.map((event) => (
            <li className="flex gap-4 py-5" key={event.id}>
              <EventGlyph kind={event.kind} />
              <time className="w-10 shrink-0 text-xs text-zinc-500">
                {event.time}
              </time>
              <div>
                <p className="text-sm font-medium text-zinc-800">
                  {event.action}
                </p>
                <p className="mt-1 text-xs text-zinc-500">
                  由 {event.actor} 触发
                </p>
              </div>
            </li>
          ))
        ) : (
          <li className="py-8 text-sm text-zinc-500">
            预览已清除。新的活动会在这里出现。
          </li>
        )}
      </ol>
    </section>
  )
}

function StorageAwarenessTab({ onReview }: { onReview: () => void }) {
  return (
    <section className="mx-auto max-w-3xl pt-8">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-xl font-semibold tracking-[-0.035em]">
            存储感知
          </h2>
          <p className="mt-1 text-sm text-zinc-500">
            所有容量均可追溯到仓库、LFS、制品或备份。
          </p>
        </div>
        <StorageIcon className="size-7 text-zinc-700" />
      </div>
      <div className="mt-8 border-y border-zinc-200 py-6">
        <div className="flex items-baseline justify-between">
          <span className="text-sm font-medium">可用容量</span>
          <span className="text-2xl font-semibold tracking-[-0.04em]">
            3.12 / 4 TB
          </span>
        </div>
        <div
          aria-label="存储已使用 78%"
          className="mt-4 h-2 overflow-hidden rounded-full bg-zinc-100"
        >
          <div className="h-full w-[78%] rounded-full bg-zinc-900" />
        </div>
        <p className="mt-3 text-sm leading-6 text-amber-700">
          按当前增长率，预计 18
          天后达到预警线。建议先清理已过期制品，而非删除仓库对象。
        </p>
      </div>
      <div className="divide-y divide-zinc-200">
        <div className="flex justify-between py-4 text-sm">
          <span>Git 对象与索引</span>
          <span className="font-medium">410 GB</span>
        </div>
        <div className="flex justify-between py-4 text-sm">
          <span>LFS 与游戏资产</span>
          <span className="font-medium">2.31 TB</span>
        </div>
        <div className="flex justify-between py-4 text-sm">
          <span>构建制品与备份</span>
          <span className="font-medium">400 GB</span>
        </div>
      </div>
      <Button className="mt-6" onClick={onReview} variant="outline">
        查看清理方案
      </Button>
    </section>
  )
}

function GitClientDownloadTab({
  platform,
  onPlatformChange,
  onMirrorOpen,
}: {
  platform: "windows" | "macos"
  onPlatformChange: (platform: "windows" | "macos") => void
  onMirrorOpen: () => void
}) {
  const isWindows = platform === "windows"
  return (
    <section className="mx-auto max-w-3xl pt-8">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-xl font-semibold tracking-[-0.035em]">
            Git 客户端下载
          </h2>
          <p className="mt-1 text-sm text-zinc-500">
            为美术、策划和制作人员准备的最短安装路径。
          </p>
        </div>
        <DownloadIcon className="size-7 text-zinc-700" />
      </div>
      <div className="mt-7 flex gap-2">
        <Button
          onClick={() => onPlatformChange("windows")}
          variant={isWindows ? "default" : "outline"}
        >
          Windows
        </Button>
        <Button
          onClick={() => onPlatformChange("macos")}
          variant={isWindows ? "outline" : "default"}
        >
          macOS
        </Button>
      </div>
      <div className="mt-6 border-y border-zinc-200 py-6">
        <p className="text-sm font-medium">
          {isWindows
            ? "Windows 11 / 10（x64、ARM64）"
            : "macOS（Apple Silicon、Intel）"}
        </p>
        <p className="mt-2 max-w-xl text-sm leading-6 text-zinc-600">
          下载并安装后，gitsss 会用向导连接你的团队、检查 SSH 或
          Token，并用文件树而不是命令行开始第一个仓库。
        </p>
        <div className="mt-5 flex flex-wrap gap-3">
          <a
            className="inline-flex h-9 items-center rounded-lg bg-zinc-950 px-4 text-sm font-medium text-white"
            href={
              isWindows
                ? "https://git-scm.com/download/win"
                : "https://git-scm.com/download/mac"
            }
            rel="noreferrer"
            target="_blank"
          >
            官方下载安装
          </a>
          <Button onClick={onMirrorOpen} variant="outline">
            查看可用镜像
          </Button>
        </div>
      </div>
      <p className="text-xs leading-5 text-zinc-500">
        镜像地址、版本与校验和由服务器端受签名目录提供；离线内网环境由管理员预置软件包源。
      </p>
    </section>
  )
}

export function App() {
  const [area, setArea] = useState<
    "workspace" | "repository" | "notifications" | "takeover"
  >("workspace")
  const [commandOpen, setCommandOpen] = useState(false)
  const [notice, setNotice] = useState("今天有 3 项需要你处理")
  const [eventPreview, setEventPreview] = useState(() =>
    RecentEventsPreview.from(6, initialEvents)
  )
  const [clientPlatform, setClientPlatform] = useState<"windows" | "macos">(
    "windows"
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

  const openWorkspace = (sectionID?: string) => {
    setArea("workspace")
    if (sectionID) {
      window.requestAnimationFrame(() =>
        document
          .getElementById(sectionID)
          ?.scrollIntoView({ behavior: "smooth", block: "start" })
      )
    }
  }

  return (
    <div className="min-h-svh bg-white text-zinc-950 selection:bg-blue-100">
      <div className="grid min-h-svh lg:grid-cols-[13.5rem_minmax(0,1fr)]">
        <aside className="hidden border-r border-zinc-200/80 px-4 py-6 lg:flex lg:flex-col">
          <a
            className="flex items-center gap-2 px-2 text-[17px] font-semibold tracking-[-0.03em]"
            href="#overview"
            onClick={() => openWorkspace()}
          >
            <BrandMark className="size-6" />
            gitsss
          </a>
          <nav aria-label="主导航" className="mt-12 flex flex-col gap-1">
            <button
              className={`flex h-11 items-center gap-3 rounded-xl px-3 text-left text-sm transition-colors ${area === "workspace" ? "bg-blue-50 font-medium text-blue-700" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950"}`}
              onClick={() => openWorkspace()}
              type="button"
            >
              <OverviewIcon className="size-5" />
              概览
            </button>
            <button
              className="flex h-11 items-center gap-3 rounded-xl px-3 text-sm text-zinc-600 transition-colors hover:bg-zinc-100 hover:text-zinc-950"
              onClick={() => openWorkspace("next")}
              type="button"
            >
              <WorkIcon className="size-5" />
              工作
            </button>
            <button
              className={`flex h-11 items-center gap-3 rounded-xl px-3 text-left text-sm transition-colors ${area === "repository" ? "bg-blue-50 font-medium text-blue-700" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950"}`}
              onClick={() => setArea("repository")}
              type="button"
            >
              <RepositoryIcon className="size-5" />
              仓库
            </button>
            <button
              className={`flex h-11 items-center gap-3 rounded-xl px-3 text-left text-sm transition-colors ${area === "notifications" ? "bg-blue-50 font-medium text-blue-700" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950"}`}
              onClick={() => setArea("notifications")}
              type="button"
            >
              <NotificationIcon className="size-5" />
              通知
            </button>
            <button
              className={`flex h-11 items-center gap-3 rounded-xl px-3 text-left text-sm transition-colors ${area === "takeover" ? "bg-blue-50 font-medium text-blue-700" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950"}`}
              onClick={() => setArea("takeover")}
              type="button"
            >
              <TakeoverIcon className="size-5" />
              接管
            </button>
            <button
              className="flex h-11 items-center gap-3 rounded-xl px-3 text-sm text-zinc-600 transition-colors hover:bg-zinc-100 hover:text-zinc-950"
              onClick={() => setNotice("设置中心将在授权工作流完成后开放")}
              type="button"
            >
              <SettingsIcon className="size-5" />
              设置
            </button>
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
                {area === "workspace"
                  ? "今天"
                  : area === "repository"
                    ? "仓库"
                    : area === "notifications"
                      ? "通知"
                      : "接管"}
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
                aria-label="打开命令面板"
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

          <nav
            aria-label="移动端导航"
            className="grid grid-cols-4 border-b border-zinc-200 lg:hidden"
          >
            <button
              className={`flex h-11 items-center justify-center gap-2 text-sm ${area === "workspace" ? "border-b-2 border-zinc-950 font-medium text-zinc-950" : "text-zinc-500"}`}
              onClick={() => openWorkspace()}
              type="button"
            >
              <OverviewIcon className="size-4" />
              概览
            </button>
            <button
              className={`flex h-11 items-center justify-center gap-2 text-sm ${area === "repository" ? "border-b-2 border-zinc-950 font-medium text-zinc-950" : "text-zinc-500"}`}
              onClick={() => setArea("repository")}
              type="button"
            >
              <RepositoryIcon className="size-4" />
              仓库
            </button>
            <button
              className={`flex h-11 items-center justify-center gap-2 text-sm ${area === "notifications" ? "border-b-2 border-zinc-950 font-medium text-zinc-950" : "text-zinc-500"}`}
              onClick={() => setArea("notifications")}
              type="button"
            >
              <NotificationIcon className="size-4" />
              通知
            </button>
            <button
              className={`flex h-11 items-center justify-center gap-1.5 text-sm ${area === "takeover" ? "border-b-2 border-zinc-950 font-medium text-zinc-950" : "text-zinc-500"}`}
              onClick={() => setArea("takeover")}
              type="button"
            >
              <TakeoverIcon className="size-4" />
              接管
            </button>
          </nav>

          <p aria-live="polite" className="sr-only">
            {notice}
          </p>

          {area === "repository" ? (
            <MotionSurface key="repository">
              <RepositoryBrowser />
            </MotionSurface>
          ) : area === "notifications" ? (
            <MotionSurface key="notifications">
              <NotificationCenter />
            </MotionSurface>
          ) : area === "takeover" ? (
            <MotionSurface key="takeover">
              <TakeoverCenter />
            </MotionSurface>
          ) : (
            <Tabs className="pt-6" defaultValue="dashboard">
              <TabsList className="max-w-full overflow-x-auto" variant="line">
                <TabsTrigger value="dashboard">数据大盘</TabsTrigger>
                <TabsTrigger value="events">近期事件流</TabsTrigger>
                <TabsTrigger value="storage">存储感知</TabsTrigger>
                <TabsTrigger value="download">Git 客户端下载</TabsTrigger>
              </TabsList>
              <TabsContent value="dashboard">
                <MotionSurface className="pt-8">
                  <section
                    aria-label="团队概览"
                    className="grid grid-cols-3 divide-x divide-zinc-200"
                  >
                    <div className="pr-4">
                      <p className="text-xs font-medium text-zinc-500">
                        待处理
                      </p>
                      <p className="mt-1 text-3xl font-medium tracking-[-0.05em]">
                        12
                      </p>
                    </div>
                    <div className="px-4">
                      <p className="text-xs font-medium text-zinc-500">
                        构建失败
                      </p>
                      <p className="mt-1 text-3xl font-medium tracking-[-0.05em]">
                        3
                      </p>
                    </div>
                    <div className="pl-4">
                      <p className="text-xs font-medium text-zinc-500">
                        待发布
                      </p>
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
                              <th className="py-2 text-right font-medium">
                                更新
                              </th>
                            </tr>
                          </thead>
                          <tbody>
                            {repositories.map((repository) => (
                              <tr
                                className="border-b border-zinc-100 last:border-0"
                                key={repository.name}
                              >
                                <td className="py-3 font-medium text-zinc-800">
                                  <button
                                    className="hover:text-blue-700"
                                    onClick={() => setArea("repository")}
                                    type="button"
                                  >
                                    {repository.name}
                                  </button>
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
                </MotionSurface>
              </TabsContent>
              <TabsContent value="events">
                <MotionSurface>
                  <EventStreamTab
                    events={eventPreview.items()}
                    onClear={clearPreview}
                  />
                </MotionSurface>
              </TabsContent>
              <TabsContent value="storage">
                <MotionSurface>
                  <StorageAwarenessTab
                    onReview={() => setNotice("已打开存储清理预览")}
                  />
                </MotionSurface>
              </TabsContent>
              <TabsContent value="download">
                <MotionSurface>
                  <GitClientDownloadTab
                    onPlatformChange={setClientPlatform}
                    onMirrorOpen={() => setNotice("已打开受签名镜像目录")}
                    platform={clientPlatform}
                  />
                </MotionSurface>
              </TabsContent>
            </Tabs>
          )}
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
