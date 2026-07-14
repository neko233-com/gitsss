import { describe, expect, it } from "vitest"

import { buildRepositoryTree } from "./repository-tree"

describe("buildRepositoryTree", () => {
  it("groups paths and sorts directories before files", () => {
    const tree = buildRepositoryTree([
      {
        path: "README.md",
        object_id: "a",
        mode: "100644",
        kind: "file",
        size: 12,
        binary_asset: false,
      },
      {
        path: "assets/scene.fbx",
        object_id: "b",
        mode: "100644",
        kind: "file",
        size: 42,
        binary_asset: true,
      },
    ])

    expect(tree.map((node) => node.name)).toEqual(["assets", "README.md"])
    expect(tree[0]!.children[0]).toMatchObject({
      path: "assets/scene.fbx",
      binaryAsset: true,
    })
  })
})
