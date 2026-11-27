import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertCircle,
  Calendar,
  FileCode2,
  Flame,
  Folder,
  GitCommitHorizontal,
  Loader2,
  RefreshCw,
  Search,
  ShieldAlert,
  Users,
  X,
} from 'lucide-react'
import { fetchHotspots, fetchVCS } from '../api/client'
import type {
  ContributorSummary,
  FileHotspot,
  ModificationsInfo,
  ProjectInfo,
  VCSAnalysisInfo,
} from '../types/api'
import { shortenPackageName } from '../utils/packages'

interface VCSViewProps {
  project: ProjectInfo
  refreshSignal: number
}

type BusFactorRisk = 'high' | 'moderate' | 'healthy'

const RISK_META: Record<
  BusFactorRisk,
  { label: string; badgeClass: string; description: string }
> = {
  high: {
    label: 'High Risk',
    badgeClass: 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-600/20',
    description: 'Only 1 contributor modified this source.',
  },
  moderate: {
    label: 'Moderate',
    badgeClass: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-600/20',
    description: 'Fewer than 3 contributors in a Repository with > 4 contributors.',
  },
  healthy: {
    label: 'Healthy',
    badgeClass: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
    description: 'Shared ownership across multiple contributors.',
  },
}

const AUTHOR_PALETTE = [
  '#4f46e5',
  '#0284c7',
  '#059669',
  '#d97706',
  '#e11d48',
  '#7c3aed',
  '#0d9488',
  '#ea580c',
  '#2563eb',
  '#db2777',
  '#65a30d',
  '#0891b2',
]

const PRESETS = [
  { label: '1 mo', months: 1 },
  { label: '3 mo', months: 3 },
  { label: '6 mo', months: 6 },
  { label: '12 mo', months: 12 },
  { label: 'All time', months: 0 },
]

function classifyBusFactorRisk(
  authorsCount: number,
  totalRepoAuthors: number,
): BusFactorRisk {
  if (authorsCount <= 1) return 'high'
  if (totalRepoAuthors > 4 && authorsCount < 3) return 'moderate'
  return 'healthy'
}

