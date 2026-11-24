import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  AlertTriangle,
  Calendar,
  CheckCircle2,
  Database,
  GitCommitHorizontal,
  Loader2,
  RefreshCw,
  Tag,
  TrendingUp,
} from 'lucide-react'
import { subscribeTrendsStream } from '../api/client'
import type {
  CommitSnapshot,
  ProjectInfo,
  SamplingMode,
  TrendsProgressEvent,
  TrendsReport,
} from '../types/api'

interface TrendsViewProps {
  project: ProjectInfo
  refreshSignal: number
}

interface SeriesConfig {
  id: string
  label: string
  color: string
  getValue: (p: CommitSnapshot) => number | null
  formatValue?: (v: number) => string
}

const TIME_WINDOWS = [
  { label: '3 mo', months: 3 },
  { label: '6 mo', months: 6 },
  { label: '12 mo', months: 12 },
  { label: 'All time', months: 0 },
]

function formatShortDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: '2-digit',
  })
}

interface ProgressState {
  total: number
  completed: number
  cachedCount: number
  computedCount: number
  skippedCount: number
  current: TrendsProgressEvent | null
}

export function TrendsView({ project, refreshSignal }: TrendsViewProps) {
  const [mode, setMode] = useState<SamplingMode>('buckets')
  const [months, setMonths] = useState(12)
  const [includeComplexity, setIncludeComplexity] = useState(false)

  const [report, setReport] = useState<TrendsReport | null>(null)
  const [livePoints, setLivePoints] = useState<CommitSnapshot[]>([])
  const [progress, setProgress] = useState<ProgressState | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null)
  const [retryToken, setRetryToken] = useState(0)

  const startStream = useCallback(() => {
    setLoading(true)
    setError(null)
    setLivePoints([])
    setProgress(null)
    setHoveredIdx(null)

    return subscribeTrendsStream(
      {
        mode,
        months,
        limit: mode === 'tags' ? 20 : 12,
        includeComplexity,
      },
      {
        onStart: (ev) => {
          setProgress({
            total: ev.total,
            completed: 0,
            cachedCount: 0,
            computedCount: 0,
            skippedCount: 0,
            current: null,
          })
        },
        onProgress: (ev) => {
          setProgress((prev) => {
            const base: ProgressState = prev ?? {
              total: ev.total,
              completed: 0,
              cachedCount: 0,
              computedCount: 0,
              skippedCount: 0,
              current: null,
            }
            if (ev.status === 'computing') {
              return { ...base, total: ev.total, current: ev }
            }
            return {
              total: ev.total,
              completed: base.completed + 1,
              cachedCount:
                base.cachedCount + (ev.status === 'cached' ? 1 : 0),
              computedCount:
                base.computedCount + (ev.status === 'done' ? 1 : 0),
              skippedCount:
                base.skippedCount + (ev.status === 'skipped' ? 1 : 0),
              current: ev,
            }
          })

          const snap = ev.snapshot
          if (snap && (ev.status === 'cached' || ev.status === 'done')) {
            setLivePoints((prev) => {
              if (prev.some((p) => p.commit_sha === snap.commit_sha)) {
                return prev
              }
              return [...prev, snap]
            })
          }
        },
        onComplete: (finalReport) => {
          setReport(finalReport)
          setLivePoints(finalReport.points)
          setLoading(false)
        },
        onError: (msg) => {
          setError(msg)
          setLoading(false)
        },
      },
    )
  }, [mode, months, includeComplexity])

  useEffect(() => {
    const cleanup = startStream()
    return cleanup
  }, [
    project.path,
    project.organization_packages,
    refreshSignal,
    retryToken,
    startStream,
  ])

  const points = loading ? livePoints : (report?.points ?? livePoints)
  const first = points.length > 0 ? points[0] : null
  const latest = points.length > 0 ? points[points.length - 1] : null

  const hasComplexityData = useMemo(
    () =>
      points.some(
        (p) =>
          p.metrics.cyclomatic_complexity_avg !== undefined ||
          p.metrics.cognitive_complexity_avg !== undefined,
      ),
    [points],
  )

  return (
    <div className="flex flex-col gap-5">
      {/* Controls Bar */}
      <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-3.5 shadow-xs lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-1 rounded-lg bg-slate-100 p-1">
            <button
              type="button"
              onClick={() => setMode('buckets')}
              className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-semibold transition ${
                mode === 'buckets'
                  ? 'bg-white text-slate-900 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              <Calendar className="h-3.5 w-3.5" />
              Time-bucketed
            </button>
            <button
              type="button"
              onClick={() => setMode('tags')}
              className={`inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-semibold transition ${
                mode === 'tags'
                  ? 'bg-white text-slate-900 shadow-2xs'
                  : 'text-slate-600 hover:text-slate-900'
              }`}
            >
              <Tag className="h-3.5 w-3.5" />
              Git Tags
            </button>
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            {TIME_WINDOWS.map((w) => (
              <button
                key={w.months}
                type="button"
                onClick={() => setMonths(w.months)}
                className={`rounded-full px-2.5 py-1 text-xs font-medium transition ${
                  months === w.months
                    ? 'bg-slate-900 text-white'
                    : 'bg-slate-100 text-slate-700 hover:bg-slate-200'
                }`}
              >
                {w.label}
              </button>
            ))}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <label
            className="inline-flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-700"
            title="Opt-in: computes cyclomatic and cognitive complexity for uncached commits"
          >
            <input
              type="checkbox"
              checked={includeComplexity}
              onChange={(e) => setIncludeComplexity(e.target.checked)}
              className="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
            />
            Include Complexity (opt-in)
          </label>

          {report && (
            <span className="inline-flex items-center gap-1.5 text-[11px] text-slate-500">
              <Database className="h-3.5 w-3.5 text-slate-400" />
              {report.points.length} pts ({report.cached_count} cached,{' '}
              {report.computed_count} computed) · {report.duration_ms} ms
            </span>
          )}
        </div>
      </div>

      {/* Live Progress Card */}
      {loading && (
        <div className="flex flex-col gap-2.5 rounded-xl border border-indigo-200 bg-indigo-50/70 px-4 py-3 text-xs text-indigo-950">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2 font-medium">
              <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
              {progress ? (
                <span>
                  Processing commits: <strong>{progress.completed}</strong> of{' '}
                  <strong>{progress.total}</strong> done
                  {progress.total > 0 && (
                    <span className="ml-1.5 text-indigo-700">
                      (
                      {Math.round(
                        (progress.completed / progress.total) * 100,
                      )}
                      %)
                    </span>
                  )}
                </span>
              ) : (
                <span>Sampling Git commit history...</span>
              )}
            </div>

            {progress && (
              <div className="flex items-center gap-3 font-mono text-[11px] text-indigo-800">
                <span>{progress.cachedCount} cached</span>
                <span>{progress.computedCount} computed</span>
                {progress.skippedCount > 0 && (
                  <span className="text-amber-700">
                    {progress.skippedCount} skipped
                  </span>
                )}
              </div>
            )}
          </div>

          {progress && progress.total > 0 && (
            <div className="h-2 w-full overflow-hidden rounded-full bg-indigo-100">
              <div
                className="h-full rounded-full bg-indigo-600 transition-all duration-200"
                style={{
                  width: `${Math.min(100, (progress.completed / progress.total) * 100)}%`,
                }}
              />
            </div>
          )}

          {progress?.current && (
            <div className="flex flex-wrap items-center justify-between gap-2 text-[11px] text-indigo-900">
              <div className="flex min-w-0 items-center gap-2">
                {progress.current.status === 'computing' ? (
                  <span className="rounded bg-indigo-600 px-1.5 py-0.5 text-[10px] font-semibold text-white">
                    Analyzing
                  </span>
                ) : progress.current.status === 'cached' ? (
                  <span className="inline-flex items-center gap-1 rounded bg-slate-200 px-1.5 py-0.5 text-[10px] font-semibold text-slate-700">
                    <CheckCircle2 className="h-3 w-3" />
                    Cached
                  </span>
                ) : progress.current.status === 'skipped' ? (
                  <span className="rounded bg-amber-200 px-1.5 py-0.5 text-[10px] font-semibold text-amber-900">
                    Skipped
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-800">
                    <CheckCircle2 className="h-3 w-3" />
                    Done
                  </span>
                )}
                <span className="font-mono font-semibold">
                  {progress.current.short_sha}
                </span>
                {progress.current.tag && (
                  <span className="rounded bg-white/80 px-1.5 py-0.5 font-mono text-[10px] text-indigo-700">
                    {progress.current.tag}
                  </span>
                )}
                <span className="truncate text-indigo-800">
                  {progress.current.message}
                </span>
              </div>
              {progress.current.index && (
                <span className="shrink-0 font-mono text-indigo-700">
                  #{progress.current.index}/{progress.total}
                </span>
              )}
            </div>
          )}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-rose-900">
          <div className="flex items-start gap-3">
            <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
            <div>
              <h3 className="text-sm font-semibold">
                Historical trends unavailable
              </h3>
              <p className="mt-1 text-sm text-rose-700">{error}</p>
              <button
                type="button"
                onClick={() => setRetryToken((t) => t + 1)}
                className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-rose-700"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                Retry
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Skipped Commits Notice */}
      {!loading && report && report.skipped_count > 0 && !error && (
        <div className="rounded-xl border border-amber-200 bg-amber-50/80 px-4 py-3 text-xs text-amber-900">
          <div className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
            <div>
              <span className="font-semibold">
                Skipped {report.skipped_count}{' '}
                {report.skipped_count === 1 ? 'commit' : 'commits'}
              </span>{' '}
              that could not be loaded as a Go Project (e.g. missing{' '}
              <code className="font-mono">go.mod</code> or unresolvable build).
              {report.skipped_reasons && report.skipped_reasons.length > 0 && (
                <div className="mt-1 font-mono text-[11px] text-amber-800">
                  {report.skipped_reasons.join(' · ')}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Loaded or Streaming Content */}
      {!error && points.length > 0 && first && latest && (
        <>
          {/* Delta KPI Strip */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <DeltaKpiCard
              label="Lines of Code"
              current={latest.metrics.loc.toLocaleString()}
              delta={latest.metrics.loc - first.metrics.loc}
              formatDelta={(d) => `${d >= 0 ? '+' : ''}${d.toLocaleString()}`}
              neutralColor
              hint={`Since ${first.short_sha} (${formatShortDate(first.date)})`}
            />
            <DeltaKpiCard
              label="Avg Distance (D)"
              current={latest.metrics.avg_distance.toFixed(2)}
              delta={latest.metrics.avg_distance - first.metrics.avg_distance}
              formatDelta={(d) => `${d >= 0 ? '+' : ''}${d.toFixed(2)}`}
              lowerIsBetter
              hint="Lower means closer to Main Sequence"
            />
            <DeltaKpiCard
              label="Zone of Pain + Useless"
              current={String(
                latest.metrics.zone_of_pain_count +
                  latest.metrics.zone_of_uselessness_count,
              )}
              delta={
                latest.metrics.zone_of_pain_count +
                latest.metrics.zone_of_uselessness_count -
                (first.metrics.zone_of_pain_count +
                  first.metrics.zone_of_uselessness_count)
              }
              formatDelta={(d) => `${d >= 0 ? '+' : ''}${d}`}
              lowerIsBetter
              hint={`${latest.metrics.zone_of_pain_count} pain · ${latest.metrics.zone_of_uselessness_count} useless`}
            />
            <DeltaKpiCard
              label="Packages & Files"
              current={`${latest.metrics.packages} / ${latest.metrics.source_files}`}
              delta={latest.metrics.source_files - first.metrics.source_files}
              formatDelta={(d) =>
                `${d >= 0 ? '+' : ''}${d} ${Math.abs(d) === 1 ? 'file' : 'files'}`
              }
              neutralColor
              hint={`${latest.metrics.structs} structs · ${latest.metrics.interfaces} interfaces`}
            />
          </div>

          {/* Trend Charts Grid */}
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <TrendChartCard
              title="Code Size & Comments"
              subtitle="Total Go LOC vs. comment lines"
              points={points}
              hoveredIdx={hoveredIdx}
              onHoverIdx={setHoveredIdx}
              series={[
                {
                  id: 'loc',
                  label: 'LOC',
                  color: '#4f46e5',
                  getValue: (p) => p.metrics.loc,
                  formatValue: (v) => v.toLocaleString(),
                },
                {
                  id: 'comments',
                  label: 'Comment Lines',
                  color: '#10b981',
                  getValue: (p) => p.metrics.comments_total_lines,
                  formatValue: (v) => v.toLocaleString(),
                },
              ]}
            />

            <TrendChartCard
              title="Instability Health"
              subtitle="Average Distance from Main Sequence (D)"
              points={points}
              hoveredIdx={hoveredIdx}
              onHoverIdx={setHoveredIdx}
              yDomain={[0, 1]}
              series={[
                {
                  id: 'avg_d',
                  label: 'Avg Distance (D)',
                  color: '#0284c7',
                  getValue: (p) => p.metrics.avg_distance,
                  formatValue: (v) => v.toFixed(2),
                },
              ]}
            />

            <TrendChartCard
              title="Architectural Zones"
              subtitle="Packages in Zone of Pain and Zone of Uselessness"
              points={points}
              hoveredIdx={hoveredIdx}
              onHoverIdx={setHoveredIdx}
              series={[
                {
                  id: 'pain',
                  label: 'Zone of Pain',
                  color: '#e11d48',
                  getValue: (p) => p.metrics.zone_of_pain_count,
                  formatValue: (v) => String(Math.round(v)),
                },
                {
                  id: 'useless',
                  label: 'Zone of Uselessness',
                  color: '#ea580c',
                  getValue: (p) => p.metrics.zone_of_uselessness_count,
                  formatValue: (v) => String(Math.round(v)),
                },
              ]}
            />

            <TrendChartCard
              title="Declarations"
              subtitle="Structs, interfaces, and functions/methods"
              points={points}
              hoveredIdx={hoveredIdx}
              onHoverIdx={setHoveredIdx}
              series={[
                {
                  id: 'structs',
                  label: 'Structs',
                  color: '#6366f1',
                  getValue: (p) => p.metrics.structs,
                  formatValue: (v) => String(Math.round(v)),
                },
                {
                  id: 'interfaces',
                  label: 'Interfaces',
                  color: '#0d9488',
                  getValue: (p) => p.metrics.interfaces,
                  formatValue: (v) => String(Math.round(v)),
                },
                {
                  id: 'funcs',
                  label: 'Funcs + Methods',
                  color: '#f59e0b',
                  getValue: (p) => p.metrics.functions + p.metrics.methods,
                  formatValue: (v) => String(Math.round(v)),
                },
              ]}
            />

            {includeComplexity && (
              <div className="lg:col-span-2">
                {hasComplexityData ? (
                  <TrendChartCard
                    title="Average Complexity (Opt-in)"
                    subtitle="Cyclomatic (gocyclo) and Cognitive (gocognit) averages"
                    points={points}
                    hoveredIdx={hoveredIdx}
                    onHoverIdx={setHoveredIdx}
                    series={[
                      {
                        id: 'cyclo',
                        label: 'Cyclomatic Avg',
                        color: '#d97706',
                        getValue: (p) =>
                          p.metrics.cyclomatic_complexity_avg ?? null,
                        formatValue: (v) => v.toFixed(2),
                      },
                      {
                        id: 'cognit',
                        label: 'Cognitive Avg',
                        color: '#7c3aed',
                        getValue: (p) =>
                          p.metrics.cognitive_complexity_avg ?? null,
                        formatValue: (v) => v.toFixed(2),
                      },
                    ]}
                  />
                ) : (
                  <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs text-amber-900">
                    Complexity metrics could not be measured because{' '}
                    <code className="font-mono">gocyclo</code> /{' '}
                    <code className="font-mono">gocognit</code> were not found
                    on <code className="font-mono">PATH</code>.
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Sampled Snapshots Table */}
          <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
            <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-900">
              <GitCommitHorizontal className="h-4 w-4 text-indigo-600" />
              Sampled Commit Snapshots ({points.length})
            </h2>
            <div className="overflow-auto rounded-lg border border-slate-200">
              <table className="w-full border-collapse text-left text-xs">
                <thead className="border-b border-slate-200 bg-slate-50 text-[11px] font-semibold text-slate-600 uppercase">
                  <tr>
                    <th className="px-3 py-2">Commit</th>
                    <th className="px-3 py-2">Date</th>
                    <th className="px-3 py-2">Message</th>
                    <th className="px-3 py-2 text-right">LOC</th>
                    <th className="px-3 py-2 text-right">Comment %</th>
                    <th className="px-3 py-2 text-right">Avg D</th>
                    <th className="px-3 py-2 text-right">Pain</th>
                    <th className="px-3 py-2 text-right">Useless</th>
                    <th className="px-3 py-2 text-right">Source</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {[...points].reverse().map((p, revIdx) => {
                    const idx = points.length - 1 - revIdx
                    const isHovered = hoveredIdx === idx
                    return (
                      <tr
                        key={p.commit_sha}
                        onMouseEnter={() => setHoveredIdx(idx)}
                        onMouseLeave={() => setHoveredIdx(null)}
                        className={`transition-colors ${
                          isHovered ? 'bg-indigo-50/90' : 'hover:bg-slate-50'
                        }`}
                      >
                        <td className="px-3 py-2 whitespace-nowrap">
                          <span className="font-mono font-semibold text-slate-900">
                            {p.short_sha}
                          </span>
                          {p.tag && (
                            <span className="ml-1.5 rounded bg-indigo-50 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-indigo-700 ring-1 ring-indigo-600/20">
                              {p.tag}
                            </span>
                          )}
                        </td>
                        <td className="px-3 py-2 whitespace-nowrap text-slate-600">
                          {formatShortDate(p.date)}
                        </td>
                        <td
                          className="max-w-[260px] truncate px-3 py-2 text-slate-800"
                          title={p.message}
                        >
                          {p.message}
                        </td>
                        <td className="px-3 py-2 text-right font-mono font-semibold text-slate-900 tabular-nums">
                          {p.metrics.loc.toLocaleString()}
                        </td>
                        <td className="px-3 py-2 text-right font-mono text-slate-700 tabular-nums">
                          {p.metrics.comments_ratio.toFixed(1)}%
                        </td>
                        <td className="px-3 py-2 text-right font-mono text-slate-700 tabular-nums">
                          {p.metrics.avg_distance.toFixed(2)}
                        </td>
                        <td className="px-3 py-2 text-right font-mono text-rose-700 tabular-nums">
                          {p.metrics.zone_of_pain_count}
                        </td>
                        <td className="px-3 py-2 text-right font-mono text-orange-700 tabular-nums">
                          {p.metrics.zone_of_uselessness_count}
                        </td>
                        <td className="px-3 py-2 text-right whitespace-nowrap">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${
                              p.cached
                                ? 'bg-slate-100 text-slate-600'
                                : 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-600/20'
                            }`}
                          >
                            {p.cached ? 'Cached' : 'Fresh'}
                          </span>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}

      {report && !error && points.length === 0 && !loading && (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white py-14 text-center">
          <TrendingUp className="h-8 w-8 text-slate-400" />
          <p className="mt-2 text-sm font-semibold text-slate-800">
            No buildable snapshots found in this range
          </p>
          <p className="mt-1 text-xs text-slate-500">
            Try expanding the time window or switching between Time-bucketed
            and Git Tags mode.
          </p>
        </div>
      )}
    </div>
  )
}

function DeltaKpiCard({
  label,
  current,
  delta,
  formatDelta,
  lowerIsBetter,
  neutralColor,
  hint,
}: {
  label: string
  current: string
  delta: number
  formatDelta: (d: number) => string
  lowerIsBetter?: boolean
  neutralColor?: boolean
  hint: string
}) {
  let badgeClass = 'bg-slate-100 text-slate-700'
  if (!neutralColor && Math.abs(delta) > 0.0001) {
    const good = lowerIsBetter ? delta < 0 : delta > 0
    badgeClass = good
      ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-600/20'
      : 'bg-rose-50 text-rose-700 ring-1 ring-rose-600/20'
  }

  return (
    <div className="rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-xs">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-slate-500">{label}</span>
        <span
          className={`rounded-full px-2 py-0.5 font-mono text-[10px] font-semibold ${badgeClass}`}
        >
          {formatDelta(delta)}
        </span>
      </div>
      <div className="mt-1.5 font-mono text-2xl font-bold tracking-tight text-slate-900">
        {current}
      </div>
      <div className="mt-0.5 text-[11px] text-slate-500">{hint}</div>
    </div>
  )
}

const CHART_W = 540
const CHART_H = 220
const PAD_L = 48
const PAD_R = 16
const PAD_T = 16
const PAD_B = 28
const PLOT_W = CHART_W - PAD_L - PAD_R
const PLOT_H = CHART_H - PAD_T - PAD_B

function TrendChartCard({
  title,
  subtitle,
  points,
  series,
  hoveredIdx,
  onHoverIdx,
  yDomain,
}: {
  title: string
  subtitle: string
  points: CommitSnapshot[]
  series: SeriesConfig[]
  hoveredIdx: number | null
  onHoverIdx: (idx: number | null) => void
  yDomain?: [number, number]
}) {
  const { minVal, maxVal, ticks } = useMemo(() => {
    if (yDomain) {
      const [lo, hi] = yDomain
      return { minVal: lo, maxVal: hi, ticks: [lo, (lo + hi) / 2, hi] }
    }
    let min = Number.POSITIVE_INFINITY
    let max = Number.NEGATIVE_INFINITY
    for (const s of series) {
      for (const p of points) {
        const v = s.getValue(p)
        if (v !== null && Number.isFinite(v)) {
          if (v < min) min = v
          if (v > max) max = v
        }
      }
    }
    if (!Number.isFinite(min) || !Number.isFinite(max)) {
      return { minVal: 0, maxVal: 1, ticks: [0, 0.5, 1] }
    }
    if (min === max) {
      const pad = min === 0 ? 1 : Math.max(1, Math.abs(min) * 0.2)
      const lo = Math.max(0, min - pad)
      const hi = max + pad
      return { minVal: lo, maxVal: hi, ticks: [lo, (lo + hi) / 2, hi] }
    }
    const lo = Math.max(0, min - (max - min) * 0.1)
    const hi = max + (max - min) * 0.1
    return { minVal: lo, maxVal: hi, ticks: [lo, (lo + hi) / 2, hi] }
  }, [points, series, yDomain])

  const getX = (idx: number) => {
    if (points.length <= 1) return PAD_L + PLOT_W / 2
    return PAD_L + (idx / (points.length - 1)) * PLOT_W
  }

  const getY = (val: number) => {
    const ratio = (val - minVal) / (maxVal - minVal || 1)
    return PAD_T + (1 - Math.max(0, Math.min(1, ratio))) * PLOT_H
  }

  const hoveredPoint =
    hoveredIdx !== null && hoveredIdx < points.length
      ? points[hoveredIdx]
      : null

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-slate-900">{title}</h3>
          <p className="text-xs text-slate-500">{subtitle}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-[11px] text-slate-600">
          {series.map((s) => (
            <span key={s.id} className="inline-flex items-center gap-1.5">
              <span
                className="h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: s.color }}
              />
              {s.label}
            </span>
          ))}
        </div>
      </div>

      <div className="relative">
        <svg
          viewBox={`0 0 ${CHART_W} ${CHART_H}`}
          className="h-auto w-full overflow-visible select-none"
          onMouseLeave={() => onHoverIdx(null)}
        >
          {/* Horizontal gridlines */}
          {ticks.map((t, i) => {
            const y = getY(t)
            const formatted =
              t >= 1000
                ? `${(t / 1000).toFixed(1)}k`
                : Number.isInteger(t)
                  ? String(t)
                  : t.toFixed(2)
            return (
              <g key={i}>
                <line
                  x1={PAD_L}
                  y1={y}
                  x2={PAD_L + PLOT_W}
                  y2={y}
                  stroke="#e2e8f0"
                  strokeDasharray="3,3"
                />
                <text
                  x={PAD_L - 8}
                  y={y + 3.5}
                  textAnchor="end"
                  className="fill-slate-400 font-mono text-[10px]"
                >
                  {formatted}
                </text>
              </g>
            )
          })}

          {/* Crosshair */}
          {hoveredIdx !== null && hoveredIdx < points.length && (
            <line
              x1={getX(hoveredIdx)}
              y1={PAD_T}
              x2={getX(hoveredIdx)}
              y2={PAD_T + PLOT_H}
              stroke="#6366f1"
              strokeWidth={1.25}
              strokeDasharray="3,3"
            />
          )}

          {/* Series paths & points */}
          {series.map((s) => {
            const coords: Array<{ idx: number; x: number; y: number }> = []
            points.forEach((p, idx) => {
              const val = s.getValue(p)
              if (val !== null && Number.isFinite(val)) {
                coords.push({ idx, x: getX(idx), y: getY(val) })
              }
            })
            const pathD = coords
              .map((c, i) => `${i === 0 ? 'M' : 'L'} ${c.x} ${c.y}`)
              .join(' ')

            return (
              <g key={s.id}>
                {coords.length > 1 && (
                  <path
                    d={pathD}
                    fill="none"
                    stroke={s.color}
                    strokeWidth={2.25}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                )}
                {coords.map((c) => {
                  const isHovered = hoveredIdx === c.idx
                  return (
                    <circle
                      key={c.idx}
                      cx={c.x}
                      cy={c.y}
                      r={isHovered ? 5 : 3.25}
                      fill={isHovered ? '#ffffff' : s.color}
                      stroke={s.color}
                      strokeWidth={isHovered ? 2.5 : 1.5}
                    />
                  )
                })}
              </g>
            )
          })}

          {/* X-axis end labels */}
          {points.length > 0 && (
            <>
              <text
                x={getX(0)}
                y={CHART_H - 6}
                textAnchor={points.length === 1 ? 'middle' : 'start'}
                className="fill-slate-400 font-mono text-[10px]"
              >
                {points[0].tag || formatShortDate(points[0].date)}
              </text>
              {points.length > 1 && (
                <text
                  x={getX(points.length - 1)}
                  y={CHART_H - 6}
                  textAnchor="end"
                  className="fill-slate-400 font-mono text-[10px]"
                >
                  {points[points.length - 1].tag ||
                    formatShortDate(points[points.length - 1].date)}
                </text>
              )}
            </>
          )}

          {/* Invisible hit columns for smooth hover */}
          {points.map((p, idx) => {
            const x = getX(idx)
            const step =
              points.length <= 1 ? PLOT_W : PLOT_W / (points.length - 1)
            const left = idx === 0 ? PAD_L : x - step / 2
            const right =
              idx === points.length - 1 ? PAD_L + PLOT_W : x + step / 2
            return (
              <rect
                key={p.commit_sha}
                x={left}
                y={PAD_T}
                width={Math.max(1, right - left)}
                height={PLOT_H}
                fill="transparent"
                className="cursor-crosshair"
                onMouseEnter={() => onHoverIdx(idx)}
              />
            )
          })}
        </svg>

        {/* Fixed-height Hover Readout (prevents layout shift / flickering) */}
        <div
          className={`mt-2 flex h-8 items-center justify-between gap-2 overflow-hidden rounded-lg px-3 text-[11px] transition-colors ${
            hoveredPoint && hoveredIdx !== null
              ? 'bg-slate-900 text-slate-100'
              : 'border border-slate-100 bg-slate-50 text-slate-400'
          }`}
        >
          {hoveredPoint && hoveredIdx !== null ? (
            <>
              <div className="flex min-w-0 items-center gap-2">
                <span className="shrink-0 font-mono font-semibold text-indigo-300">
                  {hoveredPoint.short_sha}
                </span>
                {hoveredPoint.tag && (
                  <span className="shrink-0 rounded bg-white/10 px-1.5 py-0.5 font-mono text-[10px]">
                    {hoveredPoint.tag}
                  </span>
                )}
                <span className="shrink-0 text-slate-400">
                  {formatShortDate(hoveredPoint.date)}
                </span>
                <span className="truncate text-slate-300">
                  {hoveredPoint.message}
                </span>
              </div>
              <div className="flex shrink-0 items-center gap-3 font-mono">
                {series.map((s) => {
                  const v = s.getValue(hoveredPoint)
                  const txt =
                    v === null
                      ? 'N/A'
                      : s.formatValue
                        ? s.formatValue(v)
                        : String(v)
                  return (
                    <span key={s.id} className="inline-flex items-center gap-1">
                      <span
                        className="h-2 w-2 rounded-full"
                        style={{ backgroundColor: s.color }}
                      />
                      <span>{txt}</span>
                    </span>
                  )
                })}
              </div>
            </>
          ) : (
            <span className="truncate">
              Hover chart points to inspect commit metrics
            </span>
          )}
        </div>
      </div>
    </div>
  )
}
