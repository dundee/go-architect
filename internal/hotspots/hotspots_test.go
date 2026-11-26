package hotspots

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dundee/go-architect/backend/git"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/go-architect/go-architect-lib/project"
	"github.com/stretchr/testify/require"
)

func TestBuildReportRanksChurnAndComplexity(t *testing.T) {
	fileStats := map[string]complexity.FileStats{
		"simple_churn.go": {
			Package:         "example.com/app",
			RelFile:         "simple_churn.go",
			LOC:             20,
			CyclomaticTotal: 1,
			CognitiveTotal:  0,
		},
		"hotspot.go": {
			Package:         "example.com/app/core",
			RelFile:         "core/hotspot.go",
			LOC:             200,
			CyclomaticTotal: 25,
			CognitiveTotal:  40,
			MaxCyclomatic:   15,
			MaxCognitive:    22,
		},
		"untouched.go": {
			Package:         "example.com/app",
			RelFile:         "untouched.go",
			LOC:             500,
			CyclomaticTotal: 50,
			CognitiveTotal:  90,
		},
	}

	vcs := &git.VCSAnalysisInfo{
		ModificationsInfo: git.Modifications{
			FileModifications: []git.ModificationsInfo{
				{
					Source:             "simple_churn.go",
					TotalModifications: 50,
					AuthorsSummary:     []git.ContributorSummary{{Name: "Alice", Modifications: 50}},
				},
				{
					Source:             filepath.FromSlash("core/hotspot.go"),
					TotalModifications: 120,
					AuthorsSummary: []git.ContributorSummary{
						{Name: "Alice", Modifications: 80},
						{Name: "Bob", Modifications: 40},
					},
				},
				{
					Source:             "deleted.go",
					TotalModifications: 300,
					AuthorsSummary:     []git.ContributorSummary{{Name: "Alice", Modifications: 300}},
				},
			},
		},
	}

	commitCounts := map[string]int{
		"simple_churn.go": 2,
		"core/hotspot.go": 8,
		"deleted.go":      10,
	}

	report := BuildReport(fileStats, vcs, commitCounts, ".")
	require.Equal(t, 2, report.TotalFiles)
	require.Len(t, report.Hotspots, 2)

	top := report.Hotspots[0]
	require.Equal(t, "core/hotspot.go", top.File)
	require.Equal(t, 100.0, top.Score)
	require.Equal(t, 8, top.Commits)
	require.Equal(t, 120, top.Churn)
	require.Equal(t, 2, top.Contributors)
	require.Equal(t, 40, top.CognitiveComplexity)
	require.Equal(t, 25, top.CyclomaticComplexity)

	second := report.Hotspots[1]
	require.Equal(t, "simple_churn.go", second.File)
	require.Less(t, second.Score, top.Score)
}

func TestAnalyzeIntegrationWithSubdir(t *testing.T) {
	repoDir := t.TempDir()
	runGit := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		if env != nil {
			cmd.Env = append(os.Environ(), env...)
		}
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))
	}

	runGit(nil, "init")
	runGit(nil, "config", "user.name", "Alice")
	runGit(nil, "config", "user.email", "alice@example.com")

	modDir := filepath.Join(repoDir, "services", "api")
	require.NoError(t, os.MkdirAll(modDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(modDir, "go.mod"), []byte("module example.com/api\n\ngo 1.24\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modDir, "handler.go"), []byte(`package main

func Handle(x int) int {
	if x > 10 {
		for i := 0; i < x; i++ {
			if i%2 == 0 {
				return i
			}
		}
	}
	return 0
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modDir, "handler_test.go"), []byte("package main\nfunc TestH() {}\n"), 0o644))

	ts := time.Now().AddDate(0, 0, -2).Format(time.RFC3339)
	env := []string{"GIT_AUTHOR_DATE=" + ts, "GIT_COMMITTER_DATE=" + ts}
	runGit(env, "add", ".")
	runGit(env, "commit", "-m", "add handler")

	since := time.Now().AddDate(0, -1, 0)
	vcs, err := git.LoadRepositoryTeamCohesion(modDir, &since)
	require.NoError(t, err)

	proj, err := project.LoadProjectInfo(modDir)
	require.NoError(t, err)

	report, err := Analyze(context.Background(), proj, vcs, since)
	require.NoError(t, err)
	require.Equal(t, 1, report.TotalFiles)
	require.Len(t, report.Hotspots, 1)
	require.Equal(t, "handler.go", report.Hotspots[0].File)
	require.Equal(t, 1, report.Hotspots[0].Commits)
	require.Greater(t, report.Hotspots[0].CognitiveComplexity, 0)
}
