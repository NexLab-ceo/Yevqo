// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: загрузка/сохранение конфига агента (agent.json).
package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config — конфиг агента (хранится в agent.json).
type Config struct {
	// ServerURL — WebSocket-URL сервера (например, "ws://100.64.1.1:8080/ws").
	ServerURL string `json:"server_url"`

	// Token — код привязки (например, "ABCD-1234").
	// Это реферальный код пользователя, к которому привязан агент.
	Token string `json:"token"`

	// DeviceID — UUID устройства. Генерируется один раз при первом запуске.
	// Не меняется, даже если IP или hostname изменились.
	DeviceID string `json:"device_id"`

	// Hostname — имя устройства (например, "Redmi-Note-8").
	Hostname string `json:"hostname"`

	// DataRoot — корень данных на телефоне (например, "/sdcard/veltrix-agent").
	DataRoot string `json:"data_root"`
}

// DefaultConfigPath — где искать agent.json по умолчанию.
const DefaultConfigPath = "/sdcard/veltrix-agent/config/agent.json"

// DefaultDataRoot — корень данных по умолчанию.
const DefaultDataRoot = "/sdcard/veltrix-agent"

// LoadConfig загружает конфиг из файла. Если файла нет — создаёт новый.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	// Если файла нет — создаём с дефолтами.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := &Config{
			DataRoot: DefaultDataRoot,
		}
		if err := cfg.generateDeviceID(); err != nil {
			return nil, err
		}
		if err := SaveConfig(path, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	// Читаем.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent.json: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent.json: %w", err)
	}

	// Если device_id пустой — генерируем (на случай, если файл редактировали).
	if cfg.DeviceID == "" {
		if err := cfg.generateDeviceID(); err != nil {
			return nil, err
		}
		if err := SaveConfig(path, &cfg); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// SaveConfig сохраняет конфиг в файл (атомарно, через .tmp).
func SaveConfig(path string, cfg *Config) error {
	if path == "" {
		path = DefaultConfigPath
	}

	// Создаём папку, если её нет.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Сериализуем.
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Пишем во временный файл, потом переименовываем (атомарно).
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// generateDeviceID генерирует UUID (v4) для устройства.
// Использует crypto/rand — не Math.random.
func (c *Config) generateDeviceID() error {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("rand: %w", err)
	}

	// RFC 4122: version 4 (random), variant 1.
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1

	// Формат UUID: 8-4-4-4-12
	c.DeviceID = fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
	return nil
}

// Hostname возвращает имя устройства.
// Сначала пробует os.Hostname(), иначе — "android-device".
func (c *Config) resolveHostname() string {
	if c.Hostname != "" {
		return c.Hostname
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "android-device"
}
