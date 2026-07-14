import { useCallback, useEffect, useState, type FormEvent } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { AlertIcon, CheckIcon, NotificationIcon } from "@/components/ui/icons"
import { Input } from "@/components/ui/input"
import { apiMutation, apiRequest } from "@/lib/api"

import {
  buildConfiguration,
  type NotificationChannel,
  type NotificationForm,
} from "./notification-config"

const channels: Array<{
  id: NotificationChannel
  name: string
  description: string
}> = [
  { id: "email", name: "邮件", description: "SMTP / STARTTLS" },
  { id: "feishu", name: "飞书", description: "群自定义机器人" },
  { id: "wecom", name: "企业微信", description: "群机器人 Webhook" },
]

const emptyForm: NotificationForm = {
  from: "",
  password: "",
  recipients: "",
  secret: "",
  smtpHost: "",
  smtpPort: "587",
  username: "",
  webhookURL: "",
}

interface DeliveryResult {
  status: "delivered" | "retrying" | "dead"
  attempts: number
  last_error?: string
}

interface ChannelState {
  channel: NotificationChannel
  configured: boolean
  destination?: string
}

interface OutboxMessage {
  id: string
  channel: NotificationChannel
  subject: string
  status: "pending" | "delivering" | "delivered" | "retrying" | "dead"
  attempts: number
  last_error?: string
  created_at: string
}

function Field({
  label,
  name,
  onChange,
  placeholder,
  required = true,
  type = "text",
  value,
}: {
  label: string
  name: keyof NotificationForm
  onChange: (name: keyof NotificationForm, value: string) => void
  placeholder?: string
  required?: boolean
  type?: string
  value: string
}) {
  return (
    <label className="grid gap-2 text-sm font-medium text-zinc-800">
      {label}
      <Input
        autoComplete={type === "password" ? "new-password" : "off"}
        name={name}
        onChange={(event) => onChange(name, event.target.value)}
        placeholder={placeholder}
        required={required}
        type={type}
        value={value}
      />
    </label>
  )
}

