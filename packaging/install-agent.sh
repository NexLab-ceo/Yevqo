#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
umask 077

PAIRING_CODE="${1:-}"
API_BASE="https://app.yevqo.cc.cd"
RELEASE_API="https://api.github.com/repos/NexLab-ceo/veltrix-dock/releases/latest"
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"; unset PAIRING_CODE' EXIT

fail(){ printf 'Yevqo installer: %s\n' "$*" >&2; exit 1; }
[ -n "$PAIRING_CODE" ] || fail "Missing one-time pairing code. Generate a fresh command in Yevqo."
[ "$(uname -m)" = "aarch64" ] || fail "Only Android ARM64 is currently supported."
command -v pkg >/dev/null 2>&1 || fail "Open this command inside Termux."

printf '==> Installing required Termux packages\n'
pkg update -y
pkg install -y curl jq coreutils php git termux-services
if [ ! -w /sdcard ] && command -v termux-setup-storage >/dev/null 2>&1; then
  printf '==> Requesting Android shared-storage permission\\n'
  termux-setup-storage || true
  for attempt in 1 2 3 4 5 6 7 8 9 10; do
    [ -w /sdcard ] && break
    sleep 1
  done
fi
[ -w /sdcard ] || fail "Grant Termux shared-storage access with termux-setup-storage, then run a fresh pairing command."
command -v dpkg-deb >/dev/null 2>&1 || fail "Termux dpkg is unavailable."

printf '==> Resolving the latest verified Yevqo Agent package\n'
curl -fsSL --connect-timeout 20 --max-time 120 --retry 3 -H 'Accept: application/vnd.github+json' -H 'User-Agent: Yevqo-Installer' "$RELEASE_API" -o "$TMP/release.json"
TAG="$(jq -er '.tag_name' "$TMP/release.json")"
VERSION="${TAG#v}"
case "$VERSION" in [0-9]*) ;; *) fail "Latest release tag is not a valid package version." ;; esac
ASSET="yevqo_${VERSION}_aarch64.deb"
jq -e --arg name "$ASSET" '[.assets[]? | select(.name == $name)] | length == 1' "$TMP/release.json" >/dev/null || fail "Latest release is missing $ASSET."
jq -e '[.assets[]? | select(.name == "SHA256SUMS")] | length == 1' "$TMP/release.json" >/dev/null || fail "Latest release has no SHA256SUMS."
PACKAGE_URL="$(jq -er --arg name "$ASSET" '.assets[] | select(.name == $name) | .browser_download_url' "$TMP/release.json")"
SUMS_URL="$(jq -er '.assets[] | select(.name == "SHA256SUMS") | .browser_download_url' "$TMP/release.json")"
curl -fL --connect-timeout 20 --max-time 300 --retry 3 "$PACKAGE_URL" -o "$TMP/$ASSET"
curl -fsSL --connect-timeout 20 --max-time 120 --retry 3 "$SUMS_URL" -o "$TMP/SHA256SUMS"
EXPECTED="$(awk -v file="$ASSET" '$2 == file {print $1; exit}' "$TMP/SHA256SUMS")"
ACTUAL="$(sha256sum "$TMP/$ASSET" | awk '{print $1}')"
[ -n "$EXPECTED" ] && [ "$EXPECTED" = "$ACTUAL" ] || fail "Package SHA-256 verification failed."
[ "$(dpkg-deb --field "$TMP/$ASSET" Package)" = "yevqo" ] || fail "Unexpected package name."
[ "$(dpkg-deb --field "$TMP/$ASSET" Architecture)" = "aarch64" ] || fail "Unexpected package architecture."
[ "$(dpkg-deb --field "$TMP/$ASSET" Version)" = "$VERSION" ] || fail "Unexpected package version."

printf '==> Installing Yevqo Agent %s\n' "$VERSION"
YEVQO_NONINTERACTIVE=1 dpkg -i "$TMP/$ASSET"
PRIVATE_ROOT="$PREFIX/var/lib/veltrix-agent"
CONFIG_DIR="$PRIVATE_ROOT/config"
CONFIG_FILE="$CONFIG_DIR/agent.json"
mkdir -p "$CONFIG_DIR"
chmod 700 "$PRIVATE_ROOT" "$CONFIG_DIR"
if [ -s "$CONFIG_FILE" ]; then
  fail "An Agent config already exists. Existing credentials were preserved; use the normal reconnect/update flow instead."
fi
DEVICE_ID="$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
HOSTNAME_VALUE="$(getprop net.hostname 2>/dev/null || hostname 2>/dev/null || echo android-device)"
printf '==> Claiming one-time pairing code\n'
jq -n --arg code "$PAIRING_CODE" --arg device_id "$DEVICE_ID" '{code:$code,device_id:$device_id}' > "$TMP/claim.json"
unset PAIRING_CODE
HTTP_CODE="$(curl -sS --connect-timeout 20 --max-time 60 -H 'Content-Type: application/json' -X POST --data-binary @"$TMP/claim.json" -o "$TMP/claim-response.json" -w '%{http_code}' "$API_BASE/api/device-pairing/claim")" || fail "Could not reach Yevqo Control Plane."
[ "$HTTP_CODE" = "200" ] || fail "$(jq -r '.error // "Pairing code was rejected or expired."' "$TMP/claim-response.json")"
TOKEN="$(jq -er '.token' "$TMP/claim-response.json")"
[ "${#TOKEN}" -eq 64 ] || fail "Control Plane returned an invalid device credential."
SERVER_URL="wss://app.yevqo.cc.cd/ws"
DATA_ROOT="/sdcard/yevqo-agent"
jq -n --arg server_url "$SERVER_URL" --arg token "$TOKEN" --arg device_id "$DEVICE_ID" --arg hostname "$HOSTNAME_VALUE" --arg data_root "$DATA_ROOT" '{server_url:$server_url,token:$token,device_id:$device_id,hostname:$hostname,data_root:$data_root}' > "$CONFIG_FILE"
chmod 600 "$CONFIG_FILE"
unset TOKEN
printf '==> Configuring and starting Agent service\n'
"$PREFIX/bin/yevqo-setup"
printf '\nYevqo Agent installed. Check the Devices page for the Online status.\n'
