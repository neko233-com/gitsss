import { useCallback, useEffect, useMemo, useState } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { AlertIcon, CheckIcon, SettingsIcon } from "@/components/ui/icons"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { apiDelete, apiMutation, apiRequest } from "@/lib/api"

interface RoleTemplate {
  id: string
  name: string
  purpose: string
  scopes: string[]
  default_hours: number
  maximum_hours: number
  warnings: string[]
}

interface TokenInfo {
  id: string
  role?: string
  scopes: string[]
  created_at: string
  expires_at: string
  revoked_at?: string
  current?: boolean
}

interface IssuedToken extends TokenInfo {
  token: string
}

const scopeNames: Record<string, string> = {
  agents: "Agent",
  auth: "授权",
  dashboard: "大盘",
  events: "事件",
  notifications: "通知",
  pipelines: "流水线",
  repositories: "仓库",
  runners: "Runner",
  storage: "存储",
  takeover: "接管",
}

export function accessAreas(scopes: string[]) {
  const areas = new Set(
    scopes.map((scope) => scopeNames[scope.split(":")[0]!] ?? scope)
  )
  return [...areas].join("、")
}

export function AccessCenter({
  onConnect,
  token,
}: {
  onConnect: () => void
  token: string
}) {
  const [roles, setRoles] = useState<RoleTemplate[]>([])
  const [tokens, setTokens] = useState<TokenInfo[]>([])
  const [roleID, setRoleID] = useState("")
  const [tokenID, setTokenID] = useState("")
  const [hours, setHours] = useState(1)
  const [issued, setIssued] = useState<IssuedToken>()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [copied, setCopied] = useState(false)

  const refresh = useCallback(async () => {
    if (!token) return
    setLoading(true)
    setError("")
    try {
      const [nextRoles, nextTokens] = await Promise.all([
        apiRequest<RoleTemplate[]>("/api/v1/auth/roles", token),
        apiRequest<TokenInfo[]>("/api/v1/auth/tokens", token),
      ])
      setRoles(nextRoles)
      setTokens(nextTokens)
      const first = nextRoles[0]
      if (first) {
        setRoleID((current) => current || first.id)
        setHours((current) => (current === 1 ? first.default_hours : current))
      }
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "授权信息读取失败")
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const selectedRole = useMemo(
    () => roles.find((role) => role.id === roleID),
    [roleID, roles]
  )

  const changeRole = (nextRoleID: string) => {
    const role = roles.find((candidate) => candidate.id === nextRoleID)
    setRoleID(nextRoleID)
    if (role) setHours(role.default_hours)
    setIssued(undefined)
    setCopied(false)
  }

  const issue = async () => {
    if (!selectedRole || !tokenID.trim()) return
    setLoading(true)
    setError("")
    try {
      const next = await apiMutation<IssuedToken>(
        "/api/v1/auth/tokens",
        token,
        {
          expires_in_hours: hours,
          id: tokenID.trim(),
          role: selectedRole.id,
        }
      )
      setIssued(next)
      setTokens((current) => [next, ...current])
      setTokenID("")
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Token 签发失败")
    } finally {
      setLoading(false)
    }
  }

  const revoke = async (id: string) => {
    setLoading(true)
    setError("")
    try {
      await apiDelete(`/api/v1/auth/tokens/${encodeURIComponent(id)}`, token)
      await refresh()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Token 撤销失败")
    } finally {
      setLoading(false)
    }
  }

  const copyIssued = async () => {
    if (!issued) return
    try {
      await navigator.clipboard.writeText(issued.token)
      setCopied(true)
    } catch {
      setError("浏览器拒绝复制，请手动选中仅此一次显示的 Token")
    }
  }

  if (!token) {
    return (
      <section className="mx-auto max-w-2xl pt-10">
        <SettingsIcon className="size-8 text-zinc-700" />
        <h2 className="mt-5 text-2xl font-semibold tracking-[-0.04em]">
          管理团队授权
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          先使用短期管理员 Token 连接，再按人员职责签发最小权限。
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
        <h2 className="text-xl font-semibold tracking-[-0.035em]">团队授权</h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          不再逐项勾选复杂权限：选择职责、确认范围、交付一次性显示的 Token。
        </p>
      </div>

      {error ? (
        <Alert className="mt-6" variant="destructive">
          <AlertIcon />
          <AlertTitle>授权操作未完成</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <div className="mt-8 grid gap-10 lg:grid-cols-[minmax(0,.9fr)_minmax(0,1.1fr)]">
        <div>
          <ol className="flex flex-col gap-7">
            <li>
              <p className="text-sm font-medium">1. 选择职责</p>
              <Select onValueChange={changeRole} value={roleID}>
                <SelectTrigger className="mt-3 w-full" disabled={loading}>
                  <SelectValue placeholder="选择角色" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {roles.map((role) => (
                      <SelectItem key={role.id} value={role.id}>
                        {role.name}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              {selectedRole ? (
                <div className="mt-3 border-y border-zinc-200 py-4">
                  <p className="text-sm text-zinc-700">
                    {selectedRole.purpose}
                  </p>
                  <p className="mt-2 text-xs leading-5 text-zinc-500">
                    可访问：{accessAreas(selectedRole.scopes)}
                  </p>
                  {selectedRole.warnings.map((warning) => (
                    <p
                      className="mt-2 text-xs leading-5 text-amber-700"
                      key={warning}
                    >
                      {warning}
                    </p>
                  ))}
                </div>
              ) : null}
            </li>
            <li>
              <p className="text-sm font-medium">2. 命名并设置有效期</p>
              <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_8rem]">
                <Input
                  aria-label="Token 名称"
                  onChange={(event) => setTokenID(event.target.value)}
                  placeholder="例如：lin-designer"
                  value={tokenID}
                />
                <Input
                  aria-label="有效小时数"
                  max={selectedRole?.maximum_hours ?? 1}
                  min={1}
                  onChange={(event) => setHours(Number(event.target.value))}
                  type="number"
                  value={hours}
                />
              </div>
              <p className="mt-2 text-xs text-zinc-500">
                最长 {selectedRole?.maximum_hours ?? 0}{" "}
                小时；到期后无需人工回收。
              </p>
            </li>
            <li>
              <p className="text-sm font-medium">3. 签发并安全交付</p>
              <Button
                className="mt-3"
                disabled={
                  loading ||
                  !tokenID.trim() ||
                  !selectedRole ||
                  hours < 1 ||
                  hours > (selectedRole?.maximum_hours ?? 0)
                }
                onClick={() => void issue()}
              >
                签发 Token
              </Button>
              {issued ? (
                <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50/40 p-4">
                  <p className="flex items-center gap-2 text-sm font-medium text-emerald-800">
                    <CheckIcon className="size-4" />
                    仅此一次显示
                  </p>
                  <output className="mt-3 block font-mono text-xs leading-5 break-all text-zinc-700">
                    {issued.token}
                  </output>
                  <Button
                    className="mt-3"
                    onClick={() => void copyIssued()}
                    size="sm"
                    variant="outline"
                  >
                    {copied ? "已复制" : "复制 Token"}
                  </Button>
                </div>
              ) : null}
            </li>
          </ol>
        </div>

        <div>
          <div className="flex items-center justify-between gap-4">
            <h3 className="text-base font-semibold">现有授权</h3>
            <span className="text-xs text-zinc-400">{tokens.length} 项</span>
          </div>
          <div className="mt-4 divide-y divide-zinc-200 border-y border-zinc-200">
            {tokens.map((item) => (
              <div className="flex items-center gap-4 py-4" key={item.id}>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-zinc-800">
                    {item.id}
                  </p>
                  <p className="mt-1 text-xs text-zinc-400">
                    {item.current ? "当前连接 · " : ""}
                    {item.role || "高级范围"} ·{" "}
                    {item.revoked_at
                      ? "已撤销"
                      : `到期 ${new Date(item.expires_at).toLocaleString()}`}
                  </p>
                </div>
                {!item.revoked_at && !item.current ? (
                  <Button
                    disabled={loading}
                    onClick={() => void revoke(item.id)}
                    size="sm"
                    variant="outline"
                  >
                    撤销
                  </Button>
                ) : null}
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
