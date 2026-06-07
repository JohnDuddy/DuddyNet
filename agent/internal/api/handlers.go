package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/duddynet/agent/internal/models"
	"github.com/duddynet/agent/internal/store"
	"github.com/duddynet/agent/internal/token"
	"github.com/duddynet/agent/internal/wol"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// genID returns a random opaque id.
func genID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// decodeJSON strictly decodes a size-limited request body into v.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is empty")
		}
		return err
	}
	return nil
}

// ---- Health ----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	hostname, _ := osHostname()
	tsStatus, tsIP := detectTailscale()
	info := models.HealthInfo{
		Service:         "duddynet-agent",
		Version:         s.version,
		UptimeSeconds:   int64(time.Since(s.startTime).Seconds()),
		Hostname:        hostname,
		LANSubnet:       s.cfg.LANSubnet,
		TailscaleStatus: tsStatus,
		TailscaleIP:     tsIP,
		ServerTime:      time.Now().UTC(),
		ReadOnlyDefault: s.cfg.ReadOnlyDefault,
	}
	writeJSON(w, http.StatusOK, info)
}

// ---- Pairing ----

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.cfg.PairingEnabled {
		errForbidden(w, "pairing is disabled")
		return
	}
	if !s.pairLimiter.Allow(ip) {
		s.audit.Record(auditEvent{Action: "pair.rate_limited", TargetType: "system", Success: false, RemoteIP: ip})
		errTooManyRequests(w, "too many pairing attempts; try again later")
		return
	}

	var req models.PairingRequest
	if err := decodeJSON(w, r, &req); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		errBadRequest(w, "pairing code is required")
		return
	}

	pc, err := s.store.ConsumePairingCode(token.NormalizeCode(req.Code), time.Now())
	if err != nil {
		s.audit.Record(auditEvent{Action: "pair.failed", TargetType: "system", Success: false, RemoteIP: ip, Detail: "invalid or expired code"})
		errUnauthorized(w, "invalid or expired pairing code")
		return
	}

	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = pc.Label
	}
	if label == "" {
		label = "paired-client"
	}

	resp, err := s.mintToken(label, pc.Scopes)
	if err != nil {
		errInternal(w, "could not create token")
		return
	}

	s.audit.Record(auditEvent{
		TokenID: resp.TokenID, AppLabel: label, Action: "pair.success",
		TargetType: "token", TargetID: resp.TokenID, Success: true, RemoteIP: ip,
	})
	writeJSON(w, http.StatusCreated, resp)
}

// mintToken creates and persists a new token, returning the one-time plaintext.
func (s *Server) mintToken(label string, scopes []models.TokenScope) (models.PairingResponse, error) {
	value, err := token.Generate()
	if err != nil {
		return models.PairingResponse{}, err
	}
	salt, err := token.NewSalt()
	if err != nil {
		return models.PairingResponse{}, err
	}
	if len(scopes) == 0 {
		scopes = []models.TokenScope{models.ScopeReadOnly}
	}
	now := time.Now().UTC()
	exp := now.AddDate(0, 0, s.cfg.TokenExpirationDays)
	rec := store.TokenRecord{
		ID:         genID(),
		Label:      label,
		Scopes:     scopes,
		CreatedAt:  now,
		ExpiresAt:  exp,
		TokenHash:  token.Hash(value, salt),
		Salt:       salt,
		LookupHash: token.HashForLookup(value),
	}
	if err := s.store.CreateToken(rec); err != nil {
		return models.PairingResponse{}, err
	}
	return models.PairingResponse{
		Token:     value,
		TokenID:   rec.ID,
		Scopes:    scopes,
		ExpiresAt: exp,
	}, nil
}

