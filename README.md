## VELTRIX Aurora migration track

The `feature/database-server-wizard` branch contains the in-progress VELTRIX Dock UI and Control Plane migration. It does not change `main` or production data.

### Workspace and operations foundation
- Projects can group devices and websites. Deleting a project deletes only its grouping records; devices, sites, files and databases remain untouched.
- Database Server configurations are stored as `planned` metadata. Saving a configuration does not install packages or modify an Android device.
- Database Server records are isolated per account and device. A server cannot be removed while database metadata references it.
- Account settings persist Simple / Advanced mode, locale and theme.
- Activity logs, notifications, account summary reports and backup metadata have read APIs.
- New additive schema is in `migrations/001_modules.sql`; the Termux installer copies SQL migrations into the control-plane data directory and applies them at startup.

### Important implementation boundary
Database engine installation, database creation, credentials, backup execution and restore are not yet wired to the Android Agent. The current UI must not imply those device-side operations have completed. These capabilities require typed Agent protocol commands, device-side validation, idempotent execution and rollback-safe behavior.


### Website lifecycle and operations
- Site creation is an explicit typed Agent command. It creates a new site directory and a starter file without overwriting an existing directory.
- Site processes bind to localhost by default; occupied ports are rejected before launch.
- Site IDs and paths are validated on the Agent. Site log files are private to the Termux account.
- Removing a website from the panel removes its metadata only; its device files are deliberately preserved.
- Domains can be assigned as metadata to a site. DNS and reverse-proxy / tunnel routing are not reported as active until verified separately.
- Deployment history records site create/start/stop and Git Pull actions without storing command payloads or credentials.
- Monitoring currently reports device heartbeat, agent version and site/server counts. CPU, RAM and temperature require a future telemetry event from the Agent.


### Device monitoring telemetry
- The Agent now emits an application heartbeat every 30 seconds; Control Plane updates device `last_seen`.
- The Monitoring screen requests typed metrics from online agents: CPU utilization sample, 1-minute load average, memory totals/available, storage totals/available, and thermal-zone temperature when readable.
- Metrics are stored as time-stamped snapshots in the additive `002_device_metrics.sql` migration. Missing Android thermal sensors are represented as unavailable, not as zero.


### Database Server configuration
- The four-step wizard persists its storage budget, RAM budget, connection limit, local bind, SSL mode, authentication, startup policy and storage monitoring preferences.
- These settings are configuration metadata only while the server remains `planned`; no package install or database initialization is implied.
- Migration `003_database_server_options.sql` adds the optional settings without deleting existing database records.


### Website backups
- A typed Agent command creates a compressed TAR.GZ archive under the device's private VELTRIX data directory.
- Archive files are created with private permissions, are isolated by site ID, skip symlinks and are not deleted when website metadata is removed.
- Backup metadata is associated with the owning account and device through migration `004_backup_device_scope.sql`.
- Restore is intentionally not exposed yet; safe extraction into a new site directory and archive verification are required first.

- Backup commands may take longer than ordinary site actions; the Control Plane now allows a bounded two-minute Agent response window for these archives.


### PostgreSQL Agent lifecycle
- The Database Server detail view can trigger a typed PostgreSQL installation command on an online Termux Agent.
- The Agent validates the Termux prefix, data path, port, storage headroom and connection limit; installs only the allowlisted `postgresql` package and initializes only an empty data directory.
- Existing non-empty directories and detected PostgreSQL clusters are rejected without overwrite. PostgreSQL binds to localhost; remote access stays disabled.
- MariaDB/MySQL installation, existing-cluster adoption, database/user creation and website credential provisioning remain separate follow-up work.

- Storage and RAM budgets, process policy and auto-start preferences are persisted as configuration; enforcement is not claimed until runtime quota and boot integration are implemented.
