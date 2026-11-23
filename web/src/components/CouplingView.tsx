import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Highlight, themes } from 'prism-react-renderer'
import {
  AlertCircle,
  ChevronDown,
  ChevronRight,
  Code2,
  FileCode2,
  Layers,
  Loader2,
  Package,
  Search,
} from 'lucide-react'
import { fetchCoupling, fetchDependencyGraph } from '../api/client'
import type {
  DependencyCoupling,
  FileCoupling,
  ModuleDependencyGraph,
  PackageCoupling,
  ProjectInfo,
} from '../types/api'
import {
  LAYER_META,
  LAYER_ORDER,
  type PackageLayer,
  shortenPackageName,
} from '../utils/packages'

interface CouplingViewProps {
  project: ProjectInfo
  initialDependency?: string | null
  refreshSignal: number
  onDependencyChange: (dep: string) => void
}

interface DependencyOption {
  name: string
  layer: PackageLayer
}

export function CouplingView({
  project,
  initialDependency,
  refreshSignal,
  onDependencyChange,
}: CouplingViewProps) {
  const [graph, setGraph] = useState<ModuleDependencyGraph | null>(null)
  const [graphLoading, setGraphLoading] = useState(true)

  const [query, setQuery] = useState(initialDependency ?? '')
  const [dropdownOpen, setDropdownOpen] = useState(false)

  const [result, setResult] = useState<DependencyCoupling | null>(null)
  const [durationMs, setDurationMs] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const containerRef = useRef<HTMLDivElement | null>(null)

  // Load dependency list for searchable combobox
  useEffect(() => {
    const controller = new AbortController()
    setGraphLoading(true)
    fetchDependencyGraph(controller.signal)
      .then((res) => {
        if (controller.signal.aborted) return
        setGraph(res.graph)
      })
      .catch(() => {
        // Non-fatal: manual input still works even if suggestion fetch fails
      })
      .finally(() => {
        if (!controller.signal.aborted) setGraphLoading(false)
      })
    return () => controller.abort()
  }, [project.path, project.organization_packages, refreshSignal])

  // Close dropdown on outside click
  useEffect(() => {
    const onPointerDown = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [])

  const runAnalysis = useCallback(
    (dep: string, signal?: AbortSignal) => {
      const normalized = dep.trim().toLowerCase()
      if (!normalized) return
      setLoading(true)
      setError(null)
      onDependencyChange(normalized)
      fetchCoupling(normalized, signal)
        .then((res) => {
          if (signal?.aborted) return
          setResult(res.coupling)
          setDurationMs(res.duration_ms ?? null)
        })
        .catch((err: unknown) => {
          if (signal?.aborted) return
          setError(
            err instanceof Error ? err.message : 'Failed to calculate coupling',
          )
        })
        .finally(() => {
          if (!signal?.aborted) setLoading(false)
        })
    },
    [onDependencyChange],
  )

  useEffect(() => {
    if (!initialDependency) return
    setQuery(initialDependency)
    const controller = new AbortController()
    runAnalysis(initialDependency, controller.signal)
    return () => controller.abort()
  }, [initialDependency, refreshSignal, runAnalysis])

  const options: DependencyOption[] = useMemo(() => {
    if (!graph) return []
    const list: DependencyOption[] = []
    const seen = new Set<string>()
    const addLayer = (pkgs: string[], layer: PackageLayer) => {
      for (const pkg of pkgs) {
        if (!seen.has(pkg)) {
          seen.add(pkg)
          list.push({ name: pkg, layer })
        }
      }
    }
    addLayer(graph.internal, 'internal')
    addLayer(graph.organization, 'organization')
    addLayer(graph.external, 'external')
    addLayer(graph.standard, 'standard')
    return list
  }, [graph])

  const filteredOptions = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return options
    return options.filter((opt) => opt.name.toLowerCase().includes(q))
  }, [options, query])

  const groupedOptions = useMemo(() => {
    const grouped: Record<PackageLayer, DependencyOption[]> = {
      internal: [],
      organization: [],
      external: [],
      standard: [],
    }
    for (const opt of filteredOptions) {
      grouped[opt.layer].push(opt)
    }
    return grouped
  }, [filteredOptions])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setDropdownOpen(false)
    runAnalysis(query)
  }

  return (
    <div className="flex flex-col gap-5">
      {/* Dependency Selector Card */}
      <form
        onSubmit={handleSubmit}
        className="rounded-xl border border-slate-200 bg-white p-4 shadow-xs"
      >
        <label
          htmlFor="coupling-dependency-input"
          className="mb-2 block text-xs font-semibold text-slate-700"
        >
          Target Dependency Package
        </label>
        <div className="flex flex-col gap-2 sm:flex-row">
          <div ref={containerRef} className="relative flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              id="coupling-dependency-input"
              type="text"
              value={query}
              onFocus={() => setDropdownOpen(true)}
              onChange={(e) => {
                setQuery(e.target.value)
                setDropdownOpen(true)
              }}
              placeholder={
                graphLoading
                  ? 'Loading project dependencies...'
                  : 'Select or type a package import path (e.g. net/http, github.com/...)'
              }
              className="w-full rounded-lg border border-slate-300 bg-white py-2 pr-9 pl-9 font-mono text-xs text-slate-900 placeholder:font-sans placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 focus:outline-none"
            />
            <button
              type="button"
              onClick={() => setDropdownOpen((o) => !o)}
              className="absolute top-1/2 right-2.5 -translate-y-1/2 text-slate-400 hover:text-slate-600"
              aria-label="Toggle dependency suggestions"
            >
              <ChevronDown className="h-4 w-4" />
            </button>

            {dropdownOpen && filteredOptions.length > 0 && (
              <div className="absolute z-30 mt-1.5 max-h-72 w-full overflow-auto rounded-xl border border-slate-200 bg-white p-1.5 shadow-xl">
                {LAYER_ORDER.map((layer) => {
                  const items = groupedOptions[layer]
                  if (items.length === 0) return null
                  const meta = LAYER_META[layer]
                  return (
                    <div key={layer} className="mb-1 last:mb-0">
                      <div className="flex items-center gap-1.5 px-2 py-1 text-[10px] font-semibold tracking-wider text-slate-500 uppercase">
                        <span
                          className="h-2 w-2 rounded-full"
                          style={{ backgroundColor: meta.dotColor }}
                        />
                        {meta.label} ({items.length})
                      </div>
                      {items.map((opt) => (
                        <button
                          key={opt.name}
                          type="button"
                          onClick={() => {
                            setQuery(opt.name)
                            setDropdownOpen(false)
                            runAnalysis(opt.name)
                          }}
                          className="flex w-full items-center justify-between rounded-lg px-2.5 py-1.5 text-left font-mono text-xs text-slate-800 hover:bg-indigo-50 hover:text-indigo-900"
                        >
                          <span className="truncate">{opt.name}</span>
                        </button>
                      ))}
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          <button
            type="submit"
            disabled={!query.trim() || loading}
            className="inline-flex items-center justify-center gap-1.5 rounded-lg bg-indigo-600 px-4 py-2 text-xs font-semibold text-white shadow-xs transition hover:bg-indigo-700 disabled:opacity-50"
          >
            {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            Analyze Coupling
          </button>
        </div>
      </form>

      {/* Loading */}
      {loading && (
        <div className="flex items-center gap-2.5 rounded-xl border border-indigo-200 bg-indigo-50/70 px-4 py-3 text-sm font-medium text-indigo-900">
          <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
          Scanning Go source files for coupling to{' '}
          <code className="font-mono text-xs font-semibold">{query}</code>...
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="flex items-start gap-2.5 rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-900">
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
          <div>
            <div className="font-semibold">Coupling analysis failed</div>
            <div className="mt-0.5 text-xs text-rose-700">{error}</div>
          </div>
        </div>
      )}

      {/* Empty prompt */}
      {!loading && !error && !result && (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white px-6 py-14 text-center">
          <Code2 className="h-8 w-8 text-slate-400" />
          <h2 className="mt-3 text-sm font-semibold text-slate-800">
            Select a dependency to inspect coupling
          </h2>
          <p className="mt-1 max-w-md text-xs text-slate-500">
            Choose any internal, organization, external, or standard library
            package from the dropdown above to see every package, file, and line
            that depends on it.
          </p>
        </div>
      )}

      {/* Result */}
      {!loading && result && (
        <div className="flex flex-col gap-4">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div className="rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-xs">
              <div className="flex items-center justify-between text-xs font-medium text-slate-500">
                <span>Evaluated Dependency</span>
                <Layers className="h-4 w-4 text-indigo-600" />
              </div>
              <div
                className="mt-1.5 truncate font-mono text-sm font-bold text-slate-900"
                title={result.dependency}
              >
                {result.dependency}
              </div>
            </div>

            <div className="rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-xs">
              <div className="flex items-center justify-between text-xs font-medium text-slate-500">
                <span>Overall Coupling Level</span>
                <Code2 className="h-4 w-4 text-amber-600" />
              </div>
              <div className="mt-1 font-mono text-2xl font-bold text-slate-900">
                {result.coupling_level}
              </div>
            </div>

            <div className="rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-xs">
              <div className="flex items-center justify-between text-xs font-medium text-slate-500">
                <span>Affected Packages</span>
                <Package className="h-4 w-4 text-emerald-600" />
              </div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="font-mono text-2xl font-bold text-slate-900">
                  {result.details.length}
                </span>
                {durationMs !== null && (
                  <span className="text-[11px] text-slate-400">
                    ({durationMs} ms)
                  </span>
                )}
              </div>
            </div>
          </div>

          {result.details.length === 0 ? (
            <div className="rounded-xl border border-slate-200 bg-white p-8 text-center text-sm text-slate-500">
              No files in <code className="font-mono">{project.package}</code>{' '}
              import or couple to{' '}
              <code className="font-mono font-semibold">
                {result.dependency}
              </code>
              .
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {result.details.map((pkgCoupling, idx) => (
                <PackageCouplingCard
                  key={pkgCoupling.package}
                  pkgCoupling={pkgCoupling}
                  mainPackage={project.package}
                  defaultOpen={idx === 0 || result.details.length === 1}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function PackageCouplingCard({
  pkgCoupling,
  mainPackage,
  defaultOpen,
}: {
  pkgCoupling: PackageCoupling
  mainPackage: string
  defaultOpen: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)
  const files = pkgCoupling.details ?? []

  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xs">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between gap-3 bg-slate-50/70 px-4 py-3 text-left transition hover:bg-slate-100/70"
      >
        <div className="flex min-w-0 items-center gap-2.5">
          {open ? (
            <ChevronDown className="h-4 w-4 shrink-0 text-slate-500" />
          ) : (
            <ChevronRight className="h-4 w-4 shrink-0 text-slate-500" />
          )}
          <span
            className="truncate font-mono text-xs font-semibold text-slate-900"
            title={pkgCoupling.package}
          >
            {shortenPackageName(pkgCoupling.package, mainPackage)}
          </span>
          <span className="text-[11px] text-slate-400">
            {files.length} {files.length === 1 ? 'file' : 'files'}
          </span>
        </div>

        <span className="shrink-0 rounded-full bg-amber-50 px-2.5 py-0.5 font-mono text-[11px] font-semibold text-amber-800 ring-1 ring-amber-600/20">
          Coupling: {pkgCoupling.coupling_level}
        </span>
      </button>

      {open && (
        <div className="divide-y divide-slate-100 border-t border-slate-200 p-3">
          {files.map((fileCoupling) => (
            <FileCouplingCard
              key={fileCoupling.file}
              fileCoupling={fileCoupling}
              defaultOpen={files.length === 1}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function FileCouplingCard({
  fileCoupling,
  defaultOpen,
}: {
  fileCoupling: FileCoupling
  defaultOpen: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)
  const lineRefs = useRef<Record<number, HTMLDivElement | null>>({})

  const coupledLineSet = useMemo(
    () => new Set<number>(fileCoupling.coupling_lines ?? []),
    [fileCoupling.coupling_lines],
  )

  const jumpToLine = (lineNumber: number) => {
    if (!open) {
      setOpen(true)
      setTimeout(() => {
        lineRefs.current[lineNumber]?.scrollIntoView({
          behavior: 'smooth',
          block: 'center',
        })
      }, 40)
    } else {
      lineRefs.current[lineNumber]?.scrollIntoView({
        behavior: 'smooth',
        block: 'center',
      })
    }
  }

  return (
    <div className="py-2 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg px-2 py-1.5 hover:bg-slate-50">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="flex min-w-0 items-center gap-2 text-left"
        >
          {open ? (
            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-slate-400" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 shrink-0 text-slate-400" />
          )}
          <FileCode2 className="h-4 w-4 shrink-0 text-indigo-500" />
          <span className="truncate font-mono text-xs font-medium text-slate-800">
            {fileCoupling.file}
          </span>
        </button>

        <div className="flex flex-wrap items-center gap-1.5">
          {(fileCoupling.coupling_lines ?? []).map((line) => (
            <button
              key={line}
              type="button"
              onClick={() => jumpToLine(line)}
              className="rounded bg-indigo-50 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-indigo-700 ring-1 ring-indigo-600/20 hover:bg-indigo-100"
              title={`Jump to line ${line}`}
            >
              L{line}
            </button>
          ))}
          <span className="rounded-full bg-slate-100 px-2 py-0.5 font-mono text-[10px] font-semibold text-slate-700">
            Level: {fileCoupling.coupling_level}
          </span>
        </div>
      </div>

      {open && (
        <div className="mt-2 overflow-hidden rounded-lg border border-slate-200 bg-slate-50">
          {fileCoupling.details && fileCoupling.details.length > 0 && (
            <div className="flex flex-wrap gap-1.5 border-b border-slate-200 bg-white px-3 py-2">
              {fileCoupling.details.map((d, i) => (
                <button
                  key={`${d.line}-${i}`}
                  type="button"
                  onClick={() => jumpToLine(d.line)}
                  className="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2 py-0.5 text-[11px] text-amber-900 ring-1 ring-amber-600/20 hover:bg-amber-100"
                >
                  <span className="font-mono font-semibold">L{d.line}:</span>
                  <span className="font-mono">{d.details}</span>
                </button>
              ))}
            </div>
          )}

          <div className="max-h-[460px] overflow-auto">
            <Highlight
              theme={themes.github}
              code={fileCoupling.file_content}
              language="go"
            >
              {({ className, style, tokens, getLineProps, getTokenProps }) => (
                <pre
                  className={`${className} m-0 py-2 font-mono text-xs leading-5`}
                  style={{ ...style, backgroundColor: 'transparent' }}
                >
                  {tokens.map((lineTokens, i) => {
                    const lineNumber = i + 1
                    const isCoupled = coupledLineSet.has(lineNumber)
                    const lineProps = getLineProps({ line: lineTokens })
                    return (
                      <div
                        key={lineNumber}
                        ref={(el) => {
                          lineRefs.current[lineNumber] = el
                        }}
                        {...lineProps}
                        className={`flex px-3 ${
                          isCoupled
                            ? 'border-l-2 border-amber-500 bg-amber-100/70'
                            : 'hover:bg-slate-100/80'
                        }`}
                      >
                        <span
                          className={`mr-4 w-9 shrink-0 text-right select-none ${
                            isCoupled
                              ? 'font-bold text-amber-800'
                              : 'text-slate-400'
                          }`}
                        >
                          {lineNumber}
                        </span>
                        <span className="whitespace-pre">
                          {lineTokens.map((token, key) => (
                            <span key={key} {...getTokenProps({ token })} />
                          ))}
                        </span>
                      </div>
                    )
                  })}
                </pre>
              )}
            </Highlight>
          </div>
        </div>
      )}
    </div>
  )
}
