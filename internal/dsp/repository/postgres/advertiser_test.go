//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// newTestPool подключается к тестовой БД и очищает таблицы
// перед тестом.
//
// Использует TEST_DATABASE_URL, а не DATABASE_URL — чтобы тесты
// никогда не ходили в dev-базу. Если переменная не задана — тест
// скипается.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(context.Background(), `TRUNCATE dsp.advertisers CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func testAdvertiser(balance int64) *domain.Advertiser {
	return &domain.Advertiser{
		ID:      uuid.NewString(),
		Name:    "Nike",
		Balance: balance,
	}
}

func TestPostgresAdvertiserRepo_AddAndGet(t *testing.T) {
	pool := newTestPool(t)
	repo := NewAdvertiserRepo(pool)
	ctx := context.Background()

	a := testAdvertiser(1_000_000_000)
	if err := repo.Add(ctx, a); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := repo.Get(ctx, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != a.Name {
		t.Errorf("name = %q, want %q", got.Name, a.Name)
	}
	if got.Balance != a.Balance {
		t.Errorf("balance = %d, want %d", got.Balance, a.Balance)
	}
}

func TestPostgresAdvertiserRepo_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := NewAdvertiserRepo(pool)

	_, err := repo.Get(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrAdvertiserNotFound) {
		t.Errorf("want ErrAdvertiserNotFound, got %v", err)
	}
}

func TestPostgresAdvertiserRepo_Add_DuplicateID(t *testing.T) {
	pool := newTestPool(t)
	repo := NewAdvertiserRepo(pool)
	ctx := context.Background()

	a := testAdvertiser(1_000_000_000)
	_ = repo.Add(ctx, a)

	err := repo.Add(ctx, a)
	if err == nil {
		t.Error("want error for duplicate id, got nil")
	}
}
