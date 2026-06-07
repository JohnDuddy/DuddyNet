// Package store defines the persistence abstraction for the agent and ships a
// JSON-file implementation. The interface is deliberately narrow so a SQLite (or
// other) backend can be dropped in later without touching the API layer.
package store

import (
	"errors"
	"time"

	"github.com/duddynet/agent/internal/models"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned on uniqueness violations (e.g. duplicate id).
var ErrConflict = errors.New("conflict")

// TokenRecord is the full persisted token, including secret material. It is
// internal to the store/auth layers and is never serialized to API clients
// (handlers return models.AppToken instead).
type TokenRecord struct {
	ID         string              `json:"id"`
	Label      string              `json:"label"`
	Scopes     []models.TokenScope `json:"scopes"`
	CreatedAt  time.Time           `json:"created_at"`
	ExpiresAt  time.Time           `json:"expires_at"`
	LastUsedAt *time.Time          `json:"last_used_at,omitempty"`
	Revoked    bool                `json:"revoked"`
	TokenHash  string              `json:"token_hash"`
	Salt       string              `json:"salt"`
	LookupHash string              `json:"lookup_hash"` // unsalted hash, map key only
}

// Public converts a TokenRecord to its secret-free API representation.
func (r TokenRecord) Public() models.AppToken {
	return models.AppToken{
		ID:         r.ID,
		Label:      r.Label,
		Scopes:     r.Scopes,
		CreatedAt:  r.CreatedAt,
		ExpiresAt:  r.ExpiresAt,
		LastUsedAt: r.LastUsedAt,
		Revoked:    r.Revoked,
	}
}

// PairingCode is a single-use, time-boxed code that exchanges for a token.
type PairingCode struct {
	Code      string              `json:"code"` // normalized (see token.NormalizeCode)
	Label     string              `json:"label"`
	Scopes    []models.TokenScope `json:"scopes"`
	CreatedAt time.Time           `json:"created_at"`
	ExpiresAt time.Time           `json:"expires_at"`
	Used      bool                `json:"used"`
}

// AuditFilter constrains a log query. Zero values mean "no constraint".
type AuditFilter struct {
	DeviceID string
	Action   string
	Success  *bool
	Since    time.Time
	Until    time.Time
	Limit    int
	Offset   int
}

// Store is the persistence contract. Implementations must be safe for
// concurrent use.
type Store interface {
	// Devices.
	ListDevices() ([]models.Device, error)
	GetDevice(id string) (models.Device, error)
	CreateDevice(d models.Device) error
	UpdateDevice(d models.Device) error
	DeleteDevice(id string) error

	// Services.
	ListServices() ([]models.Service, error)
	GetService(id string) (models.Service, error)
	CreateService(s models.Service) error
	UpdateService(s models.Service) error
	DeleteService(id string) error

	// Tokens.
	CreateToken(r TokenRecord) error
	GetToken(id string) (TokenRecord, error)
	FindTokenByLookup(lookupHash string) (TokenRecord, error)
	ListTokens() ([]models.AppToken, error)
	TouchToken(id string, at time.Time) error
	RevokeToken(id string) error

	// Pairing codes.
	SavePairingCode(pc PairingCode) error
	// ConsumePairingCode atomically validates and marks a code as used. It
	// returns ErrNotFound for unknown/expired/used codes.
	ConsumePairingCode(normalizedCode string, now time.Time) (PairingCode, error)
	PrunePairingCodes(now time.Time) error

	// Audit.
	AppendAudit(e models.AuditLogEntry) error
	ListAudit(f AuditFilter) (items []models.AuditLogEntry, total int, err error)

	Close() error
}
