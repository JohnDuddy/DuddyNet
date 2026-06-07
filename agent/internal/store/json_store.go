package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/duddynet/agent/internal/models"
)

// jsonDB is the on-disk shape of the JSON store.
type jsonDB struct {
	Devices      []models.Device   `json:"devices"`
	Services     []models.Service  `json:"services"`
	Tokens       []TokenRecord     `json:"tokens"`
	PairingCodes []PairingCode     `json:"pairing_codes"`
	Audit        []models.AuditLogEntry `json:"audit"`
}

// JSONStore is a mutex-guarded, in-memory store that persists the whole DB to a
// JSON file on every mutation using atomic write-and-rename. It is intended for
// the MVP / small single-home deployments; swap in SQLite for larger datasets.
type JSONStore struct {
	mu   sync.RWMutex
	path string
	db   jsonDB
	// maxAudit caps audit entries kept in memory/on disk (ring buffer).
	maxAudit int
	// modTime is the file modtime of the last copy we loaded. It lets a running
	// `serve` process notice mutations made by a separate CLI invocation (e.g.
	// `revoke-token`, `pair`) and reload, so revocation/pairing take effect
	// without a restart. This is a pragmatic measure for the JSON backend;
	// SQLite would handle cross-process consistency natively.
	modTime time.Time
}

