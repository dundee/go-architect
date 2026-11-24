package trends

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dundee/go-architect/internal/cache"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/go-architect/go-architect-lib/project"
	"github.com/go-git/go-git/v5"
)

type Options struct {
	Mode              SamplingMode
	Months            int
	MaxPoints         int
	IncludeComplexity bool
}

func Analyze(ctx context.Context, proj *project.ProjectInfo, trendCache *Cache, opts Options) (*TrendsReport, error) {
	return AnalyzeWithProgress(ctx, proj, trendCache, opts, nil)
}

func AnalyzeWithProgress(
	ctx context.Context,
	proj *project.ProjectInfo,
	trendCache *Cache,
	opts Options,
	onProgress func(ProgressEvent),
) (*TrendsReport, error) {
	if proj == nil {
		return nil, fmt.Errorf("project is required")
	}
	if opts.Mode == "" {
		opts.Mode = ModeBuckets
	}
	if opts.MaxPoints <= 0 {
		opts.MaxPoints = 12
	}

	emit := func(ev ProgressEvent) {
		if onProgress != nil {
			onProgress(ev)
		}
	}

	start := time.Now()
	commits, err := sampleCommits(proj.Path, opts.Mode, opts.Months, opts.MaxPoints)
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpenWithOptions(proj.Path, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open git repository: %w", err)
	}

	relPath := cache.ProjectSubdir(ctx, proj.Path)
	relPrefix := filepath.ToSlash(relPath)
	if relPrefix == "." {
		relPrefix = ""
	}

	total := len(commits)
	emit(ProgressEvent{
		Type:  "start",
		Total: total,
	})

	baseProjectKey := cache.ProjectKey(proj.Package, relPath, nil)
	complexityKey := cache.ProjectKey(proj.Package, relPath, nil)

	report := &TrendsReport{
		Mode:              opts.Mode,
		Months:            opts.Months,
		IncludeComplexity: opts.IncludeComplexity,
		Points:            make([]CommitSnapshot, 0, total),
	}

	for idx, c := range commits {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		step := idx + 1
		cachedSnap, hasBase := trendCache.GetSnapshot(baseProjectKey, c.SHA)
		if hasBase {
			if c.Tag != "" && cachedSnap.Tag == "" {
				cachedSnap.Tag = c.Tag
			}

			if !opts.IncludeComplexity {
				cachedSnap.Cached = true
				report.CachedCount++
				report.Points = append(report.Points, *cachedSnap)
				emit(ProgressEvent{
					Type:      "progress",
					Index:     step,
					Total:     total,
					CommitSHA: c.SHA,
					ShortSHA:  c.Short,
					Message:   c.Message,
					Tag:       c.Tag,
					Status:    "cached",
					Snapshot:  cachedSnap,
				})
				continue
			}

			if compReport, hasComp := trendCache.GetComplexity(complexityKey, c.SHA); hasComp {
				applyComplexity(&cachedSnap.Metrics, compReport)
				cachedSnap.Cached = true
				report.CachedCount++
				report.Points = append(report.Points, *cachedSnap)
				emit(ProgressEvent{
					Type:      "progress",
					Index:     step,
					Total:     total,
					CommitSHA: c.SHA,
					ShortSHA:  c.Short,
					Message:   c.Message,
					Tag:       c.Tag,
					Status:    "cached",
					Snapshot:  cachedSnap,
				})
				continue
			}

			emit(ProgressEvent{
				Type:      "progress",
				Index:     step,
				Total:     total,
				CommitSHA: c.SHA,
				ShortSHA:  c.Short,
				Message:   c.Message,
				Tag:       c.Tag,
				Status:    "computing",
			})
			compReport, compErr := measureComplexityFromTree(repo, relPrefix, c.SHA)
			if compErr == nil && compReport != nil {
				_ = trendCache.PutComplexity(complexityKey, c.SHA, compReport)
				applyComplexity(&cachedSnap.Metrics, compReport)
				cachedSnap.Cached = false
				report.ComputedCount++
				report.Points = append(report.Points, *cachedSnap)
				emit(ProgressEvent{
					Type:      "progress",
					Index:     step,
					Total:     total,
					CommitSHA: c.SHA,
					ShortSHA:  c.Short,
					Message:   c.Message,
					Tag:       c.Tag,
					Status:    "done",
					Snapshot:  cachedSnap,
				})
				continue
			}

			cachedSnap.Cached = true
			report.CachedCount++
			report.Points = append(report.Points, *cachedSnap)
			emit(ProgressEvent{
				Type:      "progress",
				Index:     step,
				Total:     total,
				CommitSHA: c.SHA,
				ShortSHA:  c.Short,
				Message:   c.Message,
				Tag:       c.Tag,
				Status:    "cached",
				Snapshot:  cachedSnap,
			})
			continue
		}

		emit(ProgressEvent{
			Type:      "progress",
			Index:     step,
			Total:     total,
			CommitSHA: c.SHA,
			ShortSHA:  c.Short,
			Message:   c.Message,
			Tag:       c.Tag,
			Status:    "computing",
		})

		snap, compReport, err := measureCommitFromTree(repo, relPrefix, c, opts.IncludeComplexity)
		if err != nil {
			report.SkippedCount++
			reason := fmt.Sprintf("%s: %v", c.Short, err)
			if len(report.SkippedReasons) < 5 {
				report.SkippedReasons = append(report.SkippedReasons, reason)
			}
			emit(ProgressEvent{
				Type:      "progress",
				Index:     step,
				Total:     total,
				CommitSHA: c.SHA,
				ShortSHA:  c.Short,
				Message:   c.Message,
				Tag:       c.Tag,
				Status:    "skipped",
				Reason:    err.Error(),
			})
			continue
		}

		_ = trendCache.PutSnapshot(baseProjectKey, *snap)
		if compReport != nil {
			_ = trendCache.PutComplexity(complexityKey, c.SHA, compReport)
			applyComplexity(&snap.Metrics, compReport)
		}

		report.ComputedCount++
		report.Points = append(report.Points, *snap)
		emit(ProgressEvent{
			Type:      "progress",
			Index:     step,
			Total:     total,
			CommitSHA: c.SHA,
			ShortSHA:  c.Short,
			Message:   c.Message,
			Tag:       c.Tag,
			Status:    "done",
			Snapshot:  snap,
		})
	}

	report.DurationMs = time.Since(start).Milliseconds()
	return report, nil
}

func applyComplexity(dst *SnapshotMetrics, report *complexity.Report) {
	if report == nil {
		return
	}
	cAvg := report.Cyclomatic.AverageComplexity
	gAvg := report.Cognitive.AverageComplexity
	dst.CyclomaticComplexityAvg = &cAvg
	dst.CognitiveComplexityAvg = &gAvg
}
