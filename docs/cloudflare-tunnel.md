# Cloudflare Tunnel on Yevqo

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
2. Copy its Tunnel token and Tunnel ID.
3. In Yevqo → Site → Domains, save the hostname and Tunnel token. Yevqo stores the token encrypted and sends it to the owning Agent over the authenticated command channel.
4. The Agent writes the token to a private `0600` file and starts `cloudflared tunnel run --token-file`. The token is never placed in the process command line.
5. Connect a Cloudflare API token and Zone ID in the Domains tab, then enter the Tunnel ID and choose **Configure public route**. Yevqo adds the ingress rule and matching proxied CNAME.
6. The Agent waits for Cloudflare to report a registered edge connection before the Control Plane changes the connector status to active.

## Status semantics

**Tunnel connected** means the connector registered a connection to Cloudflare Edge. It does not prove that the Public Hostname route or application origin is healthy. Yevqo can create the hostname ingress route and CNAME when the Cloudflare API integration is configured. HTTP-origin health checks are a separate capability and are not implied by the connector status.

Stopping a website process does not stop its tunnel connector; the connector remains registered while the origin may be unavailable. Removing a domain or deleting the site sends an explicit stop command. Tunnel logs and token files are stored under the Agent data directory with private file permissions; stopping the tunnel removes its local token file.


## Automatic public hostname routing

1. Connect a Cloudflare API token and Zone ID in the site's Domains tab.
2. The API token needs **Account: Cloudflare Tunnel — Edit** (API permission `Cloudflare Tunnel Write`), **Zone: DNS — Edit**, and **Zone — Read**.
3. Save the site's Cloudflare Tunnel token so the Agent starts `cloudflared`.
4. Enter the Tunnel ID and select **Configure public route**. Yevqo updates the remotely managed ingress configuration to `http://127.0.0.1:<site-port>` and creates the matching proxied CNAME.
5. Remove the managed route before changing the hostname or disconnecting the Cloudflare API integration.

Existing unrelated ingress rules are preserved. Only the matching Yevqo-managed hostname and CNAME are removed when you remove a route. Workspace members must ask the workspace owner to configure Cloudflare.
