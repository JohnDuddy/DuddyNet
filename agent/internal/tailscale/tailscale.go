// Package tailscale provides best-effort detection of the local Tailscale state.
//
// The agent does NOT control Tailscale and never handles auth keys. It merely
// reports whether a tailscaled appears to be running and, if so, this node's
// tailnet IP — purely for the /health response and diagnostics. Detection works
// by invoking the `tailscale` CLI if present; absence is reported as "unknown",
// never as an error.
package tailscale

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// Status is a minimal view of `tailscale status --json`.
type Status struct {
	Running bool
	Self    string // this node's first tailnet IP (100.x.y.z)
	DNSName string // MagicDNS name, if available
}

// tsStatusJSON mirrors the subset of fields we read.
type tsStatusJSON struct {
	BackendState string `json:"BackendState"`
	Self         struct {
		TailscaleIPs []string `json:"TailscaleIPs"`
		DNSName      string   `json:"DNSName"`
	} `json:"Self"`
}

// Detect returns the current Tailscale status. The bool is false (with empty
// Status) when the CLI is unavailable or the command fails.
func Detect(ctx context.Context) (Status, bool) {
	path, err := exec.LookPath("tailscale")
	if err != nil {
		return Status{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, path, "status", "--json").Output()
	if err != nil {
		return Status{}, false
	}
	var raw tsStatusJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return Status{}, false
	}
	st := Status{
		Running: strings.EqualFold(raw.BackendState, "Running"),
		DNSName: strings.TrimSuffix(raw.Self.DNSName, "."),
	}
	if len(raw.Self.TailscaleIPs) > 0 {
		st.Self = raw.Self.TailscaleIPs[0]
	}
	return st, true
}
