// Package audit records security-relevant actions to the store (for retrieval
// via GET /logs) and mirrors them to a structured slog logger.
//
// Invariant: audit entries NEVER contain secrets — no token values, no
// Authorization headers, no device/NAS/router passwords. Callers pass only the
// token *id* and a non-sensitive label.
package audit

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/duddynet/agent/internal/models"
)

// Sink is the subset of the store the auditor needs.
type Sink interface {
	AppendAudit(e models.AuditLogEntry) error
}

// Logger writes audit entries to a Sink and an slog.Logger.
type Logger struct {
	sink Sink
	log  *slog.Logger
}

// New returns a Logger. log may be nil (a no-op default is used).
func New(sink Sink, log *slog.Logger) *Logger {
	if log == nil {
		log = slog.Default()
	}
	return &Logger{sink: sink, log: log}
}

// Event describes one auditable action.
type Event struct {
	TokenID    string
	AppLabel   string
	Action     string
	TargetType string
	TargetID   string
	Success    bool
	RemoteIP   string
	Detail     string
}

// Record persists and logs an event. Errors writing to the store are logged but
// not returned — auditing must never block the primary request path, and a
// failed audit write should be visible in the process logs.
func (l *Logger) Record(e Event) {
	entry := models.AuditLogEntry{
		ID:         newID(),
		Timestamp:  time.Now().UTC(),
		TokenID:    e.TokenID,
		AppLabel:   e.AppLabel,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		Success:    e.Success,
		RemoteIP:   e.RemoteIP,
		Detail:     e.Detail,
	}
	if l.sink != nil {
		if err := l.sink.AppendAudit(entry); err != nil {
			l.log.Error("audit persist failed", "err", err, "action", e.Action)
		}
	}
	l.log.Info("audit",
		"action", e.Action,
		"success", e.Success,
		"token_id", e.TokenID,
		"app", e.AppLabel,
		"target_type", e.TargetType,
		"target_id", e.TargetID,
		"remote_ip", e.RemoteIP,
		"detail", e.Detail,
	)
}

func newID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fall back to timestamp-based id; collisions are harmless for logs.
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}
