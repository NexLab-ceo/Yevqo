# VELTRIX Build Engine — implementation foundation

The Agent provides deterministic project analysis and reports which build tools are present on the Agent host.

## Detection contract

1. Detection is read-only. It never executes repository scripts or follows symlinks.
2. Explicit `veltrix.yaml` / `veltrix.yml` configuration takes precedence.
3. A Dockerfile takes precedence over language heuristics.
4. Nixpacks configuration is recognized before language heuristics.
5. Known language markers produce a BuildPlan with suggested commands and evidence.
6. Unknown projects return a valid plan with `needs_configuration=true`; they are not treated as errors.

## Supported initial markers

Node.js (npm/pnpm/yarn/Bun), Python, Rust, Go, Java/Maven, JVM/Gradle, Ruby, PHP/Composer, .NET, Dockerfile, Nixpacks and static HTML.

## Host capability preflight

Every BuildPlan now includes a `capabilities` object. The Agent checks PATH for Docker, Podman, Nixpacks and Git. This is deliberately only an executable-presence check: it does not prove that a container daemon is running, that permissions are sufficient, or that storage/network access is available. Those checks belong immediately before a build starts.

Dockerfile builds are marked available when Docker or Podman CLI is present. Nixpacks additionally requires the Nixpacks executable. The result includes warnings when a provider cannot run.

## Execution and security boundary

No project build command is executed by this feature. Custom commands are explicitly marked as not isolated and must remain disabled until a resource-limited sandbox exists. Dockerfile and Nixpacks builds must run through a container backend; they must never fall back to executing project scripts directly on the Agent host. The next implementation stage is a build-job runner with timeouts, bounded logs, cancellation, resource limits, and secret redaction.

## Future compatibility

BuildPlan is provider-neutral so internal providers and future third-party plugins can implement the same contract without changing detection callers.
