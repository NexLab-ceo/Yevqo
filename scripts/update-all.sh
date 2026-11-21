#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail

REPO="NexLab-ceo/veltrix"
API="https://api.github.com/repos/$REPO/releases/latest"
TOKEN_FILE="/sdcard/yevqo/config/github-token"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

command -v curl >/dev/null || { echo "Install curl: pkg install curl"; exit 1; }
command -v jq >/dev/null || { echo "Install jq: pkg install jq"; exit 1; }
[ "$(uname -m)" = "aarch64" ] || { echo "YEVQO All package requires Android ARM64"; exit 1; }
[ -s "$TOKEN_FILE" ] || { echo "Private GitHub token missing: $TOKEN_FILE"; echo "Create it with: mkdir -p $(dirname '$TOKEN_FILE'); nano '$TOKEN_FILE'"; exit 1; }
TOKEN="$(cat "$TOKEN_FILE")"
AUTH="Authorization: Bearer $TOKEN"
URL="$(curl -fsSL -H "$AUTH" -H 'Accept: application/vnd.github+json' "$API" | jq -r '.assets[] | select(.name | startswith("yevqo-all_") and endswith("_aarch64.deb")) | .browser_download_url' | head -n1)"
[ -n "$URL" ] && [ "$URL" != "null" ] || { echo "No yevqo-all .deb in latest private release"; exit 1; }

curl -fL --progress-bar -H "$AUTH" "$URL" -o "$TMP/yevqo-all.deb"
dpkg-deb --field "$TMP/yevqo-all.deb" Package | grep -qx yevqo-all
dpkg-deb --field "$TMP/yevqo-all.deb" Architecture | grep -qx aarch64
dpkg -i "$TMP/yevqo-all.deb"
echo "YEVQO All updated. Control/Agent configs and /sdcard data remain in place."
