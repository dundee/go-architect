package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
)

func testDistFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {Data: []byte("<html><body>Go Architect</body></html>")},
		"assets/app.js": {
			Data: []byte("console.log('ok')"),
		},
	}
}

func runGitTest(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
}

func createTempGoGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init")
	runGitTest(t, dir, nil, "config", "user.name", "Test User")
	runGitTest(t, dir, nil, "config", "user.email", "test@example.com")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/ci-test\n\ngo 1.24\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644))
	t1 := time.Now().AddDate(0, -2, 0).Format(time.RFC3339)
	runGitTest(t, dir, []string{"GIT_AUTHOR_DATE=" + t1, "GIT_COMMITTER_DATE=" + t1}, "add", ".")
	runGitTest(t, dir, []string{"GIT_AUTHOR_DATE=" + t1, "GIT_COMMITTER_DATE=" + t1}, "commit", "-m", "initial commit")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "util.go"), []byte("package main\nfunc add(a, b int) int { return a + b }\n"), 0o644))
	t2 := time.Now().AddDate(0, -1, 0).Format(time.RFC3339)
	runGitTest(t, dir, []string{"GIT_AUTHOR_DATE=" + t2, "GIT_COMMITTER_DATE=" + t2}, "add", ".")
	runGitTest(t, dir, []string{"GIT_AUTHOR_DATE=" + t2, "GIT_COMMITTER_DATE=" + t2}, "commit", "-m", "add util")

	return dir
}

func TestServerEndpoints(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	srv, err := New(repoRoot, []string{"github.com/go-architect/go-architect-lib"}, testDistFS())
	require.NoError(t, err)

	t.Run("GET /api/health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"status":"ok"`)
	})

	t.Run("GET /api/project", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/project", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp ProjectContextResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotNil(t, resp.Project)
		require.Equal(t, "github.com/dundee/go-architect", resp.Project.Package)
		require.Equal(t, []string{"github.com/go-architect/go-architect-lib"}, resp.Project.OrganizationPackages)
	})

	t.Run("GET /api/instability", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/instability", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp InstabilityResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotEmpty(t, resp.Packages)
	})

	t.Run("GET /api/dependency-graph", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/dependency-graph", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp DependencyGraphResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotNil(t, resp.Graph)
		require.Equal(t, "github.com/dundee/go-architect", resp.Graph.Module)
		require.NotEmpty(t, resp.Graph.Internal)
		require.NotEmpty(t, resp.Graph.Relations)
	})

	t.Run("GET /api/dsm", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/dsm", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp DSMResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotNil(t, resp.DSM)
		require.NotEmpty(t, resp.DSM.Packages)
		require.Equal(t, len(resp.DSM.Packages), len(resp.DSM.Dependencies))
	})

	t.Run("GET /api/coupling", func(t *testing.T) {
		reqMissing := httptest.NewRequest(http.MethodGet, "/api/coupling", nil)
		recMissing := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recMissing, reqMissing)
		require.Equal(t, http.StatusBadRequest, recMissing.Code)

		req := httptest.NewRequest(http.MethodGet, "/api/coupling?dependency=github.com/go-architect/go-architect-lib/instability", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp CouplingResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotNil(t, resp.Coupling)
		require.Equal(t, "github.com/go-architect/go-architect-lib/instability", resp.Coupling.Dependency)
		require.Greater(t, resp.Coupling.CouplingLevel, 0)
		require.NotEmpty(t, resp.Coupling.PackageDetails)
	})

	t.Run("GET /api/metrics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp MetricsResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.NotNil(t, resp.LOC)
		require.Greater(t, resp.LOC.Total, 0)
		require.NotNil(t, resp.Types)
		require.NotNil(t, resp.Interfaces)
		require.NotNil(t, resp.Comments)
		require.NotNil(t, resp.Tests)
		require.Greater(t, resp.Tests.LOC, 0)
		require.Greater(t, resp.Tests.Files, 0)
		require.NotNil(t, resp.CyclomaticComplexity.Data)
		require.NotEmpty(t, resp.CyclomaticComplexity.Data.ComplexityDetails)
		require.NotNil(t, resp.CognitiveComplexity.Data)
		require.NotEmpty(t, resp.CognitiveComplexity.Data.ComplexityDetails)
	})

	t.Run("GET /api/vcs", func(t *testing.T) {
		gitRepoDir := createTempGoGitRepo(t)
		vcsSrv, err := New(gitRepoDir, nil, testDistFS())
		require.NoError(t, err)

		reqInvalid := httptest.NewRequest(http.MethodGet, "/api/vcs?months=-2", nil)
		recInvalid := httptest.NewRecorder()
		vcsSrv.Handler().ServeHTTP(recInvalid, reqInvalid)
		require.Equal(t, http.StatusBadRequest, recInvalid.Code)

		req := httptest.NewRequest(http.MethodGet, "/api/vcs?months=12", nil)
		rec := httptest.NewRecorder()
		vcsSrv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp VCSResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, 12, resp.Months)
		require.NotNil(t, resp.VCS)
		require.Greater(t, resp.VCS.AuthorsInfo.TotalCommits, 0)
		require.NotNil(t, resp.VCS.AuthorsInfo.AuthorDetails)
	})

	t.Run("GET /api/trends", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		gitRepoDir := createTempGoGitRepo(t)
		trendsSrv, err := New(gitRepoDir, nil, testDistFS())
		require.NoError(t, err)

		reqInvalid := httptest.NewRequest(http.MethodGet, "/api/trends?mode=invalid", nil)
		recInvalid := httptest.NewRecorder()
		trendsSrv.Handler().ServeHTTP(recInvalid, reqInvalid)
		require.Equal(t, http.StatusBadRequest, recInvalid.Code)

		req := httptest.NewRequest(http.MethodGet, "/api/trends?mode=buckets&months=12&limit=3", nil)
		rec := httptest.NewRecorder()
		trendsSrv.Handler().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"points"`)

		reqStream := httptest.NewRequest(http.MethodGet, "/api/trends/stream?mode=buckets&months=12&limit=3", nil)
		recStream := httptest.NewRecorder()
		trendsSrv.Handler().ServeHTTP(recStream, reqStream)
		require.Equal(t, http.StatusOK, recStream.Code)
		require.Contains(t, recStream.Header().Get("Content-Type"), "text/event-stream")
		require.Contains(t, recStream.Body.String(), "event: start")
		require.Contains(t, recStream.Body.String(), "event: progress")
		require.Contains(t, recStream.Body.String(), "event: complete")
	})

	t.Run("POST /api/project rejects invalid path", func(t *testing.T) {
		body := bytes.NewBufferString(`{"path":"/definitely/nonexistent/path/xyz"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/project", body)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
		require.NotEmpty(t, errResp.Error)
	})

	t.Run("SPA static & fallback routes", func(t *testing.T) {
		reqAsset := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		recAsset := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recAsset, reqAsset)
		require.Equal(t, http.StatusOK, recAsset.Code)
		require.Equal(t, "console.log('ok')", recAsset.Body.String())

		reqFallback := httptest.NewRequest(http.MethodGet, "/graph", nil)
		recFallback := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recFallback, reqFallback)
		require.Equal(t, http.StatusOK, recFallback.Code)
		require.Contains(t, recFallback.Body.String(), "Go Architect")

		reqUnknownAPI := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
		recUnknownAPI := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recUnknownAPI, reqUnknownAPI)
		require.Equal(t, http.StatusNotFound, recUnknownAPI.Code)
	})
}

