import { useCallback, useEffect, useMemo, useState } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { AlertIcon, ClockIcon, WorkIcon } from "@/components/ui/icons"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  relativeTime,
  type PipelineRun,
  type RepositorySummary,
  type RunnerSummary,
} from "@/features/dashboard/dashboard-data"
import { apiMutation, apiRequest } from "@/lib/api"

interface PipelineStep {
  id: string
  name: string
}

interface PipelineTemplate {
  id: string
  name: string
  description: string
  steps: PipelineStep[]
}

const statusLabel: Record<PipelineRun["status"], string> = {
  failed: "失败",
  queued: "等待 Runner",
  running: "运行中",
  succeeded: "成功",
}

export function PipelineCenter({
  onConnect,
  token,
}: {
  onConnect: () => void
  token: string
}) {
  const [repositories, setRepositories] = useState<RepositorySummary[]>([])
  const [templates, setTemplates] = useState<PipelineTemplate[]>([])
  const [runs, setRuns] = useState<PipelineRun[]>([])
  const [runners, setRunners] = useState<RunnerSummary[]>([])
  const [repositoryID, setRepositoryID] = useState("")
  const [templateID, setTemplateID] = useState("")
  const [ref, setRef] = useState("HEAD")
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")

  const refresh = useCallback(
    async (signal?: AbortSignal) => {
      if (!token) return
      setLoading(true)
      setError("")
      try {
        const [nextRepositories, nextTemplates, nextRuns, nextRunners] =
          await Promise.all([
            apiRequest<RepositorySummary[]>(
              "/api/v1/repositories",
              token,
              signal
            ),
            apiRequest<PipelineTemplate[]>(
              "/api/v1/pipelines/templates",
              token,
              signal
            ),
            apiRequest<PipelineRun[]>("/api/v1/pipelines/runs", token, signal),
            apiRequest<RunnerSummary[]>("/api/v1/runners", token, signal),
          ])
        if (signal?.aborted) return
        setRepositories(nextRepositories)
        setTemplates(nextTemplates)
        setRuns(nextRuns)
        setRunners(nextRunners)
        setRepositoryID((current) => current || nextRepositories[0]?.id || "")
        setTemplateID((current) => current || nextTemplates[0]?.id || "")
      } catch (caught) {
        if (signal?.aborted) return
        setError(caught instanceof Error ? caught.message : "流水线读取失败")
      } finally {
        if (!signal?.aborted) setLoading(false)
      }
    },
    [token]
  )

  useEffect(() => {
    const controller = new AbortController()
    void refresh(controller.signal)
    const interval = window.setInterval(
      () => void refresh(controller.signal),
      15_000
    )
    return () => {
      controller.abort()
      window.clearInterval(interval)
    }
  }, [refresh])

  const selectedTemplate = useMemo(
    () => templates.find((template) => template.id === templateID),
    [templateID, templates]
  )
  const readyRunners = runners.filter((runner) => runner.status === "ready")

  const createRun = async () => {
    if (!repositoryID || !templateID || !ref.trim()) return
    setSubmitting(true)
    setError("")
    try {
      const created = await apiMutation<PipelineRun>(
        "/api/v1/pipelines/runs",
        token,
        {
          ref: ref.trim(),
          repository_id: repositoryID,
          template_id: templateID,
        }
      )
      setRuns((current) => [created, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "流水线启动失败")
    } finally {
      setSubmitting(false)
    }
  }

  if (!token) {
    return (
      <section className="mx-auto max-w-2xl pt-10">
        <WorkIcon className="size-8 text-zinc-700" />
        <h2 className="mt-5 text-2xl font-semibold tracking-[-0.04em]">
          连接后开始交付
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          选择仓库和固定模板即可运行；命令、超时和权限由服务器控制。
        </p>
        <Button className="mt-6" onClick={onConnect}>
          连接服务器
        </Button>
      </section>
    )
  }

  return (
    <section className="mx-auto max-w-5xl pt-8">
      <div className="max-w-2xl">
        <h2 className="text-xl font-semibold tracking-[-0.035em]">本地 CI</h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          按仓库、模板、确认三个步骤运行。Runner
          只执行服务器内置的固定命令，不解释仓库脚本。
        </p>
      </div>

      {error ? (
        <Alert className="mt-6" variant="destructive">
          <AlertIcon />
          <AlertTitle>无法继续流水线</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <div className="mt-8 grid gap-10 lg:grid-cols-[minmax(0,.82fr)_minmax(0,1.18fr)]">
        <div>
          <ol className="flex flex-col gap-7">
            <li className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3">
              <span className="flex size-7 items-center justify-center rounded-full bg-zinc-950 text-xs font-medium text-white">
                1
              </span>
              <div>
                <p className="text-sm font-medium">选择仓库</p>
                <Select onValueChange={setRepositoryID} value={repositoryID}>
                  <SelectTrigger className="mt-3 w-full" disabled={loading}>
                    <SelectValue placeholder="选择仓库" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {repositories.map((repository) => (
                        <SelectItem key={repository.id} value={repository.id}>
                          {repository.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            </li>
            <li className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3">
              <span className="flex size-7 items-center justify-center rounded-full bg-zinc-950 text-xs font-medium text-white">
                2
              </span>
              <div>
                <p className="text-sm font-medium">选择验证模板</p>
                <Select onValueChange={setTemplateID} value={templateID}>
                  <SelectTrigger className="mt-3 w-full" disabled={loading}>
                    <SelectValue placeholder="选择模板" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {templates.map((template) => (
                        <SelectItem key={template.id} value={template.id}>
                          {template.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                {selectedTemplate ? (
                  <p className="mt-2 text-xs leading-5 text-zinc-500">
                    {selectedTemplate.description} ·{" "}
                    {selectedTemplate.steps.length} 个固定步骤
                  </p>
                ) : null}
              </div>
            </li>
            <li className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3">
              <span className="flex size-7 items-center justify-center rounded-full bg-zinc-950 text-xs font-medium text-white">
                3
              </span>
              <div>
                <p className="text-sm font-medium">确认提交</p>
                <Input
                  aria-label="Git ref"
                  className="mt-3"
                  onChange={(event) => setRef(event.target.value)}
                  value={ref}
                />
                <p className="mt-2 text-xs leading-5 text-zinc-500">
                  服务器会把 ref 解析成不可变提交；Runner
                  不会跟随之后的分支移动。
                </p>
                <Button
                  className="mt-4"
                  disabled={
                    submitting ||
                    loading ||
                    !repositoryID ||
                    !templateID ||
                    readyRunners.length === 0
                  }
                  onClick={() => void createRun()}
                >
                  {submitting ? "正在加入队列…" : "开始验证"}
                </Button>
                {readyRunners.length === 0 ? (
                  <p className="mt-2 text-xs text-amber-700">
                    当前没有就绪 Runner；请先安装或修复内置 Agent。
                  </p>
                ) : null}
              </div>
            </li>
          </ol>
        </div>

        <div>
          <div className="flex items-center justify-between gap-4">
            <h3 className="text-base font-semibold">最近运行</h3>
            <span className="text-xs text-zinc-400">
              {readyRunners.length} / {runners.length} Runner 就绪
            </span>
          </div>
          <div className="mt-4 divide-y divide-zinc-200 border-y border-zinc-200">
            {runs.slice(0, 12).map((run) => (
              <div className="flex items-center gap-4 py-4" key={run.id}>
                <span
                  aria-hidden="true"
                  className={`size-2 shrink-0 rounded-full ${run.status === "succeeded" ? "bg-emerald-500" : run.status === "failed" ? "bg-red-500" : run.status === "running" ? "bg-blue-500" : "bg-amber-500"}`}
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-zinc-800">
                    {run.repository_id} · {run.template_id}
                  </p>
                  <p className="mt-1 truncate font-mono text-xs text-zinc-400">
                    {run.commit.slice(0, 12)}
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-xs font-medium">
                    {statusLabel[run.status]}
                  </p>
                  <p className="mt-1 flex items-center gap-1 text-xs text-zinc-400">
                    <ClockIcon className="size-3.5" />
                    {relativeTime(run.completed_at ?? run.created_at)}
                  </p>
                </div>
              </div>
            ))}
            {runs.length === 0 ? (
              <p className="py-8 text-sm text-zinc-500">还没有运行记录。</p>
            ) : null}
          </div>
        </div>
      </div>
    </section>
  )
}
