import { useMemo, useState } from 'react'
import type { PackageInstability } from '../types/api'
import {
  coordinateKey,
  formatMetric,
  formatPackageLabel,
  type PointGroup,
  ZONE_META,
} from '../utils/instability'

interface InstabilityChartProps {
  groups: PointGroup[]
  modulePath?: string
  selectedKey: string | null
  highlightedKey: string | null
  onSelectGroup: (group: PointGroup | null) => void
  onHoverGroup: (key: string | null) => void
}

const PAD_LEFT = 62
const PAD_TOP = 24
const PLOT_SIZE = 500
const VIEW_WIDTH = 592
const VIEW_HEIGHT = 584

const TICKS = [0, 0.2, 0.4, 0.6, 0.8, 1]

function toX(instability: number): number {
  return PAD_LEFT + Math.max(0, Math.min(1, instability)) * PLOT_SIZE
}

function toY(abstractness: number): number {
  return PAD_TOP + (1 - Math.max(0, Math.min(1, abstractness))) * PLOT_SIZE
}

function pointsAttr(coords: Array<[number, number]>): string {
  return coords.map(([i, a]) => `${toX(i)},${toY(a)}`).join(' ')
}

export function InstabilityChart({
  groups,
  modulePath,
  selectedKey,
  highlightedKey,
  onSelectGroup,
  onHoverGroup,
}: InstabilityChartProps) {
  const [hoveredGroup, setHoveredGroup] = useState<PointGroup | null>(null)

  const activeHoveredKey = hoveredGroup?.key ?? highlightedKey

  const sortedGroups = useMemo(() => {
    // Render selected/hovered groups last so they sit on top in SVG paint order
    return [...groups].sort((a, b) => {
      const aPriority =
        a.key === selectedKey ? 2 : a.key === activeHoveredKey ? 1 : 0
      const bPriority =
        b.key === selectedKey ? 2 : b.key === activeHoveredKey ? 1 : 0
      return aPriority - bPriority
    })
  }, [groups, selectedKey, activeHoveredKey])

  const tooltipGroup = useMemo(() => {
    if (hoveredGroup) return hoveredGroup
    if (highlightedKey) {
      return groups.find((g) => g.key === highlightedKey) ?? null
    }
    return null
  }, [hoveredGroup, highlightedKey, groups])

  const tooltipPosition = useMemo(() => {
    if (!tooltipGroup) return null
    const xPct = (toX(tooltipGroup.instability) / VIEW_WIDTH) * 100
    const yPct = (toY(tooltipGroup.abstractness) / VIEW_HEIGHT) * 100
    const alignRight = tooltipGroup.instability > 0.55
    const alignBelow = tooltipGroup.abstractness > 0.65
    return { xPct, yPct, alignRight, alignBelow }
  }, [tooltipGroup])

  const handleBackgroundClick = () => {
    if (selectedKey !== null) {
      onSelectGroup(null)
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="relative w-full select-none rounded-xl border border-slate-200 bg-white p-2 shadow-xs">
        <svg
          viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
          className="h-auto w-full overflow-visible"
          role="img"
          aria-label="Abstractness and Instability scatter plot"
        >
          {/* Clickable background to clear selection */}
          <rect
            x={0}
            y={0}
            width={VIEW_WIDTH}
            height={VIEW_HEIGHT}
            fill="transparent"
            onClick={handleBackgroundClick}
          />

          {/* Main Sequence Base (Green) */}
          <rect
            x={PAD_LEFT}
            y={PAD_TOP}
            width={PLOT_SIZE}
            height={PLOT_SIZE}
            fill="#ecfdf5"
            rx={4}
            onClick={handleBackgroundClick}
          />

          {/* Buffer Zones (Amber) */}
          <polygon
            points={pointsAttr([
              [0, 0.8],
              [0, 0],
              [0.8, 0],
            ])}
            fill="#fef3c7"
            onClick={handleBackgroundClick}
          />
          <polygon
            points={pointsAttr([
              [0.2, 1],
              [1, 1],
              [1, 0.2],
            ])}
            fill="#fef3c7"
            onClick={handleBackgroundClick}
          />

          {/* Zone of Pain & Zone of Uselessness (Rose) */}
          <polygon
            points={pointsAttr([
              [0, 0.6],
              [0, 0],
              [0.6, 0],
            ])}
            fill="#ffe4e6"
            onClick={handleBackgroundClick}
          />
          <polygon
            points={pointsAttr([
              [0.4, 1],
              [1, 1],
              [1, 0.4],
            ])}
            fill="#ffe4e6"
            onClick={handleBackgroundClick}
          />

          {/* Gridlines */}
          {TICKS.map((t) => (
            <g key={t} className="pointer-events-none">
              <line
                x1={toX(t)}
                y1={PAD_TOP}
                x2={toX(t)}
                y2={PAD_TOP + PLOT_SIZE}
                stroke="#cbd5e1"
                strokeWidth={t === 0 ? 1.5 : 1}
                strokeDasharray={t === 0 || t === 1 ? undefined : '3,4'}
                opacity={t === 0 || t === 1 ? 0.7 : 0.55}
              />
              <line
                x1={PAD_LEFT}
                y1={toY(t)}
                x2={PAD_LEFT + PLOT_SIZE}
                y2={toY(t)}
                stroke="#cbd5e1"
                strokeWidth={t === 0 ? 1.5 : 1}
                strokeDasharray={t === 0 || t === 1 ? undefined : '3,4'}
                opacity={t === 0 || t === 1 ? 0.7 : 0.55}
              />

              {/* X-axis tick labels */}
              <text
                x={toX(t)}
                y={PAD_TOP + PLOT_SIZE + 20}
                textAnchor="middle"
                className="fill-slate-500 text-[12px] font-medium"
              >
                {t.toFixed(1)}
              </text>

              {/* Y-axis tick labels */}
              <text
                x={PAD_LEFT - 10}
                y={toY(t) + 4}
                textAnchor="end"
                className="fill-slate-500 text-[12px] font-medium"
              >
                {t.toFixed(1)}
              </text>
            </g>
          ))}

          {/* Plot Border */}
          <rect
            x={PAD_LEFT}
            y={PAD_TOP}
            width={PLOT_SIZE}
            height={PLOT_SIZE}
            fill="none"
            stroke="#94a3b8"
            strokeWidth={1.25}
            rx={4}
            className="pointer-events-none"
          />

          {/* Main Sequence Diagonal */}
          <line
            x1={toX(0)}
            y1={toY(1)}
            x2={toX(1)}
            y2={toY(0)}
            stroke="#047857"
            strokeWidth={2}
            strokeDasharray="6,6"
            opacity={0.7}
            className="pointer-events-none"
          />

          {/* Zone Labels */}
          <text
            x={toX(0.05)}
            y={toY(0.06)}
            className="pointer-events-none fill-rose-700/75 text-[12px] font-semibold"
          >
            Zone of Pain
          </text>
          <text
            x={toX(0.95)}
            y={toY(0.92)}
            textAnchor="end"
            className="pointer-events-none fill-orange-700/75 text-[12px] font-semibold"
          >
            Zone of Uselessness
          </text>
          <text
            x={toX(0.5)}
            y={toY(0.52)}
            textAnchor="middle"
            transform={`rotate(45 ${toX(0.5)} ${toY(0.52)})`}
            className="pointer-events-none fill-emerald-800/70 text-[11px] font-semibold tracking-wide"
          >
            Main Sequence (A + I = 1)
          </text>

          {/* Axis Titles */}
          <text
            x={PAD_LEFT + PLOT_SIZE / 2}
            y={VIEW_HEIGHT - 12}
            textAnchor="middle"
            className="fill-slate-700 text-[13px] font-semibold"
          >
            Instability (I) — Stable → Unstable
          </text>
          <text
            x={18}
            y={PAD_TOP + PLOT_SIZE / 2}
            textAnchor="middle"
            transform={`rotate(-90 18 ${PAD_TOP + PLOT_SIZE / 2})`}
            className="fill-slate-700 text-[13px] font-semibold"
          >
            Abstractness (A) — Concrete → Abstract
          </text>

          {/* Package Dots */}
          {sortedGroups.map((group) => {
            const cx = toX(group.instability)
            const cy = toY(group.abstractness)
            const count = group.packages.length
            const isMulti = count > 1
            const isSelected = group.key === selectedKey
            const isHovered = group.key === activeHoveredKey
            const zoneMeta = ZONE_META[group.zone]
            const baseRadius = isMulti ? 10.5 : 6.5
            const radius = isSelected
              ? baseRadius + 2.5
              : isHovered
                ? baseRadius + 2
                : baseRadius

            return (
              <g
                key={group.key}
                transform={`translate(${cx}, ${cy})`}
                className="cursor-pointer outline-none"
                tabIndex={0}
                role="button"
                aria-label={`${count} ${count === 1 ? 'package' : 'packages'} at instability ${formatMetric(group.instability)}, abstractness ${formatMetric(group.abstractness)}`}
                onMouseEnter={() => {
                  setHoveredGroup(group)
                  onHoverGroup(group.key)
                }}
                onMouseLeave={() => {
                  setHoveredGroup(null)
                  onHoverGroup(null)
                }}
                onFocus={() => {
                  setHoveredGroup(group)
                  onHoverGroup(group.key)
                }}
                onBlur={() => {
                  setHoveredGroup(null)
                  onHoverGroup(null)
                }}
                onClick={(e) => {
                  e.stopPropagation()
                  onSelectGroup(isSelected ? null : group)
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    onSelectGroup(isSelected ? null : group)
                  }
                }}
              >
                {/* Selection / Hover Halo */}
                {(isSelected || isHovered) && (
                  <circle
                    r={radius + 5}
                    fill={isSelected ? '#4f46e5' : zoneMeta.dotFill}
                    opacity={isSelected ? 0.22 : 0.16}
                  />
                )}

                {/* Main Dot */}
                <circle
                  r={radius}
                  fill={isSelected ? '#4f46e5' : zoneMeta.dotFill}
                  stroke={isSelected ? '#ffffff' : zoneMeta.dotStroke}
                  strokeWidth={isSelected ? 2.5 : 1.75}
                  className="transition-all duration-150"
                />

                {/* Count badge inside overlapping cluster */}
                {isMulti && (
                  <text
                    textAnchor="middle"
                    dominantBaseline="central"
                    className="pointer-events-none fill-white text-[10px] font-bold"
                  >
                    {count}
                  </text>
                )}
              </g>
            )
          })}
        </svg>

        {/* Floating Tooltip */}
        {tooltipGroup && tooltipPosition && (
          <div
            className="pointer-events-none absolute z-20 w-80 rounded-xl border border-slate-200 bg-slate-900/95 p-3 text-xs text-slate-100 shadow-xl backdrop-blur-xs transition-all duration-100"
            style={{
              left: `${tooltipPosition.xPct}%`,
              top: `${tooltipPosition.yPct}%`,
              transform: `translate(${tooltipPosition.alignRight ? 'calc(-100% - 14px)' : '14px'}, ${tooltipPosition.alignBelow ? '14px' : 'calc(-100% - 14px)'})`,
            }}
          >
            <div className="mb-2 flex items-center justify-between gap-2">
              <span className="inline-flex items-center gap-1.5 rounded-full bg-white/10 px-2 py-0.5 text-[11px] font-semibold text-white">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{
                    backgroundColor: ZONE_META[tooltipGroup.zone].dotFill,
                  }}
                />
                {ZONE_META[tooltipGroup.zone].label}
              </span>
              <span className="font-mono text-[11px] text-slate-300">
                D={formatMetric(tooltipGroup.distance)}
              </span>
            </div>

            <div className="mb-2 grid grid-cols-2 gap-1.5 rounded-lg bg-white/5 px-2.5 py-1.5 font-mono text-[11px]">
              <div>
                <span className="text-slate-400">I: </span>
                <span className="font-semibold text-white">
                  {formatMetric(tooltipGroup.instability)}
                </span>
              </div>
              <div>
                <span className="text-slate-400">A: </span>
                <span className="font-semibold text-white">
                  {formatMetric(tooltipGroup.abstractness)}
                </span>
              </div>
            </div>

            {tooltipGroup.packages.length === 1 ? (
              <TooltipSinglePackage
                pkg={tooltipGroup.packages[0]}
                modulePath={modulePath}
              />
            ) : (
              <div>
                <div className="mb-1 text-[11px] font-semibold text-slate-300">
                  {tooltipGroup.packages.length} packages at this point:
                </div>
                <ul className="space-y-1">
                  {tooltipGroup.packages.slice(0, 5).map((pkg) => {
                    const label = formatPackageLabel(pkg.package_name, modulePath)
                    return (
                      <li
                        key={pkg.package_name}
                        className="flex items-start justify-between gap-2 text-[11px]"
                      >
                        <span className="font-mono break-all text-slate-100">
                          {label.display}
                        </span>
                        <span className="shrink-0 font-mono text-slate-400">
                          Ca:{pkg.afferent_coupling} Ce:{pkg.efferent_coupling}
                        </span>
                      </li>
                    )
                  })}
                </ul>
                {tooltipGroup.packages.length > 5 && (
                  <div className="mt-1.5 text-[11px] text-indigo-300">
                    +{tooltipGroup.packages.length - 5} more — click dot to inspect all
                  </div>
                )}
              </div>
            )}

            <div className="mt-2 border-t border-white/10 pt-1.5 text-[10px] text-slate-400">
              Click dot to highlight matching rows in table
            </div>
          </div>
        )}
      </div>

      {/* Chart Legend */}
      <div className="flex flex-wrap items-center justify-between gap-2 px-1 text-xs text-slate-600">
        <div className="flex flex-wrap items-center gap-3">
          {(['healthy', 'warning', 'pain', 'useless'] as const).map((zone) => (
            <span key={zone} className="inline-flex items-center gap-1.5">
              <span
                className="h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: ZONE_META[zone].dotFill }}
              />
              {ZONE_META[zone].label}
            </span>
          ))}
        </div>
        <span className="text-[11px] text-slate-500">
          Numbered dots indicate overlapping packages
        </span>
      </div>
    </div>
  )
}

