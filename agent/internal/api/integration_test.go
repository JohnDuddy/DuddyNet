package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/duddynet/agent/internal/audit"
	"github.com/duddynet/agent/internal/config"
	"github.com/duddynet/agent/internal/health"
	"github.com/duddynet/agent/internal/models"
	"github.com/duddynet/agent/internal/store"
)

// TestHealthCheckAgainstMockDevice exercises the full stack: pair -> create a
// device pointing at a mock home-network endpoint -> run a health check via the
// API -> assert the agent reports it online.
func TestHealthCheckAgainstMockDevice(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer mock.Close()

	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(mock.URL, "http://"))
	port, _ := strconv.Atoi(portStr)

	e := newTestEnv(t)
	tok := e.pair(t, models.ScopeDeviceAdmin)

	// Create the device.
	_, body := e.do(t, http.MethodPost, "/api/v1/devices", tok, map[string]any{
		"name":          "Mock NAS",
		"type":          "nas",
		"lan_ip":        host,
		"allowed_ports": []int{port},
	})
	var dev models.Device
	if err := json.Unmarshal(body, &dev); err != nil {
		t.Fatalf("decode device: %v", err)
	}

	// Run the health check.
	resp, checkBody := e.do(t, http.MethodPost, "/api/v1/devices/"+dev.ID+"/check", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("check status = %d: %s", resp.StatusCode, checkBody)
	}
	var hs models.HealthStatus
	if err := json.Unmarshal(checkBody, &hs); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if !hs.Online {
		t.Fatalf("expected mock device online, got %+v", hs)
	}
	if len(hs.OpenPorts) != 1 || hs.OpenPorts[0] != port {
		t.Fatalf("expected open port %d, got %v", port, hs.OpenPorts)
	}

	// The action should have been audited (visible to a logs-scoped token).
	logTok := e.pair(t, models.ScopeLogs)
	lResp, lBody := e.do(t, http.MethodGet, "/api/v1/logs?action=device.check", logTok, nil)
	if lResp.StatusCode != http.StatusOK {
		t.Fatalf("logs status = %d: %s", lResp.StatusCode, lBody)
	}
	var page models.Page[models.AuditLogEntry]
	if err := json.Unmarshal(lBody, &page); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	if page.Total < 1 {
		t.Fatal("expected at least one device.check audit entry")
	}
}

// fakeWOL captures wake calls instead of sending UDP.
type fakeWOL struct {
	mu     sync.Mutex
	mac    string
	addr   string
	called bool
}

func (f *fakeWOL) Send(mac, addr string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mac, f.addr, f.called = mac, addr, true
	return nil
}

// TestWakeOnLANEndpoint verifies the WOL path with an injected sender so no
// packet hits the network during tests.
func TestWakeOnLANEndpoint(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.AllowedIPRanges = nil
	cfg.LANSubnet = "192.168.1.0/24"
	cfg.DatabasePath = filepath.Join(dir, "db.json")

	st, err := store.OpenJSON(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	fake := &fakeWOL{}
	srv := NewServer(Options{
		Config: cfg, Store: st, Audit: audit.New(st, quiet),
		Checker: health.NewChecker(), WOL: fake, Log: quiet, Version: "test",
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	e := &testEnv{ts: ts, store: st, cfg: cfg}
	adminTok := e.pair(t, models.ScopeDeviceAdmin, models.ScopeWOL)

	_, body := e.do(t, http.MethodPost, "/api/v1/devices", adminTok, map[string]any{
		"name": "Office PC", "type": "pc", "lan_ip": "192.168.1.20",
		"mac_address": "AA:BB:CC:DD:EE:FF",
	})
	var dev models.Device
	if err := json.Unmarshal(body, &dev); err != nil {
		t.Fatalf("decode device: %v", err)
	}

	resp, wakeBody := e.do(t, http.MethodPost, "/api/v1/devices/"+dev.ID+"/wake", adminTok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("wake status = %d: %s", resp.StatusCode, wakeBody)
	}
	var wr models.WakeResult
	if err := json.Unmarshal(wakeBody, &wr); err != nil {
		t.Fatalf("decode wake result: %v", err)
	}
	if !wr.Success {
		t.Fatalf("expected wake success, got %+v", wr)
	}
	if !fake.called || fake.mac != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("fake sender not invoked correctly: %+v", fake)
	}
	if fake.addr != "192.168.1.255:9" {
		t.Fatalf("expected broadcast 192.168.1.255:9, got %q", fake.addr)
	}
}

// TestWakeRejectsDeviceWithoutMAC ensures WOL validation refuses MAC-less devices.
func TestWakeRejectsDeviceWithoutMAC(t *testing.T) {
	e := newTestEnv(t)
	adminTok := e.pair(t, models.ScopeDeviceAdmin, models.ScopeWOL)
	_, body := e.do(t, http.MethodPost, "/api/v1/devices", adminTok, map[string]any{
		"name": "No MAC", "type": "pc", "lan_ip": "127.0.0.1",
	})
	var dev models.Device
	_ = json.Unmarshal(body, &dev)

	resp, _ := e.do(t, http.MethodPost, "/api/v1/devices/"+dev.ID+"/wake", adminTok, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wake without MAC should be 400, got %d", resp.StatusCode)
	}
}
