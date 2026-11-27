package server

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// guard protects the local API from hostile web pages open in the user's browser.
//
//   - DNS rebinding: only Host headers that are IP literals or "localhost" are
//     accepted, so a rebound attacker domain cannot read API responses.
//   - CSRF: state-changing requests must send Content-Type application/json
//     (which forces a CORS preflight we never approve) and, when an Origin is
//     present, it must be same-host or loopback (the Vite dev proxy).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHost(r.Host) {
			writeError(w, http.StatusForbidden, "forbidden host")
			return
		}
		if !isSafeMethod(r.Method) {
			if !isJSONContentType(r.Header.Get("Content-Type")) {
				writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" && !allowedOrigin(origin, r.Host) {
				writeError(w, http.StatusForbidden, "cross-origin request rejected")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func isJSONContentType(ct string) bool {
	mt, _, err := mime.ParseMediaType(ct)
	return err == nil && mt == "application/json"
}

func hostname(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return strings.Trim(hostport, "[]")
}

func allowedHost(hostport string) bool {
	h := strings.ToLower(hostname(hostport))
	if h == "localhost" {
		return true
	}
	return net.ParseIP(h) != nil
}

func isLoopbackHost(h string) bool {
	h = strings.ToLower(h)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func allowedOrigin(origin, reqHost string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, reqHost) {
		return true
	}
	return isLoopbackHost(u.Hostname())
}
