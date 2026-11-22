# Dokploy UI Research — Yevqo Design Reference

**Reviewed:** 2026-11-22  
**Reference products:** Dokploy self-hosted v0.30.7 and Dokploy Cloud v0.31.4  
**Scope:** Read-only review of selected Dokploy screens. This document records observed UI patterns and design conclusions for Yevqo; it does not describe implemented Yevqo functionality.

> No Dokploy settings were changed. No deployments, builds, container operations, backup/restore jobs, or other mutations were triggered. Secret values, webhook URLs, and private infrastructure details are intentionally excluded.

## 1. Yevqo architecture and UX constraints

These constraints were clarified during the review and take precedence over assumptions drawn from Dokploy:

- A Project can contain services placed on multiple Agents.
- One Agent can host services belonging to multiple Projects.
- Every individual service already has its own assigned Agent.
- All tabs inside a service inherit that service's Agent context. Do not add a repeated Agent selector to Deployments, Domains, Logs, Environment, Monitoring, Backups, Volumes, Networks, or Terminal.
- Show the assigned Agent as contextual metadata where useful. Changing placement belongs to a deliberate service-level placement/migration flow.
- Do not add redundant Project / Service selectors inside pages already scoped to a service.
- Application and Compose are distinct service types. Their type-specific settings can differ while common concepts and navigation remain consistent.
- Projects remains the primary place to create and manage projects/sites.
- Never present a UI concept or planned capability as implemented until the Yevqo code and tests verify it.
- Yevqo's primary execution environment is Android + Termux. Docker, Compose, Traefik, Nixpacks and similar Linux server tooling must not be presented as native Termux capabilities. Any such execution must be explicitly tied to a compatible external executor/Agent and verified implementation.

## 2. Dokploy global navigation and Projects

### Observed

Global navigation in the self-hosted view includes Home, Projects, Overview, Monitoring, and Settings entries (Profile, Sessions, Tags, Git), plus Documentation and Support.

The Projects page provides project creation, search, tags, sorting, and project cards with environment/service counts. A project environment view lists services of different types and provides an All servers filter.

The inspected environment contained Application, Compose, and database services.

### Yevqo design implications

- Keep project creation and service management in Projects.
- Project-level inventory may filter/group by Agent, because a Project can span Agents.
- A service card can show its assigned Agent, but do not model Project → single Agent.
- Keep global Agent inventory separate from project/service inventory.

## 3. Application service

### General / Deploy Settings

Observed service tabs:
- General
- Environment
- Domains
- Deployments
- Preview Deployments
- Schedules
- Volume Backups
- Logs
- Patches
- Monitoring
- Advanced

Observed actions include Deploy, Reload, Rebuild, Stop, and Open Terminal. General settings include autodeploy and clean-cache toggles.

Source/build options observed:
- GitHub, GitLab, Bitbucket, Gitea, Docker, Git, Drop
- Repository/account, branch, build path, trigger type (push/tag), submodules
- Build providers: Dockerfile, Railpack, Nixpacks, Heroku Buildpacks, Paketo Buildpacks, Static

**Yevqo direction:** keep source configuration, build configuration, and runtime/deployment behavior understandable as separate concerns. Do not claim a provider is supported by the Android/Termux runtime unless an actual compatible executor exists.

### Deployments

Observed:
- Recent deployment list (last 10 in the inspected view)
- Status, commit message/description, commit SHA, start time, duration
- View and Delete actions
- Clear deployments, Kill Build, Cancel Queues, Configure Rollbacks
- Git-provider webhook integration

**Yevqo direction:** deployment history should show status, commit/branch/SHA, trigger, start time, duration, and result. Detailed deployment view should expose stages, build/runtime logs, errors, cancellation where supported, redeploy, and rollback where supported.

Do not add Project / Service selectors inside the service's Deployments tab. Agent is inherited from the service.

### Environment

Observed .env-style editor with KEY=VALUE syntax, line numbers, syntax highlighting, and Save.

**Yevqo direction:** provide a searchable Key / Value editor, mask secrets by default, reveal individual values deliberately, and support safe add/update/delete plus .env import/export. Never expose secret values in logs or ordinary list responses.

### Domains

