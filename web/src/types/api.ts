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

export interface ModuleDependencyGraph {
  module: string
  internal: string[]
  external: string[]
  organization: string[]
  standard: string[]
  relations: Record<string, string[]>
}

export interface DependencyGraphResponse {
  graph: ModuleDependencyGraph
  duration_ms: number
}

export interface DependencyStructureMatrix {
  module: string
  packages: string[]
  dependencies: number[][]
}

export interface DSMResponse {
  dsm: DependencyStructureMatrix
  duration_ms: number
}

export interface CouplingDetail {
  line: number
  details: string
}

export interface FileCoupling {
  package: string
  file: string
  file_content: string
  coupling_lines: number[] | null
  coupling_level: number
  details: CouplingDetail[] | null
}

export interface PackageCoupling {
  package: string
  coupling_level: number
  details: FileCoupling[] | null
}

export interface DependencyCoupling {
  dependency: string
  coupling_level: number
  details: PackageCoupling[]
}

export interface CouplingResponse {
  coupling: DependencyCoupling
  duration_ms: number
}

export interface ApiErrorResponse {
  error: string
}
