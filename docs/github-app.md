# GitHub App integration

Yevqo uses a GitHub App installation as the preferred GitHub connection. The browser never asks for or stores the App private key.

## Register the GitHub App

Create the App in GitHub Developer Settings and configure:

- **Homepage URL:** your Yevqo Control Panel origin.
- **Setup URL:** `https://YOUR-PANEL-HOST/settings/integrations/github`.
- **Repository permissions:** Metadata — Read-only; Contents — Read-only; Pull requests — Read-only; Checks — Read-only; Commit statuses — Read-only where available.
- **Administration — Read and write:** optional, required only if Yevqo should create/update/delete repository push webhooks automatically. Without this permission, Yevqo falls back to showing a one-time webhook URL and secret for manual setup.
- **Subscribe to events:** no App-level event subscription is required for the per-repository push webhook flow.

The installation flow uses GitHub's official `/apps/{slug}/installations/new` page. The callback returns `installation_id`, `setup_action` and the one-time `state` created by the authenticated Control Plane session. The state expires after 10 minutes and is consumed once. The Control Plane verifies that the installation belongs to the configured App before showing it as connected.

## Configure the Control Plane on Android / Termux

The Termux installer creates a protected configuration directory at
`$PREFIX/etc/yevqo` and never overwrites existing GitHub App credentials:

- `github-app.env` — App ID, slug and optional public URL (mode `0600`).
- `github-app.pem` — RSA private key in PEM format (mode `0600`).

Edit `$PREFIX/etc/yevqo/github-app.env` and set:

```sh
YEVQO_GITHUB_APP_ID=YOUR_NUMERIC_APP_ID
YEVQO_GITHUB_APP_SLUG=your-app-slug
YEVQO_PUBLIC_URL=https://your-public-panel-host
```

Place the downloaded GitHub App private key at
`$PREFIX/etc/yevqo/github-app.pem`. Do not paste the PEM into the env file,
browser, shell command arguments or chat. Restrict access to both files.

The `yevqo-control` runit service loads the env file and reads the PEM file
into the Control Plane process environment on every start. The key is not
printed or returned to the browser. Restart the service after editing either
file:

```sh
sv restart yevqo-control
sv status yevqo-control
```

For non-Termux deployments, set the same server-side environment variables
directly on the Control Plane service. The public config endpoint returns only
whether the App is configured and its public slug; it never returns the App ID
or private key.

## Disconnect and permissions

Disconnecting removes the user's encrypted installation record. It does not uninstall the App from GitHub; use GitHub's **Installed GitHub Apps** settings to revoke the installation itself.

Use the narrow permissions required by the features you enable. Metadata and Contents read are needed for repository and branch access; Pull requests read and Checks read power the PR/CI panels. Administration write is optional and should be granted only when automatic repository webhook management is desired.

## Current boundaries

- Repository listing is backed by the verified installation token.
- Installation linking uses a short-lived, single-use state tied to the signed-in Yevqo account.
- Set `YEVQO_PUBLIC_URL` to the public HTTPS origin of the Control Plane (for example, the Cloudflare Tunnel hostname) to allow Yevqo to create and update repository push webhooks automatically. Without it, or without Administration: Write, configure the returned one-time webhook URL and secret manually.
- Projects can now persist one verified GitHub App repository binding through `POST /api/projects/{id}/repository`. The Control Plane verifies that the selected installation belongs to the repository owner, fetches repository metadata and confirms the selected deployment branch before saving it. The project Source Repository tab supports connect, view and disconnect.
- The repository binding is stored by additive migration `024_project_repositories.sql`; it is not applied to an existing/production database automatically. Disconnecting the GitHub App first deletes managed push webhooks and restores the original site Git/webhook settings, then removes dependent project repository bindings.
- The Source Repository tab lists paginated branches, open pull requests, commit status and check runs. Branches are verified with the GitHub App before they can be assigned to a site. Pull requests and checks require the matching read permissions described above.
- Project-level automatic deployment-on-push is wired to the existing signed per-site webhook pipeline. When `YEVQO_PUBLIC_URL` and Administration: Write are configured, the GitHub push webhook is registered/updated by the Control Plane; otherwise the UI provides a manual one-time webhook setup. Durable deployment runs now record Agent job progress, state transitions, build output, cancellation, redeploy and rollback actions. PR/checks UI is backed by the GitHub App endpoints. The webhook secret is AES-GCM encrypted at rest and the endpoint continues to enforce HMAC signature validation, delivery replay protection, branch filtering and exact commit checkout.


## Troubleshooting installation verification

If installation linking fails, the Control Plane returns a sanitized diagnostic category. HTTP 401 usually means the configured App ID and private key do not belong together; HTTP 404 points to an incorrect or removed installation ID; an installation ownership mismatch means the installation belongs to another App. Network and unexpected failures use a generic message. Raw GitHub response bodies, JWTs, installation tokens and private-key contents are never returned to the browser.
