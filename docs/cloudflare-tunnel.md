# Cloudflare Tunnel on VELTRIX Dock

## Termux / Android

The Agent installs the native Termux package when the user selects **Install cloudflared**:

```bash
pkg update -y
pkg install cloudflared -y
cloudflared --version
```

Do not use the generic Linux release binary on Android/Termux. The Termux package is built for the Android userspace.

## Connect a site

1. Create a remotely-managed Tunnel in Cloudflare Zero Trust → Networks → Tunnels.
2. Add a Public Hostname route for the domain, pointing to the local site origin, for example `http://127.0.0.1:3000`.
3. Copy the Tunnel token into VELTRIX Domains and save the hostname.
4. VELTRIX stores the token encrypted in the Control Plane database and sends it to the owning Agent over the authenticated command channel.
5. The Agent writes the token to a private `0600` file and starts `cloudflared tunnel run --token-file`. The token is never placed in the process command line.
6. The Agent waits for Cloudflare to report a registered edge connection before the Control Plane changes the connector status to active.

## Status semantics

**Tunnel connected** means the connector registered a connection to Cloudflare Edge. It does not prove that the Public Hostname route or application origin is healthy. The hostname and origin route must exist in Cloudflare Zero Trust. HTTP-origin health checks are a separate capability and are not implied by the connector status.

Tunnel logs and token files are stored under the Agent data directory with private file permissions. Removing a tunnel stops its process and removes its local token file.
