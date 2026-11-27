package trends

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dundee/go-architect/internal/cache/cachepb"
	"github.com/go-architect/go-architect-lib/project"
	"github.com/stretchr/testify/require"
)

func runGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
}

func TestTrendsAnalyzeAndCache(t *testing.T) {
	repoDir := t.TempDir()
	cacheDir := t.TempDir()

	runGit(t, repoDir, nil, "init")
	runGit(t, repoDir, nil, "config", "user.name", "Test User")
	runGit(t, repoDir, nil, "config", "user.email", "test@example.com")

	// Commit 1: no go.mod yet (should be gracefully skipped when analyzed)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# init\n"), 0o644))
	runGit(t, repoDir, []string{
		"GIT_AUTHOR_DATE=2025-01-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2025-01-01T10:00:00Z",
	}, "add", ".")
	runGit(t, repoDir, []string{
		"GIT_AUTHOR_DATE=2025-01-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2025-01-01T10:00:00Z",
	}, "commit", "-m", "initial readme without go.mod")

	// Commit 2: valid Go module with 1 struct & 1 func, tagged v0.1.0
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/sample\n\ngo 1.24\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\n\ntype Foo struct{}\n\nfunc main() {}\n"), 0o644))
	runGit(t, repoDir, []string{
		"GIT_AUTHOR_DATE=2025-02-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2025-02-01T10:00:00Z",
	}, "add", ".")
	runGit(t, repoDir, []string{
		"GIT_AUTHOR_DATE=2025-02-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2025-02-01T10:00:00Z",
	}, "commit", "-m", "add go module")
	runGit(t, repoDir, nil, "tag", "v0.1.0")

	// Commit 3: add another file + interface (HEAD)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "service.go"), []byte("package main\n\n// Service runs work.\ntype Service interface {\n\tRun()\n}\n"), 0o644))
	runGit(t, repoDir, []string{
		"GIT_AUTHOR_DATE=2025-03-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2025-03-01T10:00:00Z",
	}, "add", ".")
	runGit(t, repoDir, []string{
		"GIT_AUTHOR_DATE=2025-03-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2025-03-01T10:00:00Z",
	}, "commit", "-m", "add Service interface")

	// Dirty working tree modification must NOT pollute HEAD snapshot
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "dirty.go"), []byte("package main\ntype Dirty struct{}\n"), 0o644))

	proj, err := project.LoadProjectInfo(repoDir)
	require.NoError(t, err)

	cache := NewCache(cacheDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First run with progress callback: buckets mode, all time
	var events []ProgressEvent
	report1, err := AnalyzeWithProgress(ctx, proj, cache, Options{
		Mode:      ModeBuckets,
		Months:    0,
		MaxPoints: 12,
	}, func(ev ProgressEvent) {
		events = append(events, ev)
	})
	require.NoError(t, err)
	require.NotEmpty(t, events)
	require.Equal(t, "start", events[0].Type)
	require.Equal(t, 3, events[0].Total)
	require.Equal(t, 1, report1.SkippedCount)
	require.Len(t, report1.Points, 2)
	require.Equal(t, 2, report1.ComputedCount)
	require.Equal(t, 0, report1.CachedCount)

	// Verify metrics grew across commits and dirty.go was NOT included in HEAD
	p0 := report1.Points[0]
	p1 := report1.Points[1]
	require.Equal(t, "add go module", p0.Message)
	require.Equal(t, "add Service interface", p1.Message)
	require.Greater(t, p1.Metrics.LOC, p0.Metrics.LOC)
	require.Equal(t, 1, p1.Metrics.Structs) // dirty.go has another struct; must be excluded!
	require.Equal(t, 1, p1.Metrics.Interfaces)

	// Second run: hits cache completely
	report2, err := Analyze(ctx, proj, cache, Options{
		Mode:      ModeBuckets,
		Months:    0,
		MaxPoints: 12,
	})
	require.NoError(t, err)
	require.Len(t, report2.Points, 2)
	require.Equal(t, 2, report2.CachedCount)
	require.Equal(t, 0, report2.ComputedCount)
	require.True(t, report2.Points[0].Cached)

	// Tags mode includes v0.1.0 + HEAD and verifies separate trends_*.pb and complexity_*.pb caches.
	tagReport, err := Analyze(ctx, proj, cache, Options{
		Mode:      ModeTags,
		Months:    0,
		MaxPoints: 12,
	})
	require.NoError(t, err)
	require.Len(t, tagReport.Points, 2)
	require.Equal(t, "v0.1.0", tagReport.Points[0].Tag)
	require.NotNil(t, tagReport.Points[0].Metrics.CyclomaticComplexityAvg)
	require.NotNil(t, tagReport.Points[0].Metrics.CognitiveComplexityAvg)

	compFiles, err := filepath.Glob(filepath.Join(cacheDir, "complexity_*.pb"))
	require.NoError(t, err)
	require.Len(t, compFiles, 2)
	trendFiles, err := filepath.Glob(filepath.Join(cacheDir, "trends_*.pb"))
	require.NoError(t, err)
	require.Len(t, trendFiles, 2)

	// Subsequent run hits both trends and complexity caches.
	tagReportCached, err := Analyze(ctx, proj, cache, Options{
		Mode:      ModeTags,
		Months:    0,
		MaxPoints: 12,
	})
	require.NoError(t, err)
	require.Equal(t, 2, tagReportCached.CachedCount)
	require.Equal(t, 0, tagReportCached.ComputedCount)
}

