# Project workspaces and environments

Yevqo projects are organizational scopes over existing workspace resources. Attaching or detaching a resource does not move, stop, overwrite, or delete the underlying device, application, database, or its data.

## Workspace ownership and roles

- Projects and resources remain owned by the workspace owner.
- Active workspace members are authorized on the server for each operation; the dashboard is not an authorization boundary.
- Owners and Admins can manage project resources, databases, backups, and environments.
- Owners and Admins can invite/remove members and change active member roles; Developers cannot manage workspace membership.
- Developers can read project/device state and use the explicitly allowed application build/deploy actions. Database and backup management are not granted to Developers.
- Resource IDs are resolved to their owning workspace before reads or mutations.

## Project resources

A project can group Android devices, applications/sites on those devices, and database servers on those devices.

Attaching a site or database server also attaches its host device to the project. Detaching a site or database server removes only that association; the host device is automatically detached only when no other project application or database server still references it. A device cannot be detached while project applications or database servers still depend on it.

Deleting a project removes the grouping and its associations only. It does not delete the underlying resources or their data.

## Application-scoped resources

An application is identified by its host `device_id` plus its `site_id`. It is not a separate device. One Android device can host multiple applications, and each application can have its own attached operational resources.

- **Device:** remains a host/infrastructure object. It can host multiple applications and database servers; it is not owned by one application.
- **Domain and Cloudflare route:** belong to one application/site. The site's detail page owns its primary domain and tunnel ingress configuration.
- **Logs and process monitoring:** are scoped to the application process and its host device. Device-level telemetry remains available separately.
- **Builds, releases, previews, Git deployment targets and scheduled application tasks:** are tied to a site/device pair. Project deployment runs can target one or more attached applications.
- **Databases:** a database server belongs to a workspace and can be assigned to a project/environment. A database instance belongs to its server. The connection from an application to a database instance is application-specific and injects that connection's managed environment values into the application.
- **Backups:** site backups identify the application by `device_id` + `site_id`; database backups identify a database instance. Backup schedules are resource-scoped.
- **Reports:** aggregate workspace or project resources and are not an application-owned resource.

The global sidebar is the workspace inventory/aggregate view. The application detail view is the contextual place to manage its domain, logs, environment/database connections, build, Git/release and scheduled tasks. Do not nest devices under applications or make one application the owner of a shared host device.

## Project GitHub delivery and deployments

A project may connect one GitHub App repository and attach it to one or more project sites/devices. Each target has its own verified deployment branch. Connecting a target snapshots the site's previous Git URL, branch and webhook settings; detaching the target, disconnecting the repository, or deleting the project restores those original settings.

Automatic Deploy uses the existing per-site HMAC-SHA256 GitHub push endpoint, including delivery replay protection, branch filtering and exact commit checkout. Configure `YEVQO_PUBLIC_URL` to the public HTTPS Control Plane origin (Cloudflare Tunnel is supported). With GitHub App Administration: Write permission, Yevqo creates, rotates and removes repository push webhooks. Without that permission, the UI returns a one-time webhook secret and URL for manual setup. The App also needs Pull requests: Read and Checks: Read for the PR and CI panels.

Project deployment runs persist source, branch, commit, Agent job ID, status, progress, timestamps and bounded output. The project Deployments tab supports manual Deploy, progress/log viewing, cancellation, redeploy and rollback to a successful immutable release. Agent deployment events and build output are persisted; cancellation is treated as terminal and cannot be overwritten by a later completion race.

Migration `025_project_deployment_lifecycle.sql` adds site targets, webhook registration IDs and durable deployment runs. It is additive and must be applied through the normal migration/upgrade process; it is not applied to any existing or production database as part of this change.

## Project source repository

A project can store one verified GitHub App repository binding. The authenticated project owner/admin selects an installed GitHub App and a repository; the Control Plane verifies that the installation belongs to the repository owner, reads repository metadata, and checks the selected deployment branch before persisting the binding. The project Source Repository tab can display and disconnect the binding.

The binding stores repository metadata and the selected branch, not a long-lived GitHub token. Installation credentials remain encrypted in the existing integration record; short-lived installation tokens are requested when needed. Disconnecting the GitHub App removes managed push webhooks, restores each linked site's original Git/webhook settings, and then removes the dependent project repository bindings.

Migration `024_project_repositories.sql` is additive and must be applied through the normal migration/upgrade process. It has not been applied to a live or production database as part of this work.

**Boundary:** connecting a repository alone does not trigger a build. At least one project site/device target must be linked. Automatic webhook registration requires `YEVQO_PUBLIC_URL` plus GitHub App Administration: Write; otherwise the one-time webhook URL and secret must be added manually. Environment promotion pipelines and PR merge automation are not part of this stage.

## Environments

Every project receives Production, Development, and Staging environments. Custom environments can be created, renamed, updated, and deleted.

Production is protected server-side: it cannot be renamed or deleted; its application and database-server assignments cannot be removed; applications and database servers may be assigned to it.

