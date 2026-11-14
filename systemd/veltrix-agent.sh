#!/data/data/com.termux/files/usr/bin/sh
# VELTRIX Agent — скрипт для termux-services.
# Кладётся в $PREFIX/var/service/veltrix-agent/run
# Управление: sv start|stop|restart|status veltrix-agent

set -e

exec 2>&1

BIN="${PREFIX}/bin/veltrix-agent"

if [ ! -x "$BIN" ]; then
    echo "error: $BIN not found or not executable"
    exit 1
fi

CONFIG="${VELTRIX_AGENT_CONFIG:-${PREFIX}/var/lib/veltrix-agent/config/agent.json}"

echo "starting veltrix-agent (config=$CONFIG)"

exec "$BIN" -config "$CONFIG"
