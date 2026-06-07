# Running duddynet-agent as a Windows service

The agent is a single static binary. The cleanest way to run it as a Windows
service is the built-in **Service Control Manager** via `sc.exe`, or
[NSSM](https://nssm.cc/) for nicer log handling. Both are shown below. Run an
**elevated** PowerShell (Administrator).

## 0. Build / place the binary

```powershell
# From the repo, build for Windows:
cd agent
go build -o duddynet-agent.exe ./cmd/duddynet-agent

# Place it somewhere stable:
New-Item -ItemType Directory -Force "C:\Program Files\DuddyNet" | Out-Null
Copy-Item .\duddynet-agent.exe "C:\Program Files\DuddyNet\"
```

## 1. Initialize config

```powershell
& "C:\Program Files\DuddyNet\duddynet-agent.exe" init
# Writes %ProgramData%\DuddyNet\config.yaml and creates the data dir.
notepad "$env:ProgramData\DuddyNet\config.yaml"   # review lan_subnet etc.
```

## 2a. Install with sc.exe (built-in)

```powershell
sc.exe create DuddyNetAgent `
  binPath= "\"C:\Program Files\DuddyNet\duddynet-agent.exe\" serve --config \"$env:ProgramData\DuddyNet\config.yaml\"" `
  start= auto `
  DisplayName= "DuddyNet Agent"

sc.exe description DuddyNetAgent "DuddyNet home agent (Tailscale-only remote access)"
sc.exe start DuddyNetAgent
```

> Note the **spaces after `=`** in `sc.exe` — they are required.

To remove:

```powershell
sc.exe stop DuddyNetAgent
sc.exe delete DuddyNetAgent
```

## 2b. Install with NSSM (recommended for logging)

```powershell
# After installing nssm (e.g. via: winget install NSSM.NSSM)
nssm install DuddyNetAgent "C:\Program Files\DuddyNet\duddynet-agent.exe" `
  serve --config "$env:ProgramData\DuddyNet\config.yaml"
nssm set DuddyNetAgent AppStdout "$env:ProgramData\DuddyNet\agent.out.log"
nssm set DuddyNetAgent AppStderr "$env:ProgramData\DuddyNet\agent.err.log"
nssm set DuddyNetAgent Start SERVICE_AUTO_START
nssm start DuddyNetAgent
```

## 3. Firewall (defense in depth)

Tailscale is the primary boundary, but you can also scope the listen port to the
tailnet at the Windows Firewall. Example allowing only the Tailscale CGNAT range:

```powershell
New-NetFirewallRule -DisplayName "DuddyNet Agent (Tailscale only)" `
  -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8765 `
  -RemoteAddress 100.64.0.0/10
```

## 4. Pair the Android app

```powershell
& "C:\Program Files\DuddyNet\duddynet-agent.exe" pair --label "Pixel 8" --scopes read-only,wol
```

Enter the printed code in the app's onboarding screen.

## Wake-on-LAN note

For WOL to wake this or other PCs, enable **"Allow this device to wake the
computer"** / **Wake-on-LAN / Magic Packet** in the target PC's NIC driver
settings and BIOS/UEFI. The agent only *sends* the packet; the target must be
configured to listen for it.
