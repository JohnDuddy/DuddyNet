// Package wol builds and sends Wake-on-LAN "magic packets".
//
// A magic packet is 6 bytes of 0xFF followed by the target's 6-byte MAC
// repeated 16 times (102 bytes total), sent as a UDP broadcast on the LAN.
// Because it must originate inside the broadcast domain, the HOME AGENT sends
// it — the Android app only asks the agent to do so.
package wol

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
)

// ErrInvalidMAC indicates the MAC address could not be parsed.
var ErrInvalidMAC = errors.New("invalid MAC address")

// ParseMAC normalizes and validates a MAC address in common formats
// ("AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff", "aabbccddeeff"). It returns the
// 6 raw bytes.
func ParseMAC(mac string) ([]byte, error) {
	cleaned := strings.NewReplacer(":", "", "-", "", ".", "", " ", "").Replace(strings.TrimSpace(mac))
	if len(cleaned) != 12 {
		return nil, fmt.Errorf("%w: expected 12 hex digits, got %d", ErrInvalidMAC, len(cleaned))
	}
	b, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMAC, err)
	}
	return b, nil
}

// BuildMagicPacket returns the 102-byte magic packet for the given MAC.
func BuildMagicPacket(mac string) ([]byte, error) {
	hw, err := ParseMAC(mac)
	if err != nil {
		return nil, err
	}
	packet := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		packet = append(packet, 0xFF)
	}
	for i := 0; i < 16; i++ {
		packet = append(packet, hw...)
	}
	return packet, nil
}

// Sender sends magic packets. It is an interface so tests can capture packets
// without touching the network.
type Sender interface {
	Send(mac, broadcastAddr string) error
}

// UDPSender sends the packet via UDP broadcast. The default port is 9
// (discard); 7 is also common. broadcastAddr should be the LAN broadcast
// address, e.g. "192.168.1.255:9".
type UDPSender struct{}

// Send transmits a magic packet for mac to broadcastAddr (host:port).
func (UDPSender) Send(mac, broadcastAddr string) error {
	packet, err := BuildMagicPacket(mac)
	if err != nil {
		return err
	}
	if _, _, err := net.SplitHostPort(broadcastAddr); err != nil {
		return fmt.Errorf("wol: invalid broadcast address %q: %w", broadcastAddr, err)
	}
	conn, err := net.Dial("udp", broadcastAddr)
	if err != nil {
		return fmt.Errorf("wol: dial %s: %w", broadcastAddr, err)
	}
	defer conn.Close()
	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("wol: write: %w", err)
	}
	return nil
}

// BroadcastForCIDR derives the directed-broadcast "host:port" for a subnet CIDR
// (e.g. "192.168.1.0/24" -> "192.168.1.255:9"). Falls back to global broadcast
// if the CIDR can't be parsed.
func BroadcastForCIDR(cidr string, port int) string {
	if port == 0 {
		port = 9
	}
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil || ipnet.IP.To4() == nil {
		return fmt.Sprintf("255.255.255.255:%d", port)
	}
	ip := ipnet.IP.To4()
	mask := ipnet.Mask
	bcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		bcast[i] = ip[i] | ^mask[i]
	}
	return fmt.Sprintf("%s:%d", bcast.String(), port)
}
