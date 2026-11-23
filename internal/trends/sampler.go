package trends

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type sampledCommit struct {
	SHA     string
	Short   string
	Date    time.Time
	Message string
	Tag     string
	IsHead  bool
}

func sampleCommits(repoPath string, mode SamplingMode, months, maxPoints int) ([]sampledCommit, error) {
	if maxPoints < 2 {
		maxPoints = 12
	}

	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open git repository: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD: %w", err)
	}
	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("load HEAD commit: %w", err)
	}

	var since *time.Time
	if months > 0 {
		t := time.Now().AddDate(0, -months, 0)
		since = &t
	}

	if mode == ModeTags {
		return sampleByTags(repo, headCommit, since, maxPoints)
	}
	return sampleByBuckets(repo, headCommit, since, maxPoints)
}

func sampleByBuckets(repo *git.Repository, headCommit *object.Commit, since *time.Time, maxPoints int) ([]sampledCommit, error) {
	iter, err := repo.Log(&git.LogOptions{
		From:  headCommit.Hash,
		Order: git.LogOrderCommitterTime,
		Since: since,
	})
	if err != nil {
		return nil, fmt.Errorf("read git log: %w", err)
	}
	defer iter.Close()

	var all []*object.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		all = append(all, c)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate commits: %w", err)
	}

	if len(all) == 0 {
		all = []*object.Commit{headCommit}
	}

	// Sort oldest -> newest
	sort.Slice(all, func(i, j int) bool {
		return all[i].Committer.When.Before(all[j].Committer.When)
	})

	if len(all) <= maxPoints {
		return toSampledSlice(all, headCommit.Hash.String()), nil
	}

	oldest := all[0].Committer.When
	newest := all[len(all)-1].Committer.When
	totalSpan := newest.Sub(oldest)

	var selected []*object.Commit
	seen := make(map[string]struct{})

	if totalSpan <= 0 {
		// All commits share identical timestamp: sample evenly by index
		step := float64(len(all)-1) / float64(maxPoints-1)
		for i := 0; i < maxPoints; i++ {
			idx := int(float64(i)*step + 0.5)
			if idx >= len(all) {
				idx = len(all) - 1
			}
			c := all[idx]
			sha := c.Hash.String()
			if _, ok := seen[sha]; !ok {
				seen[sha] = struct{}{}
				selected = append(selected, c)
			}
		}
	} else {
		// Divide time span into (maxPoints - 1) buckets and keep latest commit per bucket, plus HEAD.
		bucketCount := maxPoints - 1
		bucketLatest := make([]*object.Commit, bucketCount)
		for _, c := range all {
			if c.Hash == headCommit.Hash {
				continue
			}
			offset := c.Committer.When.Sub(oldest)
			bucket := int(float64(offset) / float64(totalSpan) * float64(bucketCount))
			if bucket >= bucketCount {
				bucket = bucketCount - 1
			}
			if bucket < 0 {
				bucket = 0
			}
			bucketLatest[bucket] = c
		}

		for _, c := range bucketLatest {
			if c == nil {
				continue
			}
			sha := c.Hash.String()
			if _, ok := seen[sha]; !ok {
				seen[sha] = struct{}{}
				selected = append(selected, c)
			}
		}
		if _, ok := seen[headCommit.Hash.String()]; !ok {
			selected = append(selected, headCommit)
		}
	}

	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Committer.When.Before(selected[j].Committer.When)
	})
	return toSampledSlice(selected, headCommit.Hash.String()), nil
}

func sampleByTags(repo *git.Repository, headCommit *object.Commit, since *time.Time, maxPoints int) ([]sampledCommit, error) {
	tagRefs, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("list git tags: %w", err)
	}
	defer tagRefs.Close()

	type tagEntry struct {
		tag    string
		commit *object.Commit
	}

	bySHA := make(map[string]tagEntry)
	err = tagRefs.ForEach(func(ref *plumbing.Reference) error {
		tagName := ref.Name().Short()
		var c *object.Commit
		if tagObj, tagErr := repo.TagObject(ref.Hash()); tagErr == nil {
			c, _ = tagObj.Commit()
		}
		if c == nil {
			c, _ = repo.CommitObject(ref.Hash())
		}
		if c == nil {
			return nil
		}
		if since != nil && c.Committer.When.Before(*since) {
			return nil
		}
		sha := c.Hash.String()
		if existing, ok := bySHA[sha]; ok {
			// Keep lexicographically latest tag if multiple tags point to same commit
			if tagName > existing.tag {
				bySHA[sha] = tagEntry{tag: tagName, commit: c}
			}
		} else {
			bySHA[sha] = tagEntry{tag: tagName, commit: c}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate git tags: %w", err)
	}

	entries := make([]tagEntry, 0, len(bySHA))
	for _, e := range bySHA {
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].commit.Committer.When.Before(entries[j].commit.Committer.When)
	})

	if len(entries) > maxPoints {
		// Sample evenly across all tags in the window (always keeping oldest and newest)
		sampled := make([]tagEntry, 0, maxPoints)
		seenIdx := make(map[int]struct{}, maxPoints)
		step := float64(len(entries)-1) / float64(maxPoints-1)
		for i := 0; i < maxPoints; i++ {
			idx := int(float64(i)*step + 0.5)
			if idx >= len(entries) {
				idx = len(entries) - 1
			}
			if _, ok := seenIdx[idx]; !ok {
				seenIdx[idx] = struct{}{}
				sampled = append(sampled, entries[idx])
			}
		}
		entries = sampled
	}

	headSHA := headCommit.Hash.String()
	var result []sampledCommit
	hasHead := false
	for _, e := range entries {
		sha := e.commit.Hash.String()
		if sha == headSHA {
			hasHead = true
		}
		result = append(result, sampledCommit{
			SHA:     sha,
			Short:   sha[:7],
			Date:    e.commit.Committer.When,
			Message: firstLine(e.commit.Message),
			Tag:     e.tag,
			IsHead:  sha == headSHA,
		})
	}

	if !hasHead {
		result = append(result, sampledCommit{
			SHA:     headSHA,
			Short:   headSHA[:7],
			Date:    headCommit.Committer.When,
			Message: firstLine(headCommit.Message),
			IsHead:  true,
		})
	}

	return result, nil
}

func toSampledSlice(commits []*object.Commit, headSHA string) []sampledCommit {
	out := make([]sampledCommit, 0, len(commits))
	for _, c := range commits {
		sha := c.Hash.String()
		out = append(out, sampledCommit{
			SHA:     sha,
			Short:   sha[:7],
			Date:    c.Committer.When,
			Message: firstLine(c.Message),
			IsHead:  sha == headSHA,
		})
	}
	return out
}

func firstLine(msg string) string {
	trimmed := strings.TrimSpace(msg)
	if idx := strings.IndexByte(trimmed, '\n'); idx >= 0 {
		return strings.TrimSpace(trimmed[:idx])
	}
	return trimmed
}
