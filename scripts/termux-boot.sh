#!/data/data/com.termux/files/usr/bin/bash
# Yevqo startup hook for the Termux:Boot add-on.
# Installed to ~/.termux/boot/00-yevqo by the installer.
set -u
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
LOG_DIR="${HOME}/.local/var/log"
mkdir -p "$LOG_DIR"
exec >>"$LOG_DIR/yevqo-boot.log" 2>&1
echo "[$(date -Iseconds)] boot hook started"

# Keep Android from suspending the phone while the server is active.
if command -v termux-wake-lock >/dev/null 2>&1; then
  termux-wake-lock || true
fi

# Start MariaDB only when it is not already running; never initialize or alter its data.
if command -v mysqld_safe >/dev/null 2>&1 && ! pgrep -x mariadbd >/dev/null 2>&1 && ! pgrep -x mysqld >/dev/null 2>&1; then
  echo "Starting existing MariaDB data directory"
  nohup mysqld_safe >/dev/null 2>&1 &
fi

# Wait up to 45 seconds for the existing local database.
if command -v mysqld_safe >/dev/null 2>&1; then
  ready=0
  for i in $(seq 1 45); do
    if mysqladmin ping --silent >/dev/null 2>&1; then ready=1; break; fi
    sleep 1
  done
  if [ "$ready" -ne 1 ]; then
    echo "MariaDB did not become ready; Control Plane will not be started"
    exit 1
  fi
fi

# Bring up only services installed on this device.
for service in yevqo-control yevqo-agent; do
  if [ -d "${PREFIX}/var/service/$service" ] || [ -L "${PREFIX}/var/service/$service" ]; then
    echo "Starting $service"
    sv up "$service" || echo "Could not start $service"
  fi
done
echo "[$(date -Iseconds)] boot hook finished"
