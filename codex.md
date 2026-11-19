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
| P0 — Android package build and CI | DONE | Verified on code head a96d98e: Go tests, Android ARM64 cross-compilation, frontend build, Debian package assembly and metadata checks passed (build run 44370256506); PR checks passed (run 44370256837). Installer hardening after that run is queued for fresh CI verification. Physical Android startup is tracked separately under P3. |
| P0 — Device pairing and one-device Free limit | VERIFY | Preserve one connected device per Free account; reconnect same device; reject a second device. |
| P1 — Project/site lifecycle | PARTIAL | Per-site Environment tab supports encrypted custom variables, masked listing, add/update/delete, search and .env bulk import; a shared set currently applies to Build and Runtime. Built-in runtimes plus Custom Language runtime; arbitrary installed Termux tools and in-site executables can be launched with explicit shell-free start commands. Full end-to-end runtime coverage remains open. |
| P1 — Git deployments, immutable releases, rollback | PARTIAL | Deployment/release flow is documented; verify edge cases and current test coverage. |
| P1 — Domains and public routing | PARTIAL | Domain metadata and tunnel-related work exist; do not claim public routing is universally operational without end-to-end verification. |
| P2 — Device monitoring telemetry | PARTIAL | Heartbeat and typed metrics are documented; metric availability depends on Android/device support. |
| P2 — Database management | PARTIAL | PostgreSQL lifecycle and database connections are documented; MySQL/MariaDB, restore/export, and other items remain incomplete unless current code proves otherwise. |
| P2 — Website/database backups and restore | PARTIAL | Website backup creation is documented; restore and remaining backup operations require verification/implementation. |
| P2 — Build providers and templates | PARTIAL | Automatic detection covers Node, Python, Go, Rust, PHP, Java/JVM, .NET, Ruby, C/C++/Make/CMake, Swift, Elixir, Scala, Zig, Dart, Deno, Erlang, Haskell, OCaml, Crystal, Perl, Nim and Lua; universal argv commands support other installed toolchains. Persistent language caches, configurable per-site cache paths, build cancellation/history, custom runtime and seven starters are implemented. `yevqo.yml` now wires environment injection, HTTP healthchecks with ongoing supervision/recovery, include/exclude artifact collection (including `build.output` defaults), memory preflight, deploy restart policy, project root/start command and cache paths into build/runtime/deploy flows. Artifact listing/chunk-download API and Build Engine UI are now implemented with path checks and bounded transfers. General-purpose YAML compatibility and physical Termux/toolchain validation remain open. Remote Linux executor remains outside Android-only scope. |
| P2 — CLI and preview deployments | VERIFY | Check current commands, authorization, lifecycle, and Android-only limitations before updating. |
| P2 — Team, RBAC, audit | VERIFY | Check endpoint coverage and enforcement in the API, not only UI visibility. |
| P1 — Project workspaces and environments | PARTIAL | Existing projects group devices/sites; additive migration 021 adds Production/Development/Staging environments and environment-site assignments, with authenticated environment list/create/assignment API. Project-scoped database/build/deployment filtering and full nested navigation remain open. |\n| P2 — AI Agents / Agent Builder | TODO | Do not imply real agent execution until model integration, permission enforcement, and execution lifecycle are implemented. |
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
