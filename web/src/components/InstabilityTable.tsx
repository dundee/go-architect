import { useEffect, useMemo, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, ArrowUpDown, Search, X } from 'lucide-react'
import type { PackageInstability } from '../types/api'
import {
  classifyZone,
  coordinateKey,
  formatMetric,
  formatPackageLabel,
  type InstabilityZone,
  ZONE_META,
} from '../utils/instability'

type SortField =
  | 'package_name'
  | 'afferent_coupling'
  | 'efferent_coupling'
  | 'abstractness'
  | 'instability'
  | 'distance'

type SortDirection = 'asc' | 'desc'

interface InstabilityTableProps {
  packages: PackageInstability[]
  modulePath?: string
  selectedKey: string | null
  selectedPackageName: string | null
  hoveredKey: string | null
  onSelectPackage: (pkg: PackageInstability | null) => void
  onHoverPackage: (pkg: PackageInstability | null) => void
}

export function InstabilityTable({
  packages,
  modulePath,
  selectedKey,
  selectedPackageName,
  hoveredKey,
  onSelectPackage,
  onHoverPackage,
}: InstabilityTableProps) {
  const [searchQuery, setSearchQuery] = useState('')
  const [zoneFilter, setZoneFilter] = useState<InstabilityZone | 'all'>('all')
  const [onlySelected, setOnlySelected] = useState(false)
  const [sortField, setSortField] = useState<SortField>('distance')
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc')

  const rowRefs = useRef<Record<string, HTMLTableRowElement | null>>({})

  // Reset isolate toggle when selection is cleared
  useEffect(() => {
    if (!selectedKey) {
      setOnlySelected(false)
    }
  }, [selectedKey])

  // Scroll first matching row into view when selected from the chart
  useEffect(() => {
    if (!selectedKey || onlySelected) return
    const firstMatch = packages.find(
      (p) => coordinateKey(p.instability, p.abstractness) === selectedKey,
    )
    if (firstMatch) {
      const el = rowRefs.current[firstMatch.package_name]
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
      }
    }
  }, [selectedKey, packages, onlySelected])

  const zoneCounts = useMemo(() => {
    const counts: Record<InstabilityZone, number> = {
      healthy: 0,
      warning: 0,
      pain: 0,
      useless: 0,
    }
    for (const pkg of packages) {
      counts[classifyZone(pkg)]++
    }
    return counts
  }, [packages])

  const selectedCount = useMemo(() => {
    if (!selectedKey) return 0
    return packages.filter(
      (p) => coordinateKey(p.instability, p.abstractness) === selectedKey,
    ).length
  }, [packages, selectedKey])

  const filteredAndSorted = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    const filtered = packages.filter((pkg) => {
      if (onlySelected && selectedKey) {
        if (coordinateKey(pkg.instability, pkg.abstractness) !== selectedKey) {
          return false
        }
      }
      if (zoneFilter !== 'all' && classifyZone(pkg) !== zoneFilter) {
        return false
      }
      if (q && !pkg.package_name.toLowerCase().includes(q)) {
        return false
      }
      return true
    })

    return [...filtered].sort((a, b) => {
      const valA = a[sortField]
      const valB = b[sortField]
      let cmp = 0
      if (typeof valA === 'string' && typeof valB === 'string') {
        cmp = valA.localeCompare(valB)
      } else {
        cmp = Number(valA) - Number(valB)
      }
      if (cmp === 0) {
        cmp = a.package_name.localeCompare(b.package_name)
      }
      return sortDirection === 'asc' ? cmp : -cmp
    })
  }, [
    packages,
    searchQuery,
    zoneFilter,
    onlySelected,
    selectedKey,
    sortField,
    sortDirection,
  ])

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortField(field)
      setSortDirection(field === 'package_name' ? 'asc' : 'desc')
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
      {/* Filter Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Filter packages by name..."
            className="w-full rounded-lg border border-slate-200 bg-slate-50 py-1.5 pr-8 pl-9 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:bg-white focus:ring-2 focus:ring-indigo-500/20 focus:outline-none"
          />
          {searchQuery && (
            <button
              type="button"
              onClick={() => setSearchQuery('')}
              className="absolute top-1/2 right-2.5 -translate-y-1/2 text-slate-400 hover:text-slate-600"
              aria-label="Clear search"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <FilterPill
            active={zoneFilter === 'all'}
            onClick={() => setZoneFilter('all')}
            label={`All (${packages.length})`}
          />
          {(['healthy', 'warning', 'pain', 'useless'] as const).map((zone) => (
            <FilterPill
              key={zone}
              active={zoneFilter === zone}
              onClick={() =>
                setZoneFilter((curr) => (curr === zone ? 'all' : zone))
              }
              label={`${ZONE_META[zone].shortLabel} (${zoneCounts[zone]})`}
              dotColor={ZONE_META[zone].dotFill}
            />
          ))}
        </div>
      </div>

      {/* Active Chart Selection Bar */}
      {selectedKey && (
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-indigo-200 bg-indigo-50/70 px-3 py-2 text-xs text-indigo-900">
          <div className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full bg-indigo-600" />
            <span>
              Highlighting{' '}
              <strong className="font-semibold">
                {selectedCount}{' '}
                {selectedCount === 1 ? 'package' : 'packages'}
              </strong>{' '}
              at selected chart coordinates
            </span>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setOnlySelected((v) => !v)}
              className={`rounded-md px-2 py-1 font-medium transition ${
                onlySelected
                  ? 'bg-indigo-600 text-white'
                  : 'bg-white text-indigo-700 ring-1 ring-indigo-200 hover:bg-indigo-100'
              }`}
            >
              {onlySelected ? 'Showing selected only' : 'Isolate in table'}
            </button>
            <button
              type="button"
              onClick={() => onSelectPackage(null)}
              className="inline-flex items-center gap-1 rounded-md px-2 py-1 font-medium text-indigo-700 hover:bg-indigo-100"
            >
              <X className="h-3.5 w-3.5" />
              Clear
            </button>
          </div>
        </div>
      )}

      {/* Table */}
      <div className="max-h-[540px] overflow-auto rounded-lg border border-slate-200">
        <table className="w-full border-collapse text-left text-xs">
          <thead className="sticky top-0 z-10 border-b border-slate-200 bg-slate-50 text-[11px] font-semibold tracking-wider text-slate-600 uppercase">
            <tr>
              <SortableTh
                label="Package"
                field="package_name"
                sortField={sortField}
                sortDirection={sortDirection}
                onSort={handleSort}
              />
              <th className="px-3 py-2.5">Zone</th>
              <SortableTh
                label="Ca"
                title="Afferent Coupling (incoming dependencies)"
                field="afferent_coupling"
                sortField={sortField}
                sortDirection={sortDirection}
                onSort={handleSort}
                align="right"
              />
              <SortableTh
                label="Ce"
                title="Efferent Coupling (outgoing dependencies)"
                field="efferent_coupling"
                sortField={sortField}
                sortDirection={sortDirection}
                onSort={handleSort}
                align="right"
              />
              <SortableTh
                label="Abstractness"
                field="abstractness"
                sortField={sortField}
                sortDirection={sortDirection}
                onSort={handleSort}
                align="right"
              />
              <SortableTh
                label="Instability"
                field="instability"
                sortField={sortField}
                sortDirection={sortDirection}
                onSort={handleSort}
                align="right"
              />
              <SortableTh
                label="Distance"
                field="distance"
                sortField={sortField}
                sortDirection={sortDirection}
                onSort={handleSort}
                align="right"
              />
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {filteredAndSorted.length === 0 ? (
              <tr>
                <td
                  colSpan={7}
                  className="px-4 py-10 text-center text-sm text-slate-500"
                >
                  No packages match the current filters.
                </td>
              </tr>
            ) : (
              filteredAndSorted.map((pkg) => {
                const key = coordinateKey(pkg.instability, pkg.abstractness)
                const zone = classifyZone(pkg)
                const zoneMeta = ZONE_META[zone]
                const label = formatPackageLabel(pkg.package_name, modulePath)
                const isSelectedGroup = selectedKey === key
                const isExactSelected =
                  selectedPackageName === pkg.package_name
                const isHoveredGroup = hoveredKey === key

                return (
                  <tr
                    key={pkg.package_name}
                    ref={(el) => {
                      rowRefs.current[pkg.package_name] = el
                    }}
                    onClick={() =>
                      onSelectPackage(isExactSelected ? null : pkg)
                    }
                    onMouseEnter={() => onHoverPackage(pkg)}
                    onMouseLeave={() => onHoverPackage(null)}
                    className={`cursor-pointer transition-colors ${
                      isSelectedGroup
                        ? 'bg-indigo-50/90 hover:bg-indigo-100/70'
                        : isHoveredGroup
                          ? 'bg-slate-100/90'
                          : 'hover:bg-slate-50'
                    }`}
                  >
                    <td className="max-w-[260px] px-3 py-2.5">
                      <div className="flex items-center gap-2">
                        {isSelectedGroup && (
                          <span className="h-2 w-2 shrink-0 rounded-full bg-indigo-600" />
                        )}
                        <div className="min-w-0">
                          <div
                            className="truncate font-mono text-xs font-medium text-slate-900"
                            title={label.full}
                          >
                            {label.display}
                          </div>
                          <div className="text-[10px] text-slate-400">
                            {pkg.abstractions_count} abs ·{' '}
                            {pkg.implementations_count} impl
                          </div>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-2.5 whitespace-nowrap">
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${zoneMeta.badgeClass}`}
                        title={zoneMeta.description}
                      >
                        {zoneMeta.shortLabel}
                      </span>
                    </td>
                    <td className="px-3 py-2.5 text-right font-mono text-slate-700 tabular-nums">
                      {pkg.afferent_coupling}
                    </td>
                    <td className="px-3 py-2.5 text-right font-mono text-slate-700 tabular-nums">
                      {pkg.efferent_coupling}
                    </td>
                    <td className="px-3 py-2.5 text-right font-mono text-slate-700 tabular-nums">
                      {formatMetric(pkg.abstractness)}
                    </td>
                    <td className="px-3 py-2.5 text-right font-mono text-slate-700 tabular-nums">
                      {formatMetric(pkg.instability)}
                    </td>
                    <td className="px-3 py-2.5 text-right font-mono font-semibold text-slate-900 tabular-nums">
                      {formatMetric(pkg.distance)}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function FilterPill({
  active,
  label,
  dotColor,
  onClick,
}: {
  active: boolean
  label: string
  dotColor?: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium transition ${
        active
          ? 'bg-slate-900 text-white shadow-xs'
          : 'bg-slate-100 text-slate-700 hover:bg-slate-200'
      }`}
    >
      {dotColor && (
        <span
          className="h-2 w-2 rounded-full"
          style={{ backgroundColor: dotColor }}
        />
      )}
      {label}
    </button>
  )
}

function SortableTh({
  label,
  title,
  field,
  sortField,
  sortDirection,
  onSort,
  align = 'left',
}: {
  label: string
  title?: string
  field: SortField
  sortField: SortField
  sortDirection: SortDirection
  onSort: (field: SortField) => void
  align?: 'left' | 'right'
}) {
  const isActive = sortField === field
  return (
    <th
      title={title}
      className={`px-3 py-2.5 ${align === 'right' ? 'text-right' : 'text-left'}`}
    >
      <button
        type="button"
        onClick={() => onSort(field)}
        className={`inline-flex items-center gap-1 font-semibold uppercase transition hover:text-slate-900 ${
          align === 'right' ? 'flex-row-reverse' : ''
        } ${isActive ? 'text-indigo-700' : 'text-slate-600'}`}
      >
        <span>{label}</span>
        {isActive ? (
          sortDirection === 'asc' ? (
            <ArrowUp className="h-3 w-3" />
          ) : (
            <ArrowDown className="h-3 w-3" />
          )
        ) : (
          <ArrowUpDown className="h-3 w-3 opacity-40" />
        )}
      </button>
    </th>
  )
}
