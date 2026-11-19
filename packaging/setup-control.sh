#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
DATA_ROOT="${VELTRIX_DATA_ROOT:-/sdcard/veltrix-control}"
if [ ! -d /sdcard ]; then
  echo "Shared storage is not available. Run termux-setup-storage and grant permission first." >&2
  exit 1
fi
CONFIG_FILE="$DATA_ROOT/config/config.json"
SERVICE_DIR="$PREFIX/var/service/veltrix-control"
log(){ printf '==> %s\n' "$*"; }
warn(){ printf 'WARNING: %s\n' "$*" >&2; }
command -v mysql >/dev/null 2>&1 || { echo "MariaDB client missing; install package mariadb first" >&2; exit 1; }
mkdir -p "$DATA_ROOT"/{config,logs,data,migrations,public,backups}
if [ ! -d "$PREFIX/var/lib/mysql/mysql" ]; then
  log "Initializing a new MariaDB data directory"
  mysql_install_db
fi
if ! pgrep -x mariadbd >/dev/null 2>&1 && ! pgrep -x mysqld >/dev/null 2>&1; then
  mysqld_safe >/dev/null 2>&1 &
fi
ready=0
for i in $(seq 1 45); do
  if mysqladmin ping --silent >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[ "$ready" = 1 ] || { echo "MariaDB did not become ready" >&2; exit 1; }

if [ ! -s "$CONFIG_FILE" ]; then
  read -r -p 'Admin username [admin]: ' ADMIN_USER
  ADMIN_USER="${ADMIN_USER:-admin}"
  if [[ ! "$ADMIN_USER" =~ ^[A-Za-z0-9_.-]{1,64}$ ]]; then
    echo "Admin username must be 1-64 characters: letters, digits, dot, underscore or hyphen." >&2
    exit 1
  fi
  DB_PASSWORD="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
  mysql -u root <<SQL
CREATE DATABASE IF NOT EXISTS veltrix CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'veltrix'@'localhost' IDENTIFIED BY '$DB_PASSWORD';
ALTER USER 'veltrix'@'localhost' IDENTIFIED BY '$DB_PASSWORD';
GRANT ALL PRIVILEGES ON veltrix.* TO 'veltrix'@'localhost';
FLUSH PRIVILEGES;
SQL
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
  chmod 600 "$CONFIG_FILE"
else
  log "Existing Control Plane config preserved; database credentials were not changed."
  mysql -u root -e "CREATE DATABASE IF NOT EXISTS veltrix CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" || true
fi

# Optional private GitHub token for veltrix-all self-updates.
TOKEN_FILE="$DATA_ROOT/config/github-token"
if [ ! -s "$TOKEN_FILE" ]; then
  printf '\nPrivate updater setup (optional)\n'
  read -r -p 'GitHub token with Contents: read on NexLab-ceo/veltrix (leave blank to skip): ' -s GITHUB_TOKEN
  printf '\n'
  if [ -n "$GITHUB_TOKEN" ]; then
    printf '%s' "$GITHUB_TOKEN" > "$TOKEN_FILE"
    chmod 600 "$TOKEN_FILE"
    log "Private updater token saved to $TOKEN_FILE"
  else
    log "Private updater token skipped; configure it later in $TOKEN_FILE"
  fi
fi

SHARE="$PREFIX/share/veltrix"
[ -d "$SHARE/migrations" ] && cp -an "$SHARE/migrations/." "$DATA_ROOT/migrations/" || true
[ -d "$SHARE/public" ] && cp -a "$SHARE/public/." "$DATA_ROOT/public/" || true
mkdir -p "$SERVICE_DIR"
cat > "$SERVICE_DIR/run" <<'RUN'
#!/data/data/com.termux/files/usr/bin/sh
set -e
exec 2>&1
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
BIN="$PREFIX/bin/veltrix-control"
DATA_ROOT="${VELTRIX_DATA_ROOT:-/sdcard/veltrix-control}"
[ -x "$BIN" ] || { echo "Control Plane binary missing: $BIN"; exit 1; }
exec "$BIN" -data "$DATA_ROOT"
RUN
chmod 700 "$SERVICE_DIR/run"
sv-enable veltrix-control >/dev/null 2>&1 || true

BOOT_DIR="$HOME/.termux/boot"
mkdir -p "$BOOT_DIR"
if [ -f "$SHARE/termux-boot.sh" ]; then cp "$SHARE/termux-boot.sh" "$BOOT_DIR/00-veltrix"; chmod 700 "$BOOT_DIR/00-veltrix"; fi
sv up veltrix-control || warn "Could not start Control Plane service now"
log "Control Plane configured at http://127.0.0.1:8080"
log "Status: sv status veltrix-control"
