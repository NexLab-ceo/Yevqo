# Yevqo Build Engine — Android / Termux

The Agent Build Engine is Android-only. It does not require root, Docker, Podman,
Nixpacks, a remote build server, or a Linux server. Container markers may be
detected for explanation but are not executable providers.

## Components
- Analyzer: read-only project detection plus the supported `veltrix.yaml` subset (`project.root`, `build.provider`, `build.install`, `build.command`, `run.command`, `run.port`).
- Android Environment Detector: Termux/Android, architecture, CPU count, installed
  tools, memory estimate and free disk estimate.
- Local Provider: invokes allowlisted Termux tools with argv; never a shell string.
- Process Supervisor: one build at a time, timeout/cancellation, bounded logs,
  sanitized environment.
- Artifact result: local project directory and build status.

## Local stacks
- Node.js: npm/pnpm/yarn/Bun install and build script.
- Python: pip install from requirements.txt or pyproject.toml.
- Go: go build -o veltrix-app .
- Rust: cargo build --release.
- PHP: Composer dependency installation.
- Static: no compilation step.
- Java/Maven (including `mvnw`), Gradle (including `gradlew`), Ruby/Bundler and .NET (`dotnet publish`) are executable when their toolchains are installed. The runner invokes fixed argv vectors and does not evaluate shell command strings.
A manifest install/build command is tokenized into argv (no shell expansion) and limited to an executable allowlist. Shell operators, substitutions and shell executables are rejected. Actual compatibility depends on device architecture and installed Termux packages.

## Security and Android limits
Project package scripts and compiler plugins execute with the Agent user's
permissions. The runner requires explicit allow_project_code=true; only build
trusted repositories. Path checks reject traversal and symlink escapes. Child
processes receive a small allowlisted environment, a timeout, and bounded logs.
Builds are serialized. These controls are not a sandbox.

Without root/kernel support, this implementation cannot enforce cgroup CPU/RAM
limits, isolate networking, provide a separate UID, or guarantee termination of
grandchild processes. Memory/disk checks are preflight estimates, not hard limits.

## Disabled by design
Dockerfile execution, Docker/Podman, Nixpacks, arbitrary custom shell commands,
remote builds, and claims of container-grade isolation are disabled.
Unsupported stacks return explicit errors rather than falling back to host shell.


## Git deployment releases

The Agent checks out each commit into an immutable
`sites/<site>/.yevqo/releases/<commit>` directory. It does not run
`git pull` inside the active working tree or delete starter files. The selected
project root is resolved inside the staged release, then the detected build
provider runs with a timeout and bounded log.

The activation path supports `php`, `go`, `static` and `nodejs` runtimes.
Node.js uses npm, pnpm, Yarn or Bun based on the lockfile, runs the project's
`start` script without invoking a shell, and receives `HOST=127.0.0.1` and
the configured `PORT`. All project scripts execute with Agent-user privileges
and require explicit project-code consent.

The Agent exposes release history from immutable release directories, obtains
commit timestamps from Git metadata, and marks the currently active release
using its durable process registry. Rollback activates a selected existing
release without rebuilding it. The Agent validates the commit and release path,
runs the same readiness check as deployment, and attempts to restore the prior
process if activation fails. Release history remains on the device and requires
no database schema migration.

Git deployment accepts HTTPS remotes without embedded credentials and SSH
remotes (including the standard `git@` user). A saved access token is passed to
Git through temporary process environment configuration, not embedded in the
remote URL. Branch names are validated before invoking Git. Build output is
bounded; custom shell build commands remain disabled.
