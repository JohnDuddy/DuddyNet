// Package config loads and validates the DuddyNet agent configuration.
//
// Precedence (highest wins):
//  1. Explicit env overrides (DUDDYNET_*).
//  2. Values in the YAML config file.
//  3. Built-in defaults.
package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config mirrors config.yaml. See agent/config.example.yaml for documentation.
type Config struct {
	ListenAddress      string   `yaml:"listen_address"`
	ListenPort         int      `yaml:"listen_port"`
	LANSubnet          string   `yaml:"lan_subnet"`
	AllowedOrigins     []string `yaml:"allowed_origins"`
	DatabasePath       string   `yaml:"database_path"`
	LogPath            string   `yaml:"log_path"`
	PairingEnabled     bool     `yaml:"pairing_enabled"`
	TokenExpirationDays int     `yaml:"token_expiration_days"`
	ReadOnlyDefault    bool     `yaml:"read_only_default"`
	AllowedIPRanges    []string `yaml:"allowed_ip_ranges"`

	// TsnetMode, when true, makes the agent join the tailnet directly as an
	// application node via tsnet instead of binding a local socket. Only honored
	// in builds compiled with the "tsnet" build tag (see tsnet_enabled.go).
	TsnetMode     bool   `yaml:"tsnet_mode"`
	TsnetHostname string `yaml:"tsnet_hostname"`

	// path is the file this config was loaded from (for diagnostics).
	path string `yaml:"-"`
}

// Default returns a Config populated with safe defaults.
func Default() *Config {
	return &Config{
		ListenAddress:       "0.0.0.0", // bound to the tailnet interface in practice; see docs/SECURITY.md
		ListenPort:          8765,
		LANSubnet:           "192.168.1.0/24",
		AllowedOrigins:      []string{}, // empty = same-origin only (no browser CORS); app uses native client
		DatabasePath:        defaultDataPath("duddynet.json"),
		LogPath:             defaultDataPath("audit.log"),
		PairingEnabled:      true,
		TokenExpirationDays: 90,
		ReadOnlyDefault:     false,
		// Allow the Tailscale CGNAT range (how clients normally connect) plus the
		// default LAN. Tailscale ACLs remain the primary boundary; this is a
		// defense-in-depth source-IP backstop.
		AllowedIPRanges:     []string{"100.64.0.0/10", "192.168.1.0/24"},
		TsnetMode:           false,
		TsnetHostname:       "duddynet-agent",
	}
}

// DefaultConfigPath returns the OS-appropriate config file location.
func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "DuddyNet", "config.yaml")
	}
	return "/etc/duddynet/config.yaml"
}

// defaultDataPath returns an OS-appropriate writable data path.
func defaultDataPath(name string) string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "DuddyNet", name)
	}
	return filepath.Join("/var/lib/duddynet", name)
}

// Load reads config from path (or the OS default if empty), applies env
// overrides, and validates. A missing file is NOT an error when path is the
// default — defaults are used so `serve` can run before `init`. A missing file
// at an explicitly-requested path IS an error.
func Load(path string) (*Config, error) {
	cfg := Default()

	explicit := path != ""
	if envPath := os.Getenv("DUDDYNET_CONFIG"); envPath != "" && !explicit {
		path, explicit = envPath, true
	}
	if path == "" {
		path = DefaultConfigPath()
	}
	cfg.path = path

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	case errors.Is(err, os.ErrNotExist) && !explicit:
		// Fine: run with defaults.
	default:
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg.applyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("DUDDYNET_LISTEN_ADDRESS"); v != "" {
		c.ListenAddress = v
	}
	if v := os.Getenv("DUDDYNET_LISTEN_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.ListenPort = p
		}
	}
	if v := os.Getenv("DUDDYNET_DATABASE_PATH"); v != "" {
		c.DatabasePath = v
	}
}

// Validate checks invariants and normalizes a few fields.
func (c *Config) Validate() error {
	if c.ListenPort <= 0 || c.ListenPort > 65535 {
		return fmt.Errorf("listen_port %d out of range", c.ListenPort)
	}
	if c.LANSubnet != "" {
		if _, _, err := net.ParseCIDR(c.LANSubnet); err != nil {
			return fmt.Errorf("lan_subnet %q is not valid CIDR: %w", c.LANSubnet, err)
		}
	}
	for _, r := range c.AllowedIPRanges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(r); err != nil {
			// Allow bare IPs too.
			if net.ParseIP(r) == nil {
				return fmt.Errorf("allowed_ip_ranges entry %q is not a valid CIDR or IP", r)
			}
		}
	}
	if c.TokenExpirationDays < 0 {
		return fmt.Errorf("token_expiration_days cannot be negative")
	}
	if c.DatabasePath == "" {
		return errors.New("database_path must be set")
	}
	return nil
}

// Path returns the file this config was loaded from.
func (c *Config) Path() string { return c.path }

// Addr returns the host:port the HTTP server should bind.
func (c *Config) Addr() string {
	return net.JoinHostPort(c.ListenAddress, strconv.Itoa(c.ListenPort))
}

// IPAllowed reports whether remoteIP falls within any configured allowed range.
// An empty AllowedIPRanges means "allow any" (Tailscale ACLs remain the primary
// boundary); callers decide whether to enforce. A nil/unparseable IP is denied.
func (c *Config) IPAllowed(remoteIP string) bool {
	if len(c.AllowedIPRanges) == 0 {
		return true
	}
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}
	for _, r := range c.AllowedIPRanges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, cidr, err := net.ParseCIDR(r); err == nil {
			if cidr.Contains(ip) {
				return true
			}
			continue
		}
		if bare := net.ParseIP(r); bare != nil && bare.Equal(ip) {
			return true
		}
	}
	return false
}

// Marshal renders the config back to YAML (used by `init`).
func (c *Config) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}
