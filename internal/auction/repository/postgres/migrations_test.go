//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// applyMigrations выполняет все .sql файлы из internal/db/migrations
// в порядке сортировки имён. Упрощённая замена goose для тестов.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir := findMigrationsDir()
	entries, _ := os.ReadDir(dir)

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return err
		}
		up := extractUpSection(string(content))
		if _, err := pool.Exec(ctx, up); err != nil {
			return fmt.Errorf("apply %s: %w", f, err)
		}
	}
	return nil
}

// extractUpSection берёт часть между -- +goose Up и -- +goose Down.
func extractUpSection(content string) string {
	const upMarker = "-- +goose Up"
	const downMarker = "-- +goose Down"

	start := len(upMarker)
	if idx := findMarker(content, upMarker); idx >= 0 {
		start = idx + len(upMarker)
	}

	end := len(content)
	if idx := findMarker(content, downMarker); idx >= 0 {
		end = idx
	}

	return content[start:end]
}

func findMarker(content, marker string) int {
	for i := 0; i+len(marker) <= len(content); i++ {
		if content[i:i+len(marker)] == marker {
			return i
		}
	}
	return -1
}

// findMigrationsDir ищет папку migrations, поднимаясь вверх по дереву.
func findMigrationsDir() string {
	dir, _ := os.Getwd()
	for {
		candidate := filepath.Join(dir, "internal", "db", "migrations")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
