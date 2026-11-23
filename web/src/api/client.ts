import type {
  ApiErrorResponse,
  InstabilityResponse,
  ProjectContextResponse,
} from '../types/api'

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function requestJson<T>(
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(input, {
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
    ...init,
  })

  const text = await response.text()
  let parsed: unknown = null
  if (text) {
    try {
      parsed = JSON.parse(text)
    } catch {
      if (!response.ok) {
        throw new ApiError(text, response.status)
      }
      throw new ApiError('Invalid JSON response from server', response.status)
    }
  }

  if (!response.ok) {
    const errBody = parsed as Partial<ApiErrorResponse> | null
    throw new ApiError(
      errBody?.error || `Request failed with status ${response.status}`,
      response.status,
    )
  }

  return parsed as T
}

export function fetchProjectContext(signal?: AbortSignal): Promise<ProjectContextResponse> {
  return requestJson<ProjectContextResponse>('/api/project', { signal })
}

export function switchProjectPath(
  path: string,
  signal?: AbortSignal,
): Promise<ProjectContextResponse> {
  return requestJson<ProjectContextResponse>('/api/project', {
    method: 'POST',
    body: JSON.stringify({ path }),
    signal,
  })
}

export function fetchInstability(signal?: AbortSignal): Promise<InstabilityResponse> {
  return requestJson<InstabilityResponse>('/api/instability', { signal })
}
