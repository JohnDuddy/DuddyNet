// Package models defines the shared API contracts (the "wire" types) used by the
// DuddyNet home agent and consumed by the Android app. These types are the
// single source of truth for the JSON shapes documented in /docs/API.md.
//
// Design notes:
//   - All timestamps are RFC3339 (UTC) strings on the wire via time.Time's
//     default JSON marshalling, which keeps the contract language-agnostic and
//     trivially parseable on Android (kotlinx-datetime / Instant.parse).
//   - IDs are opaque strings (UUID-like). Callers must not assume a format.
//   - We never serialize secrets. Token *values* exist only transiently in
//     PairingResponse; the persisted AppToken stores a hash only (see the
//     token package), so no plaintext token is ever returned by a list/GET.
package models

import "time"

// DeviceType is a coarse classification used for icons/grouping in the UI.
type DeviceType string

const (
	DeviceTypeRouter   DeviceType = "router"
	DeviceTypeNAS      DeviceType = "nas"
	DeviceTypePC       DeviceType = "pc"
	DeviceTypeServer   DeviceType = "server"
	DeviceTypePi       DeviceType = "pi"
	DeviceTypeCamera   DeviceType = "camera"
	DeviceTypePrinter  DeviceType = "printer"
	DeviceTypePhone    DeviceType = "phone"
	DeviceTypeOther    DeviceType = "other"
)

// Device is a configured machine on the home LAN (or tailnet).
type Device struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         DeviceType `json:"type"`
	LANIP        string     `json:"lan_ip"`
	TailnetName  string     `json:"tailnet_name,omitempty"`
	MACAddress   string     `json:"mac_address,omitempty"`
	AllowedPorts []int      `json:"allowed_ports"`
	Notes        string     `json:"notes,omitempty"`
	Critical     bool       `json:"critical"`
	Hidden       bool       `json:"hidden"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
}

// DeviceInput is the mutable subset of a Device accepted on POST/PATCH.
// Pointer fields on PATCH mean "only change if provided".
type DeviceInput struct {
	Name         *string     `json:"name,omitempty"`
	Type         *DeviceType `json:"type,omitempty"`
	LANIP        *string     `json:"lan_ip,omitempty"`
	TailnetName  *string     `json:"tailnet_name,omitempty"`
	MACAddress   *string     `json:"mac_address,omitempty"`
	AllowedPorts *[]int      `json:"allowed_ports,omitempty"`
	Notes        *string     `json:"notes,omitempty"`
	Critical     *bool       `json:"critical,omitempty"`
	Hidden       *bool       `json:"hidden,omitempty"`
}

// ServiceType maps a service to a known UI affordance (e.g. "open web admin").
type ServiceType string

const (
	ServiceTypeRouterAdmin   ServiceType = "router_admin"
	ServiceTypeNASAdmin      ServiceType = "nas_admin"
	ServiceTypeSMB           ServiceType = "smb"
	ServiceTypePlex          ServiceType = "plex"
	ServiceTypeJellyfin      ServiceType = "jellyfin"
	ServiceTypeHomeAssistant ServiceType = "home_assistant"
	ServiceTypePrinter       ServiceType = "printer"
	ServiceTypeRDP           ServiceType = "rdp"
	ServiceTypeSSH           ServiceType = "ssh"
	ServiceTypeWeb           ServiceType = "web"
	ServiceTypeOther         ServiceType = "other"
)

// Service is an addressable service exposed by a Device.
type Service struct {
	ID          string      `json:"id"`
	DeviceID    string      `json:"device_id"`
	Name        string      `json:"name"`
	Protocol    string      `json:"protocol"` // tcp, udp, http, https, ssh, rdp, smb
	Port        int         `json:"port"`
	Path        string      `json:"path,omitempty"`
	URL         string      `json:"url,omitempty"`
	ServiceType ServiceType `json:"service_type"`
	ReadOnly    bool        `json:"read_only"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ServiceInput is the mutable subset of a Service accepted on POST/PATCH.
type ServiceInput struct {
	DeviceID    *string      `json:"device_id,omitempty"`
	Name        *string      `json:"name,omitempty"`
	Protocol    *string      `json:"protocol,omitempty"`
	Port        *int         `json:"port,omitempty"`
	Path        *string      `json:"path,omitempty"`
	URL         *string      `json:"url,omitempty"`
	ServiceType *ServiceType `json:"service_type,omitempty"`
	ReadOnly    *bool        `json:"read_only,omitempty"`
	Enabled     *bool        `json:"enabled,omitempty"`
}

