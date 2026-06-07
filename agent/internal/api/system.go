package api

import (
	"context"
	"os"

	"github.com/duddynet/agent/internal/tailscale"
)

// osHostname returns the machine hostname (empty on error).
func osHostname() (string, error) {
	return os.Hostname()
}

// detectTailscale returns a human-readable status string and tailnet IP for the
// /health response. It never blocks for long and never errors out the request.
func detectTailscale() (status, ip string) {
	st, ok := tailscale.Detect(context.Background())
	if !ok {
		return "not-detected", ""
	}
	if st.Running {
		return "running", st.Self
	}
	return "stopped", st.Self
}
