# Cloudflare Tunnel on Yevqo

## Termux / Android

The Agent installs the native Termux package when the user selects **Install cloudflared**:

```bash
pkg update -y
pkg install cloudflared -y
cloudflared --version
```

Do not use the generic Linux release binary on Android/Termux. The Termux package is built for the Android userspace.

## Connect a site automatically

1. Create a scoped Cloudflare API Token with **Account: Cloudflare Tunnel — Edit**, **Zone: DNS — Edit**, and **Zone: Read**.
2. In Yevqo → Site → Domains, paste the API Token and choose **Загрузить мои домены**. Select the active zone and connect it.
3. Enter the desired hostname in **Primary domain**, then choose **Создать Tunnel и подключить приложение**.
4. Yevqo creates a remotely-managed Tunnel through the Cloudflare API, obtains its Tunnel token, configures the public hostname ingress and proxied DNS CNAME, then sends the token to the selected Agent over its authenticated command channel.
5. The Agent stores the token in a private file and starts `cloudflared tunnel run --token-file`; the token is not placed in the process command line.
6. Yevqo reports the connector active only after the Agent confirms its Cloudflare edge connection.

The API Token is used only by the Control Plane for Cloudflare API calls. The generated Tunnel token is encrypted at rest and delivered only to the owning Agent. OAuth is not part of this flow; users explicitly provide a scoped API Token. Existing manual Tunnel-token setup remains available for tunnels created outside Yevqo.


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

1. Connect a scoped Cloudflare API token and select a zone in the site's Domains tab. Yevqo can create the Tunnel automatically, or you can connect an existing Tunnel.
2. The API token needs **Account: Cloudflare Tunnel — Edit** (API permission `Cloudflare Tunnel Write`), **Zone: DNS — Edit**, and **Zone — Read**.
3. For an automatically created Tunnel, Yevqo obtains its token from the Cloudflare API, stores it encrypted and sends it only to the owning Agent, which starts `cloudflared` and waits for an edge connection.
4. Yevqo configures remotely-managed ingress to `http://127.0.0.1:<site-port>` and creates the matching proxied CNAME. Existing manually created tunnels can still be connected by supplying their token and using **Configure public route**.
5. A successful route API response confirms that ingress and DNS configuration were accepted; it does not prove that the public HTTP origin is healthy. Verify the hostname from an external network.
6. Remove the managed route before changing the hostname or disconnecting the Cloudflare API integration. Deleting a site or unpairing its device also removes its managed routes first; the operation is blocked if Cloudflare cleanup fails.

The API token and Tunnel token serve different purposes: the API token authorizes Cloudflare configuration, while the Tunnel token authenticates the Agent connector. The Tunnel token is never sent to the route API. Existing unrelated ingress rules are preserved. Only the matching Yevqo-managed hostname and CNAME are removed when you remove a route. Workspace members must ask the workspace owner to configure Cloudflare.
