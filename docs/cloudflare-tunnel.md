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


## Existing locally-managed Tunnel (Termux config.yml)

A locally-managed Tunnel is different from the remotely-managed flow above. Its ingress rules live in the Termux cloudflared configuration file, not in Cloudflare's remotely-managed Tunnel configuration API. A route shown in the Dashboard therefore does not, by itself, prove that a public DNS record exists.

Keep the existing config.yml and add the hostname rule before its final catch-all rule. For a site listening on port 8085, the relevant ingress entry is:

```yaml
- hostname: <hostname>
  service: http://127.0.0.1:8085
```

Keep the final catch-all entry (normally `- service: http_status:404`) last. Do not replace unrelated ingress rules.

### Create the DNS record

The DNS CNAME is separate from the local ingress rule. For a locally-managed Tunnel, create it with the Cloudflare CLI:

```bash
cloudflared tunnel route dns <TUNNEL-NAME-OR-UUID> <hostname>
```

This creates a CNAME pointing to `<TUNNEL-UUID>.cfargotunnel.com`; it does not edit config.yml or start the Tunnel. The command requires the account certificate (`cert.pem`) created by `cloudflared tunnel login`. If that certificate is not available on the Termux device, create the equivalent CNAME in Cloudflare DNS manually:

- Type: `CNAME`
- Name: the hostname (use `@` for the zone apex when configuring the root domain)
- Target: `<TUNNEL-UUID>.cfargotunnel.com`
- Proxy status: Proxied

Do not overwrite an existing DNS record unless you have confirmed it belongs to this Tunnel.

### Yevqo Domains integration

When Yevqo detects a locally-managed Tunnel, **Configure public route** creates and tracks only the matching DNS CNAME. It deliberately does not attempt to edit the device's local ingress file. The Domains tab labels this as a DNS-only setup and shows the expected hostname-to-origin mapping. Confirm that the local config.yml contains the same hostname and origin port.

For a remotely-managed Tunnel, Yevqo continues to configure both the ingress rule and the proxied CNAME through the Cloudflare API.

### Verify each layer separately

1. Confirm the local origin responds: `curl -I http://127.0.0.1:<port>`.
2. Confirm DNS resolves: `nslookup <hostname>` (or query a public resolver such as `1.1.1.1` / `8.8.8.8`).
3. Confirm the Tunnel connector is running and connected: `cloudflared tunnel info <TUNNEL-NAME-OR-UUID>`.
4. Test the public hostname: `curl -I --max-time 15 https://<hostname>`.

A successful origin response proves only the local service is available. A successful DNS lookup proves only that the hostname resolves. A running Tunnel does not prove that the local ingress points to the correct origin.

## Status semantics

**Tunnel connected** means the connector registered a connection to Cloudflare Edge. It does not prove that the Public Hostname route or application origin is healthy. Yevqo can create the hostname ingress route and CNAME when the Cloudflare API integration is configured. HTTP-origin health checks are a separate capability and are not implied by the connector status.

Stopping a website process does not stop its tunnel connector; the connector remains registered while the origin may be unavailable. Removing a domain or deleting the site sends an explicit stop command. Tunnel logs and token files are stored under the Agent data directory with private file permissions; stopping the tunnel removes its local token file.


## Automatic public hostname routing

1. Connect a Cloudflare API token and Zone ID in the site's Domains tab before saving the site's Tunnel token if you want the route to be configured automatically.
2. The API token needs **Account: Cloudflare Tunnel — Edit** (API permission `Cloudflare Tunnel Write`), **Zone: DNS — Edit**, and **Zone — Read**.
3. Save the site's hostname and Cloudflare Tunnel token. Yevqo stores the Tunnel token encrypted and sends it only to the owning Agent, which starts `cloudflared` and waits for an edge connection.
4. When the Cloudflare API integration is already connected, the UI extracts the Tunnel UUID from the supplied token and automatically asks the existing route API to configure the remotely managed ingress to `http://127.0.0.1:<site-port>` and create the matching proxied CNAME. If the API integration was connected later, use **Configure public route** with the Tunnel ID shown in Cloudflare.
5. A successful route API response confirms that ingress and DNS configuration were accepted; it does not prove that the public HTTP origin is healthy. Verify the hostname from an external network.
6. Remove the managed route before changing the hostname or disconnecting the Cloudflare API integration. Deleting a site or unpairing its device also removes its managed routes first; the operation is blocked if Cloudflare cleanup fails.

The API token and Tunnel token serve different purposes: the API token authorizes Cloudflare configuration, while the Tunnel token authenticates the Agent connector. The Tunnel token is never sent to the route API. Existing unrelated ingress rules are preserved. Only the matching Yevqo-managed hostname and CNAME are removed when you remove a route. Workspace members must ask the workspace owner to configure Cloudflare.
