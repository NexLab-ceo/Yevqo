# Codex Instructions — Yevqo

## Product identity

Yevqo is an Android-first, self-hosted application platform built around **Android + Termux**. A phone or tablet runs the Yevqo Agent (and optionally the Control Plane); the web interface manages projects and device-side workloads.

- Product name: **Yevqo**.
- The Go module and some internal package, service, and data-directory names still use `veltrix` for upgrade compatibility. The project configuration filename is exclusively `yevqo.yml`; do not support `veltrix.yaml` or `veltrix.yml` as aliases.
- Android/Termux is the primary execution environment, not a thin client for a cloud VPS.
- Do not describe features as available unless they are implemented and verified.

## Repository and branch rules

- Work only in the existing `main` branch.
- Do not create feature branches unless the user explicitly asks.
- Do not force-push, rewrite history, or modify unrelated files.
- Before editing, inspect the current implementation and tests. Preserve existing user data and backward compatibility.
- Keep changes focused. Do not perform production deployment unless explicitly requested.

## Architecture

The repository contains the Yevqo Control Plane and Agent:

- **Control Plane:** SvelteKit-based web interface and Go-backed API/control services.
- **Agent:** Go service/CLI running in Termux on Android; it executes approved device-side operations.
- **Database:** MySQL/MariaDB-backed Control Plane persistence and migrations.
- **Android packaging:** Termux-compatible packages, including combined Control Plane + Agent and Agent-only packages.
- **Frontend:** SvelteKit, Svelte, TypeScript, and Vite. Follow the versions and conventions already present in the repository rather than upgrading dependencies without need.
- **Agent / CLI:** Go. Preserve the module path and compatibility with existing installed packages.

Read the relevant source, migrations, package scripts, and CI workflows before making architectural assumptions.

## Core product constraints

1. Android + Termux is the primary supported runtime.
2. Do not assume Docker is available or supported natively in Termux.
3. Docker, Nixpacks, and Railpack require a separate compatible Linux executor unless a specific implementation proves otherwise. Never present them as native Termux build providers.
4. Do not claim public hosting, public domains, reverse-proxy routing, tunnels, or external execution are working unless the implementation verifies them.
5. Respect Android background execution, battery restrictions, storage permissions, architecture, and package availability.
6. Do not report an operation as successful until the Agent confirms it.
7. Keep dangerous operations typed, validated, permission-checked, and scoped to the selected device/project.
8. Never overwrite existing project files, databases, or non-empty data directories without explicit, safe user intent.
9. Preserve private permissions for secrets, credentials, logs, and backups. Never log tokens or passwords.
10. The Free plan permits one connected Android device. Reconnection of the same device is allowed; a second connected device must be rejected. Do not bypass this limit through bonuses or referrals.

## Product and UX direction

- Yevqo is the product; this repository is the actual platform, not merely a landing page.
- The public marketing website is a separate concern from the authenticated management Dashboard.
- The Dashboard should feel like a practical server-management tool: compact navigation, dense but readable tables, clear statuses, logs, resource metrics, and focused settings.
- Keep the Android device and Termux Agent visible as the core infrastructure.
- Do not invent cloud infrastructure or imply that a mock UI is connected to a real device.
- Clearly label demo, planned, unavailable, and coming-soon functionality.
- The user has asked for a staged approach: finish visual structure first when explicitly requested, then add interactions/backend in later stages. Do not silently expand a UI-only task into backend work.


## Visual design system

The authenticated Yevqo interface uses a graphite and cool-blue palette as its default dark appearance. Keep these values centralized in `frontend/src/app.css` and reuse the CSS variables rather than introducing unrelated one-off colors.

| Token | Value | Usage |
|---|---|---|
| Background | `#090D16` | Main application background |
| Surface | `#111827` | Sidebar, cards, inputs and standard surfaces |
| Elevated | `#1A2638` | Elevated surfaces, hover and accent-soft backgrounds |
| Primary blue | `#8AB4FF` | Primary actions, focus and key accents |
| Secondary blue | `#6D9EFF` | Hover states and secondary accents |
| Main text | `#E9EEF8` | Primary readable text |
| Muted text | `#9AA8BD` | Secondary and subdued text |
| Borders | `#2B3548` | Dividers, outlines and card borders |

Keep semantic danger and warning colors distinct. The existing selectable light appearance remains separate and must not be overwritten by dark-palette changes.

## Project progress tracking (mandatory)

Maintain project progress in this same root-level `codex.md` on every development task. This file is the source of truth for the phase/status overview.

### Phase model

Use these phases consistently:

- **P0 — Foundation:** repository/build setup, packaging, installation, Control Plane ↔ Agent communication, authentication, device pairing, core data model, safety boundaries.
- **P1 — Core platform:** projects, sites/applications, process lifecycle, Git-based deployment, releases/rollback, domains/networking, logs.
- **P2 — Platform capabilities:** monitoring/telemetry, databases, backups, templates/build providers, CLI, previews, teams/RBAC, audit, notifications, analytics.
- **P3 — Product completion:** remaining integrations, UX consistency, operational hardening, documentation, upgrade testing, Android device validation, release readiness.

A phase is not automatically complete because its headline features exist. Mark it **DONE** only when implementation and relevant automated checks are verified. Mark physical Android/Termux validation separately.

### Status vocabulary

Use only these labels in the progress ledger:

- `DONE` — implemented and verified by relevant tests/checks.
- `PARTIAL` — some functionality exists, but meaningful scope remains.
- `TODO` — not implemented.
- `VERIFY` — implementation may exist, but current code/tests/evidence have not been checked.
- `BLOCKED` — cannot proceed because of a specific external dependency or decision; explain it.

Never turn a planned feature into DONE based only on a UI, README statement, mock data, or an earlier conversation. Re-check the current repository and CI before changing a status.

### Current progress snapshot

This is a conservative starting snapshot based on the repository documentation and previously recorded implementation work. Re-verify the relevant source and workflows before relying on it or upgrading any status.

