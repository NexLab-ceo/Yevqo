# VELTRIX Build Engine — Android / Termux

The Agent Build Engine is Android-only. It does not require root, Docker, Podman,
Nixpacks, a remote build server, or a Linux server. Container markers may be
detected for explanation but are not executable providers.

## Components
- Analyzer: read-only project detection and veltrix.yaml configuration.
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
Actual compatibility depends on device architecture and installed Termux packages.

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
