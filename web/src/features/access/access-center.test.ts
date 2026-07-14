import { describe, expect, it } from "vitest"

import { accessAreas } from "./AccessCenter"

describe("access role summary", () => {
  it("groups low-level scopes into understandable product areas", () => {
    expect(
      accessAreas([
        "repositories:read",
        "repositories:clone",
        "pipelines:read",
        "storage:read",
      ])
    ).toBe("仓库、流水线、存储")
  })
})
