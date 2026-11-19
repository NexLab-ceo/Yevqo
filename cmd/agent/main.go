// Package main — точка входа агента Yevqo (телефоны пользователей).
// Загружает конфиг, проверяет зависимости, подключается к серверу.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"veltrix/internal/agent"
)

func main() {
	// Флаги.
	prefix := os.Getenv("PREFIX")
	if prefix == "" { prefix = "/data/data/com.termux/files/usr" }
	defaultConfigPath := os.Getenv("VELTRIX_AGENT_CONFIG")
	if defaultConfigPath == "" {
		privateConfig := filepath.Join(prefix, "var", "lib", "veltrix-agent", "config", "agent.json")
		if _, err := os.Stat(privateConfig); err == nil {
			defaultConfigPath = privateConfig
		} else {
			defaultConfigPath = agent.DefaultConfigPath
		}
	}
	configPath := flag.String("config", defaultConfigPath, "path to agent.json (default: "+defaultConfigPath+")")
	flag.Parse()

	// 1. Загружаем конфиг.
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.Printf("Yevqo Agent v%s", agent.Version)
	log.Printf("device_id: %s", cfg.DeviceID)
	log.Printf("data_root: %s", cfg.DataRoot)

	// 2. Проверяем Tailscale.
	if ok, msg := agent.CheckTailscale(); !ok {
		log.Printf("warning: %s", msg)
		log.Printf("hint: pkg install tailscale && tailscale up")
	}

	// 3. Проверяем cloudflared (не обязательно, но полезно).
	if ok, msg := agent.CheckCloudflared(); !ok {
		log.Printf("warning: %s", msg)
	}

	// 4. Проверяем, что server_url и token заполнены.
	if cfg.ServerURL == "" || cfg.Token == "" {
		log.Fatalf("agent.json is incomplete: server_url and token must be set")
	}

	// 5. Создаём клиент и запускаем.
	client := agent.NewClient(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ловим Ctrl+C.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
		client.Stop()
	}()

	// Запускаем главный цикл (блокируется до Stop/cancel).
	if err := client.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("client: %v", err)
	}

	log.Println("stopped")
}