| Phase / area | Initial status | Notes |
|---|---|---|
| P0 — Go Control Plane and Agent foundation | PARTIAL | Core services and Android/Termux packaging are present. The Termux Control Plane runner writes bounded private stdout/stderr logs to `$DATA_ROOT/logs/control.log`, independently of runit `svlogd`. Integration HTTP requests now use a Go resolver that reads `$PREFIX/etc/resolv.conf` on Termux (falling back to `/etc/resolv.conf` elsewhere), avoiding Android's erroneous `[::1]:53` fallback when Termux DNS configuration is available. Resolver parser tests and explicit Termux-precedence / missing-or-invalid-config fallback tests are added; fresh Go CI and physical Android/GitHub App connection verification remain pending. |
| P0 — Android package build and CI | PARTIAL | Android packages and setup scripts are present. Setup launchers now switch to a stable Termux working directory before setup; this prevents a legacy data-root migration or package operation from leaving child service commands in a deleted cwd. Shell/CI and physical Android startup verification remain pending. |
| P0 — Automatic package updates and VELTRIX Dock releases | PARTIAL | Both package updaters now target the public `NexLab-ceo/veltrix-dock` releases, compare installed versions, verify SHA-256/package metadata and install newer `.deb` files. Debian post-install enables a six-hour Termux updater service. The build workflow publishes both packages and `SHA256SUMS` on version tags using `PUBLIC_RELEASE_TOKEN`; token permissions, current CI, first release and physical Android/background execution remain unverified. |
| P0 — Agent pairing token separation | PARTIAL | Dedicated 256-bit random token is issued from Profile, stored only as SHA-256, and checked by WebSocket auth; public referral codes remain for referrals. Rotation revokes the previous token. Existing Agent configs must be re-paired; physical-device verification and multi-device token management remain open. |
| P0 — Account password policy | PARTIAL | Registration and password changes now share a 12–72 byte policy; bcrypt's 72-byte limit and leading/trailing whitespace are enforced. Unit coverage added; full CI verification pending. Existing password hashes and accounts are unchanged. |
| P0 — Device pairing and one-device Free limit | PARTIAL | `UpsertDevice` serializes registration per account, preserves same-device reconnect, rejects cross-account UUID rebinding, and enforces the Free one-device cap regardless of referral bonuses. Unit tests cover the Free cap; Hub tests cover replacement sockets and pending-command shutdown. Physical Android pairing remains under P3. |
| P1 — Project/site lifecycle | PARTIAL | Per-site Environment tab supports encrypted custom variables, masked listing, add/update/delete, search and .env bulk import; a shared set currently applies to Build and Runtime. Built-in runtimes plus Custom Language runtime; arbitrary installed Termux tools and in-site executables can be launched with explicit shell-free start commands. Full end-to-end runtime coverage remains open. |
| P1 — Git deployments, immutable releases, rollback | PARTIAL | Project GitHub App repository binding is persisted by additive migration `024_project_repositories.sql`. Migration `025_project_deployment_lifecycle.sql` adds per-site/device repository targets, per-target verified branches, optional managed push-webhook IDs and durable deployment runs. The Control Plane lists paginated branches and PRs, reads commit statuses/check runs, registers GitHub push webhooks when `YEVQO_PUBLIC_URL` and App Administration: Write are configured (otherwise exposes a one-time manual webhook URL/secret), and routes pushes through the existing HMAC/replay-protected exact-commit deploy path. Agent job events and bounded build output are persisted; project Deployments supports manual Deploy, progress/logs, cancellation, redeploy and immutable-release rollback. Original site Git/webhook settings are restored when targets/repos/projects/App installations are detached. Go tests, Android ARM64 cross-compile, frontend build and packaging passed at code commit `665da05efba7cd7da203b192539e2f6b330a31d7`. Live GitHub App installation, private-repository deploy smoke test, public Cloudflare Tunnel webhook delivery and physical Android acceptance remain open. Migrations `024` and `025` are pending and were not applied to production. |
| P1 — Domains and public routing | PARTIAL | Domain metadata, encrypted Cloudflare API credentials, DNS CNAME automation and Agent connector start/stop are implemented. Domains UI can configure/remove remotely managed Tunnel ingress to the site's local port while preserving unrelated rules and global origin settings. Real Cloudflare account/Android end-to-end route verification remains open. |
| P2 — Device monitoring telemetry | PARTIAL | Heartbeat, typed metrics and project-scoped monitoring/report queries are implemented. CPU, memory, storage and temperature alerts now use persisted state, threshold/recovery hysteresis and deduplicated transition notifications; Agent disconnect/reconnect transitions are also notified. Metric availability depends on Android/device support; physical-device validation remains open. |
| P2 — Database management | PARTIAL | Workspace-owner authorization covers database-server inventory/create/install/start/stop/delete, role/privilege operations, database create/read/stats and site connections. Environment-scoped database-instance assignments, safe legacy backfill and runtime credential injection for start/deploy/rollback/webhooks are implemented. Creating a site/database link now requires both resources to belong to the selected environment. Managed MariaDB/MySQL install/start/stop, isolated per-server Termux data directories, loopback-only listener, optional TLS, database and least-privilege user provisioning, user management, database-level privileges, connection tests, statistics, verified SQL backups and restore into a new database with cleanup on failure are implemented. SQL backup chunk download and retention are supported. Existing-cluster adoption, cross-engine transfer, physical Android verification and end-to-end integration tests remain open. |
| P2 — Website/database backups and restore | PARTIAL | Workspace-scoped backup create/list, database/site restore, chunk download, transfer and schedules are implemented with owner/actor notifications and audit outcomes. Database restore attaches the new instance to the selected project environment only after Agent success. Detaching database servers from projects/environments clears associated membership records without deleting databases. Control Plane MySQL backup now writes a valid newline-delimited private defaults file; site restore, database restore and backup chunk handlers reject empty Agent responses. Site restore state/audit/notification persistence now uses a bounded background context after Agent confirmation. Agent archive extraction rejects traversal, absolute paths, links, unsupported entries, empty archives and payloads over 2 GiB; database restore rejects unsafe names and symlink escapes. MariaDB SQL dump verification, chunk download and retention are also implemented. End-to-end Agent/Android restore verification, physical MariaDB restore tests and integration coverage remain open. |
| P2 — Build providers and templates | PARTIAL | Automatic detection covers Node, Python, Go, Rust, PHP, Java/JVM, .NET, Ruby, C/C++/Make/CMake, Swift, Elixir, Scala, Zig, Dart, Deno, Erlang, Haskell, OCaml, Crystal, Perl, Nim and Lua; universal argv commands support other installed toolchains. Persistent language caches, configurable per-site cache paths, build cancellation/history, custom runtime and seven starters are implemented. `yevqo.yml` now wires environment injection, HTTP healthchecks with ongoing supervision/recovery, include/exclude artifact collection (including `build.output` defaults), memory preflight, deploy restart policy, project root/start command and cache paths into build/runtime/deploy flows. Artifact listing/chunk-download API and Build Engine UI are now implemented with path checks and bounded transfers. General-purpose YAML compatibility and physical Termux/toolchain validation remain open. Remote Linux executor remains outside Android-only scope. |
| P2 — CLI and preview deployments | PARTIAL | `yevqo-cli` supports workspace inventory, project/environment inspection and create/update/delete lifecycle commands, site deploy/start/stop/restart/logs, database/backup inventory and site/database backup queueing, and preview list/create/delete. Site deploy and preview creation require explicit `--allow-project-code` consent; destructive project/environment/preview deletion requires `--confirm`. Preview create/delete and 24-hour expiry cleanup with retry-until-Agent-online are implemented; cancellation-safe cleanup and key/port tests are added. Real Android command verification remains open. |
| P2 — Team, RBAC, audit | PARTIAL | Server-side workspace roles cover project/device/site/database/backup actions. Owners and Admins can switch workspaces, invite/remove members and update active member roles; Developers cannot manage membership or databases/backups. Full endpoint matrix and audit coverage remain open. |
| P2 — Logs, notifications, analytics and reports | PARTIAL | Project-aware logs, deployment history, monitoring, domains and reports are wired through the dashboard; Security Report reads the append-only audit log and supports project filtering. Infrastructure Summary now includes period-scoped build success/failure and backup completion/failure counts, with project filtering. Workspace notifications cover deployment, database lifecycle, backup/restore, metric thresholds and Agent connectivity transitions. Deeper resource analytics and complete audit event coverage remain open. |
| P1 — Project workspaces and environments | PARTIAL | All work is on `main`. Each project owns Production, Development, Staging and custom environments; resources are project/environment scoped and each application can belong to only one environment per project. Per-environment variables are AES-GCM encrypted, masked in API responses, and injected into application start, Android local build and deployment with project → site → managed DB precedence; reserved keys are blocked. Project settings support Admin/Developer/Viewer overrides for active workspace members; project detail, environment/resource assignment, repository/deployment, project-scoped device commands, site lifecycle/deployment, preview, webhook, and site Git/ENV operations enforce effective project roles, with workspace-role inheritance when no override exists. Developers can create apps on attached devices and assign them to project environments. Overview includes devices, applications, DB servers, per-environment app/database counts and recent deployments; Applications supports Agent-backed create/start/stop; Activity combines project-filtered audit and operational events. The project environment detail route now correctly loads all eight datasets, including encrypted environment-variable metadata, and defines its locale state; the previously missing result binding and locale reference were fixed. Remaining hardening: OS/container isolation, full role propagation across legacy workspace-wide database/backup routes, DB-backed migration/lifecycle integration tests, and physical Android runtime verification.
| P2 — AI Agents / Agent Builder | TODO | Do not imply real agent execution until model integration, permission enforcement, and execution lifecycle are implemented. |
| P3 — Full Android device validation | TODO | Requires tests on a real supported Android + Termux device; CI cross-compilation is not a substitute. |
| P3 — Dashboard UX consistency | PARTIAL | Shared shell uses grouped navigation for Operations, Infrastructure and Management; Domains is a real workspace inventory backed by `listDomains`; Schedules aggregates per-site Agent tasks with create/enable/disable/delete actions; Reports supports project scope; Maintenance shows Agent inventory and diagnostics links. Shared shell legacy green brand accents and hard-coded legacy dark surfaces in Deployments/Profile were aligned to graphite/cool-blue tokens; deployment history now distinguishes confirmed success/failure from records without a terminal result. Light appearance remains supported. Projects list, project detail and environment detail received a focused functional/theme pass: repaired environment-page data loading and locale state, removed a stale disabled GitHub-import placeholder in favor of the implemented post-create repository workflow, corrected malformed CSS newline escapes and replaced legacy/undefined color tokens. Remaining: full visual pass across all legacy pages, frontend build, browser/mobile QA and accessibility verification. |

### Mandatory update procedure for every task

At the start of a task:
1. Read this progress ledger and inspect the relevant current source, tests, migrations, and workflows.
2. Identify the phase(s) and exact ledger rows affected.
3. Do not assume the ledger is current; verify before implementation.

