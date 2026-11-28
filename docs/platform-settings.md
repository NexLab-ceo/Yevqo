# Yevqo platform settings

The administrator panel exposes platform-wide settings at **Admin → Platform settings**.

## Public Control Plane URL
Set the canonical HTTPS origin (for example `https://app.yevqo.cc.cd`). The panel derives the Cloudflare OAuth callback as:

`<public-url>/oauth/cloudflare/callback`

The URL must be an origin only: no path, query string, or fragment.

## Cloudflare OAuth client
Create a self-managed OAuth client in Cloudflare Dashboard → Manage Account → OAuth clients. For a server-side SaaS integration, use Authorization Code, include the refresh-token grant, configure the callback URL shown by Yevqo, and choose the same confidential-client token authentication method that you select in Yevqo Admin → Platform Settings (client_secret_post or client_secret_basic). Make the client public only after completing Cloudflare's required publisher/domain verification. The Cloudflare Client ID and Client Secret can then be entered in Admin → Platform settings.

Yevqo encrypts the Client Secret at rest with its Control Plane credential key. The secret is write-only in the admin UI: it is never returned by the settings API. A blank secret field preserves the existing secret; the explicit clear checkbox removes it.

The OAuth authorization, state validation, code exchange, encrypted access/refresh token storage, token refresh, and zone selection are implemented. Live Cloudflare account verification is still required before treating the integration as production-validated.


## GitHub App

The administrator can configure the GitHub App from the same Platform Settings screen:

- **App ID** — numeric ID from GitHub Developer Settings.
- **App slug** — public installation slug used by the GitHub installation redirect.
- **Private key** — paste the PEM downloaded from the GitHub App settings. The Control Plane validates the PEM RSA key and encrypts it at rest with the credential key. The private key is write-only and is never returned by the API.

The existing Termux environment variables and protected PEM file remain a backward-compatible fallback. Values stored in the administrator panel take precedence when present. The GitHub App installation/config endpoints now read this shared platform configuration.


The OAuth client must allow these scopes: `account-settings.read`, `zone.read`, `dns.write`, `argotunnel.read`, `argotunnel.write`, and `offline_access`. The user chooses the zone after authorizing Yevqo. Access and refresh tokens are encrypted at rest; the Control Plane refreshes access tokens when they are close to expiry. Existing scoped API Token connections remain supported.
