import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { MotionSurface } from "@/components/MotionSurface"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
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
import { Input } from "@/components/ui/input"
import {
  formatBytes,
  loadOperationalData,
  relativeTime,
  type OperationalData,
  type PipelineRun,
  type StorageSnapshot,
} from "@/features/dashboard/dashboard-data"
import { AccessCenter } from "@/features/access/AccessCenter"
import { NotificationCenter } from "@/features/notifications/NotificationCenter"
import { PipelineCenter } from "@/features/pipelines/PipelineCenter"
import { RepositoryBrowser } from "@/features/repository-browser/RepositoryBrowser"
import { TakeoverCenter } from "@/features/takeover/TakeoverCenter"
import { apiDelete } from "@/lib/api"
import type { RecentEvent } from "@/lib/recent-events"
import {
  readSessionToken,
  writeSelectedRepository,
  writeSessionToken,
} from "@/lib/session"

type WorkspaceTab = "dashboard" | "events" | "storage" | "download"
type Area =
  | "workspace"
  | "pipelines"
  | "repository"
  | "notifications"
  | "takeover"
  | "access"

const pipelineStatus = {
  failed: { label: "失败", tone: "text-red-600" },
  queued: { label: "等待", tone: "text-amber-700" },
  running: { label: "运行中", tone: "text-blue-600" },
  succeeded: { label: "成功", tone: "text-emerald-600" },
} satisfies Record<PipelineRun["status"], { label: string; tone: string }>

function recentEvents(data?: OperationalData): RecentEvent[] {
  return (data?.dashboard?.events ?? []).map((event) => ({
    id: event.id,
    action: event.action,
    actor: event.actor,
    time: event.at.includes("T") ? relativeTime(event.at) : event.at,
    kind:
      event.kind === "build" ||
      event.kind === "release" ||
      event.kind === "review" ||
      event.kind === "member"
        ? event.kind
        : "change",
  }))
}

function latestRunByRepository(runs: PipelineRun[] = []) {
  const latest = new Map<string, PipelineRun>()
  for (const run of runs) {
    if (!latest.has(run.repository_id)) latest.set(run.repository_id, run)
  }
  return latest
}

function pipelineGroups(runs: PipelineRun[] = []) {
  const groups = new Map<string, PipelineRun[]>()
  for (const run of runs) {
    const group = groups.get(run.repository_id) ?? []
    if (group.length < 5) group.push(run)
    groups.set(run.repository_id, group)
  }
  return [...groups.entries()].slice(0, 3)
}

const commands = [
  { id: "pipelines", label: "开始本地 CI", shortcut: "P" },
  { id: "repositories", label: "打开仓库", shortcut: "R" },
  { id: "storage", label: "检查存储容量", shortcut: "S" },
  { id: "takeover", label: "继续接管流程", shortcut: "A" },
  { id: "failed", label: "查看失败流水线", shortcut: "F" },
  { id: "connect", label: "连接服务器", shortcut: "," },
  { id: "access", label: "管理团队授权", shortcut: "U" },
]

const areaTitles: Record<Area, string> = {
  access: "授权",
  notifications: "通知",
  pipelines: "工作",
  repository: "仓库",
  takeover: "接管",
  workspace: "今天",
}

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

const storageLabels: Record<string, string> = {
  artifacts: "构建制品",
  attachments: "附件",
  backups: "备份",
  database: "数据库",
  git_objects: "Git 对象与索引",
  lfs_assets: "LFS 与游戏资产",
  other_volume_usage: "卷内其他数据",
}