Observed:
- Add Domain
- Path, internal Port, HTTP/HTTPS and certificate configuration
- DNS validation
- Existing domain list

**Yevqo direction:** show Domain, Path, Port, SSL/certificate, and status. Support DNS validation and certificate lifecycle where backed by real integrations. Do not add Agent selection here; the service's Agent is implicit.

### Logs

Observed:
- Container selector
- Native log view
- Line limit, time range, log type, search
- Pause, Copy, Download, AI analysis

**Yevqo direction:** black terminal-like surface, JetBrains Mono, streaming logs, search/time/level filters, pause/resume, autoscroll, copy/download, and clear separation of build logs from runtime logs. Browser clients should receive logs through the Control Plane/Master, not connect directly to an Agent.

### Monitoring

Observed metric groups:
- CPU Usage
- Memory Usage and limit
- Block I/O read/write
- Network I/O in/out

The inspected values were zero. They must not be interpreted as proof of zero real resource use or as validated telemetry.

**Yevqo direction:** show CPU, RAM used/limit, disk I/O, network I/O, container count and uptime when the Agent actually supplies them. Provide time ranges. Distinguish service-level metrics from Agent/device-level health.

### Volume Backups (Application)

Observed backup task fields:
- Task Name
- Schedule (Cron and presets)
- Destination
- Volumes
- Backup Prefix
- Keep Latest Backups / retention count
- Option to turn off the container during backup
- Enabled

Restore form includes destination, backup file search, volume name, and restore action. Dokploy warns that the target volume must not be in use by another container.

**Yevqo direction:** backup task editor with schedule, destination, volume selection, prefix, retention, and online/offline backup mode. Backup history should show date, size, duration, status, and integrity. Restore should show source and target details, warn about overwrite/in-use volumes, and offer a pre-restore backup when possible.

### Schedules

Observed scheduled task fields:
- Task Name and Description
- Cron schedule and presets
- Timezone (UTC default)
- Shell type (Bash/Sh)
- Command
- Enabled

**Yevqo direction:** keep scheduled commands distinct from backup schedules. Provide a Cron editor, IANA timezone, shell selection, execution history, exit code, duration, stdout/stderr, and manual run only where implemented. Command execution must be scoped to the service's Agent/runtime and permission-checked.

### Patches

Observed patch manager applies code patches during build after repository clone and before build. Rows include file path, type, enabled state, and edit/delete actions.

**Yevqo direction:** patch manager should support create/update/delete, content editor, enable/disable, deterministic order, history, diff preview, and application to the build working copy rather than silently modifying the upstream Git repository.

### Preview Deployments

Observed preview deployments were disabled for the inspected Application, with a Configure action.

**Yevqo direction:** if implemented, PR previews should be isolated temporary instances with branch/PR, status, URL, Agent context, logs, stop and expiry/cleanup controls. A preview must not mutate the primary service. Clearly label unavailable/planned preview support.

## 4. Compose service

Compose has a distinct settings surface from Application. The inspected Compose service was named bot.

### General / Deploy Settings

Observed actions:
- Deploy
- Fresh Volumes
- Rebuild
- Start
- Open Terminal

Observed source types:
- GitHub, GitLab, Bitbucket, Gitea, Git, Raw

Observed source settings:
- Account/repository/branch
- Compose Path
- Trigger Type (push/tag)
- Watch Paths
- Git submodules
- Preview Compose

**Yevqo direction:** share a consistent source-connection experience with Application, but keep Compose-specific configuration and lifecycle operations distinct. Provide a Compose editor/preview and validation where actually supported.

### Containers

The inspected Containers page showed an empty state: no containers were found and the user was prompted to deploy the Compose project.

This did not expose the populated container list or its controls, so those details remain unverified.

**Yevqo direction:** once available, list each runtime container with name, image/tag, status, CPU/RAM, ports, and creation time. Container details can expose logs, metrics, inspect data, and supported lifecycle actions. Keep the distinction clear: one Compose service definition can produce one or more runtime containers.

### Backups

The inspected Compose Backups page described backups to another provider and offered Create Backup and Restore Backup. No backup configuration was present, so the detailed creation form was not observed.

**Yevqo direction:** a Backup Manager with destinations (Agent-local and supported remote providers), schedules, retention, history, integrity checks, and safe restore. Because the service already has an assigned Agent, backup execution should use that Agent context; do not ask the user to select it again.

