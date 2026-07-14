import { describe, expect, it } from "vitest"

import { buildConfiguration } from "./notification-config"

describe("buildConfiguration", () => {
  it("normalizes email recipients without exposing unrelated webhook fields", () => {
    expect(
      buildConfiguration("email", {
        from: " gitsss@example.com ",
        password: "secret",
        recipients: "one@example.com, two@example.com ",
        secret: "",
        smtpHost: " smtp.example.com ",
        smtpPort: "587",
        username: "gitsss",
        webhookURL: "",
      })
    ).toEqual({
      from: "gitsss@example.com",
      password: "secret",
      recipients: ["one@example.com", "two@example.com"],
      smtp_host: "smtp.example.com",
      smtp_port: 587,
      username: "gitsss",
    })
  })
})
