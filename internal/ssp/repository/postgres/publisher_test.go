//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// newTestPool подключается к Postgres из docker-compose и очищает
// таблицы перед тестом. Если DATABASE_URL не задан — тест скипается.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(context.Background(), `TRUNCATE ssp.publishers CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func testPublisher() *domain.Publisher {
	return &domain.Publisher{
		ID:      uuid.NewString(),
		Name:    "T2",
		APIKey:  "test_key_" + uuid.NewString()[:8],
		Balance: 0,
	}
}

func TestPostgresPublisherRepo_AddAndGet(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPublisherRepo(pool)
	ctx := context.Background()

	p := testPublisher()
	if err := repo.Add(ctx, p); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := repo.Get(ctx, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("name = %q, want %q", got.Name, p.Name)
	}
	if got.APIKey != p.APIKey {
		t.Errorf("api_key = %q, want %q", got.APIKey, p.APIKey)
	}
}

func TestPostgresPublisherRepo_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPublisherRepo(pool)

	_, err := repo.Get(context.Background(), uuid.NewString())
	if !errors.Is(err, domain.ErrPublisherNotFound) {
		t.Errorf("want ErrPublisherNotFound, got %v", err)
	}
}

func TestPostgresPublisherRepo_GetByAPIKey(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPublisherRepo(pool)
	ctx := context.Background()

	p := testPublisher()
	_ = repo.Add(ctx, p)

	got, err := repo.GetByAPIKey(ctx, p.APIKey)
	if err != nil {
		t.Fatalf("get by api key: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("id = %q, want %q", got.ID, p.ID)
	}
}

func TestPostgresPublisherRepo_GetByAPIKey_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPublisherRepo(pool)

	_, err := repo.GetByAPIKey(context.Background(), "unknown_key")
	if !errors.Is(err, domain.ErrPublisherNotFound) {
		t.Errorf("want ErrPublisherNotFound, got %v", err)
	}
}

func TestPostgresPublisherRepo_AddBalance(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPublisherRepo(pool)
	ctx := context.Background()

	p := testPublisher()
	_ = repo.Add(ctx, p)

	if err := repo.AddBalance(ctx, p.ID, 1_500_000); err != nil {
		t.Fatalf("add balance: %v", err)
	}

	got, _ := repo.Get(ctx, p.ID)
	if got.Balance != 1_500_000 {
		t.Errorf("balance = %d, want 1_500_000", got.Balance)
	}

	// Ещё раз — баланс копится.
	_ = repo.AddBalance(ctx, p.ID, 500_000)
	got, _ = repo.Get(ctx, p.ID)
	if got.Balance != 2_000_000 {
		t.Errorf("balance = %d, want 2_000_000", got.Balance)
	}
}

func TestPostgresPublisherRepo_AddBalance_NotFound(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPublisherRepo(pool)

	err := repo.AddBalance(context.Background(), uuid.NewString(), 100)
	if !errors.Is(err, domain.ErrPublisherNotFound) {
		t.Errorf("want ErrPublisherNotFound, got %v", err)
	}
}
