import { describe, expect, it } from "vitest"

import { RecentEventsPreview, type RecentEvent } from "./recent-events"

const event = (id: string): RecentEvent => ({
  id,
  action: id,
  actor: "gitsss",
  time: "刚刚",
  kind: "change",
})

describe("RecentEventsPreview", () => {
  it("keeps only the most recently touched events", () => {
    const preview = RecentEventsPreview.from(2, [
      event("first"),
      event("second"),
    ])

    preview.touch(event("first"))
    preview.touch(event("third"))

    expect(preview.items().map(({ id }) => id)).toEqual(["third", "first"])
  })

  it("renders seeded events newest first", () => {
    const preview = RecentEventsPreview.from(3, [
      event("newest"),
      event("middle"),
      event("oldest"),
    ])

    expect(preview.items().map(({ id }) => id)).toEqual([
      "newest",
      "middle",
      "oldest",
    ])
  })

  it("clears only the preview buffer", () => {
    const preview = RecentEventsPreview.from(2, [event("first")])

    preview.clear()

    expect(preview.items()).toEqual([])
  })
})
