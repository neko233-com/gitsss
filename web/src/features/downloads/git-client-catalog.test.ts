import { describe, expect, it } from "vitest"

import { parseGitClientCatalog } from "./git-client-catalog"

const catalog = {
  version: 1,
  verified_at: "2026-07-15T00:00:00Z",
  trust: "embedded-release-catalog",
  platforms: [
    {
      id: "windows",
      label: "Windows",
      version: "2.55.0.windows.2",
      sources: [
        {
          kind: "official",
          label: "官方 x64 安装包",
          url: "https://example.com/git.exe",
          sha256: "a".repeat(64),
          bytes: 1024,
        },
      ],
    },
  ],
}

describe("Git client catalog", () => {
  it("accepts a versioned HTTPS catalog", () => {
    expect(parseGitClientCatalog(catalog).platforms[0]!.version).toBe(
      "2.55.0.windows.2"
    )
  })

  it("rejects untrusted links before rendering them", () => {
    expect(() =>
      parseGitClientCatalog({
        ...catalog,
        platforms: [
          {
            ...catalog.platforms[0],
            sources: [
              {
                ...catalog.platforms[0]!.sources[0],
                url: "javascript:alert(1)",
              },
            ],
          },
        ],
      })
    ).toThrow("仅允许 HTTPS")
  })

  it("rejects malformed checksums", () => {
    expect(() =>
      parseGitClientCatalog({
        ...catalog,
        platforms: [
          {
            ...catalog.platforms[0],
            sources: [
              { ...catalog.platforms[0]!.sources[0], sha256: "unknown" },
            ],
          },
        ],
      })
    ).toThrow("SHA-256")
  })
})
