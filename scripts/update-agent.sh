#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail

REPO="NexLab-ceo/veltrix-dock"
API="https://api.github.com/repos/$REPO/releases/latest"
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

command -v curl >/dev/null || { echo "Install curl: pkg install curl"; exit 1; }
command -v jq >/dev/null || { echo "Install jq: pkg install jq"; exit 1; }
[ "$(uname -m)" = "aarch64" ] || { echo "YEVQO Agent package requires Android ARM64"; exit 1; }

URL="$(curl -fsSL "$API" | jq -r '.assets[] | select(.name | startswith("yevqo-agent_") and endswith("_aarch64.deb")) | .browser_download_url' | head -n1)"
[ -n "$URL" ] && [ "$URL" != "null" ] || { echo "No Agent .deb in latest public release"; exit 1; }

curl -fL --progress-bar "$URL" -o "$TMP/agent.deb"
dpkg-deb --field "$TMP/agent.deb" Package | grep -qx yevqo-agent
dpkg-deb --field "$TMP/agent.deb" Architecture | grep -qx aarch64
dpkg -i "$TMP/agent.deb"
echo "YEVQO Agent updated. Existing config/data were not deleted."
