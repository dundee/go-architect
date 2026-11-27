package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dundee/go-architect/backend/git"
	"github.com/dundee/go-architect/internal/arch"
	"github.com/dundee/go-architect/internal/cache"
	"github.com/dundee/go-architect/internal/complexity"
	"github.com/dundee/go-architect/internal/hotspots"
	"github.com/dundee/go-architect/internal/metrics"
	"github.com/dundee/go-architect/internal/trends"
	"github.com/go-architect/go-architect-lib/coupling"
	"github.com/go-architect/go-architect-lib/dependency"
	"github.com/go-architect/go-architect-lib/dsm"
	"github.com/go-architect/go-architect-lib/instability"
	"github.com/go-architect/go-architect-lib/metrics/comments"
	"github.com/go-architect/go-architect-lib/metrics/interfaces"
	"github.com/go-architect/go-architect-lib/metrics/loc"
	"github.com/go-architect/go-architect-lib/metrics/types"
	"github.com/go-architect/go-architect-lib/project"
	"github.com/go-architect/go-architect-lib/repository"
)

type Server struct {
	mu          sync.RWMutex
	projectInfo *project.ProjectInfo
	repoInfo    *repository.RepositoryInfo
	distFS      fs.FS
	cacheStore  *cache.Store
	trendsCache *trends.Cache
	mux         *http.ServeMux
}

type ProjectContextResponse struct {
	Project    *project.ProjectInfo       `json:"project"`
	Repository *repository.RepositoryInfo `json:"repository"`
}

type InstabilityResponse struct {
	Packages   []*instability.PackageInstability `json:"packages"`
	Cached     bool                              `json:"cached"`
	DurationMs int64                             `json:"duration_ms"`
}

type DependencyGraphResponse struct {
	Graph      *dependency.ModuleDependencyGraph `json:"graph"`
	Cached     bool                              `json:"cached"`
	DurationMs int64                             `json:"duration_ms"`
}

type DSMResponse struct {
	DSM        *dsm.DependencyStructureMatrix `json:"dsm"`
	Density    arch.GraphDensity              `json:"density"`
	Cached     bool                           `json:"cached"`
	DurationMs int64                          `json:"duration_ms"`
}

type CouplingResponse struct {
	Coupling   *coupling.DependencyCoupling `json:"coupling"`
	Cached     bool                         `json:"cached"`
	DurationMs int64                        `json:"duration_ms"`
}

type ComplexityResult struct {
	Data *complexity.Output `json:"data"`
}

type MetricsResponse struct {
	LOC                  *loc.ProjectLOC              `json:"loc"`
	Types                *types.ProjectTypes          `json:"types"`
	Interfaces           *interfaces.InterfaceMetrics `json:"interfaces"`
	Comments             *comments.CommentsMetrics    `json:"comments"`
	Tests                *cache.TestMetrics           `json:"tests"`
	Functions            *cache.FunctionMetrics       `json:"functions"`
	DocCoverage          *cache.DocCoverageMetrics    `json:"doc_coverage"`
	CyclomaticComplexity ComplexityResult             `json:"cyclomatic_complexity"`
	CognitiveComplexity  ComplexityResult             `json:"cognitive_complexity"`
	Cached               bool                         `json:"cached"`
	DurationMs           int64                        `json:"duration_ms"`
}

type VCSResponse struct {
	VCS        *git.VCSAnalysisInfo `json:"vcs"`
	Months     int                  `json:"months"`
	Cached     bool                 `json:"cached"`
	DurationMs int64                `json:"duration_ms"`
}

