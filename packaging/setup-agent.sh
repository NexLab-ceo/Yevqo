#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
DATA_ROOT="${VELTRIX_AGENT_DATA_ROOT:-/sdcard/yevqo-agent}"
PRIVATE_DATA_ROOT="$PREFIX/var/lib/veltrix-agent"
CONFIG_DIR="$PRIVATE_DATA_ROOT/config"
CONFIG_FILE="$CONFIG_DIR/agent.json"
LEGACY_CONFIG_FILE="$DATA_ROOT/config/agent.json"
SETUP_COMMAND="$(basename "$0")"
if [ "$SETUP_COMMAND" = "yevqo-setup" ]; then
  SERVICE_NAME="yevqo"
  AGENT_BIN="yevqo"
else
  SERVICE_NAME="yevqo-agent"
  AGENT_BIN="yevqo-agent"
fi
SERVICE_DIR="$PREFIX/var/service/$SERVICE_NAME"
BOOT_DIR="$HOME/.termux/boot"

log(){ printf '==> %s
' "$*"; }
warn(){ printf 'WARNING: %s
' "$*" >&2; }

if [ ! -d /sdcard ]; then
  warn "Shared storage is not mounted. Run: termux-setup-storage"
  exit 1
fi
mkdir -p "$DATA_ROOT"/{config,sites,databases,logs,cloudflared,backups} "$CONFIG_DIR"
chmod 700 "$DATA_ROOT" "$DATA_ROOT/config" "$DATA_ROOT/backups" "$PRIVATE_DATA_ROOT" "$CONFIG_DIR" 2>/dev/null || true

# Move any older shared-storage config (which may expose the pairing token) into Termux private storage.
if [ ! -s "$CONFIG_FILE" ] && [ -s "$LEGACY_CONFIG_FILE" ]; then
  install -m 0600 "$LEGACY_CONFIG_FILE" "$CONFIG_FILE"
  if jq -e '.server_url and .token and .device_id' "$CONFIG_FILE" >/dev/null 2>&1; then
    rm -f "$LEGACY_CONFIG_FILE"
    log "Migrated Agent credentials into private Termux storage."
  else
    rm -f "$CONFIG_FILE"
    echo "Existing Agent config is invalid; it was left untouched at $LEGACY_CONFIG_FILE" >&2
    exit 1
  fi
fi

if [ ! -s "$CONFIG_FILE" ]; then
  printf 'Yevqo setup
'
  read -r -p 'Control Plane WebSocket URL (example: ws://192.168.1.10:8080/ws): ' SERVER_URL
  case "$SERVER_URL" in ws://*|wss://*) ;; *) echo "URL must start with ws:// or wss://" >&2; exit 1;; esac
  read -r -s -p 'Agent pairing code from Profile > Agent pairing code: ' TOKEN
  printf '
'
  [ -n "$TOKEN" ] || { echo "Token is required" >&2; exit 1; }
  DEVICE_ID="$(od -An -N16 -tx1 /dev/urandom | tr -d ' 
')"
  HOSTNAME_VALUE="$(getprop net.hostname 2>/dev/null || hostname 2>/dev/null || echo android-device)"
  json_escape() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }
  SERVER_JSON="$(json_escape "$SERVER_URL")"
  TOKEN_JSON="$(json_escape "$TOKEN")"
  DEVICE_JSON="$(json_escape "$DEVICE_ID")"
  HOST_JSON="$(json_escape "$HOSTNAME_VALUE")"
  DATA_JSON="$(json_escape "$DATA_ROOT")"
  cat > "$CONFIG_FILE" <<JSON
{
  "server_url": "$SERVER_JSON",
  "token": "$TOKEN_JSON",
  "device_id": "$DEVICE_JSON",
  "hostname": "$HOST_JSON",
  "data_root": "$DATA_JSON"
}
JSON
  chmod 600 "$CONFIG_FILE"
else
  log "Existing Agent config preserved: $CONFIG_FILE"
fi

if [ "$SERVICE_NAME" = "yevqo" ] && [ -d "$PREFIX/var/service/yevqo-agent" ]; then
  sv down yevqo-agent >/dev/null 2>&1 || true
  sv-disable yevqo-agent >/dev/null 2>&1 || true
  mv "$PREFIX/var/service/yevqo-agent" "$PREFIX/var/service/yevqo" 2>/dev/null || true
fi
mkdir -p "$SERVICE_DIR"
cat > "$SERVICE_DIR/run" <<'RUN'
#!/data/data/com.termux/files/usr/bin/sh
set -e
exec 2>&1
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
BIN="$PREFIX/bin/__YEVQO_AGENT_BIN__"
CONFIG="${VELTRIX_AGENT_CONFIG:-$PREFIX/var/lib/veltrix-agent/config/agent.json}"
[ -x "$BIN" ] || { echo "Yevqo binary not found: $BIN"; exit 1; }
[ -s "$CONFIG" ] || { echo "Agent config missing: $CONFIG"; exit 1; }
exec "$BIN" -config "$CONFIG"
RUN
sed -i "s|__YEVQO_AGENT_BIN__|$AGENT_BIN|g" "$SERVICE_DIR/run"
chmod 700 "$SERVICE_DIR/run"
sv-enable "$SERVICE_NAME" >/dev/null 2>&1 || true

mkdir -p "$BOOT_DIR"
if [ -e "$BOOT_DIR/00-veltrix" ] && [ ! -e "$BOOT_DIR/00-yevqo" ]; then mv "$BOOT_DIR/00-veltrix" "$BOOT_DIR/00-yevqo"; fi
if [ ! -s "$BOOT_DIR/00-yevqo" ] || grep -q "Yevqo Agent boot hook started" "$BOOT_DIR/00-yevqo" 2>/dev/null; then
  cat > "$BOOT_DIR/00-yevqo" <<'BOOT'
#!/data/data/com.termux/files/usr/bin/bash
set -u
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
LOG_DIR="${HOME}/.local/var/log"
mkdir -p "$LOG_DIR"
exec >>"$LOG_DIR/yevqo-boot.log" 2>&1
echo "[$(date -Iseconds)] Yevqo boot hook started"
command -v termux-wake-lock >/dev/null 2>&1 && termux-wake-lock || true
for service in yevqo-control yevqo-agent yevqo; do
  if [ -d "$PREFIX/var/service/$service" ] || [ -L "$PREFIX/var/service/$service" ]; then
    sv up "$service" || echo "Could not start $service"
  fi
done
echo "[$(date -Iseconds)] Yevqo boot hook finished"
BOOT
  chmod 700 "$BOOT_DIR/00-yevqo"
else
  log "Existing Termux boot hook preserved: $BOOT_DIR/00-yevqo"
fi
if command -v sv >/dev/null 2>&1; then sv up "$SERVICE_NAME" || warn "Could not start Yevqo service now"; fi

log "Yevqo configured."
log "Config: $CONFIG_FILE"
log "Command: $AGENT_BIN"
log "Status: sv status $SERVICE_NAME"
log "Logs: svlogtail $SERVICE_NAME"
if ! command -v cloudflared >/dev/null 2>&1; then
  log "cloudflared is optional and was not installed."
fi
