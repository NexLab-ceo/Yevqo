#!/data/data/com.termux/files/usr/bin/bash
# VELTRIX Dock — установка агента (Data Plane) на телефон пользователя.
# Запуск: bash scripts/install-agent.sh

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}▶${NC} $1"; }
warn() { echo -e "${YELLOW}⚠${NC} $1"; }
err()  { echo -e "${RED}✗${NC} $1"; }

# ============================================================
# 0. ПРОВЕРКИ
# ============================================================
if [ ! -f "go.mod" ]; then
    err "go.mod not found. Run from project root."
    exit 1
fi

DATA_ROOT="${VELTRIX_AGENT_DATA_ROOT:-$PREFIX/var/lib/veltrix-agent}"

log "VELTRIX Agent installer"
log "data root: $DATA_ROOT"

# ============================================================
# 1. ЗАВИСИМОСТИ
# ============================================================
log "installing dependencies..."

pkg update -y >/dev/null 2>&1 || true

if ! command -v go >/dev/null 2>&1; then
    log "  → installing Go"
    pkg install -y golang
fi

if ! command -v php >/dev/null 2>&1; then
    log "  → installing PHP"
    pkg install -y php
fi

if ! command -v git >/dev/null 2>&1; then
    log "  → installing Git"
    pkg install -y git
fi

if ! command -v tailscale >/dev/null 2>&1; then
    log "  → installing Tailscale"
    pkg install -y tailscale || warn "Tailscale install failed"
fi

if ! command -v cloudflared >/dev/null 2>&1; then
    log "  → installing cloudflared"
    pkg install -y cloudflared || warn "cloudflared not in repo, install manually"
fi

if ! command -v sv >/dev/null 2>&1; then
    log "  → installing termux-services"
    pkg install -y termux-services
fi

if ! command -v protoc >/dev/null 2>&1; then
    log "  → installing protobuf"
    pkg install -y protobuf
fi

# ============================================================
# 2. СТРУКТУРА ПАПОК
# ============================================================
log "creating directory structure..."
mkdir -p "$DATA_ROOT"/{config,sites,databases,logs,cloudflared,backups}
chmod 700 "$DATA_ROOT" "$DATA_ROOT/backups"

# ============================================================
# 3. КОНФИГ
# ============================================================
CONFIG_FILE="$DATA_ROOT/config/agent.json"

if [ ! -f "$CONFIG_FILE" ]; then
    log "creating agent.json"
    
    # Спрашиваем URL сервера и токен.
    read -p "Server WebSocket URL (ws://100.x.x.x:8080/ws): " SERVER_URL
    if [ -z "$SERVER_URL" ]; then
        err "server URL is required"
        exit 1
    fi
    
    read -p "Bind token (from your VELTRIX panel): " TOKEN
    if [ -z "$TOKEN" ]; then
        err "token is required"
        exit 1
    fi
    
    # device_id сгенерируется в LoadConfig при первом запуске.
    # Но мы можем сгенерировать сразу, чтобы не запускать дважды.
    DEVICE_ID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || \
                openssl rand -hex 16 2>/dev/null || \
                date +%s%N | sha256sum | head -c 32)
    
    cat > "$CONFIG_FILE" <<JSON
{
  "server_url": "$SERVER_URL",
  "token": "$TOKEN",
  "device_id": "$DEVICE_ID",
  "hostname": "",
  "data_root": "$DATA_ROOT"
}
JSON
    log "  config written to $CONFIG_FILE"
else
    log "  config already exists: $CONFIG_FILE"
fi

# ============================================================
# 4. СБОРКА АГЕНТА
# ============================================================
log "building agent binary..."

export PATH="$PATH:$(go env GOPATH)/bin"

# Генерируем protobuf (если ещё не сгенерирован).
if [ ! -f "proto/veltrix.pb.go" ]; then
    bash scripts/gen-proto.sh
fi

mkdir -p bin
go build -ldflags="-s -w" -o bin/veltrix-agent ./cmd/agent
log "  → bin/veltrix-agent ($(du -h bin/veltrix-agent | cut -f1))"

# ============================================================
# 5. УСТАНОВКА БИНАРНИКА
# ============================================================
log "installing binary to \$PREFIX/bin..."
cp bin/veltrix-agent "$PREFIX/bin/veltrix-agent"
chmod +x "$PREFIX/bin/veltrix-agent"

# ============================================================
# 6. TERMUX-SERVICES
# ============================================================
log "setting up service..."
SERVICE_DIR="$PREFIX/var/service/veltrix-agent"
mkdir -p "$SERVICE_DIR"
cp systemd/veltrix-agent.sh "$SERVICE_DIR/run"
chmod +x "$SERVICE_DIR/run"

sv-enable veltrix-agent 2>/dev/null || true

# ============================================================
# 7. TAILSCALE
# ============================================================
if command -v tailscale >/dev/null 2>&1; then
    if ! tailscale status >/dev/null 2>&1; then
        log ""
        log "Tailscale is installed but not connected."
        log "Run: tailscale up"
        log ""
    fi
fi

# ============================================================
# 8. НАПОМИНАНИЕ ПРО CLOUDFLARED
# ============================================================
if ! command -v cloudflared >/dev/null 2>&1; then
    log ""
    warn "cloudflared not installed. Публичные туннели работать не будут."
    warn "Установи вручную: pkg install cloudflared"
    warn "Или собери из исходников: https://github.com/cloudflare/cloudflared"
    log ""
fi

# ============================================================
# ГОТОВО
# ============================================================
log ""
log " VELTRIX Agent installed"
log ""
log "  binary:   $PREFIX/bin/veltrix-agent"
log "  config:   $CONFIG_FILE"
log "  data:     $DATA_ROOT"
log "  service:  $SERVICE_DIR"
log ""
log "  start:    sv start veltrix-agent"
log "  stop:     sv stop veltrix-agent"
log "  status:   sv status veltrix-agent"
log "  logs:     svlogtail veltrix-agent"
log ""
log "  Next steps:"
log "  1. Run 'tailscale up' and log in (if not done yet)"
log "  2. Copy your Tailscale IP (tailscale ip -4)"
log "  3. In VELTRIX panel, add a device with this IP"
log "  4. Start the agent: sv start veltrix-agent"
log ""
