package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/duddynet/agent/internal/models"
	"github.com/duddynet/agent/internal/token"
)

// identity is the authenticated caller, stored in the request context.
type identity struct {
	TokenID string
	Label   string
	Scopes  map[models.TokenScope]bool
}

// Has reports whether the identity holds scope (full-admin implies all).
func (id *identity) Has(scope models.TokenScope) bool {
	if id == nil {
		return false
	}
	if id.Scopes[models.ScopeFullAdmin] {
		return true
	}
	return id.Scopes[scope]
}

type ctxKey int

const identityKey ctxKey = 1

func identityFrom(ctx context.Context) *identity {
	v, _ := ctx.Value(identityKey).(*identity)
	return v
}

// authenticate resolves a bearer token to an identity. Returns (nil, reason) on
// failure. It records last-used and treats expired/revoked tokens as invalid.
func (s *Server) authenticate(r *http.Request) (*identity, string) {
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return nil, "missing bearer token"
	}
	raw := strings.TrimSpace(authz[len(prefix):])
	if raw == "" {
		return nil, "empty token"
	}

	rec, err := s.store.FindTokenByLookup(token.HashForLookup(raw))
	if err != nil {
		return nil, "unknown token"
	}
	if !token.Verify(raw, rec.Salt, rec.TokenHash) {
		// LookupHash collision is cryptographically implausible, but verify anyway.
		return nil, "token verification failed"
	}
	if rec.Revoked {
		return nil, "token revoked"
	}
	if !rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt) {
		return nil, "token expired"
	}

	// Best-effort last-used update (don't fail the request on error).
	_ = s.store.TouchToken(rec.ID, time.Now().UTC())

	scopes := make(map[models.TokenScope]bool, len(rec.Scopes))
	for _, sc := range rec.Scopes {
		scopes[sc] = true
	}
	return &identity{TokenID: rec.ID, Label: rec.Label, Scopes: scopes}, ""
}

// requireAuth wraps a handler, enforcing a valid token plus all listed scopes.
func (s *Server) requireAuth(scopes []models.TokenScope, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, reason := s.authenticate(r)
		if id == nil {
			s.audit.Record(auditFromReq(r, nil, "auth.denied", "system", "", false, reason))
			errUnauthorized(w, reason)
			return
		}
		for _, sc := range scopes {
			if !id.Has(sc) {
				s.audit.Record(auditFromReq(r, id, "auth.forbidden", "system", "", false, "missing scope "+string(sc)))
				errForbidden(w, "token lacks required scope: "+string(sc))
				return
			}
		}
		ctx := context.WithValue(r.Context(), identityKey, id)
		h(w, r.WithContext(ctx))
	}
}

// clientIP extracts the remote IP (Tailscale peer IP when invoked over the
// tailnet). We intentionally do NOT trust X-Forwarded-For by default because the
// agent is meant to be reached directly over Tailscale, not behind a proxy.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func auditFromReq(r *http.Request, id *identity, action, targetType, targetID string, success bool, detail string) (e auditEvent) {
	e = auditEvent{
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Success:    success,
		RemoteIP:   clientIP(r),
		Detail:     detail,
	}
	if id != nil {
		e.TokenID = id.TokenID
		e.AppLabel = id.Label
	}
	return e
}

// rateLimiter is a tiny fixed-window limiter keyed by string (e.g. client IP).
// Used to throttle pairing attempts and brute force of pairing codes.
type rateLimiter struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	hits   map[string][]time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{window: window, max: max, hits: make(map[string][]time.Time)}
}

// Allow records an attempt for key and reports whether it is within the limit.
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.max {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}