### Advanced

Observed sections and behavior:

**Run Command**
- Command field for overriding the Compose execution command.
- The UI showed a default Docker Compose up/build/remove-orphans command.
- Save action.

Yevqo should expose the effective command and explain consequences of overriding it. Avoid making custom commands look harmless; validate and permission-check them.

**Volumes**
- Volume list and Add Volume action.
- The inspected service had no configured volumes.

Yevqo should provide a mount editor distinguishing Docker Volume from Bind Mount, with source, target, type, and read-only mode. Show persistence and data-loss implications clearly.

**Import**
- Base64 configuration import.
- Dokploy warns that import removes/replaces existing Environment Variables, Mounts, and Domains.

Yevqo should preview a diff and enumerate affected settings before import. Never silently overwrite configuration.

**Networks**
- Refresh Compose service list.
- Connect Docker networks to individual Compose services.
- Disconnect from the dokploy-network.
- Changes take effect on the next Deploy.

The inspected service list did not load, so the complete network editor was not observed.

Yevqo should model networks declaratively and show service-to-network membership, aliases, and exposure where supported. Do not imply changes are active before the required deployment/reconciliation succeeds.

**Isolated Deployment**
- Dokploy displayed this as deprecated.
- The description referenced unique prefixes and a separate network.
- The UI recommended declarative Networks instead.

Yevqo should prefer an explicit, declarative network/isolation model rather than copying a deprecated feature. Be precise about actual isolation guarantees; do not promise OS/container isolation that the Android/Termux runtime does not provide.

## 5. Dokploy Cloud settings relevant to Yevqo (v0.31.4)

The Cloud dashboard was reviewed separately from the self-hosted v0.30.7 instance. The following are observed UI patterns and product concepts, not a recommendation to reproduce Dokploy's pricing or hosting assumptions.

### Remote Servers

Observed Create Server flow:
- Connection modes: Run a command and Manual (SSH).
- Name and Description.
- Server Type: Deploy Server or Build Server.
- Enable Docker Cleanup, described as daily pruning of unused Docker images.
- Run the server setup option, checked by default; setup installs Docker, Traefik and server requirements after connection.
- Generate command, Create, Close.
- Empty state explains that servers are used to deploy applications remotely.

**Yevqo direction:**
- Provide a clear Agent inventory with connection state, last activity, OS/architecture, version and available resources.
- Pairing should use a generated, short-lived/single-use command or token, followed by a verified handshake. Never expose reusable raw credentials.
- Consider separate build and deploy capabilities only where the actual executor architecture supports them.
- Make optional setup and cleanup explicit and explain their effects.
- Do not copy Dokploy's automatic Docker/Traefik setup for Android/Termux. Detect platform capabilities and only offer compatible setup steps. Linux-only execution belongs to a compatible remote executor.

### Secrets

Observed external secret provider concept:
- Providers shown: HashiCorp Vault / OpenBao, Infisical, AWS Secrets Manager, AWS Parameter Store, Doppler, Azure Key Vault, Scaleway Secret Manager and Phase.
- Provider configuration includes a display name, provider type, endpoint/URL where applicable, masked token, mount/namespace fields where applicable, and access scoping to projects and optionally environments.
- Reference syntax shown as `${{vault.<name>.<secret>}}` and a path/field form.
- UI describes fetching secrets at deploy time rather than storing fetched secret values in Dokploy.
- Test Connection and Create actions.

**Yevqo direction:**
- Keep the built-in encrypted secret store and consider optional external secret providers as a separate integration.
- Scope access by workspace/project/environment and enforce it server-side.
- Mask credentials, test provider connectivity, and audit access without logging secret values.
- Resolve secrets at execution/deploy time only when the Agent protocol and security model can do so safely. Do not imply secrets are never stored unless the implementation truly guarantees that.

### Container registries

Observed Add Registry fields:
- Registry Name, Username, masked Password, Image Prefix and Registry URL/host.
- Server selector used to test/authenticate from the selected server.
- Test Registry and Create.