Environment application assignments must refer to applications already attached to the project. Environment database-server assignments must refer to database servers already attached to the project.

**Important limitation:** environment assignments currently organize project resources; they do not create separate database instances, clone application files, or automatically switch a site's database connection. Runtime isolation and environment-aware deployment promotion still require further implementation.

### Project-level membership and roles

A project can assign an explicit role to an existing active workspace member. Available project roles are **Admin**, **Developer**, and **Viewer**. Developers can create applications on devices already attached to the project, attach the resulting application to the project, assign it to one of the project's environments, and run supported build/lifecycle/deployment actions; only Admins can change project configuration and secrets. An explicit project role overrides the member's workspace role only for project-scoped API operations; when no project role is assigned, the workspace role is inherited. The project Owner always retains full control. Viewer is read-only. Project membership does not create a separate Yevqo account or send an email invitation; the person must already be an active member of the workspace.

Project roles are checked server-side for project detail, environment/resource management, project deployment/repository endpoints, project-scoped device commands, site lifecycle/deployment actions, preview deployments, webhook configuration, and site Git/ENV settings opened from a project. Workspace-level operations outside a project continue to use workspace RBAC.

### Per-environment variables and secrets

Each environment has its own variable namespace at `/projects/{projectId}/environments/{environmentId}/variables`. Values are encrypted with the Control Plane credential key (AES-GCM) before persistence; the API returns names and masked state only, never plaintext values. Saving an existing key rotates its value; deleting a key removes only that environment's entry.

At application start, Android local builds, and deployment, the selected project's environment values are injected into the Agent process. Precedence is: project environment values, then application-specific values, then Yevqo-managed database connection values (highest priority). Reserved process, port/host, DB and Yevqo keys cannot be overridden. Build scripts receive the same environment map after explicit project-code consent; build output can still reveal values if the user's build script deliberately prints them, so only trusted project code should be built with secrets configured. A site's environment assignment is resolved even when it has no database connection, so application configuration is not coupled to database provisioning.

## Project dashboard sections

The Overview includes resource counts plus attached devices, applications, database servers, each project environment, and recent deployment runs. Project Activity combines project-filtered audit events (where the effective project role permits audit access) with operational logs for roles without audit access.

The project detail page separates the resource summary, project-owned environments, applications, deployments, activity, source repository, and settings.

- **Overview** summarizes the devices, applications, database servers, environments, and deployment runs currently linked to this project.
- **Environments** lists only the environments whose `project_id` is this project. Production, Development, Staging, and custom environments are not global/shared records. Each environment has its own application and database-server/database assignments and its own settings page.
- **Applications** can scaffold a new site on an online device already attached to this project using the Agent's typed `create_site` command, attach the resulting site to this project, and optionally assign it to one of that project's environments (Production is the default). Existing sites can also be attached. The Applications tab shows the site's runtime/status and can start or stop it through typed Agent commands. Scaffolding does not deploy arbitrary repository code.
- **Activity** shows project-filtered audit events to Owners/Admins and project-filtered operational logs to Developers, respecting the existing role policy.
- **Settings** shows the project configuration and project members/roles. Explicit Admin, Developer, and Viewer project-role overrides are supported for active workspace members; when no project override exists, the workspace role is inherited. All permissions are enforced server-side.

Creating an application requires a manageable project and an attached device. The Agent must be online. If site creation succeeds but the subsequent project association fails, the UI reports that partial outcome rather than claiming the application was attached.

## Database and backup access

Database-server inventory and lifecycle, database inventory and creation, database roles/privileges, site connection links, backup inventory, backup creation, restore, transfer, chunk download, and schedules are scoped to the workspace owner and role.

Environment scoping is enforced for database inventory and runtime credential injection. Creating a site/database link while selecting an environment requires the site and database server to be assigned to that same project environment. This is application-level scoping only; processes still run under the same Termux OS user and are not isolated by containers or OS users.

Managed PostgreSQL and MariaDB/MySQL server installation and local database operations are available through typed Agent commands. MariaDB/MySQL uses a dedicated per-server data directory, localhost-only binding, optional TLS, database/user provisioning, privileges, statistics, verified SQL backups and restore into a new database. Existing non-empty directories are never initialized over. Existing-cluster adoption and physical Android verification remain outstanding.

## Migrations

- `021_project_environments.sql` adds project environments, application assignments, and project database-server associations.
- `022_project_environment_databases.sql` adds environment-level database-server assignments.
- `024_project_repositories.sql` adds the optional project-level GitHub App repository binding.
- `025_project_deployment_lifecycle.sql` adds repository-to-site deployment targets, webhook registration IDs, and durable deployment runs.

These are additive migrations. The all-in-one `.deb` stages missing, byte-identical migration files into the Control Plane data root during package post-install and `yevqo-setup-control`; the Control Plane applies pending files on startup. Existing migration files are never overwritten, and same-name content conflicts or symlink destinations stop setup. This delivery mechanism does not itself run migrations against a database; the Control Plane applies them on its next startup. Take a verified backup before upgrading an existing installation.