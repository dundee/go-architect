package server

import (
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
	"github.com/go-architect/go-architect/backend/git"
	"github.com/go-architect/go-architect/internal/complexity"
	"github.com/go-architect/go-architect/internal/trends"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	mu          sync.RWMutex
	projectInfo *project.ProjectInfo
	repoInfo    *repository.RepositoryInfo
	distFS      fs.FS
	trendsCache *trends.Cache
	mux         *http.ServeMux
}

type ProjectContextResponse struct {
	Project    *project.ProjectInfo       `json:"project"`
	Repository *repository.RepositoryInfo `json:"repository"`
}

type InstabilityResponse struct {
	Packages   []*instability.PackageInstability `json:"packages"`
	DurationMs int64                             `json:"duration_ms"`
}

type DependencyGraphResponse struct {
	Graph      *dependency.ModuleDependencyGraph `json:"graph"`
	DurationMs int64                             `json:"duration_ms"`
}

type DSMResponse struct {
	DSM        *dsm.DependencyStructureMatrix `json:"dsm"`
	DurationMs int64                          `json:"duration_ms"`
}

type CouplingResponse struct {
	Coupling   *coupling.DependencyCoupling `json:"coupling"`
	DurationMs int64                        `json:"duration_ms"`
}

type ComplexityResult struct {
	Data        *complexity.Output `json:"data"`
	Error       string             `json:"error,omitempty"`
	InstallHint string             `json:"install_hint,omitempty"`
}

type MetricsResponse struct {
	LOC                  *loc.ProjectLOC              `json:"loc"`
	Types                *types.ProjectTypes          `json:"types"`
	Interfaces           *interfaces.InterfaceMetrics `json:"interfaces"`
	Comments             *comments.CommentsMetrics    `json:"comments"`
	CyclomaticComplexity ComplexityResult             `json:"cyclomatic_complexity"`
	CognitiveComplexity  ComplexityResult             `json:"cognitive_complexity"`
	DurationMs           int64                        `json:"duration_ms"`
}

type VCSResponse struct {
	VCS        *git.VCSAnalysisInfo `json:"vcs"`
	Months     int                  `json:"months"`
	DurationMs int64                `json:"duration_ms"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type switchProjectRequest struct {
	Path                 string   `json:"path"`
	OrganizationPackages []string `json:"organization_packages"`
}

func New(initialPath string, orgPackages []string, distFS fs.FS) (*Server, error) {
	var cache *trends.Cache
	if dir, err := trends.DefaultCacheDir(); err == nil {
		cache = trends.NewCache(dir)
	}

	s := &Server{
		distFS:      distFS,
		trendsCache: cache,
		mux:         http.NewServeMux(),
	}

	if err := s.LoadProject(initialPath, orgPackages); err != nil {
		return nil, err
	}

	s.registerRoutes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
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

	repo, err := repository.LoadRepositoryInfo(absPath)
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

func (s *Server) handleGetInstability(w http.ResponseWriter, _ *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	start := time.Now()
	pkgs, err := instability.GetInstability(proj)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("calculate instability: %v", err))
		return
	}
	if pkgs == nil {
		pkgs = []*instability.PackageInstability{}
	}

	writeJSON(w, http.StatusOK, InstabilityResponse{
		Packages:   pkgs,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetDependencyGraph(w http.ResponseWriter, _ *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	start := time.Now()
	graph, err := dependency.GetDependencyGraph(proj, "ALL")
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

	writeJSON(w, http.StatusOK, DependencyGraphResponse{
		Graph:      graph,
		DurationMs: time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetDSM(w http.ResponseWriter, _ *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	start := time.Now()
	matrix, err := dsm.GetDependencyStructureMatrix(proj)
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

	writeJSON(w, http.StatusOK, DSMResponse{
		DSM:        matrix,
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

	start := time.Now()
	result, err := coupling.CalculateCoupling(proj, dep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("calculate coupling: %v", err))
		return
	}

	if result.PackageDetails == nil {
		result.PackageDetails = []*coupling.PackageCoupling{}
	}

	writeJSON(w, http.StatusOK, CouplingResponse{
		Coupling:   result,
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

	var (
		locRes      *loc.ProjectLOC
		typesRes    *types.ProjectTypes
		ifaceRes    *interfaces.InterfaceMetrics
		commentsRes *comments.CommentsMetrics
		cycloRes    ComplexityResult
		cognitRes   ComplexityResult
	)

	g, _ := errgroup.WithContext(r.Context())

	g.Go(func() error {
		res, err := loc.CountLoc(proj)
		if err != nil {
			return fmt.Errorf("count LOC: %w", err)
		}
		locRes = res
		return nil
	})

	g.Go(func() error {
		res, err := types.ResolveProjectTypes(proj)
		if err != nil {
			return fmt.Errorf("resolve project types: %w", err)
		}
		typesRes = res
		return nil
	})

	g.Go(func() error {
		res, err := interfaces.ResolveInterfaceMetrics(proj)
		if err != nil {
			return fmt.Errorf("resolve interface metrics: %w", err)
		}
		ifaceRes = res
		return nil
	})

	g.Go(func() error {
		res, err := comments.ResolveCommentsMetrics(proj)
		if err != nil {
			return fmt.Errorf("resolve comments metrics: %w", err)
		}
		commentsRes = res
		return nil
	})

	g.Go(func() error {
		compReport, err := complexity.Calculate(proj.Path, proj.Package, 5)
		if err != nil {
			cycloRes = ComplexityResult{Error: err.Error()}
			cognitRes = ComplexityResult{Error: err.Error()}
			return nil
		}
		cycloRes = ComplexityResult{Data: &compReport.Cyclomatic}
		cognitRes = ComplexityResult{Data: &compReport.Cognitive}
		return nil
	})

	if err := g.Wait(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, MetricsResponse{
		LOC:                  locRes,
		Types:                typesRes,
		Interfaces:           ifaceRes,
		Comments:             commentsRes,
		CyclomaticComplexity: cycloRes,
		CognitiveComplexity:  cognitRes,
		DurationMs:           time.Since(start).Milliseconds(),
	})
}

func (s *Server) handleGetVCS(w http.ResponseWriter, r *http.Request) {
	proj := s.currentProjectSnapshot()
	if proj == nil {
		writeError(w, http.StatusBadRequest, "no Go project is loaded")
		return
	}

	months := 6
	if raw := strings.TrimSpace(r.URL.Query().Get("months")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "query parameter 'months' must be a non-negative integer")
			return
		}
		months = parsed
	}

	var since time.Time
	if months == 0 {
		since = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		since = time.Now().AddDate(0, -months, 0)
	}

	start := time.Now()
	vcsInfo, err := git.LoadRepositoryTeamCohesion(proj.Path, &since)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("analyze Git repository: %v", err))
		return
	}
	if vcsInfo == nil {
		writeError(w, http.StatusInternalServerError, "empty VCS analysis result")
		return
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

	writeJSON(w, http.StatusOK, VCSResponse{
		VCS:        vcsInfo,
		Months:     months,
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

	includeComplexity := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_complexity")), "true")

	return trends.Options{
		Mode:              mode,
		Months:            months,
		MaxPoints:         maxPoints,
		IncludeComplexity: includeComplexity,
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
