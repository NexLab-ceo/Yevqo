// Package main — точка входа сервера Yevqo (Redmi 9).
// Загружает конфиг, создаёт сервер (миграции применяются внутри), запускает HTTP.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"veltrix/internal/control"
)

// defaultConfig — значения по умолчанию.
var defaultConfig = control.Config{
	ListenAddr:    ":8080",
	DataRoot:      "/sdcard/yevqo",
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

	// 2. Создаём сервер. Внутри: подключение к БД, миграции, хаб.
	//    Если что-то упадёт — log.Fatal, сервер не стартует.
	srv, err := control.NewServer(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	defer srv.Close()

	// 3. Запускаем HTTP-сервер в горутине.
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	// 4. Ждём сигнала остановки (Ctrl+C, SIGTERM).
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

	// An explicit -data flag is authoritative over a legacy config.json.
	// Keep the existing absolute static path aligned with the migrated root.
	if dataRoot != "" {
		previousRoot := cfg.DataRoot
		cfg.DataRoot = dataRoot
		if cfg.StaticDir == filepath.Join(previousRoot, "public") ||
			cfg.StaticDir == filepath.Join("/sdcard/veltrix-control", "public") {
			cfg.StaticDir = filepath.Join(dataRoot, "public")
		}
	}

	log.Printf("config loaded from %s", path)
	return cfg
}
