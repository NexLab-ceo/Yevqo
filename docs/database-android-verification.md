# Проверка Database Management на Android / Termux

Сценарий проверяет цепочку на реальном телефоне. Используйте отдельный тестовый PostgreSQL server и сайт. Не выполняйте на production и не используйте пользовательскую БД.

## 1. Подготовка

1. Запустите VELTRIX Agent в Termux; устройство должно быть Online.
2. В Database Servers создайте отдельный PostgreSQL server (Planned), затем запустите через Dock.
3. В Termux проверьте бинарники/статус:

```sh
command -v pg_ctl; command -v psql; command -v pg_dump
pg_ctl status -D "$PREFIX/var/lib/veltrix-agent/postgresql/<SERVER_ID>/data"
```

Путь к data directory берите из конфигурации созданного сервера. Не запускайте параллельно `pg_ctl`, если server управляется Dock.

## 2. Тестовая БД и роль

В Database Server → Databases создайте уникальную тестовую БД `veltrix_chain_test`. В Users & Roles создайте LOGIN-роль `veltrix_chain_app` с временным паролем.

```sh
psql -h "$PREFIX/tmp" -p <PORT> -d postgres -c \
  "SELECT rolname, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls FROM pg_roles WHERE rolname='veltrix_chain_app';"
```

Ожидаются все boolean-флаги `false`. Не храните пароль в shell history/Git.

## 3. Проверка psql с credentials приложения

```sh
PGPASSWORD='<TEMP_PASSWORD>' psql -h "$PREFIX/tmp" -p <PORT> \
  -U veltrix_chain_app -d veltrix_chain_test -v ON_ERROR_STOP=1 \
  -c 'SELECT current_database(), current_user, 1 AS connection_ok;'
```

При отказе назначьте минимум CONNECT на database, USAGE на schema и необходимые table privileges. Не выдавайте SUPERUSER.

## 4. Тестовый сайт и PDO

Создайте отдельный тестовый сайт на устройстве и передайте credentials через Environment/Secrets (не в репозитории):

```env
DB_CONNECTION=pgsql
DB_HOST=<host-reachable-from-site-container>
DB_PORT=<PORT>
DB_DATABASE=veltrix_chain_test
DB_USERNAME=veltrix_chain_app
DB_PASSWORD=<TEMP_PASSWORD>
```

`DB_HOST`/`DB_PORT` должны быть доступны из network namespace сайта. `127.0.0.1` внутри контейнера обычно указывает на контейнер, а не Android host. Используйте фактический адрес/forwarding из Dock, не угадывайте.

В контейнере сайта:

```sh
php -r '$p=new PDO("pgsql:host=".getenv("DB_HOST").";port=".getenv("DB_PORT").";dbname=".getenv("DB_DATABASE"),getenv("DB_USERNAME"),getenv("DB_PASSWORD")); echo $p->query("SELECT current_database(), current_user")->fetchColumn(), PHP_EOL;'
```

Для Laravel также проверьте безопасный `SELECT 1` или `php artisan db:show`, если команда поддерживается. Не запускайте миграции до подтверждения тестового target.

## 5. Критерии успешного теста

- [ ] Server создан как Planned и запущен (`pg_ctl status` — running).
- [ ] `veltrix_chain_test` создана.
- [ ] LOGIN-роль имеет все опасные атрибуты false.
- [ ] `psql` под app credentials выполняет SELECT.
- [ ] Тестовый сайт подключается к именно этой БД и выполняет запрос.
- [ ] Production/существующие пользовательские базы не изменялись.

Запишите модель Android, версию Termux/PostgreSQL, Server ID и результаты пунктов, не публикуя пароль. CI и статический анализ не заменяют тест на телефоне.