Before finishing every task:
1. Update the relevant row(s) in this file, including status and a concise evidence/remaining-work note.
2. If a new feature area appears, add it under the appropriate phase rather than leaving it undocumented.
3. Record meaningful completed work with a short dated entry in the **Progress history** below. Use the current date and reference the commit SHA when available.
4. Update README or feature documentation too when user-facing behavior or setup changes; the ledger does not replace technical documentation.
5. Commit the code and this progress-file update together whenever possible. If a separate documentation commit is necessary, state why.
6. Never mark a feature DONE until its relevant tests/checks have passed. Record failed, skipped, and unavailable checks honestly.
7. Keep the ledger concise and current. Do not append repetitive logs; update existing rows and add only meaningful history entries.


- 2026-11-21 — Added focused tests for the Termux integration DNS selection on `main`: `$PREFIX/etc/resolv.conf` takes precedence, and a missing or invalid Termux file falls back to the system resolver. Extracted path iteration into a small helper without changing resolver behavior. Commits `151fb12` and `d979e3f`; Go CI and physical Android GitHub App verification remain pending. No package was installed, no database or project data was changed, and no production service was restarted.
- 2026-11-21 — Expanded the project detail experience on `main` (commit `fb5d27c`): added resource summary metrics, a dedicated per-project Environments tab, an Applications tab that creates a site through the selected attached device's Agent and then attaches it to the project, project-filtered audit Activity, and workspace member/role visibility in Project Settings. Documented that project memberships inherit workspace RBAC and that environments are never shared globally. The follow-up docs/ledger update records CI as pending; frontend build, browser verification, Agent-backed site creation, and environment resource lifecycle still require validation. No migration was applied, and no existing project/site/database data or production service was changed.

- 2026-11-21 — Extended Project Applications on `main` (commit `0fe5035`): new sites are scaffolded by the selected attached Agent, optionally assigned to the selected project-owned environment (Production by default), and existing applications expose Agent-backed start/stop actions plus open/detach controls. Project Activity now selects the audit-log payload correctly for Owners/Admins while Developers retain operational logs; workspace-member reads are gated to roles allowed to manage membership. Follow-up CI for the Svelte page is pending; no migration or production data/service was changed.

- 2026-11-21 — Finalized the project tab structure on `main` (commit `032cfa7`) after CI caught a missing closing `section` around the Applications panel. Environment and Applications are separate sections; the application flow includes project-owned environment selection and Agent-backed start/stop controls. CI for this revision is running. No database migration, project data, Agent installation or production service was changed.

- 2026-11-21 — Added per-project-environment variables/secrets on `main` (migration `027_project_environment_variables.sql`; commits `49c1260`, `55828e4`, `86d8d59`, `7d9fb20`, `67e8428`, `9c5ce72`, `bb262f8`). Control Plane encrypts values with the existing AES-GCM credential key; list responses expose only variable names/masked state; writes/deletes are permission-checked and audited without logging values. Application runtime merges project-environment variables, then site variables, then managed DB values; environment scope is resolved for all assigned apps, even without DB connections. UI supports add/replace/delete in each environment. Migration is additive and has not been applied to the user's database; CI and physical runtime verification remain pending.

- 2026-11-21 — Added project-scoped membership (migration `028_project_members.sql`; commits `0244354`, `c98df68`, `5ee19e6`, `ce3360e`, `c872a8e`, `1de8a44`, `38d0044`, `41144fb`, `4acdefb`, `4185fa7`, `caad5a5`). Active workspace members can receive project-specific Admin, Developer or Viewer overrides; absent an override, workspace RBAC is inherited. Project detail authorization and project-scoped device commands enforce the effective project role. Settings UI can assign/reset roles. Viewer read-only matrix was added; CI is in progress after an earlier test run correctly caught a stale expectation in the role matrix.

- 2026-11-21 — Completed project Overview resource/deployment summaries (commit `bdb0696`) and propagated project roles through site-page lifecycle/deployment commands (commits `61a76b9`, `b76cbaf`, `288c949`, `7cc5d5e`). Project Activity audit visibility now evaluates the effective project role instead of only the workspace role (`db77ed6`). Project settings and per-environment secret management are documented in `docs/project-workspaces.md`. Fresh CI is running; no migration has been applied to the user's live database and no `.deb` has been installed.

- 2026-11-21 — Extended project environment variables into Android local build payloads as well as application start/deploy (`b050cc9`, `d9991a3`, `4161e03`, `29765d0`). Added automatic project resolution for site commands so removing the `project_id` query cannot bypass a project-specific role (`2b966ff`, `7dd5e53`, `10abd64`); site-level ENV and Git settings now also enforce project role overrides (`c9050e0`, `33b097a`). Project member roles can be changed in place (`04062fc`). The latest Go tests, Android/ARM64 build and frontend build passed on the preceding CI revision; the newest role-hardening/UI changes are still running through CI. Build scripts receive configured secrets after explicit project-code consent; users must only build trusted code.

- 2026-11-21 — Completed project role propagation to preview deploy/delete and automatic GitHub webhook configuration (`3b067e0`, `3ac457a`); project roles now resolve from the attached site's project even if callers omit a `project_id` query. Developers can create apps from the project Applications tab and attach them to the project/environment (`063655d`, `714c017`, `dae78dd`, `8a83f4e`, `9424c48`, `a70eaaa`). Removing a workspace member also clears dormant project-role overrides (`6556d48`). The preceding full build at `1ddae7d` passed Go tests, Android/ARM64 build, frontend build and package verification; the cleanup commit's build is still running.

- 2026-11-21 — Corrected environment-variable and project-member endpoint path segment parsing (`6c411a9`, `a55caf1`), so GET/POST/DELETE member operations and per-environment variable CRUD route to the intended handlers. Added per-environment resource counts (applications, database servers, database instances) to the environments API and Overview (`e8f3be9`, `4ef921d`), using one aggregate response instead of N+1 frontend requests. The newest count/UI build is running; previous route-fix CI passed.

- 2026-11-21 — Final UI/summary polish: Project Activity now displays useful resource context for audit and operational entries (`3d4eb1b`); environment resource counts are localized for Russian users (`2bf8e0e`). The localization commit `2bf8e0e` passed Go tests, Android/ARM64 compilation, Svelte/Vite frontend build, all-in-one/standalone `.deb` package verification, checksums and artifact upload. The follow-up `codex.md`-only commit `21dae64` has no source-code changes.

### Progress history

- 2026-11-21 — Fixed the integration HTTP client's DNS selection on `main`: outbound GitHub/GitLab/Cloudflare integration requests now use Go's resolver with DNS servers read from Termux `$PREFIX/etc/resolv.conf`, with `/etc/resolv.conf` fallback outside Termux. Added parser tests for IPv4/IPv6 nameservers and invalid/empty entries. This targets the reproduced `lookup api.github.com on [::1]:53: connection refused` failure; Go CI and a fresh physical Android GitHub App connection attempt are still required before marking it verified. No GitHub App credentials or database records were changed.


- 2026-11-21 — Added credential-safe GitHub App failure logging and corrected stage attribution for transport errors: installation lookup and installation-token creation now have distinct wrapped errors, and the authenticated callback logs the server-side request/transport diagnostic plus installation ID without logging JWTs, PEM contents or installation tokens. Added a regression test for token-creation timeout messaging. CI and physical Android verification pending.

- 2026-11-21 — Added independent Control Plane service logging to the Termux runner on `main`: stdout/stderr are redirected to `$DATA_ROOT/logs/control.log` with a private directory/file (0700/0600), symlink/non-regular-file checks and startup rotation above 5 MiB. This bypasses a missing or broken nested runit logger without changing GitHub App credentials or database state. CI and physical Android verification pending.

- 2026-11-21 — Fixed a Termux installer working-directory hazard on `main`: the combined, Control Plane and Agent setup scripts now move to `$HOME` (or `$PREFIX`) before performing setup, so a data-root migration cannot leave subsequent `sv`/service commands running from a directory that was moved. Also moved Control Plane log helpers before the legacy migration branch so migration diagnostics are callable. Existing configs and database data are preserved; shell/CI and physical-device verification remain pending.

- 2026-11-21 — Added periodic Termux package updates for both Yevqo editions, public VELTRIX Dock release lookup, SHA-256 and Debian metadata validation, non-interactive package installation, and service restarts. The build workflow now publishes both packages plus checksums to `NexLab-ceo/veltrix-dock` for version tags. CI, cross-repository token permissions, first published release and physical Android validation remain pending; no device or production database was changed.


