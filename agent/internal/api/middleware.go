package api

import (
	"net/http"
	"time"
)

// statusRecorder captures the response status code for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// recoverMiddleware converts panics into 500s and logs them. Without this a
// single bad request could take down the agent.
func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic recovered", "err", rec, "path", r.URL.Path)
				errInternal(w, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// logMiddleware emits a structured access log line per request. It deliberately
// logs only method, path, status, and duration — never headers or bodies, which
// could contain the Authorization token.
func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(sr, r)
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.status,
			"remote", clientIP(r),
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}

// ipAllowMiddleware enforces the configured allowed_ip_ranges. With Tailscale as
// the primary boundary this is defense-in-depth; an empty range list allows all.
func (s *Server) ipAllowMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !s.cfg.IPAllowed(ip) {
			s.log.Warn("blocked by ip allowlist", "remote", ip)
			errForbidden(w, "source address not permitted")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// chain applies middlewares in order (outermost first).
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
