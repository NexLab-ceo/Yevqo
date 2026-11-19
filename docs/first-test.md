# Yevqo — first Android test checklist

This is a first-run smoke test for a **test installation** on Android + Termux. It is not a production deployment guide and does not replace the physical-device test record.

## Before installing

- Use a supported Android device with current Termux from F-Droid or the official Termux GitHub releases. Do not mix Termux with add-ons installed from a different source.
- Keep the phone connected to power and temporarily disable battery optimization for Termux during the test.
- Make sure there is enough free storage for Go/Termux dependencies, MariaDB, the application and build caches.
- Back up any existing Yevqo/legacy Veltrix data first. Do not use a device with valuable existing databases for the first clean-install test.
- Grant shared-storage access when prompted:

```sh
termux-setup-storage
```

## Install the current CI package

1. Open the successful [Yevqo build workflow run #989](https://github.com/NexLab-ceo/veltrix/actions/runs/44488615633).
2. Download the `veltrix-android-aarch64-989` artifact and extract the ZIP in Termux-accessible storage.
3. Install the all-in-one package (the artifact contains both all-in-one and Agent-only packages):

```sh
cd ~/storage/downloads
dpkg -i ./yevqo-all_0.1.0-dev.977_aarch64.deb
```

If the ZIP was extracted to another directory, change the `cd` path accordingly. The package targets Android/Termux aarch64. The Agent-only package is also included in the artifact; both packages now carry the same shared Termux boot hook.

## First Control Plane start

Run the interactive setup command:

```sh
yevqo-setup
```

Choose **Control Plane** for a single-phone first smoke test. Complete the prompts. The setup creates a local MariaDB database/config only when needed and preserves an existing Control Plane config. Do not delete or manually replace the data directory to fix setup errors.

After the dashboard loads, sign in or register. Open **Profile → Agent pairing code** and copy the code. Then configure the Agent on this same phone:

```sh
yevqo-setup-agent
```

Use `ws://127.0.0.1:8080/ws` as the Control Plane WebSocket URL and paste the pairing code when prompted. The pairing code is the account's referral/authentication token; it is not a separate per-device token. Keep it private. The installer stores Agent credentials in Termux's private app storage (not shared `/sdcard` storage) and migrates a valid older shared-storage config into the private location. Existing Agent config is preserved.

Check service state and logs:

```sh
sv status yevqo-control
svlogtail yevqo-control
```

Open the local dashboard on the same phone:

```
http://127.0.0.1:8080
```

## Smoke-test sequence

Record PASS/FAIL and the exact error for each step:

- [ ] Package installs without dpkg errors.
- [ ] `yevqo-setup` launches and completes.
- [ ] MariaDB responds and the Control Plane service reaches `run`.
- [ ] Dashboard loads at `http://127.0.0.1:8080`.
- [ ] Account/session flow works; verify sign-in and sign-out.
- [ ] A new test site can be created without overwriting an existing directory.
- [ ] The Agent can be paired with the Control Plane on the same phone.
- [ ] Agent status changes to online and remains connected for at least 10 minutes.
- [ ] A simple static site can be built/deployed and opened locally.
- [ ] Stop/start the site and verify status and logs.
- [ ] Restart the Control Plane and Agent services; verify the site state is reconciled.
- [ ] Reboot the phone only after the above passes; verify Termux:Boot behavior separately.

Do not mark a step PASS based only on a UI status if the corresponding service or site was not actually reachable.

## Capture test evidence

For each failed or successful step, record:

- Android version, device model and CPU architecture (`uname -m`)
- Termux version and package source
- Yevqo package version
- Step number, expected result and actual result
- Relevant logs with tokens, passwords, cookies and private URLs removed

Useful diagnostics:

```sh
uname -a
uname -m
pkg list-installed | grep -E 'termux-services|mariadb|golang|php|git'
sv status yevqo-control
sv status yevqo-agent
```

## Important limitations

- A green GitHub Actions run verifies Go tests, Android ARM64 cross-compilation, frontend build and Debian package assembly/metadata. It does **not** prove that the app starts on a real Android device.
- The first test should stay local. Do not configure a public domain or Cloudflare Tunnel until the local Control Plane, Agent and test site pass.
- The Build Engine runs project code with the Termux user's permissions; only build repositories you trust.
- This checklist does not claim the first physical Android test has already passed.
