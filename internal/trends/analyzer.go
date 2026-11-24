package trends

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dundee/go-architect/internal/cache"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/go-architect/go-architect-lib/instability"
	"github.com/go-architect/go-architect-lib/metrics/comments"
	"github.com/go-architect/go-architect-lib/metrics/interfaces"
	"github.com/go-architect/go-architect-lib/metrics/loc"
	"github.com/go-architect/go-architect-lib/metrics/types"
	"github.com/go-architect/go-architect-lib/project"
	"golang.org/x/sync/errgroup"
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

	gitRoot, relPath, err := resolveGitSubdir(ctx, proj.Path)
	if err != nil {
		return nil, err
	}

	total := len(commits)
	emit(ProgressEvent{
		Type:  "start",
		Total: total,
	})

	// Base trend snapshot key (independent of orgPackages except Instability counts, which are internal-only).
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

			// Base metrics are cached, only complexity needs computing in a worktree.
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
			compReport, compErr := measureComplexityWithWorktree(ctx, gitRoot, relPath, proj.Package, c.SHA)
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
			// Fallback to returning cached base snapshot if complexity failed.
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

		snap, compReport, err := measureWithWorktree(ctx, gitRoot, relPath, proj.OrganizationPackages, c, opts.IncludeComplexity)
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

		// Store base trend snapshot and complexity in separate protobuf cache files.
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

func resolveGitSubdir(ctx context.Context, projectPath string) (string, string, error) {
	absProj, err := filepath.Abs(projectPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve project path: %w", err)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", absProj, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve git top-level directory: %w", err)
	}
	gitRoot := strings.TrimSpace(string(out))
	// Resolve symlinks on both to ensure filepath.Rel matches on macOS (/var vs /private/var).
	if evalRoot, e := filepath.EvalSymlinks(gitRoot); e == nil {
		gitRoot = evalRoot
	}
	if evalProj, e := filepath.EvalSymlinks(absProj); e == nil {
		absProj = evalProj
	}
	rel, err := filepath.Rel(gitRoot, absProj)
	if err != nil {
		return "", "", fmt.Errorf("resolve relative project path: %w", err)
	}
	return gitRoot, rel, nil
}

func withTempWorktree(ctx context.Context, gitRoot, sha string, fn func(wtPath string) error) error {
	tmpRoot, err := os.MkdirTemp("", "go-architect-wt-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpRoot)
	}()

	wtPath := filepath.Join(tmpRoot, "wt")
	cmd := exec.CommandContext(ctx, "git", "-C", gitRoot, "worktree", "add", "--detach", "--quiet", wtPath, sha)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	defer func() {
		rmCmd := exec.Command("git", "-C", gitRoot, "worktree", "remove", "--force", wtPath)
		_ = rmCmd.Run()
	}()

	return fn(wtPath)
}

func measureComplexityWithWorktree(ctx context.Context, gitRoot, relPath, mainPkg, sha string) (*complexity.Report, error) {
	var report *complexity.Report
	err := withTempWorktree(ctx, gitRoot, sha, func(wtPath string) error {
		targetPath := filepath.Join(wtPath, relPath)
		var calcErr error
		report, calcErr = complexity.Calculate(targetPath, mainPkg, 5)
		return calcErr
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}

func measureWithWorktree(
	ctx context.Context,
	gitRoot string,
	relPath string,
	orgPackages []string,
	c sampledCommit,
	includeComplexity bool,
) (*CommitSnapshot, *complexity.Report, error) {
	var (
		metrics    SnapshotMetrics
		compReport *complexity.Report
	)

	err := withTempWorktree(ctx, gitRoot, c.SHA, func(wtPath string) error {
		targetPath := filepath.Join(wtPath, relPath)
		proj, err := project.LoadProjectInfo(targetPath)
		if err != nil {
			return fmt.Errorf("load Go project: %w", err)
		}
		proj.OrganizationPackages = orgPackages

		var mErr error
		metrics, compReport, mErr = measureDirectory(ctx, proj, includeComplexity)
		return mErr
	})
	if err != nil {
		return nil, nil, err
	}

	return &CommitSnapshot{
		CommitSHA: c.SHA,
		ShortSHA:  c.Short,
		Date:      c.Date,
		Message:   c.Message,
		Tag:       c.Tag,
		Metrics:   metrics,
		Cached:    false,
	}, compReport, nil
}

func measureDirectory(ctx context.Context, proj *project.ProjectInfo, includeComplexity bool) (SnapshotMetrics, *complexity.Report, error) {
	var (
		locRes      *loc.ProjectLOC
		typesRes    *types.ProjectTypes
		ifaceRes    *interfaces.InterfaceMetrics
		commentsRes *comments.CommentsMetrics
		instabRes   []*instability.PackageInstability
		compReport  *complexity.Report
	)

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		res, err := loc.CountLoc(proj)
		if err != nil {
			return fmt.Errorf("LOC: %w", err)
		}
		locRes = res
		return nil
	})

	g.Go(func() error {
		res, err := types.ResolveProjectTypes(proj)
		if err != nil {
			return fmt.Errorf("types: %w", err)
		}
		typesRes = res
		return nil
	})

	g.Go(func() error {
		res, err := interfaces.ResolveInterfaceMetrics(proj)
		if err != nil {
			return fmt.Errorf("interfaces: %w", err)
		}
		ifaceRes = res
		return nil
	})

	g.Go(func() error {
		res, err := comments.ResolveCommentsMetrics(proj)
		if err != nil {
			return fmt.Errorf("comments: %w", err)
		}
		commentsRes = res
		return nil
	})

	g.Go(func() error {
		res, err := instability.GetInstability(proj)
		if err != nil {
			return fmt.Errorf("instability: %w", err)
		}
		instabRes = res
		return nil
	})

	if includeComplexity {
		g.Go(func() error {
			res, err := complexity.Calculate(proj.Path, proj.Package, 5)
			if err != nil {
				return nil
			}
			compReport = res
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return SnapshotMetrics{}, nil, err
	}

	var distanceSum float64
	painCount := 0
	uselessCount := 0
	for _, pkg := range instabRes {
		distanceSum += pkg.Distance
		if pkg.Distance >= 0.4 {
			if pkg.Abstractness+pkg.Instability < 1 {
				painCount++
			} else {
				uselessCount++
			}
		}
	}
	avgDistance := 0.0
	if len(instabRes) > 0 {
		avgDistance = distanceSum / float64(len(instabRes))
	}

	return SnapshotMetrics{
		LOC:                    locRes.Total,
		Packages:               typesRes.Packages,
		SourceFiles:            typesRes.SourceFiles,
		Structs:                typesRes.Structs,
		Interfaces:             typesRes.Interfaces,
		Functions:              typesRes.Functions,
		Methods:                typesRes.Methods,
		CommentsTotalLines:     commentsRes.TotalLines,
		CommentsRatio:          commentsRes.CommentsRatio,
		InterfaceAvgMethods:    ifaceRes.AverageMethods,
		AvgDistance:            avgDistance,
		ZoneOfPainCount:        painCount,
		ZoneOfUselessnessCount: uselessCount,
	}, compReport, nil
}
