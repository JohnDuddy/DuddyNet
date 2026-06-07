# Tailscale setup for DuddyNet

DuddyNet uses Tailscale as its encrypted transport. **No router ports are
forwarded.** Your phone reaches the home agent over the tailnet; the agent
reaches LAN-only devices via an (optional) subnet route. This guide covers every
piece.

> Background: Tailscale builds a private WireGuard mesh between your devices.
> Each device gets a stable `100.x.y.z` address (the CGNAT range `100.64.0.0/10`)
> and an optional MagicDNS name like `duddynet-agent.tailXXXX.ts.net`.

---

## 1. Install Tailscale on Android

1. Install **Tailscale** from the Google Play Store.
2. Open it and **sign in with the same account** that owns your tailnet.
3. Toggle the VPN **on**. Accept the Android VPN permission prompt.
4. Confirm you see a `100.x.y.z` address in the app.

DuddyNet does **not** control this VPN. The DuddyNet app simply detects whether
it can reach the agent; you keep Tailscale connected yourself. (This avoids
relying on undocumented intents and keeps DuddyNet within Android's rules.)

## 2. Install Tailscale on the home server

The home server is the always-on machine that runs `duddynet-agent` (your UGREEN
NAS, a mini PC, or a Raspberry Pi).

**Linux:**
```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
```

**Windows:** download the installer from <https://tailscale.com/download>, run
it, and sign in.

Verify:
```bash
tailscale status      # should list this machine + your phone
tailscale ip -4       # this machine's 100.x.y.z
```

## 3. (Optional) Advertise a subnet route

Run this **only if** you want to reach LAN devices that can't run Tailscale
themselves (router admin page, NAS SMB, cameras, printers). The agent's host
becomes a "subnet router" for your home LAN.

```bash
# On the home server. Match the CIDR to your LAN (and to lan_subnet in config).
sudo tailscale up --advertise-routes=192.168.1.0/24
```

On Linux you also need IP forwarding enabled (Tailscale's installer usually does
this; if not):
```bash
echo 'net.ipv4.ip_forward = 1' | sudo tee /etc/sysctl.d/99-tailscale.conf
echo 'net.ipv6.conf.all.forwarding = 1' | sudo tee -a /etc/sysctl.d/99-tailscale.conf
sudo sysctl -p /etc/sysctl.d/99-tailscale.conf
```

### Approve the route in the admin console

1. Go to <https://login.tailscale.com/admin/machines>.
2. Find the home server, open the **⋯** menu → **Edit route settings**.
3. **Approve** `192.168.1.0/24`.

To approve automatically on every restart, use the `autoApprovers` block in
[`tailscale-policy.example.hujson`](./tailscale-policy.example.hujson).

> Even with a subnet route, the **DuddyNet agent still only exposes the devices
> and ports you configure**, and Tailscale ACLs further restrict what each device
> may reach. Three layers, see [SECURITY.md](./SECURITY.md).

## 4. (Optional) Exit node

Not required for DuddyNet. If you want all your phone's internet traffic to exit
via home:
```bash
sudo tailscale up --advertise-exit-node
```
Approve it in the console, then select it on the phone. DuddyNet does not depend
on this.

## 5. MagicDNS

Enable **MagicDNS** in the admin console (DNS tab) so you can use names like
`duddynet-agent.tailXXXX.ts.net` instead of raw `100.x.y.z` addresses. The app
accepts either form on the onboarding screen.

## 6. Apply the access policy (ACLs / grants)

Open the **Access Controls** page and merge in
[`tailscale-policy.example.hujson`](./tailscale-policy.example.hujson). It:

- lets **your phone reach only the agent**, only on its port;
- lets the **agent reach only the specific LAN devices/ports** you list;
- gives **admins** broader maintenance access;
- **auto-approves** the subnet route for the agent tag.

Tag the agent machine so the policy applies:
```bash
sudo tailscale up --advertise-routes=192.168.1.0/24 --advertise-tags=tag:duddynet-agent
```
(Approve the tag in the console if prompted.)

## 7. Test from cellular data

1. Turn **Wi-Fi off** on the phone; stay on mobile data.
2. Ensure the Tailscale app shows **Connected**.
3. Open a browser to `http://duddynet-agent.tailXXXX.ts.net:8765/api/v1/health`
   (or the `100.x.y.z` form). You should get JSON.
4. Open the DuddyNet app — the dashboard should show the agent reachable.

If step 3 fails, see [TROUBLESHOOTING.md](./TROUBLESHOOTING.md).

## Why no port forwarding?

See [SECURITY.md](./SECURITY.md#why-no-port-forwarding). In short: forwarding
RDP/SMB/NAS-admin/camera ports exposes them to the entire internet and to
automated scanners. Tailscale gives you an encrypted, authenticated private path
instead, with per-device access control — and **Funnel is intentionally not used**
because this is private remote access, not public sharing.
