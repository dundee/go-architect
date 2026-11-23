package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

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
		require.Equal(t, "github.com/go-architect/go-architect", resp.Project.Package)
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
		require.Equal(t, "github.com/go-architect/go-architect", resp.Graph.Module)
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
		require.NotNil(t, resp.CyclomaticComplexity.Data)
		require.NotEmpty(t, resp.CyclomaticComplexity.Data.ComplexityDetails)
		require.NotNil(t, resp.CognitiveComplexity.Data)
		require.NotEmpty(t, resp.CognitiveComplexity.Data.ComplexityDetails)
	})

	t.Run("GET /api/vcs", func(t *testing.T) {
		reqInvalid := httptest.NewRequest(http.MethodGet, "/api/vcs?months=-2", nil)
		recInvalid := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recInvalid, reqInvalid)
		require.Equal(t, http.StatusBadRequest, recInvalid.Code)

		req := httptest.NewRequest(http.MethodGet, "/api/vcs?months=12", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var resp VCSResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, 12, resp.Months)
		require.NotNil(t, resp.VCS)
		require.NotNil(t, resp.VCS.AuthorsInfo.AuthorDetails)
	})

	t.Run("GET /api/trends", func(t *testing.T) {
		reqInvalid := httptest.NewRequest(http.MethodGet, "/api/trends?mode=invalid", nil)
		recInvalid := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recInvalid, reqInvalid)
		require.Equal(t, http.StatusBadRequest, recInvalid.Code)

		req := httptest.NewRequest(http.MethodGet, "/api/trends?mode=buckets&months=12&limit=3", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"points"`)

		reqStream := httptest.NewRequest(http.MethodGet, "/api/trends/stream?mode=buckets&months=12&limit=3", nil)
		recStream := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recStream, reqStream)
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
