import { useState } from "react"

import { Button } from "@/components/ui/button"
import { CheckIcon, TakeoverIcon } from "@/components/ui/icons"
import { Input } from "@/components/ui/input"
import { apiMutation, apiRequest } from "@/lib/api"

interface Asset {
  kind: string
  path: string
  exists: boolean
  readable: boolean
  files: number
  bytes: number
  truncated: boolean
}

interface Finding {
  severity: "blocker" | "warning" | "info"
  code: string
  message: string
}

interface Assessment {
  id: string
  agent_id: string
  config_path: string
  config_digest: string
  source_version?: string
  public_origin?: string
  database: { type: string; name?: string; location: string; tls_mode?: string }
  assets: Asset[]
  actions_enabled: boolean
  secret_field_count: number
  findings: Finding[]
  assessed_at: string
}

interface AgentJob {
  id: string
  kind:
    | "takeover.assess"
    | "backup.audit-restore"
    | "backup.export"
    | "backup.restore-isolated"
    | "backup.verify"
    | "takeover.shadow-audit"
    | "takeover.freeze-check"
    | "takeover.cutover"
    | "takeover.rollback"
  status: "queued" | "running" | "succeeded" | "failed"
  input: Record<string, unknown>
  result?:
    | Assessment
    | BackupAuditResult
    | BackupExportResult
    | BackupRestoreResult
    | ShadowAuditResult
    | FreezeEvidence
    | CutoverResult
    | RollbackResult
  error?: string
  claimed_by?: string
}

interface BackupExportResult {
  assessment_id: string
  output_path: string
  files: number
  source_bytes: number
  container_bytes: number
  container_sha256: string
  freeze_id?: string
}

interface FreezeEvidence {
  id: string
  agent_id: string
  network: string
  address: string
  offline: boolean
  finding?: string
  checked_at: string
}

interface BackupRestoreResult {
  assessment_id: string
  target: string
  content_path: string
  files: number
  bytes: number
  container_sha256: string
  restored_at: string
}

interface BackupAuditResult {
  assessment_id: string
  target: string
  files: {
    new: number
    identical: number
    conflicts: number
    blocked: number
  }
  git?: Array<{
    path: string
    valid: boolean
    objects: number
    refs: number
    reference_sha256?: string
    error?: string
  }>
  lfs_objects: number
  lfs_invalid?: Array<{
    path: string
    expected_oid: string
    actual_sha256: string
  }>
  findings?: string[]
  ready_for_shadow_mode: boolean
  container_sha256: string
  audited_at: string
}

interface ShadowAuditResult {
  assessment_id: string
  target: string
  config_matches: boolean
  restore_ready: boolean
  assets: Array<{
    kind: string
    source_files: number
    snapshot_files: number
    source_bytes: number
    snapshot_bytes: number
    matches: boolean
  }>
  repository_summary: {
    source: number
    restored: number
    matched: number
    mismatched: number
  }
  repository_mismatches?: Array<{
    path: string
    refs_match: boolean
    objects_match: boolean
  }>
  mismatch_truncated: boolean
  findings?: Finding[]
  ready_for_shadow_mode: boolean
  audited_at: string
}

interface CutoverResult {
  id: string
  assessment_id: string
  active_path: string
  rollback_path?: string
  state_path: string
  previous_active: boolean
  activated_sha256: string
  cutover_at: string
}

interface RollbackResult {
  cutover_id: string
  active_path: string
  archived_path: string
  restored_previous: boolean
  rolled_back_at: string
}

const assetNames: Record<string, string> = {
  repositories: "Git 仓库",
  lfs: "LFS",
  attachments: "附件",
  packages: "软件包",
  "actions-log": "流水线日志",
  "actions-artifacts": "流水线制品",
  database: "数据库文件",
  "database-snapshot": "内置数据库快照",
}

const takeoverSteps = [
  "连接 AI Agent",
  "只读评估",
  "冻结/快照",
  "加密备份",
  "隔离恢复",
  "数据审计",
  "影子模式",
  "原子切换",
  "回滚保护",
]

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  return `${(bytes / 1024 ** 3).toFixed(1)} GB`
}

function jobStatus(job?: AgentJob) {
  if (!job) return "尚未下发"
  if (job.status === "queued") return "等待 Agent 领取"
  if (job.status === "running") return `由 ${job.claimed_by ?? "Agent"} 执行中`
  if (job.status === "failed") return job.error || "执行失败"
  if (job.kind === "backup.audit-restore") return "数据审计已回传"
  if (job.kind === "backup.export") return "加密备份已回传"
  if (job.kind === "backup.restore-isolated") return "隔离恢复已回传"
  if (job.kind === "backup.verify") return "备份验证已回传"
  if (job.kind === "takeover.shadow-audit") return "影子差异已回传"
  if (job.kind === "takeover.freeze-check") return "停服状态已回传"
  if (job.kind === "takeover.cutover") return "目录切换已完成"
  if (job.kind === "takeover.rollback") return "回滚已完成"
  return "评估已回传"
}

function isAssessment(
  result:
    | Assessment
    | BackupAuditResult
    | BackupExportResult
    | BackupRestoreResult
    | ShadowAuditResult
    | FreezeEvidence
    | CutoverResult
    | RollbackResult
    | undefined
): result is Assessment {
  return Boolean(result && "assets" in result)
}

