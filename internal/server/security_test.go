package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLocalRequest builds a request as a browser on the loopback UI would send it.
func newLocalRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = "127.0.0.1:8080"
	return req
}

func TestGuard(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	srv, err := New(repoRoot, nil, testDistFS())
	require.NoError(t, err)

	// Encode via json.Marshal: Windows paths contain backslashes that are
	// invalid JSON escapes when concatenated into a literal.
	switchPayload, err := json.Marshal(switchProjectRequest{Path: repoRoot})
	require.NoError(t, err)
	switchBody := func() io.Reader {
		return bytes.NewReader(switchPayload)
	}

	tests := []struct {
		name   string
		build  func() *http.Request
		status int
	}{
		{"GET loopback IP", func() *http.Request { return newLocalRequest(http.MethodGet, "/api/health", nil) }, http.StatusOK},
		{"GET localhost", func() *http.Request {
			r := newLocalRequest(http.MethodGet, "/api/health", nil)
			r.Host = "localhost:8080"
			return r
		}, http.StatusOK},
		{"GET IPv6 loopback", func() *http.Request {
			r := newLocalRequest(http.MethodGet, "/api/health", nil)
			r.Host = "[::1]:8080"
			return r
		}, http.StatusOK},
		{"GET rebinding domain", func() *http.Request {
			r := newLocalRequest(http.MethodGet, "/api/project", nil)
			r.Host = "evil.example.com:8080"
			return r
		}, http.StatusForbidden},
		{"POST text/plain (simple CSRF)", func() *http.Request {
			r := newLocalRequest(http.MethodPost, "/api/project", switchBody())
			r.Header.Set("Content-Type", "text/plain")
			return r
		}, http.StatusUnsupportedMediaType},
		{"POST cross-origin JSON", func() *http.Request {
			r := newLocalRequest(http.MethodPost, "/api/project", switchBody())
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", "https://evil.example.com")
			return r
		}, http.StatusForbidden},
		{"POST same-origin JSON", func() *http.Request {
			r := newLocalRequest(http.MethodPost, "/api/project", switchBody())
			r.Header.Set("Content-Type", "application/json; charset=utf-8")
			r.Header.Set("Origin", "http://127.0.0.1:8080")
			return r
		}, http.StatusOK},
		{"POST via Vite dev proxy", func() *http.Request {
			r := newLocalRequest(http.MethodPost, "/api/project", switchBody())
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", "http://localhost:5173")
			return r
		}, http.StatusOK},
		{"POST without Origin (curl)", func() *http.Request {
			r := newLocalRequest(http.MethodPost, "/api/project", switchBody())
			r.Header.Set("Content-Type", "application/json")
			return r
		}, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, tc.build())
			assert.Equal(t, tc.status, rec.Code, rec.Body.String())
		})
	}
}
