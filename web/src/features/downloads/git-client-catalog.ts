export interface GitClientSource {
  kind: "official" | "mirror"
  label: string
  url: string
  architecture?: string
  sha256?: string
  bytes?: number
  command?: string
}

export interface GitClientCatalog {
  version: 1
  verified_at: string
  trust: "embedded-release-catalog"
  platforms: Array<{
    id: "windows" | "macos"
    label: string
    version: string
    sources: GitClientSource[]
  }>
}

function record(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("下载目录格式无效")
  }
  return value as Record<string, unknown>
}

function text(value: unknown, field: string) {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`下载目录字段 ${field} 无效`)
  }
  return value
}

function optionalText(value: unknown, field: string) {
  return value === undefined ? undefined : text(value, field)
}

function secureURL(value: unknown) {
  const raw = text(value, "url")
  let parsed: URL
  try {
    parsed = new URL(raw)
  } catch {
    throw new Error("下载目录包含无效链接")
  }
  if (parsed.protocol !== "https:") {
    throw new Error("下载目录仅允许 HTTPS 链接")
  }
  return raw
}

function parseSource(value: unknown): GitClientSource {
  const source = record(value)
  if (source.kind !== "official" && source.kind !== "mirror") {
    throw new Error("下载目录包含未知来源")
  }
  const sha256 = optionalText(source.sha256, "sha256")
  if (sha256 && !/^[a-f0-9]{64}$/.test(sha256)) {
    throw new Error("下载目录包含无效 SHA-256")
  }
  if (
    source.bytes !== undefined &&
    (!Number.isSafeInteger(source.bytes) || (source.bytes as number) <= 0)
  ) {
    throw new Error("下载目录包含无效文件大小")
  }
  return {
    kind: source.kind,
    label: text(source.label, "label"),
    url: secureURL(source.url),
    architecture: optionalText(source.architecture, "architecture"),
    sha256,
    bytes: source.bytes as number | undefined,
    command: optionalText(source.command, "command"),
  }
}

export function parseGitClientCatalog(value: unknown): GitClientCatalog {
  const catalog = record(value)
  if (catalog.version !== 1) throw new Error("不支持此下载目录版本")
  if (catalog.trust !== "embedded-release-catalog") {
    throw new Error("下载目录信任模型无效")
  }
  const verifiedAt = text(catalog.verified_at, "verified_at")
  if (!Number.isFinite(Date.parse(verifiedAt))) {
    throw new Error("下载目录校验时间无效")
  }
  if (!Array.isArray(catalog.platforms) || catalog.platforms.length === 0) {
    throw new Error("下载目录未包含平台")
  }
  const seen = new Set<string>()
  const platforms = catalog.platforms.map((value) => {
    const platform = record(value)
    if (platform.id !== "windows" && platform.id !== "macos") {
      throw new Error("下载目录包含未知平台")
    }
    const id: "windows" | "macos" = platform.id
    if (seen.has(id)) throw new Error("下载目录包含重复平台")
    seen.add(id)
    if (!Array.isArray(platform.sources) || platform.sources.length === 0) {
      throw new Error("下载目录平台未包含来源")
    }
    return {
      id,
      label: text(platform.label, "label"),
      version: text(platform.version, "version"),
      sources: platform.sources.map(parseSource),
    }
  })
  return {
    version: 1,
    verified_at: verifiedAt,
    trust: "embedded-release-catalog",
    platforms,
  }
}