- 2026-11-21 — Improved GitHub App installation verification diagnostics: backend now maps GitHub HTTP 401/403/404/429, App/installation mismatch, PEM parsing and network failures to actionable messages without exposing provider response bodies or credentials. Added unit coverage for safe error mapping. Automated tests and CI are pending; no production service or database was changed.

- 2026-11-21 — Fixed the Debian all-in-one package Control Plane UI refresh: `packaging/setup-control.sh` now reads frontend assets from `$PREFIX/share/yevqo/public`, validates the packaged `index.html`, and copies the current bundle into the configured `$DATA_ROOT/public` served by Control Plane. Deliberately does not copy packaged migrations during setup, because server startup can auto-apply pending migrations to the existing database. Shell/CI verification is pending; no production device, database, or deployment was changed.


- 2026-11-20 — Began Control Panel product-spec implementation on `main`: consolidated global navigation to Overview, Projects, Deployments, Logs, Devices and Settings; restored the canonical dark-green Yevqo design tokens and shell styling. Existing routes and APIs are preserved. Visual/build verification pending; global search, complete EN/RU coverage and official GitHub App installation callback remain open. The dedicated `/overview` route now reads real projects/devices/sites; active deployment count is explicitly unavailable until the API exposes active jobs.

- 2026-11-20 — Unified registration and password-change validation: passwords must be 12–72 bytes, cannot have leading/trailing whitespace, and are validated consistently before bcrypt hashing. Updated registration UI and added boundary/UTF-8 tests. Existing accounts and stored hashes are untouched; CI verification is pending.

- 2026-11-20 — Full main CI passed at `755e16f` (build run 44744440881; PR checks run 44744440856): Go tests, Android ARM64, frontend, setup/updater validation, and both package verifications. No release was published.

- 2026-11-20 — Fixed the MySQL client defaults-file regression test to ignore the expected trailing newline and added explicit quote/backslash escaping coverage; included in successful main CI at `755e16f`.

- 2026-11-20 — Added audit events for Cloudflare account connect/disconnect and device unpairing after managed routes are removed; included in successful main CI at `755e16f`.

- 2026-11-20 — Fixed Control Plane MySQL backup option-file formatting (real newlines instead of literal `\
`) and added a regression test. Hardened site/database restore and backup download against empty Agent responses; all included in successful main CI at `755e16f`.

- 2026-11-20 — Added Cloudflare route cleanup to device unpairing: all managed ingress/DNS routes are removed before the Agent is disconnected or device record deleted; included in successful main CI at `755e16f`.

- 2026-11-20 — Cloudflare ingress updates now preserve global `originRequest`, path-only rules and catch-all ordering, and refuse to overwrite an existing unmanaged hostname route. Real account/Android route verification remains open.

- 2026-11-20 — Cloudflare route API and Domains UI configure remotely managed ingress to a site's local port and create/remove the matching CNAME; unrelated ingress rules and global origin-request settings are preserved. Domain changes, site/device deletion and Cloudflare disconnect are guarded against dangling managed routes. Go tests, Android ARM64 build, frontend build and package checks passed (build run 44743655215; checks run 44743655367; commit `1cfe9a8`).

- 2026-11-20 — Completed CLI project/environment lifecycle and preview commands with explicit consent/confirmation; Android ARM64, frontend, Go tests and both packages passed (build run 44740854997; checks run 44740854986; commit `41699cf`).
- 2026-11-20 — Added Cloudflare remotely-managed Tunnel ingress and DNS route API, including preserved existing ingress rules and safe route removal. Cloudflare account and real Android validation pending.

- 2026-11-20 — Added CLI preview listing, creation and deletion. Preview creation requires explicit code consent; deletion requires `--confirm`; creation output includes the preview key/URL/port. Tests cover all three API workflows; included in successful main CI at `755e16f`.

- 2026-11-20 — Added project and environment create/update/delete commands to `yevqo-cli`; destructive commands require `--confirm`, and server-side Production protections remain authoritative. Added command/API tests and documented lifecycle commands (implementation commit `e319e39`).

- 2026-11-20 — Extended `yevqo-cli` with authenticated project/environment create, update and delete workflows; delete commands require `--confirm` and preserve app/database data. Added API-method and command tests; included in successful main CI at `755e16f`.

- 2026-11-20 — CLI deploy now refuses to run without the explicit `--allow-project-code` flag; the flag is passed to the Control Plane and covered by tests. Updated `docs/cli.md`. Go tests passed; full CI run 44740248528 / 44740248717 in progress (commit `36e02b1`).

- 2026-11-20 — Expanded Infrastructure Summary with 7/30/90-day build and backup success/failure analytics and project-scoped filtering. Go tests and Android ARM64 cross-compilation passed; frontend/package CI was still running when the CLI hardening task started (implementation commit `ef18bb7`).
- 2026-11-20 — CLI deploy now requires explicit `--allow-project-code` consent; tests verify the flag is required and transmitted; included in successful main CI at `755e16f`.

- 2026-11-20 — Hardened preview creation/deletion against client cancellation, retained the 24-hour expiry cleanup worker, and added key/port/error tests. Go tests, Android ARM64 builds, frontend build and both package verifications passed (build run 44739751259; checks run 44739751260; commit `f4f77d6`).
- 2026-11-20 — Added period-scoped build and backup analytics to Infrastructure Summary, including project filtering and success/failure counts; included in successful main CI at `755e16f`.

- 2026-11-20 — Added persisted CPU/RAM/storage/temperature threshold alerts and deduplicated Agent offline/reconnect notifications. Go tests, Android ARM64 builds, frontend build, package assembly and package verification passed (build run 44739395185; checks run 44739395193; commit `7c44984`).
- 2026-11-20 — Hardened preview lifecycle cleanup so client cancellation cannot leave a live preview after failed metadata updates; added key, port-range and error-truncation tests; included in successful main CI at `755e16f`.

- 2026-11-20 — Verified device metric alerts and added deduplicated offline/reconnect notifications using the same persisted alert-state table; all included in successful main CI at `755e16f`.

- 2026-11-20 — Added persisted device alert state with CPU/memory/storage/temperature thresholds, recovery hysteresis and transition-only notifications. Metric ingestion from the Agent WebSocket now evaluates alerts without failing telemetry ingestion; included in successful main CI at `755e16f`.

- 2026-11-20 — Added environment-scoped database-instance assignments and runtime credential injection across site start/deploy/rollback and GitHub webhooks; added environment-aware PostgreSQL restore and safe membership cleanup on server detach. CI passed (build run 44738111638; checks run 44738111616; commit `83e0654`).

- 2026-11-20 — Environment-scoped database membership and runtime credential injection are implemented across site start/deploy/rollback and GitHub webhook flows. Ambiguous multi-environment sites require explicit environment selection; rollback now carries the protected environment into the Agent. CI passed (build run 44737658706; checks run 44737658666; commits `11ffe18`, `6ea6733`).

- 2026-11-20 — Verified environment-scoped database-instance membership after fixing the migration-test assertion: Go tests, Android ARM64 cross-compilation, frontend build, all-in-one and Agent package assembly/verification passed (build run 44736548473; checks run 44736548519; implementation commit `e9d4760`). Added database.read enforcement to environment database inventory endpoints. Runtime environment selection is the next hardening step.

- 2026-11-20 — Added environment-scoped database-instance membership on top of the existing server grouping. New instances created from an environment are linked to it; project environment pages can attach/detach existing instances without deleting data; environment database inventory now filters by instance membership. Migration `024` safely backfills only databases whose server belongs to exactly one environment in that project. Go tests, Android ARM64 builds, frontend builds, package assembly and package verification passed after correcting a migration-test SQL-spacing assertion (build run 44736548473; checks run 44736548519). Environment-aware runtime credential injection is being completed separately.

