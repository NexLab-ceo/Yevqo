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
- [x] Durable write-ahead deployment journal for Git activation and rollback; Agent startup restores the last confirmed process after an interrupted activation, and unresolved recovery records cannot be overwritten.
- [x] Regression coverage for failed build steps, deployment cancellation races, Agent restart process adoption, process crashes, occupied ports, process-group shutdown and HTTP health-check failures/timeouts.

### P0 implementation notes

- Restart attempts are intentionally bounded to avoid an endless crash loop. After the automatic retry budget is exhausted, manual intervention is required.
- A successful TCP readiness check is the default. Configure an HTTP health-check path when the application exposes a health endpoint.
- Release activation is crash-recoverable, but it still uses a stop/start handoff on the site's port. True zero-downtime switching requires a stable reverse-proxy/blue-green layer and remains open; the Agent does not claim an atomic live traffic swap yet.
- Logs are local to the device. The live viewer timestamps received lines; timestamps are not injected into the application's original stdout/stderr bytes.
- Deployment job events are streamed to the Control Plane. The durable job record is retained on the Agent for local recovery/diagnostics; it is not a cross-device job-history database.
- Android battery optimization, force-stop and OS process reclamation can still terminate workloads outside the Agent's control. Use the supported boot/background setup where appropriate.

## P1 — Deployment UX and recovery

- [x] Per-site environment editor with AES-GCM encryption in Control Plane storage, masked readback, reserved-key validation and scoped injection.
- [x] Agent-side encrypted environment persistence so crash recovery and Agent restart can re-inject secrets without plaintext process-registry storage.
- [x] Release history with commit, runtime, timestamp and active status.
- [x] Rollback to a previous immutable release without rebuilding.
- [x] Signed GitHub push webhooks (HMAC-SHA256), delivery replay protection, branch filtering and exact commit checkout.
- [x] Manual branch and full commit SHA-1/SHA-256 selection for deployments.
- [x] Persistent per-site scheduled commands with fixed intervals (15 minutes–30 days), pause/resume, bounded runtime and task output history.
- [x] In-app notifications for manual and webhook deployments, scheduled tasks, scheduled backups and transfer outcomes.
- [x] Per-site main-process CPU and RSS telemetry from Linux /proc; the UI explicitly does not claim container/cgroup or child-process aggregate metrics.
- [x] Cloudflare API token verification and encrypted storage; scoped zone DNS CNAME create/update/list/delete for Cloudflare Tunnel hostnames.
- [x] Website backup download, safe archive restore with atomic directory swap, and preservation of target-device environment secrets.
- [x] Scheduled website and PostgreSQL backups with configurable interval and on-device retention pruning.
- [x] Chunked database and website backup downloads plus cross-device transfer with destination-side archive verification.
- [x] PostgreSQL backup restore into a new database, including transferred backups.

### P1 implementation notes

- Environment values are masked on read. They are encrypted at rest in the Control Plane and again on the Agent; they are never included in website backup archives or copied to another device.
- GitHub webhook enablement requires explicit consent to execute repository code. The endpoint verifies the raw request body signature and ignores duplicate delivery IDs. Agent-offline deliveries return a retryable error before the delivery ID is committed.
- Scheduled tasks use fixed intervals rather than arbitrary cron expressions. Commands execute as the Termux Agent user in the site's directory, with that site's protected environment and a 30-minute hard timeout.
- CPU/RSS telemetry samples the managed main PID from /proc; Android may not expose all process/cgroup accounting. Child processes are not aggregated.
- Cloudflare automation manages proxied CNAME records pointing to a tunnel UUID hostname. It does not create or provision Cloudflare Tunnels, issue certificates, or configure Public Hostname origins.
- Backup transfer is streamed through the Control Plane in 256 KiB chunks and is currently limited to 2 GiB per archive. Both source and destination Agents must be online; database transfers require a ready PostgreSQL server on the destination.
- Website restore replaces the target site's files and leaves its process stopped. Review the restored files before starting it.
- Backup archives exclude Agent deployment metadata and encrypted environment files. Restore preserves the target Agent's existing encrypted environment rather than importing secrets from another device.

## P2 — Platform capabilities

- [ ] App templates and one-click starters. (Initial built-in runtime starters are exposed in the device create-site dialog; framework/Git templates and full one-click build/deploy remain open. See [app templates](app-templates.md).)
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
