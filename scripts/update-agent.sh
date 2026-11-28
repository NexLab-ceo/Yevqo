#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail

PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
PACKAGE="yevqo"
CONFIG="${YEVQO_AGENT_CONFIG:-$PREFIX/var/lib/veltrix-agent/config/agent.json}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

command -v curl >/dev/null || { echo "Install curl: pkg install curl" >&2; exit 1; }
command -v jq >/dev/null || { echo "Install jq: pkg install jq" >&2; exit 1; }
command -v dpkg-deb >/dev/null || { echo "dpkg-deb is unavailable" >&2; exit 1; }
[ "$(uname -m)" = "aarch64" ] || { echo "Yevqo Agent requires Android ARM64" >&2; exit 1; }

# Prefer an explicit override; otherwise derive the Control Plane origin from
# the Agent's configured ws(s):// URL. No GitHub Release lookup is used.
if [ -n "${YEVQO_UPDATE_BASE_URL:-}" ]; then
  BASE="${YEVQO_UPDATE_BASE_URL%/}"
else
  [ -f "$CONFIG" ] || { echo "Agent config not found: $CONFIG" >&2; exit 1; }
  SERVER_URL="$(jq -er '.server_url' "$CONFIG")"
  case "$SERVER_URL" in
    wss://*) HOST="${SERVER_URL#wss://}"; HOST="${HOST%%/*}"; BASE="https://$HOST" ;;
    ws://*) HOST="${SERVER_URL#ws://}"; HOST="${HOST%%/*}"; BASE="http://$HOST" ;;
    https://*) HOST="${SERVER_URL#https://}"; HOST="${HOST%%/*}"; BASE="https://$HOST" ;;
    http://*) HOST="${SERVER_URL#http://}"; HOST="${HOST%%/*}"; BASE="http://$HOST" ;;
    *) echo "Unsupported Agent server_url scheme" >&2; exit 1 ;;
  esac
fi

CURRENT="$(dpkg-query -W -f='${Version}' "$PACKAGE" 2>/dev/null || true)"
curl -fsSL --connect-timeout 20 --max-time 60 --retry 3 "$BASE/updates/agent/latest.json" -o "$TMP/latest.json"
LATEST="$(jq -er '.version' "$TMP/latest.json")"
ASSET="$(jq -er '.filename' "$TMP/latest.json")"
EXPECTED="$(jq -er '.sha256' "$TMP/latest.json")"
case "$ASSET" in yevqo_*_aarch64.deb) ;; *) echo "Invalid Agent package filename from update server" >&2; exit 1 ;; esac
printf '%s' "$EXPECTED" | grep -Eq '^[0-9a-f]{64}$' || { echo "Invalid package checksum metadata" >&2; exit 1; }

if [ -n "$CURRENT" ] && dpkg --compare-versions "$LATEST" le "$CURRENT"; then
  echo "Yevqo Agent is up to date ($CURRENT)."
  exit 0
fi

curl -fL --connect-timeout 20 --max-time 600 --retry 3 --progress-bar "$BASE/updates/agent/$ASSET" -o "$TMP/$ASSET"
ACTUAL="$(sha256sum "$TMP/$ASSET" | awk '{ print $1 }')"
[ "$EXPECTED" = "$ACTUAL" ] || { echo "Yevqo Agent package checksum verification failed" >&2; exit 1; }
dpkg-deb --field "$TMP/$ASSET" Package | grep -Fx "$PACKAGE" >/dev/null
dpkg-deb --field "$TMP/$ASSET" Architecture | grep -Fx aarch64 >/dev/null
dpkg-deb --field "$TMP/$ASSET" Version | grep -Fx "$LATEST" >/dev/null

echo "Installing Yevqo Agent $LATEST (current: ${CURRENT:-not registered})"
YEVQO_NONINTERACTIVE=1 dpkg -i "$TMP/$ASSET"
if command -v sv >/dev/null 2>&1 && { [ -d "$PREFIX/var/service/yevqo" ] || [ -L "$PREFIX/var/service/yevqo" ]; }; then
  sv restart yevqo >/dev/null 2>&1 || sv up yevqo >/dev/null 2>&1 || true
fi
echo "Yevqo Agent updated to $LATEST. Existing Agent configuration and data were preserved."