- 2026-11-18 — Added the initial Codex project instructions and established the P0–P3 progress ledger. This is a tracking baseline, not a claim that all listed areas have been re-verified.
- 2026-11-18 — Expanded first-party App Templates with PHP and Node.js JSON API starters, stable template-ID selection in the device UI, regression tests, and updated template documentation. Pull request checks passed (run 44344232085); Android build/package workflow passed, including Go tests, ARM64 cross-compilation, frontend build, and both Debian package verifications (run 44344231782).
- 2026-11-18 — Expanded Yevqo Engine with broader language detection, universal shell-free argv commands for installed Termux toolchains and project-local executables, persistent per-language caches, build process-group cancellation, and a Custom Language site runtime that can deploy with an explicit start command. Added regression tests and updated Build Engine/template docs. Go tests, Android ARM64 cross-compilation, frontend build, package assembly and both Debian package verifications passed (build run 44348095105; checks run 44348094193). Physical Android execution remains unverified.
- 2026-11-18 — Added private per-site Build History with 10-record retention, local log persistence, a history API command and Build Engine UI. Added persistence/retention/symlink-safety and command API tests. Go tests, Android ARM64 cross-compilation, frontend build and both Debian package verifications passed (checks run 44349279135; build run 44349279009).
- 2026-11-18 — Optimized Build History so the list returns metadata only and full build logs are fetched on demand, preventing large multi-build responses. Added detail command, UI integration and common language-toolchain environment forwarding. Go tests, Android ARM64 cross-compilation, frontend build and both Debian package verifications passed (checks run 44350513323; build run 44350513255).
- 2026-11-18 — Standardized project build configuration on the single filename `yevqo.yml`. Removed `veltrix.yaml` / `veltrix.yml` fallback behavior, updated validation and Git-deploy errors, migrated regression fixtures and documented the supported schema. Checks run 44354471130 passed; packaging run 44354472014 was still running at the last recorded check.
- 2026-11-18 — Hardened the `yevqo.yml` scalar parser: it now rejects unsupported schema versions/sections/keys, duplicate keys, tabs, unsupported list syntax, malformed indentation, non-regular config files and invalid ports; it exposes project name/runtime metadata and adds regression tests. The strict scalar subset requires explicit `version: 1`; Go tests, Android ARM64 cross-compilation, frontend build and both Debian package verifications passed (checks run 44359041463; build run 44359041497). Full schema features such as environment injection, healthchecks, artifact collection, cache paths, resource enforcement and deploy policy remain unimplemented; do not mark the manifest feature complete.

- 2026-11-19 — Hardened the packaged Termux setup scripts: Control Plane setup now validates the administrator username before embedding it in JSON configuration, removed a duplicate `PREFIX` assignment, and labels the Agent credential as the pairing code. README and first-test instructions now explain same-phone pairing through Profile → Agent pairing code and `ws://127.0.0.1:8080/ws`. Fresh CI verification is required; Agent setup now installs the shared packaged boot hook used by Control Plane, which starts/waits for the existing MariaDB before starting installed services. No physical Android test has been run.

- 2026-11-19 — Closed a first-test setup gap: the installer, README and smoke-test guide now correctly identify the Agent credential as the account's Agent pairing code (Profile → Agent pairing code) and provide the same-phone WebSocket URL `ws://127.0.0.1:8080/ws`. This aligns the setup prompt with the actual WebSocket authentication implementation.

- 2026-11-19 — Fixed the combined Control Plane + Agent reboot path: Agent setup no longer replaces the Termux:Boot hook with one that skips MariaDB. The generated hook now starts the existing database, waits for readiness, then starts installed services. This preserves existing database data and only starts the existing MariaDB data directory.

- 2026-11-19 — Finalized the safer combined-install boot behavior without changing package contents: Agent setup now preserves an existing `~/.termux/boot/00-veltrix` hook (including the Control Plane hook with MariaDB startup). Only when no hook exists does Agent setup create a minimal Agent-only boot hook. This avoids overwriting Control Plane startup and keeps the Agent-only package self-contained. The first-test guide is being moved to the latest verified package build.

- 2026-11-19 — CI caught a malformed Agent-package `grep` verification line introduced while extending the package manifest; corrected the workflow command before accepting the change. Re-run CI on the corrected workflow commit is required.

- 2026-11-19 — Corrected the Agent package verification block after CI's workflow-file parser reported a malformed line. The verification now checks the Agent updater and shared boot-hook paths with simple content matches. Awaiting the build/check run for the corrected workflow.

- 2026-11-19 — Fixed a second CI issue in the package manifest: the new Agent share-directory entry had an extra shell continuation slash, which broke the `mkdir -p` command. Corrected it to a single continuation; verify the next package workflow before marking this task complete.

- 2026-11-19 — Restored `.github/workflows/build.yml` byte-for-byte from the last known-good revision after exploratory package-manifest edits produced invalid workflow content. The Agent-only package remains unchanged; only its setup script is hardened. Current build run #1008 and checks run #959 are pending.

- 2026-11-19 — Security hardening: Agent pairing credentials now live under `$PREFIX/var/lib/veltrix-agent/config/agent.json` in Termux private storage, not shared `/sdcard`. A valid legacy config is migrated into the private location and removed from shared storage after validation; the runit service reads the private path. The first-test guide documents this behavior. CI verification is pending.

## Implementation guidelines

### Go

- Follow existing package boundaries and error-handling patterns.
- Validate all IDs, paths, ports, command arguments, and filesystem operations at the Agent boundary.
- Prefer explicit typed commands over arbitrary shell execution.
- Avoid shell interpolation of user-controlled values. Use argument arrays or safe stdin-based input.
- Add or update unit tests for success, validation failures, concurrency, and failure/recovery paths.
- Preserve lifecycle locking and crash-safety guarantees around builds, deployments, and process changes.

### Frontend

- Follow the existing SvelteKit route/layout/component structure.
- Use TypeScript types for API payloads and UI state.
- Keep components reusable and accessible.
- Include loading, empty, error, unavailable, and success states where relevant.
- Avoid fake actions: a control must either perform its implemented action or be clearly presented as a placeholder/disabled state.
- Do not add a new UI framework or dependency without a clear need.

### Database and migrations

- Migrations must be additive and safe for existing installations.
- Never drop or rewrite user data as part of routine migrations.
- Keep migration application idempotent and preserve migration checksums.
- Scope records by account/workspace and device where applicable.
- Encrypt stored credentials and keep encryption keys outside the database with restrictive file permissions.

### Security

- Enforce authorization on the server/API, not only in the UI.
- Check ownership and role permissions for every resource mutation.
- Treat device commands, Git URLs, paths, environment variables, archives, and webhook payloads as untrusted input.
- Prevent path traversal, symlink escapes, command injection, unsafe archive extraction, and secret disclosure.
- Keep audit logs append-only where the existing design requires it.
- Never add real secrets, tokens, private keys, or production credentials to source control.

## Testing and verification

Before finishing a change:

1. Inspect the relevant workflow files and existing project scripts.
2. Run the narrowest relevant tests first.
3. Run the repository's applicable Go tests and frontend checks/build.
4. Verify Android/Termux cross-compilation or package checks when the change affects Agent/runtime/package code.
5. Report exactly which checks ran and their results.
6. If a check could not run, state why. Do not claim a physical Android runtime test unless it was actually performed on an Android/Termux device.

Do not introduce paid CI services or paid infrastructure. Prefer the repository's existing free GitHub Actions setup.

## Documentation

- Update README/docs when behavior, setup, supported platforms, commands, migrations, or limitations change.
- Distinguish implemented behavior from planned work.
- Keep user-facing installation and upgrade instructions compatible with existing package names and data directories.

## Working style

- Make progress directly; avoid asking the user to repeat information already present in the repository or project context.
- Ask only when a missing decision blocks a safe implementation.
- Do not claim work is complete until changes are committed and relevant checks have been inspected.
- Summarize changed files, behavior, tests, and any remaining limitations in the final response.

- 2026-11-18 — Expanded the canonical `yevqo.yml` schema and connected supported settings to execution: per-project build/runtime environment, configured HTTP readiness and periodic health supervision (including Agent restart recovery), persistent project cache paths, bounded include/exclude artifact collection and `build.output` default, memory preflight, custom run command/project root, and deployment restart policy. Added parser, cache, artifact and retention regression tests; updated Build Engine documentation.

- 2026-11-18 — Completed the first Artifact Manager workflow: successful build artifacts can be listed per build-history record and downloaded in bounded 256 KiB chunks through Agent commands and the Build Engine UI. Added path/symlink validation and regression tests. Go tests, Android ARM64 cross-compilation, frontend build, package assembly, both Debian package verifications and artifact upload passed (build run 44363848948; checks run 44363849004). Publishing was skipped by workflow configuration. Physical Android/Termux validation remains outstanding.


