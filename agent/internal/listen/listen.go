//go:build !tsnet

// Package listen abstracts how the agent obtains its network listener.
//
// Default build: a plain TCP listener bound to the configured address. On the
// home machine this address should be the Tailscale interface (or 0.0.0.0 with
// the OS firewall restricting the port to the tailnet) — see docs/SECURITY.md.
//
// The `tsnet` build (go build -tags tsnet) swaps this for a listener that joins
// the tailnet directly as an application node; see listen_tsnet.go.
package listen

import (
	"net"

	"github.com/duddynet/agent/internal/config"
)

// Mode reports the active listener mode for logging.
const Mode = "tcp"

// New returns a TCP listener for cfg.Addr(). The returned cleanup func is a
// no-op in this build.
func New(cfg *config.Config) (net.Listener, func(), error) {
	ln, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		return nil, nil, err
	}
	return ln, func() {}, nil
}
