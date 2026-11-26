package arch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-architect/go-architect-lib/project"
	"github.com/stretchr/testify/require"
)

func TestArchAnalyzers(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.27\n"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "core"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "core", "core.go"), []byte(`package core

type Store interface {
	Get(id string) string
}

type Item struct{ ID string }
`), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "svc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "svc", "svc.go"), []byte(`package svc

import (
	"fmt"
	"example.com/app/core"
	"example.com/org/sdk"
	"github.com/third/party"
)

func Run(s core.Store, item core.Item) string {
	_ = sdk.Ping()
	_ = party.Do()
	return fmt.Sprintf("%s:%s", s.Get(item.ID), item.ID)
}
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import (
	"example.com/app/core"
	"example.com/app/svc"
)

func main() {
	_ = svc.Run(nil, core.Item{ID: "1"})
}
`), 0o644))

	// Test file importing testing + core should be excluded by default.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "core", "core_test.go"), []byte(`package core

import "testing"

func TestCore(t *testing.T) {}
`), 0o644))

	proj, err := project.LoadProjectInfo(dir)
	require.NoError(t, err)
	proj.OrganizationPackages = []string{"example.com/org"}

	// 1. Instability (excludes core_test.go, so core remains 100% abstract)
	instab, err := GetInstability(proj)
	require.NoError(t, err)
	require.Len(t, instab, 3)

	byName := make(map[string]int)
	for i, p := range instab {
		byName[p.PackageName] = i
	}
	corePkg := instab[byName["example.com/app/core"]]
	require.Equal(t, 2, corePkg.AfferentCoupling)
	require.Equal(t, 0, corePkg.EfferentCoupling)
	require.Equal(t, 0.0, corePkg.Instability)
	require.Equal(t, 1.0, corePkg.Abstractness)
	require.Equal(t, 0.0, corePkg.Distance)

	// 2. Dependency Graph (excludes "testing" from core_test.go)
	graph, err := GetDependencyGraph(proj)
	require.NoError(t, err)
	require.Equal(t, "example.com/app", graph.Module)
	require.Equal(t, []string{"example.com/app", "example.com/app/core", "example.com/app/svc"}, graph.Internal)
	require.Equal(t, []string{"example.com/org/sdk"}, graph.Organization)
	require.Equal(t, []string{"github.com/third/party"}, graph.External)
	require.Equal(t, []string{"fmt"}, graph.StandardLib)

	// 3. DSM (default excludes tests; includeTests=true includes "testing")
	matrix, err := GetDSM(proj, false)
	require.NoError(t, err)
	require.Len(t, matrix.Packages, 6)
	require.Len(t, matrix.Dependencies, 6)

	matrixWithTests, err := GetDSM(proj, true)
	require.NoError(t, err)
	require.Len(t, matrixWithTests.Packages, 7)
	require.Contains(t, matrixWithTests.Packages, "testing")

	pkgIdx := make(map[string]int)
	for i, p := range matrix.Packages {
		pkgIdx[p] = i
	}
	// svc imports core (ImportSpec + core.Store + core.Item = 3)
	require.Equal(t, int64(3), matrix.Dependencies[pkgIdx["example.com/app/core"]][pkgIdx["example.com/app/svc"]])

	// 4. Coupling (default excludes tests; includeTests=true includes testing)
	coup, err := CalculateCoupling(proj, "example.com/app/core", false)
	require.NoError(t, err)
	require.Equal(t, 5, coup.CouplingLevel) // svc (3) + main (2)
	require.Len(t, coup.PackageDetails, 2)
	require.Equal(t, "example.com/app/svc", coup.PackageDetails[0].Package)

	coupTestingDefault, err := CalculateCoupling(proj, "testing", false)
	require.NoError(t, err)
	require.Equal(t, 0, coupTestingDefault.CouplingLevel)

	coupTestingIncluded, err := CalculateCoupling(proj, "testing", true)
	require.NoError(t, err)
	require.Greater(t, coupTestingIncluded.CouplingLevel, 0)

	// 5. Graph Density from DSM
	density := CalculateDensityFromDSM(matrix)
	require.Equal(t, 3, density.InternalPackages)
	require.Equal(t, 3, density.DirectEdges)
	require.Equal(t, 50.0, density.PropagationCost)
	require.Equal(t, 2, density.MaxDependencyDepth)
	require.Equal(t, 0, density.CycleCount)
}

func TestCalculateGraphDensityCyclesAndEdgeCases(t *testing.T) {
	// Empty and single-node graphs
	require.Equal(t, GraphDensity{}, CalculateGraphDensity(nil, nil))
	require.Equal(t, GraphDensity{InternalPackages: 1}, CalculateGraphDensity([]string{"a"}, nil))

	// Linear chain A -> B -> C -> D (4 nodes, 6 reachable pairs out of 12 = 50%, depth 3)
	chain := CalculateGraphDensity(
		[]string{"a", "b", "c", "d"},
		map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"d"},
		},
	)
	require.Equal(t, 4, chain.InternalPackages)
	require.Equal(t, 3, chain.DirectEdges)
	require.Equal(t, 50.0, chain.PropagationCost)
	require.Equal(t, 3, chain.MaxDependencyDepth)
	require.Equal(t, 0, chain.CycleCount)

	// Graph with a 3-node cycle (a -> b -> c -> a) plus leaf (c -> d)
	cyclic := CalculateGraphDensity(
		[]string{"a", "b", "c", "d"},
		map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"a", "d"},
		},
	)
	require.Equal(t, 4, cyclic.InternalPackages)
	require.Equal(t, 4, cyclic.DirectEdges)
	require.Equal(t, 1, cyclic.CycleCount)
	require.Equal(t, 1, cyclic.MaxDependencyDepth) // SCC{a,b,c} -> SCC{d}
	require.Equal(t, 75.0, cyclic.PropagationCost) // 9 reachable pairs out of 12
}
