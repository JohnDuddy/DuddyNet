package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	c := Default()
	c.ListenPort = 0
	if err := c.Validate(); err == nil {
		t.Error("expected error for port 0")
	}
	c = Default()
	c.LANSubnet = "not-a-cidr"
	if err := c.Validate(); err == nil {
		t.Error("expected error for bad subnet")
	}
	c = Default()
	c.AllowedIPRanges = []string{"banana"}
	if err := c.Validate(); err == nil {
		t.Error("expected error for bad allowed range")
	}
}

func TestIPAllowed(t *testing.T) {
	c := Default()
	c.AllowedIPRanges = []string{"100.64.0.0/10", "192.168.1.50"}
	cases := map[string]bool{
		"100.100.5.5":  true,  // in tailnet CGNAT range
		"192.168.1.50": true,  // exact bare IP
		"192.168.1.51": false, // not listed
		"8.8.8.8":      false,
		"":             false,
	}
	for ip, want := range cases {
		if got := c.IPAllowed(ip); got != want {
			t.Errorf("IPAllowed(%q) = %v, want %v", ip, got, want)
		}
	}

	// Empty list = allow all.
	c.AllowedIPRanges = nil
	if !c.IPAllowed("8.8.8.8") {
		t.Error("empty allowlist should allow any IP")
	}
}

func TestLoadMissingExplicitFails(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("explicit missing config should error")
	}
}

func TestLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	src := Default()
	src.LANSubnet = "10.0.0.0/24"
	src.ListenPort = 9999
	src.DatabasePath = filepath.Join(dir, "db.json")
	data, err := src.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.LANSubnet != "10.0.0.0/24" || got.ListenPort != 9999 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
