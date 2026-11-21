# Yevqo package updates

Yevqo packages use the public GitHub Releases page in [NexLab-ceo/veltrix-dock](https://github.com/NexLab-ceo/veltrix-dock/releases).

## Automatic updates

After package installation, the Debian post-install hook enables the `yevqo-updater` Termux service. It waits 30 seconds for package configuration to finish, checks the latest stable release, and then repeats the check every six hours.

The updater:

1. Reads the latest stable release tag from the public VELTRIX Dock repository.
2. Looks for the package matching the installed edition (`yevqo-all` or `yevqo`) and Android `aarch64`.
3. Downloads the package and the release's `SHA256SUMS` file.
4. Verifies the checksum, Debian package name, architecture and embedded version before installation.
5. Installs only a newer version and restarts the installed Yevqo services.

The updater does not need a GitHub token on the Android device. It does need network access, and Android must allow Termux to run in the background. Battery optimization or Android process restrictions can delay checks. The default interval is six hours; values below 15 minutes are ignored.

## Service controls

Check status:

```sh
sv status yevqo-updater
```

Temporarily disable automatic updates:

```sh
sv down yevqo-updater
```

Enable automatic updates again:

```sh
sv up yevqo-updater
```

Run an update check immediately:

- All-in-one package: `yevqo-update-all`
- Standalone Agent: `yevqo-update`

The update commands are safe to run more than once. If the installed version is already current, they exit without reinstalling it.

## Publishing a release

The build workflow in the Yevqo source repository builds both Debian packages from the same version and attaches them, plus `SHA256SUMS`, to a release in `NexLab-ceo/veltrix-dock` when a `v*` tag is pushed.

The source repository needs a GitHub Actions secret named `PUBLIC_RELEASE_TOKEN`. It must be a token that can create releases and upload assets in `NexLab-ceo/veltrix-dock` (repository Contents: write). For a fine-grained token, grant access to that repository specifically. The token is used only by GitHub Actions; it is not embedded in the packages or sent to devices.

Create a versioned tag on the Yevqo `main` branch to publish a release, for example:

```sh
git tag v0.2.0
git push origin v0.2.0
```

Normal pushes to `main` still run the build and upload CI artifacts, but do not publish a stable GitHub Release.
