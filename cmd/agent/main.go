// Package main — точка входа агента Yevqo (телефоны пользователей).
// Загружает конфиг, проверяет зависимости, подключается к серверу.
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"veltrix/internal/agent"
)

func resolveAgentConfigPath(prefix, override, legacyConfig string) string {
	if override != "" { return override }
	privateConfig := filepath.Join(prefix, "var", "lib", "veltrix-agent", "config", "agent.json")
	if _, err := os.Stat(privateConfig); err == nil { return privateConfig }
	// Preserve an existing legacy config, but never create a new token-bearing
	// config in shared storage on a fresh install.
	if _, err := os.Stat(legacyConfig); err == nil { return legacyConfig }
	return privateConfig
}

func main() {
	// Флаги.
	prefix := os.Getenv("PREFIX")
	if prefix == "" { prefix = "/data/data/com.termux/files/usr" }
	defaultConfigPath := resolveAgentConfigPath(prefix, os.Getenv("VELTRIX_AGENT_CONFIG"), agent.LegacyDefaultConfigPath)
	configPath := flag.String("config", defaultConfigPath, "path to agent.json (default: "+defaultConfigPath+")")
	flag.Parse()

	// 1. Загружаем конфиг.
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logFile, logErr := agent.OpenAgentLog(cfg.DataRoot)
	if logErr != nil {
		log.Printf("warning: could not open private Agent log: %v", logErr)
	} else {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
		defer logFile.Close()
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
	client.ForwardLogs(log.Writer())

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