// ---- Devices ----

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.store.ListDevices()
	if err != nil {
		errInternal(w, "")
		return
	}
	includeHidden := r.URL.Query().Get("include_hidden") == "true"
	out := make([]models.Device, 0, len(devices))
	for _, d := range devices {
		if d.Hidden && !includeHidden {
			continue
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, models.Page[models.Device]{Items: out, Total: len(out), Limit: len(out)})
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.GetDevice(r.PathValue("id"))
	if err != nil {
		errNotFound(w, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	if s.writeBlockedByReadOnly(w) {
		return
	}
	var in models.DeviceInput
	if err := decodeJSON(w, r, &in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	now := time.Now().UTC()
	d := models.Device{
		ID:           genID(),
		Type:         models.DeviceTypeOther,
		AllowedPorts: []int{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := applyDeviceInput(&d, in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	if strings.TrimSpace(d.Name) == "" {
		errBadRequest(w, "name is required")
		return
	}
	if err := s.store.CreateDevice(d); err != nil {
		errInternal(w, "could not create device")
		return
	}
	s.recordWrite(r, "device.create", "device", d.ID, true, d.Name)
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	if s.writeBlockedByReadOnly(w) {
		return
	}
	d, err := s.store.GetDevice(r.PathValue("id"))
	if err != nil {
		errNotFound(w, "device not found")
		return
	}
	var in models.DeviceInput
	if err := decodeJSON(w, r, &in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	if err := applyDeviceInput(&d, in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	d.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateDevice(d); err != nil {
		errInternal(w, "could not update device")
		return
	}
	s.recordWrite(r, "device.update", "device", d.ID, true, d.Name)
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	if s.writeBlockedByReadOnly(w) {
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteDevice(id); err != nil {
		errNotFound(w, "device not found")
		return
	}
	s.recordWrite(r, "device.delete", "device", id, true, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---- Device health check ----

func (s *Server) handleCheckDevice(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.GetDevice(r.PathValue("id"))
	if err != nil {
		errNotFound(w, "device not found")
		return
	}
	result := s.checker.Check(r.Context(), d)

	// Update last_seen_at on success (best-effort).
	if result.Online {
		now := time.Now().UTC()
		d.LastSeenAt = &now
		_ = s.store.UpdateDevice(d)
	}
	s.recordWrite(r, "device.check", "device", d.ID, result.Online, "")
	writeJSON(w, http.StatusOK, result)
}

// ---- Wake-on-LAN ----

func (s *Server) handleWakeDevice(w http.ResponseWriter, r *http.Request) {
	d, err := s.store.GetDevice(r.PathValue("id"))
	if err != nil {
		errNotFound(w, "device not found")
		return
	}
	if strings.TrimSpace(d.MACAddress) == "" {
		s.recordWrite(r, "device.wake", "device", d.ID, false, "no MAC address")
		errBadRequest(w, "device has no MAC address")
		return
	}
	if _, err := wol.ParseMAC(d.MACAddress); err != nil {
		s.recordWrite(r, "device.wake", "device", d.ID, false, "invalid MAC")
		errBadRequest(w, "device has an invalid MAC address")
		return
	}

	broadcast := wol.BroadcastForCIDR(s.cfg.LANSubnet, 9)
	result := models.WakeResult{
		DeviceID:   d.ID,
		MACAddress: d.MACAddress,
		SentTo:     broadcast,
		SentAt:     time.Now().UTC(),
	}
	if err := s.wol.Send(d.MACAddress, broadcast); err != nil {
		result.Success = false
		result.Message = err.Error()
		s.recordWrite(r, "device.wake", "device", d.ID, false, err.Error())
		writeJSON(w, http.StatusOK, result)
		return
	}
	result.Success = true
	result.Message = "magic packet sent"
	s.recordWrite(r, "device.wake", "device", d.ID, true, broadcast)
	writeJSON(w, http.StatusOK, result)
}

// ---- Services ----

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	svcs, err := s.store.ListServices()
	if err != nil {
		errInternal(w, "")
		return
	}
	if deviceID := r.URL.Query().Get("device_id"); deviceID != "" {
		filtered := svcs[:0]
		for _, svc := range svcs {
			if svc.DeviceID == deviceID {
				filtered = append(filtered, svc)
			}
		}
		svcs = filtered
	}
	writeJSON(w, http.StatusOK, models.Page[models.Service]{Items: svcs, Total: len(svcs), Limit: len(svcs)})
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	svc, err := s.store.GetService(r.PathValue("id"))
	if err != nil {
		errNotFound(w, "service not found")
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleCreateService(w http.ResponseWriter, r *http.Request) {
	if s.writeBlockedByReadOnly(w) {
		return
	}
	var in models.ServiceInput
	if err := decodeJSON(w, r, &in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	now := time.Now().UTC()
	svc := models.Service{
		ID:          genID(),
		ServiceType: models.ServiceTypeOther,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := applyServiceInput(&svc, in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	if svc.DeviceID == "" {
		errBadRequest(w, "device_id is required")
		return
	}
	if _, err := s.store.GetDevice(svc.DeviceID); err != nil {
		errBadRequest(w, "device_id does not refer to a known device")
		return
	}
	if strings.TrimSpace(svc.Name) == "" {
		errBadRequest(w, "name is required")
		return
	}
	if err := s.store.CreateService(svc); err != nil {
		errInternal(w, "could not create service")
		return
	}
	s.recordWrite(r, "service.create", "service", svc.ID, true, svc.Name)
	writeJSON(w, http.StatusCreated, svc)
}

func (s *Server) handleUpdateService(w http.ResponseWriter, r *http.Request) {
	if s.writeBlockedByReadOnly(w) {
		return
	}
	svc, err := s.store.GetService(r.PathValue("id"))
	if err != nil {
		errNotFound(w, "service not found")
		return
	}
	var in models.ServiceInput
	if err := decodeJSON(w, r, &in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	if err := applyServiceInput(&svc, in); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	svc.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateService(svc); err != nil {
		errInternal(w, "could not update service")
		return
	}
	s.recordWrite(r, "service.update", "service", svc.ID, true, svc.Name)
	writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	if s.writeBlockedByReadOnly(w) {
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteService(id); err != nil {
		errNotFound(w, "service not found")
		return
	}
	s.recordWrite(r, "service.delete", "service", id, true, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---- Logs ----

func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AuditFilter{
		DeviceID: q.Get("device_id"),
		Action:   q.Get("action"),
		Limit:    atoiDefault(q.Get("limit"), 100),
		Offset:   atoiDefault(q.Get("offset"), 0),
	}
	if v := q.Get("success"); v != "" {
		b := v == "true"
		f.Success = &b
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = t
		}
	}
	if v := q.Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Until = t
		}
	}
	items, total, err := s.store.ListAudit(f)
	if err != nil {
		errInternal(w, "")
		return
	}
	next := f.Offset + len(items)
	if next >= total {
		next = 0
	}
	writeJSON(w, http.StatusOK, models.Page[models.AuditLogEntry]{
		Items: items, Total: total, Limit: f.Limit, Offset: f.Offset, NextOffset: next,
	})
}

// ---- Tokens ----

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.store.ListTokens()
	if err != nil {
		errInternal(w, "")
		return
	}
	writeJSON(w, http.StatusOK, models.Page[models.AppToken]{Items: tokens, Total: len(tokens), Limit: len(tokens)})
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.RevokeToken(id); err != nil {
		errNotFound(w, "token not found")
		return
	}
	s.recordWrite(r, "token.revoke", "token", id, true, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----

// recordWrite audits a mutating action using the authenticated identity.
func (s *Server) recordWrite(r *http.Request, action, targetType, targetID string, success bool, detail string) {
	id := identityFrom(r.Context())
	s.audit.Record(auditFromReq(r, id, action, targetType, targetID, success, detail))
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// applyDeviceInput validates and applies a DeviceInput onto d.
func applyDeviceInput(d *models.Device, in models.DeviceInput) error {
	if in.Name != nil {
		d.Name = strings.TrimSpace(*in.Name)
	}
	if in.Type != nil {
		d.Type = *in.Type
	}
	if in.LANIP != nil {
		v := strings.TrimSpace(*in.LANIP)
		if v != "" && net.ParseIP(v) == nil {
			return errors.New("lan_ip is not a valid IP address")
		}
		d.LANIP = v
	}
	if in.TailnetName != nil {
		d.TailnetName = strings.TrimSpace(*in.TailnetName)
	}
	if in.MACAddress != nil {
		v := strings.TrimSpace(*in.MACAddress)
		if v != "" {
			if _, err := wol.ParseMAC(v); err != nil {
				return errors.New("mac_address is not valid")
			}
		}
		d.MACAddress = v
	}
	if in.AllowedPorts != nil {
		for _, p := range *in.AllowedPorts {
			if p <= 0 || p > 65535 {
				return errors.New("allowed_ports contains an out-of-range port")
			}
		}
		d.AllowedPorts = *in.AllowedPorts
	}
	if in.Notes != nil {
		d.Notes = *in.Notes
	}
	if in.Critical != nil {
		d.Critical = *in.Critical
	}
	if in.Hidden != nil {
		d.Hidden = *in.Hidden
	}
	return nil
}

// applyServiceInput validates and applies a ServiceInput onto svc.
func applyServiceInput(svc *models.Service, in models.ServiceInput) error {
	if in.DeviceID != nil {
		svc.DeviceID = strings.TrimSpace(*in.DeviceID)
	}
	if in.Name != nil {
		svc.Name = strings.TrimSpace(*in.Name)
	}
	if in.Protocol != nil {
		svc.Protocol = strings.ToLower(strings.TrimSpace(*in.Protocol))
	}
	if in.Port != nil {
		if *in.Port < 0 || *in.Port > 65535 {
			return errors.New("port is out of range")
		}
		svc.Port = *in.Port
	}
	if in.Path != nil {
		svc.Path = *in.Path
	}
	if in.URL != nil {
		svc.URL = strings.TrimSpace(*in.URL)
	}
	if in.ServiceType != nil {
		svc.ServiceType = *in.ServiceType
	}
	if in.ReadOnly != nil {
		svc.ReadOnly = *in.ReadOnly
	}
	if in.Enabled != nil {
		svc.Enabled = *in.Enabled
	}
	return nil
}
