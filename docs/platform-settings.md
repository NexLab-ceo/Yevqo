# Yevqo platform settings

The administrator panel exposes platform-wide settings at **Admin → Platform settings**.

## Public Control Plane URL
Set the canonical HTTPS origin (for example `https://app.yevqo.cc.cd`). The panel derives the Cloudflare OAuth callback as:

`<public-url>/oauth/cloudflare/callback`

The URL must be an origin only: no path, query string, or fragment.

## Cloudflare OAuth client
Create a self-managed OAuth client in Cloudflare Dashboard → Manage Account → OAuth clients. For a server-side SaaS integration, use Authorization Code, include the refresh-token grant, configure the callback URL shown by Yevqo, and use a confidential-client token authentication method. Make the client public only after completing Cloudflare's required publisher/domain verification. The Cloudflare Client ID and Client Secret can then be entered in Admin → Platform settings.

Yevqo encrypts the Client Secret at rest with its Control Plane credential key. The secret is write-only in the admin UI: it is never returned by the settings API. A blank secret field preserves the existing secret; the explicit clear checkbox removes it.

**Important:** this settings storage is only the configuration prerequisite. It does not itself implement Cloudflare OAuth authorization, callback handling, code exchange, token refresh/revocation, or user-level Cloudflare account linking. Those remain separate implementation work and must be completed before the user-facing “Connect Cloudflare” OAuth button is enabled.
