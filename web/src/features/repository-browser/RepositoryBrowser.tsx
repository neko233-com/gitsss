import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react"

import { Button } from "@/components/ui/button"
import {
  ChevronIcon,
  FileIcon,
  FolderIcon,
  RepositoryIcon,
  SearchIcon,
} from "@/components/ui/icons"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { apiRequest } from "@/lib/api"
import { readSelectedRepository, writeSelectedRepository } from "@/lib/session"

import {
  buildRepositoryTree,
  type RepositoryEntry,
  type RepositoryNode,
} from "./repository-tree"

interface RepositorySummary {
  id: string
  name: string
  default_branch: string
}

interface RepositoryTreeResponse {
  repository: RepositorySummary
  ref: string
  commit: string
  entries: RepositoryEntry[]
}

interface RepositoryFile {
  path: string
  object_id: string
  size: number
  binary: boolean
  binary_asset: boolean
  preview_available: boolean
  content?: string
}

interface SearchMatch {
  path: string
  line: number
  content: string
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function TreeNodes({
  expandedPaths,
  nodes,
  onSelect,
  onToggle,
  selectedPath,
  level = 0,
}: {
  expandedPaths: Set<string>
  nodes: RepositoryNode[]
  onSelect: (path: string) => void
  onToggle: (path: string) => void
  selectedPath: string
  level?: number
}) {
  return (
    <ul>
      {nodes.map((node) =>
        node.kind === "directory" ? (
          <li key={node.path}>
            <button
              aria-expanded={expandedPaths.has(node.path)}
              className="flex h-8 w-full items-center gap-2 rounded-md pr-2 text-left text-xs font-medium text-zinc-700 hover:bg-zinc-100"
              onClick={() => onToggle(node.path)}
              style={{ paddingLeft: `${level * 14 + 8}px` }}
              type="button"
            >
              <ChevronIcon
                className={`size-3.5 text-zinc-400 transition-transform ${expandedPaths.has(node.path) ? "rotate-90" : ""}`}
              />
              <FolderIcon className="size-4 text-zinc-500" />
              <span className="truncate">{node.name}</span>
            </button>
            {expandedPaths.has(node.path) ? (
              <TreeNodes
                expandedPaths={expandedPaths}
                level={level + 1}
                nodes={node.children}
                onSelect={onSelect}
                onToggle={onToggle}
                selectedPath={selectedPath}
              />
            ) : null}
          </li>
        ) : (
          <li key={node.path}>
            <button
              className={`flex h-8 w-full items-center gap-2 rounded-md pr-2 text-left text-xs transition-colors ${selectedPath === node.path ? "bg-blue-50 font-medium text-blue-700" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950"}`}
              onClick={() => onSelect(node.path)}
              style={{ paddingLeft: `${level * 14 + 25}px` }}
              type="button"
            >
              <FileIcon className="size-4 shrink-0" />
              <span className="truncate">{node.name}</span>
            </button>
          </li>
        )
      )}
    </ul>
  )
}

export function RepositoryBrowser({
  onConnect,
  token,
}: {
  onConnect: () => void
  token: string
}) {
  const [repositories, setRepositories] = useState<RepositorySummary[]>([])
  const [repositoryID, setRepositoryID] = useState("")
  const [treeResponse, setTreeResponse] =
    useState<RepositoryTreeResponse | null>(null)
  const [file, setFile] = useState<RepositoryFile | null>(null)
  const [selectedPath, setSelectedPath] = useState("")
  const [query, setQuery] = useState("")
  const [matches, setMatches] = useState<SearchMatch[] | null>(null)
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(
    () => new Set()
  )
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const tree = useMemo(
    () => buildRepositoryTree(treeResponse?.entries ?? []),
    [treeResponse?.entries]
  )

  const loadFile = useCallback(
    async (
      path: string,
      activeToken = token,
      activeRepository = repositoryID
    ) => {
      if (!activeToken || !activeRepository) return
      setLoading(true)
      setError("")
      try {
        const result = await apiRequest<RepositoryFile>(
          `/api/v1/repositories/${encodeURIComponent(activeRepository)}/files?ref=HEAD&path=${encodeURIComponent(path)}`,
          activeToken
        )
        setSelectedPath(path)
        setFile(result)
        setMatches(null)
      } catch (requestError) {
        setError(
          requestError instanceof Error ? requestError.message : "文件读取失败"
        )
      } finally {
        setLoading(false)
      }
    },
    [repositoryID, token]
  )

  const loadRepositoryContent = useCallback(
    async (
      activeRepository: string,
      activeToken: string,
      signal?: AbortSignal
    ) => {
      writeSelectedRepository(activeRepository)
      setRepositoryID(activeRepository)
      setTreeResponse(null)
      setSelectedPath("")
      setFile(null)
      setMatches(null)
      const response = await apiRequest<RepositoryTreeResponse>(
        `/api/v1/repositories/${encodeURIComponent(activeRepository)}/tree?ref=HEAD`,
        activeToken,
        signal
      )
      setTreeResponse(response)
      setExpandedPaths(
        new Set(
          response.entries.flatMap((entry) => {
            const separator = entry.path.indexOf("/")
            return separator > 0 ? [entry.path.slice(0, separator)] : []
          })
        )
      )
      const firstPath =
        response.entries.find((entry) => entry.path === "README.md")?.path ??
        response.entries.find((entry) => !entry.binary_asset)?.path
      if (!firstPath) return
      const firstFile = await apiRequest<RepositoryFile>(
        `/api/v1/repositories/${encodeURIComponent(activeRepository)}/files?ref=HEAD&path=${encodeURIComponent(firstPath)}`,
        activeToken,
        signal
      )
      setSelectedPath(firstPath)
      setFile(firstFile)
    },
    []
  )

  useEffect(() => {
    if (!token) return
    const controller = new AbortController()
    setLoading(true)
    setError("")

    async function loadRepository() {
      try {
        const availableRepositories = await apiRequest<RepositorySummary[]>(
          "/api/v1/repositories",
          token,
          controller.signal
        )
        const preferredID = readSelectedRepository()
        const repository =
          availableRepositories.find((item) => item.id === preferredID) ??
          availableRepositories[0]
        if (!repository) throw new Error("当前账号没有可访问的仓库")
        setRepositories(availableRepositories)
        await loadRepositoryContent(repository.id, token, controller.signal)
      } catch (requestError) {
        if (controller.signal.aborted) return
        setError(
          requestError instanceof Error ? requestError.message : "仓库加载失败"
        )
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }

    void loadRepository()
    return () => controller.abort()
  }, [loadRepositoryContent, token])

  const changeRepository = async (nextRepositoryID: string) => {
    setLoading(true)
    setError("")
    try {
      await loadRepositoryContent(nextRepositoryID, token)
    } catch (requestError) {
      setError(
        requestError instanceof Error ? requestError.message : "仓库加载失败"
      )
    } finally {
      setLoading(false)
    }
  }

  const search = async (event: FormEvent) => {
    event.preventDefault()
    const normalizedQuery = query.trim()
    if (!normalizedQuery || !token || !repositoryID) return
    setLoading(true)
    setError("")
    try {
      const result = await apiRequest<SearchMatch[]>(
        `/api/v1/repositories/${encodeURIComponent(repositoryID)}/search?ref=HEAD&q=${encodeURIComponent(normalizedQuery)}&limit=40`,
        token
      )
      setMatches(result)
    } catch (requestError) {
      setError(
        requestError instanceof Error ? requestError.message : "搜索失败"
      )
    } finally {
      setLoading(false)
    }
  }

  const toggleDirectory = (path: string) => {
    setExpandedPaths((current) => {
      const next = new Set(current)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  if (!token) {
    return (
      <section className="mx-auto max-w-xl pt-10">
        <RepositoryIcon className="size-8 text-zinc-700" />
        <h2 className="mt-5 text-2xl font-semibold tracking-[-0.04em]">
          连接仓库服务
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          输入管理员为你签发的访问 Token。Token 仅保存在当前浏览器标签中。
        </p>
        <Button className="mt-7" onClick={onConnect}>
          连接
        </Button>
      </section>
    )
  }

  return (
    <section className="pt-7">
      <div className="flex flex-col gap-4 border-b border-zinc-200 pb-5 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <RepositoryIcon className="size-5 text-zinc-500" />
            <h2 className="truncate text-xl font-semibold tracking-[-0.035em]">
              {treeResponse?.repository.name ?? "仓库"}
            </h2>
            <span className="text-xs text-zinc-400">
              {treeResponse?.repository.default_branch}
            </span>
          </div>
          <p className="mt-1 truncate font-mono text-xs text-zinc-400">
            {treeResponse?.commit.slice(0, 12)}
          </p>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">
          <Select
            disabled={loading}
            onValueChange={(value) => void changeRepository(value)}
            value={repositoryID}
          >
            <SelectTrigger aria-label="切换仓库" className="h-9 sm:max-w-48">
              <SelectValue placeholder="选择仓库" />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {repositories.map((repository) => (
                  <SelectItem key={repository.id} value={repository.id}>
                    {repository.name}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <form className="relative min-w-0 sm:w-80" onSubmit={search}>
            <SearchIcon className="pointer-events-none absolute top-2.5 left-3 size-4 text-zinc-400" />
            <Input
              aria-label="搜索当前仓库"
              className="h-9 pr-16 pl-9 text-sm"
              onChange={(event) => setQuery(event.target.value)}
              placeholder="搜索路径或内容"
              value={query}
            />
            <button
              className="absolute top-1 right-1 h-7 rounded-md px-2 text-xs font-medium text-zinc-500 hover:bg-zinc-100"
              type="submit"
            >
              搜索
            </button>
          </form>
          <Button onClick={onConnect} size="sm" variant="outline">
            更换连接
          </Button>
        </div>
      </div>

      {error ? (
        <p
          className="border-b border-red-100 bg-red-50 px-3 py-2 text-sm text-red-700"
          role="alert"
        >
          {error}
        </p>
      ) : null}

      <div className="grid min-h-[34rem] md:grid-cols-[17rem_minmax(0,1fr)]">
        <aside className="max-h-72 overflow-auto border-b border-zinc-200 py-4 pr-3 md:max-h-[calc(100svh-14rem)] md:border-r md:border-b-0">
          <div className="flex items-center justify-between px-2 pb-3 text-xs text-zinc-400">
            <span>文件</span>
            <span>{treeResponse?.entries.length ?? 0}</span>
          </div>
          <TreeNodes
            expandedPaths={expandedPaths}
            nodes={tree}
            onSelect={(path) => void loadFile(path)}
            onToggle={toggleDirectory}
            selectedPath={selectedPath}
          />
        </aside>

        <div className="min-w-0 py-5 md:pl-7">
          {loading && !file ? (
            <p className="text-sm text-zinc-500">正在读取仓库…</p>
          ) : matches ? (
            <div>
              <div className="flex items-center justify-between gap-4">
                <h3 className="text-sm font-semibold">搜索结果</h3>
                <span className="text-xs text-zinc-400">
                  {matches.length} 项
                </span>
              </div>
              <div className="mt-4 divide-y divide-zinc-200 border-y border-zinc-200">
                {matches.length ? (
                  matches.map((match) => (
                    <button
                      className="block w-full px-1 py-4 text-left hover:bg-zinc-50"
                      key={`${match.path}:${match.line}`}
                      onClick={() => void loadFile(match.path)}
                      type="button"
                    >
                      <span className="text-xs font-medium text-blue-700">
                        {match.path}:{match.line}
                      </span>
                      <span className="mt-1 block truncate font-mono text-xs text-zinc-600">
                        {match.content}
                      </span>
                    </button>
                  ))
                ) : (
                  <p className="py-8 text-sm text-zinc-500">没有匹配内容。</p>
                )}
              </div>
            </div>
          ) : file ? (
            <div>
              <div className="flex items-center justify-between gap-4 border-b border-zinc-200 pb-3">
                <h3 className="truncate font-mono text-sm font-medium">
                  {file.path}
                </h3>
                <span className="shrink-0 text-xs text-zinc-400">
                  {formatBytes(file.size)}
                </span>
              </div>
              {file.preview_available ? (
                <pre className="mt-4 max-h-[calc(100svh-18rem)] overflow-auto font-mono text-[13px] leading-6 text-zinc-700">
                  <code>{file.content}</code>
                </pre>
              ) : (
                <div className="py-16 text-center">
                  <FileIcon className="mx-auto size-8 text-zinc-400" />
                  <p className="mt-4 text-sm font-medium">
                    {file.binary_asset
                      ? "大型二进制资产"
                      : "此文件无法文本预览"}
                  </p>
                  <p className="mt-1 text-xs text-zinc-500">
                    已保留对象 ID、大小和类型元数据，不会将整个文件载入浏览器。
                  </p>
                </div>
              )}
            </div>
          ) : (
            <p className="text-sm text-zinc-500">从左侧选择一个文件。</p>
          )}
        </div>
      </div>

      <p className="sr-only" aria-live="polite">
        {loading ? "正在加载" : `${repositories.length} 个仓库可用`}
      </p>
    </section>
  )
}
