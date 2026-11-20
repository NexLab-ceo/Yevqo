# Project workspaces and environments

Yevqo projects are organizational scopes over existing workspace resources. Attaching or detaching a resource does not move, stop, overwrite, or delete the underlying device, application, database, or its data.

## Workspace ownership and roles

- Projects and resources remain owned by the workspace owner.
- Active workspace members are authorized on the server for each operation; the dashboard is not an authorization boundary.
- Owners and Admins can manage project resources, databases, backups, and environments.
- Developers can read project/device state and use the explicitly allowed application build/deploy actions. Database and backup management are not granted to Developers.
- Resource IDs are resolved to their owning workspace before reads or mutations.

## Project resources

A project can group Android devices, applications/sites on those devices, and database servers on those devices.

Attaching a site or database server also attaches its host device to the project. Detaching a site or database server removes only that association; the host device is automatically detached only when no other project application or database server still references it. A device cannot be detached while project applications or database servers still depend on it.

Deleting a project removes the grouping and its associations only. It does not delete the underlying resources or their data.

## Environments

Every project receives Production, Development, and Staging environments. Custom environments can be created, renamed, updated, and deleted.

Production is protected server-side: it cannot be renamed or deleted; its application and database-server assignments cannot be removed; applications and database servers may be assigned to it.

Environment application assignments must refer to applications already attached to the project. Environment database-server assignments must refer to database servers already attached to the project.

**Important limitation:** environment assignments currently organize project resources; they do not create separate database instances, clone application files, or automatically switch a site's database connection. Runtime isolation and environment-aware deployment promotion still require further implementation.

## Database and backup access

Database-server inventory and lifecycle, database inventory and creation, database roles/privileges, site connection links, backup inventory, backup creation, restore, transfer, chunk download, and schedules are scoped to the workspace owner and role.

The managed database engine lifecycle is currently PostgreSQL-focused. Do not assume that MySQL/MariaDB server installation, restore, or migration is available merely because those engine names appear in configuration or UI.

## Migrations

- `021_project_environments.sql` adds project environments, application assignments, and project database-server associations.
- `022_project_environment_databases.sql` adds environment-level database-server assignments.

Both are additive migrations. They have not been applied to production or to the user's existing database as part of this work. Apply them only through the normal migration/upgrade process after taking a verified backup and checking the target installation.