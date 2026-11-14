# VELTRIX Build Engine — implementation foundation

The first implementation slice adds a deterministic project analyzer to the Agent.

## Detection contract

1. Detection is read-only. It never executes repository scripts or follows symlinks.
2. Explicit `veltrix.yaml` / `veltrix.yml` configuration takes precedence.
3. A Dockerfile takes precedence over language heuristics.
4. Nixpacks configuration is recognized before language heuristics.
5. Known language markers produce a BuildPlan with suggested commands and evidence.
6. Unknown projects return a valid plan with `needs_configuration=true`; they are not treated as errors.

## Supported initial markers

Node.js (npm/pnpm/yarn/Bun), Python, Rust, Go, Java/Maven, JVM/Gradle, Ruby, PHP/Composer, .NET, Dockerfile, Nixpacks and static HTML.

## Execution boundary

This analyzer only proposes a plan. It does not execute install/build/start commands. Command execution must be added through a separate, resource-limited build runner with explicit provider selection and safe secret/log handling. Dockerfile and Nixpacks execution require an available container/build backend; they must not silently fall back to executing Dockerfile contents on the host.

## Future compatibility

BuildPlan is intentionally provider-neutral so internal providers and future third-party plugins can implement the same contract without changing detection callers.