func TestSampleByTagsSpansFullWindow(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, nil, "init")
	runGit(t, repoDir, nil, "config", "user.name", "Test User")
	runGit(t, repoDir, nil, "config", "user.email", "test@example.com")

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/tags\n\ngo 1.24\n"), 0o644))
	for i := 1; i <= 10; i++ {
		ts := time.Date(2025, time.Month(i), 1, 10, 0, 0, 0, time.UTC).Format(time.RFC3339)
		env := []string{"GIT_AUTHOR_DATE=" + ts, "GIT_COMMITTER_DATE=" + ts}
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644))
		runGit(t, repoDir, env, "add", ".")
		runGit(t, repoDir, env, "commit", "--allow-empty", "-m", "release")
		runGit(t, repoDir, nil, "tag", fmt.Sprintf("v0.0.%d", i))
	}

	commits, err := sampleCommits(repoDir, ModeTags, 0, 5)
	require.NoError(t, err)
	require.Len(t, commits, 5)
	require.Equal(t, "v0.0.1", commits[0].Tag)
}

func TestShallowCloneHandledGracefully(t *testing.T) {
	origin := t.TempDir()
	runGit(t, origin, nil, "init")
	runGit(t, origin, nil, "config", "user.name", "Test User")
	runGit(t, origin, nil, "config", "user.email", "test@example.com")

	require.NoError(t, os.WriteFile(filepath.Join(origin, "go.mod"), []byte("module example.com/shallow\n\ngo 1.24\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(origin, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644))
	runGit(t, origin, nil, "add", ".")
	runGit(t, origin, nil, "commit", "-m", "c1")

	require.NoError(t, os.WriteFile(filepath.Join(origin, "main.go"), []byte("package main\nfunc main() { println(1) }\n"), 0o644))
	runGit(t, origin, nil, "add", ".")
	runGit(t, origin, nil, "commit", "-m", "c2")

	shallow := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", "--depth=1", "file://"+origin, shallow)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "shallow clone failed: %s", string(out))

	proj, err := project.LoadProjectInfo(shallow)
	require.NoError(t, err)

	report, err := Analyze(context.Background(), proj, NewCache(t.TempDir()), Options{
		Mode:      ModeBuckets,
		Months:    12,
		MaxPoints: 12,
	})
	require.NoError(t, err)
	require.Len(t, report.Points, 1)
}

func TestAnalyzeWorksWithoutExternalGoModDownloads(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, nil, "init")
	runGit(t, dir, nil, "config", "user.name", "Test User")
	runGit(t, dir, nil, "config", "user.email", "test@example.com")

	// Intentionally reference a nonexistent external dependency without go.sum.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/offline\n\ngo 1.24\n\nrequire example.com/does-not-exist v1.2.3\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "svc.go"), []byte("package pkg\n\n// Svc does work.\ntype Svc interface{ Do() }\nfunc Run(x int) int { if x > 0 { return 1 }; return 0 }\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"example.com/offline/pkg\"\n\nfunc main() { _ = pkg.Run(1) }\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\n\nimport \"testing\"\n\nfunc TestMainHelper(t *testing.T) {}\n"), 0o644))
	runGit(t, dir, nil, "add", ".")
	runGit(t, dir, nil, "commit", "-m", "offline deps")

	proj, err := project.LoadProjectInfo(dir)
	require.NoError(t, err)

	report, err := Analyze(context.Background(), proj, NewCache(t.TempDir()), Options{
		Mode:      ModeBuckets,
		Months:    12,
		MaxPoints: 12,
	})
	require.NoError(t, err)
	require.Len(t, report.Points, 1)
	m := report.Points[0].Metrics
	require.Equal(t, 2, m.Packages)
	require.Equal(t, 2, m.SourceFiles) // excludes main_test.go
	require.Equal(t, 1, m.Interfaces)
	require.Equal(t, 2, m.Functions)
	require.Equal(t, 1, m.TestFiles)
	require.Equal(t, 1, m.TestFunctions)
	require.Greater(t, m.TestLOC, 0)
	require.Equal(t, 1, m.TestedPackages)
	require.Equal(t, 1, m.UntestedPackages)
	require.Equal(t, 50.0, m.TestedPackagesRatio)
	require.Equal(t, 1.0, m.AvgFunctionLOC)
	require.Equal(t, 0.5, m.AvgFunctionParams)
	require.Equal(t, 50.0, m.APIDocCoverage) // Svc is documented, Run is not
	require.Equal(t, 50.0, m.PropagationCost)
	require.Equal(t, 1, m.MaxDependencyDepth)
	require.NotNil(t, m.CyclomaticComplexityAvg)
	require.NotNil(t, m.CognitiveComplexityAvg)
}

func TestTreeBuildContextHandlesWindowsPaths(t *testing.T) {
	byPath := map[string][]byte{
		"main.go":     []byte("package main\nfunc main() {}\n"),
		"pkg/svc.go":  []byte("package pkg\nfunc Run() {}\n"),
		"pkg/skip.go": []byte("//go:build ignore\n\npackage pkg\n"),
	}

	buildCtx := newTreeBuildContext(byPath)

	ok, err := buildCtx.MatchFile(".", "main.go")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = buildCtx.MatchFile("pkg", "svc.go")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = buildCtx.MatchFile("pkg", "skip.go")
	require.NoError(t, err)
	require.False(t, ok)

	// Also verify OpenFile tolerates backslash-joined paths (as produced by Windows filepath.Join).
	buildCtx.JoinPath = func(elem ...string) string {
		return strings.Join(elem, "\\")
	}
	ok, err = buildCtx.MatchFile("pkg", "svc.go")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestLegacySnapshotCacheIsInvalidated(t *testing.T) {
	cacheDir := t.TempDir()
	trendCache := NewCache(cacheDir)
	projectKey := "example.com/legacy::.::"
	sha := "1111111111111111111111111111111111111111"

	require.NoError(t, trendCache.store.PutProto("trends", fmt.Sprintf("%s@%s", projectKey, sha), &cachepb.CommitSnapshotProto{
		CommitSha: sha,
		ShortSha:  sha[:7],
		Metrics: &cachepb.SnapshotMetricsProto{
			Loc:       100,
			Functions: 5,
			// SchemaVersion intentionally 0 (legacy entry missing avg_function_loc/params)
		},
	}))

	_, ok := trendCache.GetSnapshot(projectKey, sha)
	require.False(t, ok)
}

func TestAnalyzeSkipsGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, nil, "init")
	runGit(t, dir, nil, "config", "user.name", "Test User")
	runGit(t, dir, nil, "config", "user.email", "test@example.com")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/gentrends\n\ngo 1.24\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\n// Run runs.\nfunc Run() {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "api.pb.go"), []byte(`// Code generated by protoc-gen-go. DO NOT EDIT.

package main

type GeneratedStruct struct{}

func (g *GeneratedStruct) Huge(x int) int {
	if x > 0 {
		if x > 1 {
			return x
		}
	}
	return 0
}
`), 0o644))
	runGit(t, dir, nil, "add", ".")
	runGit(t, dir, nil, "commit", "-m", "with generated file")

	proj, err := project.LoadProjectInfo(dir)
	require.NoError(t, err)

	report, err := Analyze(context.Background(), proj, NewCache(t.TempDir()), Options{
		Mode:      ModeBuckets,
		Months:    12,
		MaxPoints: 12,
	})
	require.NoError(t, err)
	require.Len(t, report.Points, 1)
	m := report.Points[0].Metrics
	require.Equal(t, 1, m.SourceFiles)
	require.Equal(t, 0, m.Structs)
	require.Equal(t, 1, m.Functions)
	require.Equal(t, 0, m.Methods)
	require.Equal(t, 100.0, m.APIDocCoverage)
	require.NotNil(t, m.CyclomaticComplexityAvg)
	require.Equal(t, 1.0, *m.CyclomaticComplexityAvg)
}
