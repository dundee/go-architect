import type {
  ApiErrorResponse,
  CouplingResponse,
  DependencyGraphResponse,
  DSMResponse,
  InstabilityResponse,
  MetricsResponse,
  ProjectContextResponse,
  VCSResponse,
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

export function fetchProjectContext(
  signal?: AbortSignal,
): Promise<ProjectContextResponse> {
  return requestJson<ProjectContextResponse>('/api/project', { signal })
}

export function switchProjectPath(
  path: string,
  organizationPackages?: string[],
  signal?: AbortSignal,
): Promise<ProjectContextResponse> {
  return requestJson<ProjectContextResponse>('/api/project', {
    method: 'POST',
    body: JSON.stringify({
      path,
      organization_packages: organizationPackages,
    }),
    signal,
  })
}

export function fetchInstability(
  signal?: AbortSignal,
): Promise<InstabilityResponse> {
  return requestJson<InstabilityResponse>('/api/instability', { signal })
}

export function fetchDependencyGraph(
  signal?: AbortSignal,
): Promise<DependencyGraphResponse> {
  return requestJson<DependencyGraphResponse>('/api/dependency-graph', {
    signal,
  })
}

export function fetchDSM(signal?: AbortSignal): Promise<DSMResponse> {
  return requestJson<DSMResponse>('/api/dsm', { signal })
}

export function fetchCoupling(
  dependency: string,
  signal?: AbortSignal,
): Promise<CouplingResponse> {
  const params = new URLSearchParams({ dependency })
  return requestJson<CouplingResponse>(`/api/coupling?${params.toString()}`, {
    signal,
  })
}

export function fetchMetrics(signal?: AbortSignal): Promise<MetricsResponse> {
  return requestJson<MetricsResponse>('/api/metrics', { signal })
}

export function fetchVCS(
  months: number,
  signal?: AbortSignal,
): Promise<VCSResponse> {
  const params = new URLSearchParams({ months: String(months) })
  return requestJson<VCSResponse>(`/api/vcs?${params.toString()}`, { signal })
}
