//go:build integration

package postgres

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/7cout/minissp/internal/db"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Поднять контейнер PostgreSQL.
	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("minissp"),
		tcpostgres.WithUsername("minissp"),
		tcpostgres.WithPassword("minissp"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}
	defer func() {
		_ = testcontainers.TerminateContainer(pgContainer)
	}()

	// Получить адрес контейнера.
	host, err := pgContainer.Host(ctx)
	if err != nil {
		log.Fatalf("get container host: %v", err)
	}
	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("get container port: %v", err)
	}

	// Подключиться через pgxpool.
	pool, err := db.NewPostgresPool(ctx, db.PostgresConfig{
		Host:     host,
		Port:     port.Port(),
		User:     "minissp",
		Password: "minissp",
		Database: "minissp",
	})
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	testPool = pool
	defer pool.Close()

	// Применить миграции.
	if err := applyMigrations(ctx, pool); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	// 5. Запустить тесты.
	os.Exit(m.Run())
}
