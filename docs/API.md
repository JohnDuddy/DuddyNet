# DuddyNet agent API (v1)

Base URL: `http://<agent-host>:8765` where `<agent-host>` is the agent's
MagicDNS name or `100.x.y.z` tailnet IP. All paths are prefixed `/api/v1`.

- **Transport:** reachable only over Tailscale (see [SECURITY.md](./SECURITY.md)).
- **Auth:** `Authorization: Bearer <token>` on everything except `/health` and
  `/pair`. Tokens are scoped and expire.
- **Content type:** `application/json`. Errors use a stable envelope:
  ```json
  { "error": "forbidden", "message": "token lacks required scope: wol", "status": 403 }
  ```

## Scopes

| Scope | Grants |
| --- | --- |
| `read-only` | All `GET`s + health checks |
| `device-admin` | Create/update/delete devices & services |
| `wol` | Wake-on-LAN |
| `logs` | Read the audit log |
| `full-admin` | Everything, incl. token management (implies all) |

> Any valid token may read. Writes require `device-admin`. WOL requires `wol`.
> Logs require `logs`. `full-admin` implies all scopes.

## Endpoints

| Method | Path | Scope | Description |
| --- | --- | --- | --- |
| GET | `/health` | — (public) | Agent identity, uptime, Tailscale status, LAN subnet |
| POST | `/pair` | — (public, rate-limited) | Exchange a pairing code for a scoped token |
| GET | `/devices` | read | List devices (`?include_hidden=true`) |
| POST | `/devices` | device-admin | Create a device |
| GET | `/devices/{id}` | read | Get one device |
| PATCH | `/devices/{id}` | device-admin | Update device metadata |
| DELETE | `/devices/{id}` | device-admin | Delete device (cascades services) |
| POST | `/devices/{id}/check` | read | Run a health check |
| POST | `/devices/{id}/wake` | wol | Send a Wake-on-LAN magic packet |
| GET | `/services` | read | List services (`?device_id=`) |
| POST | `/services` | device-admin | Create a service |
| GET | `/services/{id}` | read | Get one service |
| PATCH | `/services/{id}` | device-admin | Update a service |
| DELETE | `/services/{id}` | device-admin | Delete a service |
| GET | `/logs` | logs | Audit log, paginated & filterable |
| GET | `/tokens` | full-admin | List issued tokens (no secrets) |
| POST | `/tokens/{id}/revoke` | full-admin | Revoke a token |

### `GET /health` (public)
```json
{
  "service": "duddynet-agent",
  "version": "1.0.0",
  "uptime_seconds": 4210,
  "hostname": "ugreen-nas",
  "lan_subnet": "192.168.1.0/24",
  "tailscale_status": "running",
  "tailscale_ip": "100.101.102.103",
  "server_time": "2026-06-07T12:00:00Z",
  "read_only_default": false
}
```

### `POST /pair` (public)
Request:
```json
{ "code": "K7QM-29FB-XTRP", "label": "Pixel 8" }
```
Response `201`:
```json
{
  "token": "ddn_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "token_id": "9f...",
  "scopes": ["read-only", "wol"],
  "expires_at": "2026-09-05T12:00:00Z"
}
```
The `token` is returned **once**. Store it in Android Keystore.

### `POST /devices/{id}/check` (read)
Response:
```json
{
  "device_id": "9f...",
  "online": true,
  "latency_ms": 7,
  "checked_ports": [80, 443],
  "open_ports": [443],
  "failed_ports": [80],
  "http_status": 200,
  "checked_at": "2026-06-07T12:01:00Z"
}
```

### `POST /devices/{id}/wake` (wol)
Response:
```json
{
  "device_id": "9f...",
  "success": true,
  "mac_address": "AA:BB:CC:DD:EE:FF",
  "sent_to": "192.168.1.255:9",
  "message": "magic packet sent",
  "sent_at": "2026-06-07T12:02:00Z"
}
```

### `GET /logs` (logs)
Query params: `device_id`, `action`, `success` (`true`/`false`), `since`,
`until` (RFC3339), `limit` (default 100), `offset`. Returns a `Page`:
```json
{ "items": [ /* AuditLogEntry */ ], "total": 42, "limit": 100, "offset": 0 }
```

See `agent/internal/models/models.go` for the authoritative struct definitions.
