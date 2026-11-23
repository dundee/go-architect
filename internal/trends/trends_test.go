package trends

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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

	// Tags mode includes v0.1.0 + HEAD (also verify opt-in complexity upgrades cached snapshots)
	tagReport, err := Analyze(ctx, proj, cache, Options{
		Mode:              ModeTags,
		Months:            0,
		MaxPoints:         12,
		IncludeComplexity: true,
	})
	require.NoError(t, err)
	require.Len(t, tagReport.Points, 2)
	require.Equal(t, "v0.1.0", tagReport.Points[0].Tag)
	require.NotNil(t, tagReport.Points[0].Metrics.CyclomaticComplexityAvg)
	require.NotNil(t, tagReport.Points[0].Metrics.CognitiveComplexityAvg)
}

func TestSampleByTagsSpansFullWindow(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	commits, err := sampleCommits(repoRoot, ModeTags, 0, 5)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(commits), 5)
	require.Equal(t, "v0.0.1", commits[0].Tag)
}
