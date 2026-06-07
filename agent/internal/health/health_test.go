package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/duddynet/agent/internal/models"
)

// listenTCP opens a throwaway TCP listener and returns its port plus a closer.
func listenTCP(t *testing.T) (int, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	return port, func() { _ = ln.Close() }
}

func newTestChecker() *Checker {
	c := NewChecker()
	c.Safe = nil // allow any port so tests can use ephemeral ports
	c.Timeout = time.Second
	c.Dialer = &net.Dialer{Timeout: time.Second}
	return c
}

func TestCheckOnline(t *testing.T) {
	port, closeFn := listenTCP(t)
	defer closeFn()

	d := models.Device{
		ID:           "d1",
		LANIP:        "127.0.0.1",
		AllowedPorts: []int{port},
	}
	res := newTestChecker().Check(context.Background(), d)
	if !res.Online {
		t.Fatalf("expected online, got %+v", res)
	}
	if len(res.OpenPorts) != 1 || res.OpenPorts[0] != port {
		t.Fatalf("expected open port %d, got %v", port, res.OpenPorts)
	}
	if res.LatencyMS < 0 {
		t.Fatalf("latency should be >= 0, got %d", res.LatencyMS)
	}
}

func TestCheckOfflinePort(t *testing.T) {
	port, closeFn := listenTCP(t)
	closeFn() // close immediately so nothing is listening

	d := models.Device{
		ID:           "d2",
		LANIP:        "127.0.0.1",
		AllowedPorts: []int{port},
	}
	res := newTestChecker().Check(context.Background(), d)
	if res.Online {
		t.Fatal("expected offline")
	}
	if len(res.FailedPorts) != 1 {
		t.Fatalf("expected 1 failed port, got %v", res.FailedPorts)
	}
	if res.ErrorMessage == "" {
		t.Fatal("expected an error message for offline device")
	}
}

func TestCheckNoHost(t *testing.T) {
	res := newTestChecker().Check(context.Background(), models.Device{ID: "d3", AllowedPorts: []int{80}})
	if res.Online || res.ErrorMessage == "" {
		t.Fatalf("expected error for hostless device, got %+v", res)
	}
}

func TestSafePortFiltering(t *testing.T) {
	c := NewChecker() // default SafePorts active
	// Port 12345 is not in SafePorts, so it must be filtered out entirely.
	d := models.Device{ID: "d4", LANIP: "127.0.0.1", AllowedPorts: []int{12345}}
	res := c.Check(context.Background(), d)
	if len(res.CheckedPorts) != 0 {
		t.Fatalf("non-safe port should be filtered, got checked=%v", res.CheckedPorts)
	}
	if !strings.Contains(res.ErrorMessage, "no allowlisted ports") {
		t.Fatalf("unexpected message: %q", res.ErrorMessage)
	}
}

func TestHTTPProbe(t *testing.T) {
	// A mock device serving HTTP on an ephemeral port. We point the checker at
	// it and confirm the TCP probe sees the port open. (The status-code probe
	// only triggers on well-known web ports, so we assert reachability here.)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ := strconv.Atoi(portStr)
	d := models.Device{ID: "d5", LANIP: host, AllowedPorts: []int{port}}
	res := newTestChecker().Check(context.Background(), d)
	if !res.Online {
		t.Fatalf("expected mock HTTP device online, got %+v", res)
	}
}
