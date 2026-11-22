# Dokploy UI Research — Yevqo Design Reference

**Reviewed:** 2026-11-22  
**Reference product:** Dokploy v0.30.7  
**Scope:** Read-only review of the user's Dokploy dashboard. This document records observed UI patterns and design conclusions for Yevqo; it does not describe implemented Yevqo functionality.

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

## 2. Dokploy global navigation and Projects

### Observed

Global navigation includes Home, Projects, Overview, Monitoring, and Settings entries (Profile, Sessions, Tags, Git), plus Documentation and Support.

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

## 5. Cross-cutting design recommendations for Yevqo

1. **Service context is authoritative.** The assigned Agent is inherited by every service tab. No repeated Agent dropdowns.
2. **Project and Agent are many-to-many through service placement.** Projects can span Agents; Agents can host multiple Projects.
3. **Separate service types without fragmenting UX.** Application and Compose share familiar navigation patterns but expose type-specific controls.
4. **Make destructive changes explicit.** Import, volume recreation, restore, removal, and network changes need clear impact summaries and confirmations.
5. **Show actual state, not assumptions.** Distinguish configured, pending, running, failed, unavailable, and unverified states. Never fabricate telemetry or successful operations.
6. **Preserve secret safety.** Mask credentials and tokens; never include them in docs, logs, screenshots, or ordinary UI summaries.
7. **Respect the Android/Termux architecture.** Docker/Compose execution requires a compatible executor; do not imply native Docker support in Termux without verified implementation.
8. **Keep the interface operational and dense but readable.** Use tables for inventories, focused detail pages, terminal-style logs, clear status labels, and concise confirmations.

## 6. Inspection coverage and limitations

Reviewed:
- Dokploy Home / Projects / environment service inventory
- Application General, Deployments, Environment, Domains, Logs, Monitoring, Advanced, Volume Backups, Schedules, Patches, Preview Deployments
- Compose General / Deploy Settings, Containers (empty state), Backups (empty state), Advanced (Run Command, Volumes, Import, Networks, deprecated Isolated Deployment)

Not fully inspected:
- Populated Compose Containers list and container detail/actions
- Compose Backup create/restore forms
- Compose Networks editor after service list failed to load
- Other Dokploy pages not listed above

This is design research only. Revisit the live UI if exact field names, operational semantics, or current Dokploy behavior are needed before implementation.
