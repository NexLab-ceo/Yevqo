#!/data/data/com.termux/files/usr/bin/bash
# Yevqo — установка сервера (Control Plane) на Redmi 9.
# Запуск: bash scripts/install-control.sh

set -e
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"

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

DATA_ROOT="${YEVQO_DATA_ROOT:-${VELTRIX_DATA_ROOT:-/sdcard/yevqo}}"
LEGACY_DATA_ROOT="/sdcard/veltrix-control"
if [ ! -d /sdcard ]; then
    err "Shared storage is unavailable. Run termux-setup-storage first."
    exit 1
fi
if [ -z "${YEVQO_DATA_ROOT:-}" ] && [ -z "${VELTRIX_DATA_ROOT:-}" ] && [ -e "$LEGACY_DATA_ROOT" ]; then
    if [ -L "$LEGACY_DATA_ROOT" ] || [ -L "$DATA_ROOT" ] || [ ! -d "$LEGACY_DATA_ROOT" ]; then
        err "Unsafe legacy/new data-root path; refusing automatic migration."
        exit 1
    fi
    if [ ! -e "$DATA_ROOT" ]; then
        mv "$LEGACY_DATA_ROOT" "$DATA_ROOT"
        log "Migrated Control Plane data to $DATA_ROOT"
    elif [ ! -d "$DATA_ROOT" ]; then
        err "New data root exists and is not a directory: $DATA_ROOT"
        exit 1
    elif [ -z "$(find "$DATA_ROOT" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
        rmdir "$DATA_ROOT"
        mv "$LEGACY_DATA_ROOT" "$DATA_ROOT"
        log "Migrated Control Plane data to $DATA_ROOT"
    elif [ -z "$(find "$LEGACY_DATA_ROOT" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
        warn "Legacy data directory is empty; leaving it in place."
    else
        err "Both legacy and new data roots contain data; refusing to merge or overwrite either."
        exit 1
    fi
fi
LEGACY_CONFIG_FILE="$DATA_ROOT/config/config.json"
CONFIG_DIR="$PREFIX/var/lib/yevqo-control/config"
CONFIG_FILE="$CONFIG_DIR/config.json"
mkdir -p "$CONFIG_DIR"
chmod 700 "$PREFIX/var/lib/yevqo-control" "$CONFIG_DIR"
if [ -L "$CONFIG_FILE" ] || { [ -e "$CONFIG_FILE" ] && [ ! -f "$CONFIG_FILE" ]; }; then
    err "Unsafe private Control Plane config path; refusing to follow it."
    exit 1
fi
if [ ! -e "$CONFIG_FILE" ] && [ -e "$LEGACY_CONFIG_FILE" ]; then
    if [ -L "$LEGACY_CONFIG_FILE" ] || [ ! -f "$LEGACY_CONFIG_FILE" ]; then
        err "Unsafe legacy Control Plane config; refusing migration."
        exit 1
    fi
    TMP_CONFIG="$CONFIG_FILE.tmp"
    cp "$LEGACY_CONFIG_FILE" "$TMP_CONFIG"
    chmod 600 "$TMP_CONFIG"
    if ! cmp -s "$LEGACY_CONFIG_FILE" "$TMP_CONFIG"; then
        rm -f "$TMP_CONFIG"
        err "Control Plane config migration verification failed."
        exit 1
    fi
    mv "$TMP_CONFIG" "$CONFIG_FILE"
    rm -f "$LEGACY_CONFIG_FILE"
    log "Migrated Control Plane credentials into Termux private storage"
fi
if [ -f "$CONFIG_FILE" ]; then chmod 600 "$CONFIG_FILE"; fi

# Generate a strong database password only when creating a fresh config.
# Existing configs and their database credentials are never overwritten.
if [ ! -f "$CONFIG_FILE" ]; then
    DB_PASSWORD="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \\n')"
else
    DB_PASSWORD="veltrix"
fi

log "Yevqo — Control Plane installer"
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
CREATE USER IF NOT EXISTS 'veltrix'@'localhost' IDENTIFIED BY '$DB_PASSWORD';
SQL
if [ ! -f "$CONFIG_FILE" ]; then
    mysql -u root -e "ALTER USER 'veltrix'@'localhost' IDENTIFIED BY '$DB_PASSWORD';" 2>/dev/null || warn "Could not rotate fresh-install database credential"
fi
mysql -u root <<SQL 2>/dev/null || warn "MySQL grants warning"
GRANT ALL PRIVILEGES ON veltrix.* TO 'veltrix'@'localhost';
FLUSH PRIVILEGES;
SQL

# Таблицы создаёт сам Control Plane при первом запуске.
# Установщик только подготавливает серверную БД и учётную запись.
log "  → core tables will be created by Control Plane migrations on first start"

# ============================================================
# 4. КОНФИГ
# ============================================================
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
  "mysql_dsn": "veltrix:$DB_PASSWORD@tcp(127.0.0.1:3306)/veltrix?parseTime=true&charset=utf8mb4",
  "admin_username": "$ADMIN_USER",
  "use_https": false
}
JSON
    log "  config written to $CONFIG_FILE"
    chmod 600 "$CONFIG_FILE" 2>/dev/null || true
else
    log "  config already exists: $CONFIG_FILE"
fi
chmod 600 "$CONFIG_FILE" 2>/dev/null || warn "Could not restrict config file permissions"

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
go build -ldflags="-s -w" -o bin/yevqo-control ./cmd/control
log "  → bin/yevqo-control ($(du -h bin/yevqo-control | cut -f1))"

# ============================================================
# 7. УСТАНОВКА БИНАРНИКА
# ============================================================
log "installing binary to \$PREFIX/bin..."
cp bin/yevqo-control "$PREFIX/bin/yevqo-control"
chmod +x "$PREFIX/bin/yevqo-control"

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
SERVICE_DIR="$PREFIX/var/service/yevqo-control"
mkdir -p "$SERVICE_DIR"
cp systemd/yevqo-control.sh "$SERVICE_DIR/run"
chmod +x "$SERVICE_DIR/run"

# Автозапуск.
sv-enable yevqo-control 2>/dev/null || true

# Termux:Boot hook. The Termux:Boot Android add-on must also be installed by the user.
BOOT_DIR="$HOME/.termux/boot"
mkdir -p "$BOOT_DIR"
if [ -f "scripts/termux-boot.sh" ]; then
    cp scripts/termux-boot.sh "$BOOT_DIR/00-yevqo"
    chmod 700 "$BOOT_DIR/00-yevqo"
    log "Termux:Boot hook installed at $BOOT_DIR/00-yevqo"
    warn "Install and open the Termux:Boot Android add-on once; then reboot to verify autostart."
else
    warn "scripts/termux-boot.sh not found; install the boot hook manually."
fi

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
log " Yevqo (Control Plane) installed"
log ""
log "  binary:   $PREFIX/bin/yevqo-control"
log "  config:   $CONFIG_FILE"
log "  data:     $DATA_ROOT"
log "  service:  $SERVICE_DIR"
log ""
log "  start:    sv start yevqo-control"
log "  stop:     sv stop yevqo-control"
log "  status:   sv status yevqo-control"
log "  logs:     svlogtail yevqo-control"
log ""
log "  web UI:   http://localhost:8080"
log ""
