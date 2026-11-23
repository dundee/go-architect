import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  Loader2,
  Package,
  RefreshCw,
  ShieldAlert,
  Sparkles,
  Target,
} from 'lucide-react'
import { fetchInstability } from '../api/client'
import type { PackageInstability, ProjectInfo } from '../types/api'
import {
  classifyZone,
  coordinateKey,
  formatMetric,
  groupPackagesByCoordinate,
  type PointGroup,
} from '../utils/instability'
import { InstabilityChart } from './InstabilityChart'
import { InstabilityTable } from './InstabilityTable'

interface InstabilityViewProps {
  project: ProjectInfo
  refreshSignal: number
}

export function InstabilityView({
  project,
  refreshSignal,
}: InstabilityViewProps) {
  const [packages, setPackages] = useState<PackageInstability[]>([])
  const [durationMs, setDurationMs] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [selectedPackageName, setSelectedPackageName] = useState<string | null>(
    null,
  )
  const [hoveredKey, setHoveredKey] = useState<string | null>(null)

  const loadData = useCallback((signal?: AbortSignal) => {
    setLoading(true)
    setError(null)
    fetchInstability(signal)
      .then((res) => {
        if (signal?.aborted) return
        setPackages(res.packages ?? [])
        setDurationMs(res.duration_ms ?? null)
        setSelectedKey(null)
        setSelectedPackageName(null)
      })
      .catch((err: unknown) => {
        if (signal?.aborted) return
        const message =
          err instanceof Error ? err.message : 'Failed to analyze project'
        setError(message)
      })
      .finally(() => {
        if (signal?.aborted) return
        setLoading(false)
      })
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    loadData(controller.signal)
    return () => controller.abort()
  }, [project.path, refreshSignal, loadData])

  const pointGroups = useMemo(
    () => groupPackagesByCoordinate(packages),
    [packages],
  )

  const summary = useMemo(() => {
    const total = packages.length
    if (total === 0) {
      return {
        total: 0,
        avgDistance: 0,
        healthyCount: 0,
        painCount: 0,
        uselessCount: 0,
      }
    }
    let distanceSum = 0
    let healthyCount = 0
    let painCount = 0
    let uselessCount = 0
    for (const pkg of packages) {
      distanceSum += pkg.distance
      const zone = classifyZone(pkg)
      if (zone === 'healthy') healthyCount++
      if (zone === 'pain') painCount++
      if (zone === 'useless') uselessCount++
    }
    return {
      total,
      avgDistance: distanceSum / total,
      healthyCount,
      painCount,
      uselessCount,
    }
  }, [packages])

  const handleSelectGroup = (group: PointGroup | null) => {
    if (!group) {
      setSelectedKey(null)
      setSelectedPackageName(null)
      return
    }
    setSelectedKey(group.key)
    setSelectedPackageName(
      group.packages.length === 1 ? group.packages[0].package_name : null,
    )
  }

  const handleSelectPackage = (pkg: PackageInstability | null) => {
    if (!pkg) {
      setSelectedKey(null)
      setSelectedPackageName(null)
      return
    }
    setSelectedKey(coordinateKey(pkg.instability, pkg.abstractness))
    setSelectedPackageName(pkg.package_name)
  }

  const handleHoverPackage = (pkg: PackageInstability | null) => {
    setHoveredKey(
      pkg ? coordinateKey(pkg.instability, pkg.abstractness) : null,
    )
  }

  if (loading && packages.length === 0) {
    return <InstabilitySkeleton />
  }

  if (error && packages.length === 0) {
    return (
      <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-rose-900">
        <div className="flex items-start gap-3">
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
          <div className="flex-1">
            <h3 className="text-sm font-semibold">
              Failed to compute Abstractness &amp; Instability metrics
            </h3>
            <p className="mt-1 text-sm text-rose-700">{error}</p>
            <button
              type="button"
              onClick={() => loadData()}
              className="mt-4 inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white shadow-xs transition hover:bg-rose-700"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Retry Analysis
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="relative flex flex-col gap-5">
      {/* Inline Re-analysis Banner */}
      {loading && packages.length > 0 && (
        <div className="flex items-center gap-2 rounded-lg border border-indigo-200 bg-indigo-50 px-3.5 py-2 text-xs font-medium text-indigo-800">
          <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
          Re-analyzing Go packages...
        </div>
      )}

      {error && packages.length > 0 && (
        <div className="flex items-center justify-between gap-2 rounded-lg border border-rose-200 bg-rose-50 px-3.5 py-2 text-xs text-rose-800">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-600" />
            <span>{error}</span>
          </div>
          <button
            type="button"
            onClick={() => loadData()}
            className="font-semibold underline hover:no-underline"
          >
            Retry
          </button>
        </div>
      )}

      {/* KPI Strip */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <KpiCard
          icon={<Package className="h-4 w-4 text-indigo-600" />}
          label="Packages Analyzed"
          value={String(summary.total)}
          hint={
            durationMs !== null
              ? `${pointGroups.length} unique coordinates · ${durationMs} ms`
              : `${pointGroups.length} unique coordinates`
          }
        />
        <KpiCard
          icon={<Target className="h-4 w-4 text-emerald-600" />}
          label="Avg Distance (D)"
          value={formatMetric(summary.avgDistance)}
          hint={`${summary.healthyCount} on Main Sequence (D ≤ 0.2)`}
        />
        <KpiCard
          icon={<ShieldAlert className="h-4 w-4 text-rose-600" />}
          label="Zone of Pain"
          value={String(summary.painCount)}
          hint="Rigid & concrete packages"
          highlight={summary.painCount > 0 ? 'rose' : undefined}
        />
        <KpiCard
          icon={<Sparkles className="h-4 w-4 text-orange-600" />}
          label="Zone of Uselessness"
          value={String(summary.uselessCount)}
          hint="Unreferenced abstractions"
          highlight={summary.uselessCount > 0 ? 'orange' : undefined}
        />
      </div>

      {/* Main Chart + Table Split */}
      <div className="grid grid-cols-1 items-start gap-5 xl:grid-cols-12">
        <div className="xl:col-span-5">
          <InstabilityChart
            groups={pointGroups}
            modulePath={project.package}
            selectedKey={selectedKey}
            highlightedKey={hoveredKey}
            onSelectGroup={handleSelectGroup}
            onHoverGroup={setHoveredKey}
          />
        </div>
        <div className="xl:col-span-7">
          <InstabilityTable
            packages={packages}
            modulePath={project.package}
            selectedKey={selectedKey}
            selectedPackageName={selectedPackageName}
            hoveredKey={hoveredKey}
            onSelectPackage={handleSelectPackage}
            onHoverPackage={handleHoverPackage}
          />
        </div>
      </div>
    </div>
  )
}

function KpiCard({
  icon,
  label,
  value,
  hint,
  highlight,
}: {
  icon: React.ReactNode
  label: string
  value: string
  hint: string
  highlight?: 'rose' | 'orange'
}) {
  const ringClass =
    highlight === 'rose'
      ? 'border-rose-200/80'
      : highlight === 'orange'
        ? 'border-orange-200/80'
        : 'border-slate-200'

  return (
    <div
      className={`rounded-xl border ${ringClass} bg-white px-4 py-3 shadow-xs`}
    >
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

function InstabilitySkeleton() {
  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-center gap-2.5 rounded-xl border border-indigo-200 bg-indigo-50/70 px-4 py-3 text-sm font-medium text-indigo-900">
        <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
        Analyzing package ASTs, coupling, and abstractness...
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[0, 1, 2, 3].map((i) => (
          <div
            key={i}
            className="h-24 animate-pulse rounded-xl border border-slate-200 bg-white p-4"
          >
            <div className="h-3 w-24 rounded bg-slate-200" />
            <div className="mt-3 h-7 w-16 rounded bg-slate-200" />
            <div className="mt-2 h-2.5 w-32 rounded bg-slate-100" />
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-5 xl:grid-cols-12">
        <div className="aspect-square animate-pulse rounded-xl border border-slate-200 bg-white xl:col-span-5" />
        <div className="h-[520px] animate-pulse rounded-xl border border-slate-200 bg-white xl:col-span-7" />
      </div>
    </div>
  )
}
