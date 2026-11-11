// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: простые миграции схемы БД.
package control

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// migrationsTable — таблица, где хранится история применённых миграций.
const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    name        VARCHAR(255) PRIMARY KEY,
    hash        CHAR(64)     NOT NULL,
    applied_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

// RunMigrations применяет все SQL-файлы из папки migrationsDir,
// которые ещё не были применены.
//
// Имена файлов должны быть вида: 001_initial.sql, 002_add_column.sql, ...
// Файлы применяются в лексикографическом порядке.
func (db *DB) RunMigrations(ctx context.Context, migrationsDir string) error {
	// 1. Создаём таблицу истории (если её нет).
	if _, err := db.conn.ExecContext(ctx, migrationsTable); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// 2. Загружаем уже применённые миграции.
	applied, err := db.appliedMigrations(ctx)
	if err != nil {
		return err
	}

	// 3. Читаем список файлов.
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// 4. Сортируем по имени (001, 002, ...).
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	// 5. Применяем те, которых нет в applied.
	appliedCount := 0
	for _, name := range files {
		if _, ok := applied[name]; ok {
			continue
		}

		path := filepath.Join(migrationsDir, name)
		if err := db.applyMigration(ctx, name, path); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		appliedCount++
		log.Printf("migration applied: %s", name)
	}

	if appliedCount == 0 {
		log.Printf("migrations: nothing to apply (up to date)")
	} else {
		log.Printf("migrations: %d applied", appliedCount)
	}
	return nil
}

// appliedMigrations возвращает map[name]hash уже применённых миграций.
func (db *DB) appliedMigrations(ctx context.Context) (map[string]string, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT name, hash FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var name, hash string
		if err := rows.Scan(&name, &hash); err != nil {
			return nil, err
		}
		out[name] = hash
	}
	return out, rows.Err()
}

// applyMigration применяет один файл в транзакции.
func (db *DB) applyMigration(ctx context.Context, name, path string) error {
	// Читаем файл.
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Считаем хеш содержимого (для проверки, что файл не менялся).
	h := sha256.Sum256(content)
	hash := hex.EncodeToString(h[:])

	// Начинаем транзакцию.
	tx, err := db.conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // безопасно даже после Commit (no-op)
	}()

	// Разбиваем на отдельные SQL-запросы по ";" и выполняем по одному.
	// MySQL-драйвер Go не поддерживает multi-statement из коробки,
	// поэтому разбиваем вручную.
	statements := splitSQL(string(content))
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("statement %d: %w", i+1, err)
		}
	}

	// Записываем в историю.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (name, hash, applied_at) VALUES (?, ?, ?)`,
		name, hash, time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	// Коммитим.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// splitSQL разбивает SQL-скрипт на отдельные запросы по ";".
// Учитывает строки в кавычках (не режет по ";" внутри '...' или "...").
func splitSQL(script string) []string {
	var out []string
	var buf strings.Builder
	inSingle := false
	inDouble := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(script); i++ {
		ch := script[i]

		// Начало строчного комментария.
		if !inSingle && !inDouble && !inBlockComment && ch == '-' && i+1 < len(script) && script[i+1] == '-' {
			inLineComment = true
			i++
			continue
		}

		// Начало блочного комментария.
		if !inSingle && !inDouble && !inLineComment && ch == '/' && i+1 < len(script) && script[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}

		// Конец строчного комментария.
		if inLineComment && ch == '\n' {
			inLineComment = false
			continue
		}

		// Конец блочного комментария.
		if inBlockComment && ch == '*' && i+1 < len(script) && script[i+1] == '/' {
			inBlockComment = false
			i++
			continue
		}

		// Пропускаем комментарии.
		if inLineComment || inBlockComment {
			continue
		}

		// Кавычки.
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			buf.WriteByte(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			buf.WriteByte(ch)
			continue
		}

		// Конец запроса.
		if ch == ';' && !inSingle && !inDouble {
			out = append(out, buf.String())
			buf.Reset()
			continue
		}

		buf.WriteByte(ch)
	}

	if s := strings.TrimSpace(buf.String()); s != "" {
		out = append(out, s)
	}
	return out
}
