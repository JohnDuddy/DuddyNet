// Package health performs conservative reachability checks against configured
// devices: TCP connects to allowlisted ports and an optional HTTP status probe.
//
// Safety: the checker only ever connects to ports that are BOTH listed on the
// device (allowed_ports) AND within the agent's safe port set. This prevents a
// tampered device record from coercing the agent into connecting to arbitrary
// ports on arbitrary hosts.
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/duddynet/agent/internal/models"
)

// SafePorts is the default set of ports the checker is permitted to probe.
var SafePorts = map[int]bool{
	22:   true, // SSH
	80:   true, // HTTP
	443:  true, // HTTPS
	445:  true, // SMB (connect-only detection)
	3389: true, // RDP (connect-only detection)
	5000: true, // NAS web (e.g. Synology/DSM, UGREEN)
	5001: true, // NAS web TLS
	8006: true, // Proxmox
	8123: true, // Home Assistant
	9000: true, // Portainer / misc
	32400: true, // Plex
}

// Dialer abstracts net.Dialer for testing.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Checker runs health checks.
type Checker struct {
	Timeout    time.Duration
	Dialer     Dialer
	HTTPClient *http.Client
	Safe       map[int]bool
}

// NewChecker returns a Checker with sensible defaults.
func NewChecker() *Checker {
	return &Checker{
		Timeout: 2 * time.Second,
		Dialer:  &net.Dialer{Timeout: 2 * time.Second},
		HTTPClient: &http.Client{
			Timeout: 3 * time.Second,
			// Don't follow redirects: we only want the first status code.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Safe: SafePorts,
	}
}

// portsToCheck intersects the device's allowed ports with the safe set.
func (c *Checker) portsToCheck(d models.Device) []int {
	var out []int
	for _, p := range d.AllowedPorts {
		if p <= 0 || p > 65535 {
			continue
		}
		if c.Safe == nil || c.Safe[p] {
			out = append(out, p)
		}
	}
	sort.Ints(out)
	return out
}

// Check probes a single device and returns a structured result.
func (c *Checker) Check(ctx context.Context, d models.Device) models.HealthStatus {
	res := models.HealthStatus{
		DeviceID:  d.ID,
		CheckedAt: time.Now().UTC(),
	}
	host := d.LANIP
	if host == "" {
		host = d.TailnetName
	}
	if host == "" {
		res.ErrorMessage = "device has no LAN IP or tailnet name"
		return res
	}

	ports := c.portsToCheck(d)
	res.CheckedPorts = ports
	if len(ports) == 0 {
		res.ErrorMessage = "no allowlisted ports to check"
		return res
	}

	var bestLatency time.Duration = -1
	for _, p := range ports {
		addr := net.JoinHostPort(host, strconv.Itoa(p))
		start := time.Now()
		conn, err := c.Dialer.DialContext(ctx, "tcp", addr)
		elapsed := time.Since(start)
		if err != nil {
			res.FailedPorts = append(res.FailedPorts, p)
			continue
		}
		_ = conn.Close()
		res.OpenPorts = append(res.OpenPorts, p)
		if bestLatency < 0 || elapsed < bestLatency {
			bestLatency = elapsed
		}
	}

	res.Online = len(res.OpenPorts) > 0
	if bestLatency >= 0 {
		res.LatencyMS = bestLatency.Milliseconds()
	}

	// Optional HTTP status probe on a web-ish open port.
	if status := c.httpProbe(ctx, host, res.OpenPorts); status > 0 {
		res.HTTPStatus = status
	}

	if !res.Online {
		res.ErrorMessage = "no allowlisted ports open"
	}
	return res
}

func (c *Checker) httpProbe(ctx context.Context, host string, open []int) int {
	var scheme string
	var port int
	switch {
	case contains(open, 443):
		scheme, port = "https", 443
	case contains(open, 5001):
		scheme, port = "https", 5001
	case contains(open, 80):
		scheme, port = "http", 80
	case contains(open, 5000):
		scheme, port = "http", 5000
	case contains(open, 8123):
		scheme, port = "http", 8123
	default:
		return 0
	}
	url := fmt.Sprintf("%s://%s:%d/", scheme, host, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
