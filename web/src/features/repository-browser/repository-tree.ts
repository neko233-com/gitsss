export interface RepositoryEntry {
  path: string
  object_id: string
  mode: string
  kind: "file" | "submodule"
  size: number
  binary_asset: boolean
}

export interface RepositoryNode {
  name: string
  path: string
  kind: "directory" | "file" | "submodule"
  size: number
  binaryAsset: boolean
  children: RepositoryNode[]
}

export function buildRepositoryTree(
  entries: RepositoryEntry[]
): RepositoryNode[] {
  const roots: RepositoryNode[] = []
  const directories = new Map<string, RepositoryNode>()

  for (const entry of entries) {
    const segments = entry.path.split("/")
    let parentPath = ""
    let siblings = roots

    for (const segment of segments.slice(0, -1)) {
      const directoryPath = parentPath ? `${parentPath}/${segment}` : segment
      let directory = directories.get(directoryPath)
      if (!directory) {
        directory = {
          name: segment,
          path: directoryPath,
          kind: "directory",
          size: 0,
          binaryAsset: false,
          children: [],
        }
        directories.set(directoryPath, directory)
        siblings.push(directory)
      }
      parentPath = directoryPath
      siblings = directory.children
    }

    siblings.push({
      name: segments.at(-1)!,
      path: entry.path,
      kind: entry.kind,
      size: entry.size,
      binaryAsset: entry.binary_asset,
      children: [],
    })
  }

  sortNodes(roots)
  return roots
}

function sortNodes(nodes: RepositoryNode[]) {
  nodes.sort((left, right) => {
    if (left.kind === "directory" && right.kind !== "directory") return -1
    if (left.kind !== "directory" && right.kind === "directory") return 1
    return left.name.localeCompare(right.name)
  })
  for (const node of nodes) sortNodes(node.children)
}