function isBackupExport(
  result:
    | Assessment
    | BackupAuditResult
    | BackupExportResult
    | BackupRestoreResult
    | ShadowAuditResult
    | FreezeEvidence
    | CutoverResult
    | RollbackResult
    | undefined
): result is BackupExportResult {
  return Boolean(result && "output_path" in result)
}

function isBackupRestore(
  result:
    | Assessment
    | BackupAuditResult
    | BackupExportResult
    | BackupRestoreResult
    | ShadowAuditResult
    | FreezeEvidence
    | CutoverResult
    | RollbackResult
    | undefined
): result is BackupRestoreResult {
  return Boolean(result && "content_path" in result)
}

function isBackupAudit(
  result:
    | Assessment
    | BackupAuditResult
    | BackupExportResult
    | BackupRestoreResult
    | ShadowAuditResult
    | FreezeEvidence
    | CutoverResult
    | RollbackResult
    | undefined
): result is BackupAuditResult {
  return Boolean(result && "files" in result && "lfs_objects" in result)
}

function isShadowAudit(
  result:
    | Assessment
    | BackupAuditResult
    | BackupExportResult
    | BackupRestoreResult
    | ShadowAuditResult
    | FreezeEvidence
    | CutoverResult
    | RollbackResult
    | undefined
): result is ShadowAuditResult {
  return Boolean(result && "repository_summary" in result)
}

function isFreezeEvidence(
  result:
    | Assessment
    | BackupAuditResult
    | BackupExportResult
    | BackupRestoreResult
    | ShadowAuditResult
    | FreezeEvidence
    | CutoverResult
    | RollbackResult
    | undefined
): result is FreezeEvidence {
  return Boolean(result && "offline" in result && "network" in result)
}

function isCutoverResult(result: AgentJob["result"]): result is CutoverResult {
  return Boolean(
    result && "state_path" in result && "activated_sha256" in result
  )
}

function isRollbackResult(
  result: AgentJob["result"]
): result is RollbackResult {
  return Boolean(result && "cutover_id" in result && "rolled_back_at" in result)
}

