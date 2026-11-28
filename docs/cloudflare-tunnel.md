# Cloudflare Tunnel on Yevqo

## Termux / Android

The Agent installs the native Termux package when the user selects **Install cloudflared**:

```bash
pkg update -y
pkg install cloudflared -y
cloudflared --version
```

Do not use the generic Linux release binary on Android/Termux. The Termux package is built for the Android userspace.

## Configure the Cloudflare OAuth client

The OAuth client is created once by the Yevqo platform administrator in Cloudflare (**Manage Account → OAuth clients**). It must be configured for a server-side Authorization Code flow:

- Response type: `code`.
- Grant types: `authorization_code` and `refresh_token`.
- Token endpoint authentication: choose `client_secret_post` or `client_secret_basic`, then select the same method in Yevqo → Admin → Platform Settings.
- Redirect URL: an exact match for `<YEVQO_PUBLIC_URL>/oauth/cloudflare/callback`. For the production app this is `https://app.yevqo.cc.cd/oauth/cloudflare/callback`.
- Required API scopes: `zone.read`, `dns.write`, `argotunnel.read`, and `argotunnel.write`. Configure `authorization_code` and `refresh_token` as grant types and `code` as the response type. Do **not** add `offline_access` manually to the requested `scope` parameter: Cloudflare automatically manages this protocol scope from the configured grant and response types.

Cloudflare's OAuth scope IDs correspond to API permission groups. The Tunnel scopes used by Yevqo are `argotunnel.read` and `argotunnel.write`; DNS and zone discovery use `dns.write` and `zone.read`. Cloudflare documents `offline_access` as a protocol scope that is automatically added or removed based on `grant_types` and `response_types`, so Yevqo intentionally omits it from the explicit authorization request. The client must include every scope requested by Yevqo. For a client intended for users outside the Cloudflare account that created it, Cloudflare requires public visibility and publisher-domain verification; a private OAuth client is limited to members of its owning account.

After creating the client, copy its Client ID and Client Secret into Yevqo → Admin → Platform Settings, save the public URL and matching token authentication method, then reconnect from Settings → Integrations → Cloudflare. The callback consumes a one-time state value and returns to the integration page; users then load and select an active DNS zone.

### If Cloudflare returns immediately

- `invalid_scope`: compare the client's configured API scopes with the four IDs above. Confirm `authorization_code` + `refresh_token` grants and `code` response type; do not add `offline_access` to the explicit API scope list.
- `unauthorized_client` or `invalid_client`: check the Client ID, Client Secret, grant/response types and token authentication method.
- A redirect URL error: compare the callback URL character-for-character, including scheme, hostname and path, with Yevqo's configured public URL.
- If the authorization succeeds but no refresh token is returned, ensure `refresh_token` is an enabled grant and `offline_access` is included as a required scope.

Yevqo deliberately does not treat a redirect as proof that Cloudflare is connected. The user must return with a valid callback, and Yevqo must be able to list zones before a zone can be selected. Live account authorization still needs to be tested against the actual OAuth client.

## Connect a site automatically

1. Recommended: in Yevqo → Site → Domains select **Подключить Cloudflare через OAuth**. Authorize Yevqo in Cloudflare, then choose the active zone. The OAuth Client is configured by the platform administrator in Admin → Platform Settings.
2. Alternative: create a scoped Cloudflare API Token with **Account: Cloudflare Tunnel — Edit**, **Zone: DNS — Edit**, and **Zone: Read**. Paste it in Site → Domains, choose **Загрузить мои домены**, and select the active zone.
3. Enter the desired hostname in **Primary domain**, then choose **Создать Tunnel и подключить приложение**.
4. Yevqo creates a remotely-managed Tunnel through the Cloudflare API, obtains its Tunnel token, configures the public hostname ingress and proxied DNS CNAME, then sends the token to the selected Agent over its authenticated command channel.
5. The Agent stores the token in a private file and starts `cloudflared tunnel run --token-file`; the token is not placed in the process command line.
6. Yevqo reports the connector active only after the Agent confirms its Cloudflare edge connection.

OAuth access and refresh tokens are encrypted at rest in the Control Plane; access tokens refresh automatically and are revoked when the user disconnects Cloudflare. The API Token fallback is also encrypted at rest. The generated Tunnel token is a separate credential, encrypted and delivered only to the owning Agent. Existing manual Tunnel-token setup remains available for tunnels created outside Yevqo.


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

OAuth or an API token authorizes Cloudflare configuration. OAuth access and refresh tokens are encrypted and refreshed by the Control Plane. The Tunnel token is separate: the API token authorizes Cloudflare configuration, while the Tunnel token authenticates the Agent connector. The Tunnel token is never sent to the route API. Existing unrelated ingress rules are preserved. Only the matching Yevqo-managed hostname and CNAME are removed when you remove a route. Workspace members must ask the workspace owner to configure Cloudflare.
