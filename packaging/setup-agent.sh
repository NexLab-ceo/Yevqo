#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
DATA_ROOT="${VELTRIX_AGENT_DATA_ROOT:-/sdcard/veltrix-agent}"
CONFIG_FILE="$DATA_ROOT/config/agent.json"
SERVICE_DIR="$PREFIX/var/service/veltrix-agent"
BOOT_DIR="$HOME/.termux/boot"

log(){ printf '==> %s\n' "$*"; }
warn(){ printf 'WARNING: %s\n' "$*" >&2; }

if [ ! -d /sdcard ]; then
  warn "Shared storage is not mounted. Run: termux-setup-storage"
  exit 1
fi
mkdir -p "$DATA_ROOT"/{config,sites,databases,logs,cloudflared,backups}
chmod 700 "$DATA_ROOT" "$DATA_ROOT/config" "$DATA_ROOT/backups" 2>/dev/null || true

if [ ! -s "$CONFIG_FILE" ]; then
  printf 'VELTRIX Agent setup\n'
  read -r -p 'Control Plane WebSocket URL (example: ws://192.168.1.10:8080/ws): ' SERVER_URL
  case "$SERVER_URL" in ws://*|wss://*) ;; *) echo "URL must start with ws:// or wss://" >&2; exit 1;; esac
  read -r -s -p 'Device token from the VELTRIX panel: ' TOKEN
  printf '\n'
  [ -n "$TOKEN" ] || { echo "Token is required" >&2; exit 1; }
  DEVICE_ID="$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
  HOSTNAME_VALUE="$(getprop net.hostname 2>/dev/null || hostname 2>/dev/null || echo android-device)"
  export SERVER_URL TOKEN DEVICE_ID DATA_ROOT HOSTNAME_VALUE CONFIG_FILE
  "$PREFIX/bin/veltrix-agent-config-write"
else
  log "Existing Agent config preserved: $CONFIG_FILE"
fi

mkdir -p "$SERVICE_DIR"
cat > "$SERVICE_DIR/run" <<'RUN'
#!/data/data/com.termux/files/usr/bin/sh
set -e
exec 2>&1
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
BIN="$PREFIX/bin/veltrix-agent"
CONFIG="${VELTRIX_AGENT_CONFIG:-/sdcard/veltrix-agent/config/agent.json}"
[ -x "$BIN" ] || { echo "Agent binary not found: $BIN"; exit 1; }
[ -s "$CONFIG" ] || { echo "Agent config missing: $CONFIG"; exit 1; }
exec "$BIN" -config "$CONFIG"
RUN
chmod 700 "$SERVICE_DIR/run"
sv-enable veltrix-agent >/dev/null 2>&1 || true

mkdir -p "$BOOT_DIR"
if [ -f "$PREFIX/share/veltrix/termux-boot.sh" ]; then
  cp "$PREFIX/share/veltrix/termux-boot.sh" "$BOOT_DIR/00-veltrix"
  chmod 700 "$BOOT_DIR/00-veltrix"
fi
if command -v sv >/dev/null 2>&1; then sv up veltrix-agent || warn "Could not start Agent service now"; fi

log "Agent configured."
log "Config: $CONFIG_FILE"
log "Status: sv status veltrix-agent"
log "Logs: svlogtail veltrix-agent"
if ! command -v cloudflared >/dev/null 2>&1; then
  log "cloudflared is optional and was not installed."
fi
