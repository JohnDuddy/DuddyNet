//go:build tsnet

// This file is compiled only with: go build -tags tsnet
//
// It makes the agent join the tailnet directly as an application node using
// tsnet, so the agent gets its own tailnet identity and listens purely on the
// tailnet (no LAN-facing socket at all).
//
// Building this requires the tsnet dependency:
//
//	cd agent
//	go get tailscale.com/tsnet
//	go build -tags tsnet ./...
//
// Provide an auth key out-of-band via the TS_AUTHKEY environment variable on
// FIRST run only (tsnet persists node state in its state dir afterwards). The
// auth key is NEVER read from config files or committed — consistent with the
// project's "no auth keys in repo / app" rule.
package listen

import (
	"net"
	"os"
	"path/filepath"

	"tailscale.com/tsnet"

	"github.com/duddynet/agent/internal/config"
)

// Mode reports the active listener mode for logging.
const Mode = "tsnet"

// New starts a tsnet server and returns a listener on the configured port. The
// cleanup func shuts the tsnet server down.
func New(cfg *config.Config) (net.Listener, func(), error) {
	stateDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "tsnet")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, nil, err
	}
	hostname := cfg.TsnetHostname
	if hostname == "" {
		hostname = "duddynet-agent"
	}
	srv := &tsnet.Server{
		Hostname: hostname,
		Dir:      stateDir,
		AuthKey:  os.Getenv("TS_AUTHKEY"), // first-run only; empty afterwards
	}
	ln, err := srv.Listen("tcp", cfg.Addr())
	if err != nil {
		_ = srv.Close()
		return nil, nil, err
	}
	return ln, func() { _ = srv.Close() }, nil
}
