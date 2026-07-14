import { APIError, apiRequest } from "@/lib/api"

export interface DashboardMetric {
  label: string
  value: number
}

export interface DashboardEvent {
  id: string
  action: string
  actor: string
  at: string
  kind: string
}

export interface DashboardSnapshot {
  metrics: DashboardMetric[]
  events: DashboardEvent[]
}

export interface StorageUsage {
  category: string
  bytes: number
  files?: number
}

export interface StorageSnapshot {
  capacity_bytes: number
  used_bytes: number
  available_bytes: number
  reserved_bytes: number
  usage_ratio: number
  warning: boolean
  write_protected: boolean
  measurement_error?: string
  inventory_error?: string
  inventory_truncated: boolean
  measured_at: string
  usage: StorageUsage[]
}

export interface RepositorySummary {
  id: string
  name: string
  default_branch: string
  hosted: boolean
  created_at?: string
}

export interface PipelineRun {
  id: string
  repository_id: string
  commit: string
  template_id: string
  status: "queued" | "running" | "succeeded" | "failed"
  runner_id?: string
  created_at: string
  completed_at?: string
  error?: string
}

export interface RunnerSummary {
  id: string
  platform: string
  version: string
  status: "provisioning" | "ready" | "degraded" | "paused" | "offline"
  healthy: boolean
  updated_at: string
}

export interface OperationalData {
  dashboard?: DashboardSnapshot
  storage?: StorageSnapshot
  repositories?: RepositorySummary[]
  runs?: PipelineRun[]
  runners?: RunnerSummary[]
  authenticationFailed: boolean
  unavailable: string[]
}

const endpoints = [
  ["dashboard", "/api/v1/dashboard"],
  ["storage", "/api/v1/storage"],
  ["repositories", "/api/v1/repositories"],
  ["runs", "/api/v1/pipelines/runs"],
  ["runners", "/api/v1/runners"],
] as const

export async function loadOperationalData(
  token: string,
  signal?: AbortSignal
): Promise<OperationalData> {
  const settled = await Promise.allSettled(
    endpoints.map(([, path]) => apiRequest<unknown>(path, token, signal))
  )
  const result: OperationalData = {
    authenticationFailed: false,
    unavailable: [],
  }
  settled.forEach((entry, index) => {
    const key = endpoints[index]![0]
    if (entry.status === "rejected") {
      if (signal?.aborted) return
      const reason = entry.reason
      if (reason instanceof APIError && reason.status === 401) {
        result.authenticationFailed = true
      }
      result.unavailable.push(
        reason instanceof APIError && reason.status === 403
          ? `${key}:permission`
          : `${key}:error`
      )
      return
    }
    if (key === "dashboard") result.dashboard = entry.value as DashboardSnapshot
    if (key === "storage") result.storage = entry.value as StorageSnapshot
    if (key === "repositories") {
      result.repositories = entry.value as RepositorySummary[]
    }
    if (key === "runs") result.runs = entry.value as PipelineRun[]
    if (key === "runners") result.runners = entry.value as RunnerSummary[]
  })
  return result
}

export function formatBytes(bytes: number) {
  const units = ["B", "KB", "MB", "GB", "TB", "PB"]
  let value = Math.max(0, bytes)
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  const digits = unit > 0 && value < 10 ? 1 : 0
  return `${value.toFixed(digits)} ${units[unit]}`
}

export function relativeTime(value: string, now = Date.now()) {
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return "未知"
  const elapsed = Math.max(0, now - timestamp)
  const minutes = Math.floor(elapsed / 60_000)
  if (minutes < 1) return "刚刚"
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  return `${Math.floor(hours / 24)} 天前`
}
