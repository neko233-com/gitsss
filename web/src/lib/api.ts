export class APIError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      error?: string
    } | null
    throw new APIError(
      body?.error ?? `请求失败（${response.status}）`,
      response.status
    )
  }
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
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
  return readResponse<T>(response)
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
  return readResponse<T>(response)
}

export async function apiDelete(
  path: string,
  token: string,
  signal?: AbortSignal
): Promise<void> {
  const response = await fetch(path, {
    headers: { Authorization: `Bearer ${token}` },
    method: "DELETE",
    signal,
  })
  await readResponse<void>(response)
}
