# Yevqo package updates

Yevqo packages are distributed through the public GitHub Releases page in [NexLab-ceo/veltrix-dock](https://github.com/NexLab-ceo/veltrix-dock/releases). The source code and CI workflows live in [NexLab-ceo/Yevqo](https://github.com/NexLab-ceo/Yevqo).

## Automatic updates

After package installation, the Debian post-install hook enables the `yevqo-updater` Termux service. It waits 30 seconds for package configuration to finish, checks the latest stable release, and then repeats the check every six hours.

The updater:

1. Reads the latest stable release tag from the public VELTRIX Dock repository.
2. Looks for the package matching the installed edition (`yevqo-all` or `yevqo`) and Android `aarch64`.
3. Downloads the package and the release's `SHA256SUMS` file.
4. Verifies the checksum, Debian package name, architecture and embedded version before installation.
5. Installs only a newer version and restarts the installed Yevqo services.

If the latest release tag is not a Debian version, the expected package asset is missing, or `SHA256SUMS` is absent, the updater stops with an explicit error. It never falls back to installing an unverified asset.

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

## Building and publishing a release

Versioned tag builds publish automatically to the public Releases page in `NexLab-ceo/veltrix-dock`. A push to `main` still runs verification and uploads a CI artifact only; it does not publish a release.

To prepare a stable package version:

1. Create a version tag from the verified `main` commit, for example:

   ```sh
   git tag v0.2.0
   ```

2. Push the tag. This automatically starts **Build Yevqo** for that exact tag:

   ```sh
   git push origin v0.2.0
   ```

   The workflow runs Go tests, builds the Android ARM64 binaries and frontend, assembles and verifies both Debian packages, creates `SHA256SUMS`, and uploads a CI artifact.
3. The same successful tag workflow publishes the two packages and `SHA256SUMS` to the matching `v0.2.0` release in `veltrix-dock`. If that release already exists, its three assets are replaced with the newly verified build. The workflow fails closed if publishing credentials are missing or the release upload fails.
   - `yevqo-all_0.2.0_aarch64.deb`
   - `yevqo_0.2.0_aarch64.deb`
   - `SHA256SUMS`
4. Confirm the workflow's **Publish verified packages to release repository** step succeeded and that the matching release exposes both package assets and `SHA256SUMS`. Never announce a release from a failed or incomplete workflow.

### Required repository secret

In the **Yevqo source repository** settings, configure `PUBLIC_RELEASE_TOKEN` as an Actions secret. It must be a GitHub token with **Contents: Read and write** access to `NexLab-ceo/veltrix-dock`. The token is used only by the release-publishing step; it is not embedded in packages or sent to Android devices. Do not paste the token into source code, issues, or chat.

Never publish an artifact from a failed or unverified build.