export function VCSView({ project, refreshSignal }: VCSViewProps) {
  const [months, setMonths] = useState(6)
  const [customMonths, setCustomMonths] = useState('6')
  const [vcs, setVcs] = useState<VCSAnalysisInfo | null>(null)
  const [durationMs, setDurationMs] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [scopeTab, setScopeTab] = useState<'paths' | 'files'>('paths')
  const [riskFilter, setRiskFilter] = useState<BusFactorRisk | 'all'>('all')
  const [searchQuery, setSearchQuery] = useState('')

  const [hotspots, setHotspots] = useState<FileHotspot[] | null>(null)
  const [hotspotsLoading, setHotspotsLoading] = useState(false)
  const [hotspotSearch, setHotspotSearch] = useState('')

  const lastRefreshRef = useRef(refreshSignal)

  const loadData = useCallback((targetMonths: number, signal?: AbortSignal, refresh?: boolean) => {
    setLoading(true)
    setError(null)
    setHotspotsLoading(true)
    fetchVCS(targetMonths, signal, refresh)
      .then((res) => {
        if (signal?.aborted) return
        setVcs(res.vcs)
        setDurationMs(res.duration_ms ?? null)
        setLoading(false)

        return fetchHotspots(targetMonths, signal, refresh)
          .then((hRes) => {
            if (signal?.aborted) return
            setHotspots(hRes.hotspots)
          })
          .catch(() => {
            if (signal?.aborted) return
            setHotspots(null)
          })
      })
      .catch((err: unknown) => {
        if (signal?.aborted) return
        setError(
          err instanceof Error ? err.message : 'Failed to analyze Git history',
        )
      })
      .finally(() => {
        if (!signal?.aborted) {
          setLoading(false)
          setHotspotsLoading(false)
        }
      })
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    const isRefresh = lastRefreshRef.current !== refreshSignal
    lastRefreshRef.current = refreshSignal
    loadData(months, controller.signal, isRefresh)
    return () => controller.abort()
  }, [project.path, months, refreshSignal, loadData])

  const sortedAuthors = useMemo(() => {
    if (!vcs) return []
    return [...vcs.AuthorsInfo.AuthorDetails].sort(
      (a, b) => b.ModifiedLines - a.ModifiedLines,
    )
  }, [vcs])

  const colorMap = useMemo(() => {
    const map = new Map<string, string>()
    sortedAuthors.forEach((a, idx) => {
      map.set(a.Name, AUTHOR_PALETTE[idx % AUTHOR_PALETTE.length])
    })
    return map
  }, [sortedAuthors])

  const totalContributors = sortedAuthors.length
  const rawItems = useMemo(() => {
    if (!vcs) return []
    return scopeTab === 'paths'
      ? vcs.ModificationsInfo.PathModifications
      : vcs.ModificationsInfo.FileModifications
  }, [vcs, scopeTab])

  const riskCounts = useMemo(() => {
    const counts: Record<BusFactorRisk, number> = {
      high: 0,
      moderate: 0,
      healthy: 0,
    }
    for (const item of rawItems) {
      counts[classifyBusFactorRisk(item.AuthorsSummary.length, totalContributors)]++
    }
    return counts
  }, [rawItems, totalContributors])

  const filteredItems = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    return rawItems.filter((item) => {
      const risk = classifyBusFactorRisk(
        item.AuthorsSummary.length,
        totalContributors,
      )
      if (riskFilter !== 'all' && risk !== riskFilter) return false
      if (q && !item.Source.toLowerCase().includes(q)) return false
      return true
    })
  }, [rawItems, riskFilter, searchQuery, totalContributors])

  const filteredHotspots = useMemo(() => {
    if (!hotspots) return []
    const q = hotspotSearch.trim().toLowerCase()
    const list = q
      ? hotspots.filter(
          (h) =>
            h.file.toLowerCase().includes(q) ||
            h.package.toLowerCase().includes(q),
        )
      : hotspots
    return list.slice(0, 50)
  }, [hotspots, hotspotSearch])

  const applyCustomMonths = (e: React.FormEvent) => {
    e.preventDefault()
    const parsed = Number.parseInt(customMonths, 10)
    if (!Number.isNaN(parsed) && parsed >= 0) {
      setMonths(parsed)
    }
  }

  return (
    <div className="flex flex-col gap-5">
      {/* Time Window Toolbar */}
      <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-3.5 shadow-xs sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-600">
            <Calendar className="h-3.5 w-3.5 text-slate-400" />
            Time Window:
          </span>
          {PRESETS.map((p) => (
            <button
              key={p.months}
              type="button"
              onClick={() => {
                setMonths(p.months)
                setCustomMonths(String(p.months))
              }}
              className={`rounded-full px-2.5 py-1 text-xs font-medium transition ${
                months === p.months
                  ? 'bg-slate-900 text-white'
                  : 'bg-slate-100 text-slate-700 hover:bg-slate-200'
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>

        <form onSubmit={applyCustomMonths} className="flex items-center gap-2">
          <label
            htmlFor="vcs-months-input"
            className="text-xs text-slate-500"
          >
            Custom months:
          </label>
          <input
            id="vcs-months-input"
            type="number"
            min={0}
            value={customMonths}
            onChange={(e) => setCustomMonths(e.target.value)}
            className="w-16 rounded-lg border border-slate-200 px-2 py-1 font-mono text-xs text-slate-900 focus:border-indigo-500 focus:outline-none"
          />
          <button
            type="submit"
            className="rounded-lg bg-indigo-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-indigo-700"
          >
            Apply
          </button>
          {durationMs !== null && (
            <span className="text-[11px] text-slate-400">{durationMs} ms</span>
          )}
        </form>
      </div>

      {/* Loading */}
      {loading && !vcs && (
        <div className="flex flex-col gap-4">
          <div className="flex items-center gap-2.5 rounded-xl border border-indigo-200 bg-indigo-50/70 px-4 py-3 text-sm font-medium text-indigo-900">
            <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
            Inspecting Repository commit history and contributor activity...
          </div>
          <div className="h-96 animate-pulse rounded-xl border border-slate-200 bg-white" />
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-rose-900">
          <div className="flex items-start gap-3">
            <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
            <div>
              <h3 className="text-sm font-semibold">VCS analysis unavailable</h3>
              <p className="mt-1 text-sm text-rose-700">{error}</p>
              <button
                type="button"
                onClick={() => loadData(months)}
                className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-rose-700"
              >
                <RefreshCw className="h-3.5 w-3.5" />
                Retry
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Loaded */}
      {vcs && !error && (
        <>
          {loading && (
            <div className="flex items-center gap-2 rounded-lg border border-indigo-200 bg-indigo-50 px-3.5 py-2 text-xs font-medium text-indigo-800">
              <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
              Updating VCS window...
            </div>
          )}

          {/* KPI Strip */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <VcsKpiCard
              icon={<Users className="h-4 w-4 text-indigo-600" />}
              label="Contributors"
              value={String(vcs.AuthorsInfo.TotalAuthors)}
              hint={months === 0 ? 'All time' : `Last ${months} months`}
            />
            <VcsKpiCard
              icon={<GitCommitHorizontal className="h-4 w-4 text-sky-600" />}
              label="Commits"
              value={vcs.AuthorsInfo.TotalCommits.toLocaleString()}
              hint="Commits in window"
            />
            <VcsKpiCard
              icon={<FileCode2 className="h-4 w-4 text-emerald-600" />}
              label="Modified Lines"
              value={vcs.AuthorsInfo.TotalModifications.toLocaleString()}
              hint="Additions + deletions"
            />
            <VcsKpiCard
              icon={<ShieldAlert className="h-4 w-4 text-rose-600" />}
              label="High Bus Factor Risk"
              value={String(riskCounts.high)}
              hint={`${scopeTab === 'paths' ? 'Paths' : 'Files'} with 1 contributor`}
            />
          </div>

          {vcs.AuthorsInfo.TotalCommits === 0 ? (
            <div className="rounded-xl border border-slate-200 bg-white p-10 text-center text-sm text-slate-500">
              No commits found in the selected time window. Try expanding to{' '}
              <strong>12 mo</strong> or <strong>All time</strong>.
            </div>
          ) : (
            <div className="grid grid-cols-1 items-start gap-5 xl:grid-cols-12">
              {/* Modifications & Bus Factor Table */}
              <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-xs xl:col-span-8">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-1 rounded-lg bg-slate-100 p-1">
                    <button
                      type="button"
                      onClick={() => setScopeTab('paths')}
                      className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-semibold transition ${
                        scopeTab === 'paths'
                          ? 'bg-white text-slate-900 shadow-2xs'
                          : 'text-slate-600 hover:text-slate-900'
                      }`}
                    >
                      <Folder className="h-3.5 w-3.5" />
                      By Path ({vcs.ModificationsInfo.PathModifications.length})
                    </button>
                    <button
                      type="button"
                      onClick={() => setScopeTab('files')}
                      className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-semibold transition ${
                        scopeTab === 'files'
                          ? 'bg-white text-slate-900 shadow-2xs'
                          : 'text-slate-600 hover:text-slate-900'
                      }`}
                    >
                      <FileCode2 className="h-3.5 w-3.5" />
                      By File ({vcs.ModificationsInfo.FileModifications.length})
                    </button>
                  </div>

                  <div className="relative flex-1 sm:max-w-xs">
                    <Search className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
                    <input
                      type="text"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder={`Filter ${scopeTab}...`}
                      className="w-full rounded-lg border border-slate-200 bg-slate-50 py-1.5 pr-7 pl-8 text-xs text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:bg-white focus:outline-none"
                    />
                    {searchQuery && (
                      <button
                        type="button"
                        onClick={() => setSearchQuery('')}
                        className="absolute top-1/2 right-2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
                      >
                        <X className="h-3.5 w-3.5" />
                      </button>
                    )}
                  </div>
                </div>

                {/* Risk Filter Pills */}
                <div className="flex flex-wrap items-center gap-1.5">
                  <button
                    type="button"
                    onClick={() => setRiskFilter('all')}
                    className={`rounded-full px-2.5 py-1 text-[11px] font-medium transition ${
                      riskFilter === 'all'
                        ? 'bg-slate-900 text-white'
                        : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                    }`}
                  >
                    All ({rawItems.length})
                  </button>
                  {(['high', 'moderate', 'healthy'] as const).map((risk) => (
                    <button
                      key={risk}
                      type="button"
                      onClick={() =>
                        setRiskFilter((curr) => (curr === risk ? 'all' : risk))
                      }
                      className={`rounded-full px-2.5 py-1 text-[11px] font-medium transition ${
                        riskFilter === risk
                          ? 'bg-slate-900 text-white'
                          : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                      }`}
                    >
                      {RISK_META[risk].label} ({riskCounts[risk]})
                    </button>
                  ))}
                </div>

                <div className="max-h-[540px] overflow-auto rounded-lg border border-slate-200">
                  <table className="w-full border-collapse text-left text-xs">
                    <thead className="sticky top-0 z-10 border-b border-slate-200 bg-slate-50 text-[11px] font-semibold text-slate-600 uppercase">
                      <tr>
                        <th className="px-3 py-2.5">
                          {scopeTab === 'paths' ? 'Path' : 'File'}
                        </th>
                        <th className="px-3 py-2.5">Bus Factor Risk</th>
                        <th className="px-3 py-2.5 text-right">Changes</th>
                        <th className="min-w-[200px] px-3 py-2.5">
                          Contributor Share
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {filteredItems.length === 0 ? (
                        <tr>
                          <td
                            colSpan={4}
                            className="px-4 py-10 text-center text-slate-500"
                          >
                            No matching entries.
                          </td>
                        </tr>
                      ) : (
                        filteredItems.map((item) => (
                          <ModificationRow
                            key={item.Source}
                            item={item}
                            totalContributors={totalContributors}
                            colorMap={colorMap}
                          />
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Contributors Breakdown */}
              <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-xs xl:col-span-4">
                <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-900">
                  <Users className="h-4 w-4 text-indigo-600" />
                  Contributors ({sortedAuthors.length})
                </h2>

                <ul className="max-h-[560px] space-y-2.5 overflow-auto pr-1">
                  {sortedAuthors.map((author) => {
                    const pct =
                      vcs.AuthorsInfo.TotalModifications > 0
                        ? (author.ModifiedLines /
                            vcs.AuthorsInfo.TotalModifications) *
                          100
                        : 0
                    const color = colorMap.get(author.Name) ?? '#64748b'

                    return (
                      <li
                        key={author.Name}
                        className="rounded-lg border border-slate-100 bg-slate-50/70 p-2.5"
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="flex min-w-0 items-center gap-2">
                            <span
                              className="h-2.5 w-2.5 shrink-0 rounded-full"
                              style={{ backgroundColor: color }}
                            />
                            <span className="truncate text-xs font-semibold text-slate-900">
                              {author.Name}
                            </span>
                          </span>
                          <span className="shrink-0 font-mono text-xs font-bold text-slate-800">
                            {pct.toFixed(1)}%
                          </span>
                        </div>
                        <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-slate-200">
                          <div
                            className="h-full rounded-full"
                            style={{
                              width: `${Math.max(pct, 2)}%`,
                              backgroundColor: color,
                            }}
                          />
                        </div>
                        <div className="mt-1.5 flex items-center justify-between text-[11px] text-slate-500">
                          <span>{author.Commits} commits</span>
                          <span className="font-mono">
                            {author.ModifiedLines.toLocaleString()} lines
                          </span>
                        </div>
                      </li>
                    )
                  })}
                </ul>
              </div>
            </div>
          )}

          {/* Code Hotspots (Churn x Complexity) */}
          {vcs.AuthorsInfo.TotalCommits > 0 && (
            <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
              <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-900">
                    <Flame className="h-4 w-4 text-orange-600" />
                    Code Hotspots (Churn &times; Complexity)
                    {hotspots && (
                      <span className="rounded-full bg-slate-100 px-2 py-0.5 font-mono text-[11px] font-medium text-slate-600">
                        {hotspots.length} files
                      </span>
                    )}
                  </h2>
                  <p className="mt-0.5 text-xs text-slate-500">
                    Non-generated production Go files ranked by combining recent change activity (commits &amp; modified lines) with complexity and size.
                  </p>
                </div>

                <div className="relative w-full sm:max-w-xs">
                  <Search className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
                  <input
                    type="text"
                    value={hotspotSearch}
                    onChange={(e) => setHotspotSearch(e.target.value)}
                    placeholder="Filter hotspot files..."
                    className="w-full rounded-lg border border-slate-200 bg-slate-50 py-1.5 pr-7 pl-8 text-xs text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:bg-white focus:outline-none"
                  />
                  {hotspotSearch && (
                    <button
                      type="button"
                      onClick={() => setHotspotSearch('')}
                      className="absolute top-1/2 right-2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  )}
                </div>
              </div>

              {hotspotsLoading && !hotspots ? (
                <div className="flex items-center gap-2 py-8 text-xs text-slate-500">
                  <Loader2 className="h-4 w-4 animate-spin text-orange-600" />
                  Correlating git churn with per-file complexity...
                </div>
              ) : filteredHotspots.length === 0 ? (
                <div className="rounded-lg border border-slate-100 bg-slate-50 px-4 py-8 text-center text-xs text-slate-500">
                  No production Go files matched churn activity in this time window.
                </div>
              ) : (
                <div className="max-h-[420px] overflow-auto rounded-lg border border-slate-200">
                  <table className="w-full border-collapse text-left text-xs">
                    <thead className="sticky top-0 z-10 border-b border-slate-200 bg-slate-50 text-[11px] font-semibold text-slate-600 uppercase">
                      <tr>
                        <th className="px-3 py-2.5">File</th>
                        <th className="px-3 py-2.5">Hotspot Score</th>
                        <th className="px-3 py-2.5 text-right">Commits</th>
                        <th className="px-3 py-2.5 text-right">Churn</th>
                        <th className="px-3 py-2.5 text-right">Cognitive</th>
                        <th className="px-3 py-2.5 text-right">Cyclomatic</th>
                        <th className="px-3 py-2.5 text-right">LOC</th>
                        <th className="px-3 py-2.5 text-right">Authors</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                      {filteredHotspots.map((h) => {
                        const scoreBadge =
                          h.score >= 70
                            ? 'bg-rose-50 text-rose-700 ring-rose-600/20'
                            : h.score >= 40
                              ? 'bg-amber-50 text-amber-700 ring-amber-600/20'
                              : 'bg-slate-100 text-slate-700 ring-slate-500/20'
                        const barColor =
                          h.score >= 70
                            ? '#e11d48'
                            : h.score >= 40
                              ? '#d97706'
                              : '#64748b'
                        return (
                          <tr key={h.file} className="hover:bg-slate-50/70">
                            <td className="max-w-[280px] px-3 py-2.5">
                              <div
                                className="truncate font-mono text-xs font-medium text-slate-900"
                                title={h.file}
                              >
                                {h.file}
                              </div>
                              <div
                                className="truncate font-mono text-[10px] text-slate-400"
                                title={h.package}
                              >
                                {shortenPackageName(h.package, project.package)}
                              </div>
                            </td>
                            <td className="px-3 py-2.5 whitespace-nowrap">
                              <div className="flex items-center gap-2">
                                <span
                                  className={`inline-flex min-w-[46px] items-center justify-center rounded-full px-2 py-0.5 font-mono text-[11px] font-semibold ring-1 ring-inset ${scoreBadge}`}
                                >
                                  {h.score.toFixed(0)}
                                </span>
                                <div className="hidden h-1.5 w-16 overflow-hidden rounded-full bg-slate-100 sm:block">
                                  <div
                                    className="h-full rounded-full"
                                    style={{
                                      width: `${Math.min(100, Math.max(h.score, 4))}%`,
                                      backgroundColor: barColor,
                                    }}
                                  />
                                </div>
                              </div>
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono text-slate-800 tabular-nums">
                              {h.commits}
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono text-slate-800 tabular-nums">
                              {h.churn.toLocaleString()}
                            </td>
                            <td
                              className="px-3 py-2.5 text-right font-mono text-slate-800 tabular-nums"
                              title={`Max function cognitive complexity: ${h.max_cognitive}`}
                            >
                              {h.cognitive_complexity}
                              <span className="ml-1 text-[10px] text-slate-400">
                                (max {h.max_cognitive})
                              </span>
                            </td>
                            <td
                              className="px-3 py-2.5 text-right font-mono text-slate-800 tabular-nums"
                              title={`Max function cyclomatic complexity: ${h.max_cyclomatic}`}
                            >
                              {h.cyclomatic_complexity}
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono text-slate-600 tabular-nums">
                              {h.loc.toLocaleString()}
                            </td>
                            <td className="px-3 py-2.5 text-right font-mono tabular-nums">
                              <span
                                className={
                                  h.contributors <= 1
                                    ? 'font-semibold text-rose-600'
                                    : 'text-slate-600'
                                }
                              >
                                {h.contributors}
                              </span>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}

function ModificationRow({
  item,
  totalContributors,
  colorMap,
}: {
  item: ModificationsInfo
  totalContributors: number
  colorMap: Map<string, string>
}) {
  const [hoveredAuthor, setHoveredAuthor] = useState<ContributorSummary | null>(
    null,
  )
  const risk = classifyBusFactorRisk(
    item.AuthorsSummary.length,
    totalContributors,
  )
  const riskMeta = RISK_META[risk]
  const sortedSummary = useMemo(
    () =>
      [...item.AuthorsSummary].sort(
        (a, b) => b.Modifications - a.Modifications,
      ),
    [item.AuthorsSummary],
  )

  return (
    <tr className="hover:bg-slate-50/70">
      <td className="max-w-[260px] px-3 py-2.5">
        <div
          className="truncate font-mono text-xs font-medium text-slate-900"
          title={item.Source}
        >
          {item.Source}
        </div>
        <div className="text-[10px] text-slate-400">
          {item.AuthorsSummary.length}{' '}
          {item.AuthorsSummary.length === 1 ? 'contributor' : 'contributors'}
        </div>
      </td>
      <td className="px-3 py-2.5 whitespace-nowrap">
        <span
          className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${riskMeta.badgeClass}`}
          title={riskMeta.description}
        >
          {riskMeta.label}
        </span>
      </td>
      <td className="px-3 py-2.5 text-right font-mono font-semibold text-slate-800 tabular-nums">
        {item.TotalModifications.toLocaleString()}
      </td>
      <td className="relative px-3 py-2.5">
        <div className="flex h-3.5 w-full overflow-hidden rounded-full bg-slate-100">
          {sortedSummary.map((summary) => {
            const pct =
              item.TotalModifications > 0
                ? (summary.Modifications / item.TotalModifications) * 100
                : 0
            const color = colorMap.get(summary.Name) ?? '#64748b'
            return (
              <div
                key={summary.Name}
                onMouseEnter={() => setHoveredAuthor(summary)}
                onMouseLeave={() => setHoveredAuthor(null)}
                style={{
                  width: `${pct}%`,
                  backgroundColor: color,
                }}
                className="h-full cursor-pointer transition-opacity hover:opacity-80"
              />
            )
          })}
        </div>
        {hoveredAuthor && (
          <div className="pointer-events-none absolute right-3 bottom-full z-20 mb-1 rounded-lg bg-slate-900 px-2.5 py-1.5 text-[11px] whitespace-nowrap text-white shadow-lg">
            <span className="font-semibold">{hoveredAuthor.Name}</span>:{' '}
            <span className="font-mono">
              {hoveredAuthor.Modifications.toLocaleString()} lines (
              {item.TotalModifications > 0
                ? (
                    (hoveredAuthor.Modifications / item.TotalModifications) *
                    100
                  ).toFixed(1)
                : '0.0'}
              %)
            </span>
          </div>
        )}
      </td>
    </tr>
  )
}

function VcsKpiCard({
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
