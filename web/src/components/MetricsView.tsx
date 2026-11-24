import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertCircle,
  AlertTriangle,
  Box,
  Check,
  Code2,
  Copy,
  FileText,
  Gauge,
  Loader2,
  MessageSquareText,
  RefreshCw,
  Terminal,
} from 'lucide-react'
import { fetchMetrics } from '../api/client'
import type {
  ComplexityResult,
  InterfaceInfo,
  MetricsResponse,
  ProjectInfo,
} from '../types/api'
import { shortenPackageName } from '../utils/packages'

interface MetricsViewProps {
  project: ProjectInfo
  refreshSignal: number
}

export function MetricsView({ project, refreshSignal }: MetricsViewProps) {
  const [data, setData] = useState<MetricsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const lastRefreshRef = useRef(refreshSignal)

  const loadData = useCallback((signal?: AbortSignal, refresh?: boolean) => {
    setLoading(true)
    setError(null)
    fetchMetrics(signal, refresh)
      .then((res) => {
        if (signal?.aborted) return
        setData(res)
      })
      .catch((err: unknown) => {
        if (signal?.aborted) return
        setError(
          err instanceof Error ? err.message : 'Failed to load Project Metrics',
        )
      })
      .finally(() => {
        if (!signal?.aborted) setLoading(false)
      })
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    const isRefresh = lastRefreshRef.current !== refreshSignal
    lastRefreshRef.current = refreshSignal
    loadData(controller.signal, isRefresh)
    return () => controller.abort()
  }, [project.path, refreshSignal, loadData])

  const topPackages = useMemo(() => {
    if (!data?.loc.packages) return []
    return [...data.loc.packages].sort((a, b) => b.loc - a.loc).slice(0, 5)
  }, [data])

  const topFiles = useMemo(() => {
    if (!data?.loc.files) return []
    return [...data.loc.files].sort((a, b) => b.loc - a.loc).slice(0, 5)
  }, [data])

  if (loading && !data) {
    return (
      <div className="flex flex-col gap-5">
        <div className="flex items-center gap-2.5 rounded-xl border border-indigo-200 bg-indigo-50/70 px-4 py-3 text-sm font-medium text-indigo-900">
          <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
          Measuring lines of code, types, interfaces, comments, and complexity...
        </div>
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {[0, 1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-24 animate-pulse rounded-xl border border-slate-200 bg-white"
            />
          ))}
        </div>
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <div className="h-72 animate-pulse rounded-xl border border-slate-200 bg-white" />
          <div className="h-72 animate-pulse rounded-xl border border-slate-200 bg-white" />
        </div>
      </div>
    )
  }

  if (error && !data) {
    return (
      <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-rose-900">
        <div className="flex items-start gap-3">
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
          <div>
            <h3 className="text-sm font-semibold">
              Failed to load Project Metrics
            </h3>
            <p className="mt-1 text-sm text-rose-700">{error}</p>
            <button
              type="button"
              onClick={() => loadData()}
              className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-rose-700"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Retry
            </button>
          </div>
        </div>
      </div>
    )
  }

  if (!data) return null

  const mainPkg = data.types.project_package || project.package
  const commentPct = data.comments.ratio.toFixed(1)
  const filesWithCommentsPct = data.comments.files_with_comments_ratio.toFixed(1)

  return (
    <div className="flex flex-col gap-5">
      {loading && (
        <div className="flex items-center gap-2 rounded-lg border border-indigo-200 bg-indigo-50 px-3.5 py-2 text-xs font-medium text-indigo-800">
          <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
          Refreshing Project metrics...
        </div>
      )}

      {/* KPI Strip */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <MetricKpiCard
          icon={<Code2 className="h-4 w-4 text-indigo-600" />}
          label="Total Lines of Code"
          value={data.loc.total.toLocaleString()}
          hint={`${data.types.packages} packages · ${data.types.source_files} Go files`}
        />
        <MetricKpiCard
          icon={<Box className="h-4 w-4 text-sky-600" />}
          label="Types & Functions"
          value={(
            data.types.structs +
            data.types.interfaces +
            data.types.functions +
            data.types.methods
          ).toLocaleString()}
          hint={`${data.types.structs} structs · ${data.types.interfaces} interfaces`}
        />
        <MetricKpiCard
          icon={<MessageSquareText className="h-4 w-4 text-emerald-600" />}
          label="Comment Ratio"
          value={`${commentPct}%`}
          hint={`${data.comments.total_lines.toLocaleString()} lines · ${filesWithCommentsPct}% of files`}
        />
        <MetricKpiCard
          icon={<Gauge className="h-4 w-4 text-amber-600" />}
          label="Avg Complexity"
          value={
            data.cyclomatic_complexity.data
              ? data.cyclomatic_complexity.data.average_complexity.toFixed(2)
              : 'N/A'
          }
          hint={`Measured in ${data.duration_ms} ms`}
        />
      </div>

      {/* LOC + Types & Elements */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-12">
        <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-xs lg:col-span-6">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
              <FileText className="h-4 w-4 text-indigo-600" />
              Lines of Code Breakdown
            </h2>
            <span className="font-mono text-xs font-semibold text-slate-500">
              {data.loc.total.toLocaleString()} LOC
            </span>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <h3 className="mb-2 text-[11px] font-semibold tracking-wider text-slate-500 uppercase">
                Largest Packages
              </h3>
              <ul className="divide-y divide-slate-100 rounded-lg border border-slate-200">
                {topPackages.map((p) => (
                  <li
                    key={p.package}
                    className="flex items-center justify-between gap-2 px-3 py-2 text-xs"
                  >
                    <span
                      className="truncate font-mono text-slate-800"
                      title={p.package}
                    >
                      {shortenPackageName(p.package, mainPkg)}
                    </span>
                    <span className="shrink-0 font-mono font-semibold text-slate-900">
                      {p.loc.toLocaleString()}
                    </span>
                  </li>
                ))}
              </ul>
            </div>

            <div>
              <h3 className="mb-2 text-[11px] font-semibold tracking-wider text-slate-500 uppercase">
                Largest Files
              </h3>
              <ul className="divide-y divide-slate-100 rounded-lg border border-slate-200">
                {topFiles.map((f) => (
                  <li
                    key={`${f.package}/${f.file}`}
                    className="flex items-center justify-between gap-2 px-3 py-2 text-xs"
                  >
                    <div className="min-w-0">
                      <div
                        className="truncate font-mono text-slate-800"
                        title={f.file}
                      >
                        {f.file}
                      </div>
                      <div className="truncate font-mono text-[10px] text-slate-400">
                        {shortenPackageName(f.package, mainPkg)}
                      </div>
                    </div>
                    <span className="shrink-0 font-mono font-semibold text-slate-900">
                      {f.loc.toLocaleString()}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>

        <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-xs lg:col-span-6">
          <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-slate-900">
            <Box className="h-4 w-4 text-sky-600" />
            Declarations &amp; Visibility
          </h2>

          <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3">
            <ElementStat
              label="Structs"
              total={data.types.structs}
              exported={data.types.public_structs}
            />
            <ElementStat
              label="Interfaces"
              total={data.types.interfaces}
              exported={data.types.public_interfaces}
            />
            <ElementStat
              label="Functions"
              total={data.types.functions}
              exported={data.types.public_functions}
            />
            <ElementStat
              label="Methods"
              total={data.types.methods}
              exported={data.types.public_methods}
            />
            <ElementStat
              label="Variables"
              total={data.types.variables}
              exported={data.types.public_variables}
            />
            <ElementStat
              label="Constants"
              total={data.types.constants}
              exported={data.types.public_constants}
            />
          </div>

          {/* Interfaces & Comments Summary */}
          <div className="mt-4 grid grid-cols-1 gap-3 border-t border-slate-100 pt-4 sm:grid-cols-2">
            <div className="rounded-lg bg-slate-50 p-3">
              <div className="text-[11px] font-semibold text-slate-500 uppercase">
                Interface Size
              </div>
              <div className="mt-1 font-mono text-lg font-bold text-slate-900">
                {data.interfaces.average_methods.toFixed(2)}{' '}
                <span className="text-xs font-normal text-slate-500">
                  avg methods
                </span>
              </div>
              <InterfaceSummaryList
                label="Largest"
                items={data.interfaces.max_methods}
              />
            </div>

            <div className="rounded-lg bg-slate-50 p-3">
              <div className="text-[11px] font-semibold text-slate-500 uppercase">
                Documentation
              </div>
              <div className="mt-1 font-mono text-lg font-bold text-slate-900">
                {data.comments.files_with_comments}{' '}
                <span className="text-xs font-normal text-slate-500">
                  / {data.types.source_files} files commented
                </span>
              </div>
              <div className="mt-1 text-xs text-slate-600">
                {data.comments.total_lines.toLocaleString()} comment lines (
                {commentPct}% of LOC)
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Cyclomatic & Cognitive Complexity */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <ComplexityCard
          title="Cyclomatic Complexity"
          result={data.cyclomatic_complexity}
          mainPkg={mainPkg}
        />
        <ComplexityCard
          title="Cognitive Complexity"
          result={data.cognitive_complexity}
          mainPkg={mainPkg}
        />
      </div>
    </div>
  )
}

function MetricKpiCard({
  icon,
  label,
  value,
  hint,
}: {
  icon: React.ReactNode
  label: string
  value: string
  hint: string
}) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-xs">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-slate-500">{label}</span>
        {icon}
      </div>
      <div className="mt-1.5 font-mono text-2xl font-bold tracking-tight text-slate-900">
        {value}
      </div>
      <div className="mt-0.5 text-[11px] text-slate-500">{hint}</div>
    </div>
  )
}

function ElementStat({
  label,
  total,
  exported,
}: {
  label: string
  total: number
  exported: number
}) {
  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50/60 px-3 py-2.5">
      <div className="text-[11px] font-medium text-slate-500">{label}</div>
      <div className="mt-0.5 font-mono text-lg font-bold text-slate-900">
        {total.toLocaleString()}
      </div>
      <div className="text-[10px] text-slate-500">
        {exported.toLocaleString()} exported
      </div>
    </div>
  )
}

function InterfaceSummaryList({
  label,
  items,
}: {
  label: string
  items: InterfaceInfo[] | null
}) {
  if (!items || items.length === 0) {
    return (
      <div className="mt-1 text-xs text-slate-400">No interfaces declared</div>
    )
  }
  const first = items[0]
  return (
    <div className="mt-1 truncate text-xs text-slate-600" title={`${first.package}.${first.name}`}>
      {label}: <span className="font-mono font-semibold">{first.name}</span> (
      {first.methods} methods)
    </div>
  )
}

function ComplexityCard({
  title,
  result,
  mainPkg,
}: {
  title: string
  result: ComplexityResult
  mainPkg: string
}) {
  const [copied, setCopied] = useState(false)

  const copyHint = () => {
    if (!result.install_hint) return
    navigator.clipboard?.writeText(result.install_hint)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-xs">
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
          <Gauge className="h-4 w-4 text-amber-600" />
          {title}
        </h2>
        {result.data && (
          <span className="rounded-full bg-amber-50 px-2.5 py-0.5 font-mono text-xs font-semibold text-amber-800 ring-1 ring-amber-600/20">
            Avg: {result.data.average_complexity.toFixed(2)}
          </span>
        )}
      </div>

      {result.error ? (
        <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-4 text-xs text-amber-950">
          <div className="flex items-start gap-2.5">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
            <div className="flex-1">
              <div className="font-semibold">Optional CLI tool unavailable</div>
              <p className="mt-0.5 text-amber-800">{result.error}</p>
              {result.install_hint && (
                <div className="mt-3 flex items-center justify-between gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 font-mono text-[11px] text-slate-800">
                  <span className="flex items-center gap-2 truncate">
                    <Terminal className="h-3.5 w-3.5 shrink-0 text-slate-400" />
                    <span className="truncate">{result.install_hint}</span>
                  </span>
                  <button
                    type="button"
                    onClick={copyHint}
                    className="inline-flex shrink-0 items-center gap-1 rounded bg-slate-100 px-2 py-1 text-[10px] font-semibold text-slate-700 hover:bg-slate-200"
                  >
                    {copied ? (
                      <>
                        <Check className="h-3 w-3 text-emerald-600" />
                        Copied
                      </>
                    ) : (
                      <>
                        <Copy className="h-3 w-3" />
                        Copy
                      </>
                    )}
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      ) : !result.data || result.data.ComplexityDetails.length === 0 ? (
        <div className="rounded-lg border border-dashed border-slate-200 p-6 text-center text-xs text-slate-500">
          No complexity hotspots reported.
        </div>
      ) : (
        <div className="overflow-auto rounded-lg border border-slate-200">
          <table className="w-full border-collapse text-left text-xs">
            <thead className="border-b border-slate-200 bg-slate-50 text-[11px] font-semibold text-slate-600 uppercase">
              <tr>
                <th className="px-3 py-2">Function</th>
                <th className="px-3 py-2">Package</th>
                <th className="px-3 py-2 text-right">Score</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {result.data.ComplexityDetails.map((d, idx) => (
                <tr key={`${d.Package}-${d.File}-${d.Function}-${idx}`}>
                  <td className="px-3 py-2">
                    <div className="font-mono font-semibold text-slate-900">
                      {d.Function}
                    </div>
                    <div className="font-mono text-[10px] text-slate-400">
                      {d.File}
                    </div>
                  </td>
                  <td
                    className="max-w-[180px] truncate px-3 py-2 font-mono text-slate-600"
                    title={d.Package}
                  >
                    {shortenPackageName(d.Package, mainPkg)}
                  </td>
                  <td className="px-3 py-2 text-right font-mono font-bold text-slate-900">
                    {d.Complexity}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