export function TakeoverCenter({
  onConnect,
  token,
}: {
  onConnect: () => void
  token: string
}) {
  const [agentID, setAgentID] = useState("old-server")
  const [configPath, setConfigPath] = useState("/etc/old-service/app.ini")
  const [workPath, setWorkPath] = useState("")
  const [backupOutputPath, setBackupOutputPath] = useState(
    "/var/backups/source.gitsss-backup"
  )
  const [passphraseEnv, setPassphraseEnv] = useState("GITSSS_BACKUP_PASSPHRASE")
  const [backupConfirmation, setBackupConfirmation] = useState("")
  const [restoreTargetPath, setRestoreTargetPath] = useState(
    "/var/lib/gitsss/imports/source"
  )
  const [restoreConfirmation, setRestoreConfirmation] = useState("")
  const [activePath, setActivePath] = useState("/var/lib/gitsss/data")
  const [cutoverConfirmation, setCutoverConfirmation] = useState("")
  const [rollbackConfirmation, setRollbackConfirmation] = useState("")
  const [reports, setReports] = useState<Assessment[]>([])
  const [jobs, setJobs] = useState<AgentJob[]>([])
  const [selectedID, setSelectedID] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const jobReports = jobs.flatMap((job) =>
    job.kind === "takeover.assess" &&
    job.status === "succeeded" &&
    isAssessment(job.result)
      ? [job.result]
      : []
  )
  const knownIDs = new Set(reports.map((report) => report.id))
  const allReports = [
    ...reports,
    ...jobReports.filter((report) => !knownIDs.has(report.id)),
  ]
  const selected =
    allReports.find((report) => report.id === selectedID) ?? allReports[0]
  const assessmentJob = jobs.find(
    (job) => job.kind === "takeover.assess" && job.input.agent_id === agentID
  )
  const selectedAssessmentJob = jobs.find(
    (job) =>
      job.kind === "takeover.assess" &&
      isAssessment(job.result) &&
      job.result.id === selected?.id
  )
  const assessedWorkPath =
    typeof selectedAssessmentJob?.input.work_path === "string"
      ? selectedAssessmentJob.input.work_path
      : workPath
  const requiresFreeze =
    selected?.database.type === "postgres" ||
    selected?.database.type === "mysql"
  const freezeJob = jobs.find(
    (job) =>
      job.kind === "takeover.freeze-check" &&
      job.input.agent_id === selected?.agent_id &&
      job.input.config_path === selected?.config_path
  )
  const freezeResult =
    freezeJob?.status === "succeeded" && isFreezeEvidence(freezeJob.result)
      ? freezeJob.result
      : undefined
  const freezeSatisfied = !requiresFreeze || freezeResult?.offline === true
  const backupJob = jobs.find(
    (job) =>
      job.kind === "backup.export" &&
      job.input.expected_assessment_id === selected?.id
  )
  const backupResult =
    backupJob?.status === "succeeded" && isBackupExport(backupJob.result)
      ? backupJob.result
      : undefined
  const backupPassphraseEnv =
    typeof backupJob?.input.passphrase_env === "string"
      ? backupJob.input.passphrase_env
      : passphraseEnv
  const restoreJob = jobs.find(
    (job) =>
      job.kind === "backup.restore-isolated" &&
      job.input.expected_container_sha256 === backupResult?.container_sha256
  )
  const restoreResult =
    restoreJob?.status === "succeeded" && isBackupRestore(restoreJob.result)
      ? restoreJob.result
      : undefined
  const auditJob = jobs.find(
    (job) =>
      job.kind === "backup.audit-restore" &&
      job.input.expected_container_sha256 === backupResult?.container_sha256 &&
      job.input.target_path === restoreResult?.target
  )
  const auditResult =
    auditJob?.status === "succeeded" && isBackupAudit(auditJob.result)
      ? auditJob.result
      : undefined
  const shadowJob = jobs.find(
    (job) =>
      job.kind === "takeover.shadow-audit" &&
      job.input.expected_container_sha256 === backupResult?.container_sha256 &&
      job.input.target_path === restoreResult?.target
  )
  const shadowResult =
    shadowJob?.status === "succeeded" && isShadowAudit(shadowJob.result)
      ? shadowJob.result
      : undefined
  const cutoverJob = jobs.find(
    (job) =>
      job.kind === "takeover.cutover" &&
      job.input.expected_container_sha256 === backupResult?.container_sha256 &&
      job.input.target_path === restoreResult?.target
  )
  const cutoverResult =
    cutoverJob?.status === "succeeded" && isCutoverResult(cutoverJob.result)
      ? cutoverJob.result
      : undefined
  const rollbackJob = jobs.find(
    (job) =>
      job.kind === "takeover.rollback" &&
      job.input.cutover_id === cutoverResult?.id
  )
  const rollbackResult =
    rollbackJob?.status === "succeeded" && isRollbackResult(rollbackJob.result)
      ? rollbackJob.result
      : undefined

  const refresh = async () => {
    setLoading(true)
    setError("")
    try {
      const [nextReports, nextJobs] = await Promise.all([
        apiRequest<Assessment[]>("/api/v1/takeover/assessments", token),
        apiRequest<AgentJob[]>("/api/v1/agent/jobs", token),
      ])
      setReports(nextReports)
      setJobs(nextJobs)
      const nextJobResult = nextJobs.find(
        (job) => job.kind === "takeover.assess" && isAssessment(job.result)
      )?.result
      const nextJobReport = isAssessment(nextJobResult)
        ? nextJobResult
        : undefined
      setSelectedID(nextReports[0]?.id ?? nextJobReport?.id ?? "")
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法读取接管状态")
    } finally {
      setLoading(false)
    }
  }

  const dispatchAssessment = async () => {
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "takeover.assess",
        input: {
          agent_id: agentID,
          config_path: configPath,
          work_path: workPath || undefined,
          max_entries: 10_000,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法下发评估")
    } finally {
      setLoading(false)
    }
  }

  const dispatchBackup = async () => {
    if (!selected) return
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "backup.export",
        input: {
          agent_id: selected.agent_id,
          config_path: selected.config_path,
          work_path: assessedWorkPath || undefined,
          source_version: selected.source_version,
          max_entries: 1_000_000,
          output_path: backupOutputPath,
          passphrase_env: passphraseEnv,
          expected_assessment_id: backupConfirmation,
          expected_freeze_id: freezeResult?.offline
            ? freezeResult.id
            : undefined,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法下发加密备份")
    } finally {
      setLoading(false)
    }
  }

  const dispatchFreezeCheck = async () => {
    if (!selected) return
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "takeover.freeze-check",
        input: {
          agent_id: selected.agent_id,
          config_path: selected.config_path,
          work_path: assessedWorkPath || undefined,
          source_version: selected.source_version,
          max_entries: 1_000_000,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法验证停服状态")
    } finally {
      setLoading(false)
    }
  }

  const dispatchRestore = async () => {
    if (!selected || !backupResult) return
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "backup.restore-isolated",
        input: {
          agent_id: selected.agent_id,
          input_path: backupResult.output_path,
          passphrase_env: backupPassphraseEnv,
          target_path: restoreTargetPath,
          expected_assessment_id: backupResult.assessment_id,
          expected_container_sha256: restoreConfirmation,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法下发隔离恢复")
    } finally {
      setLoading(false)
    }
  }

  const dispatchAudit = async () => {
    if (!selected || !backupResult || !restoreResult) return
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "backup.audit-restore",
        input: {
          agent_id: selected.agent_id,
          input_path: backupResult.output_path,
          passphrase_env: backupPassphraseEnv,
          target_path: restoreResult.target,
          expected_assessment_id: backupResult.assessment_id,
          expected_container_sha256: backupResult.container_sha256,
          expected_freeze_id: backupResult.freeze_id,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法下发数据审计")
    } finally {
      setLoading(false)
    }
  }

  const dispatchShadowAudit = async () => {
    if (!selected || !backupResult || !restoreResult || !auditResult) return
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "takeover.shadow-audit",
        input: {
          agent_id: selected.agent_id,
          config_path: selected.config_path,
          work_path: assessedWorkPath || undefined,
          source_version: selected.source_version,
          max_entries: 1_000_000,
          input_path: backupResult.output_path,
          passphrase_env: backupPassphraseEnv,
          target_path: restoreResult.target,
          expected_assessment_id: backupResult.assessment_id,
          expected_container_sha256: backupResult.container_sha256,
          expected_freeze_id: backupResult.freeze_id,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法下发影子审计")
    } finally {
      setLoading(false)
    }
  }

  const dispatchCutover = async () => {
    if (!selected || !backupResult || !restoreResult || !shadowResult) return
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "takeover.cutover",
        input: {
          agent_id: selected.agent_id,
          config_path: selected.config_path,
          work_path: assessedWorkPath || undefined,
          source_version: selected.source_version,
          max_entries: 1_000_000,
          input_path: backupResult.output_path,
          passphrase_env: backupPassphraseEnv,
          target_path: restoreResult.target,
          active_path: activePath,
          expected_assessment_id: backupResult.assessment_id,
          expected_container_sha256: backupResult.container_sha256,
          expected_freeze_id: backupResult.freeze_id,
          confirmation: cutoverConfirmation,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法下发原子切换")
    } finally {
      setLoading(false)
    }
  }

  const dispatchRollback = async () => {
    if (!selected || !cutoverResult) return
    setLoading(true)
    setError("")
    try {
      const job = await apiMutation<AgentJob>("/api/v1/agent/jobs", token, {
        kind: "takeover.rollback",
        input: {
          agent_id: selected.agent_id,
          active_path: cutoverResult.active_path,
          state_path: cutoverResult.state_path,
          cutover_id: cutoverResult.id,
          confirmation: rollbackConfirmation,
        },
      })
      setJobs((current) => [job, ...current])
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法下发回滚")
    } finally {
      setLoading(false)
    }
  }

  const blockers = selected?.findings.filter(
    (finding) => finding.severity === "blocker"
  ).length
  const totalBytes = selected?.assets.reduce(
    (total, asset) => total + asset.bytes,
    0
  )
  const completedSteps = cutoverResult
    ? 9
    : shadowResult?.ready_for_shadow_mode
      ? 7
      : auditResult?.ready_for_shadow_mode
        ? 6
        : restoreResult
          ? 5
          : backupResult
            ? 4
            : selected && freezeSatisfied
              ? 3
              : selected
                ? 2
                : assessmentJob?.status === "running"
                  ? 1
                  : 0

  return (
    <section className="mx-auto max-w-5xl pt-8">
      <div className="max-w-2xl">
        <h2 className="text-xl font-semibold tracking-[-0.035em]">
          AI 接管旧服务器
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          Agent
          主动领取受限任务。先评估、再备份和隔离验证，任何阶段都不会直接覆盖生产数据。
        </p>
      </div>

      <ol className="mt-8 grid grid-cols-2 border-y border-zinc-200 py-4 text-xs text-zinc-500 sm:grid-cols-3 lg:grid-cols-9">
        {takeoverSteps.map((step, index) => {
          const complete = index < completedSteps
          const active = index === completedSteps
          return (
            <li
              aria-current={active ? "step" : undefined}
              className={`flex min-w-0 items-center gap-2 py-2 ${complete || active ? "font-medium text-zinc-950" : ""}`}
              key={step}
            >
              <span
                className={`flex size-5 shrink-0 items-center justify-center rounded-full border ${complete || active ? "border-zinc-950 bg-zinc-950 text-white" : "border-zinc-300"}`}
              >
                {complete ? <CheckIcon className="size-3" /> : index + 1}
              </span>
              <span className="truncate">{step}</span>
            </li>
          )
        })}
      </ol>

      <div className="mt-8 grid min-w-0 gap-10 lg:grid-cols-[19rem_minmax(0,1fr)]">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold">连接旧服务器</h3>
          <p className="mt-2 text-xs leading-5 text-zinc-500">
            旧服务器运行 <code>gitsss-agent work-once</code>{" "}
            后会主动领取这里的任务，无需开放入站端口。
          </p>
          <div className="mt-5 grid gap-4">
            <label
              className="grid gap-2 text-sm font-medium"
              htmlFor="takeover-agent-id"
            >
              Agent 设备 ID
              <Input
                id="takeover-agent-id"
                onChange={(event) => setAgentID(event.target.value)}
                value={agentID}
              />
            </label>
            <label
              className="grid gap-2 text-sm font-medium"
              htmlFor="takeover-config-path"
            >
              app.ini 路径
              <Input
                id="takeover-config-path"
                onChange={(event) => setConfigPath(event.target.value)}
                value={configPath}
              />
            </label>
            <label
              className="grid gap-2 text-sm font-medium"
              htmlFor="takeover-work-path"
            >
              工作目录（可选）
              <Input
                id="takeover-work-path"
                onChange={(event) => setWorkPath(event.target.value)}
                placeholder="由 Agent 自动识别"
                value={workPath}
              />
            </label>
            <div className="flex items-center justify-between gap-4 border-y border-zinc-200 py-3 text-sm">
              <span>
                服务器连接
                <span className="ml-2 text-xs text-zinc-400">
                  {token ? "已就绪" : "未连接"}
                </span>
              </span>
              <Button onClick={onConnect} size="sm" variant="outline">
                {token ? "更换" : "连接"}
              </Button>
            </div>
          </div>
          <div className="mt-5 flex flex-wrap gap-2">
            <Button
              disabled={loading || !token || !agentID || !configPath}
              onClick={dispatchAssessment}
            >
              {loading ? "处理中…" : "下发只读评估"}
            </Button>
            <Button
              disabled={loading || !token}
              onClick={refresh}
              variant="outline"
            >
              刷新状态
            </Button>
          </div>
          <output
            className={`mt-4 text-xs ${assessmentJob?.status === "failed" ? "text-red-600" : "text-zinc-500"}`}
          >
            {jobStatus(assessmentJob)}
          </output>
          {error ? (
            <p className="mt-3 text-sm text-red-600" role="alert">
              {error}
            </p>
          ) : null}
        </div>

        {selected ? (
          <div className="min-w-0">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div className="min-w-0">
                <p className="text-xs text-zinc-500">只读评估已回传</p>
                <h3 className="mt-1 text-lg font-semibold tracking-[-0.03em]">
                  {selected.public_origin || selected.agent_id}
                </h3>
                <p className="mt-1 truncate text-xs text-zinc-400">
                  {selected.config_path}
                </p>
              </div>
              {allReports.length > 1 ? (
                <select
                  className="h-9 rounded-lg border border-zinc-200 bg-white px-3 text-sm"
                  onChange={(event) => setSelectedID(event.target.value)}
                  value={selected.id}
                >
                  {allReports.map((report) => (
                    <option key={report.id} value={report.id}>
                      {report.agent_id}
                    </option>
                  ))}
                </select>
              ) : null}
            </div>

            <dl className="mt-6 grid grid-cols-3 divide-x divide-zinc-200 border-y border-zinc-200 py-5">
              <div className="pr-4">
                <dt className="text-xs text-zinc-500">数据库</dt>
                <dd className="mt-1 text-sm font-medium">
                  {selected.database.type}
                </dd>
              </div>
              <div className="px-4">
                <dt className="text-xs text-zinc-500">已发现数据</dt>
                <dd className="mt-1 text-sm font-medium">
                  {formatBytes(totalBytes ?? 0)}
                </dd>
              </div>
              <div className="pl-4">
                <dt className="text-xs text-zinc-500">阻断项</dt>
                <dd
                  className={`mt-1 text-sm font-medium ${blockers ? "text-red-600" : "text-emerald-700"}`}
                >
                  {blockers ?? 0}
                </dd>
              </div>
            </dl>

            <div className="mt-7 divide-y divide-zinc-200 border-y border-zinc-200">
              {selected.assets.map((asset) => (
                <div
                  className="grid grid-cols-[minmax(0,1fr)_auto] gap-4 py-4"
                  key={asset.kind}
                >
                  <div className="min-w-0">
                    <p className="flex items-center gap-2 text-sm font-medium">
                      {asset.exists && asset.readable ? (
                        <CheckIcon className="size-4 text-emerald-700" />
                      ) : (
                        <TakeoverIcon className="size-4 text-amber-600" />
                      )}
                      {assetNames[asset.kind] ?? asset.kind}
                    </p>
                    <p className="mt-1 truncate text-xs text-zinc-400">
                      {asset.path}
                    </p>
                  </div>
                  <p className="text-right text-xs text-zinc-500">
                    {asset.exists
                      ? `${asset.files} 个文件 · ${formatBytes(asset.bytes)}`
                      : "未找到"}
                  </p>
                </div>
              ))}
            </div>

            <div className="mt-7 border-y border-zinc-200 py-5">
              <p className="text-sm font-medium">下一步：加密备份</p>
              <p className="mt-2 text-xs leading-5 text-zinc-500">
                已脱敏 {selected.secret_field_count}{" "}
                个敏感字段。评估通过后，系统将要求确认范围和口令环境变量，再生成不可覆盖的加密备份；当前不会自动开始写入。
              </p>
              {selected.findings.map((finding) => (
                <p
                  className="mt-3 text-xs leading-5 text-zinc-600"
                  key={finding.code}
                >
                  • {finding.message}
                </p>
              ))}
              {requiresFreeze ? (
                <div className="mt-5 border-l-2 border-amber-500 pl-4 text-xs leading-5 text-zinc-600">
                  <p className="font-medium text-zinc-900">
                    先停止旧实例，再验证冻结窗口
                  </p>
                  <p className="mt-1">
                    PostgreSQL/MySQL 由 Agent
                    内嵌驱动导出，不需要安装数据库工具；为了让数据库、Git
                    和附件处在同一最终状态，旧实例必须停止写入。
                  </p>
                  <div className="mt-3 flex flex-wrap items-center gap-3">
                    <Button
                      disabled={
                        loading ||
                        freezeJob?.status === "queued" ||
                        freezeJob?.status === "running"
                      }
                      onClick={dispatchFreezeCheck}
                      variant="outline"
                    >
                      {freezeJob ? "重新验证停服" : "我已停止旧实例，开始验证"}
                    </Button>
                    <output
                      className={
                        freezeResult?.offline
                          ? "text-emerald-700"
                          : "text-zinc-500"
                      }
                    >
                      {freezeResult?.offline
                        ? `已冻结 · ${freezeResult.id}`
                        : freezeResult?.finding ||
                          (freezeJob
                            ? jobStatus(freezeJob)
                            : "Agent 将直接探测本地监听")}
                    </output>
                  </div>
                </div>
              ) : (
                <p className="mt-4 border-l-2 border-emerald-500 pl-4 text-xs leading-5 text-zinc-600">
                  SQLite 使用 gitsss 内嵌 online-backup 引擎吸收
                  WAL，不需要任何外部数据库或命令行工具。
                </p>
              )}
              <div className="mt-5 grid gap-4 sm:grid-cols-2">
                <label
                  className="grid gap-2 text-xs font-medium"
                  htmlFor="backup-output-path"
                >
                  旧服务器上的备份路径
                  <Input
                    id="backup-output-path"
                    onChange={(event) =>
                      setBackupOutputPath(event.target.value)
                    }
                    value={backupOutputPath}
                  />
                </label>
                <label
                  className="grid gap-2 text-xs font-medium"
                  htmlFor="backup-passphrase-env"
                >
                  口令环境变量名
                  <Input
                    id="backup-passphrase-env"
                    onChange={(event) => setPassphraseEnv(event.target.value)}
                    value={passphraseEnv}
                  />
                </label>
              </div>
              <label
                className="mt-4 grid gap-2 text-xs font-medium"
                htmlFor="backup-confirmation"
              >
                输入评估 ID 以确认范围
                <Input
                  id="backup-confirmation"
                  onChange={(event) =>
                    setBackupConfirmation(event.target.value)
                  }
                  placeholder={selected.id}
                  value={backupConfirmation}
                />
              </label>
              <div className="mt-5 flex flex-wrap items-center gap-3">
                <Button
                  disabled={
                    Boolean(blockers) ||
                    !freezeSatisfied ||
                    loading ||
                    backupConfirmation !== selected.id ||
                    !backupOutputPath ||
                    !passphraseEnv ||
                    backupJob?.status === "queued" ||
                    backupJob?.status === "running"
                  }
                  onClick={dispatchBackup}
                  variant="outline"
                >
                  {blockers
                    ? "先处理阻断项"
                    : !freezeSatisfied
                      ? "先验证旧实例已停止"
                      : backupJob
                        ? "重新下发加密备份"
                        : "下发加密备份"}
                </Button>
                <output
                  className={`text-xs ${backupJob?.status === "failed" ? "text-red-600" : "text-zinc-500"}`}
                >
                  {backupJob ? jobStatus(backupJob) : "口令不会离开旧服务器"}
                </output>
              </div>
              {backupResult ? (
                <div className="mt-5 border-l-2 border-emerald-500 pl-4 text-xs leading-5 text-zinc-600">
                  <p className="font-medium text-emerald-800">加密备份已生成</p>
                  <p>
                    {backupResult.files} 个文件 ·{" "}
                    {formatBytes(backupResult.container_bytes)}
                  </p>
                  {backupResult.freeze_id ? (
                    <p>冻结证据 {backupResult.freeze_id}</p>
                  ) : null}
                  <p className="truncate font-mono text-[11px] text-zinc-500">
                    {backupResult.output_path}
                  </p>
                  <p className="truncate font-mono text-[11px] text-zinc-400">
                    SHA-256 {backupResult.container_sha256}
                  </p>
                </div>
              ) : null}
            </div>
            {backupResult ? (
              <div className="mt-7 border-y border-zinc-200 py-5">
                <p className="text-sm font-medium">下一步：隔离恢复</p>
                <p className="mt-2 text-xs leading-5 text-zinc-500">
                  恢复到全新目录，不会覆盖旧实例或 gitsss 的在线数据。Agent
                  会在发布目录前再次验证容器，并要求前后 SHA-256 完全一致。
                </p>
                <label
                  className="mt-5 grid gap-2 text-xs font-medium"
                  htmlFor="restore-target-path"
                >
                  旧服务器上的隔离目录
                  <Input
                    id="restore-target-path"
                    onChange={(event) =>
                      setRestoreTargetPath(event.target.value)
                    }
                    value={restoreTargetPath}
                  />
                </label>
                <label
                  className="mt-4 grid gap-2 text-xs font-medium"
                  htmlFor="restore-confirmation"
                >
                  输入容器 SHA-256 以确认
                  <Input
                    id="restore-confirmation"
                    onChange={(event) =>
                      setRestoreConfirmation(event.target.value)
                    }
                    placeholder={backupResult.container_sha256}
                    value={restoreConfirmation}
                  />
                </label>
                <div className="mt-5 flex flex-wrap items-center gap-3">
                  <Button
                    disabled={
                      loading ||
                      !restoreTargetPath ||
                      restoreConfirmation !== backupResult.container_sha256 ||
                      restoreJob?.status === "queued" ||
                      restoreJob?.status === "running"
                    }
                    onClick={dispatchRestore}
                    variant="outline"
                  >
                    {restoreJob ? "重新下发隔离恢复" : "下发隔离恢复"}
                  </Button>
                  <output
                    className={`text-xs ${restoreJob?.status === "failed" ? "text-red-600" : "text-zinc-500"}`}
                  >
                    {restoreJob
                      ? jobStatus(restoreJob)
                      : "目标必须是尚不存在的目录"}
                  </output>
                </div>
                {restoreResult ? (
                  <div className="mt-5 border-l-2 border-emerald-500 pl-4 text-xs leading-5 text-zinc-600">
                    <p className="font-medium text-emerald-800">
                      隔离副本已发布
                    </p>
                    <p>
                      {restoreResult.files} 个文件 ·{" "}
                      {formatBytes(restoreResult.bytes)}
                    </p>
                    <p className="truncate font-mono text-[11px] text-zinc-500">
                      {restoreResult.content_path}
                    </p>
                  </div>
                ) : null}
              </div>
            ) : null}
            {restoreResult ? (
              <div className="mt-7 border-y border-zinc-200 py-5">
                <p className="text-sm font-medium">下一步：数据审计</p>
                <p className="mt-2 text-xs leading-5 text-zinc-500">
                  逐文件对照清单，并对每个 bare Git 执行严格 fsck、统计对象和
                  refs，同时校验 LFS OID。该任务只读，不会修复或改写副本。
                </p>
                <div className="mt-5 flex flex-wrap items-center gap-3">
                  <Button
                    disabled={
                      loading ||
                      auditJob?.status === "queued" ||
                      auditJob?.status === "running"
                    }
                    onClick={dispatchAudit}
                    variant="outline"
                  >
                    {auditJob ? "重新运行数据审计" : "开始数据审计"}
                  </Button>
                  <output
                    className={`text-xs ${auditJob?.status === "failed" ? "text-red-600" : "text-zinc-500"}`}
                  >
                    {auditJob ? jobStatus(auditJob) : "等待只读审计"}
                  </output>
                </div>
                {auditResult ? (
                  <div
                    className={`mt-5 border-l-2 pl-4 text-xs leading-5 ${auditResult.ready_for_shadow_mode ? "border-emerald-500 text-zinc-600" : "border-red-500 text-red-700"}`}
                  >
                    <p className="font-medium">
                      {auditResult.ready_for_shadow_mode
                        ? "数据一致，可以进入影子模式"
                        : "审计发现阻断项"}
                    </p>
                    <p>
                      文件 {auditResult.files.identical} 一致 ·{" "}
                      {auditResult.files.conflicts} 冲突 ·{" "}
                      {auditResult.files.blocked} 阻断
                    </p>
                    <p>
                      Git{" "}
                      {auditResult.git?.filter((item) => item.valid).length ??
                        0}
                      /{auditResult.git?.length ?? 0} 通过 · LFS{" "}
                      {auditResult.lfs_objects} 个对象 ·{" "}
                      {auditResult.lfs_invalid?.length ?? 0} 异常
                    </p>
                    {auditResult.git
                      ?.filter((repository) => !repository.valid)
                      .map((repository) => (
                        <p key={repository.path}>
                          {repository.path}：
                          {repository.error || "Git 校验失败"}
                        </p>
                      ))}
                    {auditResult.lfs_invalid?.map((issue) => (
                      <p className="truncate" key={issue.path}>
                        {issue.path}：LFS OID 不一致
                      </p>
                    ))}
                    {auditResult.findings?.map((finding) => (
                      <p key={finding}>{finding}</p>
                    ))}
                  </div>
                ) : null}
              </div>
            ) : null}
            {auditResult?.ready_for_shadow_mode ? (
              <div className="mt-7 border-y border-zinc-200 py-5">
                <p className="text-sm font-medium">下一步：影子模式预检</p>
                <p className="mt-2 text-xs leading-5 text-zinc-500">
                  再次读取源配置和资产清单，并逐仓库比较 live refs
                  指纹与隔离副本。SQLite
                  与远程源库都会由内嵌驱动重新生成确定性快照；冻结证据失效或数据变化会明确阻断。
                </p>
                <div className="mt-5 flex flex-wrap items-center gap-3">
                  <Button
                    disabled={
                      loading ||
                      shadowJob?.status === "queued" ||
                      shadowJob?.status === "running"
                    }
                    onClick={dispatchShadowAudit}
                    variant="outline"
                  >
                    {shadowJob ? "重新运行影子预检" : "开始影子预检"}
                  </Button>
                  <output
                    className={
                      shadowJob?.status === "failed"
                        ? "text-xs text-red-600"
                        : "text-xs text-zinc-500"
                    }
                  >
                    {shadowJob ? jobStatus(shadowJob) : "等待源与副本差异审计"}
                  </output>
                </div>
                {shadowResult ? (
                  <div
                    className={
                      shadowResult.ready_for_shadow_mode
                        ? "mt-5 border-l-2 border-emerald-500 pl-4 text-xs leading-5 text-zinc-600"
                        : "mt-5 border-l-2 border-red-500 pl-4 text-xs leading-5 text-red-700"
                    }
                  >
                    <p className="font-medium">
                      {shadowResult.ready_for_shadow_mode
                        ? "影子模式预检通过"
                        : "源实例与快照之间存在阻断差异"}
                    </p>
                    <p>
                      配置{shadowResult.config_matches ? "一致" : "已变化"} ·
                      恢复{shadowResult.restore_ready ? "通过" : "未通过"} · Git
                      refs {shadowResult.repository_summary.matched}/
                      {shadowResult.repository_summary.source} 一致
                    </p>
                    {shadowResult.assets
                      .filter((asset) => !asset.matches)
                      .map((asset) => (
                        <p key={asset.kind}>
                          {assetNames[asset.kind] ?? asset.kind}：源
                          {asset.source_files} 个/
                          {formatBytes(asset.source_bytes)}，快照
                          {asset.snapshot_files} 个/
                          {formatBytes(asset.snapshot_bytes)}
                        </p>
                      ))}
                    {shadowResult.repository_mismatches?.map((repository) => (
                      <p className="truncate" key={repository.path}>
                        {repository.path}：refs 指纹不一致
                      </p>
                    ))}
                    {shadowResult.mismatch_truncated ? (
                      <p>差异过多，仅显示前 64 项。</p>
                    ) : null}
                    {shadowResult.findings?.map((finding) => (
                      <p key={finding.code + finding.message}>
                        {finding.message}
                      </p>
                    ))}
                  </div>
                ) : null}
              </div>
            ) : null}
            {shadowResult?.ready_for_shadow_mode ? (
              <div className="mt-7 border-y border-zinc-200 py-5">
                <p className="text-sm font-medium">最终步骤：原子切换</p>
                <p className="mt-2 text-xs leading-5 text-zinc-500">
                  Agent
                  会再次执行完整影子校验，然后保留当前活跃目录作为回滚点，并把隔离副本原子切入。不会删除任一代数据。
                </p>
                <label
                  className="mt-4 grid gap-2 text-xs font-medium"
                  htmlFor="cutover-active-path"
                >
                  gitsss 活跃数据目录
                  <Input
                    id="cutover-active-path"
                    onChange={(event) => setActivePath(event.target.value)}
                    value={activePath}
                  />
                </label>
                <label
                  className="mt-4 grid gap-2 text-xs font-medium"
                  htmlFor="cutover-confirmation"
                >
                  输入 CUTOVER {selected!.id}
                  <Input
                    id="cutover-confirmation"
                    onChange={(event) =>
                      setCutoverConfirmation(event.target.value)
                    }
                    value={cutoverConfirmation}
                  />
                </label>
                <div className="mt-5 flex flex-wrap items-center gap-3">
                  <Button
                    disabled={
                      loading ||
                      !activePath ||
                      cutoverConfirmation !== `CUTOVER ${selected!.id}` ||
                      cutoverJob?.status === "queued" ||
                      cutoverJob?.status === "running" ||
                      Boolean(cutoverResult)
                    }
                    onClick={dispatchCutover}
                  >
                    执行原子切换
                  </Button>
                  <output
                    className={
                      cutoverJob?.status === "failed"
                        ? "text-xs text-red-600"
                        : "text-xs text-zinc-500"
                    }
                  >
                    {cutoverJob
                      ? jobStatus(cutoverJob)
                      : "等待明确确认；切换前会重新校验"}
                  </output>
                </div>
                {cutoverResult ? (
                  <div className="mt-6 border-l-2 border-emerald-500 pl-4 text-xs leading-5 text-zinc-600">
                    <p className="font-medium text-emerald-800">
                      接管完成，回滚点已保留
                    </p>
                    <p className="truncate font-mono text-[11px]">
                      活跃：{cutoverResult.active_path}
                    </p>
                    {cutoverResult.rollback_path ? (
                      <p className="truncate font-mono text-[11px]">
                        回滚：{cutoverResult.rollback_path}
                      </p>
                    ) : (
                      <p>切换前没有活跃目录；回滚会停用本次接管目录。</p>
                    )}
                    <p className="mt-3 text-amber-700">
                      一旦新实例产生写入，安全回滚会拒绝丢弃这些变化。请先停止新实例并导出增量。
                    </p>
                    {!rollbackResult ? (
                      <>
                        <label
                          className="mt-4 grid gap-2 font-medium"
                          htmlFor="rollback-confirmation"
                        >
                          紧急回滚时输入 ROLLBACK {cutoverResult.id}
                          <Input
                            id="rollback-confirmation"
                            onChange={(event) =>
                              setRollbackConfirmation(event.target.value)
                            }
                            value={rollbackConfirmation}
                          />
                        </label>
                        <div className="mt-4 flex flex-wrap items-center gap-3">
                          <Button
                            disabled={
                              loading ||
                              rollbackConfirmation !==
                                `ROLLBACK ${cutoverResult.id}` ||
                              rollbackJob?.status === "queued" ||
                              rollbackJob?.status === "running"
                            }
                            onClick={dispatchRollback}
                            variant="outline"
                          >
                            验证并回滚
                          </Button>
                          <output
                            className={
                              rollbackJob?.status === "failed"
                                ? "text-red-600"
                                : "text-zinc-500"
                            }
                          >
                            {rollbackJob
                              ? jobStatus(rollbackJob)
                              : "未触发；回滚点保持不变"}
                          </output>
                        </div>
                      </>
                    ) : (
                      <p className="mt-4 font-medium text-emerald-800">
                        已恢复切换前目录；本次接管数据保存在
                        {rollbackResult.archived_path}
                      </p>
                    )}
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        ) : (
          <div className="flex min-h-80 flex-col items-center justify-center border-y border-zinc-200 text-center">
            <TakeoverIcon className="size-7 text-zinc-400" />
            <p className="mt-4 text-sm font-medium">等待 AI Agent 领取任务</p>
            <p className="mt-1 max-w-sm text-xs leading-5 text-zinc-400">
              下发只读评估后，旧服务器会主动领取并回传脱敏报告。
            </p>
          </div>
        )}
      </div>
    </section>
  )
}