- 2026-11-18 — Expanded the existing per-site Environment tab toward the Dokploy-style workflow: clearer variable management, search, add/update/delete, .env bulk import with partial-failure reporting, and explicit managed-database variable separation. Values remain encrypted and are never returned to the browser; separate Build/Runtime scopes remain future work.
- 2026-11-19 — Verified the Environment changes on current main: PR checks passed (run 44370256837); Go tests, Android ARM64 cross-compilation, frontend build, Debian package assembly and package metadata checks passed (build run 44370256506, head a96d98ee5a5eaa62d35dcbb829893cf09a10e73f). Added docs/first-test.md with a clean-device Android/Termux smoke-test procedure. This is CI verification only; no physical-device test has been performed yet.

- 2026-11-19 — Started Dokploy-style project workspaces: added additive migration `021_project_environments.sql` with Production, Development and Staging defaults plus project/environment site and database-server association tables; added authenticated environment list/create/site-assignment API and client methods; project detail now displays and creates environments. Full project isolation for database inventory, Build Engine, Deployments, navigation, and end-to-end tests remains in progress. No production deployment or database migration has been run from this change.

- 2026-11-19 — Project workspace continuation on `feature/project-workspaces`: project sidebar now scopes Infrastructure (Devices, Applications, Databases) and Delivery (Build Engine, Deployments); project-aware list endpoints filter devices, applications, database servers and deployment history; new projects receive Production/Development/Staging; environment detail page supports assigning project applications; database servers can be assigned to projects; global Domains route now redirects to Applications and domain/tunnel-token settings are available on each site's Domains tab. Changes are not merged or deployed. Automated checks and end-to-end verification are still required; database instances, backups, monitoring/log scoping, and complete environment lifecycle remain to finish.

- 2026-11-19 — Continued project workspace hardening on `feature/project-workspaces`: fixed duplicate Svelte `page` import that broke frontend compilation; added authenticated environment update/delete endpoints, transactionally removing environment-site assignments while preserving sites, and server-side Production protection; added environment settings UI and API helpers. Latest Go tests/Android cross-compile were green in the failing CI run; frontend failure was the duplicate import. New CI runs are pending. Database CRUD isolation, broad resource authorization tests, and migration integration tests remain in progress.


- 2026-11-19 — Verification and database project-scope follow-up: fixed duplicate Svelte `$app/stores` import in `frontend/src/routes/sites/+page.svelte`; GitHub Actions run 44535626760 passed Go tests, Android ARM64 cross-compilation, dependency installation and frontend production build. Added project-aware database instance listing and preserved project context through database inventory/create navigation; database provisioning now rejects a server not attached to the authenticated project. These are CI-verified code changes only; no database migration or production deployment was run. Remaining: complete project-aware authorization for database detail/lifecycle/role/privilege/connection/backup operations, add migration/resource-assignment integration tests, and verify the latest follow-up CI runs.


- 2026-11-19 — Yevqo rename landed directly on `main`: Termux executables, runit service names, setup/update commands, Debian package identifiers/artifact names, systemd runner filenames, boot hook (`00-yevqo`), build workflow, and user documentation use Yevqo. The Control Plane now uses `/sdcard/yevqo`; setup safely migrates an existing `/sdcard/veltrix-control` directory when the destination is absent/empty, and refuses ambiguous dual-populated roots. Agent private config remains under `$PREFIX/var/lib/veltrix-agent` for compatibility. Agent setup migrates the old boot-hook filename without discarding its contents. Database names and Go module/import paths remain unchanged for compatibility. CI verification is pending.


- 2026-11-19 — Standalone Agent package simplified: package name `yevqo`, executable/service `yevqo`, setup `yevqo-setup`, updater `yevqo-update`. All-in-one keeps `yevqo-agent` to distinguish its Agent from Control Plane; deploy helper CLI becomes `yevqo-cli` to avoid a binary-name collision. Legacy data/config locations remain intact. Standalone `yevqo` resolves the private Agent config written by setup while retaining the legacy config fallback. CI validation required.

- 2026-11-19 — The source-based Agent installer and the dashboard's device-connection instructions now use the standalone `yevqo` service name too. The all-in-one packaged setup retains `yevqo-agent`.
- 2026-11-20 — Continued non-AI platform work on `main`: completed workspace-scoped database/backup operations, safe project resource attach/detach, project and environment database assignments, environment-filtered database creation, workspace switching and Admin team membership/role management. Added deployment/database/backup/restore notifications with actor attribution, project/workspace audit events and failure outcome classification; expanded `yevqo-cli`; corrected Security Reports to use project-filtered audit logs; and hardened Agent reconnect/shutdown behavior. Full build and PR checks passed at `99bce1e`; websocket reconnect tests passed at `041779a`. Only documentation commits followed that verified code head. Additive migrations `021` and `022` were added but not applied. No production deployment or existing DB/data changes were made.
- 2026-11-20 — Fixed the latest `main` CI blocker in `internal/control/modules.go`: failed project-environment initialization now rolls back the newly created project using the authenticated owner ID (`u.ID`), rather than an out-of-scope identifier. The failure was confirmed in both build and verification logs at `65854d4`; fresh CI for `5556517` is required before treating the build as green. No database migration, production deployment, or existing-data change was performed.

- 2026-11-20 — Separated public referral codes from Agent authentication on `main`: added additive migration `023_agent_pairing_tokens.sql`, SHA-256-only token storage, authenticated token rotation endpoint, one-time token reveal in Profile, WebSocket validation against dedicated credentials, duplicate-auth rejection on an established socket, and updated setup/first-test documentation. Existing referral codes and account rows are preserved; existing Agent configs using referral codes must be re-paired. Automated CI and physical Android reconnection checks are pending. No migration was run against a live database and no production deployment occurred.

- 2026-11-20 — Continued the five requested hardening areas on `main`: separated Agent pairing credentials from referral codes using migration `023`, one-time token reveal, SHA-256-only storage and revocation on rotation; added GitHub App installation support using RS256 JWTs, encrypted PEM credentials, short-lived installation tokens, App-scoped repository listing and on-demand private Git HTTPS authentication for deployments. Existing signed push webhook auto-Deploy remains protected by HMAC, branch filtering and delivery replay protection. Added a same-environment check before linking a site to a managed database and clarified that this is application-level scoping, not OS/container isolation. Backup/restore paths and existing archive-safety tests were reviewed; end-to-end Agent/Android restore validation remains open. GitHub Actions verification is pending. No live database migration, production deployment or existing-data modification was performed.

- 2026-11-20 — Follow-up hardening: Agent token rotation now closes all live Agent WebSocket sessions for that account, not just future authentication attempts. Added additive-migration tests for `023_agent_pairing_tokens.sql`, expanded site archive restore tests for traversal/absolute/backslash/hardlink/device/empty/oversized inputs, and verified invalid database backup names are rejected before touching backup storage. A CI failure exposed a malformed escaped newline in GitHub App JWT code; the source was corrected and a fresh run is pending. No live data or production deployment was touched.

- 2026-11-20 — Implemented managed MariaDB/MySQL on `main`: Termux package bootstrap and system-table initialization refuse non-empty or symlinked data directories; each Control Plane server gets a dedicated `server-<id>` directory and binds only to `127.0.0.1`; TLS certificates are generated and protected for `require`/`prefer` modes. Agent commands now support lifecycle, database/user creation, role/user list/create/drop/password, database privileges, connection checks, statistics, SQL dump creation/verification, chunk download, retention and restore into a new target database. Provisioning and failed restores clean up newly created objects; SQL restore rewrites the dump's `USE` statement to the requested target database. Control Plane APIs, scheduled backups, and database/server UI now route by engine. Added tests for safe paths, dump rewriting and backup-name validation. CI at `69a1436` passed Go tests, Android ARM64 build, frontend build, package assembly and package verification before the final small follow-up commits; fresh CI is pending. No existing MariaDB data directory was adopted or changed, and no physical Android installation/restore was claimed.

- 2026-11-20 — MariaDB integration follow-up: linked-site runtime variables now select `DB_CONNECTION=mysql` for MariaDB/MySQL-compatible servers (and retain `pgsql` for PostgreSQL); the connection inventory includes the engine, `DB_SSL_CA` is injected for MySQL-compatible sites and allowed by the Agent process environment, and users created under `ssl_mode=require` receive `REQUIRE SSL`. Added the matching PDO_MYSQL CA verification instructions and driver-selection unit test. Current CI: Go tests and Android ARM64 cross-compilation passed; frontend build/package verification is still running. `mysql` remains a Termux MariaDB compatibility alias, not Oracle MySQL Server.

