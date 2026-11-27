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

**Important: publishing is currently a manual, separate step.** A push to `main` runs the CI build and uploads both packages plus `SHA256SUMS` as a GitHub Actions artifact. The current workflows do not create or upload a GitHub Release automatically. Pushing a version tag alone does not publish a release.

To prepare a stable package version:

1. Create and push a version tag from the verified `main` commit, for example:

   ```sh
   git tag v0.2.0
   git push origin v0.2.0
   ```

2. Open the Yevqo repository's Actions page and manually run the **Build Yevqo** workflow with the ref set to that tag (for example, `v0.2.0`). Do not use an artifact from a different commit or tag.
3. Wait for the workflow to complete successfully. Download its artifact, which contains:
   - `yevqo-all_0.2.0_aarch64.deb`
   - `yevqo_0.2.0_aarch64.deb`
   - `SHA256SUMS`
4. Verify the checksums, then create the matching `v0.2.0` release in [NexLab-ceo/veltrix-dock](https://github.com/NexLab-ceo/veltrix-dock/releases) and attach all three files without renaming them.
5. Confirm the release is public and both package assets plus `SHA256SUMS` are downloadable before announcing the update.

The current workflow does not use a `PUBLIC_RELEASE_TOKEN` secret. That secret will only be needed if automated publishing to `veltrix-dock` is implemented later. Never publish an artifact from a failed or unverified build.
