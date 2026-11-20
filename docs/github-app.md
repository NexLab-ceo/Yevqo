# GitHub App integration

Yevqo uses a GitHub App installation as the preferred GitHub connection. The browser never asks for or stores the App private key.

## Register the GitHub App

Create the App in GitHub Developer Settings and configure:

- **Homepage URL:** your Yevqo Control Panel origin.
- **Setup URL:** `https://YOUR-PANEL-HOST/settings/integrations/github`.
- **Webhook URL:** configure separately only when enabling the existing project/site webhook flow.
- **Repository permissions:** Metadata — Read-only; Contents — Read-only.
- **Subscribe to events:** only events needed by the deployment workflow. Do not enable broad permissions by default.

The installation flow uses GitHub's official `/apps/{slug}/installations/new` page. The callback returns `installation_id`, `setup_action` and the one-time `state` created by the authenticated Control Plane session. The state expires after 10 minutes and is consumed once. The Control Plane verifies that the installation belongs to the configured App before showing it as connected.

## Configure the Control Plane

Set these environment variables on the **Control Plane service**, not in frontend build arguments:

| Variable | Purpose |
|---|---|
| `YEVQO_GITHUB_APP_ID` | Numeric GitHub App ID |
| `YEVQO_GITHUB_APP_SLUG` | Public GitHub App slug used to build the installation URL |
| `YEVQO_GITHUB_APP_PRIVATE_KEY` | RSA PEM private key; use real newlines or escaped `\n` |

Restart the Control Plane after setting the variables. The public config endpoint returns only whether the App is configured and its public slug. It never returns the App ID or private key.

When a user completes installation, Yevqo verifies the installation using an RS256 App JWT, obtains an installation access token, and stores the installation ID and credentials encrypted with the Control Plane credential key. Short-lived installation tokens are requested on demand and are not persisted.

If these variables are absent, the UI shows an explicit not-configured state. It does not show a false Connected state.

## Disconnect and permissions

Disconnecting removes the user's encrypted installation record. It does not uninstall the App from GitHub; use GitHub's **Installed GitHub Apps** settings to revoke the installation itself.

The App should request only Metadata and Contents read access until a feature proves that more permissions are necessary. Deployment automation, checks, pull requests and webhook subscriptions must be enabled as separate capabilities with their own minimal permissions and verification.

## Current boundaries

- Repository listing is backed by the verified installation token.
- Installation linking uses a short-lived, single-use state tied to the signed-in Yevqo account.
- Projects can now persist one verified GitHub App repository binding through `POST /api/projects/{id}/repository`. The Control Plane verifies that the selected installation belongs to the repository owner, fetches repository metadata and confirms the selected deployment branch before saving it. The project Source Repository tab supports connect, view and disconnect.
- The repository binding is stored by additive migration `024_project_repositories.sql`; it is not applied to an existing/production database automatically. Disconnecting the GitHub App removes dependent project bindings through the migration foreign key.
- Branches can be selected by entering a branch name; the Control Plane verifies that the branch exists and is visible to the installation. A full branch browser/synchronization UI is still open.
- Project-level automatic deployment-on-push, PR/checks UI and deployment lifecycle are still not claimed complete. Existing signed per-site GitHub push webhook handling remains a separate implemented capability.
