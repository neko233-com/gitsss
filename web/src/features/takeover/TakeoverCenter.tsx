import { useState } from "react"

import { Button } from "@/components/ui/button"
import { CheckIcon, TakeoverIcon } from "@/components/ui/icons"
import { Input } from "@/components/ui/input"
import { apiRequest } from "@/lib/api"

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
  gitea_version?: string
  public_origin?: string
  database: { type: string; name?: string; location: string; tls_mode?: string }
  assets: Asset[]
  actions_enabled: boolean
  secret_field_count: number
  findings: Finding[]
  assessed_at: string
}

const assetNames: Record<string, string> = {
  repositories: "Git 仓库",
  lfs: "LFS",
  attachments: "附件",
  packages: "软件包",
  "actions-log": "流水线日志",
  "actions-artifacts": "流水线制品",
  database: "数据库文件",
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  return `${(bytes / 1024 ** 3).toFixed(1)} GB`
}

export function TakeoverCenter() {
  const [token, setToken] = useState(
    () => window.sessionStorage.getItem("gitsss-api-token") ?? ""
  )
  const [reports, setReports] = useState<Assessment[]>([])
  const [selectedID, setSelectedID] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [planOpen, setPlanOpen] = useState(false)

  const selected =
    reports.find((report) => report.id === selectedID) ?? reports[0]
  const loadReports = async () => {
    setLoading(true)
    setError("")
    window.sessionStorage.setItem("gitsss-api-token", token)
    try {
      const nextReports = await apiRequest<Assessment[]>(
        "/api/v1/takeover/assessments",
        token
      )
      setReports(nextReports)
      setSelectedID(nextReports[0]?.id ?? "")
      setPlanOpen(false)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法读取评估报告")
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

  return (
    <section className="mx-auto max-w-5xl pt-8">
      <div className="max-w-2xl">
        <h2 className="text-xl font-semibold tracking-[-0.035em]">
          接管旧服务器
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          Agent 只读扫描配置和资产。确认报告之前，gitsss
          不连接数据库，也不修改原服务器。
        </p>
      </div>

      <ol className="mt-8 grid grid-cols-3 border-y border-zinc-200 py-4 text-xs text-zinc-500">
        {["运行扫描", "核对报告", "生成计划"].map((step, index) => (
          <li
            className={`flex items-center gap-2 ${index === 0 || (index === 1 && reports.length > 0) || (index === 2 && planOpen) ? "font-medium text-zinc-950" : ""}`}
            key={step}
          >
            <span
              className={`flex size-5 items-center justify-center rounded-full border ${index === 0 || (index === 1 && reports.length > 0) || (index === 2 && planOpen) ? "border-zinc-950 bg-zinc-950 text-white" : "border-zinc-300"}`}
            >
              {index + 1}
            </span>
            {step}
          </li>
        ))}
      </ol>

      <div className="mt-8 grid min-w-0 gap-8 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold">1. 在旧服务器运行</h3>
          <pre className="mt-3 max-w-full overflow-x-auto border-y border-zinc-200 py-4 text-xs leading-5 text-zinc-600">
            <code>
              gitsss-agent assess --agent-id old-gitea --config
              /etc/gitea/app.ini --server … --token …
            </code>
          </pre>
          <label
            className="mt-6 grid gap-2 text-sm font-medium text-zinc-800"
            htmlFor="takeover-token"
          >
            管理员 API Token
            <Input
              autoComplete="off"
              id="takeover-token"
              onChange={(event) => setToken(event.target.value)}
              placeholder="仅保存在当前标签页"
              type="password"
              value={token}
            />
          </label>
          <Button
            className="mt-4"
            disabled={loading || token.length === 0}
            onClick={loadReports}
          >
            {loading ? "正在读取…" : "读取评估报告"}
          </Button>
          {error ? (
            <p className="mt-3 text-sm text-red-600" role="alert">
              {error}
            </p>
          ) : null}
        </div>

        {selected ? (
          <div className="min-w-0">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div>
                <p className="text-xs text-zinc-500">只读评估</p>
                <h3 className="mt-1 text-lg font-semibold tracking-[-0.03em]">
                  {selected.public_origin || selected.agent_id}
                </h3>
                <p className="mt-1 text-xs text-zinc-400">
                  {selected.config_path}
                </p>
              </div>
              {reports.length > 1 ? (
                <select
                  className="h-9 rounded-lg border border-zinc-200 bg-white px-3 text-sm"
                  onChange={(event) => setSelectedID(event.target.value)}
                  value={selected.id}
                >
                  {reports.map((report) => (
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
              <p className="text-sm font-medium">安全确认</p>
              <p className="mt-2 text-xs leading-5 text-zinc-500">
                已脱敏 {selected.secret_field_count}{" "}
                个敏感字段。当前报告仅用于生成备份预览，不包含数据库密码、OAuth
                密钥或 SMTP 凭据。
              </p>
              {selected.findings.map((finding) => (
                <p
                  className="mt-3 text-xs leading-5 text-zinc-600"
                  key={finding.code}
                >
                  • {finding.message}
                </p>
              ))}
              <Button
                className="mt-5"
                disabled={Boolean(blockers)}
                onClick={() => setPlanOpen(true)}
                variant="outline"
              >
                {blockers ? "先处理阻断项" : "下一步：生成备份预览"}
              </Button>
              {planOpen ? (
                <div className="mt-5 border-l-2 border-emerald-500 pl-4">
                  <p className="text-sm font-medium text-emerald-800">
                    备份预览已生成
                  </p>
                  <p className="mt-1 text-xs leading-5 text-zinc-500">
                    下一阶段将按数据库、Git、LFS、附件和流水线制品顺序生成校验清单；原服务器仍未发生任何写入。
                  </p>
                </div>
              ) : null}
            </div>
          </div>
        ) : (
          <div className="flex min-h-80 flex-col items-center justify-center border-y border-zinc-200 text-center">
            <TakeoverIcon className="size-7 text-zinc-400" />
            <p className="mt-4 text-sm font-medium">等待第一份只读报告</p>
            <p className="mt-1 max-w-sm text-xs leading-5 text-zinc-400">
              在旧服务器运行命令后，使用同一个 Token 读取结果。
            </p>
          </div>
        )}
      </div>
    </section>
  )
}
