# Yevqo Build Engine — Android / Termux

Yevqo Engine builds projects locally on the connected Android device through Termux.
It does not require root, Docker, Podman, Nixpacks, a remote build server, or a VPS.
The Engine is language-extensible: automatic detection covers common ecosystems,
and projects in other languages can provide their own executable plus argv commands.

## Architecture

- **Analyzer:** read-only project detection, build-plan generation and the supported
  `yevqo.yml` configuration (`project.root`, `build.provider`, `build.install`,
  `build.command`, `run.command`, `run.port`). The only recognized filename is
  exactly `yevqo.yml`; legacy `veltrix.yaml` / `veltrix.yml` files are ignored.
- **Environment detector:** Android/Termux, architecture, CPU count, installed
  compiler/build tools, memory estimate and free-disk estimate.
- **Local provider:** executes one program at a time with an argv vector; it never
  interprets a command through a shell.
- **Supervisor:** serialized builds, explicit project-code consent, timeout and cancellation of the
  complete build process group, bounded combined stdout/stderr, path validation and
  a filtered environment.
- **Build cache:** persistent per-tool caches under `<agent-data>/build-cache`
  for Go, Rust/Cargo, Node/npm, Python/pip, Deno, Gradle, Composer, Ruby, Dart,
  .NET and Julia. Existing explicitly configured cache paths are preserved.
- **Result and history:** reports provider, stack, timestamps, duration, bounded logs,
  truncation state, artifact directory and failure reason. Completed manual and Git
  deployment builds are stored locally per site with private file permissions; the
  latest 10 metadata records are listed in the Build Engine page, and a selected log
  is fetched on demand to avoid transferring every historical log at once.

## Automatic language detection

When the corresponding toolchain is installed for the device architecture, the
Analyzer can create plans for:

| Ecosystem | Project marker | Default build/install tools |
|---|---|---|
| Node.js | `package.json` and lockfiles | npm, pnpm, Yarn or Bun |
| Python | `requirements.txt`, `pyproject.toml`, `Pipfile` | pip |
| Go | `go.mod` | Go compiler |
| Rust | `Cargo.toml` | Cargo |
| PHP | `composer.json` | Composer |
| Java | `pom.xml` | Maven / Maven wrapper |
| Kotlin / JVM / Gradle | Gradle build/settings files | Gradle / Gradle wrapper |
| C / C++ / Fortran | `CMakeLists.txt` or `Makefile` | CMake / Make |
| Swift | `Package.swift` | SwiftPM |
| Elixir | `mix.exs` | Mix |
| Scala | `build.sbt` | sbt |
| Zig | `build.zig` | Zig |
| Dart | `pubspec.yaml` and `bin/main.dart` | Dart |
| Deno | `deno.json` / `deno.jsonc` | Deno tasks |
| Erlang | `rebar.config` | rebar3 |
| Haskell | `*.cabal` | Cabal |
| OCaml | `dune-project` | Dune |
| Crystal | `shard.yml` | Shards / Crystal |
| Perl | `cpanfile` | cpanm |
| Nim | `*.nimble` | Nimble |
| Lua | `*.rockspec` | LuaRocks |
| .NET | `*.csproj` / `global.json` | dotnet |
| Ruby | `Gemfile` | Bundler |
| Static | `index.html` | no compilation required |

Detection is heuristic, not a promise that every framework or project layout has a
universal build command. For nonstandard projects, use the universal custom-command
configuration below.

## Universal custom build commands

If the Analyzer cannot recognize a language or project, use the Build Engine's
custom Install and Build fields, or define them in the project-root `yevqo.yml`:

```yaml
version: 1

project:
  name: example-app
  root: .
  runtime: nodejs

build:
  provider: custom
  install: "npm ci"
  command: "npm run build"

run:
  command: "npm run start"
  port: 3000

environment:
  NODE_ENV: production

healthcheck:
  path: /health
  interval: 30s

artifacts:
  include:
    - dist/**
  exclude:
    - dist/*.map

cache:
  enabled: true
  paths:
    - .cache

resources:
  memory: 512MiB

deploy:
  restart: true
```

Supported configuration includes:

