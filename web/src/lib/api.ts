export class APIError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

export async function apiRequest<T>(
  path: string,
  token: string,
  signal?: AbortSignal
): Promise<T> {
  const response = await fetch(path, {
    headers: { Authorization: `Bearer ${token}` },
    signal,
  })
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      error?: string
    } | null
    throw new APIError(
      body?.error ?? `请求失败（${response.status}）`,
      response.status
    )
  }
  return (await response.json()) as T
}

export async function apiMutation<T>(
  path: string,
  token: string,
  body: unknown,
  signal?: AbortSignal
): Promise<T> {
  const response = await fetch(path, {
    body: JSON.stringify(body),
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    method: "POST",
    signal,
  })
  if (!response.ok) {
    const responseBody = (await response.json().catch(() => null)) as {
      error?: string
    } | null
    throw new APIError(
      responseBody?.error ?? `请求失败（${response.status}）`,
      response.status
    )
  }
  return (await response.json()) as T
}
