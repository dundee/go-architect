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

	proj, err := project.LoadProjectInfo(dir)
	require.NoError(t, err)
	proj.OrganizationPackages = []string{"example.com/org"}

	// 1. Instability
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

	// 2. Dependency Graph
	graph, err := GetDependencyGraph(proj)
	require.NoError(t, err)
	require.Equal(t, "example.com/app", graph.Module)
	require.Equal(t, []string{"example.com/app", "example.com/app/core", "example.com/app/svc"}, graph.Internal)
	require.Equal(t, []string{"example.com/org/sdk"}, graph.Organization)
	require.Equal(t, []string{"github.com/third/party"}, graph.External)
	require.Equal(t, []string{"fmt"}, graph.StandardLib)

	// 3. DSM
	matrix, err := GetDSM(proj)
	require.NoError(t, err)
	require.Len(t, matrix.Packages, 6)
	require.Len(t, matrix.Dependencies, 6)

	pkgIdx := make(map[string]int)
	for i, p := range matrix.Packages {
		pkgIdx[p] = i
	}
	// svc imports core (ImportSpec + core.Store + core.Item = 3)
	require.Equal(t, int64(3), matrix.Dependencies[pkgIdx["example.com/app/core"]][pkgIdx["example.com/app/svc"]])

	// 4. Coupling
	coup, err := CalculateCoupling(proj, "example.com/app/core")
	require.NoError(t, err)
	require.Equal(t, 5, coup.CouplingLevel) // svc (3) + main (2)
	require.Len(t, coup.PackageDetails, 2)
	require.Equal(t, "example.com/app/svc", coup.PackageDetails[0].Package)
}