- 2026-11-20 — Applied the approved Yevqo dark-green palette to the authenticated frontend in `frontend/src/app.css`: `#080A09` background, `#101611` surface, `#18221A` elevated, `#A5EC79` primary, `#78C85A` secondary, `#E9EEE8` main text, `#929D91` muted text and `#2C392D` borders. Primary buttons and focus states now use the green accent; semantic danger/warning colors and the separately selectable light theme remain intact. Frontend/CI verification for this CSS-only change is pending.

- 2026-11-20 — Audited the browser → Control Plane API → Agent WebSocket command path. Fixed three frontend commands that were supported by the Agent but rejected by Control Plane payload parsing: `get_site_restart_policy`, `set_site_restart_policy`, and `cancel_deployment_job`. Added payload-routing tests and corrected README guidance to distinguish the dedicated Agent pairing token from public referral codes. Full browser/Android end-to-end communication still requires physical-device acceptance testing; no production deployment or live database change was performed.
- 2026-11-20 — Communication follow-up: failed Agent WebSocket authentication now closes the client socket before reconnecting; the Control Plane gives unauthenticated sockets a 10-second read deadline and restores the normal heartbeat deadline only after successful authentication. Developer role can read site restart policy while changing it remains owner/admin-only. Added role-matrix coverage. These changes reduce stale unauthenticated connections and preserve the intended configuration permission boundary.
- 2026-11-20 — Fixed a long-lived WebSocket heartbeat regression found during the communication audit: Agent previously kept the 10-second authentication read deadline after successful login, which could disconnect an otherwise healthy session before the 30-second heartbeat. Successful authentication now restores the 60-second pong/read deadline. Fresh CI is running; physical Android verification remains outstanding.


- 2026-11-20 — Added project-level GitHub App source binding on `main`: additive migration `024_project_repositories.sql`, authenticated GET/POST/PUT/DELETE project repository API, installation-owner validation, repository metadata verification, selected-branch existence check, encrypted-installation-token use, and Source Repository connect/view/disconnect UI. Added repository-name validation tests and updated GitHub App/project workspace documentation. The connected-project UI also supports changing the deployment branch with server-side branch verification. Main implementation commits include `f750621`, `42419ed`, `acecd866`, `c806426`, `ff48992`, `8129c37` and `aa75a93`. Migration is pending and has not been applied to any live database; automatic deployment-on-push and project-to-site mapping remain open. GitHub has not yet reported CI status checks for the latest code commit.


- 2026-11-20 — Completed the next project GitHub/Deploy integration stage on `main`: per-site/device repository targets with per-target branch verification; safe snapshots/restoration of prior site Git and webhook settings; optional automatic GitHub push webhook create/update/delete using `YEVQO_PUBLIC_URL` and App Administration: Write, with one-time manual webhook fallback; paginated branch/PR listing and commit checks/status API; private repository token resolution through the exact linked GitHub App installation; and project Deployments with durable Agent state/progress/build output, manual Deploy, cancel, redeploy and immutable-release rollback. Push deliveries create tracked runs and update the repository's latest commit. Added migration `025_project_deployment_lifecycle.sql`, deployment state/public URL tests, site Git settings synchronization and disconnect cleanup across target, repository, project and GitHub App removal. Original site Git/webhook settings are restored on detach. CI run `44814096981` and PR checks `44814096966` passed on latest code revision `665da05efba7cd7da203b192539e2f6b330a31d7` (Go tests, Android ARM64 cross-compile, frontend build, packaging). Live GitHub App permission/installation, Cloudflare Tunnel webhook delivery and real-device private repository deployment smoke tests remain outstanding. Migrations `024` and `025` remain unapplied; no production database or deployment was changed.

- 2026-11-22 — Started a minimal Dokploy-inspired visual refinement on `main`: compact navigation and top bar, neutral low-noise surfaces, restrained borders, consistent controls/tables, and removal of decorative gradients/graphics from the Overview while preserving Yevqo's dark-green palette, light theme, responsive layout, routes and existing interactions. Frontend build and browser/mobile visual verification remain pending; no API, database, Agent or production service was changed.\n\n### Control Panel specification implementation — 2026-11-20

- Unified the global navigation to Overview, Projects, Deployments, Logs, Devices and Settings; applied the Yevqo dark-green design tokens to the shared shell.
- Added a dedicated Overview route backed by existing project/device/site/deployment/log APIs. Unsupported active-job counts remain explicitly unavailable instead of being fabricated.
- Added global search across projects, devices, deployment records, logs and connected GitHub repositories; notification popover with read state; and profile dropdown.
- Added a guided Device Connect route and removed the unsafe use of the referral code as an Agent pairing token. The pairing token is issued from Profile and remains separate.
- Added project tabs for Overview, Deployments, Activity, Source Repository and Settings; added device tabs for Overview, Projects, Activity and Settings; added log filtering/export/details and deployment operation details.
- Added persisted workspace name, locale, timezone, date format, appearance, compact-mode and notification preferences with backend validation. Notification categories are filtered at the main workspace/device, scheduled task/backup, webhook and workspace invitation emitters.
- Added an official GitHub App installation page at `/settings/integrations/github`, server-side App credentials via `YEVQO_GITHUB_APP_ID`, `YEVQO_GITHUB_APP_SLUG` and `YEVQO_GITHUB_APP_PRIVATE_KEY`, plus optional `YEVQO_PUBLIC_URL` for managed push webhook registration, a short-lived one-time account-bound installation state, installation verification and repository listing. No App private key is returned to the browser. Setup guide: `docs/github-app.md`.
- The old `/integrations` page now redirects to the official GitHub App page.

**Remaining product work (not represented as complete):** configure the real GitHub App permissions (`Contents: Read`, `Pull requests: Read`, `Checks: Read`, optional `Administration: Write`) and set `YEVQO_PUBLIC_URL` to the Cloudflare Tunnel HTTPS origin; then verify installation, webhook delivery, private repository clone/build and cancel/redeploy/rollback on a real Android Agent. Physical mobile visual QA, fully translated legacy sub-pages and project/device relationship discovery API remain open. Additive migrations `024` and `025` have not been applied to the existing or production database; no production deployment was performed.

- Follow-up refinements: added timezone-aware date formatting using the saved IANA timezone/date format (with embedded tzdata for Android), initial unread-notification loading in the global header, category-grouped search results, notification priority labels, richer log filters (type, level, device, project and time range), and optional device attachment during project creation.
- CI verification: the `10d3ab1f` revision completed both the full Yevqo build and Pull Request checks successfully, including Go tests, Android ARM64 cross-compilation and frontend build. Later additive refinements are being checked by newer runs; verify the newest `main` run before release.


- 2026-11-21 — Standardized the Control Plane's shared-storage root to `/sdcard/yevqo`. Debian setup and source installation safely move the legacy `/sdcard/veltrix-control` directory only when the destination is absent or empty, refuse ambiguous dual-populated roots, and keep legacy environment-variable overrides working. The service runner and updater use the new path; Control Plane startup now honors the explicit data-root flag and remaps the legacy static directory. Go/shell/CI verification pending; no device data or live database was accessed.
- 2026-11-21 — Updated the Termux Control Plane service runner to load GitHub App ID/slug/public URL from protected `$PREFIX/etc/yevqo/github-app.env` and the RSA PEM from `$PREFIX/etc/yevqo/github-app.pem` on every service start. Setup creates a non-secret env template and preserves existing credentials; the PEM is not printed or exposed to the browser. Updated `docs/github-app.md`. CI and on-device configuration/installation verification remain pending; no database or production deployment was changed.

- 2026-11-21 — Fixed GitHub App installation callback verification on `main`: removed the browser `sessionStorage` equality gate, which could reject valid GitHub callbacks returned in a new tab/window. The frontend now passes GitHub's returned state to the authenticated Control Plane, where the existing short-lived, single-use, account-bound hash is validated and consumed. Missing callback state still fails closed. No database schema/data, credentials, Agent, or production service was changed; CI and live installation verification remain pending.

- 2026-11-21 — Improved GitHub App installation failure diagnostics on `main`: distinguish installation lookup from installation-token creation, report safe HTTP 400/422/5xx categories, and map DNS/TLS/timeout failures to actionable guidance without exposing credentials or raw provider details. Added unit cases for HTTP 422 and TLS errors. Automated tests/CI and live GitHub installation verification remain pending; no database, Agent or production service was changed.

