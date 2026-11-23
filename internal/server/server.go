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

type ErrorResponse struct {
	Error string `json:"error"`
}

type switchProjectRequest struct {
	Path string `json:"path"`
}

func New(initialPath string, distFS fs.FS) (*Server, error) {
	s := &Server{
		distFS: distFS,
		mux:    http.NewServeMux(),
	}

	if err := s.LoadProject(initialPath); err != nil {
		return nil, err
	}

	s.registerRoutes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) LoadProject(rawPath string) error {
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

	repo, err := repository.LoadRepositoryInfo(absPath)
	if err != nil {
		// Non-git directories are still valid Go modules.
		repo = nil
	}

	s.mu.Lock()
	s.projectInfo = proj
	s.repoInfo = repo
	s.mu.Unlock()

	return nil
}

func (s *Server) ProjectContext() ProjectContextResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ProjectContextResponse{
		Project:    s.projectInfo,
		Repository: s.repoInfo,
	}
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/project", s.handleGetProject)
	s.mux.HandleFunc("POST /api/project", s.handleSwitchProject)
	s.mux.HandleFunc("GET /api/instability", s.handleGetInstability)
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

	if err := s.LoadProject(req.Path); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, s.ProjectContext())
}

func (s *Server) handleGetInstability(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	proj := s.projectInfo
	s.mu.RUnlock()

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
