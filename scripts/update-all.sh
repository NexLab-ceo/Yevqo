#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail

REPO="NexLab-ceo/veltrix-dock"
API="https://api.github.com/repos/$REPO/releases/latest"
PREFIX="\${PREFIX:-/data/data/com.termux/files/usr}"
PACKAGE="yevqo-all"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

command -v curl >/dev/null || { echo "Install curl: pkg install curl" >&2; exit 1; }
command -v jq >/dev/null || { echo "Install jq: pkg install jq" >&2; exit 1; }
command -v dpkg-deb >/dev/null || { echo "dpkg-deb is unavailable" >&2; exit 1; }
if ! dpkg-query -W -f='\${Status}' coreutils 2>/dev/null | grep -q 'install ok installed'; then
  command -v pkg >/dev/null || { echo "Install coreutils: pkg install coreutils" >&2; exit 1; }
  pkg install -y coreutils
fi
[ "$(uname -m)" = "aarch64" ] || { echo "Yevqo All requires Android ARM64" >&2; exit 1; }

CURRENT="$(dpkg-query -W -f='\${Version}' "$PACKAGE" 2>/dev/null || true)"
curl -fsSL --connect-timeout 20 --max-time 300 --retry 3 -H 'Accept: application/vnd.github+json' -H 'User-Agent: Yevqo-Updater' "$API" -o "$TMP/release.json"
TAG="$(jq -er '.tag_name' "$TMP/release.json")"
LATEST="\${TAG#v}"
case "$LATEST" in
  [0-9]*) ;;
  *) echo "Latest release tag '$TAG' is not a Debian package version; refusing to update." >&2; exit 1 ;;
esac
ASSET="\${PACKAGE}_\${LATEST}_aarch64.deb"
if ! jq -e --arg name "$ASSET" '[.assets[]? | select(.name == $name)] | length == 1' "$TMP/release.json" >/dev/null; then
  echo "Latest release '$TAG' is missing the expected package asset: $ASSET. Refusing to update." >&2
  exit 1
fi
if ! jq -e '[.assets[]? | select(.name == "SHA256SUMS")] | length == 1' "$TMP/release.json" >/dev/null; then
  echo "Latest release '$TAG' is missing SHA256SUMS. Refusing to install an unverified package." >&2
  exit 1
fi
URL="$(jq -er --arg name "$ASSET" '.assets[] | select(.name == $name) | .browser_download_url' "$TMP/release.json")"
CHECKSUM_URL="$(jq -er '.assets[] | select(.name == "SHA256SUMS") | .browser_download_url' "$TMP/release.json")"

if [ -n "$CURRENT" ] && dpkg --compare-versions "$LATEST" le "$CURRENT"; then
  echo "Yevqo All is up to date ($CURRENT)."
  exit 0
fi

curl -fL --connect-timeout 20 --max-time 300 --retry 3 --progress-bar "$URL" -o "$TMP/$ASSET"
curl -fsSL --connect-timeout 20 --max-time 300 --retry 3 "$CHECKSUM_URL" -o "$TMP/SHA256SUMS"
EXPECTED="$(awk -v file="$ASSET" '$2 == file { print $1; exit }' "$TMP/SHA256SUMS")"
ACTUAL="$(sha256sum "$TMP/$ASSET" | awk '{ print $1 }')"
[ -n "$EXPECTED" ] && [ "$EXPECTED" = "$ACTUAL" ] || { echo "Yevqo All package checksum verification failed" >&2; exit 1; }

dpkg-deb --field "$TMP/$ASSET" Package | grep -Fx "$PACKAGE" >/dev/null
dpkg-deb --field "$TMP/$ASSET" Architecture | grep -Fx aarch64 >/dev/null
dpkg-deb --field "$TMP/$ASSET" Version | grep -Fx "$LATEST" >/dev/null

echo "Installing Yevqo All $LATEST (current: \${CURRENT:-not registered})"
YEVQO_NONINTERACTIVE=1 dpkg -i "$TMP/$ASSET"

if command -v sv >/dev/null 2>&1; then
  for service in yevqo-control yevqo-agent yevqo; do
    if [ -d "$PREFIX/var/service/$service" ] || [ -L "$PREFIX/var/service/$service" ]; then
      sv restart "$service" >/dev/null 2>&1 || sv up "$service" >/dev/null 2>&1 || true
    fi
  done
fi

echo "Yevqo All updated to $LATEST. Existing configuration, database and /sdcard data were preserved."
