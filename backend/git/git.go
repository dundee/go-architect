package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func LoadRepositoryTeamCohesion(path string, since *time.Time) (*VCSAnalysisInfo, error) {
	if since == nil {
		return nil, nil
	}

	if _, lookErr := exec.LookPath("git"); lookErr == nil {
		logDetails, err := loadViaNativeGit(path, since)
		if err != nil {
			return nil, err
		}
		return prepareResult(since, logDetails), nil
	}

	log, err := getGitLog(path, since)
	if err != nil {
		return nil, err
	}

	return prepareResult(since, analyzeLog(log)), nil
}

func loadViaNativeGit(repoPath string, since *time.Time) (*LogDetails, error) {
	checkCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--is-inside-work-tree")
	if out, err := checkCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("open git repository: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	args := []string{
		"-C", repoPath,
		"log",
		"--no-renames",
		"--no-merges",
		"--numstat",
		"--format=@@@%aN",
	}
	if since != nil && since.Year() > 1970 {
		args = append(args, "--since="+since.UTC().Format(time.RFC3339))
	}

	cmd := exec.Command("git", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	logDetails := &LogDetails{
		AuthorInfo:            make(map[string]AuthorInfo),
		PathModificationsInfo: make(map[string]ModificationsInfo),
		FileModificationsInfo: make(map[string]ModificationsInfo),
	}

	var (
		hasCommit     bool
		currentAuthor string
		currentStats  object.FileStats
	)

	flushCommit := func() {
		if !hasCommit {
			return
		}
		resolveAuthorsInfo(currentStats, currentAuthor, logDetails.AuthorInfo)
		for _, fs := range currentStats {
			extractPathModifications(currentAuthor, fs, logDetails.PathModificationsInfo)
			extractFileModifications(currentAuthor, fs, logDetails.FileModificationsInfo)
		}
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "@@@") {
			flushCommit()
			currentAuthor = strings.TrimPrefix(line, "@@@")
			currentStats = currentStats[:0]
			hasCommit = true
			continue
		}
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		add := parseNumstatCount(parts[0])
		del := parseNumstatCount(parts[1])
		name := filepath.FromSlash(parts[2])
		currentStats = append(currentStats, object.FileStat{
			Name:     name,
			Addition: add,
			Deletion: del,
		})
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "does not have any commits yet") {
			return logDetails, nil
		}
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && errMsg != "" {
			return nil, fmt.Errorf("git log failed: %s", strings.TrimSpace(errMsg))
		}
		return nil, waitErr
	}

	flushCommit()
	return logDetails, nil
}

func parseNumstatCount(raw string) int {
	if raw == "-" || raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func prepareResult(since *time.Time, logDetails *LogDetails) *VCSAnalysisInfo {
	if since == nil {
		return nil
	}
	if logDetails == nil {
		return nil
	}
	return &VCSAnalysisInfo{
		Since:       since,
		AuthorsInfo: analyzeAuthors(logDetails.AuthorInfo),
		ModificationsInfo: Modifications{
			PathModifications: analyzeModifications(logDetails.PathModificationsInfo),
			FileModifications: analyzeModifications(logDetails.FileModificationsInfo),
		},
	}
}

func getGitLog(path string, since *time.Time) (object.CommitIter, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	log, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		Since: since,
	})

	return log, err
}

// LoadFileCommitCounts returns the number of non-merge commits touching each
// repository-relative file path (normalized with forward slashes) since the given time.
func LoadFileCommitCounts(repoPath string, since *time.Time) (map[string]int, error) {
	if _, lookErr := exec.LookPath("git"); lookErr == nil {
		return loadFileCommitCountsNative(repoPath, since)
	}
	return loadFileCommitCountsGoGit(repoPath, since)
}

func loadFileCommitCountsNative(repoPath string, since *time.Time) (map[string]int, error) {
	checkCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--is-inside-work-tree")
	if out, err := checkCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("open git repository: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	args := []string{
		"-C", repoPath,
		"log",
		"--no-renames",
		"--no-merges",
		"--name-only",
		"--format=@@@%H",
	}
	if since != nil && since.Year() > 1970 {
		args = append(args, "--since="+since.UTC().Format(time.RFC3339))
	}

	cmd := exec.Command("git", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	seenInCommit := make(map[string]struct{})
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "@@@") {
			clear(seenInCommit)
			continue
		}
		slashPath := filepath.ToSlash(line)
		if _, seen := seenInCommit[slashPath]; !seen {
			seenInCommit[slashPath] = struct{}{}
			counts[slashPath]++
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if scanErr != nil {
		return nil, scanErr
	}
	if waitErr != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "does not have any commits yet") {
			return counts, nil
		}
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && errMsg != "" {
			return nil, fmt.Errorf("git log failed: %s", strings.TrimSpace(errMsg))
		}
		return nil, waitErr
	}

	return counts, nil
}

func loadFileCommitCountsGoGit(repoPath string, since *time.Time) (map[string]int, error) {
	iter, err := getGitLog(repoPath, since)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	counts := make(map[string]int)
	err = iter.ForEach(func(c *object.Commit) error {
		if c == nil {
			return nil
		}
		stats, statsErr := c.Stats()
		if statsErr != nil {
			return nil
		}
		seenInCommit := make(map[string]struct{}, len(stats))
		for _, fs := range stats {
			slashPath := filepath.ToSlash(fs.Name)
			if _, seen := seenInCommit[slashPath]; !seen {
				seenInCommit[slashPath] = struct{}{}
				counts[slashPath]++
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, plumbing.ErrObjectNotFound) {
		return nil, err
	}
	return counts, nil
}
