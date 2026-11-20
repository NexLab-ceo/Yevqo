# Yevqo CLI

`yevqo-cli` calls the authenticated Control Plane API. It does not run Docker or build workloads locally; operations are sent to the connected device Agent.

## Configuration

```sh
export YEVQO_URL='https://your-control-plane.example'
export YEVQO_SESSION='your-session-cookie-value'
```

Treat `YEVQO_SESSION` as a password. Do not commit it, paste it into issue reports, or expose it in shell history on shared devices.

Global flags `--server` and `--session` override the environment variables.

## Inventory

```sh
yevqo-cli projects
yevqo-cli project 12
yevqo-cli environments 12
yevqo-cli devices
yevqo-cli sites
yevqo-cli databases
yevqo-cli database-servers
yevqo-cli backups
yevqo-cli backup-schedules
yevqo-cli deployments
yevqo-cli monitoring
```

## Project and environment lifecycle

```sh
yevqo-cli create-project Website "Main website project"
yevqo-cli update-project 12 Website "Updated description"
yevqo-cli delete-project 12 --confirm
yevqo-cli create-environment 12 QA "Quality assurance"
yevqo-cli update-environment 12 4 QA "Updated description"
yevqo-cli delete-environment 12 4 --confirm
```

Delete commands require `--confirm`. Project deletion removes only the project grouping; it does not delete sites, databases, or their data. Environment deletion removes the environment and its assignments; Production remains protected by the Control Plane.

## Site operations

```sh
yevqo-cli deploy 12 my-site --allow-project-code
yevqo-cli start 12 my-site
yevqo-cli stop 12 my-site
yevqo-cli restart 12 my-site
yevqo-cli logs 12 my-site
```

`deploy` requires the explicit `--allow-project-code` flag because it can execute project-controlled build and start commands. Without it, the CLI refuses to send the request.

## Backups

```sh
yevqo-cli backup-site 12 my-site
yevqo-cli backup-db 42
```

Backup commands queue work on the owning device. Check `yevqo-cli backups` for the resulting status; a queued request is not proof that a backup completed or was verified.

All operations remain subject to the same server-side workspace roles and resource ownership checks as the dashboard.