// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: получение IP из Tailscale.
package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Константы.
const (
	tailscaleCmd    = "tailscale"
	tailscaleTimout = 3 * time.Second
)

// GetTailscaleIP возвращает IPv4-адрес из Tailscale (например, "100.64.1.5").
// Если Tailscale не установлен или не запущен — возвращает пустую строку и ошибку.
//
// Как работает:
//   - Вызывает `tailscale ip -4`.
//   - Если команда не найдена → ошибка "tailscale not found".
//   - Если Tailscale не запущен → ошибка "tailscale is not running".
//   - Если IP найден → возвращает его.
func GetTailscaleIP() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tailscaleTimout)
	defer cancel()

	cmd := exec.CommandContext(ctx, tailscaleCmd, "ip", "-4")
	cmd.Env = append(cmd.Environ(), "TERM=dumb") // без ANSI-цветов

	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("tailscale not found (install it first: pkg install tailscale)")
		}
		if ctx.Err() == context.DeadlineExceeded {
			return "", errors.New("tailscale command timed out (is tailscaled running?)")
		}
		return "", fmt.Errorf("tailscale ip -4: %w", err)
	}

	// Разбираем вывод: несколько строк, каждая — IPv4.
	// Обычно одна строка. Берём первую валидную.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Проверяем, что это валидный IPv4.
		if ip := net.ParseIP(line); ip != nil && ip.To4() != nil {
			return line, nil
		}
	}

	return "", errors.New("no IPv4 found in tailscale output")
}

// CheckTailscale проверяет, установлен ли Tailscale и запущен ли демон.
// Возвращает:
//   - (true, "ok") — всё работает.
//   - (false, "tailscale not found") — команда не найдена.
//   - (false, "tailscale is not running") — команда есть, но демон не запущен.
func CheckTailscale() (bool, string) {
	// Проверяем наличие команды.
	if _, err := exec.LookPath(tailscaleCmd); err != nil {
		return false, "tailscale not found"
	}

	// Проверяем, что демон запущен (tailscale status).
	ctx, cancel := context.WithTimeout(context.Background(), tailscaleTimout)
	defer cancel()

	cmd := exec.CommandContext(ctx, tailscaleCmd, "status")
	cmd.Env = append(cmd.Environ(), "TERM=dumb")
	if err := cmd.Run(); err != nil {
		return false, "tailscale is not running (try: tailscale up)"
	}

	return true, "ok"
}

// EnsureTailscaleUp пытается запустить Tailscale, если он установлен,
// но не подключён. Возвращает true, если Tailscale готов к работе.
//
// Это НЕ автоматический `tailscale up` (он требует интерактивного
// логина в браузере). Просто проверяет статус.
func EnsureTailscaleUp() (bool, string) {
	ok, msg := CheckTailscale()
	if ok {
		return true, "ok"
	}

	// Если Tailscale не установлен — не пытаемся.
	if msg == "tailscale not found" {
		return false, msg
	}

	// Если не запущен — подсказываем, что делать.
	return false, msg + " (run: tailscale up)"
}
