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
| P0 — Go Control Plane and Agent foundation | PARTIAL | Core services and Android/Termux packaging are present; re-check full install/upgrade and device runtime coverage. |
| P0 — Android package build and CI | DONE | Full build and PR checks passed at `99bce1e`: Go tests, Android ARM64 cross-compilation, frontend build, setup/updater validation, all-in-one and standalone package assembly/verification. Physical Android startup remains tracked separately under P3. |
| P0 — Device pairing and one-device Free limit | DONE | `UpsertDevice` serializes registration per account, preserves same-device reconnect, rejects cross-account UUID rebinding, and enforces the Free one-device cap regardless of referral bonuses. Unit tests cover the Free cap; Hub tests cover replacement sockets and pending-command shutdown. Physical Android pairing remains under P3. |
| P1 — Project/site lifecycle | PARTIAL | Per-site Environment tab supports encrypted custom variables, masked listing, add/update/delete, search and .env bulk import; a shared set currently applies to Build and Runtime. Built-in runtimes plus Custom Language runtime; arbitrary installed Termux tools and in-site executables can be launched with explicit shell-free start commands. Full end-to-end runtime coverage remains open. |
| P1 — Git deployments, immutable releases, rollback | PARTIAL | Project-scoped deployment history is enforced and exposed in the dashboard. Release/rollback edge cases and full Agent failure-path coverage remain open. |
| P1 — Domains and public routing | PARTIAL | Domain metadata and tunnel-related work exist; do not claim public routing is universally operational without end-to-end verification. |
| P2 — Device monitoring telemetry | PARTIAL | Heartbeat, typed metrics and project-scoped monitoring/report queries are implemented. CPU, memory, storage and temperature alerts now use persisted state, threshold/recovery hysteresis and deduplicated transition notifications; Agent disconnect/reconnect transitions are also notified. Metric availability depends on Android/device support; physical-device validation remains open. |
| P2 — Database management | PARTIAL | Workspace-owner authorization covers database-server inventory/create/install/start/stop/delete, role/privilege operations, database create/read/stats and site connections. Environment-scoped database-instance assignments, safe legacy backfill and runtime credential injection for start/deploy/rollback/webhooks are implemented. Managed MySQL/MariaDB lifecycle, export and end-to-end integration tests remain open. |
| P2 — Website/database backups and restore | PARTIAL | Workspace-scoped backup create/list, database/site restore, chunk download, transfer and schedules are implemented with owner/actor notifications and audit outcomes. Database restore attaches the new instance to the selected project environment only after Agent success. Detaching database servers from projects/environments clears associated membership records without deleting databases. End-to-end Agent verification, retention tests and integration coverage remain open. |
| P2 — Build providers and templates | PARTIAL | Automatic detection covers Node, Python, Go, Rust, PHP, Java/JVM, .NET, Ruby, C/C++/Make/CMake, Swift, Elixir, Scala, Zig, Dart, Deno, Erlang, Haskell, OCaml, Crystal, Perl, Nim and Lua; universal argv commands support other installed toolchains. Persistent language caches, configurable per-site cache paths, build cancellation/history, custom runtime and seven starters are implemented. `yevqo.yml` now wires environment injection, HTTP healthchecks with ongoing supervision/recovery, include/exclude artifact collection (including `build.output` defaults), memory preflight, deploy restart policy, project root/start command and cache paths into build/runtime/deploy flows. Artifact listing/chunk-download API and Build Engine UI are now implemented with path checks and bounded transfers. General-purpose YAML compatibility and physical Termux/toolchain validation remain open. Remote Linux executor remains outside Android-only scope. |
| P2 — CLI and preview deployments | PARTIAL | `yevqo-cli` supports workspace inventory, project/environment inspection and create/update/delete lifecycle commands, site deploy/start/stop/restart/logs, database/backup inventory and site/database backup queueing, and preview list/create/delete. Site deploy and preview creation require explicit `--allow-project-code` consent; destructive project/environment/preview deletion requires `--confirm`. Preview create/delete and 24-hour expiry cleanup with retry-until-Agent-online are implemented; cancellation-safe cleanup and key/port tests are added. Real Android command verification remains open. |
| P2 — Team, RBAC, audit | PARTIAL | Server-side workspace roles cover project/device/site/database/backup actions. Owners and Admins can switch workspaces, invite/remove members and update active member roles; Developers cannot manage membership or databases/backups. Full endpoint matrix and audit coverage remain open. |
| P2 — Logs, notifications, analytics and reports | PARTIAL | Project-aware logs, deployment history, monitoring, domains and reports are wired through the dashboard; Security Report reads the append-only audit log and supports project filtering. Infrastructure Summary now includes period-scoped build success/failure and backup completion/failure counts, with project filtering. Workspace notifications cover deployment, database lifecycle, backup/restore, metric thresholds and Agent connectivity transitions. Deeper resource analytics and complete audit event coverage remain open. |
| P1 — Project workspaces and environments | PARTIAL | All work is on `main`. Owners/Admins can create/manage projects across workspaces; resources support safe attach/detach; environments group applications and database servers, with environment-filtered database inventory/creation. Production protections are server-side. Runtime isolation, migration/lifecycle integration tests and physical DB verification remain open. |
| P2 — AI Agents / Agent Builder | TODO | Do not imply real agent execution until model integration, permission enforcement, and execution lifecycle are implemented. |
| P3 — Full Android device validation | TODO | Requires tests on a real supported Android + Termux device; CI cross-compilation is not a substitute. |
| P3 — Release readiness | TODO | Requires a verified end-to-end checklist, upgrade checks, security review, and documented limitations. |

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