- 2026-11-21 — Hardened Git branch validation on `main`: centralized validation for site Git settings, project repository targets, manual/redeploy flows and previews. It now rejects empty path components, hidden path components and `.lock` suffixes in addition to the existing unsafe-ref checks. Expanded unit cases; CI execution is pending. No migrations, live database, Agent or production service were changed. A follow-up source review also corrected the validator to keep all checks self-contained in production code.

- 2026-11-21 — Redesigned the authenticated Overview to follow the Yevqo public-site dashboard reference: compact green-accented status summary, infrastructure/resource panel, system-state list, device cards and site table. Replaced the old blue “Y.” placeholder with the three-stripe Yevqo mark in the shared app header and aligned the active global theme-token override with the documented dark-green palette. Resource telemetry is explicitly shown as waiting for Agent data rather than fabricated. UI-only change; API contracts, database and production services were not changed. Frontend build and browser/Android visual verification remain pending.

- 2026-11-21 — Expanded the Devices detail view and Agent command surface on `main`: added live CPU/RAM/disk/temperature refresh, private LAN IP (kept separate from Tailscale IP), battery/charging state where sysfs exposes it, OS and Agent uptime, installed-version inventory for PHP/Node/Git/cloudflared, listening TCP ports, a loopback TCP port probe, Control Plane command round-trip latency, and a private `$DATA_ROOT/logs/agent.log` (0600, rotate-on-start above 5 MiB and 200-line tail; `svlogtail` fallback). Added owner/admin Agent restart and updater scheduling actions; developer role is explicitly limited to read-only diagnostics and port checks. Agent update reports unavailable when the packaged `yevqo-update` command is absent; battery/temperature/log data remain capability-dependent. Added Agent port/diagnostics tests, command payload tests and RBAC matrix coverage. A prior CI run exposed a malformed branch-pattern declaration in `internal/control/site_git.go`; it has now been repaired on `main` (commit `1eadf51`). Go tests and Android ARM64 cross-compilation passed in PR check `44922912595`; the same run exposed a Svelte style-boundary error in the device detail page, now fixed in commit `c453d65`. Agent file logging was subsequently added with symlink checks and startup rotation; a missing data-root parameter was caught by CI and corrected. Fresh CI and physical Android verification remain pending. No database schema/data, Agent pairing credentials or production services were changed.

- 2026-11-21 — Fixed the Projects creation form on `main`: the optional device selector now lists only devices belonging to the selected manageable workspace, clears a stale selection when the workspace changes, and explains when no devices are available. This prevents a predictable cross-workspace attach failure while preserving server-side ownership checks. Also corrected the Projects document title to Yevqo. No backend/API, database, project records, Agent or production services were changed. Frontend CI/build verification remains pending.

- 2026-11-21 — Simplified the device detail Overview on `main`: removed the embedded site inventory, site create/start/stop/Git/log/delete controls and their modals from the device page. Site management remains in the dedicated Sites section; no site records, files, Agent commands, database schema/data or production services were changed. Frontend build and browser verification are pending.
- 2026-11-22 — Improved project detail load-failure handling on `main`: `/projects/:id` no longer silently redirects to the Projects list when `GET /api/projects/:id` fails. The page now preserves the route, displays the API error and HTTP status when available, and offers an in-place retry. This exposes the backend response needed to identify the remaining cause instead of hiding it. No API/backend, database schema/data, Agent or production service was changed. Frontend build and live API response verification remain pending; no production deployment was performed.

- 2026-11-22 — Reworked the authenticated Dashboard visual system on main using the Dokploy UI research: compact Yevqo-token shell, restrained surface hierarchy, denser tables and cards, consistent form controls, calmer operational Overview and responsive mobile spacing. Removed decorative Overview gradients/graphics and misleading static Control Plane active messaging. Shared styles preserve the selectable light appearance, existing routes, API calls and interactions. UI-only; no backend, API, database, Agent, user data or production service changed. Frontend build and browser/Android visual verification remain pending.

- 2026-11-22 — Replaced the authenticated Dashboard's green accent palette on `main` with graphite surfaces and cool-blue accents. Updated centralized CSS tokens and matching hard-coded accent/surface references, keeping the selectable light theme and existing semantic danger/warning styling. Updated this design-system specification and UX ledger. UI-only; frontend build and browser/Android visual verification remain pending; no backend, database, Agent or production deployment was changed.

- 2026-11-22 — Expanded the authenticated Dashboard on `main`: replaced the `/domains` redirect placeholder with a workspace domain inventory using the existing API, added a global `/schedules` page for real per-site Agent tasks (create, enable/disable, delete, next-run and last-result states), grouped sidebar navigation across Operations, Infrastructure and Management, and normalized remaining dark palette text/accent tokens. Existing APIs and data contracts were reused; no database migration or production deployment. Full frontend build, cross-page visual QA and Android verification remain pending.

- 2026-11-22 — Continued the Dashboard-wide visual pass on `main`: aligned project detail, project list, Settings, GitHub App, Devices, Sites, Databases, Backups, Builds, Monitoring, Notifications, Reports and Maintenance page accents with the graphite/cool-blue palette; the legacy root dashboard now redirects to the canonical Overview. Added Applications to the grouped navigation. No API, database, Agent or production deployment changes. Build verification is blocked in this environment because GitHub DNS resolution fails; GitHub Actions currently reports no workflow runs for this revision. Browser and physical Android QA remain pending.

- 2026-11-22 — Further completed Dashboard page work on `main`: Reports now uses Yevqo branding and a working project selector that scopes both report summaries and audit events; Maintenance was expanded into an Agent inventory with heartbeat/version/status context and per-device diagnostics links. The global root route redirects to the canonical Overview. No backend, API, database, Agent or production service was changed. Frontend build and browser/Android visual QA remain pending because this execution environment cannot resolve `github.com` and GitHub reports no workflow runs for the latest commits.

- 2026-11-23 — Continued the authenticated Dashboard audit on `main`: removed the static “Control plane” connectivity indicator (it did not represent a verified connection), replaced remaining shared-shell green brand accents with cool blue, and aligned navigation metadata colors. Deployment history now labels records without an explicit terminal result as `Recorded` rather than falsely reporting success, and its Failed filter includes only confirmed failures. Deployment and Profile surfaces now use the graphite/cool-blue tokens. UI-only; no API, backend, database, Agent, user data or production service changed. Frontend build, browser/mobile QA and accessibility checks remain pending.

- 2026-11-23 — Audited the Projects list, project detail and per-environment detail on `main`. Fixed a runtime failure in the environment page: its eight parallel API results were destructured into seven variables, leaving environment variables undefined; the route now binds the eighth result and initializes the shared locale used by its UI. Replaced the obsolete disabled “Import from GitHub · Coming soon” control with accurate guidance to connect a repository from the implemented project Source Repository tab after creation. Repaired literal `\\n` sequences that had leaked into the Projects page CSS, and aligned project list/detail/environment surfaces with the graphite/cool-blue design tokens, including removal of undefined text/background tokens and the last green fallback. No API, backend, database, Agent, user data or production services changed. Frontend build, browser/mobile QA and accessibility verification remain pending; GitHub Actions currently has no workflow runs for `main`.
- 2026-11-23 — Continued the frontend-only audit on `main`: authenticated sessions are now redirected from both Login and Register to the canonical Overview, and successful auth flows navigate there directly. Corrected the remaining canonical CSS token block that still gave the sidebar/elevated/hover surfaces a green tint despite the graphite/cool-blue design direction. No API, backend, database, Agent, project data or production service was changed. Frontend build, browser/mobile QA and accessibility verification remain pending.
- 2026-11-23 — Requested verification for the frontend-only audit: the repository CI workflow runs `npm install` and `npm run build` in `frontend` on every push to `main`, alongside Go tests and Android ARM64 cross-compilation. A local checkout/build attempt in the execution environment was blocked because DNS could not resolve `github.com`; CI verification is therefore pending on the resulting `main` push. No production deployment or database change.

- 2026-11-26 — Fixed the versioned Android package release path on `main`: the build artifact now includes `SHA256SUMS`, and version tags (`v*`) publish both ARM64 Debian packages plus checksums to `NexLab-ceo/veltrix-dock`, the release repository already consumed by the Termux updaters. Existing releases are updated with `--clobber`; main-branch builds remain artifacts only. Publishing requires the repository secret `PUBLIC_RELEASE_TOKEN` with Contents write access to `NexLab-ceo/veltrix-dock`. Workflow execution, secret availability and an actual tagged release still require verification; no production deployment or Android device was changed.
