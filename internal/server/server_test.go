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

	srv, err := New(repoRoot, testDistFS())
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

		reqFallback := httptest.NewRequest(http.MethodGet, "/instability", nil)
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
