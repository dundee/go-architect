import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertCircle,
  ChevronDown,
  ChevronUp,
  HelpCircle,
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
  const [showHelp, setShowHelp] = useState(false)

  const helpRef = useRef<HTMLDivElement | null>(null)

  const lastRefreshRef = useRef(refreshSignal)

  const loadData = useCallback((signal?: AbortSignal, refresh?: boolean) => {
    setLoading(true)
    setError(null)
    fetchInstability(signal, refresh)
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
    const isRefresh = lastRefreshRef.current !== refreshSignal
    lastRefreshRef.current = refreshSignal
    loadData(controller.signal, isRefresh)
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
        <div className="flex flex-col gap-3 xl:col-span-5">
          <InstabilityChart
            groups={pointGroups}
            modulePath={project.package}
            selectedKey={selectedKey}
            highlightedKey={hoveredKey}
            onSelectGroup={handleSelectGroup}
            onHoverGroup={setHoveredKey}
          />
          <button
            type="button"
            onClick={() => {
              const next = !showHelp
              setShowHelp(next)
              if (next) {
                setTimeout(() => {
                  helpRef.current?.scrollIntoView({
                    behavior: 'smooth',
                    block: 'nearest',
                  })
                }, 40)
              }
            }}
            aria-expanded={showHelp}
            className="inline-flex w-full items-center justify-center gap-2 rounded-xl border border-slate-200 bg-white px-3.5 py-2 text-xs font-medium text-slate-700 shadow-2xs transition hover:bg-slate-50 hover:text-slate-900"
          >
            <HelpCircle className="h-4 w-4 text-indigo-600" />
            <span>
              {showHelp
                ? 'Hide Abstractness & Instability guide'
                : 'What does this chart mean?'}
            </span>
            {showHelp ? (
              <ChevronUp className="h-3.5 w-3.5 text-slate-400" />
            ) : (
              <ChevronDown className="h-3.5 w-3.5 text-slate-400" />
            )}
          </button>
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

      {showHelp && (
        <div ref={helpRef}>
          <InstabilityGuide />
        </div>
      )}
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

function InstabilityGuide() {
  return (
    <section
      aria-label="How to read the Abstractness and Instability chart"
      className="flex flex-col gap-5 rounded-xl border border-slate-200 bg-white p-5 shadow-xs"
    >
      <div className="border-b border-slate-100 pb-3">
        <h2 className="text-sm font-bold text-slate-900">
          Understanding Abstractness &amp; Instability
        </h2>
        <p className="mt-1 text-xs text-slate-600">
          This chart helps check whether each Go package has the right level of
          flexibility for how heavily the rest of your Project depends on it.
        </p>
      </div>

      {/* Practical Meaning */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-200/80 bg-slate-50/70 p-4">
          <div className="text-xs font-semibold text-slate-900">
            Stable packages (I ≈ 0.0) — Library-like code
          </div>
          <p className="mt-1.5 text-xs leading-relaxed text-slate-600">
            A <strong className="font-semibold text-slate-800">stable</strong>{' '}
            package has few or no internal dependencies of its own, while other
            packages depend on it. Because changing it can break many callers,
            library-like code should expose{' '}
            <strong className="font-semibold text-slate-800">
              interfaces and abstractions
            </strong>{' '}
            and have a reasonable number of dependants—otherwise concrete shared
            code becomes rigid and unmaintainable.
          </p>
        </div>

        <div className="rounded-xl border border-slate-200/80 bg-slate-50/70 p-4">
          <div className="text-xs font-semibold text-slate-900">
            Unstable packages (I ≈ 1.0) — Application &amp; leaf code
          </div>
          <p className="mt-1.5 text-xs leading-relaxed text-slate-600">
            An{' '}
            <strong className="font-semibold text-slate-800">unstable</strong>{' '}
            package depends on other internal packages, while nothing else
            depends on it (for example <code className="font-mono">cmd/</code>,
            handlers, or wiring code). That&apos;s healthy: it is easy to
            change without breaking downstream packages, so it can stay concrete
            and simple without extra interfaces.
          </p>
        </div>
      </div>

      {/* Formulas */}
      <div>
        <h3 className="mb-2.5 text-xs font-semibold tracking-wider text-slate-500 uppercase">
          Metrics &amp; Formulas
        </h3>
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
          <div className="rounded-xl border border-slate-200 p-3.5">
            <div className="text-xs font-semibold text-slate-900">
              Instability (I)
            </div>
            <div className="mt-1.5 rounded-lg bg-slate-900 px-3 py-1.5 font-mono text-xs text-indigo-200">
              I = Ce / (Ca + Ce)
            </div>
            <p className="mt-2 text-[11px] leading-relaxed text-slate-600">
              <strong className="font-mono text-slate-800">Ce</strong> (outgoing):
              internal packages this package imports.{' '}
              <strong className="font-mono text-slate-800">Ca</strong> (incoming):
              other internal packages importing this package. Ranges from{' '}
              <span className="font-mono">0.0</span> (stable) to{' '}
              <span className="font-mono">1.0</span> (unstable).
            </p>
          </div>

          <div className="rounded-xl border border-slate-200 p-3.5">
            <div className="text-xs font-semibold text-slate-900">
              Abstractness (A)
            </div>
            <div className="mt-1.5 rounded-lg bg-slate-900 px-3 py-1.5 font-mono text-xs text-indigo-200">
              A = Abstractions / (Abstractions + Implementations)
            </div>
            <p className="mt-2 text-[11px] leading-relaxed text-slate-600">
              Counts structs and non-empty interfaces as{' '}
              <strong className="text-slate-800">Abstractions</strong>, and
              functions and methods as{' '}
              <strong className="text-slate-800">Implementations</strong>. Ranges
              from <span className="font-mono">0.0</span> (concrete) to{' '}
              <span className="font-mono">1.0</span> (abstract).
            </p>
          </div>

          <div className="rounded-xl border border-slate-200 p-3.5">
            <div className="text-xs font-semibold text-slate-900">
              Distance from Main Sequence (D)
            </div>
            <div className="mt-1.5 rounded-lg bg-slate-900 px-3 py-1.5 font-mono text-xs text-indigo-200">
              D = |A + I - 1|
            </div>
            <p className="mt-2 text-[11px] leading-relaxed text-slate-600">
              How far a package lies from the ideal{' '}
              <span className="font-mono">A + I = 1</span> diagonal.{' '}
              <span className="font-mono">0.0</span> is ideal; higher values
              indicate rigidity or unused abstractions.
            </p>
          </div>
        </div>
      </div>

      {/* Zones */}
      <div>
        <h3 className="mb-2.5 text-xs font-semibold tracking-wider text-slate-500 uppercase">
          Reading the Zones
        </h3>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="rounded-xl border border-emerald-200 bg-emerald-50/60 p-3">
            <div className="flex items-center gap-2 text-xs font-semibold text-emerald-900">
              <span className="h-2.5 w-2.5 rounded-full bg-emerald-600" />
              Main Sequence (D ≤ 0.2)
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-emerald-800">
              Balanced trade-off: heavily depended-on packages expose
              abstractions, while leaf packages stay concrete.
            </p>
          </div>

          <div className="rounded-xl border border-amber-200 bg-amber-50/60 p-3">
            <div className="flex items-center gap-2 text-xs font-semibold text-amber-900">
              <span className="h-2.5 w-2.5 rounded-full bg-amber-600" />
              Buffer Zone (0.2 &lt; D &lt; 0.4)
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-amber-800">
              Mild drift from the diagonal. Worth keeping an eye on if the
              package continues gaining dependants.
            </p>
          </div>

          <div className="rounded-xl border border-rose-200 bg-rose-50/60 p-3">
            <div className="flex items-center gap-2 text-xs font-semibold text-rose-900">
              <span className="h-2.5 w-2.5 rounded-full bg-rose-600" />
              Zone of Pain (Bottom-Left)
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-rose-800">
              Concrete code that many packages depend on without interfaces.
              Hard to modify or mock—unless it&apos;s a small, rarely changing
              utility package.
            </p>
          </div>

          <div className="rounded-xl border border-orange-200 bg-orange-50/60 p-3">
            <div className="flex items-center gap-2 text-xs font-semibold text-orange-900">
              <span className="h-2.5 w-2.5 rounded-full bg-orange-600" />
              Zone of Uselessness (Top-Right)
            </div>
            <p className="mt-1 text-[11px] leading-relaxed text-orange-800">
              Lots of interfaces/abstractions with no incoming dependants—often
              dead code or premature over-engineering.
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}