- `project.name`, `project.runtime`, `project.root`
- `build.provider`, `build.install`, `build.command`
- `run.command`, `run.port`
- `environment.KEY`: explicit per-project variables injected into build and runtime processes. Keys are allowlisted; reserved host, port, database-secret and Agent variables are rejected. Do not commit secrets in this file.
- `healthcheck.path`: local HTTP GET path. Startup requires a 2xx response; after startup the Agent probes at `healthcheck.interval` (default 30s) and terminates a process after three consecutive failures so the configured restart supervisor can recover it. Interval range: 1s–1m.
- `artifacts.include` / `artifacts.exclude`: relative glob patterns. Matching regular files are copied to the private Agent artifact directory; symlinks are skipped. Collection is limited to 256 MiB per file and 1 GiB total. Files are listed against the corresponding successful build-history record and can be downloaded from the Build Engine page in bounded 256 KiB chunks. `**` matches nested directories.
- `cache.enabled` / `cache.paths`: selected relative project paths are restored from and saved to a private per-site cache between builds/releases. Transfers are limited to 256 MiB per file and 1 GiB total per sync. This is separate from compiler/package-manager caches.
- `resources.memory`: minimum available-memory preflight threshold (for example `512MiB`). This is not a hard process memory limit.
- `deploy.restart`: when explicitly set, `true` selects the `always` restart policy and `false` selects `never`.

The parser supports a strict YAML subset with two-space indentation, scalar
values and lists; it is not a general-purpose YAML implementation. Unknown
sections/keys, duplicate keys, unsupported versions, tabs and malformed
indentation are rejected. The file is optional: when absent, Yevqo uses
automatic project detection. `veltrix.yaml` and `veltrix.yml` are ignored.

Commands consist of an executable available on the Agent's Termux `PATH` followed
by arguments. Single/double-quoted arguments are supported. The executable can be
any installed compiler, package manager or build tool; it is not limited to a
hard-coded language allowlist. Project-local Gradle/Maven wrappers are supported.
The Agent checks that the executable exists before starting it.

Shell operators, pipes, redirection, variable expansion and shell executables
(`sh`, `bash -c`, PowerShell, etc.) are rejected. Use separate install and build
commands instead of chaining commands. A build command may invoke a language
runtime or project tool; that code still executes with the Agent user's permissions.

For a tool not installed, the build fails with an explicit missing-tool error.
Yevqo does not silently download compilers, SDKs or system packages. Install the
required toolchain through Termux first, and make sure a compatible Android
ARM64 build exists.

## Security and Android limits

The runner requires explicit `allow_project_code=true`; only build trusted
repositories. Package scripts, compiler plugins, build files and project commands
execute with the Agent user's permissions. This is not a security sandbox.

Path checks reject traversal and symlink escapes. Child processes receive a
filtered environment, a timeout and bounded logs. Builds are serialized. Memory
and disk checks are preflight estimates, not hard limits.

Without root/kernel support, the Agent cannot enforce cgroup CPU/RAM limits,
isolate networking, provide a separate UID, or guarantee termination of every
grandchild process. Build caches consume local storage and can be removed by
deleting the corresponding `build-cache` directory while the Agent is stopped.

## Not provided by the Android local provider

Dockerfile execution, Docker/Podman containers, Nixpacks, Railpack, remote builds,
and container-grade isolation are not provided. These require a compatible Linux
executor, which is outside the current Android-only architecture. Projects that
require Linux-only dependencies, unsupported native libraries, x86-only binaries,
or unavailable SDKs cannot be built merely because their language is detected.

## Git deployment integration

Git deployment stages commits in immutable release directories and runs the
Analyzer/Build Engine against the staged project root before activation. Detected
install/build commands are used by default; a saved build command can override the
detected build step. The active release is not replaced when build or readiness
checks fail.

Built-in PHP, Go, Static and Node.js runtimes retain their existing launch behavior.
The **Custom Language App** runtime can launch any installed Termux tool or an
executable file inside the release directory through an explicit shell-free
`start_command`. The process must bind to `127.0.0.1` and the configured `PORT`
so the Agent readiness check can verify it. The Agent injects `HOST=127.0.0.1`
and `PORT` into custom runtime processes. Custom runtime restarts and rollbacks
reuse the saved start command.
