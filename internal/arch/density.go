package arch

import "github.com/go-architect/go-architect-lib/dsm"

type GraphDensity struct {
	InternalPackages   int     `json:"internal_packages"`
	DirectEdges        int     `json:"direct_edges"`
	PropagationCost    float64 `json:"propagation_cost"`
	MaxDependencyDepth int     `json:"max_dependency_depth"`
	CycleCount         int     `json:"cycle_count"`
}

// CalculateDensityFromDSM computes internal package coupling density metrics
// (Propagation Cost, Max Dependency Depth, and SCC cycle count) from a DSM.
func CalculateDensityFromDSM(matrix *dsm.DependencyStructureMatrix) GraphDensity {
	if matrix == nil || len(matrix.Packages) == 0 {
		return GraphDensity{}
	}

	var nodes []string
	indexByPkg := make(map[string]int, len(matrix.Packages))
	for i, pkg := range matrix.Packages {
		if matrix.Module == "" || isInternalPackage(pkg, matrix.Module) {
			nodes = append(nodes, pkg)
			indexByPkg[pkg] = i
		}
	}

	edges := make(map[string][]string, len(nodes))
	for _, colPkg := range nodes {
		col := indexByPkg[colPkg]
		for _, rowPkg := range nodes {
			if rowPkg == colPkg {
				continue
			}
			row := indexByPkg[rowPkg]
			if row < len(matrix.Dependencies) && col < len(matrix.Dependencies[row]) && matrix.Dependencies[row][col] > 0 {
				edges[colPkg] = append(edges[colPkg], rowPkg)
			}
		}
	}

	return CalculateGraphDensity(nodes, edges)
}

// CalculateGraphDensity computes Propagation Cost (percentage of ordered internal
// package pairs (u != v) connected directly or transitively), Max Dependency Depth
// (longest path in the SCC condensation DAG), and CycleCount (number of SCCs with > 1 node).
func CalculateGraphDensity(nodes []string, edges map[string][]string) GraphDensity {
	n := len(nodes)
	if n == 0 {
		return GraphDensity{}
	}

	idxOf := make(map[string]int, n)
	for i, name := range nodes {
		idxOf[name] = i
	}

	adj := make([][]int, n)
	directEdges := 0
	for uName, targets := range edges {
		u, ok := idxOf[uName]
		if !ok {
			continue
		}
		seen := make(map[int]struct{}, len(targets))
		for _, vName := range targets {
			v, ok := idxOf[vName]
			if !ok || u == v {
				continue
			}
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			adj[u] = append(adj[u], v)
			directEdges++
		}
	}

	// 1. Transitive reachability (BFS per node) -> Propagation Cost.
	reachablePairs := 0
	visited := make([]int, n)
	queue := make([]int, 0, n)
	for start := 0; start < n; start++ {
		stamp := start + 1
		visited[start] = stamp
		queue = queue[:0]
		queue = append(queue, start)
		for head := 0; head < len(queue); head++ {
			u := queue[head]
			for _, v := range adj[u] {
				if visited[v] != stamp {
					visited[v] = stamp
					reachablePairs++
					queue = append(queue, v)
				}
			}
		}
	}

	var propCost float64
	if n > 1 {
		propCost = round2(100 * float64(reachablePairs) / float64(n*(n-1)))
	}

	// 2. Tarjan's SCC -> CycleCount + condensation DAG longest path (MaxDependencyDepth).
	sccs := tarjanSCC(n, adj)
	compOf := make([]int, n)
	cycleCount := 0
	for compID, comp := range sccs {
		if len(comp) > 1 {
			cycleCount++
		}
		for _, u := range comp {
			compOf[u] = compID
		}
	}

	numComps := len(sccs)
	dagAdj := make([][]int, numComps)
	for u := 0; u < n; u++ {
		cu := compOf[u]
		seenComp := make(map[int]struct{})
		for _, v := range adj[u] {
			cv := compOf[v]
			if cu == cv {
				continue
			}
			if _, ok := seenComp[cv]; ok {
				continue
			}
			seenComp[cv] = struct{}{}
			dagAdj[cu] = append(dagAdj[cu], cv)
		}
	}

	// Tarjan emits SCCs in reverse topological order (sinks first, sources last).
	// Computing dp[c] = max_{next in dagAdj[c]} (1 + dp[next]) in emission order gives longest path.
	dp := make([]int, numComps)
	maxDepth := 0
	for c := 0; c < numComps; c++ {
		best := 0
		for _, next := range dagAdj[c] {
			if cand := 1 + dp[next]; cand > best {
				best = cand
			}
		}
		dp[c] = best
		if best > maxDepth {
			maxDepth = best
		}
	}

	return GraphDensity{
		InternalPackages:   n,
		DirectEdges:        directEdges,
		PropagationCost:    propCost,
		MaxDependencyDepth: maxDepth,
		CycleCount:         cycleCount,
	}
}

func tarjanSCC(n int, adj [][]int) [][]int {
	index := 0
	indices := make([]int, n)
	lowlink := make([]int, n)
	onStack := make([]bool, n)
	for i := range indices {
		indices[i] = -1
	}
	var stack []int
	var sccs [][]int

	var strongConnect func(v int)
	strongConnect = func(v int) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range adj[v] {
			if indices[w] == -1 {
				strongConnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
			}
		}

		if lowlink[v] == indices[v] {
			var comp []int
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, comp)
		}
	}

	for v := 0; v < n; v++ {
		if indices[v] == -1 {
			strongConnect(v)
		}
	}
	return sccs
}
