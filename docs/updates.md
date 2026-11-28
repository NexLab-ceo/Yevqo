# Yevqo package updates

**Automatic updates are supported only for the standalone Yevqo Agent. Yevqo All does not auto-update and is not published through the Admin Agent package channel.** The Admin update channel is intentionally Agent-only.

## Standalone Agent: publish from Admin Panel

1. Build the standalone ARM64 package `yevqo_VERSION_aarch64.deb` using the verified Yevqo build.
2. Open Admin → Agent packages, choose that `.deb`, then upload and publish it.
3. The Control Plane validates Debian metadata (`Package: yevqo`, `Architecture: aarch64`), accepts only a newer version, computes SHA-256 and stores the immutable package under its own data root.
4. The Agent updater reads the Control Plane URL from the device's existing Agent config, checks `/updates/agent/latest.json`, downloads the versioned package from the same Control Plane and verifies SHA-256 plus Debian metadata before installation.

The upload endpoint is administrator-only. Published update metadata and package files are public read-only endpoints so paired devices can download them without exposing device credentials. Upload size is limited to 90 MiB.

## Automatic Agent updates

Only the standalone Agent package installs the `yevqo-updater` Termux service. It waits 30 seconds after package installation, then checks the Admin-published Agent version every six hours by default.

The service invokes only `yevqo-update`. It does not query GitHub Releases and contains no fallback to Yevqo All. The updater verifies the published SHA-256 and Debian package name, architecture and version before installing a newer Agent package.

Android must allow Termux to run in the background. Battery optimization or Android process restrictions can delay checks. The default interval is six hours; values below 15 minutes are ignored.

## Yevqo All

Yevqo All has no automatic updater, no `yevqo-update-all` command and no Admin-managed update channel. Its post-install hook also removes and disables the legacy `yevqo-updater` service and stale updater binaries when upgrading an older installation.

## Agent service controls

Check status:

```sh
sv status yevqo-updater
```

Temporarily disable Agent automatic updates:

```sh
sv down yevqo-updater
```

Enable Agent automatic updates again:

```sh
sv up yevqo-updater
```

Run an Agent update check immediately:

```sh
yevqo-update
```

