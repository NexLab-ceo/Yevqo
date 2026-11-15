# VELTRIX Dock — Dokploy parity roadmap

This roadmap tracks useful Dokploy capabilities adapted to Android/Termux. It does not promise Docker support on Termux.

## P0 — Reliable app lifecycle
- [ ] Persistent process registry (site ID, PID, process group, runtime, port, working directory, start time, restart policy).
- [ ] Safe PID identity checks after Agent restart; avoid killing unrelated PID reuse.
- [ ] Process-group termination (SIGTERM, grace period, SIGKILL fallback).
- [ ] Startup readiness probe (TCP/HTTP) with timeout and useful failure reason.
- [ ] Process exit watcher and accurate running/stopped/crashed status.
- [ ] Optional restart policy (never / on-failure / always) with bounded backoff.
- [ ] Per-app stdout/stderr logs, rotation, timestamps, live streaming and log filters.
- [ ] Deployment job state machine: queued, preparing, cloning, installing, building, starting, healthy, failed, cancelled.
- [ ] One job per app by default; cancellation and progress events.
- [ ] Node.js runtime (npm/pnpm/yarn/bun detection), PHP, Go, static and custom start commands.
- [ ] Deploy = fetch/clone -> install -> build -> start -> health-check -> record release.
- [ ] Tests for process crash, agent restart, occupied port, stop timeout, failed build, health-check timeout.

## P1 — Deployment UX and recovery
- [ ] Environment editor with encrypted storage and scoped injection.
- [ ] Release history with commit, build output, runtime, timestamp and status.
- [ ] Rollback to previous successful release (immutable release directories + atomic current pointer).
- [ ] GitHub webhook deployment and branch/commit selection.
- [ ] Scheduled tasks and deployment notifications.
- [ ] Resource telemetry per app (CPU/RAM where Android exposes it; no fake container metrics).
- [ ] Cloudflare API for DNS/Public Hostname automation.
- [ ] Database backup scheduling, retention and restore workflow.

## P2 — Platform capabilities
- [ ] App templates and one-click starters.
- [ ] Preview deployments and staging environments.
- [ ] Build plugins/providers.
- [ ] Remote Linux build executor for Dockerfile/Nixpacks/Railpack.
- [ ] Multi-device orchestration and per-device capabilities.
- [ ] Granular team permissions, audit log and API/CLI polish.

## Process model (Termux)
Use a native supervisor rather than Docker:
- Agent owns a registry and watches child processes.
- Each app gets an isolated working directory, environment, process group and log stream.
- PID is only an implementation detail; status comes from supervisor state + OS process check + optional health probe.
- Agent restart reconciles registry with process identity; never trusts PID alone.
- Logs are persisted locally and streamed to the control panel over Agent events.
- Android battery/background restrictions must be surfaced as a limitation; a process cannot be promised to survive OS force-stop or device reboot without appropriate setup.

## Comparison test plan
Use the same small public Git app, same branch, same build command and same health endpoint:
1. Dokploy: create app -> connect Git -> deploy -> inspect logs -> stop/start -> redeploy.
2. VELTRIX: add device -> create app -> connect Git -> deploy -> inspect live logs -> stop/start -> redeploy.
Measure: time to first healthy response, number of manual steps, failed/ambiguous states, log clarity, recovery time and user-reported friction.
Record screenshots, timings and notes; do not claim a winner before a real run on both systems.
