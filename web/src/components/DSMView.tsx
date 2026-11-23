import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  AlertTriangle,
  ArrowRight,
  Loader2,
  RefreshCw,
} from 'lucide-react'
import { fetchDSM } from '../api/client'
import type { DependencyStructureMatrix, ProjectInfo } from '../types/api'
import {
  LAYER_META,
  LAYER_ORDER,
  type PackageLayer,
  resolvePackageLayer,
  shortenPackageName,
} from '../utils/packages'

interface DSMViewProps {
  project: ProjectInfo
  refreshSignal: number
}

interface SelectedCell {
  row: number
  col: number
}

interface ProcessedDSM {
  labels: string[]
  layers: PackageLayer[]
  matrix: number[][]
  rawMatrix: number[][]
}

export function DSMView({ project, refreshSignal }: DSMViewProps) {
  const [dsm, setDsm] = useState<DependencyStructureMatrix | null>(null)
  const [durationMs, setDurationMs] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showWeighted, setShowWeighted] = useState(false)
  const [showAggregated, setShowAggregated] = useState(false)
  const [hideNonInternalCols, setHideNonInternalCols] = useState(false)
  const [hoveredCell, setHoveredCell] = useState<SelectedCell | null>(null)
  const [selectedCell, setSelectedCell] = useState<SelectedCell | null>(null)

  const orgPackages = useMemo(
    () => project.organization_packages ?? [],
    [project.organization_packages],
  )

  const loadData = useCallback((signal?: AbortSignal) => {
    setLoading(true)
    setError(null)
    fetchDSM(signal)
      .then((res) => {
        if (signal?.aborted) return
        setDsm(res.dsm)
        setDurationMs(res.duration_ms ?? null)
        setSelectedCell(null)
        setHoveredCell(null)
      })
      .catch((err: unknown) => {
        if (signal?.aborted) return
        setError(
          err instanceof Error
            ? err.message
            : 'Failed to load Dependency Structure Matrix',
        )
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
  }, [project.path, project.organization_packages, refreshSignal, loadData])

  const processed: ProcessedDSM | null = useMemo(() => {
    if (!dsm) return null
    const mainPkg = dsm.module || project.package

    if (showAggregated) {
      const agg = [
        [0, 0, 0, 0],
        [0, 0, 0, 0],
        [0, 0, 0, 0],
        [0, 0, 0, 0],
      ]
      dsm.dependencies.forEach((rowVals, i) => {
        const destLayer = LAYER_ORDER.indexOf(
          resolvePackageLayer(dsm.packages[i], mainPkg, orgPackages),
        )
        rowVals.forEach((val, j) => {
          const origLayer = LAYER_ORDER.indexOf(
            resolvePackageLayer(dsm.packages[j], mainPkg, orgPackages),
          )
          agg[destLayer][origLayer] += val
        })
      })

      const displayMatrix = showWeighted
        ? agg
        : agg.map((r) => r.map((v) => (v > 0 ? 1 : 0)))

      return {
        labels: LAYER_ORDER.map((l) => LAYER_META[l].label),
        layers: [...LAYER_ORDER],
        matrix: displayMatrix,
        rawMatrix: agg,
      }
    }

    // Optional filtering to focus on internal + organization packages
    const indices: number[] = []
    dsm.packages.forEach((pkg, idx) => {
      const layer = resolvePackageLayer(pkg, mainPkg, orgPackages)
      if (!hideNonInternalCols || layer === 'internal' || layer === 'organization') {
        indices.push(idx)
      }
    })

    const labels = indices.map((i) => dsm.packages[i])
    const layers = indices.map((i) =>
      resolvePackageLayer(dsm.packages[i], mainPkg, orgPackages),
    )
    const rawMatrix = indices.map((i) =>
      indices.map((j) => dsm.dependencies[i]?.[j] ?? 0),
    )
    const matrix = showWeighted
      ? rawMatrix
      : rawMatrix.map((r) => r.map((v) => (v > 0 ? 1 : 0)))

    return { labels, layers, matrix, rawMatrix }
  }, [dsm, showAggregated, showWeighted, hideNonInternalCols, orgPackages, project.package])

  const cyclePairs = useMemo(() => {
    const cycles = new Set<string>()
    if (!processed) return { cycles, pairCount: 0 }
    let pairCount = 0
    const n = processed.matrix.length
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        if (processed.rawMatrix[i][j] > 0 && processed.rawMatrix[j][i] > 0) {
          cycles.add(`${i}:${j}`)
          cycles.add(`${j}:${i}`)
          pairCount++
        }
      }
    }
    return { cycles, pairCount }
  }, [processed])

  const activeCell = selectedCell ?? hoveredCell

  if (loading && !dsm) {
    return (
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-2.5 rounded-xl border border-indigo-200 bg-indigo-50/70 px-4 py-3 text-sm font-medium text-indigo-900">
          <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
          Computing Dependency Structure Matrix...
        </div>
        <div className="h-[540px] animate-pulse rounded-xl border border-slate-200 bg-white" />
      </div>
    )
  }

  if (error && !dsm) {
    return (
      <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-rose-900">
        <div className="flex items-start gap-3">
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
          <div>
            <h3 className="text-sm font-semibold">Failed to load DSM</h3>
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

  if (!dsm || !processed) return null

  const mainPkg = dsm.module || project.package

  return (
    <div className="flex flex-col gap-4">
      {/* Toolbar */}
      <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-3.5 shadow-xs lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-wrap items-center gap-4">
          <label className="inline-flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-700">
            <input
              type="checkbox"
              checked={showWeighted}
              onChange={(e) => setShowWeighted(e.target.checked)}
              className="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
            />
            Show Weighted Counts
          </label>

          <label className="inline-flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-700">
            <input
              type="checkbox"
              checked={showAggregated}
              onChange={(e) => {
                setShowAggregated(e.target.checked)
                setSelectedCell(null)
                setHoveredCell(null)
              }}
              className="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
            />
            Aggregate by Package Layer (4×4)
          </label>

          {!showAggregated && (
            <label className="inline-flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-700">
              <input
                type="checkbox"
                checked={hideNonInternalCols}
                onChange={(e) => {
                  setHideNonInternalCols(e.target.checked)
                  setSelectedCell(null)
                  setHoveredCell(null)
                }}
                className="h-4 w-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
              />
              Internal &amp; Org Only
            </label>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-3">
          {cyclePairs.pairCount > 0 ? (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-rose-50 px-2.5 py-1 text-xs font-semibold text-rose-700 ring-1 ring-rose-600/20">
              <AlertTriangle className="h-3.5 w-3.5" />
              {cyclePairs.pairCount}{' '}
              {cyclePairs.pairCount === 1
                ? 'Dependency Cycle'
                : 'Dependency Cycles'}
            </span>
          ) : (
            <span className="rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-700 ring-1 ring-emerald-600/20">
              0 Dependency Cycles
            </span>
          )}

          {durationMs !== null && (
            <span className="text-[11px] text-slate-400">
              {processed.labels.length}×{processed.labels.length} · {durationMs}{' '}
              ms
            </span>
          )}
        </div>
      </div>

      {/* Cell Inspector Banner */}
      <div className="rounded-xl border border-slate-200 bg-white px-4 py-3 text-xs shadow-xs">
        {activeCell &&
        activeCell.row < processed.labels.length &&
        activeCell.col < processed.labels.length ? (
          <CellInspector
            cell={activeCell}
            processed={processed}
            mainPkg={mainPkg}
            isAggregated={showAggregated}
            isCycle={cyclePairs.cycles.has(`${activeCell.row}:${activeCell.col}`)}
            isPinned={selectedCell !== null}
            onClearPin={() => setSelectedCell(null)}
          />
        ) : (
          <div className="flex flex-wrap items-center justify-between gap-2 text-slate-500">
            <span>
              Hover or click any matrix cell to inspect{' '}
              <strong className="font-semibold text-slate-700">
                Column (Importer) → Row (Imported)
              </strong>{' '}
              dependencies. Mutual cycles are outlined in rose.
            </span>
            <div className="flex flex-wrap items-center gap-3">
              {LAYER_ORDER.map((layer) => (
                <span key={layer} className="inline-flex items-center gap-1.5">
                  <span
                    className="h-2.5 w-2.5 rounded-full"
                    style={{ backgroundColor: LAYER_META[layer].dotColor }}
                  />
                  {LAYER_META[layer].shortLabel}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Matrix Table */}
      <div className="overflow-auto rounded-xl border border-slate-200 bg-white p-3 shadow-xs">
        <table className="border-collapse text-xs">
          <thead>
            <tr>
              <th className="sticky left-0 z-20 border border-slate-200 bg-slate-50 px-2 py-1.5 text-center font-semibold text-slate-500">
                #
              </th>
              <th className="sticky left-8 z-20 min-w-[220px] border border-slate-200 bg-slate-50 px-3 py-1.5 text-left font-semibold text-slate-700">
                {showAggregated ? 'Package Layer' : 'Package'}
              </th>
              {processed.labels.map((label, colIdx) => {
                const isColActive = activeCell?.col === colIdx
                return (
                  <th
                    key={label}
                    title={label}
                    className={`border border-slate-200 px-1 py-1.5 text-center font-mono text-[11px] font-semibold transition-colors ${
                      showAggregated ? 'min-w-[110px]' : 'min-w-[30px]'
                    } ${
                      isColActive
                        ? 'bg-indigo-100 text-indigo-900'
                        : 'bg-slate-50 text-slate-600'
                    }`}
                  >
                    {showAggregated ? LAYER_META[processed.layers[colIdx]].shortLabel : colIdx + 1}
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {processed.labels.map((rowLabel, rowIdx) => {
              const layer = processed.layers[rowIdx]
              const meta = LAYER_META[layer]
              const isRowActive = activeCell?.row === rowIdx
              const displayLabel = showAggregated
                ? rowLabel
                : shortenPackageName(rowLabel, mainPkg)

              return (
                <tr key={rowLabel}>
                  <td
                    className={`sticky left-0 z-10 w-8 border border-slate-200 px-2 py-1 text-center font-mono text-[11px] font-semibold ${
                      isRowActive
                        ? 'bg-indigo-100 text-indigo-900'
                        : 'bg-slate-50 text-slate-500'
                    }`}
                  >
                    {rowIdx + 1}
                  </td>
                  <td
                    title={rowLabel}
                    className={`sticky left-8 z-10 max-w-[280px] truncate border border-slate-200 px-3 py-1.5 font-mono text-[11px] font-medium ${
                      isRowActive ? 'bg-indigo-50 text-indigo-950' : meta.cellBg
                    }`}
                  >
                    <span className="inline-flex items-center gap-2">
                      <span
                        className="h-2 w-2 shrink-0 rounded-full"
                        style={{ backgroundColor: meta.dotColor }}
                      />
                      <span className="truncate">{displayLabel}</span>
                    </span>
                  </td>

                  {processed.matrix[rowIdx].map((val, colIdx) => {
                    const isDiagonal = rowIdx === colIdx
                    const isCycle = cyclePairs.cycles.has(`${rowIdx}:${colIdx}`)
                    const isSelected =
                      selectedCell?.row === rowIdx &&
                      selectedCell?.col === colIdx
                    const isCrosshair =
                      activeCell !== null &&
                      (activeCell.row === rowIdx || activeCell.col === colIdx)

                    let bgClass = 'bg-white'
                    if (isDiagonal) {
                      bgClass = 'bg-slate-200/80 text-slate-400'
                    } else if (isCycle) {
                      bgClass = 'bg-rose-100 text-rose-900 font-bold ring-1 ring-inset ring-rose-400'
                    } else if (val > 0) {
                      bgClass = 'bg-indigo-100 text-indigo-950 font-semibold'
                    } else if (isCrosshair) {
                      bgClass = 'bg-slate-50'
                    }

                    return (
                      <td
                        key={`${rowIdx}-${colIdx}`}
                        onMouseEnter={() =>
                          setHoveredCell({ row: rowIdx, col: colIdx })
                        }
                        onMouseLeave={() => setHoveredCell(null)}
                        onClick={() =>
                          setSelectedCell((curr) =>
                            curr?.row === rowIdx && curr?.col === colIdx
                              ? null
                              : { row: rowIdx, col: colIdx },
                          )
                        }
                        className={`cursor-pointer border border-slate-200 text-center font-mono transition-colors select-none ${
                          showAggregated
                            ? 'h-14 text-sm'
                            : 'h-7 w-7 text-[11px]'
                        } ${bgClass} ${
                          isSelected ? 'ring-2 ring-indigo-600 ring-inset' : ''
                        }`}
                      >
                        {isDiagonal ? '·' : val > 0 ? val : ''}
                      </td>
                    )
                  })}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function CellInspector({
  cell,
  processed,
  mainPkg,
  isAggregated,
  isCycle,
  isPinned,
  onClearPin,
}: {
  cell: SelectedCell
  processed: ProcessedDSM
  mainPkg: string
  isAggregated: boolean
  isCycle: boolean
  isPinned: boolean
  onClearPin: () => void
}) {
  const importedName = processed.labels[cell.row]
  const importerName = processed.labels[cell.col]
  const weight = processed.rawMatrix[cell.row][cell.col]
  const reverseWeight = processed.rawMatrix[cell.col][cell.row]
  const isDiagonal = cell.row === cell.col

  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono font-semibold text-slate-900">
          {isAggregated
            ? importerName
            : shortenPackageName(importerName, mainPkg)}
        </span>
        <ArrowRight className="h-3.5 w-3.5 text-slate-400" />
        <span className="font-mono font-semibold text-slate-900">
          {isAggregated
            ? importedName
            : shortenPackageName(importedName, mainPkg)}
        </span>

        {isDiagonal ? (
          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600">
            Self (diagonal)
          </span>
        ) : weight > 0 ? (
          <span className="rounded-full bg-indigo-50 px-2 py-0.5 font-mono text-[11px] font-semibold text-indigo-700 ring-1 ring-indigo-600/20">
            {weight} {weight === 1 ? 'dependency' : 'dependencies'}
          </span>
        ) : (
          <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] text-slate-500">
            No dependency
          </span>
        )}

        {isCycle && (
          <span className="inline-flex items-center gap-1 rounded-full bg-rose-50 px-2 py-0.5 text-[11px] font-semibold text-rose-700 ring-1 ring-rose-600/20">
            <AlertTriangle className="h-3 w-3" />
            Mutual Cycle (reverse: {reverseWeight})
          </span>
        )}
      </div>

      {isPinned && (
        <button
          type="button"
          onClick={onClearPin}
          className="text-xs font-medium text-indigo-600 hover:underline"
        >
          Unpin cell
        </button>
      )}
    </div>
  )
}
