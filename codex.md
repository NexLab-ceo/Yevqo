# Codex Instructions — Yevqo

## Product identity

Yevqo is an Android-first, self-hosted application platform built around **Android + Termux**. A phone or tablet runs the Yevqo Agent (and optionally the Control Plane); the web interface manages projects and device-side workloads.

- Product name: **Yevqo**.
- The repository/module and some legacy package, service, and data-directory names still use `veltrix` for upgrade compatibility. Do not rename these casually.
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
| P0 — Android package build and CI | VERIFY | CI/package workflows exist; verify latest successful runs before claiming current green status. |
| P0 — Device pairing and one-device Free limit | VERIFY | Preserve one connected device per Free account; reconnect same device; reject a second device. |
| P1 — Project/site lifecycle | PARTIAL | Built-in runtimes plus Custom Language runtime; arbitrary installed Termux tools and in-site executables can be launched with explicit shell-free start commands. Full end-to-end runtime coverage remains open. |
| P1 — Git deployments, immutable releases, rollback | PARTIAL | Deployment/release flow is documented; verify edge cases and current test coverage. |
| P1 — Domains and public routing | PARTIAL | Domain metadata and tunnel-related work exist; do not claim public routing is universally operational without end-to-end verification. |
| P2 — Device monitoring telemetry | PARTIAL | Heartbeat and typed metrics are documented; metric availability depends on Android/device support. |
| P2 — Database management | PARTIAL | PostgreSQL lifecycle and database connections are documented; MySQL/MariaDB, restore/export, and other items remain incomplete unless current code proves otherwise. |
| P2 — Website/database backups and restore | PARTIAL | Website backup creation is documented; restore and remaining backup operations require verification/implementation. |
| P2 — Build providers and templates | PARTIAL | Automatic detection covers Node, Python, Go, Rust, PHP, Java/JVM, .NET, Ruby, C/C++/Make/CMake, Swift, Elixir, Scala, Zig, Dart, Deno, Erlang, Haskell, OCaml, Crystal, Perl, Nim and Lua; universal argv commands support other installed toolchains. Persistent caches, process-group cancellation, custom runtime and seven starters are implemented; CI pending. Physical Termux/toolchain compatibility and Build History/Artifact Manager remain to verify/finish. Remote Linux executor remains outside Android-only scope. |
| P2 — CLI and preview deployments | VERIFY | Check current commands, authorization, lifecycle, and Android-only limitations before updating. |
| P2 — Team, RBAC, audit | VERIFY | Check endpoint coverage and enforcement in the API, not only UI visibility. |
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

- 2026-11-18 — Added the initial Codex project instructions and established the P0–P3 progress ledger. This is a tracking baseline, not a claim that all listed areas have been re-verified.
- 2026-11-18 — Expanded first-party App Templates with PHP and Node.js JSON API starters, stable template-ID selection in the device UI, regression tests, and updated template documentation. Pull request checks passed (run 44344232085); Android build/package workflow passed, including Go tests, ARM64 cross-compilation, frontend build, and both Debian package verifications (run 44344231782).
- 2026-11-18 — Expanded Yevqo Engine with broader language detection, universal shell-free argv commands for installed Termux toolchains and project-local executables, persistent per-language caches, build process-group cancellation, and a Custom Language site runtime that can deploy with an explicit start command. Added regression tests and updated Build Engine/template docs; CI validation pending.

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
