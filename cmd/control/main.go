// Package main — точка входа сервера Yevqo (Redmi 9).
// Загружает конфиг, создаёт сервер (миграции применяются внутри), запускает HTTP.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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
	configPath := flag.String("config", "", "path to config.json (default: private Termux config; legacy shared config is migrated)")
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


func resolveControlConfigPath(explicitPath, dataRoot, prefix string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}
	privatePath := filepath.Join(prefix, "var", "lib", "yevqo-control", "config", "config.json")
	legacyPath := filepath.Join(dataRoot, "config", "config.json")

	privateInfo, privateErr := os.Lstat(privatePath)
	if privateErr == nil {
		if privateInfo.Mode()&os.ModeSymlink != 0 || !privateInfo.Mode().IsRegular() {
			return "", fmt.Errorf("private Control Plane config must be a regular file")
		}
		return privatePath, nil
	}
	if !os.IsNotExist(privateErr) {
		return "", fmt.Errorf("inspect private Control Plane config: %w", privateErr)
	}

	legacyInfo, legacyErr := os.Lstat(legacyPath)
	if legacyErr == nil {
		if legacyInfo.Mode()&os.ModeSymlink != 0 || !legacyInfo.Mode().IsRegular() {
			return "", fmt.Errorf("legacy Control Plane config must be a regular file")
		}
		contents, err := os.ReadFile(legacyPath)
		if err != nil {
			return "", fmt.Errorf("read legacy Control Plane config: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(privatePath), 0o700); err != nil {
			return "", fmt.Errorf("create private Control Plane config directory: %w", err)
		}
		tmpPath := privatePath + ".tmp"
		if err := os.WriteFile(tmpPath, contents, 0o600); err != nil {
			return "", fmt.Errorf("write private Control Plane config: %w", err)
		}
		if err := os.Rename(tmpPath, privatePath); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("activate private Control Plane config: %w", err)
		}
		copied, err := os.ReadFile(privatePath)
		if err != nil || !bytes.Equal(contents, copied) {
			return "", fmt.Errorf("verify private Control Plane config migration")
		}
		// Remove the shared-storage copy only after byte-for-byte verification.
		if err := os.Remove(legacyPath); err != nil {
			log.Printf("warning: private config migrated, but old shared-storage copy remains at %s: %v", legacyPath, err)
		} else {
			log.Printf("migrated Control Plane config from shared storage to %s", privatePath)
		}
		return privatePath, nil
	}
	if !os.IsNotExist(legacyErr) {
		return "", fmt.Errorf("inspect legacy Control Plane config: %w", legacyErr)
	}
	return privatePath, nil
}

func secureConfigFile(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect config file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("config must be a regular file, not a symlink or special file")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return false, fmt.Errorf("restrict config permissions: %w", err)
	}
	return true, nil
}

// loadConfig загружает конфиг из файла или создаёт дефолтный.
func loadConfig(path, dataRoot string) control.Config {
	cfg := defaultConfig

	// Если dataRoot передан флагом — используем его.
	if dataRoot != "" {
		cfg.DataRoot = dataRoot
	}

	// Keep credential-bearing Control Plane config in Termux private storage.
	if path == "" {
		var err error
		path, err = resolveControlConfigPath("", cfg.DataRoot, os.Getenv("PREFIX"))
		if err != nil {
			log.Fatalf("resolve config path: %v", err)
		}
	}

	exists, err := secureConfigFile(path)
	if err != nil {
		log.Fatalf("secure config: %v", err)
	}
	if !exists {
		log.Printf("config not found, creating default at %s", path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			log.Fatalf("create config directory: %v", err)
		}
		b, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			log.Fatalf("encode default config: %v", err)
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			log.Fatalf("write default config: %v", err)
		}
		return cfg
	}

	// Читаем файл после проверки типа и ограничения прав доступа.
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
