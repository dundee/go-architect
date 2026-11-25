package cache

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dundee/go-architect/backend/git"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/go-architect/go-architect-lib/coupling"
	"github.com/go-architect/go-architect-lib/dependency"
	"github.com/go-architect/go-architect-lib/dsm"
	"github.com/go-architect/go-architect-lib/instability"
	"github.com/go-architect/go-architect-lib/metrics/comments"
	"github.com/go-architect/go-architect-lib/metrics/interfaces"
	"github.com/go-architect/go-architect-lib/metrics/loc"
	"github.com/go-architect/go-architect-lib/metrics/types"
	"github.com/stretchr/testify/require"
)

func TestProtobufRoundTrips(t *testing.T) {
	store := NewStore(t.TempDir())
	key := ProjectKey("example.com/mod", ".", []string{"example.com/org"})
	sha := "abcdef1234567890"

	// Complexity (separate file)
	comp := &complexity.Report{
		Cyclomatic: complexity.Output{
			AverageComplexity: 2.5,
			ComplexityDetails: []complexity.Detail{{Package: "example.com/mod", File: "main.go:10", Function: "run", Complexity: 4}},
		},
		Cognitive: complexity.Output{
			AverageComplexity: 1.5,
			ComplexityDetails: []complexity.Detail{{Package: "example.com/mod", File: "main.go:10", Function: "run", Complexity: 3}},
		},
	}
	require.NoError(t, store.PutComplexity(key, sha, comp))
	gotComp, ok := store.GetComplexity(key, sha)
	require.True(t, ok)
	require.Equal(t, comp, gotComp)

	// Instability
	instab := []*instability.PackageInstability{{
		PackageName:          "example.com/mod",
		AbstractionsCount:    1,
		ImplementationsCount: 2,
		AfferentCoupling:     3,
		EfferentCoupling:     1,
		Instability:          0.25,
		Abstractness:         0.33,
		Distance:             0.42,
	}}
	require.NoError(t, store.PutInstability(key, sha, instab))
	gotInstab, ok := store.GetInstability(key, sha)
	require.True(t, ok)
	require.Equal(t, instab, gotInstab)

	// DependencyGraph
	graph := &dependency.ModuleDependencyGraph{
		Module:       "example.com/mod",
		Internal:     []string{"example.com/mod"},
		Organization: []string{"example.com/org/lib"},
		External:     []string{"github.com/pkg/errors"},
		StandardLib:  []string{"fmt"},
		Relations:    map[string][]string{"example.com/mod": {"fmt", "example.com/org/lib"}},
	}
	require.NoError(t, store.PutDependencyGraph(key, sha, graph))
	gotGraph, ok := store.GetDependencyGraph(key, sha)
	require.True(t, ok)
	require.Equal(t, graph, gotGraph)

	// DSM
	matrix := &dsm.DependencyStructureMatrix{
		Module:       "example.com/mod",
		Packages:     []string{"a", "b"},
		Dependencies: [][]int64{{0, 2}, {1, 0}},
	}
	require.NoError(t, store.PutDSM(key, sha, matrix))
	gotDSM, ok := store.GetDSM(key, sha)
	require.True(t, ok)
	require.Equal(t, matrix, gotDSM)

	// Coupling
	coup := &coupling.DependencyCoupling{
		Dependency:    "fmt",
		CouplingLevel: 2,
		PackageDetails: []*coupling.PackageCoupling{{
			Package:       "example.com/mod",
			CouplingLevel: 2,
			FileDetails: []*coupling.FileCoupling{{
				Package:       "example.com/mod",
				File:          "main.go",
				FileContent:   "package main\n",
				Lines:         []int{3, 7},
				CouplingLevel: 2,
				Details:       []coupling.Detail{{Line: 3, Comments: "fmt.Println"}},
			}},
		}},
	}
	require.NoError(t, store.PutCoupling(key, sha, "fmt", coup))
	gotCoup, ok := store.GetCoupling(key, sha, "fmt")
	require.True(t, ok)
	require.Equal(t, coup, gotCoup)

	// MetricsBase
	var pt types.ProjectTypes
	pt.ProjectPackage = "example.com/mod"
	pt.Packages = 1
	pt.SourceFiles = 3
	pt.Structs = 2
	var pkgItem types.PackageTypes
	pkgItem.Package = "example.com/mod"
	pkgItem.SourceFiles = 3
	pkgItem.Structs = 2
	pt.PackageDetails = []types.PackageTypes{pkgItem}

	base := &MetricsBase{
		LOC:        &loc.ProjectLOC{Total: 120, Packages: []loc.PackageLOC{{Package: "example.com/mod", LOC: 120}}, Files: []loc.FileLOC{{Package: "example.com/mod", File: "main.go", LOC: 120}}},
		Types:      &pt,
		Interfaces: &interfaces.InterfaceMetrics{AverageMethods: 1.5, InterfaceMaxMethods: []interfaces.InterfaceInfo{{Package: "example.com/mod", File: "main.go", Name: "Runner", Methods: 2}}, InterfaceMinMethods: []interfaces.InterfaceInfo{}},
		Comments:   &comments.CommentsMetrics{TotalLines: 10, CommentsRatio: 0.08, FilesWithComments: 1, FilesWithCommentsRatio: 0.33},
		Tests:      &TestMetrics{LOC: 60, Files: 1, Functions: 3, Ratio: 0.5, LargestFiles: []loc.FileLOC{{Package: "example.com/mod", File: "main_test.go", LOC: 60}}},
	}
	require.NoError(t, store.PutMetricsBase(key, sha, base))
	gotBase, ok := store.GetMetricsBase(key, sha)
	require.True(t, ok)
	require.Equal(t, base, gotBase)

	// VCS
	since := time.Unix(1700000000, 0).UTC()
	vcs := &git.VCSAnalysisInfo{
		Since: &since,
		AuthorsInfo: git.AuthorsInfo{
			TotalAuthors:       1,
			TotalCommits:       2,
			TotalModifications: 15,
			AuthorDetails:      []git.AuthorInfo{{Name: "Alice", Commits: 2, ModifiedLines: 15}},
		},
		ModificationsInfo: git.Modifications{
			PathModifications: []git.ModificationsInfo{{Source: "pkg/", TotalModifications: 15, AuthorsSummary: []git.ContributorSummary{{Name: "Alice", Modifications: 15}}}},
			FileModifications: []git.ModificationsInfo{{Source: "pkg/a.go", TotalModifications: 15, AuthorsSummary: []git.ContributorSummary{{Name: "Alice", Modifications: 15}}}},
		},
	}
	require.NoError(t, store.PutVCS(key, sha, 6, "2026-11-24", vcs))
	gotVCS, ok := store.GetVCS(key, sha, 6, "2026-11-24")
	require.True(t, ok)
	require.Equal(t, vcs, gotVCS)
}

func TestResolveCleanCommit(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}

	run("init")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "init")

	sha, clean := ResolveCleanCommit(context.Background(), dir)
	require.True(t, clean)
	require.Len(t, sha, 40)

	// Untracked/dirty file marks working tree dirty.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dirty.go"), []byte("package a\n"), 0o644))
	_, clean = ResolveCleanCommit(context.Background(), dir)
	require.False(t, clean)
}
