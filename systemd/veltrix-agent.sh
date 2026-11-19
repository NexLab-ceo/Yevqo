#!/data/data/com.termux/files/usr/bin/sh
# VELTRIX Agent — скрипт для termux-services.
# Кладётся в $PREFIX/var/service/yevqo-agent/run
# Управление: sv start|stop|restart|status yevqo-agent

set -e

exec 2>&1

BIN="${PREFIX}/bin/yevqo-agent"

if [ ! -x "$BIN" ]; then
    echo "error: $BIN not found or not executable"
    exit 1
fi

CONFIG="${VELTRIX_AGENT_CONFIG:-${PREFIX}/var/lib/yevqo-agent/config/agent.json}"

echo "starting yevqo-agent (config=$CONFIG)"

exec "$BIN" -config "$CONFIG"