func TestServerCommitCaching(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	gitRepoDir := createTempGoGitRepo(t)
	srv, err := New(gitRepoDir, nil, testDistFS())
	require.NoError(t, err)

	// 1. First request misses cache and populates protobuf cache files.
	for _, path := range []string{
		"/api/instability",
		"/api/dependency-graph",
		"/api/dsm",
		"/api/coupling?dependency=fmt",
		"/api/metrics",
		"/api/vcs?months=6",
	} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path)

		var meta struct {
			Cached bool `json:"cached"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
		require.False(t, meta.Cached, "expected cache miss on first request for %s", path)
	}

	// Verify separate complexity_*.pb and metrics_*.pb files exist.
	cacheFiles, err := filepath.Glob(filepath.Join(srv.cacheStore.DirForTest(), "*.pb"))
	require.NoError(t, err)
	require.NotEmpty(t, cacheFiles)

	// 2. Second request hits protobuf cache for every tab.
	for _, path := range []string{
		"/api/instability",
		"/api/dependency-graph",
		"/api/dsm",
		"/api/coupling?dependency=fmt",
		"/api/metrics",
		"/api/vcs?months=6",
	} {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path)

		var meta struct {
			Cached bool `json:"cached"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
		require.True(t, meta.Cached, "expected cache hit on second request for %s", path)
	}

	// 3. Trends reuses HEAD's complexity_*.pb written by /api/metrics.
	recTrends := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recTrends, httptest.NewRequest(http.MethodGet, "/api/trends?mode=buckets&months=12", nil))
	require.Equal(t, http.StatusOK, recTrends.Code)
	require.Contains(t, recTrends.Body.String(), `"cyclomatic_complexity_avg"`)

	// 4. Explicit refresh=true bypasses cache read.
	recRefresh := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recRefresh, httptest.NewRequest(http.MethodGet, "/api/instability?refresh=true", nil))
	require.Equal(t, http.StatusOK, recRefresh.Code)
	var metaRefresh struct {
		Cached bool `json:"cached"`
	}
	require.NoError(t, json.Unmarshal(recRefresh.Body.Bytes(), &metaRefresh))
	require.False(t, metaRefresh.Cached)

	// 5. Dirty working tree bypasses commit cache.
	require.NoError(t, os.WriteFile(filepath.Join(gitRepoDir, "untracked.go"), []byte("package main\n"), 0o644))
	recDirty := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDirty, httptest.NewRequest(http.MethodGet, "/api/instability", nil))
	require.Equal(t, http.StatusOK, recDirty.Code)
	var metaDirty struct {
		Cached bool `json:"cached"`
	}
	require.NoError(t, json.Unmarshal(recDirty.Body.Bytes(), &metaDirty))
	require.False(t, metaDirty.Cached)
}
