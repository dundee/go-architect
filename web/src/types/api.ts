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
  cached?: boolean
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
  cached?: boolean
  duration_ms: number
}

export interface DependencyStructureMatrix {
  module: string
  packages: string[]
  dependencies: number[][]
}

export interface DSMResponse {
  dsm: DependencyStructureMatrix
  cached?: boolean
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
  cached?: boolean
  duration_ms: number
}

export interface PackageLOC {
  package: string
  loc: number
}

export interface FileLOC {
  package: string
  file: string
  loc: number
}

export interface ProjectLOC {
  total: number
  packages: PackageLOC[] | null
  files: FileLOC[] | null
}

export interface PackageTypes {
  package: string
  source_files: number
  structs: number
  interfaces: number
  functions: number
  methods: number
  variables: number
  constants: number
  public_structs: number
  public_interfaces: number
  public_functions: number
  public_methods: number
  public_variables: number
  public_constants: number
}

export interface ProjectTypes extends Omit<PackageTypes, 'package'> {
  project_package: string
  packages: number
  details: PackageTypes[] | null
}

export interface InterfaceInfo {
  package: string
  file: string
  name: string
  methods: number
}

export interface InterfaceMetrics {
  average_methods: number
  max_methods: InterfaceInfo[] | null
  min_methods: InterfaceInfo[] | null
}

export interface CommentsMetrics {
  total_lines: number
  ratio: number
  files_with_comments: number
  files_with_comments_ratio: number
}

export interface ComplexityDetail {
  Package: string
  File: string
  Function: string
  Complexity: number
}

export interface GoCycloOutput {
  average_complexity: number
  ComplexityDetails: ComplexityDetail[]
}

export interface ComplexityResult {
  data: GoCycloOutput | null
  error?: string
  install_hint?: string
}

export interface TestMetrics {
  loc: number
  files: number
  functions: number
  ratio: number
  largest_files: FileLOC[] | null
}

export interface FunctionDetail {
  package: string
  file: string
  name: string
  loc: number
  params: number
}

export interface FunctionMetrics {
  total: number
  average_loc: number
  max_loc: number
  average_params: number
  max_params: number
  long_functions: FunctionDetail[] | null
  high_param_funcs: FunctionDetail[] | null
}

export interface MetricsResponse {
  loc: ProjectLOC
  types: ProjectTypes
  interfaces: InterfaceMetrics
  comments: CommentsMetrics
  tests: TestMetrics | null
  functions: FunctionMetrics | null
  cyclomatic_complexity: ComplexityResult
  cognitive_complexity: ComplexityResult
  cached?: boolean
  duration_ms: number
}

export interface AuthorInfo {
  Name: string
  Commits: number
  ModifiedLines: number
}

export interface AuthorsInfo {
  TotalAuthors: number
  TotalCommits: number
  TotalModifications: number
  AuthorDetails: AuthorInfo[]
}

export interface ContributorSummary {
  Name: string
  Modifications: number
}

export interface ModificationsInfo {
  Source: string
  TotalModifications: number
  AuthorsSummary: ContributorSummary[]
}

export interface Modifications {
  PathModifications: ModificationsInfo[]
  FileModifications: ModificationsInfo[]
}

export interface VCSAnalysisInfo {
  Since?: string
  AuthorsInfo: AuthorsInfo
  ModificationsInfo: Modifications
}

export interface VCSResponse {
  vcs: VCSAnalysisInfo
  months: number
  cached?: boolean
  duration_ms: number
}

export interface FileHotspot {
  file: string
  package: string
  loc: number
  commits: number
  churn: number
  contributors: number
  cyclomatic_complexity: number
  cognitive_complexity: number
  max_cyclomatic: number
  max_cognitive: number
  score: number
}

export interface HotspotsResponse {
  hotspots: FileHotspot[]
  total_files: number
  months: number
  cached?: boolean
  duration_ms: number
}

export type SamplingMode = 'buckets' | 'tags'

export interface SnapshotMetrics {
  loc: number
  packages: number
  source_files: number
  structs: number
  interfaces: number
  functions: number
  methods: number
  comments_total_lines: number
  comments_ratio: number
  interface_avg_methods: number
  avg_distance: number
  zone_of_pain_count: number
  zone_of_uselessness_count: number
  test_loc: number
  test_files: number
  test_functions: number
  avg_function_loc: number
  avg_function_params: number
  cyclomatic_complexity_avg?: number
  cognitive_complexity_avg?: number
}

export interface CommitSnapshot {
  commit_sha: string
  short_sha: string
  date: string
  message: string
  tag?: string
  metrics: SnapshotMetrics
  cached: boolean
}

export interface TrendsReport {
  mode: SamplingMode
  months: number
  points: CommitSnapshot[]
  cached_count: number
  computed_count: number
  skipped_count: number
  skipped_reasons?: string[]
  duration_ms: number
}

export interface TrendsProgressEvent {
  type: 'start' | 'progress'
  index?: number
  total: number
  commit_sha?: string
  short_sha?: string
  message?: string
  tag?: string
  status?: 'cached' | 'computing' | 'done' | 'skipped'
  reason?: string
  snapshot?: CommitSnapshot
}

export interface ApiErrorResponse {
  error: string
}
