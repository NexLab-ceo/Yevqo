# Yevqo — Dokploy parity roadmap

This roadmap tracks useful Dokploy capabilities adapted to Android/Termux. It does not promise Docker support on Termux.

## P0 — Reliable app lifecycle

- [x] Durable process registry (site ID, PID, process group, runtime, port, working directory, start time and process identity token).
- [x] Safe PID identity checks after Agent restart; never signal a reused PID.
- [x] Process-group termination (SIGTERM, grace period, SIGKILL fallback).
- [x] Process exit watcher, child reaping, user-visible crash status and exit reason.
- [x] Persistent per-site restart policies: `never`, `on-failure`, `always`; bounded exponential backoff (2s/4s/8s, three automatic attempts).
- [x] Startup TCP readiness probe with timeout and actionable failure reason.
- [x] Optional HTTP 2xx health check for immutable Git deployments; redirects are not treated as healthy.
- [x] Per-site stdout/stderr capture, bounded log rotation, live streaming, timestamps in the live viewer, search and severity filters.
- [x] Deployment job lifecycle: queued, preparing, cloning, installing, building, starting, healthy, failed and cancelled.
- [x] One active deployment per site, progress events and cancellation.
- [x] Supported launch runtimes: PHP, Go, static and Node.js; custom argv start commands are supported without shell interpolation.
- [x] Immutable staged Git deployment, build-before-activation and restoration of the previous release when candidate activation/health checks fail.
- [ ] Complete failure-injection coverage for failed builds, cancellation at every deployment phase, Agent restart during recovery, occupied ports and stop timeouts.

### P0 implementation notes

- Restart attempts are intentionally bounded to avoid an endless crash loop. After the automatic retry budget is exhausted, manual intervention is required.
- A successful TCP readiness check is the default. Configure an HTTP health-check path when the application exposes a health endpoint.
- Logs are local to the device. The live viewer timestamps received lines; timestamps are not injected into the application's original stdout/stderr bytes.
- Deployment job events are streamed to the Control Plane. The durable job record is retained on the Agent for local recovery/diagnostics; it is not a cross-device job-history database.
- Android battery optimization, force-stop and OS process reclamation can still terminate workloads outside the Agent's control. Use the supported boot/background setup where appropriate.

## P1 — Deployment UX and recovery

- [ ] Environment editor with encrypted storage and scoped injection.
- [x] Release history with commit, runtime, timestamp and active status.
- [x] Rollback to a previous immutable release without rebuilding.
- [ ] GitHub webhook deployment and branch/commit selection.
- [ ] Scheduled tasks and deployment notifications.
- [ ] Resource telemetry per app (CPU/RAM where Android exposes it; no fake container metrics).
- [ ] Cloudflare API for DNS/Public Hostname automation.
- [ ] Website backup restore UI, database backup scheduling and cross-device transfer.

## P2 — Platform capabilities

- [ ] App templates and one-click starters.
- [ ] Preview deployments and staging environments.
- [ ] Build plugins/providers.
- [ ] Remote Linux build executor for Dockerfile/Nixpacks/Railpack.
- [ ] Multi-device orchestration and per-device capabilities.
- [ ] Granular team permissions, audit log and API/CLI polish.

## Process model (Termux)

Use a native supervisor rather than Docker:

- Agent owns a durable registry and watches child processes.
- Each app gets an isolated working directory, environment, process group and log stream.
- PID is only an implementation detail; status comes from supervisor state, OS process identity and optional health probes.
- Agent restart reconciles the registry with process identity; never trusts PID alone.
- Logs are persisted locally and streamed to the Control Plane.
- Android battery/background restrictions must be surfaced as a limitation; a process cannot be promised to survive OS force-stop or device reboot without appropriate setup.

## Comparison test plan

Use the same small public Git app, same branch, same build command and same health endpoint:

1. Dokploy: create app -> connect Git -> deploy -> inspect logs -> stop/start -> redeploy.
2. Yevqo: add device -> create app -> connect Git -> deploy -> inspect live logs -> stop/start -> redeploy.

Measure: time to first healthy response, number of manual steps, failed/ambiguous states, log clarity, recovery time and user-reported friction. Record screenshots, timings and notes; do not claim a winner before a real run on both systems.
