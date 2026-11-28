#!/data/data/com.termux/files/usr/bin/bash
set -u

PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
INTERVAL="${YEVQO_AUTO_UPDATE_INTERVAL:-21600}"

case "$INTERVAL" in
  ''|*[!0-9]*) echo "Invalid YEVQO_AUTO_UPDATE_INTERVAL: $INTERVAL" >&2; exit 2 ;;
esac
if [ "$INTERVAL" -lt 900 ]; then
  echo "YEVQO_AUTO_UPDATE_INTERVAL must be at least 900 seconds; using 21600."
  INTERVAL=21600
fi

# This service belongs to the standalone Agent package only.
# Never fall back to an All-in-one or Control Plane updater.
[ -x "$PREFIX/bin/yevqo-update" ] || {
  echo "Yevqo Agent updater is not installed; refusing to run another edition's updater." >&2
  exit 1
}

sleep 30
while true; do
  "$PREFIX/bin/yevqo-update" || echo "Yevqo Agent automatic update check failed; it will be retried."
  sleep "$INTERVAL"
done