### Progress history

- 2026-11-20 — Completed CLI project/environment lifecycle and preview commands with explicit consent/confirmation; Android ARM64, frontend, Go tests and both packages passed (build run 44740854997; checks run 44740854986; commit `41699cf`).
- 2026-11-20 — Added Cloudflare remotely-managed Tunnel ingress and DNS route API, including preserved existing ingress rules and safe route removal. Cloudflare account and real Android validation pending.

- 2026-11-20 — Added CLI preview listing, creation and deletion. Preview creation requires explicit code consent; deletion requires `--confirm`; creation output includes the preview key/URL/port. Tests cover all three API workflows. CI pending.

- 2026-11-20 — Added project and environment create/update/delete commands to `yevqo-cli`; destructive commands require `--confirm`, and server-side Production protections remain authoritative. Added command/API tests and documented lifecycle commands (implementation commit `e319e39`).

- 2026-11-20 — Extended `yevqo-cli` with authenticated project/environment create, update and delete workflows; delete commands require `--confirm` and preserve app/database data. Added API-method and command tests; CI pending.

- 2026-11-20 — CLI deploy now refuses to run without the explicit `--allow-project-code` flag; the flag is passed to the Control Plane and covered by tests. Updated `docs/cli.md`. Go tests passed; full CI run 44740248528 / 44740248717 in progress (commit `36e02b1`).

- 2026-11-20 — Expanded Infrastructure Summary with 7/30/90-day build and backup success/failure analytics and project-scoped filtering. Go tests and Android ARM64 cross-compilation passed; frontend/package CI was still running when the CLI hardening task started (implementation commit `ef18bb7`).
- 2026-11-20 — CLI deploy now requires explicit `--allow-project-code` consent; tests verify the flag is required and transmitted. Go tests passed; full Android/frontend/package CI pending.

- 2026-11-20 — Hardened preview creation/deletion against client cancellation, retained the 24-hour expiry cleanup worker, and added key/port/error tests. Go tests, Android ARM64 builds, frontend build and both package verifications passed (build run 44739751259; checks run 44739751260; commit `f4f77d6`).
- 2026-11-20 — Added period-scoped build and backup analytics to Infrastructure Summary, including project filtering and success/failure counts. CI pending.

- 2026-11-20 — Added persisted CPU/RAM/storage/temperature threshold alerts and deduplicated Agent offline/reconnect notifications. Go tests, Android ARM64 builds, frontend build, package assembly and package verification passed (build run 44739395185; checks run 44739395193; commit `7c44984`).
- 2026-11-20 — Hardened preview lifecycle cleanup so client cancellation cannot leave a live preview after failed metadata updates; added key, port-range and error-truncation tests. CI pending.

- 2026-11-20 — Verified device metric alerts and added deduplicated offline/reconnect notifications using the same persisted alert-state table. Go tests, Android ARM64 build, frontend build and both package verifications passed for metric alerts (build run 44738921259; checks run 44738921266; commit `7c44984`). Connectivity notification CI pending.

- 2026-11-20 — Added persisted device alert state with CPU/memory/storage/temperature thresholds, recovery hysteresis and transition-only notifications. Metric ingestion from the Agent WebSocket now evaluates alerts without failing telemetry ingestion. CI pending.

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


