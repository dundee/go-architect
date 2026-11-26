package hotspots

import (
	"context"
	"math"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/dundee/go-architect/backend/git"
	"github.com/dundee/go-architect/internal/cache"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/go-architect/go-architect-lib/project"
)

type FileHotspot struct {
	File                 string  `json:"file"`
	Package              string  `json:"package"`
	LOC                  int     `json:"loc"`
	Commits              int     `json:"commits"`
	Churn                int     `json:"churn"`
	Contributors         int     `json:"contributors"`
	CyclomaticComplexity int     `json:"cyclomatic_complexity"`
	CognitiveComplexity  int     `json:"cognitive_complexity"`
	MaxCyclomatic        int     `json:"max_cyclomatic"`
	MaxCognitive         int     `json:"max_cognitive"`
	Score                float64 `json:"score"`
}

type Report struct {
	Hotspots   []FileHotspot `json:"hotspots"`
	TotalFiles int           `json:"total_files"`
}

func Analyze(
	ctx context.Context,
	proj *project.ProjectInfo,
	vcs *git.VCSAnalysisInfo,
	since time.Time,
) (*Report, error) {
	if proj == nil {
		return &Report{Hotspots: []FileHotspot{}}, nil
	}

	fileStats, err := complexity.CalculatePerFile(proj.Path, proj.Package)
	if err != nil {
		return nil, err
	}

	commitCounts, err := git.LoadFileCommitCounts(proj.Path, &since)
	if err != nil {
		return nil, err
	}

	subDir := filepath.ToSlash(cache.ProjectSubdir(ctx, proj.Path))
	return BuildReport(fileStats, vcs, commitCounts, subDir), nil
}

func BuildReport(
	fileStats map[string]complexity.FileStats,
	vcs *git.VCSAnalysisInfo,
	commitCounts map[string]int,
	subDir string,
) *Report {
	type churnInfo struct {
		churn        int
		contributors int
	}
	churnByPath := make(map[string]churnInfo)
	if vcs != nil {
		for _, mod := range vcs.ModificationsInfo.FileModifications {
			slash := filepath.ToSlash(mod.Source)
			churnByPath[slash] = churnInfo{
				churn:        mod.TotalModifications,
				contributors: len(mod.AuthorsSummary),
			}
		}
	}

	subPrefix := ""
	if subDir != "" && subDir != "." {
		subPrefix = filepath.ToSlash(subDir)
	}

	items := make([]FileHotspot, 0)
	var (
		maxCommits    int
		maxChurn      int
		maxCognitive  int
		maxCyclomatic int
		maxLOC        int
	)

	for _, fs := range fileStats {
		gitPath := fs.RelFile
		if subPrefix != "" {
			gitPath = path.Join(subPrefix, fs.RelFile)
		}

		ch := churnByPath[gitPath]
		commits := commitCounts[gitPath]
		if ch.churn == 0 && commits == 0 {
			continue
		}

		if commits > maxCommits {
			maxCommits = commits
		}
		if ch.churn > maxChurn {
			maxChurn = ch.churn
		}
		if fs.CognitiveTotal > maxCognitive {
			maxCognitive = fs.CognitiveTotal
		}
		if fs.CyclomaticTotal > maxCyclomatic {
			maxCyclomatic = fs.CyclomaticTotal
		}
		if fs.LOC > maxLOC {
			maxLOC = fs.LOC
		}

		items = append(items, FileHotspot{
			File:                 fs.RelFile,
			Package:              fs.Package,
			LOC:                  fs.LOC,
			Commits:              commits,
			Churn:                ch.churn,
			Contributors:         ch.contributors,
			CyclomaticComplexity: fs.CyclomaticTotal,
			CognitiveComplexity:  fs.CognitiveTotal,
			MaxCyclomatic:        fs.MaxCyclomatic,
			MaxCognitive:         fs.MaxCognitive,
		})
	}

	for i := range items {
		it := &items[i]
		var activityRatio float64
		switch {
		case maxCommits > 0 && maxChurn > 0:
			activityRatio = 0.5*(float64(it.Commits)/float64(maxCommits)) + 0.5*(float64(it.Churn)/float64(maxChurn))
		case maxCommits > 0:
			activityRatio = float64(it.Commits) / float64(maxCommits)
		case maxChurn > 0:
			activityRatio = float64(it.Churn) / float64(maxChurn)
		}

		var complexityRatio float64
		switch {
		case maxCognitive > 0 && maxLOC > 0:
			complexityRatio = 0.65*(float64(it.CognitiveComplexity)/float64(maxCognitive)) + 0.35*(float64(it.LOC)/float64(maxLOC))
		case maxCyclomatic > 0 && maxLOC > 0:
			complexityRatio = 0.65*(float64(it.CyclomaticComplexity)/float64(maxCyclomatic)) + 0.35*(float64(it.LOC)/float64(maxLOC))
		case maxLOC > 0:
			complexityRatio = float64(it.LOC) / float64(maxLOC)
		}

		it.Score = round2(100 * math.Sqrt(activityRatio*complexityRatio))
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		if items[i].Churn != items[j].Churn {
			return items[i].Churn > items[j].Churn
		}
		if items[i].CognitiveComplexity != items[j].CognitiveComplexity {
			return items[i].CognitiveComplexity > items[j].CognitiveComplexity
		}
		return items[i].File < items[j].File
	})

	return &Report{
		Hotspots:   items,
		TotalFiles: len(items),
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
