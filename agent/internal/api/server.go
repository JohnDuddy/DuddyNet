package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/duddynet/agent/internal/audit"
	"github.com/duddynet/agent/internal/config"
	"github.com/duddynet/agent/internal/health"
	"github.com/duddynet/agent/internal/models"
	"github.com/duddynet/agent/internal/store"
	"github.com/duddynet/agent/internal/wol"
)

// auditEvent is an alias so api code can build audit events ergonomically.
type auditEvent = audit.Event

// Server holds the dependencies and HTTP routing for the agent API.
type Server struct {
	cfg       *config.Config
	store     store.Store
	audit     *audit.Logger
	checker   *health.Checker
	wol       wol.Sender
	log       *slog.Logger
	version   string
	startTime time.Time

	pairLimiter *rateLimiter
}

// Options configures a Server.
type Options struct {
	Config  *config.Config
	Store   store.Store
	Audit   *audit.Logger
	Checker *health.Checker
	WOL     wol.Sender
	Log     *slog.Logger
	Version string
}

// NewServer builds a Server with sane defaults for any nil collaborators.
func NewServer(o Options) *Server {
	log := o.Log
	if log == nil {
		log = slog.Default()
	}
	checker := o.Checker
	if checker == nil {
		checker = health.NewChecker()
	}
	var sender wol.Sender = o.WOL
	if sender == nil {
		sender = wol.UDPSender{}
	}
	return &Server{
		cfg:         o.Config,
		store:       o.Store,
		audit:       o.Audit,
		checker:     checker,
		wol:         sender,
		log:         log,
		version:     o.Version,
		startTime:   time.Now(),
		pairLimiter: newRateLimiter(5, time.Minute), // 5 pairing attempts / IP / minute
	}
}

// Handler returns the fully-wired http.Handler (routes + global middleware).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public (Tailscale identity is the boundary; no app token required).
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/pair", s.handlePair)

	// Devices. Reads require only a valid token; writes require device-admin.
	mux.HandleFunc("GET /api/v1/devices", s.requireAuth(read(), s.handleListDevices))
	mux.HandleFunc("POST /api/v1/devices", s.requireAuth(admin(), s.handleCreateDevice))
	mux.HandleFunc("GET /api/v1/devices/{id}", s.requireAuth(read(), s.handleGetDevice))
	mux.HandleFunc("PATCH /api/v1/devices/{id}", s.requireAuth(admin(), s.handleUpdateDevice))
	mux.HandleFunc("DELETE /api/v1/devices/{id}", s.requireAuth(admin(), s.handleDeleteDevice))
	mux.HandleFunc("POST /api/v1/devices/{id}/check", s.requireAuth(read(), s.handleCheckDevice))
	mux.HandleFunc("POST /api/v1/devices/{id}/wake", s.requireAuth(scopes(models.ScopeWOL), s.handleWakeDevice))

	// Services.
	mux.HandleFunc("GET /api/v1/services", s.requireAuth(read(), s.handleListServices))
	mux.HandleFunc("POST /api/v1/services", s.requireAuth(admin(), s.handleCreateService))
	mux.HandleFunc("GET /api/v1/services/{id}", s.requireAuth(read(), s.handleGetService))
	mux.HandleFunc("PATCH /api/v1/services/{id}", s.requireAuth(admin(), s.handleUpdateService))
	mux.HandleFunc("DELETE /api/v1/services/{id}", s.requireAuth(admin(), s.handleDeleteService))

	// Logs.
	mux.HandleFunc("GET /api/v1/logs", s.requireAuth(scopes(models.ScopeLogs), s.handleListLogs))

	// Token management (admin only).
	mux.HandleFunc("GET /api/v1/tokens", s.requireAuth(scopes(models.ScopeFullAdmin), s.handleListTokens))
	mux.HandleFunc("POST /api/v1/tokens/{id}/revoke", s.requireAuth(scopes(models.ScopeFullAdmin), s.handleRevokeToken))

	return chain(mux, s.recoverMiddleware, s.logMiddleware, s.ipAllowMiddleware)
}

// scope-set helpers keep route declarations readable.
//
// read() requires only a valid (authenticated) token — any scope can read.
// admin() requires the device-admin capability for mutations.
func read() []models.TokenScope  { return nil }
func admin() []models.TokenScope { return []models.TokenScope{models.ScopeDeviceAdmin} }
func scopes(s ...models.TokenScope) []models.TokenScope { return s }

// writeBlockedByReadOnly reports whether a mutating request must be refused
// because the agent is globally read-only.
func (s *Server) writeBlockedByReadOnly(w http.ResponseWriter) bool {
	if s.cfg != nil && s.cfg.ReadOnlyDefault {
		errForbidden(w, "agent is in read-only mode")
		return true
	}
	return false
}