- 2026-11-18 — Expanded the existing per-site Environment tab toward the Dokploy-style workflow: clearer variable management, search, add/update/delete, .env bulk import with partial-failure reporting, and explicit managed-database variable separation. Values remain encrypted and are never returned to the browser; separate Build/Runtime scopes remain future work.\n- 2026-11-19 — Verified the Environment changes on current main: PR checks passed (run 44370256837); Go tests, Android ARM64 cross-compilation, frontend build, Debian package assembly and package metadata checks passed (build run 44370256506, head a96d98ee5a5eaa62d35dcbb829893cf09a10e73f). Added docs/first-test.md with a clean-device Android/Termux smoke-test procedure. This is CI verification only; no physical-device test has been performed yet.
\n- 2026-11-19 — Started Dokploy-style project workspaces: added additive migration `021_project_environments.sql` with Production, Development and Staging defaults plus project/environment site and database-server association tables; added authenticated environment list/create/site-assignment API and client methods; project detail now displays and creates environments. Full project isolation for database inventory, Build Engine, Deployments, navigation, and end-to-end tests remains in progress. No production deployment or database migration has been run from this change.\n
- 2026-11-19 — Project workspace continuation on `feature/project-workspaces`: project sidebar now scopes Infrastructure (Devices, Applications, Databases) and Delivery (Build Engine, Deployments); project-aware list endpoints filter devices, applications, database servers and deployment history; new projects receive Production/Development/Staging; environment detail page supports assigning project applications; database servers can be assigned to projects; global Domains route now redirects to Applications and domain/tunnel-token settings are available on each site's Domains tab. Changes are not merged or deployed. Automated checks and end-to-end verification are still required; database instances, backups, monitoring/log scoping, and complete environment lifecycle remain to finish.

- 2026-11-19 — Continued project workspace hardening on `feature/project-workspaces`: fixed duplicate Svelte `page` import that broke frontend compilation; added authenticated environment update/delete endpoints, transactionally removing environment-site assignments while preserving sites, and server-side Production protection; added environment settings UI and API helpers. Latest Go tests/Android cross-compile were green in the failing CI run; frontend failure was the duplicate import. New CI runs are pending. Database CRUD isolation, broad resource authorization tests, and migration integration tests remain in progress.


- 2026-11-19 — Verification and database project-scope follow-up: fixed duplicate Svelte `$app/stores` import in `frontend/src/routes/sites/+page.svelte`; GitHub Actions run 44535626760 passed Go tests, Android ARM64 cross-compilation, dependency installation and frontend production build. Added project-aware database instance listing and preserved project context through database inventory/create navigation; database provisioning now rejects a server not attached to the authenticated project. These are CI-verified code changes only; no database migration or production deployment was run. Remaining: complete project-aware authorization for database detail/lifecycle/role/privilege/connection/backup operations, add migration/resource-assignment integration tests, and verify the latest follow-up CI runs.


- 2026-11-19 — Yevqo rename landed directly on `main`: Termux executables, runit service names, setup/update commands, Debian package identifiers/artifact names, systemd runner filenames, boot hook (`00-yevqo`), build workflow, and user documentation use Yevqo. Legacy `/sdcard/veltrix-control` and `$PREFIX/var/lib/veltrix-agent` data/config locations remain unchanged to preserve existing installations. Agent setup migrates the old boot-hook filename without discarding its contents. Database names and Go module/import paths remain unchanged for compatibility. CI verification is pending.


- 2026-11-19 — Standalone Agent package simplified: package name `yevqo`, executable/service `yevqo`, setup `yevqo-setup`, updater `yevqo-update`. All-in-one keeps `yevqo-agent` to distinguish its Agent from Control Plane; deploy helper CLI becomes `yevqo-cli` to avoid a binary-name collision. Legacy data/config locations remain intact. Standalone `yevqo` resolves the private Agent config written by setup while retaining the legacy config fallback. CI validation required.

- 2026-11-19 — The source-based Agent installer and the dashboard's device-connection instructions now use the standalone `yevqo` service name too. The all-in-one packaged setup retains `yevqo-agent`.
- 2026-11-20 — Continued non-AI platform work on `main`: completed workspace-scoped database/backup operations, safe project resource attach/detach, project and environment database assignments, environment-filtered database creation, workspace switching and Admin team membership/role management. Added deployment/database/backup/restore notifications with actor attribution, project/workspace audit events and failure outcome classification; expanded `yevqo-cli`; corrected Security Reports to use project-filtered audit logs; and hardened Agent reconnect/shutdown behavior. Full build and PR checks passed at `99bce1e`; websocket reconnect tests passed at `041779a`. Only documentation commits followed that verified code head. Additive migrations `021` and `022` were added but not applied. No production deployment or existing DB/data changes were made.
- 2026-11-20 — Fixed the latest `main` CI blocker in `internal/control/modules.go`: failed project-environment initialization now rolls back the newly created project using the authenticated owner ID (`u.ID`), rather than an out-of-scope identifier. The failure was confirmed in both build and verification logs at `65854d4`; fresh CI for `5556517` is required before treating the build as green. No database migration, production deployment, or existing-data change was performed.
