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
	"strings"
	"sync"
	"time"

	"github.com/go-architect/go-architect-lib/coupling"
	"github.com/go-architect/go-architect-lib/dependency"
	"github.com/go-architect/go-architect-lib/dsm"
	"github.com/go-architect/go-architect-lib/instability"
	"github.com/go-architect/go-architect-lib/project"
	"github.com/go-architect/go-architect-lib/repository"
)

type Server struct {
	mu          sync.RWMutex
	projectInfo *project.ProjectInfo
	repoInfo    *repository.RepositoryInfo
	distFS      fs.FS
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

type ErrorResponse struct {
	Error string `json:"error"`
}

type switchProjectRequest struct {
	Path                 string   `json:"path"`
	OrganizationPackages []string `json:"organization_packages"`
}

func New(initialPath string, orgPackages []string, distFS fs.FS) (*Server, error) {
	s := &Server{
		distFS: distFS,
		mux:    http.NewServeMux(),
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