**Yevqo direction:**
- Support multiple registry configurations where needed, with encrypted credentials and a connection test.
- Associate pull credentials with the Agent that actually pulls the image. A credential configured only on the Master is insufficient if the remote Agent performs the pull.
- Show image prefix/registry host and private-image selection only when supported by the relevant executor.

### DNS Providers

Observed providers included Cloudflare, AWS Route53, Porkbun, Infomaniak and OVHcloud. The visible form used a name, provider selection and API token, with Test Connection and Create. Cloudflare guidance recommended a scoped token with DNS edit access rather than a global API key.

**Yevqo direction:**
- Support multiple DNS provider connections, least-privilege credentials and connection tests.
- Integrate DNS automation with Domains, routing and certificate validation only where the corresponding provider/API flow is implemented.
- Show record/propagation state and errors; do not claim DNS has propagated based only on successful API submission.

### S3 Destinations

Observed:
- Backup destination setup for AWS S3 and numerous S3-compatible/object-storage providers, including Cloudflare R2, Wasabi, DigitalOcean Spaces and MinIO.
- Fields: Name, Provider, Access Key ID, masked Secret Access Key, Bucket, Region, Endpoint, optional Additional Flags and optional Server.
- Test Connection and Create.
- Description states that a destination is required for remote backups.

**Yevqo direction:**
- Provide configurable local and remote backup destinations, including compatible S3 storage where supported.
- Test connectivity and validate bucket/endpoint settings before saving.
- Store credentials securely and execute data transfer on the Agent that has access to the backup data. The Master can coordinate configuration and schedules without being the data-transfer host.

### Notifications

Observed provider options:
- Slack, Telegram, Discord, Lark, Microsoft Teams, Email, Resend, Gotify, ntfy, Mattermost, Pushover and Custom Webhook.
- Custom Webhook fields: Name, Webhook URL and optional Headers with an Add header action.
- Event categories included app deploy/build errors, database/Dokploy/volume backups, Docker cleanup and server thresholds.
- Test Notification and Create actions.

**Yevqo direction:**
- Support multiple notification destinations and per-destination event selection.
- Include a test action and custom webhook headers where safe.
- Provide delivery status/history, retry/error visibility and secret masking.
- Relevant events include deployment/build outcomes, backup/restore, Agent connectivity, security events and resource thresholds. Avoid adding channels/events that have no implemented producer.

### Users and permissions

Observed Users page:
- Table columns: Email, Role, Status, 2FA, Created At and Actions.
- Invitations section with Add Invitation.
- Custom Roles / fine-grained permissions were identified as an Enterprise feature in Dokploy.

**Yevqo direction:**
- Keep role and permission management available according to Yevqo's own product/security policy; do not copy Dokploy's plan gating.
- Invitation lifecycle should make Pending, Accepted, Expired and Revoked states understandable, with resend/revoke actions where implemented.
- Enforce workspace/project/resource permissions on the server, not only by hiding UI controls.
- Scope access to projects, services, devices/Agents and sensitive operations.

### Audit Logs

Observed:
- Dokploy described audit logs as organization-wide visibility into actions.
- Access was gated by the Dokploy Enterprise license.
- Relevant fields include Actor, Action, Resource, Timestamp, IP Address, Result and Details.

**Yevqo direction:**
- Treat audit history as a core security capability, not automatically as a premium-only feature.
- Record authentication, membership/permission changes, deployment, backup/restore, secret access and destructive operations.
- Support filtering/search and safe export where implemented.
- Protect audit records from ordinary modification. Never record passwords, tokens or secret values.

### SSH Keys

Observed:
- SSH Keys page described keys for servers, private Git repositories and related access.
- The list showed existing key entries and an Add SSH Key action.
- The add form was not opened; field-level details were not verified.

**Yevqo direction:**
- If SSH integration is implemented, distinguish server access keys from Git access keys.
- Show safe metadata such as label, fingerprint, algorithm and creation date; provide revoke/replace lifecycle.
- Do not expose private key material after saving.
- SSH should not be required for every Agent enrollment if the primary connection is outbound authenticated WebSocket/gRPC.

### Logs & Metrics integrations

Observed:
- External providers included Grafana Loki, Prometheus remote write, New Relic, InfluxDB, Datadog, AWS CloudWatch, Splunk HEC, Elasticsearch/OpenSearch and Better Stack.
- The described setup could install a Vector collector on servers; cAdvisor was described as an accompanying container metrics source.
- The UI warned that services deployed before configuration might need redeployment to receive labels.

