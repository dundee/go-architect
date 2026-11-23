export interface ProjectInfo {
  name: string
  path: string
  package: string
  organization_packages: string[] | null
}

export interface RepositoryInfo {
  path: string
  url: string
  branch: string
  revision: string
  is_up_to_date: boolean
}

export interface ProjectContextResponse {
  project: ProjectInfo
  repository: RepositoryInfo | null
}

export interface PackageInstability {
  package_name: string
  abstractions_count: number
  implementations_count: number
  afferent_coupling: number
  efferent_coupling: number
  instability: number
  abstractness: number
  distance: number
}

export interface InstabilityResponse {
  packages: PackageInstability[]
  duration_ms: number
}

export interface ApiErrorResponse {
  error: string
}
