# Yevqo — автозапуск в Termux

## Требования

1. Установите приложение **Termux:Boot** из того же источника, что и Termux (рекомендуется F-Droid). Не смешивайте сборки Termux из разных источников.
2. Откройте Termux:Boot вручную хотя бы один раз.
3. В Termux выполните `termux-setup-storage` и разрешите доступ к общему хранилищу. Control Plane использует `/sdcard/yevqo`, а рабочие файлы Agent — `/sdcard/yevqo-agent`; pairing token Agent хранится в приватном каталоге `$PREFIX/var/lib/veltrix-agent`.
4. В настройках Android разрешите Termux и Termux:Boot работать в фоне и отключите для них оптимизацию батареи. На некоторых прошивках автозапуск также нужно разрешить отдельно.

## Установка

Повторный запуск соответствующего установщика копирует `scripts/termux-boot.sh` в:

```sh
~/.termux/boot/00-yevqo
```

Скрипт:
- включает wake lock;
- запускает уже инициализированную MariaDB, только если она ещё не работает;
- ждёт готовности MariaDB до 45 секунд;
- поднимает только установленные сервисы `yevqo-control`, `yevqo-agent` или standalone `yevqo`;
- пишет диагностику в `~/.local/var/log/yevqo-boot.log`.

Скрипт не запускает `mysql_install_db`, не выполняет миграции вручную и не удаляет/пересоздаёт существующие файлы базы.

## Проверка

После перезагрузки и разблокировки устройства:

```sh
sv status yevqo-control
sv status yevqo-agent  # all-in-one
sv status yevqo        # standalone Agent
pgrep -a mariadbd
tail -n 100 ~/.local/var/log/yevqo-boot.log
```

На устройстве без Control Plane или Agent соответствующая служба просто пропускается. Автозапуск зависит от того, разрешает ли Android запуск Termux:Boot после перезагрузки; это нельзя подтвердить без теста на конкретном телефоне.