// HealthStatus is the structured result of a device health check.
type HealthStatus struct {
	DeviceID     string    `json:"device_id"`
	Online       bool      `json:"online"`
	LatencyMS    int64     `json:"latency_ms"`
	CheckedPorts []int     `json:"checked_ports"`
	OpenPorts    []int     `json:"open_ports"`
	FailedPorts  []int     `json:"failed_ports"`
	HTTPStatus   int       `json:"http_status,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CheckedAt    time.Time `json:"checked_at"`
}

// WakeResult is returned by the Wake-on-LAN endpoint.
type WakeResult struct {
	DeviceID   string    `json:"device_id"`
	Success    bool      `json:"success"`
	MACAddress string    `json:"mac_address"`
	SentTo     string    `json:"sent_to"` // broadcast address used
	Message    string    `json:"message,omitempty"`
	SentAt     time.Time `json:"sent_at"`
}

// TokenScope is a coarse capability granted to an app token.
type TokenScope string

const (
	ScopeReadOnly    TokenScope = "read-only"
	ScopeDeviceAdmin TokenScope = "device-admin"
	ScopeWOL         TokenScope = "wol"
	ScopeLogs        TokenScope = "logs"
	ScopeFullAdmin   TokenScope = "full-admin"
)

// AppToken is the persisted record of a paired client. The token *value* is
// never stored or returned — only its hash (see the token package). This struct
// is what GET /tokens / list-tokens returns.
type AppToken struct {
	ID         string       `json:"id"`
	Label      string       `json:"label"`
	Scopes     []TokenScope `json:"scopes"`
	CreatedAt  time.Time    `json:"created_at"`
	ExpiresAt  time.Time    `json:"expires_at"`
	LastUsedAt *time.Time   `json:"last_used_at,omitempty"`
	Revoked    bool         `json:"revoked"`
	// TokenHash and Salt are persisted but never serialized to clients.
	TokenHash string `json:"-"`
	Salt      string `json:"-"`
}

// PairingRequest is submitted by the Android app to exchange a one-time pairing
// code for a scoped token.
type PairingRequest struct {
	Code  string `json:"code"`
	Label string `json:"label"` // human-friendly device label, e.g. "Pixel 8"
}

// PairingResponse carries the freshly minted token value. This is the ONLY time
// the plaintext token is transmitted; the app must store it in Android Keystore.
type PairingResponse struct {
	Token     string       `json:"token"`
	TokenID   string       `json:"token_id"`
	Scopes    []TokenScope `json:"scopes"`
	ExpiresAt time.Time    `json:"expires_at"`
}

// AuditLogEntry is one recorded action. Sensitive material (token values, auth
// headers, passwords) is NEVER written here.
type AuditLogEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	TokenID    string    `json:"token_id,omitempty"`
	AppLabel   string    `json:"app_label,omitempty"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type,omitempty"` // device | service | token | system
	TargetID   string    `json:"target_id,omitempty"`
	Success    bool      `json:"success"`
	RemoteIP   string    `json:"remote_ip,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}

// HealthInfo is returned by GET /health (the agent's own liveness/identity).
type HealthInfo struct {
	Service         string    `json:"service"`
	Version         string    `json:"version"`
	UptimeSeconds   int64     `json:"uptime_seconds"`
	Hostname        string    `json:"hostname"`
	LANSubnet       string    `json:"lan_subnet"`
	TailscaleStatus string    `json:"tailscale_status"` // "running" | "unknown" | "not-detected"
	TailscaleIP     string    `json:"tailscale_ip,omitempty"`
	ServerTime      time.Time `json:"server_time"`
	ReadOnlyDefault bool      `json:"read_only_default"`
}

// Page wraps a paginated list response.
type Page[T any] struct {
	Items      []T `json:"items"`
	Total      int `json:"total"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
	NextOffset int `json:"next_offset,omitempty"`
}

// APIError is the standard error envelope. Every non-2xx response uses this.
type APIError struct {
	Error   string `json:"error"`             // stable machine code, e.g. "unauthorized"
	Message string `json:"message"`           // human-readable detail
	Status  int    `json:"status"`            // HTTP status, echoed for convenience
	TraceID string `json:"trace_id,omitempty"`
}