function StorageAwarenessTab({
  loading,
  onReview,
  storage,
}: {
  loading: boolean
  onReview: () => void
  storage?: StorageSnapshot
}) {
  const usagePercent = storage
    ? Math.min(100, Math.max(0, storage.usage_ratio * 100))
    : 0
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
            {storage
              ? `${formatBytes(storage.available_bytes)} / ${formatBytes(storage.capacity_bytes)}`
              : loading
                ? "读取中"
                : "无权读取"}
          </span>
        </div>
        <div
          aria-label={`存储已使用 ${usagePercent.toFixed(0)}%`}
          className="mt-4 h-2 overflow-hidden rounded-full bg-zinc-100"
        >
          <div
            className={`h-full rounded-full ${storage?.write_protected ? "bg-red-600" : storage?.warning ? "bg-amber-600" : "bg-zinc-900"}`}
            style={{ width: `${usagePercent}%` }}
          />
        </div>
        {storage ? (
          <p
            className={`mt-3 text-sm leading-6 ${storage.warning ? "text-amber-700" : "text-zinc-500"}`}
          >
            {storage.measurement_error
              ? `容量测量失败：${storage.measurement_error}`
              : storage.write_protected
                ? "可用空间已触及保护线，新的大文件写入会被拒绝。"
                : storage.warning
                  ? `可用空间接近预警线；已预留 ${formatBytes(storage.reserved_bytes)}，建议先清理过期制品。`
                  : `容量正常，已为服务运行预留 ${formatBytes(storage.reserved_bytes)}。`}
          </p>
        ) : null}
      </div>
      <div className="divide-y divide-zinc-200">
        {(storage?.usage ?? []).map((item) => (
          <div
            className="flex justify-between py-4 text-sm"
            key={item.category}
          >
            <span>{storageLabels[item.category] ?? item.category}</span>
            <span className="font-medium">{formatBytes(item.bytes)}</span>
          </div>
        ))}
        {storage && storage.usage.length === 0 ? (
          <p className="py-6 text-sm text-zinc-500">尚未发现已归类的数据。</p>
        ) : null}
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
  const [area, setArea] = useState<Area>("workspace")
  const [workspaceTab, setWorkspaceTab] = useState<WorkspaceTab>("dashboard")
  const [commandOpen, setCommandOpen] = useState(false)
  const [connectionOpen, setConnectionOpen] = useState(false)
  const [token, setToken] = useState(readSessionToken)
  const [tokenDraft, setTokenDraft] = useState(token)
  const [notice, setNotice] = useState("工作台已准备就绪")
  const [operational, setOperational] = useState<OperationalData>()
  const [dashboardLoading, setDashboardLoading] = useState(false)
  const [clientPlatform, setClientPlatform] = useState<"windows" | "macos">(
    "windows"
  )

  const refreshDashboard = useCallback(
    async (signal?: AbortSignal) => {
      if (!token) {
        setOperational(undefined)
        return
      }
      setDashboardLoading(true)
      try {
        const data = await loadOperationalData(token, signal)
        if (!signal?.aborted) setOperational(data)
      } finally {
        if (!signal?.aborted) setDashboardLoading(false)
      }
    },
    [token]
  )

  useEffect(() => {
    if (area !== "workspace") return
    const controller = new AbortController()
    void refreshDashboard(controller.signal)
    const interval = window.setInterval(
      () => void refreshDashboard(controller.signal),
      30_000
    )
    return () => {
      controller.abort()
      window.clearInterval(interval)
    }
  }, [area, refreshDashboard])

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

  const events = useMemo(() => recentEvents(operational), [operational])
  const latestRuns = useMemo(
    () => latestRunByRepository(operational?.runs),
    [operational?.runs]
  )
  const actionableRuns = useMemo(
    () =>
      (operational?.runs ?? [])
        .filter((run) => run.status !== "succeeded")
        .slice(0, 3),
    [operational?.runs]
  )
  const headlineMetrics = useMemo(() => {
    if (operational?.repositories && operational.runs) {
      return [
        { label: "仓库", value: operational.repositories.length },
        {
          label: "运行中",
          value: operational.runs.filter(
            (run) => run.status === "queued" || run.status === "running"
          ).length,
        },
        {
          label: "构建失败",
          value: operational.runs.filter((run) => run.status === "failed")
            .length,
        },
      ]
    }
    return operational?.dashboard?.metrics.slice(0, 3) ?? []
  }, [operational])

  const selectCommand = (command: string) => {
    if (command === "connect") setConnectionOpen(true)
    if (command === "pipelines") setArea("pipelines")
    if (command === "repositories") setArea("repository")
    if (command === "takeover") setArea("takeover")
    if (command === "access") setArea("access")
    if (command === "storage") {
      setArea("workspace")
      setWorkspaceTab("storage")
    }
    if (command === "failed") {
      setArea("workspace")
      setWorkspaceTab("dashboard")
      setNotice("已定位最近失败的流水线")
    }
    setCommandOpen(false)
  }

  const clearPreview = async () => {
    if (!token) {
      setConnectionOpen(true)
      return
    }
    try {
      await apiDelete("/api/v1/events/preview", token)
      setOperational((current) =>
        current?.dashboard
          ? {
              ...current,
              dashboard: { ...current.dashboard, events: [] },
            }
          : current
      )
      setNotice("近期事件预览已清除；审计记录不受影响")
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "事件预览清理失败")
    }
  }

  const saveConnection = (event: FormEvent) => {
    event.preventDefault()
    const nextToken = tokenDraft.trim()
    writeSessionToken(nextToken)
    setToken(nextToken)
    setConnectionOpen(false)
    setNotice(nextToken ? "服务器连接已更新" : "服务器连接已断开")
  }

  const openWorkspace = (sectionID?: string) => {
    setArea("workspace")
    setWorkspaceTab("dashboard")
    if (sectionID) {
      window.requestAnimationFrame(() =>
        document
          .getElementById(sectionID)
          ?.scrollIntoView({ behavior: "smooth", block: "start" })
      )
    }
  }

  const openConnection = () => {
    setTokenDraft(token)
    setConnectionOpen(true)
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
              className={`flex h-11 items-center gap-3 rounded-xl px-3 text-sm transition-colors ${area === "pipelines" ? "bg-blue-50 font-medium text-blue-700" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950"}`}
              onClick={() => setArea("pipelines")}
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
              className={`flex h-11 items-center gap-3 rounded-xl px-3 text-sm transition-colors ${area === "access" ? "bg-blue-50 font-medium text-blue-700" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950"}`}
              onClick={() => setArea("access")}
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
                {areaTitles[area]}
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
              <button
                aria-label={token ? "管理服务器连接" : "连接服务器"}
                onClick={openConnection}
                type="button"
              >
                <Avatar className="size-9 border border-zinc-200">
                  <AvatarFallback
                    className={
                      token
                        ? "bg-zinc-100 text-xs font-medium text-zinc-700"
                        : "bg-amber-50 text-xs font-medium text-amber-700"
                    }
                  >
                    {token ? "连" : "未"}
                  </AvatarFallback>
                </Avatar>
              </button>
            </div>
          </header>

          <nav
            aria-label="移动端导航"
            className="grid grid-cols-5 border-b border-zinc-200 lg:hidden"
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
              className={`flex h-11 items-center justify-center gap-1.5 text-sm ${area === "pipelines" ? "border-b-2 border-zinc-950 font-medium text-zinc-950" : "text-zinc-500"}`}
              onClick={() => setArea("pipelines")}
              type="button"
            >
              <WorkIcon className="size-4" />
              工作
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

          {operational?.authenticationFailed ? (
            <Alert className="mt-6" variant="destructive">
              <AlertIcon />
              <AlertTitle>服务器连接已失效</AlertTitle>
              <AlertDescription>
                Token 已过期或被撤销。重新连接后，大盘会自动刷新。
              </AlertDescription>
              <AlertAction>
                <Button onClick={openConnection} size="sm" variant="outline">
                  重新连接
                </Button>
              </AlertAction>
            </Alert>
          ) : operational && operational.unavailable.length > 0 ? (
            <Alert className="mt-6">
              <AlertIcon />
              <AlertTitle>部分数据已按角色权限隐藏</AlertTitle>
              <AlertDescription>
                当前角色可继续使用已显示的功能；需要更多权限时请让管理员调整角色。
              </AlertDescription>
            </Alert>
          ) : null}

          {area === "pipelines" ? (
            <MotionSurface key="pipelines">
              <PipelineCenter onConnect={openConnection} token={token} />
            </MotionSurface>
          ) : area === "repository" ? (
            <MotionSurface key="repository">
              <RepositoryBrowser onConnect={openConnection} token={token} />
            </MotionSurface>
          ) : area === "notifications" ? (
            <MotionSurface key="notifications">
              <NotificationCenter onConnect={openConnection} token={token} />
            </MotionSurface>
          ) : area === "takeover" ? (
            <MotionSurface key="takeover">
              <TakeoverCenter onConnect={openConnection} token={token} />
            </MotionSurface>
          ) : area === "access" ? (
            <MotionSurface key="access">
              <AccessCenter onConnect={openConnection} token={token} />
            </MotionSurface>
          ) : (
            <Tabs
              className="pt-6"
              onValueChange={(value) => setWorkspaceTab(value as WorkspaceTab)}
              value={workspaceTab}
            >
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
                    {[0, 1, 2].map((index) => {
                      const metric = headlineMetrics[index]
                      return (
                        <div
                          className={
                            index === 0 ? "pr-4" : index === 2 ? "pl-4" : "px-4"
                          }
                          key={metric?.label ?? index}
                        >
                          <p className="text-xs font-medium text-zinc-500">
                            {metric?.label ?? (token ? "读取中" : "未连接")}
                          </p>
                          <p className="mt-1 text-3xl font-medium tracking-[-0.05em]">
                            {metric?.value ?? "—"}
                          </p>
                        </div>
                      )
                    })}
                  </section>

                  <section className="mt-10" id="next">
                    <h2 className="text-lg font-semibold tracking-[-0.03em]">
                      下一步
                    </h2>
                    <div className="mt-4 divide-y divide-zinc-200 border-y border-zinc-200">
                      {actionableRuns.map((run) => (
                        <div
                          className="flex items-center gap-3 py-4 sm:gap-4"
                          key={run.id}
                        >
                          <span
                            aria-hidden="true"
                            className={
                              run.status === "failed"
                                ? "size-2 shrink-0 rounded-full bg-blue-600"
                                : "size-2 shrink-0"
                            }
                          />
                          <Avatar className="size-9 shrink-0">
                            <AvatarFallback className="bg-zinc-100 text-xs font-medium">
                              {run.repository_id.slice(0, 1).toUpperCase()}
                            </AvatarFallback>
                          </Avatar>
                          <p className="min-w-0 flex-1 text-sm leading-5 font-medium text-zinc-800">
                            {run.status === "failed" ? "处理" : "跟进"}{" "}
                            {run.repository_id}的 {run.template_id}
                          </p>
                          <span className="hidden items-center gap-1 text-xs text-zinc-500 md:flex">
                            <ClockIcon className="size-4" />
                            {relativeTime(run.completed_at ?? run.created_at)}
                          </span>
                          <Button
                            className="rounded-lg px-4"
                            onClick={() =>
                              setNotice(
                                `${run.repository_id}：${pipelineStatus[run.status].label}`
                              )
                            }
                            size="sm"
                          >
                            查看
                          </Button>
                        </div>
                      ))}
                      {actionableRuns.length === 0 ? (
                        <p className="py-6 text-sm text-zinc-500">
                          {dashboardLoading
                            ? "正在检查工作队列…"
                            : token
                              ? "当前没有需要人工处理的流水线。"
                              : "连接服务器后显示需要处理的工作。"}
                        </p>
                      ) : null}
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
                        {pipelineGroups(operational?.runs).map(
                          ([repositoryID, runs]) => (
                            <div
                              className="grid grid-cols-[5.5rem_minmax(0,1fr)] items-center gap-3"
                              key={repositoryID}
                            >
                              <span className="truncate text-sm text-zinc-700">
                                {repositoryID}
                              </span>
                              <div
                                aria-label={`${repositoryID} 流水线状态`}
                                className="flex items-center gap-2"
                              >
                                {runs.toReversed().map((run) => (
                                  <span
                                    className={`size-3 rounded-full border ${run.status === "succeeded" ? "border-emerald-600 bg-emerald-500" : run.status === "failed" ? "border-red-600 bg-red-500" : run.status === "running" ? "border-blue-600 bg-white ring-2 ring-blue-100" : "border-amber-600 bg-amber-50"}`}
                                    key={run.id}
                                    title={pipelineStatus[run.status].label}
                                  />
                                ))}
                              </div>
                            </div>
                          )
                        )}
                        {pipelineGroups(operational?.runs).length === 0 ? (
                          <p className="text-sm text-zinc-500">
                            尚无流水线执行记录。
                          </p>
                        ) : null}
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
                              <th className="py-2 font-medium">托管</th>
                              <th className="py-2 font-medium">最近流水线</th>
                              <th className="py-2 text-right font-medium">
                                创建
                              </th>
                            </tr>
                          </thead>
                          <tbody>
                            {(operational?.repositories ?? []).map(
                              (repository) => {
                                const run = latestRuns.get(repository.id)
                                return (
                                  <tr
                                    className="border-b border-zinc-100 last:border-0"
                                    key={repository.id}
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
                                      {repository.default_branch}
                                    </td>
                                    <td className="py-3 font-medium text-zinc-600">
                                      {repository.hosted ? "本机" : "接管"}
                                    </td>
                                    <td className="py-3">
                                      {run ? (
                                        <>
                                          <span
                                            className={`block font-medium ${pipelineStatus[run.status].tone}`}
                                          >
                                            {pipelineStatus[run.status].label}
                                          </span>
                                          <span className="text-zinc-400">
                                            {run.template_id} ·{" "}
                                            {run.commit.slice(0, 8)}
                                          </span>
                                        </>
                                      ) : (
                                        <span className="text-zinc-400">
                                          尚未运行
                                        </span>
                                      )}
                                    </td>
                                    <td className="py-3 text-right text-zinc-500">
                                      {repository.created_at
                                        ? relativeTime(repository.created_at)
                                        : "—"}
                                    </td>
                                  </tr>
                                )
                              }
                            )}
                            {operational?.repositories?.length === 0 ? (
                              <tr>
                                <td className="py-6 text-zinc-500" colSpan={5}>
                                  还没有仓库。通过 Ctrl+K 开始创建或接管。
                                </td>
                              </tr>
                            ) : null}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  </section>
                </MotionSurface>
              </TabsContent>
              <TabsContent value="events">
                <MotionSurface>
                  <EventStreamTab events={events} onClear={clearPreview} />
                </MotionSurface>
              </TabsContent>
              <TabsContent value="storage">
                <MotionSurface>
                  <StorageAwarenessTab
                    loading={dashboardLoading}
                    onReview={() => setNotice("已打开存储清理预览")}
                    storage={operational?.storage}
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
                  onSelect={() => selectCommand(command.id)}
                >
                  {command.label}
                  <CommandShortcut>⌘ {command.shortcut}</CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
            {(operational?.repositories?.length ?? 0) > 0 ? (
              <CommandGroup heading="仓库">
                {operational!.repositories!.map((repository) => (
                  <CommandItem
                    key={repository.id}
                    onSelect={() => {
                      writeSelectedRepository(repository.id)
                      setArea("repository")
                      setCommandOpen(false)
                      setNotice(`已打开仓库：${repository.name}`)
                    }}
                    value={`${repository.name} ${repository.id}`}
                  >
                    <RepositoryIcon />
                    {repository.name}
                    <CommandShortcut>
                      {repository.default_branch}
                    </CommandShortcut>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}
          </CommandList>
        </Command>
      </CommandDialog>

      <Dialog onOpenChange={setConnectionOpen} open={connectionOpen}>
        <DialogContent>
          <form onSubmit={saveConnection}>
            <DialogHeader>
              <DialogTitle>连接 gitsss</DialogTitle>
              <DialogDescription>
                粘贴管理员授予的角色 Token。它只保存在当前标签页的 Session
                Storage 中。
              </DialogDescription>
            </DialogHeader>
            <label
              className="mt-5 flex flex-col gap-2 text-sm font-medium"
              htmlFor="gitsss-session-token"
            >
              访问 Token
              <Input
                autoComplete="off"
                id="gitsss-session-token"
                onChange={(event) => setTokenDraft(event.target.value)}
                placeholder="粘贴 Token"
                type="password"
                value={tokenDraft}
              />
            </label>
            <DialogFooter className="mt-5">
              <Button type="submit">
                {tokenDraft.trim() ? "连接" : "断开"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default App
