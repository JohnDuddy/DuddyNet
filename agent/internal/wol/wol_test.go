package wol

import (
	"bytes"
	"testing"
)

func TestParseMACFormats(t *testing.T) {
	want := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for _, in := range []string{
		"AA:BB:CC:DD:EE:FF",
		"aa-bb-cc-dd-ee-ff",
		"aabbccddeeff",
		"AABB.CCDD.EEFF",
		" AA:BB:CC:DD:EE:FF ",
	} {
		got, err := ParseMAC(in)
		if err != nil {
			t.Fatalf("ParseMAC(%q): %v", in, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("ParseMAC(%q) = % X, want % X", in, got, want)
		}
	}
}

func TestParseMACRejectsInvalid(t *testing.T) {
	for _, in := range []string{"", "AA:BB:CC", "ZZ:BB:CC:DD:EE:FF", "AABBCCDDEE", "aabbccddeeffaa"} {
		if _, err := ParseMAC(in); err == nil {
			t.Fatalf("ParseMAC(%q) should fail", in)
		}
	}
}

func TestBuildMagicPacket(t *testing.T) {
	pkt, err := BuildMagicPacket("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("BuildMagicPacket: %v", err)
	}
	if len(pkt) != 102 {
		t.Fatalf("packet length = %d, want 102", len(pkt))
	}
	// First 6 bytes must be 0xFF.
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			t.Fatalf("byte %d = %#x, want 0xFF", i, pkt[i])
		}
	}
	// Then the MAC repeated 16 times.
	mac := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for rep := 0; rep < 16; rep++ {
		off := 6 + rep*6
		if !bytes.Equal(pkt[off:off+6], mac) {
			t.Fatalf("repetition %d mismatch: % X", rep, pkt[off:off+6])
		}
	}
}

func TestBroadcastForCIDR(t *testing.T) {
	cases := map[string]string{
		"192.168.1.0/24":  "192.168.1.255:9",
		"10.0.0.0/8":      "10.255.255.255:9",
		"192.168.1.64/26": "192.168.1.127:9",
	}
	for cidr, want := range cases {
		if got := BroadcastForCIDR(cidr, 9); got != want {
			t.Errorf("BroadcastForCIDR(%q) = %q, want %q", cidr, got, want)
		}
	}
	// Bad CIDR falls back to global broadcast.
	if got := BroadcastForCIDR("not-a-cidr", 7); got != "255.255.255.255:7" {
		t.Errorf("fallback broadcast = %q", got)
	}
}

// fakeSender captures sends without touching the network.
type fakeSender struct {
	mac, addr string
	called    bool
}

func (f *fakeSender) Send(mac, addr string) error {
	f.mac, f.addr, f.called = mac, addr, true
	return nil
}

func TestSenderInterface(t *testing.T) {
	var s Sender = &fakeSender{}
	if err := s.Send("AA:BB:CC:DD:EE:FF", "192.168.1.255:9"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	fs := s.(*fakeSender)
	if !fs.called || fs.mac == "" {
		t.Fatal("expected sender to capture the call")
	}
}
