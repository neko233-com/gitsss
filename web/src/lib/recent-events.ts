export type RecentEvent = {
  id: string
  action: string
  actor: string
  time: string
  kind: "change" | "build" | "review" | "release" | "member"
}

export class RecentEventsPreview {
  private readonly events = new Map<string, RecentEvent>()
  private readonly capacity: number

  public constructor(capacity: number) {
    if (!Number.isSafeInteger(capacity) || capacity < 1) {
      throw new Error(
        "Recent event preview capacity must be a positive integer"
      )
    }
    this.capacity = capacity
  }

  public static from(capacity: number, events: RecentEvent[]) {
    const preview = new RecentEventsPreview(capacity)
    for (const event of [...events].reverse()) {
      preview.touch(event)
    }
    return preview
  }

  public clone() {
    return RecentEventsPreview.from(this.capacity, this.items())
  }

  public clear() {
    this.events.clear()
  }

  public items() {
    return [...this.events.values()].reverse()
  }

  public touch(event: RecentEvent) {
    this.events.delete(event.id)
    this.events.set(event.id, event)

    if (this.events.size > this.capacity) {
      const oldestEventId = this.events.keys().next().value
      if (oldestEventId) {
        this.events.delete(oldestEventId)
      }
    }
  }
}
