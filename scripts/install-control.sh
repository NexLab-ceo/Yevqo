#!/data/data/com.termux/files/usr/bin/bash
# VELTRIX Dock — установка сервера (Control Plane) на Redmi 9.
# Запуск: bash scripts/install-control.sh

set -e

# Цвета.
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

if [ "$(uname -o)" != "Android" ]; then
    warn "not running on Android/Termux. Some steps may fail."
fi

DATA_ROOT="${VELTRIX_DATA_ROOT:-/sdcard/veltrix-control}"

log "VELTRIX Dock — Control Plane installer"
log "data root: $DATA_ROOT"

# ============================================================
# 1. ЗАВИСИМОСТИ
# ============================================================
log "installing dependencies..."

pkg update -y >/dev/null 2>&1 || true

# Go.
if ! command -v go >/dev/null 2>&1; then
    log "  → installing Go"
    pkg install -y golang
fi

# MySQL (MariaDB).
if ! command -v mariadb >/dev/null 2>&1 && ! command -v mysql >/dev/null 2>&1; then
    log "  → installing MariaDB"
    pkg install -y mariadb
fi

# Git.
if ! command -v git >/dev/null 2>&1; then
    log "  → installing Git"
    pkg install -y git
fi

# Tailscale.
if ! command -v tailscale >/dev/null 2>&1; then
    log "  → installing Tailscale"
    pkg install -y tailscale || warn "Tailscale install failed (optional)"
fi

# termux-services.
if ! command -v sv >/dev/null 2>&1; then
    log "  → installing termux-services"
    pkg install -y termux-services
fi

# protoc.
if ! command -v protoc >/dev/null 2>&1; then
    log "  → installing protobuf"
    pkg install -y protobuf
fi

# protoc-gen-go.
if ! command -v protoc-gen-go >/dev/null 2>&1; then
    log "  → installing protoc-gen-go"
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    export PATH="$PATH:$(go env GOPATH)/bin"
fi

# ============================================================
# 2. СТРУКТУРА ПАПОК
# ============================================================
log "creating directory structure..."
mkdir -p "$DATA_ROOT"/{config,logs,data,migrations,public}
mkdir -p "$DATA_ROOT/config"

# ============================================================
# 3. MYSQL / MARIADB
# ============================================================
log "setting up MariaDB..."

# Инициализация (если ещё не было).
if [ ! -d "$PREFIX/var/lib/mysql/mysql" ]; then
    log "  → initializing database"
    mysql_install_db >/dev/null 2>&1 || true
fi

# Запуск mysqld (если не запущен).
if ! pgrep -x mysqld >/dev/null 2>&1; then
    log "  → starting mysqld"
    mysqld_safe >/dev/null 2>&1 &
    sleep 3
fi

# Создаём базу и пользователя.
log "  → creating database 'veltrix'"
mysql -u root <<SQL 2>/dev/null || warn "MySQL setup warning (may be already configured)"
CREATE DATABASE IF NOT EXISTS veltrix
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'veltrix'@'localhost' IDENTIFIED BY 'veltrix';
GRANT ALL PRIVILEGES ON veltrix.* TO 'veltrix'@'localhost';
FLUSH PRIVILEGES;
SQL

# Применяем схему.
log "  → applying schema"
if [ -f "scripts/mysql-setup.sql" ]; then
    mysql -u root veltrix < scripts/mysql-setup.sql 2>/dev/null || warn "schema already applied"
else
    warn "scripts/mysql-setup.sql not found"
fi

# ============================================================
# 4. КОНФИГ
# ============================================================
CONFIG_FILE="$DATA_ROOT/config/config.json"
if [ ! -f "$CONFIG_FILE" ]; then
    log "creating config.json"
    
    # Спрашиваем admin username (или дефолт).
    read -p "Admin username [admin]: " ADMIN_USER
    ADMIN_USER="${ADMIN_USER:-admin}"
    
    cat > "$CONFIG_FILE" <<JSON
{
  "listen_addr": ":8080",
  "data_root": "$DATA_ROOT",
  "static_dir": "$DATA_ROOT/public",
  "mysql_dsn": "veltrix:veltrix@tcp(127.0.0.1:3306)/veltrix?parseTime=true&charset=utf8mb4",
  "admin_username": "$ADMIN_USER",
  "use_https": false
}
JSON
    log "  config written to $CONFIG_FILE"
else
    log "  config already exists: $CONFIG_FILE"
fi

# ============================================================
# 5. МИГРАЦИИ
# ============================================================
if [ -d "migrations" ]; then
    log "copying migrations..."
    mkdir -p "$DATA_ROOT/migrations"
    cp migrations/*.sql "$DATA_ROOT/migrations/" 2>/dev/null || true
fi

# ============================================================
# 6. СБОРКА
# ============================================================
log "building server binary..."
export PATH="$PATH:$(go env GOPATH)/bin"
bash scripts/gen-proto.sh 2>/dev/null || warn "proto generation skipped"
mkdir -p bin
go build -ldflags="-s -w" -o bin/veltrix-control ./cmd/control
log "  → bin/veltrix-control ($(du -h bin/veltrix-control | cut -f1))"

# ============================================================
# 7. УСТАНОВКА БИНАРНИКА
# ============================================================
log "installing binary to \$PREFIX/bin..."
cp bin/veltrix-control "$PREFIX/bin/veltrix-control"
chmod +x "$PREFIX/bin/veltrix-control"

# ============================================================
# 8. СТАТИКА (если есть)
# ============================================================
if [ -d "public" ] && [ -f "public/index.html" ]; then
    log "copying static files..."
    cp -r public/* "$DATA_ROOT/public/" 2>/dev/null || true
else
    warn "public/index.html not found. Build frontend first: bash scripts/build.sh"
fi

# ============================================================
# 9. TERMUX-SERVICES
# ============================================================
log "setting up service..."
SERVICE_DIR="$PREFIX/var/service/veltrix-control"
mkdir -p "$SERVICE_DIR"
cp systemd/veltrix-control.sh "$SERVICE_DIR/run"
chmod +x "$SERVICE_DIR/run"

# Автозапуск.
sv-enable veltrix-control 2>/dev/null || true

# ============================================================
# 10. TAILSCALE
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
# ГОТОВО
# ============================================================
log ""
log " VELTRIX Dock (Control Plane) installed"
log ""
log "  binary:   $PREFIX/bin/veltrix-control"
log "  config:   $CONFIG_FILE"
log "  data:     $DATA_ROOT"
log "  service:  $SERVICE_DIR"
log ""
log "  start:    sv start veltrix-control"
log "  stop:     sv stop veltrix-control"
log "  status:   sv status veltrix-control"
log "  logs:     svlogtail veltrix-control"
log ""
log "  web UI:   http://localhost:8080"
log ""
