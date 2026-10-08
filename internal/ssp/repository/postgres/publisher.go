package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// PublisherRepo — реализация PublisherRepository поверх PostgreSQL.
type PublisherRepo struct {
	pool *pgxpool.Pool
}

// NewPublisherRepo создаёт репозиторий.
func NewPublisherRepo(pool *pgxpool.Pool) *PublisherRepo {
	return &PublisherRepo{pool: pool}
}

// Get возвращает издателя по ID.
func (r *PublisherRepo) Get(ctx context.Context, id string) (*domain.Publisher, error) {
	const q = `
		SELECT id, name, api_key, balance
		FROM publishers
		WHERE id = $1
	`

	var p domain.Publisher
	err := r.pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.Name, &p.APIKey, &p.Balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPublisherNotFound
		}
		return nil, fmt.Errorf("get publisher %s: %w", id, err)
	}
	return &p, nil
}

// GetByAPIKey возвращает издателя по api-key.
func (r *PublisherRepo) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Publisher, error) {
	const q = `
		SELECT id, name, api_key, balance
		FROM publishers
		WHERE api_key = $1
	`

	var p domain.Publisher
	err := r.pool.QueryRow(ctx, q, apiKey).Scan(&p.ID, &p.Name, &p.APIKey, &p.Balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPublisherNotFound
		}
		return nil, fmt.Errorf("get publisher by api key: %w", err)
	}
	return &p, nil
}

// Add добавляет издателя.
//
// Не идемпотентен: повторный вызов с тем же id/api_key вернёт ошибку
// unique violation. Используется только из seed.
func (r *PublisherRepo) Add(ctx context.Context, p *domain.Publisher) error {
	const q = `
		INSERT INTO publishers (id, name, api_key, balance)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.pool.Exec(ctx, q, p.ID, p.Name, p.APIKey, p.Balance)
	if err != nil {
		return fmt.Errorf("insert publisher %s: %w", p.ID, err)
	}
	return nil
}

// AddBalance атомарно увеличивает баланс издателя.
func (r *PublisherRepo) AddBalance(ctx context.Context, id string, amount int64) error {
	const q = `
		UPDATE publishers
		SET balance    = balance + $2,
		    updated_at = now()
		WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, q, id, amount)
	if err != nil {
		return fmt.Errorf("add balance to publisher %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPublisherNotFound
	}
	return nil
}
