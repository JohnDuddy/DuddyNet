package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/duddynet/agent/internal/models"
)

func newStore(t *testing.T) *JSONStore {
	t.Helper()
	s, err := OpenJSON(filepath.Join(t.TempDir(), "db.json"))
	if err != nil {
		t.Fatalf("OpenJSON: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDeviceCRUD(t *testing.T) {
	s := newStore(t)
	now := time.Now().UTC()
	d := models.Device{ID: "d1", Name: "NAS", Type: models.DeviceTypeNAS, LANIP: "192.168.1.10", CreatedAt: now, UpdatedAt: now}

	if err := s.CreateDevice(d); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.CreateDevice(d); err != ErrConflict {
		t.Fatalf("expected ErrConflict on duplicate, got %v", err)
	}

	got, err := s.GetDevice("d1")
	if err != nil || got.Name != "NAS" {
		t.Fatalf("get: %v %+v", err, got)
	}

	got.Name = "NAS-2"
	if err := s.UpdateDevice(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reload, _ := s.GetDevice("d1")
	if reload.Name != "NAS-2" {
		t.Fatalf("update not persisted: %q", reload.Name)
	}

	if err := s.DeleteDevice("d1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetDevice("d1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteDeviceCascadesServices(t *testing.T) {
	s := newStore(t)
	_ = s.CreateDevice(models.Device{ID: "d1", Name: "NAS"})
	_ = s.CreateService(models.Service{ID: "s1", DeviceID: "d1", Name: "DSM"})
	_ = s.CreateService(models.Service{ID: "s2", DeviceID: "other", Name: "Keep"})

	if err := s.DeleteDevice("d1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	svcs, _ := s.ListServices()
	if len(svcs) != 1 || svcs[0].ID != "s2" {
		t.Fatalf("expected only s2 to remain, got %+v", svcs)
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")
	s1, err := OpenJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s1.CreateDevice(models.Device{ID: "d1", Name: "Router"})
	_ = s1.Close()

	s2, err := OpenJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := s2.GetDevice("d1")
	if err != nil || got.Name != "Router" {
		t.Fatalf("data did not persist: %v %+v", err, got)
	}
}

func TestAuditPaginationAndFilter(t *testing.T) {
	s := newStore(t)
	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		_ = s.AppendAudit(models.AuditLogEntry{
			ID: string(rune('a' + i)), Timestamp: base.Add(time.Duration(i) * time.Minute),
			Action: "device.check", Success: i%2 == 0,
		})
	}
	_ = s.AppendAudit(models.AuditLogEntry{ID: "z", Timestamp: base.Add(time.Hour), Action: "device.wake", Success: true})

	// Filter by action.
	items, total, _ := s.ListAudit(AuditFilter{Action: "device.check"})
	if total != 5 || len(items) != 5 {
		t.Fatalf("expected 5 check entries, got total=%d len=%d", total, len(items))
	}
	// Newest first.
	if !items[0].Timestamp.After(items[1].Timestamp) {
		t.Fatal("expected newest-first ordering")
	}
	// Pagination.
	page, total, _ := s.ListAudit(AuditFilter{Action: "device.check", Limit: 2, Offset: 0})
	if len(page) != 2 || total != 5 {
		t.Fatalf("expected page of 2 of 5, got len=%d total=%d", len(page), total)
	}
	// Success filter.
	yes := true
	okItems, _, _ := s.ListAudit(AuditFilter{Success: &yes})
	for _, e := range okItems {
		if !e.Success {
			t.Fatal("success filter returned a failure entry")
		}
	}
}

func TestPairingCodeConsumeOnce(t *testing.T) {
	s := newStore(t)
	now := time.Now()
	_ = s.SavePairingCode(PairingCode{Code: "ABC", ExpiresAt: now.Add(time.Minute)})

	if _, err := s.ConsumePairingCode("ABC", now); err != nil {
		t.Fatalf("first consume should succeed: %v", err)
	}
	if _, err := s.ConsumePairingCode("ABC", now); err != ErrNotFound {
		t.Fatalf("second consume should fail with ErrNotFound, got %v", err)
	}
}

func TestPairingCodeExpiry(t *testing.T) {
	s := newStore(t)
	now := time.Now()
	_ = s.SavePairingCode(PairingCode{Code: "OLD", ExpiresAt: now.Add(-time.Minute)})
	if _, err := s.ConsumePairingCode("OLD", now); err != ErrNotFound {
		t.Fatalf("expired code should not be consumable, got %v", err)
	}
}

// TestReloadOnExternalChange simulates the real-world case where the running
// `serve` process and a separate `revoke-token` CLI each open the same JSON file.
// A revocation written by the second instance must be visible to the first on its
// next read, without a restart.
func TestReloadOnExternalChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")

	server, err := OpenJSON(path) // stands in for the running agent
	if err != nil {
		t.Fatalf("open server store: %v", err)
	}
	rec := TokenRecord{
		ID: "t1", Label: "Phone", LookupHash: "lh1", TokenHash: "th1", Salt: "s1",
		Scopes: []models.TokenScope{models.ScopeReadOnly}, ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := server.CreateToken(rec); err != nil {
		t.Fatalf("create token: %v", err)
	}

	// Sanity: the server sees the token as active.
	if got, _ := server.FindTokenByLookup("lh1"); got.Revoked {
		t.Fatal("token should start active")
	}

	// Ensure a distinct file modtime for the external write.
	time.Sleep(20 * time.Millisecond)

	// A separate process (CLI) opens the same file and revokes the token.
	cli, err := OpenJSON(path)
	if err != nil {
		t.Fatalf("open cli store: %v", err)
	}
	if err := cli.RevokeToken("t1"); err != nil {
		t.Fatalf("cli revoke: %v", err)
	}

	// The server must observe the revocation on its next lookup.
	got, err := server.FindTokenByLookup("lh1")
	if err != nil {
		t.Fatalf("server lookup after external revoke: %v", err)
	}
	if !got.Revoked {
		t.Fatal("server did not observe the externally-written revocation (reload-on-change failed)")
	}
}

func TestTokenLifecycle(t *testing.T) {
	s := newStore(t)
	rec := TokenRecord{ID: "t1", Label: "Pixel", LookupHash: "lh1", TokenHash: "th1", Salt: "s1",
		Scopes: []models.TokenScope{models.ScopeReadOnly}, ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.CreateToken(rec); err != nil {
		t.Fatalf("create token: %v", err)
	}
	found, err := s.FindTokenByLookup("lh1")
	if err != nil || found.ID != "t1" {
		t.Fatalf("find by lookup: %v %+v", err, found)
	}
	// Public view must not leak secrets.
	pub := found.Public()
	if pub.TokenHash != "" || pub.Salt != "" {
		t.Fatal("public token view leaked secret material")
	}
	if err := s.RevokeToken("t1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	after, _ := s.GetToken("t1")
	if !after.Revoked {
		t.Fatal("token should be revoked")
	}
}
