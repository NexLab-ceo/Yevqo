veltrix-dock/
├── go.mod
├── go.sum
├── README.md
├── README.ru.md
├── LICENSE
│
├── cmd/
│   ├── control/             # Сервер (Redmi 9)
│   │   └── main.go
│   └── agent/               # Агент (телефоны)
│       └── main.go
│
├── internal/
│   ├── control/
│   │   ├── server.go        # HTTP-сервер, роутинг
│   │   ├── websocket.go     # WebSocket-хаб (подключения агентов)
│   │   ├── auth.go          # Логин/сессии/токены пользователей
│   │   ├── db.go            # MySQL-обёртка (CRUD)
│   │   ├── handlers.go      # API для пользователей (/api/*)
│   │   ├── admin.go         # API для админа (/admin/*)
│   │   └── migrations.go    # Миграции схемы БД
│   │
│   ├── agent/
│   │   ├── client.go        # WebSocket-клиент (подключение к серверу)
│   │   ├── commands.go      # Обработка входящих команд
│   │   ├── process.go       # Запуск/остановка php -S, Go-сайтов
│   │   ├── tunnel.go        # Запуск/остановка cloudflared
│   │   ├── tailscale.go     # Получение IP из tailscale
│   │   └── config.go        # Загрузка/сохранение agent.json
│   │
│   └── shared/
│       ├── protocol.go      # Сгенерированный из .proto код
│       └── models.go        # Общие структуры (User, Device, Site)
│
├── proto/
│   └── veltrix.proto        # Protobuf-схема протокола
│
├── frontend/                # SvelteKit (панель на Redmi 9)
│   ├── package.json
│   ├── vite.config.js
│   ├── svelte.config.js
│   ├── jsconfig.json
│   └── src/
│       ├── app.html
│       ├── app.css
│       ├── routes/
│       │   ├── +layout.svelte         # Sidebar + Topbar
│       │   ├── +page.svelte           # Главная (список устройств)
│       │   ├── login/
│       │   │   └── +page.svelte
│       │   ├── register/
│       │   │   └── +page.svelte
│       │   ├── devices/
│       │   │   └── [id]/
│       │   │       └── +page.svelte   # Управление устройством
│       │   ├── admin/
│       │   │   ├── +page.svelte       # Список пользователей
│       │   │   └── [id]/
│       │   │       └── +page.svelte
│       │   └── profile/
│       │       └── +page.svelte       # Рефералка, настройки
│       ├── lib/
│       │   ├── api.js                 # fetch к /api/*
│       │   ├── stores.js              # user, devices, notifications
│       │   ├── i18n.js                # Загрузчик переводов
│       │   └── components/            # Переиспользуемые компоненты
│       │       ├── Button.svelte
│       │       ├── Card.svelte
│       │       ├── Modal.svelte
│       │       └── StatusDot.svelte
│       └── locales/
│           ├── en.json
│           └── ru.json
│
├── scripts/
│   ├── install-control.sh   # Установка на Redmi 9 (Go, MySQL, Tailscale)
│   ├── install-agent.sh     # Установка на телефон (Go, Tailscale)
│   ├── build.sh             # Сборка Go + Svelte + .deb
│   ├── mysql-setup.sql      # Схема БД (users, devices, sites)
│   └── gen-proto.sh         # Генерация Go-кода из .proto
│
└── systemd/                 # (опционально) для termux-services
    ├── veltrix-control.sh
    └── veltrix-agent.sh


## VELTRIX Aurora migration track

The `feature/database-server-wizard` branch contains the in-progress VELTRIX Dock UI and Control Plane migration. It does not change `main` or production data.

### Workspace and operations foundation
- Projects can group devices and websites. Deleting a project deletes only its grouping records; devices, sites, files and databases remain untouched.
- Database Server configurations are stored as `planned` metadata. Saving a configuration does not install packages or modify an Android device.
- Database Server records are isolated per account and device. A server cannot be removed while database metadata references it.
- Account settings persist Simple / Advanced mode, locale and theme.
- Activity logs, notifications, account summary reports and backup metadata have read APIs.
- New additive schema is in `migrations/001_modules.sql`; the Termux installer copies SQL migrations into the control-plane data directory and applies them at startup.

### Important implementation boundary
Database engine installation, database creation, credentials, backup execution and restore are not yet wired to the Android Agent. The current UI must not imply those device-side operations have completed. These capabilities require typed Agent protocol commands, device-side validation, idempotent execution and rollback-safe behavior.


### Website lifecycle and operations
- Site creation is an explicit typed Agent command. It creates a new site directory and a starter file without overwriting an existing directory.
- Site processes bind to localhost by default; occupied ports are rejected before launch.
- Site IDs and paths are validated on the Agent. Site log files are private to the Termux account.
- Removing a website from the panel removes its metadata only; its device files are deliberately preserved.
- Domains can be assigned as metadata to a site. DNS and reverse-proxy / tunnel routing are not reported as active until verified separately.
- Deployment history records site create/start/stop and Git Pull actions without storing command payloads or credentials.
- Monitoring currently reports device heartbeat, agent version and site/server counts. CPU, RAM and temperature require a future telemetry event from the Agent.
