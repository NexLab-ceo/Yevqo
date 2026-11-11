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
