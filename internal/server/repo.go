package server

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"

	"github.com/go-architect/go-architect-lib/repository"
)

func loadRepositoryInfo(path string) (*repository.RepositoryInfo, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return repository.LoadRepositoryInfo(path)
	}

	out, err := exec.Command("git", "-C", path, "status", "--porcelain=v2", "--branch").Output()
	if err != nil {
		return nil, err
	}

	var (
		branch   string
		revision string
		isClean  = true
	)

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# branch.oid ") {
			rev := strings.TrimPrefix(line, "# branch.oid ")
			if rev != "(initial)" {
				revision = rev
			}
			continue
		}
		if strings.HasPrefix(line, "# branch.head ") {
			branch = strings.TrimPrefix(line, "# branch.head ")
			continue
		}
		if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
			isClean = false
		}
	}

	remoteURL := "unknown"
	if remoteOut, rErr := exec.Command("git", "-C", path, "remote", "get-url", "origin").Output(); rErr == nil {
		if trimmed := strings.TrimSpace(string(remoteOut)); trimmed != "" {
			remoteURL = trimmed
		}
	}

	return &repository.RepositoryInfo{
		Path:            path,
		Url:             remoteURL,
		CurrentBranch:   branch,
		CurrentRevision: revision,
		IsUpToDate:      isClean,
	}, nil
}
