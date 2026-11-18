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