function TooltipSinglePackage({
  pkg,
  modulePath,
}: {
  pkg: PackageInstability
  modulePath?: string
}) {
  const label = formatPackageLabel(pkg.package_name, modulePath)
  return (
    <div>
      <div className="font-mono text-xs font-semibold break-all text-white">
        {label.display}
      </div>
      {label.display !== label.full && (
        <div className="font-mono text-[10px] break-all text-slate-400">
          {label.full}
        </div>
      )}
      <div className="mt-2 grid grid-cols-4 gap-1 text-center font-mono text-[10px]">
        <div className="rounded bg-white/5 px-1 py-1">
          <div className="text-slate-400">Ca</div>
          <div className="font-semibold text-white">{pkg.afferent_coupling}</div>
        </div>
        <div className="rounded bg-white/5 px-1 py-1">
          <div className="text-slate-400">Ce</div>
          <div className="font-semibold text-white">{pkg.efferent_coupling}</div>
        </div>
        <div className="rounded bg-white/5 px-1 py-1">
          <div className="text-slate-400">Abs</div>
          <div className="font-semibold text-white">{pkg.abstractions_count}</div>
        </div>
        <div className="rounded bg-white/5 px-1 py-1">
          <div className="text-slate-400">Impl</div>
          <div className="font-semibold text-white">{pkg.implementations_count}</div>
        </div>
      </div>
    </div>
  )
}

export { coordinateKey }