**Yevqo direction:**
- Separate built-in Agent/device telemetry and application logs from optional external observability exports.
- External exporters should be optional and configured per compatible Agent.
- Do not install Linux collectors such as Vector/cAdvisor on Android/Termux by assumption. Use platform-appropriate telemetry and clearly report unsupported metrics.
- Show exporter connection/last-send/error state only when collected from the real Agent/provider.

### Certificates

Observed:
- Certificates page described creating certificates in the Traefik directory for Traefik to use when securing applications.
- It warned that invalid certificates can break Traefik and make applications inaccessible.
- The Add Certificate form was not opened; field-level details were not verified.

**Yevqo direction:**
- Certificate management should cover supported Let's Encrypt challenges and custom certificate import only when implemented.
- Show domain binding, issuer, expiry, renewal state and validation errors where available.
- Validate certificate/key material before applying it and provide a safe recovery path.
- Apply certificates on the Agent/reverse proxy that actually serves the domain; do not assume the Android Master is the public TLS proxy.

### Billing

The Cloud Billing page was inspected for product-design context:
- Trial and Hobby, Startup and Enterprise plan cards.
- Monthly/annual billing toggle and annual discount.
- Feature/limit comparisons covering servers, organizations, users, environments, backups, scheduled jobs, RBAC, 2FA, audit logs and support.

**Yevqo direction:** no Dokploy prices, limits or plan gating should be copied. If Yevqo later needs a pricing comparison, the useful pattern is a clear feature matrix and billing-period selector. Preserve Yevqo's own product rules, including its project-count policy.

## 6. Cross-cutting design recommendations for Yevqo

1. **Service context is authoritative.** The assigned Agent is inherited by every service tab. No repeated Agent dropdowns.
2. **Project and Agent are many-to-many through service placement.** Projects can span Agents; Agents can host multiple Projects.
3. **Separate service types without fragmenting UX.** Application and Compose share familiar navigation patterns but expose type-specific controls.
4. **Make destructive changes explicit.** Import, volume recreation, restore, removal, and network changes need clear impact summaries and confirmations.
5. **Show actual state, not assumptions.** Distinguish configured, pending, running, failed, unavailable, and unverified states. Never fabricate telemetry or successful operations.
6. **Preserve secret safety.** Mask credentials and tokens; never include them in docs, logs, screenshots, or ordinary UI summaries.
7. **Respect the Android/Termux architecture.** Docker/Compose execution requires a compatible executor; do not imply native Docker support in Termux without verified implementation.
8. **Keep the interface operational and dense but readable.** Use tables for inventories, focused detail pages, terminal-style logs, clear status labels, and concise confirmations.
9. **Do not copy features just because Dokploy has them.** Include only capabilities that fit Yevqo's architecture and have a concrete user need. Avoid unrelated AI, generic infrastructure administration and redundant monitoring screens.

## 7. Inspection coverage and limitations

Reviewed in self-hosted Dokploy v0.30.7:
- Home / Projects / environment service inventory
- Application General, Deployments, Environment, Domains, Logs, Monitoring, Advanced, Volume Backups, Schedules, Patches, Preview Deployments
- Compose General / Deploy Settings, Containers (empty state), Backups (empty state), Advanced (Run Command, Volumes, Import, Networks, deprecated Isolated Deployment)

Reviewed in Dokploy Cloud v0.31.4:
- Home and Billing
- Remote Servers list and Create Server form
- Settings: Secrets, Registry, DNS Providers, S3 Destinations, Notifications, Audit Logs, Users, SSH Keys, Logs & Metrics and Certificates

Not fully inspected:
- Populated Compose Containers list and container detail/actions
- Compose Backup create/restore forms
- Compose Networks editor after service list failed to load
- SSH Key creation form
- Certificate creation form
- Other Dokploy pages not listed above, including Docker and Traefik File System. These were intentionally excluded from the Yevqo research scope because the user decided they were not needed.

This is design research only. It does not imply that any recommended integration or UI capability is implemented in Yevqo. Revisit the live UI only if exact field names, operational semantics, or current Dokploy behavior are needed before implementation.
