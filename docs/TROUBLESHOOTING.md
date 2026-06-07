# Troubleshooting

Work top-to-bottom; each layer depends on the one above it.

## 1. Tailscale not connected (phone)
**Symptom:** app says "Tailscale unreachable".
- Open the Tailscale app; ensure it shows **Connected** with a `100.x.y.z` IP.
- Android may have killed it in the background → disable battery optimization for
  Tailscale.
- On mobile data, confirm the VPN key icon is present in the status bar.

## 2. Agent host not resolving
**Symptom:** name like `duddynet-agent.tailXXXX.ts.net` fails but the
`100.x.y.z` IP works.
- Enable **MagicDNS** in the admin console (DNS tab).
- Re-toggle the phone's Tailscale connection to pick up DNS settings.
- As a workaround, enter the `100.x.y.z` IP in the app's onboarding/settings.

## 3. Agent unreachable on its port
**Symptom:** `GET /api/v1/health` times out.
- On the home server: `sudo systemctl status duddynet-agent` (Linux) or check
  the Windows service. Look for "starting" in the logs.
- Confirm the port: `curl http://localhost:8765/api/v1/health` **on the server**.
- Check the **tailnet ACL** allows your phone → `tag:duddynet-agent:8765`.
- Check `allowed_ip_ranges` in `config.yaml` includes the tailnet
  (`100.64.0.0/10`). A too-narrow list returns `403 forbidden`.
- If you firewalled the port, confirm the rule allows `100.64.0.0/10`.

## 4. 401 Unauthorized
- The token expired (default 90 days) or was revoked → re-pair in the app.
- The token wasn't saved → re-run onboarding; ensure biometric/keystore unlock
  succeeded.

## 5. 403 Forbidden
- **"token lacks required scope"** → the token wasn't granted that capability.
  Re-pair with the needed scopes, e.g. `--scopes read-only,wol`.
- **"agent is in read-only mode"** → `read_only_default: true` in config blocks
  all writes. Set it to `false` and restart, or use only read/WOL features.
- **"source address not permitted"** → your source IP isn't in
  `allowed_ip_ranges`.

## 6. Health check says offline but the device is up
- The device's `allowed_ports` may not include a port it actually listens on,
  **or** the port isn't in the agent's safe set
  (`22,80,443,445,3389,5000,5001,8006,8123,9000,32400`).
- If the device is LAN-only (no Tailscale), confirm the **subnet route** is
  advertised **and approved**, and the ACL lets the agent reach that host/port.
- Try `duddynet-agent check <ip>` directly on the server to isolate the issue.

## 7. Wake-on-LAN doesn't wake the PC
- WOL must be enabled in the **NIC driver** ("Wake on Magic Packet") **and** the
  **BIOS/UEFI** ("Power On by PCI-E / WOL").
- The PC must be in a sleep/hibernate state that supports WOL (some "Fast
  Startup" shutdowns on Windows disable it — disable Fast Startup).
- The agent and PC must share the broadcast domain (same VLAN/subnet). The packet
  is sent to the directed broadcast of `lan_subnet`.
- Confirm the device's `mac_address` is correct.

## 8. Pairing code rejected
- Codes are single-use and expire (default 15 min). Generate a fresh one:
  `duddynet-agent pair --label "Pixel 8" --scopes read-only,wol`.
- Too many attempts → you hit the per-IP rate limit; wait a minute.

## Collecting diagnostics
- App: **Settings → Export diagnostics** produces a redacted report (no tokens).
- Agent: `journalctl -u duddynet-agent -n 200` (Linux) or the NSSM stdout/stderr
  logs (Windows). Logs never contain token values.
