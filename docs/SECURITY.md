# DuddyNet security model

DuddyNet is built around **defense in depth**. No single control is trusted on
its own.

## The three boundaries

```
Phone ──(1) Tailscale identity/ACL──▶ Agent ──(2) App token + scopes──▶ curated
                                              ──(3) Allowlisted devices/ports──▶ LAN
```

1. **Tailscale (network identity).** The agent listens only on the tailnet. To
   reach it at all, a device must be a member of your tailnet, and the tailnet
   ACL must permit it to talk to the agent's port. A stolen phone with the app
   but no Tailscale session reaches nothing.

2. **App token (application identity).** On top of Tailscale, every request to
   the agent (except `/health` and `/pair`) must carry a `Bearer` token created
   by pairing. Tokens are **scoped** (`read-only`, `device-admin`, `wol`,
   `logs`, `full-admin`), **expire**, and can be **revoked** instantly.

3. **Allowlist (resource authorization).** The agent will only health-check,
   wake, or surface the **devices and ports you configured**. It refuses to
   connect to arbitrary IPs/ports — even a tampered device record is clamped to
   a safe port set, and `allowed_ip_ranges` adds a source-IP backstop.

## What is *not* exposed

By design DuddyNet never:

- forwards router ports or uses public DDNS;
- exposes RDP, SMB, NAS admin, or camera admin pages to the internet;
- uses Tailscale **Funnel** (that would publish to the public internet);
- builds an in-app browser, SMB, RDP, or SSH client (the MVP hands you a URL or
  copyable instructions and lets the OS/native apps handle the protocol over the
  already-private tailnet).

## Token handling

| Concern | How it's handled |
| --- | --- |
| Token at rest on agent | Stored as `SHA-256(salt ‖ token)` + random per-token salt. Plaintext is shown **once** at pairing and never persisted. |
| Token lookup | A separate unsalted lookup hash indexes the token; it leaks nothing without the 256-bit secret. |
| Token on the phone | Stored in **Android Keystore**-backed `EncryptedSharedPreferences`; never in plaintext, never in logs. |
| Pairing codes | Short, single-use, time-boxed (default 15 min), and **rate-limited** per source IP. Low entropy is acceptable because of those three properties. |
| Failed auth | Logged to the audit trail (without the presented token value). |
| Compromised phone | Run `duddynet-agent revoke-token <id>` (or `POST /tokens/{id}/revoke`). The app loses access on its next request. Also remove the device from the tailnet. |

## Secrets policy

- **No** Tailscale auth keys, OAuth secrets, or admin API tokens exist anywhere
  in this repository or in the Android app. Any Tailscale API/auth interaction
  happens only on the home machine's Tailscale client / the admin console.
- The Android app stores **no** NAS/router/Windows passwords. It opens admin
  pages in the browser, where you authenticate directly with the device.
- `.gitignore` excludes `config.yaml`, `*.token`, `*.key`, `.env`, and data
  files. Only `.env.example` and `config.example.yaml` are committed.

## Logging hygiene

The audit log and access log record **method, path, status, action, target,
success, token id, and remote IP** — never Authorization headers, token values,
request bodies, or device passwords.

## Why no port forwarding?

Forwarding a port on your router publishes that service to the **entire
internet**. Within minutes, automated scanners (Shodan, botnets) will find it.
RDP, SMB, and NAS/camera admin panels are routinely brute-forced and exploited
this way; it is one of the most common ransomware entry points for home and
small-business networks.

Tailscale replaces this with an **encrypted, authenticated, private** path:
nothing is publicly listening, every peer is cryptographically identified, and
the ACL decides who may talk to what. DuddyNet then adds its own token layer and
a device/port allowlist on top. The result is remote access without an internet-
facing attack surface.

## Hardening checklist

- [ ] Apply the example tailnet ACL so the phone can reach **only** the agent.
- [ ] Tag the agent (`tag:duddynet-agent`) and scope its LAN reach in the ACL.
- [ ] Bind the agent to the Tailscale interface, or firewall its port to
      `100.64.0.0/10`.
- [ ] Issue the phone a **least-privilege** token (e.g. `read-only,wol`), not
      `full-admin`.
- [ ] Enable **biometric unlock** in the app for travel.
- [ ] Keep `read_only_default: true` if you only want monitoring + WOL.
- [ ] Know the revoke procedure before you travel.
