#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
# The legacy data-root migration may move the directory the installer was launched from.
cd "$HOME" 2>/dev/null || cd "$PREFIX"
DATA_ROOT="${YEVQO_DATA_ROOT:-${VELTRIX_DATA_ROOT:-/sdcard/yevqo}}"
LEGACY_DATA_ROOT="/sdcard/veltrix-control"
log(){ printf '==> %s\n' "$*"; }
warn(){ printf 'WARNING: %s\n' "$*" >&2; }
if [ ! -d /sdcard ]; then
  echo "Shared storage is not available. Run termux-setup-storage and grant permission first." >&2
  exit 1
fi

# Move the legacy Control Plane directory only when the new destination is
# absent or empty. Never merge/overwrite two populated data roots.
if [ -z "${YEVQO_DATA_ROOT:-}" ] && [ -z "${VELTRIX_DATA_ROOT:-}" ] && [ -e "$LEGACY_DATA_ROOT" ]; then
  if [ -L "$LEGACY_DATA_ROOT" ] || [ -L "$DATA_ROOT" ] || [ ! -d "$LEGACY_DATA_ROOT" ]; then
    echo "Unsafe legacy/new data-root path; refusing automatic migration." >&2
    exit 1
  fi
  if [ ! -e "$DATA_ROOT" ]; then
    mv "$LEGACY_DATA_ROOT" "$DATA_ROOT"
    log "Migrated Control Plane data to $DATA_ROOT"
  elif [ ! -d "$DATA_ROOT" ]; then
    echo "New data root exists and is not a directory: $DATA_ROOT" >&2
    exit 1
  elif [ -z "$(find "$DATA_ROOT" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
    rmdir "$DATA_ROOT"
    mv "$LEGACY_DATA_ROOT" "$DATA_ROOT"
    log "Migrated Control Plane data to $DATA_ROOT"
  elif [ -z "$(find "$LEGACY_DATA_ROOT" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
    warn "Legacy data directory is empty; leaving it in place."
  else
    echo "Both $LEGACY_DATA_ROOT and $DATA_ROOT contain data. Refusing to merge or overwrite either directory; resolve them manually." >&2
    exit 1
  fi
fi

CONFIG_FILE="$DATA_ROOT/config/config.json"
SERVICE_DIR="$PREFIX/var/service/yevqo-control"
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

# Updates are downloaded from the public VELTRIX Dock releases; no device-side GitHub token is needed.

# Debian packages place the current Control Plane assets and immutable migrations under share/yevqo.
SHARE="$PREFIX/share/yevqo"
if [ ! -s "$SHARE/public/index.html" ]; then
  echo "Packaged Yevqo frontend is missing: $SHARE/public/index.html" >&2
  exit 1
fi
if [ ! -x "$SHARE/scripts/sync-migrations.sh" ]; then
  echo "Packaged Yevqo migration synchronizer is missing: $SHARE/scripts/sync-migrations.sh" >&2
  exit 1
fi
log "Staging new immutable database migrations"
"$SHARE/scripts/sync-migrations.sh" "$SHARE/migrations" "$DATA_ROOT/migrations"
log "Updating Control Plane frontend from $SHARE/public"
cp -a "$SHARE/public/." "$DATA_ROOT/public/"
# Keep GitHub App metadata separate from the PEM private key. The package setup
# never overwrites either file; the service loads them on every start.
GITHUB_CONFIG_DIR="$PREFIX/etc/yevqo"
GITHUB_ENV_FILE="$GITHUB_CONFIG_DIR/github-app.env"
GITHUB_KEY_FILE="$GITHUB_CONFIG_DIR/github-app.pem"
mkdir -p "$GITHUB_CONFIG_DIR"
chmod 700 "$GITHUB_CONFIG_DIR"
if [ ! -e "$GITHUB_ENV_FILE" ]; then
  cat > "$GITHUB_ENV_FILE" <<'ENV'
# Yevqo GitHub App server configuration (no private key in this file).
YEVQO_GITHUB_APP_ID=
YEVQO_GITHUB_APP_SLUG=
# Optional public HTTPS origin; enables automatic push webhook management.
YEVQO_PUBLIC_URL=
ENV
  chmod 600 "$GITHUB_ENV_FILE"
fi
if [ -f "$GITHUB_KEY_FILE" ]; then chmod 600 "$GITHUB_KEY_FILE"; fi

mkdir -p "$SERVICE_DIR"
cat > "$SERVICE_DIR/run" <<'RUN'
#!/data/data/com.termux/files/usr/bin/sh
set -eu
exec 2>&1
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
BIN="$PREFIX/bin/yevqo-control"
DATA_ROOT="${YEVQO_DATA_ROOT:-${VELTRIX_DATA_ROOT:-/sdcard/yevqo}}"
umask 077
LOG_DIR="$DATA_ROOT/logs"
if [ -L "$DATA_ROOT" ] || [ -L "$LOG_DIR" ]; then
  echo "Unsafe Control Plane log directory; refusing to follow symlinks"
  exit 1
fi
mkdir -p "$LOG_DIR"
[ -d "$LOG_DIR" ] || { echo "Control Plane log path is not a directory"; exit 1; }
chmod 700 "$LOG_DIR"
LOG_FILE="$LOG_DIR/control.log"
if [ -e "$LOG_FILE" ]; then
  [ -f "$LOG_FILE" ] && [ ! -L "$LOG_FILE" ] || { echo "Unsafe Control Plane log file"; exit 1; }
  if [ "$(wc -c < "$LOG_FILE")" -gt 5242880 ]; then
    ROTATED_LOG="$LOG_FILE.1"
    if [ -e "$ROTATED_LOG" ]; then
      [ -f "$ROTATED_LOG" ] && [ ! -L "$ROTATED_LOG" ] || { echo "Unsafe rotated Control Plane log file"; exit 1; }
      rm -f "$ROTATED_LOG"
    fi
    mv "$LOG_FILE" "$ROTATED_LOG"
    chmod 600 "$ROTATED_LOG"
  fi
fi
: >> "$LOG_FILE"
chmod 600 "$LOG_FILE"
exec >> "$LOG_FILE" 2>&1
GITHUB_CONFIG_DIR="$PREFIX/etc/yevqo"
GITHUB_ENV_FILE="$GITHUB_CONFIG_DIR/github-app.env"
GITHUB_KEY_FILE="$GITHUB_CONFIG_DIR/github-app.pem"
[ -x "$BIN" ] || { echo "Control Plane binary missing: $BIN"; exit 1; }

# The env file contains only non-secret identifiers and the optional public URL.
if [ -f "$GITHUB_ENV_FILE" ]; then
  set -a
  . "$GITHUB_ENV_FILE"
  set +a
fi
# Load the PEM only into the service process environment; never print it.
if [ -s "$GITHUB_KEY_FILE" ]; then
  YEVQO_GITHUB_APP_PRIVATE_KEY="$(cat "$GITHUB_KEY_FILE")"
  export YEVQO_GITHUB_APP_PRIVATE_KEY
else
  unset YEVQO_GITHUB_APP_PRIVATE_KEY 2>/dev/null || true
fi
exec "$BIN" -data "$DATA_ROOT"
RUN
chmod 700 "$SERVICE_DIR/run"
sv-enable yevqo-control >/dev/null 2>&1 || true

BOOT_DIR="$HOME/.termux/boot"
mkdir -p "$BOOT_DIR"
if [ -f "$SHARE/termux-boot.sh" ] && [ ! -e "$BOOT_DIR/00-yevqo" ]; then cp "$SHARE/termux-boot.sh" "$BOOT_DIR/00-yevqo"; chmod 700 "$BOOT_DIR/00-yevqo"; fi
sv up yevqo-control || warn "Could not start Control Plane service now"
log "Control Plane configured at http://127.0.0.1:8080"
log "Status: sv status yevqo-control"
