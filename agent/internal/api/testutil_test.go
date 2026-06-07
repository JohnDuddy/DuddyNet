package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/duddynet/agent/internal/audit"
	"github.com/duddynet/agent/internal/config"
	"github.com/duddynet/agent/internal/health"
	"github.com/duddynet/agent/internal/models"
	"github.com/duddynet/agent/internal/store"
	"github.com/duddynet/agent/internal/token"
)

// testEnv bundles a running test server and its dependencies.
type testEnv struct {
	ts       *httptest.Server
	store    store.Store
	cfg      *config.Config
	codeSeq  int
	lastCode string // the pairing code used by the most recent pair() call
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.AllowedIPRanges = nil // allow loopback in tests
	cfg.LANSubnet = "127.0.0.0/8"
	cfg.DatabasePath = filepath.Join(dir, "db.json")
	cfg.TokenExpirationDays = 90

	st, err := store.OpenJSON(cfg.DatabasePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	quietLog := slog.New(slog.NewTextHandler(io.Discard, nil))
	aud := audit.New(st, quietLog)

	checker := health.NewChecker()
	checker.Safe = nil // allow ephemeral ports in tests

	srv := NewServer(Options{
		Config:  cfg,
		Store:   st,
		Audit:   aud,
		Checker: checker,
		Log:     quietLog,
		Version: "test",
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		ts.Close()
		_ = st.Close()
	})
	return &testEnv{ts: ts, store: st, cfg: cfg}
}

// pair seeds a unique pairing code with the given scopes and exchanges it for a
// token. The code used is recorded in e.lastCode for single-use assertions.
func (e *testEnv) pair(t *testing.T, scopes ...models.TokenScope) string {
	t.Helper()
	e.codeSeq++
	code := fmt.Sprintf("TEST-CODE-%04d", e.codeSeq)
	e.lastCode = code
	err := e.store.SavePairingCode(store.PairingCode{
		Code:      token.NormalizeCode(code),
		Label:     "test-device",
		Scopes:    scopes,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("seed pairing code: %v", err)
	}
	resp, body := e.do(t, http.MethodPost, "/api/v1/pair", "", map[string]string{
		"code": code, "label": "test-device",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("pair status = %d, body = %s", resp.StatusCode, body)
	}
	var pr models.PairingResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		t.Fatalf("decode pairing response: %v", err)
	}
	if pr.Token == "" {
		t.Fatal("expected a token in pairing response")
	}
	return pr.Token
}

// do performs an HTTP request against the test server and returns response+body.
func (e *testEnv) do(t *testing.T, method, path, bearer string, body any) (*http.Response, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.ts.URL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := e.ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp, data
}
