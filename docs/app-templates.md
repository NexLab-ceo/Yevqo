# App templates

Yevqo's built-in starter templates are created on the target Agent through the existing
`create_site` command. They do not create a parallel site lifecycle and do not require
a database migration.

## Available starters

| Starter | Runtime | Files created by Agent |
| --- | --- | --- |
| Static Website | `static` | `index.html` |
| PHP Website | `php` | `index.php` |
| PHP JSON API | `php` | `index.php` |
| Node.js HTTP | `nodejs` | `package.json`, `server.js` |
| Node.js JSON API | `nodejs` | `package.json`, `server.js` |
| Go HTTP Application | `go` | `go.mod`, `main.go`, compiled `veltrix-app` |
| Custom Language App | `custom` | Empty site directory; source and commands supplied by Git or the owner |

The device's **Добавить сайт** dialog exposes each template by its stable template ID, so
PHP and Node.js variants remain independently selectable even though they share a runtime.
The PHP JSON API returns a small JSON response; the Node.js JSON API also exposes
`/health`. Site ID and port validation remain in the existing UI and Agent. The Agent rejects an existing site
directory instead of overwriting user files. The Go starter is a runnable `net/http`
application that reads `PORT`; the UI invokes the existing Android build provider before
starting it, so it does not leave the user with an unbuilt README-only placeholder.

## Lifecycle

1. Choose a target device and starter.
2. Enter a site ID and port.
3. The Control Plane sends the existing authenticated `create_site` command.
4. The Agent creates the starter directory and files.
5. The Control Plane records the site and the UI starts it through the existing
   `start_site` command.
6. Connect a Git repository and use the Deployment Engine for subsequent releases.

## Current boundaries

The Custom Language App starter intentionally creates an empty site directory. It can be
used with any project whose compiler/build tool is installed in Termux and an explicit
shell-free start command is configured. Build scripts execute with Agent-user permissions;
this is not a sandbox. These are local starters, not a remote template marketplace. The P2 roadmap item
must remain open until those capabilities and their end-to-end tests are implemented.
