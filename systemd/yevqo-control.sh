#!/data/data/com.termux/files/usr/bin/sh
# VELTRIX Dock — скрипт для termux-services.
# Кладётся в $PREFIX/var/service/yevqo-control/run
# Управление: sv start|stop|restart|status yevqo-control

set -e

# Куда писать логи (termux-services сам перенаправит в svlogd).
exec 2>&1

# Путь к бинарнику. install-control.sh кладёт его сюда.
BIN="${PREFIX}/bin/yevqo-control"

if [ ! -x "$BIN" ]; then
    echo "error: $BIN not found or not executable"
    exit 1
fi

# Корень данных (можно переопределить переменной окружения).
DATA_ROOT="${YEVQO_DATA_ROOT:-${VELTRIX_DATA_ROOT:-/sdcard/yevqo}}"

echo "starting yevqo-control (data=$DATA_ROOT)"

# exec — заменяет shell на бинарник. sv будет слать сигналы прямо ему.
exec "$BIN" -data "$DATA_ROOT"