export function NotificationCenter({
  onConnect,
  token,
}: {
  onConnect: () => void
  token: string
}) {
  const [channel, setChannel] = useState<NotificationChannel | null>(null)
  const [form, setForm] = useState<NotificationForm>(emptyForm)
  const [channelStates, setChannelStates] = useState<ChannelState[]>([])
  const [outbox, setOutbox] = useState<OutboxMessage[]>([])
  const [status, setStatus] = useState<"idle" | "submitting" | "done">("idle")
  const [delivery, setDelivery] = useState<DeliveryResult | null>(null)
  const [error, setError] = useState("")

  const refresh = useCallback(async () => {
    if (!token) return
    try {
      const [states, messages] = await Promise.all([
        apiRequest<ChannelState[]>("/api/v1/notifications/channels", token),
        apiRequest<OutboxMessage[]>("/api/v1/notifications/outbox", token),
      ])
      setChannelStates(states)
      setOutbox(messages)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "通知状态读取失败")
    }
  }, [token])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const selected = channels.find((item) => item.id === channel)
  const updateForm = (name: keyof NotificationForm, value: string) => {
    setForm((current) => ({ ...current, [name]: value }))
  }

  const selectChannel = (nextChannel: NotificationChannel) => {
    setChannel(nextChannel)
    setStatus("idle")
    setDelivery(null)
    setError("")
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!channel) return
    setStatus("submitting")
    setError("")
    try {
      await apiMutation(
        `/api/v1/notifications/channels/${channel}`,
        token,
        buildConfiguration(channel, form)
      )
      const result = await apiMutation<DeliveryResult>(
        `/api/v1/notifications/channels/${channel}/test`,
        token,
        {}
      )
      setDelivery(result)
      setStatus("done")
      setForm((current) => ({ ...current, password: "", secret: "" }))
      await refresh()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "连接测试失败")
      setStatus("idle")
    }
  }

  if (!token) {
    return (
      <section className="mx-auto max-w-2xl pt-10">
        <NotificationIcon className="size-8 text-zinc-700" />
        <h2 className="mt-5 text-2xl font-semibold tracking-[-0.04em]">
          连接团队通知
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          连接服务器后配置飞书、企业微信或安全 SMTP，并查看自动重试结果。
        </p>
        <Button className="mt-6" onClick={onConnect}>
          连接服务器
        </Button>
      </section>
    )
  }

  return (
    <section className="mx-auto max-w-4xl pt-8">
      <div className="max-w-2xl">
        <h2 className="text-xl font-semibold tracking-[-0.035em]">通知连接</h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          依次选择通道、填写连接、发送测试。密钥不会在保存后再次显示。
        </p>
      </div>

      {error ? (
        <Alert className="mt-6" variant="destructive">
          <AlertIcon />
          <AlertTitle>通知操作未完成</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <ol
        aria-label="通知配置步骤"
        className="mt-8 grid grid-cols-3 border-y border-zinc-200 py-4 text-xs text-zinc-500"
      >
        {["选择通道", "填写连接", "测试完成"].map((step, index) => {
          const active = channel
            ? status === "done"
              ? index <= 2
              : index <= 1
            : index === 0
          return (
            <li
              className={`flex items-center gap-2 ${active ? "font-medium text-zinc-950" : ""}`}
              key={step}
            >
              <span
                className={`flex size-5 items-center justify-center rounded-full border ${active ? "border-zinc-950 bg-zinc-950 text-white" : "border-zinc-300"}`}
              >
                {index + 1}
              </span>
              {step}
            </li>
          )
        })}
      </ol>

      <div className="mt-8 grid gap-8 lg:grid-cols-[15rem_minmax(0,1fr)]">
        <div className="divide-y divide-zinc-200 border-y border-zinc-200">
          {channels.map((item) => (
            <button
              className={`flex w-full items-center gap-3 px-2 py-4 text-left transition-colors ${channel === item.id ? "text-blue-700" : "text-zinc-700 hover:text-zinc-950"}`}
              key={item.id}
              onClick={() => selectChannel(item.id)}
              type="button"
            >
              <NotificationIcon className="size-5 shrink-0" />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium">{item.name}</span>
                <span className="mt-0.5 block text-xs text-zinc-400">
                  {item.description}
                </span>
              </span>
              {channelStates.find((state) => state.channel === item.id)
                ?.configured ? (
                <span className="text-xs text-emerald-700">已连接</span>
              ) : channel === item.id ? (
                <CheckIcon className="size-4" />
              ) : null}
            </button>
          ))}
        </div>

        {selected ? (
          status === "done" && delivery ? (
            <div className="flex min-h-72 flex-col items-start justify-center border-y border-zinc-200 py-10">
              <span className="flex size-10 items-center justify-center rounded-full bg-emerald-50 text-emerald-700">
                <CheckIcon className="size-5" />
              </span>
              <h3 className="mt-5 text-lg font-semibold tracking-[-0.03em]">
                {selected.name}连接成功
              </h3>
              <p className="mt-2 text-sm text-zinc-500">
                测试消息已投递，共尝试 {delivery.attempts} 次。
              </p>
              <Button
                className="mt-6"
                onClick={() => {
                  setStatus("idle")
                  setDelivery(null)
                }}
                variant="outline"
              >
                重新测试
              </Button>
            </div>
          ) : (
            <form className="grid gap-5" onSubmit={submit}>
              <div>
                <h3 className="text-base font-semibold">连接{selected.name}</h3>
                <p className="mt-1 text-xs leading-5 text-zinc-500">
                  连接保存后会立即发送一条 gitsss 测试消息。
                </p>
              </div>
              {channel === "email" ? (
                <>
                  <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_7rem]">
                    <Field
                      label="SMTP 主机"
                      name="smtpHost"
                      onChange={updateForm}
                      placeholder="smtp.example.com"
                      value={form.smtpHost}
                    />
                    <Field
                      label="端口"
                      name="smtpPort"
                      onChange={updateForm}
                      value={form.smtpPort}
                    />
                  </div>
                  <Field
                    label="发件地址"
                    name="from"
                    onChange={updateForm}
                    placeholder="gitsss@example.com"
                    type="email"
                    value={form.from}
                  />
                  <Field
                    label="收件地址"
                    name="recipients"
                    onChange={updateForm}
                    placeholder="ops@example.com, owner@example.com"
                    value={form.recipients}
                  />
                  <div className="grid gap-4 sm:grid-cols-2">
                    <Field
                      label="SMTP 用户名"
                      name="username"
                      onChange={updateForm}
                      required={false}
                      value={form.username}
                    />
                    <Field
                      label="SMTP 密码"
                      name="password"
                      onChange={updateForm}
                      required={false}
                      type="password"
                      value={form.password}
                    />
                  </div>
                </>
              ) : (
                <>
                  <Field
                    label={`${selected.name} Webhook`}
                    name="webhookURL"
                    onChange={updateForm}
                    placeholder={
                      channel === "feishu"
                        ? "https://open.feishu.cn/open-apis/bot/v2/hook/…"
                        : "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…"
                    }
                    type="url"
                    value={form.webhookURL}
                  />
                  {channel === "feishu" ? (
                    <Field
                      label="签名密钥"
                      name="secret"
                      onChange={updateForm}
                      placeholder="建议在飞书开启签名校验"
                      required={false}
                      type="password"
                      value={form.secret}
                    />
                  ) : null}
                </>
              )}
              <div className="flex items-center gap-3 border-t border-zinc-200 pt-5">
                <Button
                  disabled={status === "submitting" || token.length === 0}
                  type="submit"
                >
                  {status === "submitting" ? "正在测试…" : "保存并发送测试"}
                </Button>
                <span className="text-xs text-zinc-400">
                  失败会自动进入重试队列
                </span>
              </div>
            </form>
          )
        ) : (
          <div className="flex min-h-72 items-center justify-center border-y border-zinc-200 text-sm text-zinc-400">
            从左侧选择一个通知通道
          </div>
        )}
      </div>

      <section className="mt-10">
        <div className="flex items-center justify-between gap-4">
          <h3 className="text-base font-semibold">最近投递</h3>
          <span className="text-xs text-zinc-400">{outbox.length} 条</span>
        </div>
        <div className="mt-4 divide-y divide-zinc-200 border-y border-zinc-200">
          {outbox.slice(0, 12).map((message) => (
            <div className="flex items-center gap-4 py-4" key={message.id}>
              <NotificationIcon className="size-4 shrink-0 text-zinc-400" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-zinc-800">
                  {message.subject}
                </p>
                <p className="mt-1 text-xs text-zinc-400">
                  {message.channel} · 尝试 {message.attempts} 次
                </p>
              </div>
              <span
                className={`text-xs font-medium ${message.status === "delivered" ? "text-emerald-700" : message.status === "dead" ? "text-red-600" : "text-amber-700"}`}
              >
                {message.status}
              </span>
            </div>
          ))}
          {outbox.length === 0 ? (
            <p className="py-8 text-sm text-zinc-500">还没有通知投递记录。</p>
          ) : null}
        </div>
      </section>
    </section>
  )
}
