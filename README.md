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
PostgreSQL installation, database creation, application-role provisioning, encrypted credential storage, website process injection and local database backup are now wired through typed Agent commands. Restore, download/export, MariaDB/MySQL, existing-cluster adoption and full credential rotation remain pending. The UI reports device-side actions only after the Agent confirms them.


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
- The four-step wizard persists its storage budget, RAM budget, connection limit, local bind, SSL mode, authentication, startup policy and storage monitoring preferences. SSL mode is passed to PostgreSQL installation and applied to the server configuration.
- These settings are configuration metadata only while the server remains `planned`; no package install or database initialization is implied.
- Migration `003_database_server_options.sql` adds the optional settings without deleting existing database records.


### Website backups
- A typed Agent command creates a compressed TAR.GZ archive under the Agent data directory. New Agent installs default to $PREFIX/var/lib/veltrix-agent in private Termux storage; an existing custom VELTRIX_AGENT_DATA_ROOT is preserved.
- Archive files are created with private permissions, are isolated by site ID, skip symlinks and are not deleted when website metadata is removed.
- Backup metadata is associated with the owning account and device through migration `004_backup_device_scope.sql`.
- Restore is intentionally not exposed yet; safe extraction into a new site directory and archive verification are required first.

- Backup commands may take longer than ordinary site actions; the Control Plane now allows a bounded two-minute Agent response window for these archives.


### PostgreSQL Agent lifecycle
- The Database Server detail view triggers a typed PostgreSQL installation command on an online Termux Agent.
- The Agent validates the Termux prefix, data path, port, storage headroom and connection limit; installs only the allowlisted `postgresql` package and initializes only an empty data directory.
- Existing non-empty directories and detected PostgreSQL clusters are rejected without overwrite. PostgreSQL binds to localhost; remote access stays disabled.
- SSL require/prefer modes create a local self-signed certificate with a private key file; disable explicitly turns TLS off. The generated certificate provides encryption, not a public CA identity.
- MariaDB/MySQL installation and existing-cluster adoption remain pending.

### Database creation and website connections
- A ready PostgreSQL server can create an independent database and a separate application login role.
- The application role is not the database owner. It receives CONNECT, schema USAGE, table SELECT/INSERT/UPDATE/DELETE, and sequence usage privileges; it does not receive superuser or database-owner privileges.
- Passwords travel over the authenticated Agent connection via stdin-based PostgreSQL commands and are stored encrypted with AES-GCM in the Control Plane database.
- The AES-GCM key is stored in the private user config directory at ~/.config/veltrix/credentials.key (or the platform config directory), mode 0600. Back up this key securely; losing it makes stored database passwords unrecoverable.
- A database may link to one or more websites on the same Android device while the database listener remains localhost-only. The Control Plane injects DB_* variables into only the selected website process; existing .env files are never overwritten. Removing a website detaches its links but does not delete database data.

### PostgreSQL database backups
- Database backups run asynchronously through the Agent using pg_dump -Fc --no-owner --no-acl.
- The Agent validates the database identifier, stores dumps in Termux private storage under $PREFIX/var/lib/veltrix-agent/backups, and verifies each custom archive with pg_restore --list before the Control Plane marks it verified.
- Restore, backup download and cross-device transfer remain pending.

- Storage and RAM budgets, process policy and auto-start preferences are persisted as configuration; enforcement is not claimed until runtime quota and boot integration are implemented.
