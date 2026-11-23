import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  MiniMap,
  type Node,
  type NodeProps,
  Position,
  ReactFlow,
  type Edge,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import dagre from '@dagrejs/dagre'
import {
  AlertCircle,
  ArrowDownUp,
  ArrowLeftRight,
  ArrowRight,
  Layers,
  Loader2,
  RefreshCw,
  Search,
  X,
} from 'lucide-react'
import { fetchDependencyGraph } from '../api/client'
import type { ModuleDependencyGraph, ProjectInfo } from '../types/api'
import {
  LAYER_META,
  LAYER_ORDER,
  type PackageLayer,
  shortenPackageName,
} from '../utils/packages'

interface DependencyGraphViewProps {
  project: ProjectInfo
  refreshSignal: number
  onInspectCoupling: (dependency: string) => void
}

interface PackageNodeData extends Record<string, unknown> {
  label: string
  fullName: string
  layer: PackageLayer
  isSelected: boolean
  isNeighbor: boolean
  isDimmed: boolean
  isMatch: boolean
}

type PackageFlowNode = Node<PackageNodeData, 'packageNode'>

function PackageNode({ data }: NodeProps<PackageFlowNode>) {
  const meta = LAYER_META[data.layer]
  const positionLeft = Position.Left
  const positionRight = Position.Right
  const positionTop = Position.Top
  const positionBottom = Position.Bottom

  return (
    <div
      title={data.fullName}
      className={`rounded-xl border-2 px-3 py-2 shadow-xs transition-all duration-150 ${
        data.isDimmed ? 'opacity-25' : 'opacity-100'
      } ${
        data.isSelected
          ? 'ring-4 ring-indigo-500/30 shadow-md'
          : data.isMatch
            ? 'ring-2 ring-indigo-400'
            : ''
      }`}
      style={{
        backgroundColor: meta.nodeBg,
        borderColor: data.isSelected ? '#4f46e5' : meta.nodeBorder,
        color: meta.nodeText,
        width: 220,
      }}
    >
      <Handle
        type="target"
        position={positionLeft}
        className="h-2! w-2! border-white! bg-slate-400!"
      />
      <Handle
        type="target"
        position={positionTop}
        id="top"
        className="h-2! w-2! border-white! bg-slate-400! opacity-0!"
      />
      <div className="flex items-center justify-between gap-2">
        <span className="truncate font-mono text-xs font-semibold">
          {data.label}
        </span>
        <span
          className="h-2 w-2 shrink-0 rounded-full"
          style={{ backgroundColor: meta.dotColor }}
        />
      </div>
      <div className="mt-0.5 truncate font-mono text-[10px] opacity-70">
        {meta.shortLabel}
      </div>
      <Handle
        type="source"
        position={positionRight}
        className="h-2! w-2! border-white! bg-slate-400!"
      />
      <Handle
        type="source"
        position={positionBottom}
        id="bottom"
        className="h-2! w-2! border-white! bg-slate-400! opacity-0!"
      />
    </div>
  )
}

const NODE_TYPES = {
  packageNode: PackageNode,
}

const NODE_WIDTH = 220
const NODE_HEIGHT = 54

