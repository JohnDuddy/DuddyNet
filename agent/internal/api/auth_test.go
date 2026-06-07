package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/duddynet/agent/internal/models"
)

func TestHealthIsPublic(t *testing.T) {
	e := newTestEnv(t)
	resp, body := e.do(t, http.MethodGet, "/api/v1/health", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", resp.StatusCode, body)
	}
	var info models.HealthInfo
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.Service != "duddynet-agent" {
		t.Fatalf("unexpected service: %q", info.Service)
	}
}

func TestProtectedEndpointRequiresToken(t *testing.T) {
	e := newTestEnv(t)
	resp, _ := e.do(t, http.MethodGet, "/api/v1/devices", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestPairingFlowAndAuthorizedRead(t *testing.T) {
	e := newTestEnv(t)
	tok := e.pair(t, models.ScopeReadOnly)
	resp, body := e.do(t, http.MethodGet, "/api/v1/devices", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authorized read failed: %d %s", resp.StatusCode, body)
	}
}

func TestPairingCodeIsSingleUse(t *testing.T) {
	e := newTestEnv(t)
	_ = e.pair(t, models.ScopeReadOnly)
	// Re-submitting the same code must fail (already consumed).
	resp, _ := e.do(t, http.MethodPost, "/api/v1/pair", "", map[string]string{"code": e.lastCode})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused pairing code should be rejected, got %d", resp.StatusCode)
	}
}

func TestReadOnlyTokenCannotWrite(t *testing.T) {
	e := newTestEnv(t)
	tok := e.pair(t, models.ScopeReadOnly)
	resp, _ := e.do(t, http.MethodPost, "/api/v1/devices", tok, map[string]any{"name": "NAS"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only token should not create devices, got %d", resp.StatusCode)
	}
}

func TestDeviceAdminCanWrite(t *testing.T) {
	e := newTestEnv(t)
	tok := e.pair(t, models.ScopeDeviceAdmin)
	resp, body := e.do(t, http.MethodPost, "/api/v1/devices", tok, map[string]any{
		"name": "NAS", "type": "nas", "lan_ip": "127.0.0.1", "allowed_ports": []int{5000},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("device-admin should create devices, got %d: %s", resp.StatusCode, body)
	}
}

func TestRevokedTokenLosesAccess(t *testing.T) {
	e := newTestEnv(t)
	tok := e.pair(t, models.ScopeReadOnly)

	// Confirm it works first.
	if resp, _ := e.do(t, http.MethodGet, "/api/v1/devices", tok, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("token should work before revoke, got %d", resp.StatusCode)
	}

	// Revoke every token in the store (there is exactly one).
	tokens, _ := e.store.ListTokens()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if err := e.store.RevokeToken(tokens[0].ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	resp, _ := e.do(t, http.MethodGet, "/api/v1/devices", tok, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked token should be unauthorized, got %d", resp.StatusCode)
	}
}

func TestWOLScopeEnforced(t *testing.T) {
	e := newTestEnv(t)
	// Create a device with an admin token.
	adminTok := e.pair(t, models.ScopeDeviceAdmin)
	_, body := e.do(t, http.MethodPost, "/api/v1/devices", adminTok, map[string]any{
		"name": "PC", "type": "pc", "lan_ip": "127.0.0.1", "mac_address": "AA:BB:CC:DD:EE:FF",
	})
	var dev models.Device
	if err := json.Unmarshal(body, &dev); err != nil {
		t.Fatalf("decode device: %v", err)
	}

	// A read-only token must NOT be able to wake.
	roTok := e.pair(t, models.ScopeReadOnly)
	resp, _ := e.do(t, http.MethodPost, "/api/v1/devices/"+dev.ID+"/wake", roTok, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only token must not wake, got %d", resp.StatusCode)
	}
}

func TestInvalidTokenRejected(t *testing.T) {
	e := newTestEnv(t)
	resp, _ := e.do(t, http.MethodGet, "/api/v1/devices", "ddn_not-a-real-token", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("garbage token should be 401, got %d", resp.StatusCode)
	}
}
