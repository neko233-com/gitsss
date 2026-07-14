export type NotificationChannel = "email" | "feishu" | "wecom"

export interface NotificationForm {
  webhookURL: string
  secret: string
  smtpHost: string
  smtpPort: string
  username: string
  password: string
  from: string
  recipients: string
}

export function buildConfiguration(
  channel: NotificationChannel,
  form: NotificationForm
) {
  if (channel === "email") {
    return {
      from: form.from.trim(),
      password: form.password,
      recipients: form.recipients
        .split(",")
        .map((recipient) => recipient.trim())
        .filter(Boolean),
      smtp_host: form.smtpHost.trim(),
      smtp_port: Number(form.smtpPort),
      username: form.username.trim(),
    }
  }
  return {
    secret: channel === "feishu" ? form.secret : undefined,
    webhook_url: form.webhookURL.trim(),
  }
}
