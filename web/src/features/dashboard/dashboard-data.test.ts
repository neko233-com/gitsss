import { afterEach, describe, expect, it, vi } from "vitest"

import {
  formatBytes,
  loadOperationalData,
  relativeTime,
} from "./dashboard-data"

afterEach(() => vi.unstubAllGlobals())

describe("dashboard presentation", () => {
  it("formats binary capacity without hiding small values", () => {
    expect(formatBytes(0)).toBe("0 B")
    expect(formatBytes(1536)).toBe("1.5 KB")
    expect(formatBytes(5 * 1024 ** 3)).toBe("5.0 GB")
  })

  it("renders bounded relative timestamps", () => {
    const now = Date.parse("2026-07-14T12:00:00Z")
    expect(relativeTime("2026-07-14T11:59:30Z", now)).toBe("刚刚")
    expect(relativeTime("2026-07-14T10:00:00Z", now)).toBe("2 小时前")
    expect(relativeTime("invalid", now)).toBe("未知")
  })

  it("reports an expired shared session without throwing away the shell", async () => {
    const request = vi.fn<() => Promise<Response>>(() =>
      Promise.resolve(
        new Response(JSON.stringify({ error: "invalid token" }), {
          headers: { "Content-Type": "application/json" },
          status: 401,
        })
      )
    )
    vi.stubGlobal("fetch", request)

    const result = await loadOperationalData("expired")

    expect(request).toHaveBeenCalledTimes(5)
    expect(result.authenticationFailed).toBe(true)
    expect(result.unavailable).toHaveLength(5)
  })
})
