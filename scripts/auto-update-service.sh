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

# Let dpkg finish configuring the package before the first release check.
sleep 30
while true; do
  if [ -x "$PREFIX/bin/yevqo-update-all" ]; then
    "$PREFIX/bin/yevqo-update-all" || echo "Yevqo All automatic update check failed; it will be retried."
  elif [ -x "$PREFIX/bin/yevqo-update" ]; then
    "$PREFIX/bin/yevqo-update" || echo "Yevqo Agent automatic update check failed; it will be retried."
  else
    echo "No Yevqo updater command is installed."
  fi
  sleep "$INTERVAL"
done
