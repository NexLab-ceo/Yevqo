# Yevqo

The `main` branch is the active Yevqo implementation. The all-in-one package keeps explicit `yevqo-control` and `yevqo-agent` names. The standalone Agent package exposes the simple `yevqo`, `yevqo-setup` and `yevqo-update` commands. Both packages check the public [VELTRIX Dock releases](https://github.com/NexLab-ceo/veltrix-dock/releases) automatically every six hours and install verified updates in the background. The deploy helper CLI is `yevqo-cli`; its inventory, deployment and backup commands are documented in [CLI usage](docs/cli.md).

## Что скачать

В CI собираются два отдельных Android/Termux-пакета. Оба файла в одном выпуске всегда имеют **одинаковую версию**:

- **Если нужен Control Plane и Agent на одном телефоне — скачайте `yevqo-all_<version>_aarch64.deb`.** Внутри панель, Agent, веб-интерфейс, миграции и команды `yevqo-control` / `yevqo-agent`.
- **Если нужен только агент для подключения телефона к чужой панели — скачайте `yevqo_<version>_aarch64.deb`.** В этом пакете агент запускается командой `yevqo`, настройка — `yevqo-setup`, обновление — `yevqo-update`. Укажите WebSocket URL панели и токен из Profile → Agent pairing token.

Пакет `yevqo-all` требует MariaDB, PHP, Git и `termux-services`. Пакет `yevqo` требует PHP, Git и `termux-services`. Оба пакета проверяют публичные [релизы VELTRIX Dock](https://github.com/NexLab-ceo/veltrix-dock/releases) каждые 6 часов. Обновление устанавливается только при корректном version tag, наличии подходящего `.deb` и `SHA256SUMS`; при неполном релизе updater откажется от установки. Публикация релизов пока выполняется вручную — см. [инструкцию по обновлениям](docs/updates.md). Отключить автообновления: `sv down yevqo-updater`; включить снова: `sv up yevqo-updater`. Подробности — в [обновлениях](docs/updates.md). `cloudflared` не является обязательной зависимостью: экран Domains может установить нативный пакет Termux при настройке Cloudflare Tunnel. См. [инструкцию по Cloudflare Tunnel](docs/cloudflare-tunnel.md).

Перед настройкой Control Plane или Agent выполните `termux-setup-storage` в Termux и разрешите доступ к общему хранилищу. Данные Control Plane по умолчанию находятся в `/sdcard/yevqo`, файлы и рабочие каталоги Agent — в `/sdcard/yevqo-agent`. Секреты не хранятся в общем хранилище: конфигурация Control Plane с DB credentials находится в `$PREFIX/var/lib/yevqo-control/config/config.json`, конфигурация Agent с pairing token — в `$PREFIX/var/lib/veltrix-agent/config/agent.json`. Старый Control Plane config из `/sdcard/yevqo/config/config.json` переносится в приватный каталог при первом запуске после обновления.

Для автозапуска после перезагрузки Android установите приложение **Termux:Boot** из того же источника, что и Termux, и откройте его один раз. Также разрешите Termux работать в фоне в настройках батареи Android.


### Workspace and operations foundation
- Projects can group workspace devices, websites and database servers. Resources can be attached/detached without deleting the underlying data; a project can be deleted without deleting its devices, sites, files or databases.
- Projects have Production, Development and Staging environments. Environment assignments scope database inventory and runtime credential injection; a site/database link created in an environment must target a site assigned to that same environment. This is application-level scoping, not OS/container isolation. Production rename/delete and assignment removal are blocked server-side.
- Database Server configurations are stored as `planned` metadata until an Agent installation is confirmed. Saving a configuration does not install packages or modify an Android device.
- Database Server records remain owned by the workspace owner; server-side role checks govern member access. A server cannot be removed while database metadata references it.
- Account settings persist Simple / Advanced mode, locale and theme.
- Activity logs, notifications, account summary reports and backup metadata have read APIs.
- The Termux installer prepares MariaDB and the local application account, then copies SQL migrations into the Control Plane data directory. On every startup, Control Plane idempotently ensures its own database exists using the local MariaDB client, creates the core tables from `000_core.sql` on first run, then applies all available additive migrations in order (currently `001`–`032`). Applied migration names and checksums are recorded in `schema_migrations`. Missing migration files are a startup error rather than silently skipping schema setup.

Project workspace and environment behavior, role boundaries, resource detach safety, and migration limitations are documented in [Project workspaces](docs/project-workspaces.md).

### Important implementation boundary
PostgreSQL and MariaDB/MySQL now have typed Agent workflows for server installation, start/stop, database and least-privilege application-user provisioning, user/role management, database privileges, connection tests, statistics and local backups. MariaDB uses a dedicated per-server Termux data directory, loopback-only binding, TLS certificates when SSL is enabled, verified SQL dumps and restore into a new database with rollback cleanup. Existing non-empty data directories are refused and never initialized over. Physical Android/Termux validation, existing-cluster adoption, cross-engine backup transfer and full credential rotation remain open. The UI reports device-side actions only after the Agent confirms them. The `mysql` engine selection currently uses the Termux MariaDB server package with MySQL-compatible protocol; it does not install Oracle MySQL Server.



### GitHub App and automatic Deploy
- GitHub PAT connections remain supported. GitHub App connections accept the App ID, installation ID and PEM private key; the private key is encrypted at rest and is never returned to the browser.
- Control Plane signs short-lived RS256 App JWTs and requests installation access tokens just in time. Tokens are refreshed for private HTTPS GitHub repository operations and are not persisted as long-lived credentials.
- The App installation needs read-only Contents and Metadata permissions and must be installed on the account or organization that owns the repository.
- Push webhooks use per-site HMAC-SHA256 secrets, branch filtering, exact commit checkout and delivery-ID replay protection. After a repository is connected, automatic deployments can be enabled directly for selected site targets.
- Live GitHub App installation and private-repository deployment still require configuring a GitHub App and verifying it against a real installation.

### Website lifecycle and operations
- Site creation is an explicit typed Agent command. It creates a new site directory and a starter file without overwriting an existing directory.
- Application processes listen on a private loopback backend; the Agent-owned stable public site proxy binds to `0.0.0.0` so LAN clients can reach the configured site port. This exposes that port to reachable network interfaces; use firewall/router rules for external exposure and keep the application backend loopback-only.
- Site IDs and paths are validated on the Agent. Site log files are private to the Termux account.
- Removing a website from the panel removes its metadata only; its device files are deliberately preserved.
- Domains can be assigned as metadata to a site. DNS and reverse-proxy / tunnel routing are not reported as active until verified separately.
- Git deployments check out into a private staging directory, build before activation and publish immutable commit-addressed releases under `.yevqo/releases`. A failed activation restores the previously running release.
- Release history exposes commit IDs, dates and the active release. Rollback activates an existing release without rebuilding it.
- Release cleanup is explicit (1–50 newest releases to retain); it always preserves the active release and rejects symlinked metadata/release paths. No automatic cleanup deletes rollback targets.
- Site mutations (create, build, Git pull, start, stop, deploy, rollback and release cleanup) share an Agent lifecycle lock so commands cannot interleave midway through a deployment. This currently serializes mutations across sites on the same Agent.
- Deployment history records successful and failed deploy/rebuild/rollback attempts without storing command payloads or credentials.
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
- Website backups are queued asynchronously by the Control Plane. A typed Agent command creates a compressed TAR.GZ archive under $PREFIX/var/lib/yevqo-agent/backups/sites in private Termux storage. Site files continue to use the configured Agent data directory; new installs default that directory to $PREFIX/var/lib/yevqo-agent.
- Archive files are created with private permissions, are isolated by site ID, skip symlinks and are not deleted when website metadata is removed.
- Backup metadata is associated with the owning account and device through migration `004_backup_device_scope.sql`.
- Restore is intentionally not exposed yet; safe extraction into a new site directory and archive verification are required first.

- Backup commands may take longer than ordinary site actions; the Control Plane now allows a bounded two-minute Agent response window for these archives.


### PostgreSQL Agent lifecycle
- The Database Server detail view triggers a typed PostgreSQL installation command on an online Termux Agent.
- The Agent validates the Termux prefix, data path, port, storage headroom and connection limit; installs only the allowlisted `postgresql` package and initializes only an empty data directory.
- Existing non-empty directories and detected PostgreSQL clusters are rejected without overwrite. PostgreSQL binds to localhost; remote access stays disabled.
- SSL require/prefer modes create a local self-signed certificate with a private key file; disable explicitly turns TLS off. The generated certificate provides encryption, not a public CA identity.
- MariaDB/MySQL managed lifecycle and local database operations are implemented on `main`; existing-cluster adoption and physical Android validation remain pending.

### Database creation and website connections
- A ready PostgreSQL server can create an independent database and a separate application login role.
- The application role is not the database owner. It receives CONNECT, schema USAGE, table SELECT/INSERT/UPDATE/DELETE, and sequence usage privileges; it does not receive superuser or database-owner privileges.
- Passwords travel over the authenticated Agent connection via stdin-based PostgreSQL commands and are stored encrypted with AES-GCM in the Control Plane database.
- The AES-GCM key is stored in the private user config directory at ~/.config/veltrix/credentials.key (or the platform config directory), mode 0600. Back up this key securely; losing it makes stored database passwords unrecoverable.
- A database may link to one or more websites on the same Android device while the database listener remains localhost-only. The Control Plane injects DB_* variables into only the selected website process; existing .env files are never overwritten. Removing a website detaches its links but does not delete database data.

### PostgreSQL database backups
- Database backups run asynchronously through the Agent using pg_dump -Fc --no-owner --no-acl.
- The Agent validates the database identifier, stores dumps in Termux private storage under $PREFIX/var/lib/yevqo-agent/backups, and verifies each custom archive with pg_restore --list before the Control Plane marks it verified.
- Restore is available only into a new database on the original ready PostgreSQL server; it never overwrites the source database. Backup download and cross-device transfer remain pending.

- Storage and RAM budgets, process policy and auto-start preferences are persisted as configuration; enforcement is not claimed until runtime quota and boot integration are implemented.


### Android Agent connection (Termux)
- The Control Plane registers the Agent WebSocket endpoint at `/ws`. For an Agent running on the same Android device, set `server_url` to `ws://127.0.0.1:8080/ws`. For an Agent on another device, replace `127.0.0.1` with the Control Plane device's reachable LAN or Tailscale address.
- Authentication is sent by the Agent as the first WebSocket binary message using the `AuthRequest` envelope. Its `token` field is the dedicated **Agent pairing token**, generated in Dashboard → Profile → Agent pairing token. It is separate from the public referral code; Control Plane stores only its SHA-256 hash. The raw token is shown once and must be pasted into the Agent setup. The endpoint does not require an Authorization header or a `?token=` query parameter.
- The WebSocket route is intentionally unauthenticated at HTTP upgrade time; authentication happens immediately after upgrade in the first protocol message.
- The HTTP logging middleware must preserve the `http.Hijacker` interface used by Gorilla WebSocket. The Control Plane's `responseWriter` forwards hijacking so the WebSocket handshake can complete.


### Agent process safety
- Managed applications are tracked by PID plus the Linux process start token; a reused PID is not treated as the original application.
- Each application runs in its own process group. The Agent waits for child exits while it is running and reconciles persisted records after restart.
- Startup failures are terminated instead of leaving untracked application processes behind.
- Database environment values are passed only to the selected application's runtime process. They are no longer written to the process registry; legacy `extra_env` values are scrubbed from the registry when it loads.
