// Package main — точка входа сервера VELTRIX Dock (Redmi 9).
// Загружает конфиг, подключается к MySQL, запускает миграции и HTTP-сервер.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"veltrix/internal/control"
)

// defaultConfig — значения по умолчанию.
var defaultConfig = control.Config{
	ListenAddr:    ":8080",
	DataRoot:      "/sdcard/veltrix-control",
	StaticDir:     "public",
	MySQLDSN:      "veltrix:veltrix@tcp(127.0.0.1:3306)/veltrix?parseTime=true&charset=utf8mb4",
	AdminUsername: "admin",
	UseHTTPS:      false,
}

func main() {
	// Флаги командной строки.
	configPath := flag.String("config", "", "path to config.json (default: <data_root>/config/config.json)")
	dataRoot := flag.String("data", "", "data root directory (overrides config)")
	flag.Parse()

	// 1. Загружаем конфиг.
	cfg := loadConfig(*configPath, *dataRoot)

	// 2. Создаём сервер.
	srv, err := control.NewServer(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	defer srv.Close()

	// 3. Применяем миграции.
	migrationsDir := filepath.Join(cfg.DataRoot, "migrations")
	if _, err := os.Stat(migrationsDir); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := runMigrations(srv, ctx, migrationsDir); err != nil {
			cancel()
			log.Fatalf("migrations: %v", err)
		}
		cancel()
	}

	// 4. Запускаем HTTP-сервер в горутине.
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	// 5. Ждём сигнала остановки (Ctrl+C, SIGTERM).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
}

// loadConfig загружает конфиг из файла или создаёт дефолтный.
func loadConfig(path, dataRoot string) control.Config {
	cfg := defaultConfig

	// Если dataRoot передан флагом — используем его.
	if dataRoot != "" {
		cfg.DataRoot = dataRoot
	}

	// Если configPath не задан — ищем в <data_root>/config/config.json.
	if path == "" {
		path = filepath.Join(cfg.DataRoot, "config", "config.json")
	}

	// Если файла нет — создаём дефолтный.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("config not found, creating default at %s", path)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		b, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			log.Printf("warning: cannot write default config: %v", err)
		}
		return cfg
	}

	// Читаем файл.
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	// Парсим.
	if err := json.Unmarshal(b, &cfg); err != nil {
		log.Fatalf("parse config: %v", err)
	}

	log.Printf("config loaded from %s", path)
	return cfg
}

// runMigrations — обёртка, чтобы не тащить *control.Server в пакет control.
func runMigrations(srv *control.Server, ctx context.Context, dir string) error {
	return srv.RunMigrations(ctx, dir)
}