// OpenJSON loads (or initializes) the JSON store at path.
func OpenJSON(path string) (*JSONStore, error) {
	if path == "" {
		return nil, fmt.Errorf("json store: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("json store: create dir: %w", err)
	}
	s := &JSONStore{path: path, maxAudit: 10000}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(data) > 0 {
			if err := json.Unmarshal(data, &s.db); err != nil {
				return nil, fmt.Errorf("json store: parse %s: %w", path, err)
			}
		}
		if info, statErr := os.Stat(path); statErr == nil {
			s.modTime = info.ModTime()
		}
	case os.IsNotExist(err):
		// New store; persist an empty DB so the file exists with tight perms.
		if err := s.persistLocked(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("json store: read %s: %w", path, err)
	}
	return s, nil
}

// reloadIfChangedLocked reloads the DB from disk if the file was modified by
// another process since we last wrote/read it. Caller must hold s.mu (write).
// Failures are non-fatal: we keep the current in-memory state.
func (s *JSONStore) reloadIfChangedLocked() {
	info, err := os.Stat(s.path)
	if err != nil {
		return
	}
	if info.ModTime().Equal(s.modTime) {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var fresh jsonDB
	if len(data) == 0 {
		fresh = jsonDB{}
	} else if err := json.Unmarshal(data, &fresh); err != nil {
		return
	}
	s.db = fresh
	s.modTime = info.ModTime()
}

// persistLocked writes the DB atomically. Caller must hold s.mu (write).
func (s *JSONStore) persistLocked() error {
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return fmt.Errorf("json store: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("json store: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("json store: rename: %w", err)
	}
	// Record our own write's modtime so reloadIfChangedLocked doesn't treat it
	// as an external change.
	if info, err := os.Stat(s.path); err == nil {
		s.modTime = info.ModTime()
	}
	return nil
}

func (s *JSONStore) Close() error { return nil }

// ---- Devices ----

func (s *JSONStore) ListDevices() ([]models.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	out := make([]models.Device, len(s.db.Devices))
	copy(out, s.db.Devices)
	return out, nil
}

func (s *JSONStore) GetDevice(id string) (models.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for _, d := range s.db.Devices {
		if d.ID == id {
			return d, nil
		}
	}
	return models.Device{}, ErrNotFound
}

func (s *JSONStore) CreateDevice(d models.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for _, x := range s.db.Devices {
		if x.ID == d.ID {
			return ErrConflict
		}
	}
	s.db.Devices = append(s.db.Devices, d)
	return s.persistLocked()
}

func (s *JSONStore) UpdateDevice(d models.Device) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for i, x := range s.db.Devices {
		if x.ID == d.ID {
			s.db.Devices[i] = d
			return s.persistLocked()
		}
	}
	return ErrNotFound
}

func (s *JSONStore) DeleteDevice(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for i, x := range s.db.Devices {
		if x.ID == id {
			s.db.Devices = append(s.db.Devices[:i], s.db.Devices[i+1:]...)
			// Cascade: drop services attached to this device.
			kept := s.db.Services[:0]
			for _, svc := range s.db.Services {
				if svc.DeviceID != id {
					kept = append(kept, svc)
				}
			}
			s.db.Services = kept
			return s.persistLocked()
		}
	}
	return ErrNotFound
}

// ---- Services ----

func (s *JSONStore) ListServices() ([]models.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	out := make([]models.Service, len(s.db.Services))
	copy(out, s.db.Services)
	return out, nil
}

func (s *JSONStore) GetService(id string) (models.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for _, x := range s.db.Services {
		if x.ID == id {
			return x, nil
		}
	}
	return models.Service{}, ErrNotFound
}

func (s *JSONStore) CreateService(svc models.Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for _, x := range s.db.Services {
		if x.ID == svc.ID {
			return ErrConflict
		}
	}
	s.db.Services = append(s.db.Services, svc)
	return s.persistLocked()
}

func (s *JSONStore) UpdateService(svc models.Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for i, x := range s.db.Services {
		if x.ID == svc.ID {
			s.db.Services[i] = svc
			return s.persistLocked()
		}
	}
	return ErrNotFound
}

func (s *JSONStore) DeleteService(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for i, x := range s.db.Services {
		if x.ID == id {
			s.db.Services = append(s.db.Services[:i], s.db.Services[i+1:]...)
			return s.persistLocked()
		}
	}
	return ErrNotFound
}

// ---- Tokens ----

func (s *JSONStore) CreateToken(r TokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for _, x := range s.db.Tokens {
		if x.ID == r.ID || x.LookupHash == r.LookupHash {
			return ErrConflict
		}
	}
	s.db.Tokens = append(s.db.Tokens, r)
	return s.persistLocked()
}

func (s *JSONStore) GetToken(id string) (TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for _, x := range s.db.Tokens {
		if x.ID == id {
			return x, nil
		}
	}
	return TokenRecord{}, ErrNotFound
}

func (s *JSONStore) FindTokenByLookup(lookupHash string) (TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for _, x := range s.db.Tokens {
		if x.LookupHash == lookupHash {
			return x, nil
		}
	}
	return TokenRecord{}, ErrNotFound
}

func (s *JSONStore) ListTokens() ([]models.AppToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	out := make([]models.AppToken, 0, len(s.db.Tokens))
	for _, x := range s.db.Tokens {
		out = append(out, x.Public())
	}
	return out, nil
}

func (s *JSONStore) TouchToken(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for i, x := range s.db.Tokens {
		if x.ID == id {
			t := at
			s.db.Tokens[i].LastUsedAt = &t
			return s.persistLocked()
		}
	}
	return ErrNotFound
}

func (s *JSONStore) RevokeToken(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for i, x := range s.db.Tokens {
		if x.ID == id {
			s.db.Tokens[i].Revoked = true
			return s.persistLocked()
		}
	}
	return ErrNotFound
}

// ---- Pairing codes ----

func (s *JSONStore) SavePairingCode(pc PairingCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	s.db.PairingCodes = append(s.db.PairingCodes, pc)
	return s.persistLocked()
}

func (s *JSONStore) ConsumePairingCode(normalizedCode string, now time.Time) (PairingCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	for i, pc := range s.db.PairingCodes {
		if pc.Code != normalizedCode {
			continue
		}
		if pc.Used || now.After(pc.ExpiresAt) {
			// A matching-but-spent/expired entry doesn't disqualify others.
			continue
		}
		s.db.PairingCodes[i].Used = true
		if err := s.persistLocked(); err != nil {
			return PairingCode{}, err
		}
		return pc, nil
	}
	return PairingCode{}, ErrNotFound
}

func (s *JSONStore) PrunePairingCodes(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	kept := s.db.PairingCodes[:0]
	for _, pc := range s.db.PairingCodes {
		if !pc.Used && now.Before(pc.ExpiresAt) {
			kept = append(kept, pc)
		}
	}
	s.db.PairingCodes = kept
	return s.persistLocked()
}

// ---- Audit ----

func (s *JSONStore) AppendAudit(e models.AuditLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()
	s.db.Audit = append(s.db.Audit, e)
	if len(s.db.Audit) > s.maxAudit {
		s.db.Audit = s.db.Audit[len(s.db.Audit)-s.maxAudit:]
	}
	return s.persistLocked()
}

func (s *JSONStore) ListAudit(f AuditFilter) ([]models.AuditLogEntry, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadIfChangedLocked()

	// Filter.
	var filtered []models.AuditLogEntry
	for _, e := range s.db.Audit {
		if f.DeviceID != "" && e.TargetID != f.DeviceID {
			continue
		}
		if f.Action != "" && e.Action != f.Action {
			continue
		}
		if f.Success != nil && e.Success != *f.Success {
			continue
		}
		if !f.Since.IsZero() && e.Timestamp.Before(f.Since) {
			continue
		}
		if !f.Until.IsZero() && e.Timestamp.After(f.Until) {
			continue
		}
		filtered = append(filtered, e)
	}

	// Newest first.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	total := len(filtered)

	// Paginate.
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := make([]models.AuditLogEntry, end-offset)
	copy(page, filtered[offset:end])
	return page, total, nil
}
