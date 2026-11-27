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
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	DeviceID  string `json:"device_id"`
	Hostname  string `json:"hostname"`
	DataRoot  string `json:"data_root"`
}

const DefaultConfigPath = "/data/data/com.termux/files/usr/var/lib/veltrix-agent/config/agent.json"
const LegacyDefaultConfigPath = "/sdcard/veltrix-agent/config/agent.json"
const DefaultDataRoot = "/sdcard/yevqo-agent"

// LoadConfig загружает конфиг из файла. Если файла нет — создаёт новый.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

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

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent.json: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent.json: %w", err)
	}

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

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// generateDeviceID генерирует UUID (v4).
func (c *Config) generateDeviceID() error {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("rand: %w", err)
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	c.DeviceID = fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
	return nil
}

// resolveHostname возвращает имя устройства.
func (c *Config) resolveHostname() string {
	if c.Hostname != "" {
		return c.Hostname
	}
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "android-device"
}