type HotspotsResponse struct {
	Hotspots   []hotspots.FileHotspot `json:"hotspots"`
	TotalFiles int                    `json:"total_files"`
	Months     int                    `json:"months"`
	Cached     bool                   `json:"cached"`
	DurationMs int64                  `json:"duration_ms"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type switchProjectRequest struct {
	Path                 string   `json:"path"`
	OrganizationPackages []string `json:"organization_packages"`
}

func New(initialPath string, orgPackages []string, distFS fs.FS) (*Server, error) {
	var store *cache.Store
	if dir, err := cache.DefaultDir(); err == nil {
		store = cache.NewStore(dir)
	}

	s := &Server{
		distFS:      distFS,
		cacheStore:  store,
		trendsCache: trends.NewCacheFromStore(store),
		mux:         http.NewServeMux(),
	}

	if err := s.LoadProject(initialPath, orgPackages); err != nil {
		return nil, err
	}

	s.registerRoutes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return guard(s.mux)
}

func (s *Server) LoadProject(rawPath string, orgPackages []string) error {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		trimmed = "."
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return fmt.Errorf("resolve project path %q: %w", rawPath, err)
	}

	proj, err := project.LoadProjectInfo(absPath)
	if err != nil {
		return fmt.Errorf("load Go project at %q: %w", absPath, err)
	}
	proj.OrganizationPackages = normalizeOrgPackages(orgPackages)

	repo, err := loadRepositoryInfo(absPath)
	if err != nil {
		// Non-git directories are still valid Go Projects.
		repo = nil
	}

	s.mu.Lock()
	s.projectInfo = proj
	s.repoInfo = repo
	s.mu.Unlock()

	return nil
}

func normalizeOrgPackages(orgPackages []string) []string {
	var clean []string
	seen := make(map[string]struct{})
	for _, p := range orgPackages {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		clean = append(clean, trimmed)
	}
	if clean == nil {
		return []string{}
	}
	return clean
}

func (s *Server) ProjectContext() ProjectContextResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ProjectContextResponse{
		Project:    s.projectInfo,
		Repository: s.repoInfo,
	}
}

func (s *Server) currentProjectSnapshot() *project.ProjectInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.projectInfo == nil {
		return nil
	}
	orgCopy := make([]string, len(s.projectInfo.OrganizationPackages))
	copy(orgCopy, s.projectInfo.OrganizationPackages)
	return &project.ProjectInfo{
		Name:                 s.projectInfo.Name,
		Path:                 s.projectInfo.Path,
		Package:              s.projectInfo.Package,
		OrganizationPackages: orgCopy,
	}
}

type cacheContext struct {
	projectKey    string
	complexityKey string
	sha           string
	canRead       bool
	canWrite      bool
}

func (s *Server) resolveCacheContext(ctx context.Context, proj *project.ProjectInfo, r *http.Request) cacheContext {
	if s.cacheStore == nil || proj == nil {
		return cacheContext{}
	}
	sha, isClean := cache.ResolveCleanCommit(ctx, proj.Path)
	if !isClean {
		return cacheContext{}
	}
	relPath := cache.ProjectSubdir(ctx, proj.Path)
	refresh := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "true")
	return cacheContext{
		projectKey:    cache.ProjectKey(proj.Package, relPath, proj.OrganizationPackages),
		complexityKey: cache.ProjectKey(proj.Package, relPath, nil),
		sha:           sha,
		canRead:       !refresh,
		canWrite:      true,
	}
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/project", s.handleGetProject)
	s.mux.HandleFunc("POST /api/project", s.handleSwitchProject)
	s.mux.HandleFunc("GET /api/instability", s.handleGetInstability)
	s.mux.HandleFunc("GET /api/dependency-graph", s.handleGetDependencyGraph)
	s.mux.HandleFunc("GET /api/dsm", s.handleGetDSM)
	s.mux.HandleFunc("GET /api/coupling", s.handleGetCoupling)
	s.mux.HandleFunc("GET /api/metrics", s.handleGetMetrics)
	s.mux.HandleFunc("GET /api/vcs", s.handleGetVCS)
	s.mux.HandleFunc("GET /api/hotspots", s.handleGetHotspots)
	s.mux.HandleFunc("GET /api/trends", s.handleGetTrends)
	s.mux.HandleFunc("GET /api/trends/stream", s.handleGetTrendsStream)
	s.mux.HandleFunc("/", s.handleSPA)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetProject(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.ProjectContext())
}

func (s *Server) handleSwitchProject(w http.ResponseWriter, r *http.Request) {
	var req switchProjectRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	orgPackages := req.OrganizationPackages
	if orgPackages == nil {
		if existing := s.currentProjectSnapshot(); existing != nil {
			orgPackages = existing.OrganizationPackages
		}
	}

	if err := s.LoadProject(req.Path, orgPackages); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, s.ProjectContext())
}

func (s *Server) handleGetInstability(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	start := time.Now()
	cc := s.resolveCacheContext(r.Context(), proj, r)
	if cc.canRead {
		if cached, ok := s.cacheStore.GetInstability(cc.projectKey, cc.sha); ok {
			writeJSON(w, http.StatusOK, InstabilityResponse{
				Packages:   cached,
				Cached:     true,
				DurationMs: time.Since(start).Milliseconds(),
			})
			return
		}
	}

	pkgs, err := arch.GetInstability(proj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("calculate instability: %v", err))
		return
	}
	if pkgs == nil {
		pkgs = []*instability.PackageInstability{}
	}

	if cc.canWrite {
		_ = s.cacheStore.PutInstability(cc.projectKey, cc.sha, pkgs)
	}

	writeJSON(w, http.StatusOK, InstabilityResponse{
		Packages:   pkgs,
		Cached:     false,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetDependencyGraph(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	start := time.Now()
	cc := s.resolveCacheContext(r.Context(), proj, r)
	if cc.canRead {
		if cached, ok := s.cacheStore.GetDependencyGraph(cc.projectKey, cc.sha); ok {
			writeJSON(w, http.StatusOK, DependencyGraphResponse{
				Graph:      cached,
				Cached:     true,
				DurationMs: time.Since(start).Milliseconds(),
			})
			return
		}
	}

	graph, err := arch.GetDependencyGraph(proj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("build dependency graph: %v", err))
		return
	}

	if graph.Module == "" {
		graph.Module = proj.Package
	}
	if graph.Internal == nil {
		graph.Internal = []string{}
	}
	if graph.External == nil {
		graph.External = []string{}
	}
	if graph.Organization == nil {
		graph.Organization = []string{}
	}
	if graph.StandardLib == nil {
		graph.StandardLib = []string{}
	}
	if graph.Relations == nil {
		graph.Relations = map[string][]string{}
	}

	if cc.canWrite {
		_ = s.cacheStore.PutDependencyGraph(cc.projectKey, cc.sha, graph)
	}

	writeJSON(w, http.StatusOK, DependencyGraphResponse{
		Graph:      graph,
		Cached:     false,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetDSM(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	includeTests := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_tests")), "true")

	start := time.Now()
	cc := s.resolveCacheContext(r.Context(), proj, r)
	dsmKey := cc.projectKey
	if includeTests && dsmKey != "" {
		dsmKey += "::tests"
	}
	if cc.canRead {
		if cached, ok := s.cacheStore.GetDSM(dsmKey, cc.sha); ok {
			writeJSON(w, http.StatusOK, DSMResponse{
				DSM:        cached,
				Density:    arch.CalculateDensityFromDSM(cached),
				Cached:     true,
				DurationMs: time.Since(start).Milliseconds(),
			})
			return
		}
	}

	matrix, err := arch.GetDSM(proj, includeTests)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("build DSM: %v", err))
		return
	}

	if matrix.Packages == nil {
		matrix.Packages = []string{}
	}
	if matrix.Dependencies == nil {
		matrix.Dependencies = [][]int64{}
	}

	if cc.canWrite {
		_ = s.cacheStore.PutDSM(dsmKey, cc.sha, matrix)
	}

	writeJSON(w, http.StatusOK, DSMResponse{
		DSM:        matrix,
		Density:    arch.CalculateDensityFromDSM(matrix),
		Cached:     false,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetCoupling(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	dep := strings.TrimSpace(r.URL.Query().Get("dependency"))
	if dep == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'dependency' is required")
		return
	}

	includeTests := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_tests")), "true")

	start := time.Now()
	cc := s.resolveCacheContext(r.Context(), proj, r)
	couplingKey := cc.complexityKey
	if includeTests && couplingKey != "" {
		couplingKey += "::tests"
	}
	if cc.canRead {
		if cached, ok := s.cacheStore.GetCoupling(couplingKey, cc.sha, dep); ok {
			writeJSON(w, http.StatusOK, CouplingResponse{
				Coupling:   cached,
				Cached:     true,
				DurationMs: time.Since(start).Milliseconds(),
			})
			return
		}
	}

	result, err := arch.CalculateCoupling(proj, dep, includeTests)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("calculate coupling: %v", err))
		return
	}

	if result.PackageDetails == nil {
		result.PackageDetails = []*coupling.PackageCoupling{}
	}

	if cc.canWrite {
		_ = s.cacheStore.PutCoupling(couplingKey, cc.sha, dep, result)
	}

	writeJSON(w, http.StatusOK, CouplingResponse{
		Coupling:   result,
		Cached:     false,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	start := time.Now()
	cc := s.resolveCacheContext(r.Context(), proj, r)

	var (
		baseMetrics *cache.MetricsBase
		baseCached  bool
		compReport  *complexity.Report
		compCached  bool
	)

	if cc.canRead {
		baseMetrics, baseCached = s.cacheStore.GetMetricsBase(cc.complexityKey, cc.sha)
		compReport, compCached = s.cacheStore.GetComplexity(cc.complexityKey, cc.sha)
	}

	if !baseCached || !compCached {
		res, err := metrics.Calculate(proj.Path, proj.Package, metrics.Options{
			NeedBase:       !baseCached,
			NeedComplexity: !compCached,
			TopN:           5,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("calculate project metrics: %v", err))
			return
		}
		if !baseCached && res.Base != nil {
			baseMetrics = res.Base
			if cc.canWrite {
				_ = s.cacheStore.PutMetricsBase(cc.complexityKey, cc.sha, baseMetrics)
			}
		}
		if !compCached && res.Complexity != nil {
			compReport = res.Complexity
			if cc.canWrite {
				_ = s.cacheStore.PutComplexity(cc.complexityKey, cc.sha, compReport)
			}
		}
	}

	var cycloRes, cognitRes ComplexityResult
	if compReport != nil {
		cycloRes = ComplexityResult{Data: &compReport.Cyclomatic}
		cognitRes = ComplexityResult{Data: &compReport.Cognitive}
	}

	writeJSON(w, http.StatusOK, MetricsResponse{
		LOC:                  baseMetrics.LOC,
		Types:                baseMetrics.Types,
		Interfaces:           baseMetrics.Interfaces,
		Comments:             baseMetrics.Comments,
		Tests:                baseMetrics.Tests,
		Functions:            baseMetrics.Functions,
		DocCoverage:          baseMetrics.DocCoverage,
		CyclomaticComplexity: cycloRes,
		CognitiveComplexity:  cognitRes,
		Cached:               baseCached && compCached,
		DurationMs:           time.Since(start).Milliseconds(),
	})
}

func parseMonthsQuery(r *http.Request, defaultMonths int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("months"))
	if raw == "" {
		return defaultMonths, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("query parameter 'months' must be a non-negative integer")
	}
	return parsed, nil
}

func sinceForMonths(months int) time.Time {
	if months == 0 {
		return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Now().AddDate(0, -months, 0)
}

func (s *Server) loadVCSInfo(proj *project.ProjectInfo, cc cacheContext, months int) (*git.VCSAnalysisInfo, bool, error) {
	dayKey := time.Now().UTC().Format("2006-01-02")
	if cc.canRead {
		if cached, ok := s.cacheStore.GetVCS(cc.complexityKey, cc.sha, months, dayKey); ok {
			return cached, true, nil
		}
	}

	since := sinceForMonths(months)
	vcsInfo, err := git.LoadRepositoryTeamCohesion(proj.Path, &since)
	if err != nil {
		return nil, false, err
	}
	if vcsInfo == nil {
		return nil, false, errors.New("empty VCS analysis result")
	}
	if vcsInfo.AuthorsInfo.AuthorDetails == nil {
		vcsInfo.AuthorsInfo.AuthorDetails = []git.AuthorInfo{}
	}
	if vcsInfo.ModificationsInfo.PathModifications == nil {
		vcsInfo.ModificationsInfo.PathModifications = []git.ModificationsInfo{}
	}
	if vcsInfo.ModificationsInfo.FileModifications == nil {
		vcsInfo.ModificationsInfo.FileModifications = []git.ModificationsInfo{}
	}

	if cc.canWrite {
		_ = s.cacheStore.PutVCS(cc.complexityKey, cc.sha, months, dayKey, vcsInfo)
	}
	return vcsInfo, false, nil
}

func (s *Server) handleGetVCS(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	months, err := parseMonthsQuery(r, 6)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	start := time.Now()
	cc := s.resolveCacheContext(r.Context(), proj, r)
	vcsInfo, cached, err := s.loadVCSInfo(proj, cc, months)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("analyze Git repository: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, VCSResponse{
		VCS:        vcsInfo,
		Months:     months,
		Cached:     cached,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetHotspots(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	months, err := parseMonthsQuery(r, 6)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	start := time.Now()
	cc := s.resolveCacheContext(r.Context(), proj, r)
	vcsInfo, vcsCached, err := s.loadVCSInfo(proj, cc, months)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("analyze Git repository: %v", err))
		return
	}

	report, err := hotspots.Analyze(r.Context(), proj, vcsInfo, sinceForMonths(months))
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("calculate hotspots: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, HotspotsResponse{
		Hotspots:   report.Hotspots,
		TotalFiles: report.TotalFiles,
		Months:     months,
		Cached:     vcsCached,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func parseTrendsOptions(r *http.Request) (trends.Options, error) {
	mode := trends.ModeBuckets
	if rawMode := strings.TrimSpace(r.URL.Query().Get("mode")); rawMode != "" {
		switch trends.SamplingMode(rawMode) {
		case trends.ModeBuckets, trends.ModeTags:
			mode = trends.SamplingMode(rawMode)
		default:
			return trends.Options{}, fmt.Errorf("query parameter 'mode' must be 'buckets' or 'tags'")
		}
	}

	months := 12
	if rawMonths := strings.TrimSpace(r.URL.Query().Get("months")); rawMonths != "" {
		parsed, err := strconv.Atoi(rawMonths)
		if err != nil || parsed < 0 {
			return trends.Options{}, fmt.Errorf("query parameter 'months' must be a non-negative integer")
		}
		months = parsed
	}

	maxPoints := 12
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 2 || parsed > 30 {
			return trends.Options{}, fmt.Errorf("query parameter 'limit' must be between 2 and 30")
		}
		maxPoints = parsed
	}

	return trends.Options{
		Mode:      mode,
		Months:    months,
		MaxPoints: maxPoints,
	}, nil
}

func (s *Server) handleGetTrends(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	opts, err := parseTrendsOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := trends.Analyze(r.Context(), proj, s.trendsCache, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("analyze historical trends: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleGetTrendsStream(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	opts, err := parseTrendsOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sendEvent := func(name string, payload any) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
		flusher.Flush()
	}

	report, err := trends.AnalyzeWithProgress(r.Context(), proj, s.trendsCache, opts, func(ev trends.ProgressEvent) {
		sendEvent(ev.Type, ev)
	})
	if err != nil {
		if r.Context().Err() == nil {
			sendEvent("failed", ErrorResponse{Error: fmt.Sprintf("analyze historical trends: %v", err)})
		}
		return
	}

	sendEvent("complete", report)
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "API endpoint not found")
		return
	}

	if s.distFS == nil {
		writeError(w, http.StatusNotFound, "frontend assets not available")
		return
	}

	cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if cleanPath == "" {
		cleanPath = "index.html"
	}

	if info, err := fs.Stat(s.distFS, cleanPath); err == nil && !info.IsDir() {
		http.FileServer(http.FS(s.distFS)).ServeHTTP(w, r)
		return
	}

	// SPA fallback to index.html for client-side routes.
	indexBytes, err := fs.ReadFile(s.distFS, "index.html")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusNotFound, "index.html not found in embedded assets")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load index.html")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexBytes)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}
