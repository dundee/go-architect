import { useCallback, useEffect, useState } from 'react'
import {
  Activity,
  AlertCircle,
  Compass,
  FolderGit2,
  FolderOpen,
  GitBranch,
  Grid3X3,
  Layers,
  Loader2,
  Network,
  RefreshCw,
} from 'lucide-react'
import { fetchProjectContext } from './api/client'
import { InstabilityView } from './components/InstabilityView'
import { ProjectSwitcherModal } from './components/ProjectSwitcherModal'
import type { ProjectContextResponse } from './types/api'

const TOOLS = [
  {
    id: 'instability',
    label: 'Abstractness & Instability',
    icon: Compass,
    enabled: true,
  },
  { id: 'metrics', label: 'Project Metrics', icon: Activity, enabled: false },
  { id: 'graph', label: 'Dependency Graph', icon: Network, enabled: false },
  { id: 'dsm', label: 'DSM', icon: Grid3X3, enabled: false },
  { id: 'coupling', label: 'Coupling', icon: Layers, enabled: false },
  { id: 'vcs', label: 'VCS Analysis', icon: FolderGit2, enabled: false },
] as const

export function App() {
  const [context, setContext] = useState<ProjectContextResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [refreshSignal, setRefreshSignal] = useState(0)
  const [switcherOpen, setSwitcherOpen] = useState(false)

  const loadProject = useCallback((signal?: AbortSignal) => {
    setLoading(true)
    setError(null)
    fetchProjectContext(signal)
      .then((res) => {
        if (signal?.aborted) return
        setContext(res)
      })
      .catch((err: unknown) => {
        if (signal?.aborted) return
        setError(
          err instanceof Error ? err.message : 'Failed to load project info',
        )
      })
      .finally(() => {
        if (signal?.aborted) return
        setLoading(false)
      })
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    loadProject(controller.signal)
    return () => controller.abort()
  }, [loadProject])

  return (
    <div className="min-h-screen bg-slate-50 text-slate-900">
      {/* Top Navbar */}
      <header className="sticky top-0 z-30 border-b border-slate-200 bg-white/90 backdrop-blur-md">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-6">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-600 text-white shadow-xs">
              <Compass className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold tracking-tight text-slate-900">
                  Go Architect
                </span>
                <span className="rounded-full bg-indigo-50 px-2 py-0.5 text-[10px] font-semibold text-indigo-700 ring-1 ring-indigo-600/20">
                  v2 preview
                </span>
              </div>
              {context ? (
                <div className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-xs text-slate-500">
                  <span className="font-mono font-medium text-slate-700">
                    {context.project.package}
                  </span>
                  <span className="hidden text-slate-300 sm:inline">•</span>
                  <span
                    className="hidden max-w-xs truncate font-mono text-[11px] text-slate-500 sm:inline"
                    title={context.project.path}
                  >
                    {context.project.path}
                  </span>
                  {context.repository && (
                    <>
                      <span className="text-slate-300">•</span>
                      <span className="inline-flex items-center gap-1 font-mono text-[11px] text-slate-600">
                        <GitBranch className="h-3 w-3 text-slate-400" />
                        {context.repository.branch}
                        <span
                          className={`h-1.5 w-1.5 rounded-full ${
                            context.repository.is_up_to_date
                              ? 'bg-emerald-500'
                              : 'bg-amber-500'
                          }`}
                          title={
                            context.repository.is_up_to_date
                              ? 'Working tree clean'
                              : 'Uncommitted changes'
                          }
                        />
                      </span>
                    </>
                  )}
                </div>
              ) : (
                <div className="text-xs text-slate-400">
                  {loading ? 'Loading workspace...' : 'No project loaded'}
                </div>
              )}
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setSwitcherOpen(true)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 shadow-2xs transition hover:bg-slate-50"
            >
              <FolderOpen className="h-3.5 w-3.5 text-slate-500" />
              Switch Path
            </button>
            <button
              type="button"
              onClick={() => setRefreshSignal((n) => n + 1)}
              disabled={!context}
              className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1.5 text-xs font-semibold text-white shadow-2xs transition hover:bg-slate-800 disabled:opacity-50"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Re-analyze
            </button>
          </div>
        </div>

        {/* Tool Tabs */}
        <div className="mx-auto flex max-w-7xl items-center gap-1 overflow-x-auto px-4 sm:px-6">
          {TOOLS.map((tool) => {
            const Icon = tool.icon
            return (
              <button
                key={tool.id}
                type="button"
                disabled={!tool.enabled}
                className={`inline-flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-xs font-medium transition ${
                  tool.enabled
                    ? 'border-indigo-600 text-indigo-700'
                    : 'cursor-not-allowed border-transparent text-slate-400'
                }`}
              >
                <Icon className="h-3.5 w-3.5" />
                <span>{tool.label}</span>
                {!tool.enabled && (
                  <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-400">
                    Soon
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
        {loading ? (
          <div className="flex items-center justify-center gap-2.5 rounded-xl border border-slate-200 bg-white py-16 text-sm text-slate-600 shadow-xs">
            <Loader2 className="h-5 w-5 animate-spin text-indigo-600" />
            <span>Loading Go project metadata...</span>
          </div>
        ) : error ? (
          <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-rose-900">
            <div className="flex items-start gap-3">
              <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
              <div>
                <h2 className="text-sm font-semibold">
                  Unable to load Go project
                </h2>
                <p className="mt-1 text-sm text-rose-700">{error}</p>
                <div className="mt-4 flex gap-2">
                  <button
                    type="button"
                    onClick={() => loadProject()}
                    className="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-rose-700"
                  >
                    Retry
                  </button>
                  <button
                    type="button"
                    onClick={() => setSwitcherOpen(true)}
                    className="rounded-lg border border-rose-300 bg-white px-3 py-1.5 text-xs font-semibold text-rose-800 hover:bg-rose-100"
                  >
                    Choose Another Path
                  </button>
                </div>
              </div>
            </div>
          </div>
        ) : context ? (
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <h1 className="text-xl font-bold tracking-tight text-slate-900">
                  Abstractness &amp; Instability Chart
                </h1>
                <p className="text-sm text-slate-500">
                  Evaluate package balance between abstraction (A) and
                  dependency instability (I). Hover or click any dot to inspect
                  packages.
                </p>
              </div>
            </div>

            <InstabilityView
              project={context.project}
              refreshSignal={refreshSignal}
            />
          </div>
        ) : null}
      </main>

      {switcherOpen && context && (
        <ProjectSwitcherModal
          currentPath={context.project.path}
          onClose={() => setSwitcherOpen(false)}
          onSwitched={(newCtx) => {
            setContext(newCtx)
            setRefreshSignal((n) => n + 1)
          }}
        />
      )}
      {switcherOpen && !context && (
        <ProjectSwitcherModal
          currentPath="."
          onClose={() => setSwitcherOpen(false)}
          onSwitched={(newCtx) => {
            setContext(newCtx)
            setError(null)
            setRefreshSignal((n) => n + 1)
          }}
        />
      )}
    </div>
  )
}