export function DependencyGraphView({
  project,
  refreshSignal,
  onInspectCoupling,
}: DependencyGraphViewProps) {
  const [graph, setGraph] = useState<ModuleDependencyGraph | null>(null)
  const [durationMs, setDurationMs] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [visibleLayers, setVisibleLayers] = useState<Record<PackageLayer, boolean>>({
    internal: true,
    organization: true,
    external: false,
    standard: false,
  })
  const [rankDir, setRankDir] = useState<'LR' | 'TB'>('LR')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedPkg, setSelectedPkg] = useState<string | null>(null)

  const loadData = useCallback((signal?: AbortSignal) => {
    setLoading(true)
    setError(null)
    fetchDependencyGraph(signal)
      .then((res) => {
        if (signal?.aborted) return
        setGraph(res.graph)
        setDurationMs(res.duration_ms ?? null)
        setSelectedPkg(null)
      })
      .catch((err: unknown) => {
        if (signal?.aborted) return
        setError(
          err instanceof Error
            ? err.message
            : 'Failed to load dependency graph',
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

  const layerMap = useMemo(() => {
    const map = new Map<string, PackageLayer>()
    if (!graph) return map
    for (const p of graph.internal) map.set(p, 'internal')
    for (const p of graph.organization) map.set(p, 'organization')
    for (const p of graph.external) map.set(p, 'external')
    for (const p of graph.standard) map.set(p, 'standard')
    return map
  }, [graph])

  const reverseRelations = useMemo(() => {
    const rev = new Map<string, string[]>()
    if (!graph) return rev
    for (const [source, targets] of Object.entries(graph.relations)) {
      for (const target of targets ?? []) {
        const list = rev.get(target)
        if (list) {
          list.push(source)
        } else {
          rev.set(target, [source])
        }
      }
    }
    return rev
  }, [graph])

  const selectedNeighbors = useMemo(() => {
    if (!selectedPkg || !graph) {
      return {
        dependsOn: [] as string[],
        dependants: [] as string[],
        neighborSet: new Set<string>(),
      }
    }
    const dependsOn = [...(graph.relations[selectedPkg] ?? [])].sort()
    const dependants = [...(reverseRelations.get(selectedPkg) ?? [])].sort()
    const neighborSet = new Set<string>([selectedPkg, ...dependsOn, ...dependants])
    return { dependsOn, dependants, neighborSet }
  }, [selectedPkg, graph, reverseRelations])

  const { nodes, edges, hiddenEdgesCount } = useMemo(() => {
    if (!graph) {
      return {
        nodes: [] as PackageFlowNode[],
        edges: [] as Edge[],
        hiddenEdgesCount: 0,
      }
    }

    const activeSet = new Set<string>()
    for (const [pkg, layer] of layerMap.entries()) {
      if (visibleLayers[layer]) {
        activeSet.add(pkg)
      }
    }

    // Build Dagre graph
    const dagreGraph = new dagre.graphlib.Graph()
    dagreGraph.setDefaultEdgeLabel(() => ({}))
    dagreGraph.setGraph({
      rankdir: rankDir,
      nodesep: 36,
      ranksep: 80,
      marginx: 24,
      marginy: 24,
    })

    for (const pkg of activeSet) {
      dagreGraph.setNode(pkg, { width: NODE_WIDTH, height: NODE_HEIGHT })
    }

    const visibleEdges: Array<{ source: string; target: string }> = []
    let hiddenCount = 0

    for (const [source, targets] of Object.entries(graph.relations)) {
      for (const target of targets ?? []) {
        if (activeSet.has(source) && activeSet.has(target)) {
          dagreGraph.setEdge(source, target)
          visibleEdges.push({ source, target })
        } else {
          hiddenCount++
        }
      }
    }

    dagre.layout(dagreGraph)

    const q = searchQuery.trim().toLowerCase()
    const hasSelection = selectedPkg !== null && activeSet.has(selectedPkg)

    const flowNodes: PackageFlowNode[] = Array.from(activeSet).map((pkg) => {
      const pos = dagreGraph.node(pkg)
      const layer = layerMap.get(pkg) ?? 'external'
      const isSelected = pkg === selectedPkg
      const isNeighbor = hasSelection && selectedNeighbors.neighborSet.has(pkg)
      const isMatch = q.length > 0 && pkg.toLowerCase().includes(q)
      const isDimmed =
        (hasSelection && !isNeighbor) || (q.length > 0 && !isMatch && !hasSelection)

      return {
        id: pkg,
        type: 'packageNode',
        position: {
          x: (pos?.x ?? 0) - NODE_WIDTH / 2,
          y: (pos?.y ?? 0) - NODE_HEIGHT / 2,
        },
        sourcePosition: rankDir === 'LR' ? Position.Right : Position.Bottom,
        targetPosition: rankDir === 'LR' ? Position.Left : Position.Top,
        data: {
          label: shortenPackageName(pkg, graph.module || project.package),
          fullName: pkg,
          layer,
          isSelected,
          isNeighbor,
          isDimmed,
          isMatch,
        },
      }
    })

    const flowEdges: Edge[] = visibleEdges.map(({ source, target }) => {
      const touchesSelection =
        hasSelection && (source === selectedPkg || target === selectedPkg)
      const isOutgoing = hasSelection && source === selectedPkg
      const isIncoming = hasSelection && target === selectedPkg

      const strokeColor = isOutgoing
        ? '#4f46e5'
        : isIncoming
          ? '#059669'
          : '#94a3b8'

      return {
        id: `${source}->${target}`,
        source,
        target,
        animated: touchesSelection,
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: strokeColor,
          width: 16,
          height: 16,
        },
        style: {
          stroke: strokeColor,
          strokeWidth: touchesSelection ? 2.25 : 1.25,
          opacity: hasSelection && !touchesSelection ? 0.14 : 0.78,
        },
      }
    })

    return { nodes: flowNodes, edges: flowEdges, hiddenEdgesCount: hiddenCount }
  }, [
    graph,
    layerMap,
    visibleLayers,
    rankDir,
    searchQuery,
    selectedPkg,
    selectedNeighbors,
    project.package,
  ])

  const layerCounts = useMemo(() => {
    return {
      internal: graph?.internal.length ?? 0,
      organization: graph?.organization.length ?? 0,
      external: graph?.external.length ?? 0,
      standard: graph?.standard.length ?? 0,
    }
  }, [graph])

  if (loading && !graph) {
    return (
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-2.5 rounded-xl border border-indigo-200 bg-indigo-50/70 px-4 py-3 text-sm font-medium text-indigo-900">
          <Loader2 className="h-4 w-4 animate-spin text-indigo-600" />
          Building module dependency graph...
        </div>
        <div className="h-[580px] animate-pulse rounded-xl border border-slate-200 bg-white" />
      </div>
    )
  }

  if (error && !graph) {
    return (
      <div className="rounded-xl border border-rose-200 bg-rose-50 p-6 text-rose-900">
        <div className="flex items-start gap-3">
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
          <div>
            <h3 className="text-sm font-semibold">
              Failed to build Dependency Graph
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

  if (!graph) return null

  const activeModule = graph.module || project.package

  return (
    <div className="flex flex-col gap-4">
      {/* Controls Bar */}
      <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-3.5 shadow-xs lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1 text-xs font-medium text-slate-500">
            <Layers className="h-3.5 w-3.5" />
            Layers:
          </span>
          {LAYER_ORDER.map((layer) => {
            const meta = LAYER_META[layer]
            const active = visibleLayers[layer]
            return (
              <button
                key={layer}
                type="button"
                onClick={() =>
                  setVisibleLayers((prev) => ({
                    ...prev,
                    [layer]: !prev[layer],
                  }))
                }
                className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition ${
                  active
                    ? 'bg-slate-900 text-white shadow-2xs'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                }`}
              >
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: meta.dotColor }}
                />
                <span>{meta.shortLabel}</span>
                <span className="opacity-70">({layerCounts[layer]})</span>
              </button>
            )
          })}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="relative min-w-[210px] flex-1 sm:flex-initial">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Find package..."
              className="w-full rounded-lg border border-slate-200 bg-slate-50 py-1.5 pr-7 pl-8 text-xs text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:bg-white focus:ring-2 focus:ring-indigo-500/20 focus:outline-none"
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

          <button
            type="button"
            onClick={() => setRankDir((d) => (d === 'LR' ? 'TB' : 'LR'))}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-50"
            title="Toggle graph orientation"
          >
            {rankDir === 'LR' ? (
              <>
                <ArrowLeftRight className="h-3.5 w-3.5 text-slate-500" />
                Left → Right
              </>
            ) : (
              <>
                <ArrowDownUp className="h-3.5 w-3.5 text-slate-500" />
                Top → Bottom
              </>
            )}
          </button>

          {durationMs !== null && (
            <span className="text-[11px] text-slate-400">
              {nodes.length} nodes · {edges.length} edges ({durationMs} ms)
            </span>
          )}
        </div>
      </div>

      {/* Graph + Inspector Split */}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-12">
        <div className="relative h-[620px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xs xl:col-span-8">
          {loading && (
            <div className="absolute top-3 left-3 z-10 inline-flex items-center gap-1.5 rounded-lg bg-indigo-50 px-2.5 py-1 text-xs font-medium text-indigo-800 shadow-xs">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              Refreshing...
            </div>
          )}

          {nodes.length === 0 ? (
            <div className="flex h-full items-center justify-center p-6 text-sm text-slate-500">
              Enable at least one Package Layer above to render nodes.
            </div>
          ) : (
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={NODE_TYPES}
              fitView
              fitViewOptions={{ padding: 0.18 }}
              minZoom={0.15}
              maxZoom={2}
              onNodeClick={(_, node) =>
                setSelectedPkg((curr) => (curr === node.id ? null : node.id))
              }
              onPaneClick={() => setSelectedPkg(null)}
            >
              <Background color="#e2e8f0" gap={20} />
              <Controls showInteractive={false} />
              <MiniMap
                pannable
                zoomable
                nodeColor={(node) => {
                  const layer = (node.data as PackageNodeData | undefined)?.layer
                  return layer ? LAYER_META[layer].dotColor : '#94a3b8'
                }}
              />
            </ReactFlow>
          )}
        </div>

        {/* Neighbor Details Inspector */}
        <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-xs xl:col-span-4">
          {!selectedPkg ? (
            <div className="flex h-full flex-col justify-between gap-4">
              <div>
                <h2 className="text-sm font-semibold text-slate-900">
                  Package Inspector
                </h2>
                <p className="mt-1 text-xs text-slate-500">
                  Click any node in the graph to highlight its direct{' '}
                  <span className="font-medium text-indigo-600">
                    Depends on
                  </span>{' '}
                  and{' '}
                  <span className="font-medium text-emerald-600">
                    Dependant
                  </span>{' '}
                  neighbors.
                </p>
              </div>
              {hiddenEdgesCount > 0 && (
                <div className="rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600">
                  {hiddenEdgesCount} dependency edges to hidden layers (
                  <code className="font-mono">external</code> /{' '}
                  <code className="font-mono">standard</code>) are currently
                  filtered out for readability. Use the layer toggles above to
                  show them.
                </div>
              )}
            </div>
          ) : (
            <div className="flex flex-col gap-4">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span
                      className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${
                        LAYER_META[layerMap.get(selectedPkg) ?? 'external']
                          .badgeClass
                      }`}
                    >
                      {
                        LAYER_META[layerMap.get(selectedPkg) ?? 'external']
                          .shortLabel
                      }
                    </span>
                  </div>
                  <h2
                    className="mt-1.5 font-mono text-xs font-bold break-all text-slate-900"
                    title={selectedPkg}
                  >
                    {selectedPkg}
                  </h2>
                </div>
                <button
                  type="button"
                  onClick={() => setSelectedPkg(null)}
                  className="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
                  aria-label="Clear selected node"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>

              <button
                type="button"
                onClick={() => onInspectCoupling(selectedPkg)}
                className="inline-flex items-center justify-center gap-1.5 rounded-lg bg-indigo-50 px-3 py-2 text-xs font-semibold text-indigo-700 ring-1 ring-indigo-600/20 transition hover:bg-indigo-100"
              >
                <span>Inspect Coupling to This Package</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </button>

              <NeighborList
                title={`Depends on (${selectedNeighbors.dependsOn.length})`}
                accentClass="text-indigo-700"
                items={selectedNeighbors.dependsOn}
                activeModule={activeModule}
                layerMap={layerMap}
                onSelect={(pkg) => {
                  const layer = layerMap.get(pkg)
                  if (layer && !visibleLayers[layer]) {
                    setVisibleLayers((prev) => ({ ...prev, [layer]: true }))
                  }
                  setSelectedPkg(pkg)
                }}
              />

              <NeighborList
                title={`Dependants (${selectedNeighbors.dependants.length})`}
                accentClass="text-emerald-700"
                items={selectedNeighbors.dependants}
                activeModule={activeModule}
                layerMap={layerMap}
                onSelect={(pkg) => {
                  const layer = layerMap.get(pkg)
                  if (layer && !visibleLayers[layer]) {
                    setVisibleLayers((prev) => ({ ...prev, [layer]: true }))
                  }
                  setSelectedPkg(pkg)
                }}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function NeighborList({
  title,
  accentClass,
  items,
  activeModule,
  layerMap,
  onSelect,
}: {
  title: string
  accentClass: string
  items: string[]
  activeModule: string
  layerMap: Map<string, PackageLayer>
  onSelect: (pkg: string) => void
}) {
  return (
    <div>
      <h3 className={`mb-1.5 text-xs font-semibold ${accentClass}`}>{title}</h3>
      {items.length === 0 ? (
        <div className="rounded-lg border border-dashed border-slate-200 px-3 py-2 text-xs text-slate-400">
          None
        </div>
      ) : (
        <ul className="max-h-48 divide-y divide-slate-100 overflow-auto rounded-lg border border-slate-200">
          {items.map((pkg) => {
            const layer = layerMap.get(pkg) ?? 'external'
            return (
              <li key={pkg}>
                <button
                  type="button"
                  onClick={() => onSelect(pkg)}
                  title={pkg}
                  className="flex w-full items-center justify-between gap-2 px-2.5 py-1.5 text-left text-xs transition hover:bg-slate-50"
                >
                  <span className="truncate font-mono text-slate-800">
                    {shortenPackageName(pkg, activeModule)}
                  </span>
                  <span
                    className="h-2 w-2 shrink-0 rounded-full"
                    style={{ backgroundColor: LAYER_META[layer].dotColor }}
                  />
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
