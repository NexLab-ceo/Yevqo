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

## 4. Проверка именно из процесса тестового сайта (PDO)

VELTRIX Agent запускает PHP-сайт непосредственно в Termux, не в Docker-контейнере. DB environment не следует экспортировать в обычный Termux shell: Control Plane защищённо инжектирует её в процесс сайта при старте.

1. Создайте отдельный PHP-сайт, например \`veltrix-db-check\`.
2. В Database Details для \`veltrix_chain_test\` привяжите тестовый сайт через **Attach to site**. Связь доступна только для сайта и БД на одном Android device; хост автоматически устанавливается как \`127.0.0.1\`.
3. Если сайт уже запущен — остановите и запустите его заново. DB_* переменные подаются именно при старте процесса.
4. В редакторе файлов сайта временно создайте \`db-check.php\` с содержимым ниже. Этот файл не выводит пароль и не отображает детали исключения.

\`\`\`php
<?php
header('Content-Type: application/json; charset=utf-8');
try {
    if (!in_array('pgsql', PDO::getAvailableDrivers(), true)) {
        throw new RuntimeException('PDO_PGSQL driver unavailable');
    }
    $host = getenv('DB_HOST') ?: '127.0.0.1';
    $port = getenv('DB_PORT') ?: '5432';
    $database = getenv('DB_DATABASE') ?: '';
    $username = getenv('DB_USERNAME') ?: '';
    $password = getenv('DB_PASSWORD');
    $sslmode = getenv('DB_SSLMODE') ?: 'prefer';
    if ($database === '' || $username === '' || $password === false) {
        throw new RuntimeException('Managed DB environment is missing');
    }
    $dsn = sprintf('pgsql:host=%s;port=%s;dbname=%s;sslmode=%s',
        $host, $port, $database, $sslmode);
    $pdo = new PDO($dsn, $username, $password, [
        PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
        PDO::ATTR_EMULATE_PREPARES => false,
    ]);
    $row = $pdo->query('SELECT current_database() AS db, current_user AS role, 1 AS connection_ok')
        ->fetch(PDO::FETCH_ASSOC);
    echo json_encode(['ok' => true, 'result' => $row], JSON_UNESCAPED_SLASHES);
} catch (Throwable $e) {
    http_response_code(500);
    echo json_encode(['ok' => false, 'error' => 'PDO/database connection check failed']);
}
\`\`\`

5. Откройте \`http://127.0.0.1:<SITE_PORT>/db-check.php\` на самом телефоне (или фактический локальный адрес/домен тестового сайта). Ожидается JSON \`ok: true\`, имя \`veltrix_chain_test\`, роль приложения и \`connection_ok: 1\`.
6. После успешной проверки **удалите db-check.php**. Не оставляйте диагностический endpoint доступным публично.

Для Laravel дополнительно проверьте безопасный \`SELECT 1\` или \`php artisan db:show\` из запущенного сайта, если команда поддерживается. Не запускайте миграции до подтверждения, что target — тестовая БД.

## 5. Критерии успешного теста

- [ ] Server создан как Planned и запущен (`pg_ctl status` — running).
- [ ] `veltrix_chain_test` создана.
- [ ] LOGIN-роль имеет все опасные атрибуты false.
- [ ] `psql` под app credentials выполняет SELECT.
- [ ] Тестовый сайт подключается к именно этой БД и выполняет запрос.
- [ ] Production/существующие пользовательские базы не изменялись.

Запишите модель Android, версию Termux/PostgreSQL, Server ID и результаты пунктов, не публикуя пароль. CI и статический анализ не заменяют тест на телефоне.


## 6. MariaDB/MySQL-compatible server and PDO_MYSQL site connection

Use this flow only with a disposable MariaDB server and database. The Termux `mysql` engine option currently uses the MariaDB package and MySQL-compatible protocol; it does not install Oracle MySQL Server.

1. Create a MariaDB server in Database Servers. Use the default Yevqo data directory, loopback-only network access and TLS mode `require`.
2. Install it from the server page and wait for Agent status `ready`. Confirm the persisted directory ends in `server-<SERVER_ID>`; do not point it at an existing non-empty MariaDB data directory.
3. Create a test database and application user, then run **Test connection**. Verify the application user has privileges only on its own database.
4. Create a test PHP site, attach the MariaDB database and restart the site so Control Plane injects `DB_CONNECTION=mysql`.
5. Temporarily use this PHP check (never echo credentials):

```php
<?php
header('Content-Type: application/json; charset=utf-8');
try {
    if (!in_array('mysql', PDO::getAvailableDrivers(), true)) {
        throw new RuntimeException('PDO_MYSQL driver unavailable');
    }
    $dsn = sprintf('mysql:host=%s;port=%s;dbname=%s;charset=utf8mb4',
        getenv('DB_HOST') ?: '127.0.0.1',
        getenv('DB_PORT') ?: '3306',
        getenv('DB_DATABASE') ?: '');
    $pdo = new PDO($dsn, getenv('DB_USERNAME') ?: '', getenv('DB_PASSWORD') ?: '', [
        PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
        PDO::ATTR_EMULATE_PREPARES => false,
    ]);
    echo json_encode(['ok' => true, 'result' => $pdo->query('SELECT DATABASE() AS db, CURRENT_USER() AS user, 1 AS connection_ok')->fetch(PDO::FETCH_ASSOC)]);
} catch (Throwable $e) {
    http_response_code(500);
    echo json_encode(['ok' => false, 'error' => 'PDO_MYSQL/database connection check failed']);
}
```

6. Verify `ok: true`, the expected database/user and `connection_ok: 1`, then immediately delete the temporary PHP file.
7. Create a MariaDB SQL backup, restore it into a new database name, compare the rows and verify the original database remains unchanged. Stop/start the server and repeat the connection test.

The Control Plane API and CI tests do not prove a physical Termux install or restore; record the device model, Termux/MariaDB version and outcomes without credentials.
