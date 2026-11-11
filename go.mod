module veltrix

go 1.21

require (
    // WebSocket (для связи сервер-агент)
    github.com/gorilla/websocket v1.5.1

    // MySQL-драйвер (для панели)
    github.com/go-sql-driver/mysql v1.7.1

    // Protobuf (для протокола)
    google.golang.org/protobuf v1.32.0

    // bcrypt (для хэширования паролей)
    golang.org/x/crypto v0.18.0
)
