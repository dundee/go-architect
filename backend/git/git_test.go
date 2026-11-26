package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Prepare_Result(t *testing.T) {
	type test struct {
		title           string
		inputSince      *time.Time
		inputLogDetails *LogDetails
		want            *VCSAnalysisInfo
	}

	now := time.Now()

	tests := []test{
		{
			title:           "Nil Since Param",
			inputSince:      nil,
			inputLogDetails: nil,
			want:            nil,
		},
		{
			title:           "Nil LogDetails Param",
			inputSince:      &now,
			inputLogDetails: nil,
			want:            nil,
		},
		{
			title:      "Successful case",
			inputSince: &now,
			inputLogDetails: &LogDetails{
				AuthorInfo: map[string]AuthorInfo{
					"foo": {
						Name:          "foo",
						Commits:       5,
						ModifiedLines: 2,
					},
					"bar": {
						Name:          "bar",
						Commits:       15,
						ModifiedLines: 7,
					},
				},
				PathModificationsInfo: map[string]ModificationsInfo{
					"pm1": {
						Source: "foobar",
						AuthorsSummary: []ContributorSummary{
							{
								Name:          "foobar",
								Modifications: 13,
							},
							{
								Name:          "barfoo",
								Modifications: 10,
							},
						},
					},
				},
				FileModificationsInfo: map[string]ModificationsInfo{
					"fm1": {
						Source: "foobar.go",
						AuthorsSummary: []ContributorSummary{
							{
								Name:          "foobar",
								Modifications: 13,
							},
							{
								Name:          "barfoo",
								Modifications: 10,
							},
						},
					},
					"fm2": {
						Source: "barfoo.go",
						AuthorsSummary: []ContributorSummary{
							{
								Name:          "barfoo",
								Modifications: 15,
							},
						},
					},
				},
			},
			want: &VCSAnalysisInfo{
				Since: &now,
				AuthorsInfo: AuthorsInfo{
					TotalAuthors:       2,
					TotalCommits:       20,
					TotalModifications: 9,
					AuthorDetails: []AuthorInfo{
						{
							Name:          "foo",
							Commits:       5,
							ModifiedLines: 2,
						},
						{
							Name:          "bar",
							Commits:       15,
							ModifiedLines: 7,
						},
					},
				},
				ModificationsInfo: Modifications{
					PathModifications: []ModificationsInfo{
						{
							Source:             "foobar",
							TotalModifications: 23,
							AuthorsSummary: []ContributorSummary{
								{
									Name:          "barfoo",
									Modifications: 10,
								},
								{
									Name:          "foobar",
									Modifications: 13,
								},
							},
						},
					},
					FileModifications: []ModificationsInfo{
						{
							Source:             "barfoo.go",
							TotalModifications: 15,
							AuthorsSummary: []ContributorSummary{
								{
									Name:          "barfoo",
									Modifications: 15,
								},
							},
						},
						{
							Source:             "foobar.go",
							TotalModifications: 23,
							AuthorsSummary: []ContributorSummary{
								{
									Name:          "barfoo",
									Modifications: 10,
								},
								{
									Name:          "foobar",
									Modifications: 13,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		got := prepareResult(tc.inputSince, tc.inputLogDetails)
		if !equalsVcsAnalysisInfo(t, tc.want, got) {
			t.Fatalf("For case: '%s' - expected: %v, got: %v", tc.title, tc.want, got)
		}
	}
}

func equalsVcsAnalysisInfo(t *testing.T, a *VCSAnalysisInfo, b *VCSAnalysisInfo) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil && b != nil {
		return false
	}
	if a != nil && b == nil {
		return false
	}
	if a.Since != b.Since {
		fmt.Printf("False1\n")
		return false
	}

	if !authorsEquals(t, a.AuthorsInfo, b.AuthorsInfo) {
		fmt.Printf("False2 [%+v][%+v]\n", a.AuthorsInfo, b.AuthorsInfo)
		return false
	}
	if !assert.ElementsMatch(t, a.ModificationsInfo.PathModifications, b.ModificationsInfo.PathModifications) {
		fmt.Printf("False3 [%+v][%+v]\n", a.ModificationsInfo.PathModifications, b.ModificationsInfo.PathModifications)
		return false
	}
	if !assert.ElementsMatch(t, a.ModificationsInfo.FileModifications, b.ModificationsInfo.FileModifications) {
		fmt.Printf("False4 [%+v][%+v]\n", a.ModificationsInfo.FileModifications, b.ModificationsInfo.FileModifications)
		return false
	}
	return true
}

func Test_LoadRepositoryTeamCohesion_NativeGit(t *testing.T) {
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if env != nil {
			cmd.Env = append(os.Environ(), env...)
		}
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}

	run(nil, "init")
	run(nil, "config", "user.name", "Alice")
	run(nil, "config", "user.email", "alice@example.com")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "a.go"), []byte("package pkg\nfunc A() {}\n"), 0o644))
	t1 := time.Now().AddDate(0, -1, 0).Format(time.RFC3339)
	run([]string{"GIT_AUTHOR_DATE=" + t1, "GIT_COMMITTER_DATE=" + t1}, "add", ".")
	run([]string{"GIT_AUTHOR_DATE=" + t1, "GIT_COMMITTER_DATE=" + t1}, "commit", "-m", "init")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "a.go"), []byte("package pkg\nfunc A() {}\nfunc B() {}\n"), 0o644))
	t2 := time.Now().AddDate(0, 0, -5).Format(time.RFC3339)
	run([]string{
		"GIT_AUTHOR_NAME=Bob",
		"GIT_AUTHOR_EMAIL=bob@example.com",
		"GIT_AUTHOR_DATE=" + t2,
		"GIT_COMMITTER_DATE=" + t2,
	}, "add", ".")
	run([]string{
		"GIT_AUTHOR_NAME=Bob",
		"GIT_AUTHOR_EMAIL=bob@example.com",
		"GIT_AUTHOR_DATE=" + t2,
		"GIT_COMMITTER_DATE=" + t2,
	}, "commit", "-m", "add B")

	since := time.Now().AddDate(0, -6, 0)
	info, err := LoadRepositoryTeamCohesion(dir, &since)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, 2, info.AuthorsInfo.TotalAuthors)
	require.Equal(t, 2, info.AuthorsInfo.TotalCommits)
	require.Equal(t, 3, info.AuthorsInfo.TotalModifications)
	require.Len(t, info.ModificationsInfo.FileModifications, 1)
	require.Equal(t, 3, info.ModificationsInfo.FileModifications[0].TotalModifications)
	require.Len(t, info.ModificationsInfo.FileModifications[0].AuthorsSummary, 2)

	commitCounts, err := LoadFileCommitCounts(dir, &since)
	require.NoError(t, err)
	require.Equal(t, map[string]int{"pkg/a.go": 2}, commitCounts)

	goGitCounts, err := loadFileCommitCountsGoGit(dir, &since)
	require.NoError(t, err)
	require.Equal(t, map[string]int{"pkg/a.go": 2}, goGitCounts)
}
