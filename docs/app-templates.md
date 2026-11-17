# App templates

Yevqo's built-in starter templates are created on the target Agent through the existing
`create_site` command. They do not create a parallel site lifecycle and do not require
a database migration.

## Available starters

| Starter | Runtime | Files created by Agent |
| --- | --- | --- |
| Static Website | `static` | `index.html` |
| PHP Website | `php` | `index.php` |
| Node.js HTTP | `nodejs` | `package.json`, `server.js` |
| Go Application | `go` | `README.md` |

The device's **Добавить сайт** dialog exposes these starter choices. Site ID and port
validation remain in the existing UI and Agent. The Agent rejects an existing site
directory instead of overwriting user files.

## Lifecycle

1. Choose a target device and starter.
2. Enter a site ID and port.
3. The Control Plane sends the existing authenticated `create_site` command.
4. The Agent creates the starter directory and files.
5. The Control Plane records the site and the UI starts it through the existing
   `start_site` command.
6. Connect a Git repository and use the Deployment Engine for subsequent releases.

## Current boundaries

These are local, safe starter skeletons, not framework installers. They do not run
third-party post-install hooks, clone repositories, install packages, or execute
template-supplied commands. Laravel/Next.js/React starters, Git-based templates,
template variables, and one-click build/deploy remain future work. The P2 roadmap item
must remain open until those capabilities and their end-to-end tests are implemented.
